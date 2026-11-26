package editor

import (
	"strings"
	"unicode/utf8"

	"github.com/clipperhouse/uax29/v2/graphemes"
)

func characterRangeEnd(d *Document, count int) (int, bool) {
	row, _ := d.Position()
	end := d.Cursor
	lineEnd := d.LineOffset(row) + d.LineRuneCount(row)
	for range count {
		if end == lineEnd {
			return end, false
		}
		end = d.next(end)
	}
	return end, true
}

func replaceCharacter(d *Document, key string, count int) {
	end, full := characterRangeEnd(d, count)
	if !full {
		return
	}
	switch key {
	case "space":
		key = " "
	case "tab":
		key = "\t"
	case "enter":
		key, count = "\n", 1
	default:
		iter := graphemes.FromString(key)
		if !iter.Next() || iter.End() != len(key) {
			return
		}
	}
	start := d.Cursor
	d.BeginEditGroup()
	d.SelectRange(start, end)
	d.Insert(strings.Repeat(key, count))
	if key != "\n" {
		d.Cursor = d.snap(start+(count-1)*utf8.RuneCountInString(key), false)
	}
	d.EndEditGroup()
}

func (v *Vim) handleOperator(d *Document, op, key string, count int, hadCount bool) VimRegisterValue {
	switch op {
	case "di", "da", "ci", "ca", "yi", "ya":
		if start, end, ok := v.textObject(d, key, op[1] == 'i', count); ok {
			return v.operate(d, op[:1], start, end, false)
		}
		return VimRegisterValue{}
	case "dg", "cg", "yg":
		if key == "g" {
			return v.operateMotion(d, op[:1], "gg", count)
		}
		return VimRegisterValue{}
	}

	switch key {
	case "s":
		v.beginSurround(d, op, count)
	case "i", "a", "g":
		v.beginPending(op+key, count)
	case op:
		row, _ := d.Position()
		return v.operateLineRange(d, op, row, row+count-1)
	default:
		if key == "G" && !hadCount {
			count = 0
		}
		return v.operateMotion(d, op, key, count)
	}
	return VimRegisterValue{}
}

func (v *Vim) operateMotion(d *Document, op, key string, count int) VimRegisterValue {
	start := d.Cursor
	if op == "c" && key == "w" && wordClass(d.wordRuneAt(start)) != 0 {
		// Include the current word, even when the cursor is on its last character.
		v.moveWord(d, "cw", count)
		return v.operate(d, op, start, d.next(d.Cursor), false)
	}
	if !v.motion(d, key, count) {
		return VimRegisterValue{}
	}
	end := d.Cursor
	switch key {
	case "j", "k", "G", "gg":
		first, _ := d.positionAt(start)
		last, _ := d.Position()
		d.Cursor = start
		return v.operateLineRange(d, op, first, last)
	case "w":
		startRow, _ := d.positionAt(start)
		endRow, _ := d.Position()
		if endRow > startRow {
			// Stop before trailing whitespace when w crosses a line boundary.
			for end > start {
				prev := d.previous(end)
				if wordClass(d.wordRuneAt(prev)) != 0 {
					break
				}
				end = prev
			}
		}
	case "e":
		// Include the last grapheme of the word.
		if end < wordMotionEnd(d) {
			end = d.next(end)
		}
	}
	return v.operate(d, op, start, end, false)
}

func (v *Vim) operateLineRange(d *Document, op string, first, last int) VimRegisterValue {
	first, last = min(first, last), max(first, last)
	cursor := d.Cursor
	start := d.LineOffset(first)
	end := d.runeCount
	if last+1 < len(d.Lines()) {
		end = d.LineOffset(last + 1)
	}
	copied := v.operate(d, op, start, end, true)
	if op == "y" {
		d.Cursor = cursor
	}
	return copied
}

func (v *Vim) operate(d *Document, op string, start, end int, linewise bool) VimRegisterValue {
	if start > end {
		start, end = end, start
	}
	if op != "y" {
		d.Cursor = start
		d.BeginEditGroup()
	}
	d.SelectRange(start, end)
	text := d.SelectedText()
	hasNewline := strings.HasSuffix(text, "\n")
	register := VimRegisterValue{Text: text, Linewise: linewise}
	if linewise && !hasNewline {
		register.Text += "\n"
	}
	if op == "y" {
		d.Cursor = start
		d.Anchor = -1
		return register
	}
	if linewise && op == "d" && end == d.runeCount && start > 0 && !hasNewline {
		// Removing the final line must also remove its preceding separator.
		d.SelectRange(d.previous(start), end)
	}
	if linewise && op == "c" && hasNewline {
		d.Insert("\n")
		d.Cursor = start
	} else {
		d.Insert("")
	}
	if op == "c" {
		v.InsertMode = true
	} else {
		d.EndEditGroup()
	}
	return register
}
