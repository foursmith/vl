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
func (v *Vim) motion(d *Document, key string, n int) bool {
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
	case "G":
		d.SetPosition(n-1, 0)
	case "w", "b", "e":
		runes := []rune(d.Text)
		for range n {
			before := d.Cursor
			v.word(d, key, runes)
			if d.Cursor == before {
				break
			}
		}
	default:
		return false
	}
	return true
}
func wordClass(r rune) int {
	if unicode.IsSpace(r) {
		return 0
	}
	if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
		return 1
	}
	return 2
}
func (v *Vim) word(d *Document, key string, runes []rune) {
	class := func(pos int) int {
		if pos >= d.runeCount {
			return 0
		}
		return wordClass(runes[pos])
	}
	d.Lines()
	end := d.runeCount
	switch key {
	case "w":
		c := class(d.Cursor)
		for d.Cursor < end && class(d.Cursor) == c {
			d.Cursor = d.next(d.Cursor)
		}
		for d.Cursor < end && class(d.Cursor) == 0 {
			d.Cursor = d.next(d.Cursor)
		}
	case "b":
		d.Cursor = d.previous(d.Cursor)
		for d.Cursor > 0 && class(d.Cursor) == 0 {
			d.Cursor = d.previous(d.Cursor)
		}
		c := class(d.Cursor)
		for d.Cursor > 0 && class(d.previous(d.Cursor)) == c {
			d.Cursor = d.previous(d.Cursor)
		}
	case "e":
		if d.Cursor < end {
			d.Cursor = d.next(d.Cursor)
		}
		for d.Cursor < end && class(d.Cursor) == 0 {
			d.Cursor = d.next(d.Cursor)
		}
		c := class(d.Cursor)
		for d.Cursor < end && d.next(d.Cursor) < end && class(d.next(d.Cursor)) == c {
			d.Cursor = d.next(d.Cursor)
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
