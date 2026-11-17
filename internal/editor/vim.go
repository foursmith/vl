package editor

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Vim interprets basic modal commands against the existing document model.
// Its zero value starts in Normal mode.
type Vim struct {
	InsertMode             bool
	VisualMode, VisualLine bool
	visualAnchor           int
	pending                string
	count, operatorCount   int
	clipboardText          string
	action                 string
	clipboardReady         bool
	record                 *vimRecording
	lastChange             *vimChange
	surround               *vimSurround
}

func (v *Vim) Label() string {
	if v.InsertMode {
		return "INSERT"
	}
	if v.surround != nil {
		label := "SURROUND " + v.surround.kind
		if v.surround.phase == surroundTag {
			text := []rune(v.surround.tag)
			if len(text) > 20 {
				text = text[len(text)-20:]
			}
			label += " " + string(text)
		}
		return label
	}
	if v.VisualMode {
		if v.VisualLine {
			return "VISUAL LINE"
		}
		return "VISUAL"
	}
	if v.pending == "" && v.count == 0 {
		return "NORMAL"
	}
	return "NORMAL · " + v.pending + v.countLabel()
}
func (v *Vim) countLabel() string {
	if v.count == 0 {
		return ""
	}
	return " " + strconv.Itoa(v.count)
}
func (v *Vim) Reset(d *Document) {
	d.EndEditGroup()
	v.InsertMode = false
	v.VisualMode, v.VisualLine = false, false
	v.clear()
	d.ClearSelection()
	v.clamp(d)
}
func (v *Vim) CancelPending() {
	v.clear()
	if v.InsertMode && v.record != nil {
		v.record.unsafe = true
	} else {
		v.record = nil
	}
}

func (v *Vim) clear() {
	v.pending = ""
	v.count = 0
	v.operatorCount = 0
	v.surround = nil
}
func (v *Vim) clamp(d *Document) {
	row, col := d.Position()
	end := d.LineOffset(row) + d.LineRuneCount(row)
	d.Cursor = d.snap(d.Cursor, false)
	if col > 0 && d.Cursor >= end {
		d.Cursor = d.previous(end)
	}
}
func (v *Vim) insert(d *Document) {
	v.clear()
	d.Anchor = -1
	d.BeginEditGroup()
	v.InsertMode = true
}

