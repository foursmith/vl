package app

import tea "charm.land/bubbletea/v2"

func (m *Model) leaderAvailable() bool {
	if m.sidebar || m.modal != "" || m.contextMenu != nil || m.activeTab < 0 || m.activeTab >= len(m.tabs) {
		return false
	}
	return m.editorDocument() == nil || !m.vimDisabled && m.tabs[m.activeTab].vim.AcceptsLeader()
}

func (m *Model) leaderActive() bool {
	return m.leaderTarget != "" && m.leaderAvailable() && m.leaderTarget == m.tabs[m.activeTab].clickKey()
}

func (m *Model) leaderKey(k string) (bool, tea.Cmd) {
	target, operator := m.leaderTarget, m.leaderOperator
	m.leaderTarget, m.leaderOperator = "", ""
	if !m.leaderAvailable() {
		return false, nil
	}
	if target != "" && target == m.tabs[m.activeTab].clickKey() {
		if handled, cmd := m.runLeaderShortcut(k, operator); handled {
			return true, cmd
		}
		if matchesShortcut(k, dismissKey) && m.editorDocument() != nil {
			m.tabs[m.activeTab].vim.Reset(m.editorDocument())
			m.ensureTextCursor()
		}
		// Unknown letters cancel without editing or operating on hunks. Modified
		// keys, focus switching and command palettes keep their workbench meaning.
		return matchesShortcut(k, dismissKey) || matchesShortcut(k, leaderKeys.Prefix) || len(k) == 1, nil
	}
	if !matchesShortcut(k, leaderKeys.Prefix) {
		return false, nil
	}
	if m.editorDocument() != nil {
		m.tabs[m.activeTab].vim.CancelPending()
	}
	m.navigationState = navigationState{}
	m.leaderTarget = m.tabs[m.activeTab].clickKey()
	return true, nil
}

func (m *Model) runLeaderShortcut(k, operator string) (bool, tea.Cmd) {
	if operator != "" {
		if k != operator {
			return false, nil
		}
		return true, m.vimClipboardWrite(k)
	}
	switch {
	case len(k) == 1 && k[0] >= '1' && k[0] <= '9':
		m.selectShortcutTab(int(k[0] - '1'))
		return true, nil
	case matchesShortcut(k, leaderKeys.Previous):
		return true, m.executeCommand(commandPreviousTab, "")
	case matchesShortcut(k, leaderKeys.Next):
		return true, m.executeCommand(commandNextTab, "")
	case matchesShortcut(k, leaderKeys.Save):
		return true, m.executeCommand(commandSave, "")
	case matchesShortcut(k, leaderKeys.Close):
		return true, m.executeCommand(commandClose, "")
	case matchesShortcut(k, leaderKeys.SaveClose):
		return true, m.executeCommand(commandSaveAndClose, "")
	}
	d := m.editorDocument()
	if d == nil {
		return false, nil
	}
	switch k {
	case "p", "P":
		return true, m.vimClipboardPut(d, k)
	case "d", "c", "y", "x":
		if m.tabs[m.activeTab].vim.VisualMode {
			return true, m.vimClipboardWrite(k)
		}
		if k != "x" {
			m.leaderTarget, m.leaderOperator = m.tabs[m.activeTab].clickKey(), k
			return true, nil
		}
	}
	return false, nil
}

func (m *Model) selectShortcutTab(index int) {
	if index >= 0 && index < len(m.tabs) {
		m.activateTab(index)
	}
}
