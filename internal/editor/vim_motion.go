package editor

import "unicode"

// Arrow keys and Home/End are aliases in Normal and Visual mode.
func motionKey(key string) string {
	switch key {
	case "left":
		return "h"
	case "right":
		return "l"
	case "up":
		return "k"
	case "down":
		return "j"
	case "home":
		return "0"
	case "end":
		return "L"
	default:
		return key
	}
}

func (v *Vim) firstNonblank(d *Document) {
	row, _ := d.Position()
	text := d.Lines()[row]
	col := 0
	for _, r := range text {
		if !unicode.IsSpace(r) {
			break
		}
		col++
	}
	d.SetPosition(row, col)
}
func (v *Vim) motion(d *Document, key string, count int) bool {
	n := max(1, count)
	row, _ := d.Position()
	switch key {
	case "h", "l":
		for range n {
			r, c := d.Position()
			if key == "h" {
				if c > 0 {
					d.Cursor = d.previous(d.Cursor)
				}
			} else if c < d.LineRuneCount(r) {
				d.Cursor = d.next(d.Cursor)
			}
		}
	case "j", "k":
		direction := "down"
		if key == "k" {
			direction = "up"
		}
		for range n {
			d.Move(direction, false)
		}
	case "0":
		d.Move("home", false)
	case "H", "^":
		v.firstNonblank(d)
	case "L", "$":
		d.SetPosition(row+n-1, 0)
		d.Move("end", false)
	case "G", "gg":
		last := len(d.Lines()) - 1
		if last > 0 && d.Text[len(d.Text)-1] == '\n' {
			last--
		}
		target := min(n-1, last)
		if key == "G" && count == 0 {
			target = last
		}
		d.SetPosition(target, 0)
	case "w", "b", "e":
		v.moveWord(d, key, n)
	default:
		return false
	}
	return true
}

func (v *Vim) moveWord(d *Document, key string, count int) {
	end := wordMotionEnd(d)
	class := d.wordClassifier(v.HanDictionary, vimWord)
	for range count {
		before := d.Cursor
		moveOneWord(d, key, end, class)
		if key == "cw" {
			key = "e"
		} else if d.Cursor == before {
			break
		}
	}
}

func wordMotionEnd(d *Document) int {
	d.Lines()
	// The final newline terminates the last line in Vim's buffer.
	if len(d.Text) != 0 && d.Text[len(d.Text)-1] == '\n' {
		return d.runeCount - 1
	}
	return d.runeCount
}
func moveOneWord(d *Document, key string, end int, class func(int) int) {
	emptyLine := func(pos int) bool {
		return (pos == end || d.wordRuneAt(pos) == '\n') && (pos == 0 || d.wordRuneAt(pos-1) == '\n')
	}
	switch key {
	case "w":
		c := class(d.Cursor)
		if d.Cursor < end {
			d.Cursor = d.next(d.Cursor)
		}
		for d.Cursor < end && c != 0 && class(d.Cursor) == c {
			d.Cursor = d.next(d.Cursor)
		}
		for d.Cursor < end && class(d.Cursor) == 0 && !emptyLine(d.Cursor) {
			d.Cursor = d.next(d.Cursor)
		}
	case "b":
		d.Cursor = d.previous(d.Cursor)
		for d.Cursor > 0 && class(d.Cursor) == 0 {
			if emptyLine(d.Cursor) {
				return
			}
			d.Cursor = d.previous(d.Cursor)
		}
		c := class(d.Cursor)
		for d.Cursor > 0 {
			prev := d.previous(d.Cursor)
			if class(prev) != c {
				break
			}
			d.Cursor = prev
		}
	case "e", "cw":
		if key == "e" && d.Cursor < end {
			d.Cursor = d.next(d.Cursor)
		}
		for d.Cursor < end && class(d.Cursor) == 0 {
			d.Cursor = d.next(d.Cursor)
		}
		c := class(d.Cursor)
		for d.Cursor < end {
			next := d.next(d.Cursor)
			if next >= end || class(next) != c {
				break
			}
			d.Cursor = next
		}
	}
}

// MovePage uses the same motion/selection rules for a viewport-sized movement.
func (v *Vim) MovePage(d *Document, rows int) {
	key := "j"
	if rows < 0 {
		key = "k"
		rows = -rows
	}
	count := max(1, v.count)
	v.clearPending()
	v.motion(d, key, min(1000000, rows*count))
	v.syncCursor(d)
}

// ScrollPage lets the view map screen rows through folds and diff headers.
func (v *Vim) ScrollPage(d *Document, rows int, target func(line, delta int) int) {
	if !v.CommandPending() {
		rows *= max(1, v.count)
	}
	v.CancelPending()
	line, col := d.Position()
	d.SetPosition(target(line, rows), col)
	v.syncCursor(d)
}
