package editor

import (
	"strconv"
	"strings"
)

type VimRegister uint8

const (
	UnnamedRegister VimRegister = iota
	ClipboardRegister
)

// VimRegisterValue preserves how text was selected as well as its contents.
type VimRegisterValue struct {
	Text     string
	Linewise bool
}

// Vim interprets basic modal commands against the existing document model.
// Its zero value starts in Normal mode.
type Vim struct {
	InsertMode             bool
	VisualMode, VisualLine bool
	HanDictionary          func() *HanDictionary
	visualAnchor           int
	pending                string
	register               VimRegister
	count, prefixCount     int
	foldAction             string
	record                 *vimRecording
	lastChange             *vimChange
	surround               *vimSurround
}

func (v *Vim) Label() string {
	if v.InsertMode {
		return "INSERT"
	}
	if v.surround != nil {
		return "SURROUND " + v.surround.kind
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
	v.register = UnnamedRegister
	v.clearPending()
	d.ClearSelection()
	v.clamp(d)
}

func (v *Vim) CancelPending() {
	v.register = UnnamedRegister
	v.clearPending()
	if v.InsertMode && v.record != nil {
		v.record.unsafe = true
	} else {
		v.record = nil
	}
}

func (v *Vim) CommandPending() bool {
	return v.pending != "" || v.surround != nil
}

func (v *Vim) Register() VimRegister { return v.register }

func (v *Vim) SelectRegister(register VimRegister) { v.register = register }

func (v *Vim) RegisterPending() bool { return v.pending == `"` || v.register != UnnamedRegister }

const maxVimCount = 9999

// A leading zero is a motion. Once a count has started, zero is a digit.
func appendVimCount(count *int, key string) bool {
	if len(key) != 1 || key[0] < '0' || key[0] > '9' || (key == "0" && *count == 0) {
		return false
	}
	*count = min(maxVimCount, *count*10+int(key[0]-'0'))
	return true
}

func (v *Vim) beginPending(key string, count int) {
	v.pending = key
	v.prefixCount = count
	v.count = 0
}

func (v *Vim) pendingCount() int {
	return min(maxVimCount, max(1, v.count)*max(1, v.prefixCount))
}

func (v *Vim) clearPending() {
	v.pending = ""
	v.count = 0
	v.prefixCount = 0
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
	v.clearPending()
	d.Anchor = -1
	d.BeginEditGroup()
	v.InsertMode = true
}

// Handle processes a key and records text changes for dot repeat.
// The result reports whether Vim consumed the key and text for the selected register.
func (v *Vim) Handle(d *Document, key string, register VimRegisterValue) (bool, VimRegisterValue) {
	if v.pending == `"` {
		switch key {
		case "+":
			v.count = v.prefixCount
			v.pending, v.prefixCount = "", 0
			v.SelectRegister(ClipboardRegister)
		case "esc":
			v.Reset(d)
		default:
			v.CancelPending()
		}
		return true, VimRegisterValue{}
	}
	if key == `"` && !v.InsertMode && !v.CommandPending() {
		v.beginPending(key, v.count)
		return true, VimRegisterValue{}
	}
	if v.register != UnnamedRegister {
		defer func() {
			if !v.CommandPending() && v.count == 0 {
				v.register = UnnamedRegister
			}
		}()
	}
	if d.ReadOnly && !v.readOnlyKey(key) {
		v.CancelPending()
		return true, VimRegisterValue{}
	}
	if !v.InsertMode && !v.CommandPending() && (key == "p" || key == "P") {
		v.Put(d, register, key == "p", v.TakePutCount())
		return true, VimRegisterValue{}
	}
	if key == "." && !v.InsertMode && !v.VisualMode && !v.CommandPending() {
		count := max(1, v.count)
		v.clearPending()
		v.record = nil
		return true, v.repeatChange(d, count)
	}
	if v.InsertMode {
		if v.record != nil && !repeatableInsertKey(key) {
			v.record.unsafe = true
		}
		handled, copied := v.handleKey(d, key)
		if !v.InsertMode {
			v.finishRecording(d)
		}
		return handled, copied
	}
	if v.record != nil && v.record.inserting {
		v.record = nil
	}
	if v.record == nil {
		v.record = &vimRecording{before: d.Text}
		switch key {
		case "S", "d", "c", "x", "tab", "shift+tab", ">", "<", "`":
			v.record.selection = v.repeatSelection(d)
		}
	}
	if (key == "J" || key == "g") && v.VisualMode && d.Anchor >= 0 && !v.CommandPending() {
		first, last := v.visualLineRange(d)
		v.record.selection = &vimSelection{linewise: true, size: last - first + 1}
	}
	v.record.keys = append(v.record.keys, key)
	handled, copied := v.handleKey(d, key)
	if v.InsertMode {
		v.record.beginInsert(d)
	} else if !v.CommandPending() && v.count == 0 {
		if key == "u" || key == "U" {
			v.record = nil
		} else {
			v.finishRecording(d)
		}
	}
	return handled, copied
}

// handleKey is also used for replay, which must leave the last change intact.
func (v *Vim) handleKey(d *Document, key string) (bool, VimRegisterValue) {
	if key == "esc" {
		if v.InsertMode {
			row, col := d.Position()
			if col > 0 {
				d.SetPosition(row, col-1)
			}
		}
		v.Reset(d)
		return true, VimRegisterValue{}
	}
	if v.InsertMode {
		return false, VimRegisterValue{}
	}
	if v.surround != nil {
		return v.handleSurround(d, key), VimRegisterValue{}
	}
	if !v.CommandPending() && v.handleCommand(d, key) {
		return true, VimRegisterValue{}
	}
	if v.VisualMode && d.Anchor < 0 {
		v.VisualMode, v.VisualLine = false, false
	}
	if strings.HasPrefix(key, "ctrl+") || strings.HasPrefix(key, "alt+") || strings.HasPrefix(key, "shift+") || strings.HasPrefix(key, "f") && len(key) > 1 {
		v.clearPending()
		return false, VimRegisterValue{}
	}
	if v.VisualMode {
		return v.handleVisual(d, key)
	}
	return v.handleNormal(d, key)
}

func (v *Vim) handleNormal(d *Document, key string) (bool, VimRegisterValue) {
	v.clamp(d)
	d.Anchor = -1
	if v.pending != "r" && appendVimCount(&v.count, key) {
		return true, VimRegisterValue{}
	}
	if v.pending != "" {
		copied := v.handleNormalPending(d, key)
		v.syncCursor(d)
		return true, copied
	}
	switch key {
	case "d", "c", "y", "g", "z", "r":
		v.beginPending(key, v.count)
		return true, VimRegisterValue{}
	}

	count := v.count
	v.clearPending()
	if v.motion(d, motionKey(key), count) {
		v.clamp(d)
		return true, VimRegisterValue{}
	}
	n := max(1, count)
	row, _ := d.Position()
	var copied VimRegisterValue
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
	case "x", "s":
		end, _ := characterRangeEnd(d, n)
		op := "d"
		if key == "s" {
			op = "c"
		}
		copied = v.operate(d, op, d.Cursor, end, false)
	case "D", "C":
		end := d.LineOffset(row) + d.LineRuneCount(row)
		op := "d"
		if key == "C" {
			op = "c"
		}
		copied = v.operate(d, op, d.Cursor, end, false)
	case "J":
		joinLines(d, row, row+max(2, n)-1)
	case "u":
		for range n {
			d.Undo()
		}
	case "U":
		for range n {
			d.Redo()
		}
	case "pgup", "pgdown":
		return false, VimRegisterValue{}
	}
	// Consume unmapped keys too, so they cannot insert text in Normal mode.
	v.syncCursor(d)
	return true, copied
}

func (v *Vim) handleNormalPending(d *Document, key string) VimRegisterValue {
	op, count := v.pending, v.pendingCount()
	hadCount := v.prefixCount > 0 || v.count > 0
	v.clearPending()
	switch op {
	case "r":
		replaceCharacter(d, key, count)
		return VimRegisterValue{}
	case "g":
		switch key {
		case "g":
			v.motion(d, "gg", count)
		case "c":
			if !hadCount {
				count = 0
			}
			v.beginPending("gc", count)
		}
		return VimRegisterValue{}
	case "gc":
		v.commentMotion(d, key, count, hadCount)
		return VimRegisterValue{}
	case "gcg", "gca", "gci":
		v.commentTarget(d, op, key, count)
		return VimRegisterValue{}
	case "z":
		switch key {
		case "a", "c", "o", "M", "R", "C", "O":
			v.foldAction = key
		}
		return VimRegisterValue{}
	}
	return v.handleOperator(d, op, key, count, hadCount)
}

func (v *Vim) syncCursor(d *Document) {
	if v.VisualMode {
		v.SyncVisual(d)
		return
	}
	if !v.InsertMode {
		v.clamp(d)
		d.Anchor = -1
	}
}
