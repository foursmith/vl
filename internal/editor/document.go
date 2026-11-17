package editor

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxFileSize = 4 << 20

type Document struct {
	ReadOnly            bool
	Path                string
	Text                string
	saved               []byte
	savedText           string
	savedCRLF, savedBOM bool
	indexedText         string
	lines               []string
	lineOffsets         []int
	byteOffsets         []int
	plainASCII          []bool
	graphemes           map[int][]int
	runeCount           int
	CRLF, BOM           bool
	Cursor, Anchor      int
	selectionKind       uint8
	undo, redo          []editState
	editGroup           bool
	groupRecorded       bool
	groupStart          editState
}

type editState struct {
	text           string
	cursor, anchor int
	selectionKind  uint8
}

func Open(path string) (*Document, error) {
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(real)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("only regular text files can be edited")
	}
	if info.Size() > MaxFileSize {
		return nil, fmt.Errorf("basic editor limit is %d MiB", MaxFileSize>>20)
	}
	f, err := os.Open(real)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxFileSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxFileSize {
		return nil, fmt.Errorf("basic editor limit is %d MiB", MaxFileSize>>20)
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return nil, errors.New("binary or non-UTF-8 file")
	}
	normalized := bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	if bytes.Contains(data, []byte("\r\n")) && bytes.Count(data, []byte("\n")) != bytes.Count(data, []byte("\r\n")) {
		return nil, errors.New("mixed LF/CRLF line endings; normalize the file before editing")
	}
	if bytes.ContainsRune(normalized, '\r') {
		return nil, errors.New("legacy CR line endings are not supported by the basic editor")
	}
	d := &Document{Path: real, saved: data, CRLF: bytes.Contains(data, []byte("\r\n")), BOM: bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}), Anchor: -1}
	d.Text = strings.TrimPrefix(string(normalized), "\xef\xbb\xbf")
	d.savedText, d.savedCRLF, d.savedBOM = d.Text, d.CRLF, d.BOM
	return d, nil
}

func (d *Document) Bytes() []byte {
	text := d.Text
	if d.CRLF {
		text = strings.ReplaceAll(text, "\n", "\r\n")
	}
	if d.BOM {
		text = "\xef\xbb\xbf" + text
	}
	return []byte(text)
}

// SameDisk compares the last disk snapshots without changing save conflict guards.
func (d *Document) SameDisk(other *Document) bool { return bytes.Equal(d.saved, other.saved) }

func (d *Document) Dirty() bool {
	return !d.ReadOnly && (d.Text != d.savedText || d.CRLF != d.savedCRLF || d.BOM != d.savedBOM)
}

// Lines is a read-only view, rebuilt only when document text changes.
func (d *Document) Lines() []string {
	if d.lines == nil || d.indexedText != d.Text {
		d.indexedText = d.Text
		d.graphemes = nil
		d.lines = strings.Split(d.Text, "\n")
		d.lineOffsets = make([]int, len(d.lines))
		d.byteOffsets = make([]int, len(d.lines))
		d.plainASCII = make([]bool, len(d.lines))
		offset, byteOffset := 0, 0
		for i, line := range d.lines {
			d.lineOffsets[i] = offset
			d.byteOffsets[i] = byteOffset
			byteOffset += len(line) + 1
			length := utf8.RuneCountInString(line)
			plain := length == len(line)
			if plain {
				for j := 0; j < len(line); j++ {
					if line[j] < 32 || line[j] > 126 {
						plain = false
						break
					}
				}
			}
			d.plainASCII[i] = plain
			offset += length + 1
		}
		d.runeCount = offset - 1
	}
	return d.lines
}

func (d *Document) LineOffset(line int) int {
	d.Lines()
	return d.lineOffsets[min(max(0, line), len(d.lineOffsets)-1)]
}

func (d *Document) LineRuneCount(line int) int {
	d.Lines()
	if line+1 < len(d.lineOffsets) {
		return d.lineOffsets[line+1] - d.lineOffsets[line] - 1
	}
	return d.runeCount - d.lineOffsets[line]
}

func (d *Document) PlainASCII(line int) bool { d.Lines(); return d.plainASCII[line] }

func (d *Document) Edit(text string) {
	if d.ReadOnly {
		return
	}
	if text == d.Text {
		return
	}
	if !d.editGroup || !d.groupRecorded {
		snapshot := d.state()
		if d.editGroup {
			snapshot = d.groupStart
		}
		d.undo = append(d.undo, snapshot)
		d.groupRecorded = true
	}
	budget := 0
	for i := len(d.undo) - 1; i >= 0; i-- {
		budget += len(d.undo[i].text)
		if i < len(d.undo)-1 && (budget > 8<<20 || len(d.undo)-i > 128) {
			clear(d.undo[:i+1])
			d.undo = d.undo[i+1:]
			break
		}
	}
	d.redo = nil
	d.Text = text
}

