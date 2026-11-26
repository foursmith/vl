package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/foursmith/vl/internal/editor"
)

func (m *Model) leaderAvailable() bool {
	if m.sidebar || m.modal != "" || m.contextMenu != nil || m.activeTab < 0 || m.activeTab >= len(m.tabs) {
		return false
	}
	return m.editorDocument() == nil || !m.vimDisabled && m.tabs[m.activeTab].vim.AcceptsLeader()
}

func (m *Model) leaderActive() bool {
	return m.leaderTarget != "" && m.leaderAvailable() && m.leaderTarget == m.tabs[m.activeTab].clickKey()
}

func (m *Model) cancelLeader() {
	if m.vimRegisterPending() {
		m.tabs[m.activeTab].vim.CancelPending()
	}
	m.leaderTarget = ""
}

func (m *Model) leaderKey(k string) (bool, tea.Cmd) {
	target := m.leaderTarget
	m.leaderTarget = ""
	if !m.leaderAvailable() {
		return false, nil
	}
	prefix := m.leaderPrefix
	if !prefix.Enabled() {
		prefix = defaultLeaderPrefix
	}
	if target != "" && target == m.tabs[m.activeTab].clickKey() {
		if handled, cmd := m.runLeaderShortcut(k); handled {
			return true, cmd
		}
		if matchesShortcut(k, dismissKey) && m.editorDocument() != nil {
			m.tabs[m.activeTab].vim.Reset(m.editorDocument())
			m.ensureTextCursor()
		}
		// Unknown letters cancel without editing or operating on hunks. Modified
		// keys, focus switching and command palettes keep their workbench meaning.
		return matchesShortcut(k, dismissKey) || matchesShortcut(k, prefix) || len(k) == 1, nil
	}
	if !matchesShortcut(k, prefix) {
		return false, nil
	}
	if m.editorDocument() != nil {
		m.tabs[m.activeTab].vim.CancelPending()
	}
	m.navigationState = navigationState{}
	m.leaderTarget = m.tabs[m.activeTab].clickKey()
	return true, nil
}

func (m *Model) runLeaderShortcut(k string) (bool, tea.Cmd) {
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
	case "d", "c", "y", "p", "P", "x":
		v := &m.tabs[m.activeTab].vim
		if k == "x" && !v.VisualMode {
			return false, nil
		}
		v.SelectRegister(editor.ClipboardRegister)
		return m.vimKey(k)
	}
	return false, nil
}

func (m *Model) selectShortcutTab(index int) {
	if index >= 0 && index < len(m.tabs) {
		m.activateTab(index)
	}
}
