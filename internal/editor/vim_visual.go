package editor

func (v *Vim) startVisual(d *Document, linewise bool) {
	v.clearPending()
	v.VisualMode, v.VisualLine = true, linewise
	v.visualAnchor = d.Cursor
	v.SyncVisual(d)
}

func (v *Vim) SyncVisual(d *Document) {
	v.clamp(d)
	d.SelectVisual(v.visualAnchor, d.Cursor, v.VisualLine)
}

// SelectAll keeps Visual's anchor in sync with the workbench selection.
func (v *Vim) SelectAll(d *Document) {
	d.SelectAll()
	if v.VisualMode {
		v.visualAnchor = d.Anchor
		v.SyncVisual(d)
	}
}

func (v *Vim) visualLineRange(d *Document) (first, last int) {
	anchor, _ := d.positionAt(v.visualAnchor)
	row, _ := d.Position()
	return min(anchor, row), max(anchor, row)
}

func (v *Vim) handleVisual(d *Document, key string) (bool, VimRegisterValue) {
	if appendVimCount(&v.count, key) {
		return true, VimRegisterValue{}
	}
	if v.pending == "i" || v.pending == "a" {
		v.handleVisualPending(d, key)
		return true, VimRegisterValue{}
	}
	count := v.count
	if key == "i" || key == "a" {
		v.beginPending(key, count)
		return true, VimRegisterValue{}
	}
	if v.pending == "g" {
		v.handleVisualPending(d, key)
		return true, VimRegisterValue{}
	}
	if key == "g" {
		v.beginPending(key, count)
		return true, VimRegisterValue{}
	}
	v.clearPending()

	switch key {
	case "S":
		v.beginVisualSurround(d)
		return true, VimRegisterValue{}
	case "v", "V":
		linewise := key == "V"
		if v.VisualLine == linewise {
			v.Reset(d)
		} else {
			v.VisualLine = linewise
			v.SyncVisual(d)
		}
		return true, VimRegisterValue{}
	case "o":
		d.Cursor, v.visualAnchor = v.visualAnchor, d.Cursor
		v.SyncVisual(d)
		return true, VimRegisterValue{}
	case "J":
		first, last := v.visualLineRange(d)
		v.VisualMode, v.VisualLine = false, false
		d.ClearSelection()
		joinLines(d, first, max(first+1, last))
		v.clamp(d)
		return true, VimRegisterValue{}
	case "y", "d", "x", "c":
		start, end := d.Selection()
		linewise := v.VisualLine
		if key == "x" {
			key = "d"
		}
		v.VisualMode, v.VisualLine = false, false
		d.ClearSelection()
		copied := v.operate(d, key, start, end, linewise)
		v.syncCursor(d)
		return true, copied
	case "pgup", "pgdown":
		return false, VimRegisterValue{}
	}
	v.motion(d, motionKey(key), count)
	v.SyncVisual(d)
	return true, VimRegisterValue{}
}

func (v *Vim) handleVisualPending(d *Document, key string) {
	pending, count := v.pending, v.pendingCount()
	v.clearPending()
	switch pending {
	case "g":
		if key == "g" {
			v.motion(d, "gg", count)
		} else if key == "c" {
			d.ToggleComments()
			v.VisualMode, v.VisualLine = false, false
			v.syncCursor(d)
			return
		}
	case "i", "a":
		if start, end, ok := v.textObject(d, key, pending == "i", count); ok {
			if end <= start {
				v.Reset(d)
				d.Cursor = start
				return
			}
			v.VisualLine = false
			v.visualAnchor = start
			d.Cursor = d.previous(end)
		}
	}
	v.SyncVisual(d)
}
