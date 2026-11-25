package app

import (
	"slices"

	tea "charm.land/bubbletea/v2"
)

type commandID string

const (
	commandQuit      commandID = "app.quit"
	commandCopy      commandID = "edit.copy"
	commandCut       commandID = "edit.cut"
	commandPaste     commandID = "edit.paste"
	commandSelectAll commandID = "edit.selectAll"

	commandDeleteToLineStart commandID = "edit.deleteToLineStart"
	commandDeleteToLineEnd   commandID = "edit.deleteToLineEnd"

	commandUndo         commandID = "edit.undo"
	commandRedo         commandID = "edit.redo"
	commandFiles        commandID = "workspace.quickOpen"
	commandRecent       commandID = "workspace.openRecent"
	commandPalette      commandID = "workbench.commandPalette"
	commandSearch       commandID = "view.search"
	commandExplorer     commandID = "view.explorer"
	commandChanges      commandID = "view.changes"
	commandHistory      commandID = "view.history"
	commandPullRequests commandID = "view.pullRequests"
	commandSidebar      commandID = "view.sidebar"
	commandFocus        commandID = "view.toggleFocus"
	commandRefresh      commandID = "workbench.refresh"
	commandDiffLayout   commandID = "view.diff"
	commandSave         commandID = "file.save"
	commandClose        commandID = "file.close"
	commandCloseOthers  commandID = "file.closeOthers"
	commandCloseAll     commandID = "file.closeAll"
	commandFind         commandID = "edit.find"
	commandContextMenu  commandID = "workbench.contextMenu"
	commandPreviousTab  commandID = "view.previousEditor"
	commandNextTab      commandID = "view.nextEditor"
	commandSaveAndClose commandID = "file.saveAndClose"
)

// A workbench command can be invoked on the current selection or on an
// explicit target supplied by a context menu. Tab identities survive reorder.
func (m *Model) executeCommand(id commandID, tabKey string) tea.Cmd {
	if isTextEditCommand(id) {
		return m.executeTextEdit(id)
	}
	switch id {
	case commandQuit:
		return m.quit()
	case commandFiles:
		return m.openQuickFiles()
	case commandRecent:
		return m.openRecentWorkspaces()
	case commandPalette:
		return m.openPalette()
	case commandSearch:
		return m.openSearch()
	case commandExplorer:
		return m.selectSection(explorer)
	case commandChanges:
		return m.selectSection(changes)
	case commandHistory:
		return m.selectSection(history)
	case commandPullRequests:
		return m.selectSection(pullRequests)
	case commandSidebar:
		return m.toggleSidebar()
	case commandFocus:
		m.toggleEditorFocus()
	case commandRefresh:
		return m.refreshView()
	case commandDiffLayout:
		m.split = !m.split
		m.renderDiff()
	case commandSave:
		if m.editing() {
			if v := &m.tabs[m.activeTab].vim; !v.InsertMode {
				v.CancelPending()
			}
			return m.save()
		}
	case commandClose:
		if index, ok := m.commandTabIndex(tabKey); ok {
			return m.closeTabAt(index)
		}
	case commandCloseOthers:
		if index, ok := m.commandTabIndex(tabKey); ok {
			return m.closeOtherTabsExcept(index)
		}
	case commandCloseAll:
		return m.closeAllTabs()
	case commandFind:
		if m.sidebar && m.section == search {
			m.focusSearch(0)
		} else if m.fileFindFocused() {
			m.focusFileFind(0)
		} else {
			m.openFileFind()
		}
	case commandContextMenu:
		return m.openFocusedContextMenu()
	case commandPreviousTab:
		m.selectShortcutTab(m.activeTab - 1)
	case commandNextTab:
		m.selectShortcutTab(m.activeTab + 1)
	case commandSaveAndClose:
		if m.editing() {
			return m.saveEditor(true)
		}
	}
	return nil
}

func (m *Model) commandTabIndex(target string) (int, bool) {
	if target == "" {
		return m.activeTab, true
	}
	for i, tab := range m.tabs {
		if tab.clickKey() == target {
			return i, true
		}
	}
	m.notice = m.tr("Target changed; reopen the menu")
	return -1, false
}

func (m *Model) commandAction(id commandID, label, disabled, tabKey string) action {
	return action{string(id), m.tr(label), m.commandShortcutHint(id), disabled, func() tea.Cmd {
		return m.executeCommand(id, tabKey)
	}}
}

func (m *Model) commandShortcutHint(id commandID) string {
	for _, rule := range slices.Backward(m.activeWorkbenchKeymap().byCommand[id]) {
		if m.resolveWorkbenchShortcut(rule.binding.Keys()[0]).ruleID() == rule.ruleID() {
			return rule.binding.Help().Key
		}
	}
	return ""
}
