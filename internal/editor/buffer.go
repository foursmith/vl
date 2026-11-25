package editor

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxUndoBytes = 8 << 20
	maxUndoSteps = 128
)

// Buffer holds text, selection, and edit history independently of a file.
type Buffer struct {
	Text           string
	Cursor, Anchor int
	ReadOnly       bool
	selectionKind  uint8
	indexedText    string
	lines          []string
	lineOffsets    []int
	byteOffsets    []int
	plainASCII     []bool
	graphemes      map[int][]int
	hanWordRun     hanWordRun
	wordPosition   wordPosition
	runeCount      int
	undo, redo     []editState
	readHistory    func() ([]editState, []editState)
	editGroup      bool
	groupRecorded  bool
	groupStart     editState
}

type editState struct {
	text           string
	cursor, anchor int
	selectionKind  uint8
}

func (d *Buffer) loadUndoHistory() {
	if d.readHistory != nil {
		d.undo, d.redo = d.readHistory()
		d.readHistory = nil
	}
}

// Lines is a read-only view, rebuilt only when document text changes.
func (d *Buffer) Lines() []string {
	if d.lines == nil || d.indexedText != d.Text {
		d.indexedText = d.Text
		d.graphemes = nil
		d.hanWordRun = hanWordRun{}
		d.wordPosition = wordPosition{}
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

func (d *Buffer) LineOffset(line int) int {
	d.Lines()
	return d.lineOffsets[min(max(0, line), len(d.lineOffsets)-1)]
}

func (d *Buffer) LineRuneCount(line int) int {
	d.Lines()
	if line+1 < len(d.lineOffsets) {
		return d.lineOffsets[line+1] - d.lineOffsets[line] - 1
	}
	return d.runeCount - d.lineOffsets[line]
}

func (d *Buffer) PlainASCII(line int) bool { d.Lines(); return d.plainASCII[line] }

func (d *Buffer) Edit(text string) {
	if d.ReadOnly {
		return
	}
	if text == d.Text {
		return
	}
	d.loadUndoHistory()
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
		if i < len(d.undo)-1 && (budget > maxUndoBytes || len(d.undo)-i > maxUndoSteps) {
			clear(d.undo[:i+1])
			d.undo = d.undo[i+1:]
			break
		}
	}
	d.redo = nil
	d.Text = text
}

func (d *Buffer) state() editState { return editState{d.Text, d.Cursor, d.Anchor, d.selectionKind} }
func (d *Buffer) restore(s editState) {
	d.Text, d.Cursor, d.Anchor, d.selectionKind = s.text, s.cursor, s.anchor, s.selectionKind
}

// BeginEditGroup combines a modal edit and its typed text into one undo step.
func (d *Buffer) BeginEditGroup() {
	if !d.editGroup {
		d.editGroup = true
		d.groupRecorded = false
		d.groupStart = d.state()
	}
}
func (d *Buffer) EndEditGroup() {
	d.editGroup = false
	d.groupRecorded = false
	d.groupStart = editState{}
}

func (d *Buffer) Undo() {
	if d.ReadOnly {
		return
	}
	d.EndEditGroup()
	d.loadUndoHistory()
	if len(d.undo) == 0 {
		return
	}
	d.redo = append(d.redo, d.state())
	last := len(d.undo) - 1
	d.restore(d.undo[last])
	d.undo[last] = editState{}
	d.undo = d.undo[:last]
}

func (d *Buffer) Redo() {
	if d.ReadOnly {
		return
	}
	d.EndEditGroup()
	d.loadUndoHistory()
	if len(d.redo) == 0 {
		return
	}
	d.undo = append(d.undo, d.state())
	last := len(d.redo) - 1
	d.restore(d.redo[last])
	d.redo[last] = editState{}
	d.redo = d.redo[:last]
}

func (d *Buffer) Selection() (int, int) {
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
func (d *Buffer) ByteOffset(cursor int) int {
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

func (d *Buffer) SelectedText() string {
	a, b := d.Selection()
	return d.Text[d.ByteOffset(a):d.ByteOffset(b)]
}
func (d *Buffer) SelectAll() { d.Lines(); d.selectionKind = 0; d.Anchor = 0; d.Cursor = d.runeCount }

// SelectVisual keeps the cursor on a character while including both endpoints.
func (d *Buffer) SelectVisual(anchor, cursor int, linewise bool) {
	d.Anchor = d.snap(anchor, false)
	d.Cursor = d.snap(cursor, false)
	d.selectionKind = 1
	if linewise {
		d.selectionKind = 2
	}
}
func (d *Buffer) ClearSelection() { d.Anchor = -1; d.selectionKind = 0 }

func (d *Buffer) Insert(text string) {
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

func (d *Buffer) Delete(backward bool) {
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
	d.deleteRange(a, b)
}

func (d *Buffer) deleteRange(start, end int) {
	a, b := d.ByteOffset(start), d.ByteOffset(end)
	d.Edit(d.Text[:a] + d.Text[b:])
	d.Cursor = start
	d.ClearSelection()
	if !knownBoundary(d.Text, a) {
		d.Cursor = d.snap(d.Cursor, false)
	}
}

// DeleteToLineBoundary removes text on one side of the caret on its current line.
func (d *Buffer) DeleteToLineBoundary(backward bool) bool {
	if d.ReadOnly {
		return false
	}
	cursor := d.snap(d.Cursor, false)
	row, _ := d.positionAt(cursor)
	start, end := cursor, d.LineOffset(row)+d.LineRuneCount(row)
	if backward {
		start, end = d.LineOffset(row), cursor
	}
	if start == end {
		return false
	}
	d.deleteRange(start, end)
	return true
}

func (d *Buffer) Position() (int, int) {
	return d.positionAt(d.Cursor)
}

func (d *Buffer) positionAt(offset int) (int, int) {
	d.Lines()
	cursor := min(max(0, offset), d.runeCount)
	line := sort.Search(len(d.lineOffsets), func(i int) bool { return d.lineOffsets[i] > cursor }) - 1
	return line, cursor - d.lineOffsets[line]
}

func (d *Buffer) Move(key string, selectText bool) {
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

func (d *Buffer) SetPosition(row, col int) {
	d.selectionKind = 0
	lines := d.Lines()
	row = min(max(0, row), len(lines)-1)
	d.Cursor = d.snap(d.LineOffset(row)+min(max(0, col), d.LineRuneCount(row)), false)
	d.Anchor = -1
}
