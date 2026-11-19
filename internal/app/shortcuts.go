package app

import tea "charm.land/bubbletea/v2"

type shortcutAction uint8

const (
	shortcutSave shortcutAction = iota
	shortcutClose
	shortcutSaveClose
	shortcutSelectTab
)

type shortcut struct {
	action shortcutAction
	tab    int
}

// One mapping for all content tabs. Text input and sidebar actions own Space
// in their respective contexts; they never enter this command sequence.
func leaderShortcut(k string, activeTab int) (shortcut, bool) {
	if len(k) == 1 && k[0] >= '1' && k[0] <= '9' {
		return shortcut{action: shortcutSelectTab, tab: int(k[0] - '1')}, true
	}
	switch k {
	case "left":
		return shortcut{action: shortcutSelectTab, tab: activeTab - 1}, true
	case "right":
		return shortcut{action: shortcutSelectTab, tab: activeTab + 1}, true
	case "w":
		return shortcut{action: shortcutSave}, true
	case "q":
		return shortcut{action: shortcutClose}, true
	case "z":
		return shortcut{action: shortcutSaveClose}, true
	}
	return shortcut{}, false
}

func (m *Model) leaderAvailable() bool {
	if m.sidebar || m.modal != "" || m.contextMenu != nil || m.activeTab < 0 || m.activeTab >= len(m.tabs) {
		return false
	}
	return m.editorDocument() == nil || m.tabs[m.activeTab].vim.AcceptsLeader()
}

func (m *Model) leaderActive() bool {
	return m.leaderTarget != "" && m.leaderAvailable() && m.leaderTarget == m.tabs[m.activeTab].clickKey()
}

func (m *Model) leaderKey(k string) (bool, tea.Cmd) {
	pending := m.leaderActive()
	m.leaderTarget = ""
	if !m.leaderAvailable() {
		return false, nil
	}
	if pending {
		if command, ok := leaderShortcut(k, m.activeTab); ok {
			return true, m.runShortcut(command)
		}
		if k == "esc" && m.editorDocument() != nil {
			m.tabs[m.activeTab].vim.Reset(m.editorDocument())
			m.ensureTextCursor()
		}
		// Unknown letters cancel without editing or operating on hunks. Modified
		// keys, focus switching and command palettes keep their workbench meaning.
		return k == "esc" || k == "space" || len(k) == 1, nil
	}
	if k != "space" && k != " " {
		return false, nil
	}
	if m.editorDocument() != nil {
		m.tabs[m.activeTab].vim.CancelPending()
	}
	m.navigationState = navigationState{}
	m.leaderTarget = m.tabs[m.activeTab].clickKey()
	return true, nil
}

func (m *Model) runShortcut(command shortcut) tea.Cmd {
	switch command.action {
	case shortcutSelectTab:
		if command.tab >= 0 && command.tab < len(m.tabs) {
			m.activateTab(command.tab)
		}
	case shortcutClose:
		return m.closeTab()
	case shortcutSave:
		if m.editing() {
			return m.save()
		}
	case shortcutSaveClose:
		if m.editing() {
			return m.saveEditor(true)
		}
	}
	return nil
}
