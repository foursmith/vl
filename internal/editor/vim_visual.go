package editor

func (v *Vim) startVisual(d *Document, linewise bool) {
	v.clear()
	v.VisualMode, v.VisualLine = true, linewise
	v.visualAnchor = d.Cursor
	v.SyncVisual(d)
}

func (v *Vim) SyncVisual(d *Document) {
	v.clamp(d)
	d.SelectVisual(v.visualAnchor, d.Cursor, v.VisualLine)
}

func (v *Vim) handleVisual(d *Document, key string) bool {
	if len(key) == 1 && key[0] >= '0' && key[0] <= '9' && (key != "0" || v.count > 0) {
		v.count = min(9999, v.count*10+int(key[0]-'0'))
		return true
	}
	n, hadCount := max(1, v.count), v.count > 0
	if v.pending == "i" || v.pending == "a" {
		inner := v.pending == "i"
		n = min(9999, n*max(1, v.operatorCount))
		v.clear()
		if start, end, ok := textObject(d, key, inner, n); ok {
			if end <= start {
				v.Reset(d)
				d.Cursor = start
				return true
			}
			v.VisualLine = false
			v.visualAnchor = start
			d.Cursor = d.previous(end)
		}
		v.SyncVisual(d)
		return true
	}
	if key == "i" || key == "a" {
		v.pending = key
		v.operatorCount = n
		v.count = 0
		return true
	}
	if v.pending == "g" {
		n = min(9999, n*max(1, v.operatorCount))
		v.clear()
		if key == "g" {
			d.SetPosition(n-1, 0)
		}
		v.SyncVisual(d)
		return true
	}
	if key == "g" {
		v.pending = "g"
		v.operatorCount = n
		v.count = 0
		return true
	}
	v.clear()

	switch key {
	case "S":
		v.beginVisualSurround(d)
		return true
	case "v", "V":
		linewise := key == "V"
		if v.VisualLine == linewise {
			v.Reset(d)
		} else {
			v.VisualLine = linewise
			v.SyncVisual(d)
		}
		return true
	case "o":
		d.Cursor, v.visualAnchor = v.visualAnchor, d.Cursor
		v.SyncVisual(d)
		return true
	case "y", "d", "x", "c":
		start, end := d.Selection()
		linewise := v.VisualLine
		if key == "x" {
			key = "d"
		}
		v.VisualMode, v.VisualLine = false, false
		d.ClearSelection()
		v.operate(d, key, start, end, linewise)
		v.clampUnlessInsert(d)
		return true
	case "left":
		key = "h"
	case "right":
		key = "l"
	case "up":
		key = "k"
	case "down":
		key = "j"
	case "home":
		key = "0"
	case "end":
		key = "L"
	case "pgup", "pgdown":
		return false
	}
	if key == "G" && !hadCount {
		n = len(d.Lines())
	}
	v.motion(d, key, n, false)
	v.SyncVisual(d)
	return true
}
