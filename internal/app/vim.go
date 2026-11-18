package app

import tea "charm.land/bubbletea/v2"

func (m *Model) vimKey(k string) (bool, tea.Cmd) {
	v := &m.tabs[m.activeTab].vim
	d := m.editorDocument()
	if d.ReadOnly && !v.CommandPending() && !v.VisualMode {
		if k == "[" {
			m.nextHunk(-1)
			return true, nil
		}
		if k == "]" {
			m.nextHunk(1)
			return true, nil
		}
	}
	if !v.InsertMode && !v.CommandPending() && (k == "pgup" || k == "pgdown") {
		rows := m.contentHeight()
		if k == "pgup" {
			rows = -rows
		}
		before, _ := d.Position()
		v.MovePage(d, rows)
		m.moveAcrossFolds(before)
		m.ensureTextCursor()
		return true, nil
	}
	if k == "f" {
		if operator, ok := v.FlashOperator(); ok {
			return true, m.startFlash(operator)
		}
	}
	if !v.InsertMode && !v.VisualMode && (k == ":" || k == "/" || k == "n") && !v.CommandPending() {
		v.CancelPending()
		if k == ":" {
			return true, m.openQuickPicker(":")
		}
		if k == "/" {
			return true, m.openFileFind()
		}
		return true, m.findNext()
	}
	before, _ := d.Position()
	vertical := !v.InsertMode && !v.CommandPending() && (k == "j" || k == "k" || k == "up" || k == "down")
	handled, text := v.Handle(d, k, m.vimRegister)
	if !handled {
		return false, nil
	}
	if vertical {
		m.moveAcrossFolds(before)
	}
	m.ensureTextCursor()
	if text != "" {
		m.vimRegister = text
	}
	action := v.TakeAction()
	switch action {
	case "fold:a", "fold:c", "fold:o", "fold:M", "fold:R", "fold:C", "fold:O":
		if m.editing() {
			m.editorFold(action[len("fold:"):])
		}
	case "focus":
		if m.editorOnly {
			return true, nil
		}
		m.sidebar = true
		m.sidebarVisible = true
		m.resize()
	}
	return true, nil
}