// Handle returns false for input the workbench or Insert-mode editor should own.
func (v *Vim) handleKey(d *Document, key string) bool {
	if key == "esc" || key == "ctrl+[" {
		if v.InsertMode {
			row, col := d.Position()
			if col > 0 {
				d.SetPosition(row, col-1)
			}
		}
		v.Reset(d)
		return true
	}
	if v.InsertMode {
		return false
	}
	if v.surround != nil {
		return v.handleSurround(d, key)
	}
	if v.pending == "" && v.handleCommand(d, key) {
		return true
	}
	if v.VisualMode && d.Anchor < 0 {
		v.VisualMode, v.VisualLine = false, false
	}
	if strings.HasPrefix(key, "ctrl+") || strings.HasPrefix(key, "alt+") || strings.HasPrefix(key, "shift+") || strings.HasPrefix(key, "f") && len(key) > 1 {
		v.clear()
		return false
	}
	if v.VisualMode {
		return v.handleVisual(d, key)
	}
	v.clamp(d)
	d.Anchor = -1
	if len(key) == 1 && key[0] >= '0' && key[0] <= '9' && (key != "0" || v.count > 0) {
		v.count = min(9999, v.count*10+int(key[0]-'0'))
		return true
	}
	n := max(1, v.count)
	if v.pending != "" {
		op := v.pending
		n = min(9999, n*max(1, v.operatorCount))
		v.clear()
		if op == "z" {
			switch key {
			case "a", "c", "o", "M", "R", "C", "O":
				v.action = "fold:" + key
			}
			return true
		}
		if (op == "c" || op == "d" || op == "y") && key == "s" {
			v.beginSurround(d, op, n)
			return true
		}
		if len(op) == 1 && op != "g" && (key == "i" || key == "a") {
			v.pending = op + key
			v.operatorCount = n
			return true
		}
		if len(op) == 2 {
			if start, end, ok := textObject(d, key, op[1] == 'i', n); ok {
				v.operate(d, op[:1], start, end, false)
			}
			v.clampUnlessInsert(d)
			return true
		}
		if op == "g" {
			if key == "g" {
				d.SetPosition(n-1, 0)
			}
		} else if key == op {
			v.lines(d, op, n)
		} else {
			start := d.Cursor

			if key == "j" || key == "k" {
				row, _ := d.Position()
				target := row + n
				if key == "k" {
					target = max(0, row-n)
				}
				d.SetPosition(min(row, target), 0)
				v.lines(d, op, abs(target-row)+1)
				v.clampUnlessInsert(d)
				return true
			}
			if op == "c" && key == "w" {
				v.action = "focus"
				return true
			}
			if v.motion(d, key, n, true) {
				end := d.Cursor
				startRow, _ := positionAt(d, start)
				endRow, _ := d.Position()
				if key == "w" && endRow > startRow {
					for end > start {
						prev := d.previous(end)
						r, _ := utf8.DecodeRuneInString(d.Text[d.ByteOffset(prev):])
						if !unicode.IsSpace(r) {
							break
						}
						end = prev
					}
				}
				if key == "e" {
					end = d.next(end)
				}
				v.operate(d, op, start, end, false)
			}
		}
		v.clampUnlessInsert(d)
		return true
	}
	if key == "d" || key == "c" || key == "y" || key == "g" || key == "z" {
		v.pending = key
		v.operatorCount = n
		v.count = 0
		return true
	}
	hadCount := v.count > 0
	v.clear()
	if key == "G" && !hadCount {
		n = len(d.Lines())
	}

	if v.motion(d, key, n, false) {
		v.clamp(d)
		return true
	}
	row, _ := d.Position()
	switch key {
	case "v", "V":
		v.startVisual(d, key == "V")
	case "i":
		v.insert(d)
	case "a":
		if d.LineRuneCount(row) > 0 {
			d.Cursor = d.next(d.Cursor)
		}
		v.insert(d)
	case "I":
		v.firstNonblank(d)
		v.insert(d)
	case "A":
		d.Move("end", false)
		v.insert(d)
	case "o", "O":
		v.insert(d)
		if key == "o" {
			d.Move("end", false)
			d.Insert("\n")
		} else {
			d.Move("home", false)
			d.Insert("\n")
			d.SetPosition(row, 0)
		}
	case "x":
		start := d.Cursor
		end := d.LineOffset(row) + d.LineRuneCount(row)
		for range n {
			if d.Cursor < end {
				d.Cursor = d.next(d.Cursor)
			}
		}
		v.operate(d, "d", start, d.Cursor, false)
	case "D", "C":
		end := d.LineOffset(row) + d.LineRuneCount(row)
		op := "d"
		if key == "C" {
			op = "c"
		}
		v.operate(d, op, d.Cursor, end, false)
	case "u":
		for range n {
			d.Undo()
		}
	case "U":
		for range n {
			d.Redo()
		}
	case "left", "right", "up", "down", "home", "end":
		motion := map[string]string{"left": "h", "right": "l", "up": "k", "down": "j", "home": "0", "end": "L"}[key]
		v.motion(d, motion, n, false)
	case "pgup", "pgdown":
		return false
	default:
		// Ordinary keys with no Vim meaning must not insert text in Normal mode.
	}
	v.clampUnlessInsert(d)
	return true
}
func (v *Vim) clampUnlessInsert(d *Document) {
	if v.VisualMode {
		v.SyncVisual(d)
		return
	}
	if !v.InsertMode {
		v.clamp(d)
		d.Anchor = -1
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
func (v *Vim) motion(d *Document, key string, n int, operator bool) bool {
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
		if !operator {
			v.clamp(d)
		}
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
func positionAt(d *Document, cursor int) (int, int) {
	old := d.Cursor
	d.Cursor = cursor
	row, col := d.Position()
	d.Cursor = old
	return row, col
}
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
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
func (v *Vim) lines(d *Document, op string, n int) {
	cursor := d.Cursor
	row, _ := d.Position()
	last := min(row+n, len(d.Lines()))
	start := d.LineOffset(row)
	end := d.runeCount
	if last < len(d.Lines()) {
		end = d.LineOffset(last)
	}
	v.operate(d, op, start, end, true)
	if op == "y" {
		d.Cursor = cursor
	}
}
func (v *Vim) operate(d *Document, op string, start, end int, linewise bool) {
	if start > end {
		start, end = end, start
	}
	if op != "y" {
		d.Cursor = start
		d.BeginEditGroup()
	}
	d.SelectRange(start, end)
	text := d.SelectedText()
	emptyLine := text == ""
	if text != "" || linewise {
		v.clipboardText = text
		if linewise && !strings.HasSuffix(text, "\n") {
			v.clipboardText += "\n"
		}
		v.clipboardReady = true
	}
	if op == "y" {
		d.Cursor = start
		d.Anchor = -1
		return
	}
	if linewise && op == "d" && end == d.runeCount && start > 0 && (!strings.HasSuffix(text, "\n") || emptyLine) {
		// Removing the final line must also remove its preceding separator.
		d.SelectRange(d.previous(start), end)
	}
	if linewise && op == "c" && strings.HasSuffix(text, "\n") {
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
}

// MovePage uses the same motion/selection rules for a viewport-sized movement.
func (v *Vim) MovePage(d *Document, rows int) {
	key := "j"
	if rows < 0 {
		key = "k"
		rows = -rows
	}
	count := max(1, v.count)
	v.clear()
	v.motion(d, key, min(1000000, rows*count), false)
	v.clampUnlessInsert(d)
}
