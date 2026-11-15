package editor

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxFileSize = 4 << 20

type Document struct {
	Path           string
	Text           string
	saved          []byte
	CRLF, BOM      bool
	Cursor, Anchor int
	undo, redo     []editState
}

type editState struct {
	text           string
	cursor, anchor int
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
	data, err := os.ReadFile(real)
	if err != nil {
		return nil, err
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
	d := &Document{Path: real, saved: bytes.Clone(data), CRLF: bytes.Contains(data, []byte("\r\n")), BOM: bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}), Anchor: -1}
	d.Text = strings.TrimPrefix(string(normalized), "\xef\xbb\xbf")
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

func (d *Document) Dirty() bool { return !bytes.Equal(d.Bytes(), d.saved) }

func (d *Document) Edit(text string) {
	if text == d.Text {
		return
	}
	d.undo = append(d.undo, d.state())
	budget := 0
	for i := len(d.undo) - 1; i >= 0; i-- {
		budget += len(d.undo[i].text)
		if i < len(d.undo)-1 && (budget > 8<<20 || len(d.undo)-i > 128) {
			d.undo = d.undo[i+1:]
			break
		}
	}
	d.redo = nil
	d.Text = text
}

func (d *Document) state() editState    { return editState{d.Text, d.Cursor, d.Anchor} }
func (d *Document) restore(s editState) { d.Text, d.Cursor, d.Anchor = s.text, s.cursor, s.anchor }

func (d *Document) Undo() {
	if len(d.undo) == 0 {
		return
	}
	d.redo = append(d.redo, d.state())
	d.restore(d.undo[len(d.undo)-1])
	d.undo = d.undo[:len(d.undo)-1]
}

func (d *Document) Redo() {
	if len(d.redo) == 0 {
		return
	}
	d.undo = append(d.undo, d.state())
	d.restore(d.redo[len(d.redo)-1])
	d.redo = d.redo[:len(d.redo)-1]
}

func (d *Document) Selection() (int, int) {
	if d.Anchor < 0 {
		return d.Cursor, d.Cursor
	}
	return min(d.Cursor, d.Anchor), max(d.Cursor, d.Anchor)
}

func (d *Document) SelectedText() string { a, b := d.Selection(); return string([]rune(d.Text)[a:b]) }
func (d *Document) SelectAll()           { d.Anchor = 0; d.Cursor = utf8.RuneCountInString(d.Text) }

func (d *Document) Insert(text string) {
	a, b := d.Selection()
	runes := []rune(d.Text)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	d.Edit(string(runes[:a]) + text + string(runes[b:]))
	d.Cursor = a + utf8.RuneCountInString(text)
	d.Anchor = -1
}

func (d *Document) Delete(backward bool) {
	a, b := d.Selection()
	if a == b {
		if backward && a > 0 {
			a--
		} else if !backward && b < utf8.RuneCountInString(d.Text) {
			b++
		} else {
			return
		}
	}
	runes := []rune(d.Text)
	d.Edit(string(runes[:a]) + string(runes[b:]))
	d.Cursor, d.Anchor = a, -1
}

func (d *Document) Position() (int, int) {
	line, col := 0, 0
	for i, r := range []rune(d.Text) {
		if i >= d.Cursor {
			break
		}
		if r == '\n' {
			line++
			col = 0
		} else {
			col++
		}
	}
	return line, col
}

func (d *Document) Move(key string, selectText bool) {
	if selectText {
		if d.Anchor < 0 {
			d.Anchor = d.Cursor
		}
	} else {
		d.Anchor = -1
	}
	runes := []rune(d.Text)
	start, end := d.Cursor, d.Cursor
	for start > 0 && runes[start-1] != '\n' {
		start--
	}
	for end < len(runes) && runes[end] != '\n' {
		end++
	}
	switch key {
	case "left":
		d.Cursor--
	case "right":
		d.Cursor++
	case "home":
		d.Cursor = start
	case "end":
		d.Cursor = end
	case "ctrl+home":
		d.Cursor = 0
	case "ctrl+end":
		d.Cursor = len(runes)
	case "up":
		if start > 0 {
			prev := start - 1
			for prev > 0 && runes[prev-1] != '\n' {
				prev--
			}
			d.Cursor = min(prev+d.Cursor-start, start-1)
		}
	case "down":
		if end < len(runes) {
			next := end + 1
			for next < len(runes) && runes[next] != '\n' {
				next++
			}
			d.Cursor = min(end+1+d.Cursor-start, next)
		}
	case "ctrl+left":
		for d.Cursor > 0 && unicode.IsSpace(runes[d.Cursor-1]) {
			d.Cursor--
		}
		for d.Cursor > 0 && !unicode.IsSpace(runes[d.Cursor-1]) {
			d.Cursor--
		}
	case "ctrl+right":
		for d.Cursor < len(runes) && !unicode.IsSpace(runes[d.Cursor]) {
			d.Cursor++
		}
		for d.Cursor < len(runes) && unicode.IsSpace(runes[d.Cursor]) {
			d.Cursor++
		}
	}
	d.Cursor = min(max(0, d.Cursor), len(runes))
}

func (d *Document) SetPosition(row, col int) {
	lines := strings.Split(d.Text, "\n")
	row = min(max(0, row), len(lines)-1)
	pos := 0
	for i := 0; i < row; i++ {
		pos += utf8.RuneCountInString(lines[i]) + 1
	}
	d.Cursor = pos + min(max(0, col), utf8.RuneCountInString(lines[row]))
	d.Anchor = -1
}

func (d *Document) Save() error {
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
	tmp, err := os.CreateTemp(filepath.Dir(d.Path), ".elegit-*")
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
	d.saved = bytes.Clone(data)
	return nil
}