func (d *Document) state() editState { return editState{d.Text, d.Cursor, d.Anchor, d.selectionKind} }
func (d *Document) restore(s editState) {
	d.Text, d.Cursor, d.Anchor, d.selectionKind = s.text, s.cursor, s.anchor, s.selectionKind
}

// BeginEditGroup combines a modal edit and its typed text into one undo step.
func (d *Document) BeginEditGroup() {
	if !d.editGroup {
		d.editGroup = true
		d.groupRecorded = false
		d.groupStart = d.state()
	}
}
func (d *Document) EndEditGroup() {
	d.editGroup = false
	d.groupRecorded = false
	d.groupStart = editState{}
}

func (d *Document) Undo() {
	if d.ReadOnly {
		return
	}
	d.EndEditGroup()
	if len(d.undo) == 0 {
		return
	}
	d.redo = append(d.redo, d.state())
	last := len(d.undo) - 1
	d.restore(d.undo[last])
	d.undo[last] = editState{}
	d.undo = d.undo[:last]
}

func (d *Document) Redo() {
	if d.ReadOnly {
		return
	}
	d.EndEditGroup()
	if len(d.redo) == 0 {
		return
	}
	d.undo = append(d.undo, d.state())
	last := len(d.redo) - 1
	d.restore(d.redo[last])
	d.redo[last] = editState{}
	d.redo = d.redo[:last]
}

func (d *Document) Selection() (int, int) {
	if d.Anchor < 0 || d.Anchor == d.Cursor && d.selectionKind == 0 {
		pos := d.snap(d.Cursor, false)
		return pos, pos
	}
	start, end := d.snap(min(d.Cursor, d.Anchor), false), d.snap(max(d.Cursor, d.Anchor), true)
	if d.selectionKind == 1 {
		end = d.next(end)
	}
	if d.selectionKind == 2 {
		first := sort.Search(len(d.lineOffsets), func(i int) bool { return d.lineOffsets[i] > start }) - 1
		last := sort.Search(len(d.lineOffsets), func(i int) bool { return d.lineOffsets[i] > end }) - 1
		start = d.LineOffset(first)
		end = d.runeCount
		if last+1 < len(d.Lines()) {
			end = d.LineOffset(last + 1)
		}
	}
	return start, end
}

// ByteOffset translates the editor's rune cursor using only its containing line.
func (d *Document) ByteOffset(cursor int) int {
	d.Lines()
	cursor = min(max(0, cursor), d.runeCount)
	line := sort.Search(len(d.lineOffsets), func(i int) bool { return d.lineOffsets[i] > cursor }) - 1
	offset := d.byteOffsets[line]
	remaining := cursor - d.lineOffsets[line]
	if d.plainASCII[line] {
		return offset + remaining
	}
	for byteIndex := range d.lines[line] {
		if remaining == 0 {
			return offset + byteIndex
		}
		remaining--
	}
	return offset + len(d.lines[line])
}

func (d *Document) SelectedText() string {
	a, b := d.Selection()
	return d.Text[d.ByteOffset(a):d.ByteOffset(b)]
}
func (d *Document) SelectAll() { d.Lines(); d.selectionKind = 0; d.Anchor = 0; d.Cursor = d.runeCount }

// SelectVisual keeps the cursor on a character while including both endpoints.
func (d *Document) SelectVisual(anchor, cursor int, linewise bool) {
	d.Anchor = d.snap(anchor, false)
	d.Cursor = d.snap(cursor, false)
	d.selectionKind = 1
	if linewise {
		d.selectionKind = 2
	}
}
func (d *Document) ClearSelection() { d.Anchor = -1; d.selectionKind = 0 }

func (d *Document) Insert(text string) {
	if d.ReadOnly {
		return
	}
	a, b := d.Selection()
	start, end := d.ByteOffset(a), d.ByteOffset(b)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	d.Edit(d.Text[:start] + text + d.Text[end:])
	d.Cursor, d.Anchor = a+utf8.RuneCountInString(text), -1
	if !knownBoundary(d.Text, start+len(text)) {
		d.Cursor = d.snap(d.Cursor, true)
	}
}

