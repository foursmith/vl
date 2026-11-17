package editor

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

func (v *Vim) TakeAction() string { action := v.action; v.action = ""; return action }
func (v *Vim) TakeClipboardCopy() (string, bool) {
	ready := v.clipboardReady
	v.clipboardReady = false
	text := v.clipboardText
	v.clipboardText = ""
	return text, ready
}
func (v *Vim) handleCommand(d *Document, key string) bool {
	if v.leader {
		v.clear()
		switch key {
		case "enter":
			v.action = "clearSearch"
		case "q":
			v.action = "close"
		case "z":
			v.action = "saveClose"
		case "w":
			v.action = "save"
		}
		return true
	}
	if key == "space" || key == " " {
		v.clear()
		v.leader = true
		return true
	}

	switch key {
	case "p":
		v.clear()
		v.action = "clipboardPaste"
		return true
	case "P":
		v.clear()
		v.action = "clipboardPasteBefore"
		return true
	case "`":
		v.clear()
		v.clamp(d)
		v.word(d, "b", []rune(d.Text))
		if v.VisualMode {
			v.SyncVisual(d)
		}
		v.toggleCase(d)
		return true
	case "tab", "shift+tab":
		v.indent(d, key == "tab", v.VisualMode)
		return true
	case ">", "<":
		if v.VisualMode {
			v.indent(d, key == ">", false)
			return true
		}
	}
	return false
}
func (v *Vim) toggleCase(d *Document) {
	start := d.Cursor
	end := d.next(start)
	wasVisual := v.VisualMode
	if wasVisual {
		start, end = d.Selection()
	}
	if start == end {
		return
	}
	text := d.Text[d.ByteOffset(start):d.ByteOffset(end)]
	changed := strings.Map(func(r rune) rune {
		if unicode.IsUpper(r) {
			return unicode.ToLower(r)
		}
		return unicode.ToUpper(r)
	}, text)
	d.ClearSelection()
	d.SelectRange(start, end)
	d.Insert(changed)
	d.Cursor = d.snap(start+utf8.RuneCountInString(changed), true)
	v.VisualMode, v.VisualLine = false, false
	v.clamp(d)
}
func (v *Vim) indent(d *Document, right, keepVisual bool) {
	cursorRow, cursorCol := d.Position()
	anchorRow, anchorCol := cursorRow, cursorCol
	first, last := cursorRow, cursorRow
	if v.VisualMode {
		anchorRow, anchorCol = positionAt(d, v.visualAnchor)
		start, end := d.Selection()
		first, _ = positionAt(d, start)
		last, _ = positionAt(d, end)
		if last > first && end == d.LineOffset(last) {
			last--
		}
	}
	lines := append([]string(nil), d.Lines()...)
	deltas := make(map[int]int, last-first+1)
	for row := first; row <= last; row++ {
		if right {
			lines[row] = "\t" + lines[row]
			deltas[row] = 1
		} else {
			removed := 0
			if strings.HasPrefix(lines[row], "\t") {
				removed = 1
			} else {
				for removed < len(lines[row]) && removed < 4 && lines[row][removed] == ' ' {
					removed++
				}
			}
			lines[row] = lines[row][removed:]
			deltas[row] = -removed
		}
	}
	d.ClearSelection()
	d.BeginEditGroup()
	d.Edit(strings.Join(lines, "\n"))
	d.EndEditGroup()
	d.SetPosition(cursorRow, max(0, cursorCol+deltas[cursorRow]))
	if keepVisual {
		savedCursor := d.Cursor
		d.SetPosition(anchorRow, max(0, anchorCol+deltas[anchorRow]))
		v.visualAnchor = d.Cursor
		d.Cursor = savedCursor
		v.SyncVisual(d)
	} else {
		v.VisualMode, v.VisualLine = false, false
		v.clamp(d)
	}
	v.clear()
}

// Clipboard text ending in a newline pastes as whole lines. Visual paste
// replaces the current inclusive selection.
func (v *Vim) pasteClipboard(d *Document, text string, after bool) {
	if text == "" {
		return
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if len(d.Text)+len(text) > MaxFileSize {
		return
	}
	d.BeginEditGroup()
	linewise := !v.VisualMode && strings.HasSuffix(text, "\n")
	prefix := false
	if !v.VisualMode {
		d.ClearSelection()
		row, _ := d.Position()
		if linewise {
			if after {
				if row+1 < len(d.Lines()) {
					d.SetPosition(row+1, 0)
				} else {
					d.Move("end", false)
					text = "\n" + strings.TrimSuffix(text, "\n")
					prefix = true
				}
			} else {
				d.Move("home", false)
			}
		} else if after && d.LineRuneCount(row) > 0 {
			d.Cursor = d.next(d.Cursor)
		}
	}
	start, _ := d.Selection()
	d.Insert(text)
	d.EndEditGroup()
	v.VisualMode, v.VisualLine = false, false
	d.Cursor = d.previous(start + utf8.RuneCountInString(text))
	if linewise {
		d.Cursor = start
		if prefix {
			d.Cursor++
		}
	}
	v.clamp(d)
	v.clear()
}

// JumpTo shares the normal and Visual cursor rules with external navigation.
func (v *Vim) JumpTo(d *Document, offset int) {
	v.CancelPending()
	d.Cursor = d.snap(offset, false)
	if v.VisualMode {
		v.SyncVisual(d)
	} else {
		d.ClearSelection()
		v.clamp(d)
	}
}
