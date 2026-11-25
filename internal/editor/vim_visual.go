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

func (v *Vim) handleVisual(d *Document, key string) (bool, string) {
	if appendVimCount(&v.count, key) {
		return true, ""
	}
	if v.pending == "i" || v.pending == "a" {
		v.handleVisualPending(d, key)
		return true, ""
	}
	n, hadCount := max(1, v.count), v.count > 0
	if key == "i" || key == "a" {
		v.beginPending(key, n)
		return true, ""
	}
	if v.pending == "g" {
		v.handleVisualPending(d, key)
		return true, ""
	}
	if key == "g" {
		v.beginPending(key, n)
		return true, ""
	}
	v.clearPending()

	switch key {
	case "S":
		v.beginVisualSurround(d)
		return true, ""
	case "v", "V":
		linewise := key == "V"
		if v.VisualLine == linewise {
			v.Reset(d)
		} else {
			v.VisualLine = linewise
			v.SyncVisual(d)
		}
		return true, ""
	case "o":
		d.Cursor, v.visualAnchor = v.visualAnchor, d.Cursor
		v.SyncVisual(d)
		return true, ""
	case "J":
		first, last := v.visualLineRange(d)
		v.VisualMode, v.VisualLine = false, false
		d.ClearSelection()
		joinLines(d, first, max(first+1, last))
		v.clamp(d)
		return true, ""
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
		return false, ""
	}
	if key == "G" && !hadCount {
		n = len(d.Lines())
	}
	v.motion(d, motionKey(key), n)
	v.SyncVisual(d)
	return true, ""
}

func (v *Vim) handleVisualPending(d *Document, key string) {
	pending, count := v.pending, v.pendingCount()
	v.clearPending()
	switch pending {
	case "g":
		if key == "g" {
			d.SetPosition(count-1, 0)
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