func (d *Document) Delete(backward bool) {
	if d.ReadOnly {
		return
	}
	a, b := d.Selection()
	if a == b {
		d.Lines()
		if backward && a > 0 {
			a = d.previous(a)
		} else if !backward && b < d.runeCount {
			b = d.next(b)
		} else {
			return
		}
	}
	start, end := d.ByteOffset(a), d.ByteOffset(b)
	d.Edit(d.Text[:start] + d.Text[end:])
	d.Cursor, d.Anchor = a, -1
	if !knownBoundary(d.Text, start) {
		d.Cursor = d.snap(d.Cursor, false)
	}
}

func (d *Document) Position() (int, int) {
	d.Lines()
	cursor := min(max(0, d.Cursor), d.runeCount)
	line := sort.Search(len(d.lineOffsets), func(i int) bool { return d.lineOffsets[i] > cursor }) - 1
	return line, cursor - d.lineOffsets[line]
}

func (d *Document) Move(key string, selectText bool) {
	d.selectionKind = 0
	d.Cursor = d.snap(d.Cursor, false)
	if !selectText && d.Anchor >= 0 && d.Anchor != d.Cursor && (key == "left" || key == "right") {
		a, b := d.Selection()
		d.Cursor = a
		if key == "right" {
			d.Cursor = b
		}
		d.Anchor = -1
		return
	}
	if selectText {
		if d.Anchor < 0 {
			d.Anchor = d.Cursor
		}
	} else {
		d.Anchor = -1
	}
	d.Lines()
	line, col := d.Position()
	start := d.lineOffsets[line]
	end := start + d.LineRuneCount(line)
	switch key {
	case "left":
		d.Cursor = d.previous(d.Cursor)
	case "right":
		d.Cursor = d.next(d.Cursor)
	case "home":
		d.Cursor = start
	case "end":
		d.Cursor = end
	case "ctrl+home":
		d.Cursor = 0
	case "ctrl+end":
		d.Cursor = d.runeCount
	case "up":
		if line > 0 {
			d.Cursor = d.lineOffsets[line-1] + min(col, d.LineRuneCount(line-1))
		}
	case "down":
		if line+1 < len(d.lines) {
			d.Cursor = d.lineOffsets[line+1] + min(col, d.LineRuneCount(line+1))
		}
	case "ctrl+left":
		offset := d.ByteOffset(d.Cursor)
		for offset > 0 {
			r, size := utf8.DecodeLastRuneInString(d.Text[:offset])
			if !unicode.IsSpace(r) {
				break
			}
			offset -= size
			d.Cursor--
		}
		for offset > 0 {
			r, size := utf8.DecodeLastRuneInString(d.Text[:offset])
			if unicode.IsSpace(r) {
				break
			}
			offset -= size
			d.Cursor--
		}
	case "ctrl+right":
		offset := d.ByteOffset(d.Cursor)
		for offset < len(d.Text) {
			r, size := utf8.DecodeRuneInString(d.Text[offset:])
			if unicode.IsSpace(r) {
				break
			}
			offset += size
			d.Cursor++
		}
		for offset < len(d.Text) {
			r, size := utf8.DecodeRuneInString(d.Text[offset:])
			if !unicode.IsSpace(r) {
				break
			}
			offset += size
			d.Cursor++
		}
	}
	d.Cursor = d.snap(d.Cursor, key == "ctrl+right" || key == "right")
}

func (d *Document) SetPosition(row, col int) {
	d.selectionKind = 0
	lines := d.Lines()
	row = min(max(0, row), len(lines)-1)
	d.Cursor = d.snap(d.LineOffset(row)+min(max(0, col), d.LineRuneCount(row)), false)
	d.Anchor = -1
}

func (d *Document) Save() error {
	if d.ReadOnly {
		return errors.New("file is read-only")
	}
	current, err := os.ReadFile(d.Path)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, d.saved) {
		return errors.New("file changed on disk; reopen it before saving")
	}
	info, err := os.Stat(d.Path)
	if err != nil {
		return err
	}
	if info.Mode().Perm()&0222 == 0 {
		return errors.New("file is read-only")
	}
	tmp, err := os.CreateTemp(filepath.Dir(d.Path), ".vl-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	data := d.Bytes()
	if err = tmp.Chmod(info.Mode().Perm()); err == nil {
		_, err = tmp.Write(data)
	}
	if err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	// Recheck after writing the temporary file so external edits are not silently overwritten.
	current, err = os.ReadFile(d.Path)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, d.saved) {
		return errors.New("file changed on disk while saving")
	}
	if err = os.Rename(tmp.Name(), d.Path); err != nil {
		return err
	}
	d.saved = data
	d.savedText, d.savedCRLF, d.savedBOM = d.Text, d.CRLF, d.BOM
	return nil
}
