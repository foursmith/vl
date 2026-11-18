package editor

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

func (v *Vim) handleOperator(d *Document, op, key string, count int) string {
	switch op {
	case "di", "da", "ci", "ca", "yi", "ya":
		if start, end, ok := textObject(d, key, op[1] == 'i', count); ok {
			return v.operate(d, op[:1], start, end, false)
		}
		return ""
	}

	switch key {
	case "s":
		v.beginSurround(d, op, count)
	case "i", "a":
		v.beginPending(op+key, count)
	case op:
		return v.operateLines(d, op, count)
	case "j", "k":
		row, _ := d.Position()
		first, last := row, row+count
		if key == "k" {
			first, last = max(0, row-count), row
		}
		d.SetPosition(first, 0)
		return v.operateLines(d, op, last-first+1)
	default:
		if op == "c" && key == "w" {
			v.action = "focus"
			return ""
		}
		return v.operateMotion(d, op, key, count)
	}
	return ""
}

func (v *Vim) operateMotion(d *Document, op, key string, count int) string {
	start := d.Cursor
	if !v.motion(d, key, count) {
		return ""
	}
	end := d.Cursor
	switch key {
	case "w":
		startRow, _ := d.positionAt(start)
		endRow, _ := d.Position()
		if endRow > startRow {
			// Stop before trailing whitespace when w crosses a line boundary.
			for end > start {
				prev := d.previous(end)
				r, _ := utf8.DecodeRuneInString(d.Text[d.ByteOffset(prev):])
				if !unicode.IsSpace(r) {
					break
				}
				end = prev
			}
		}
	case "e":
		// Include the last grapheme of the word.
		end = d.next(end)
	}
	return v.operate(d, op, start, end, false)
}

func (v *Vim) operateLines(d *Document, op string, n int) string {
	cursor := d.Cursor
	row, _ := d.Position()
	last := min(row+n, len(d.Lines()))
	start := d.LineOffset(row)
	end := d.runeCount
	if last < len(d.Lines()) {
		end = d.LineOffset(last)
	}
	copied := v.operate(d, op, start, end, true)
	if op == "y" {
		d.Cursor = cursor
	}
	return copied
}

func (v *Vim) operate(d *Document, op string, start, end int, linewise bool) string {
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
	register := text
	if linewise && !hasNewline {
		register += "\n"
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
