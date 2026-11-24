package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/foursmith/vl/internal/editor"
)

type textEditAction uint8

const (
	noTextEdit textEditAction = iota
	textSelectAll
	textCopy
	textCut
	textPaste
	textUndo
	textRedo
)

func textEditShortcut(k string) textEditAction {
	switch {
	case matchesShortcut(k, editorKeys.SelectAll):
		return textSelectAll
	case matchesShortcut(k, copyTextKey):
		return textCopy
	case matchesShortcut(k, editorKeys.Cut):
		return textCut
	case matchesShortcut(k, editorKeys.Paste):
		return textPaste
	case matchesShortcut(k, editorKeys.Undo):
		return textUndo
	case matchesShortcut(k, editorKeys.Redo):
		return textRedo
	}
	return noTextEdit
}

// Workbench editing commands run before the focused editor's Vim commands.
func (m *Model) textEditingShortcut(k string, route workbenchShortcut) (bool, tea.Cmd) {
	action := textEditShortcut(k)
	if action != noTextEdit {
		if _, update := m.focusedTextInput(); update != nil {
			m.navigationState = navigationState{}
			return true, update(action)
		}
	}

	if route.command == commandCopy {
		action = textCopy
	} else if action == textCopy {
		// Copy in the editor follows the configured workbench binding.
		return false, nil
	}
	if action == noTextEdit || m.sidebar || m.modal != "" || m.fileFindFocused() || route.has(beforeInput) {
		return false, nil
	}
	d := m.editorDocument()
	if d == nil && action != textCopy {
		return false, nil
	}
	m.cancelFlash()
	m.stopTextDrag()
	m.leaderTarget = ""
	m.pathCompletion = nil
	m.navigationState = navigationState{}
	if action == textCopy {
		if d != nil && !m.tabs[m.activeTab].vim.InsertMode {
			m.tabs[m.activeTab].vim.CancelPending()
		}
		return true, m.executeCommand(commandCopy, "")
	}

	v := &m.tabs[m.activeTab].vim
	v.CancelPending()
	if action == textSelectAll {
		v.SelectAll(d)
		m.ensureTextCursor()
		return true, nil
	}
	if d.ReadOnly {
		return true, nil
	}
	switch action {
	case textCut:
		return true, m.copy(true)
	case textPaste:
		return true, m.paste()
	case textUndo:
		d.Undo()
	case textRedo:
		d.Redo()
	}
	if v.VisualMode {
		v.SyncVisual(d)
	}
	m.ensureTextCursor()
	return true, nil
}

func (m *Model) focusedTextInput() (*textInput, func(tea.Msg) tea.Cmd) {
	switch {
	case m.modal != "":
		if m.input.Focused() {
			return &m.input, m.updateDialogInput
		}
	case m.fileFindFocused():
		if field := m.searchField(m.search.focus); field != nil && field.Focused() {
			return field, m.fileFindInput
		}
	case m.sidebar && m.section == search:
		if field := m.searchField(m.search.focus); field != nil && field.Focused() {
			return field, m.searchInput
		}
	}
	return nil, nil
}

func (m *Model) pasteText(msg tea.PasteMsg) tea.Cmd {
	m.pathCompletion = nil
	if _, update := m.focusedTextInput(); update != nil {
		return update(msg)
	}
	if m.flash != nil {
		return m.flashText(msg.Content)
	}
	if m.editableDocumentFocused(m.document()) && msg.Content != "" {
		m.insertClipboardText(msg.Content)
	}
	return nil
}

func (m *Model) editDocument(d *editor.Document, msg tea.KeyPressMsg, k string, updatePath bool) tea.Cmd {
	vim := &m.tabs[m.activeTab].vim
	if vim.InsertMode {
		d.BeginEditGroup()
	}
	switch {
	case m.vimDisabled && matchesShortcut(k, dismissKey):
		d.ClearSelection()
	case matchesShortcut(k, eraseCharacter):
		d.Delete(true)
	case k == "delete":
		d.Delete(false)
	case matchesShortcut(k, pickerKeys.Accept):
		d.Insert("\n")
	case k == "tab":
		d.Insert("\t")
	case k == "pgup", k == "pgdown":
		dir := "up"
		if k == "pgdown" {
			dir = "down"
		}
		for i := 0; i < m.contentHeight(); i++ {
			d.Move(dir, false)
		}
	default:
		base := strings.ReplaceAll(k, "shift+", "")
		switch base {
		case "left", "right", "up", "down", "home", "end", "ctrl+home", "ctrl+end", "ctrl+left", "ctrl+right":
			d.Move(base, base != k)
		default:
			if msg.Text != "" {
				d.Insert(msg.Text)
			}
		}
	}
	if vim.VisualMode {
		vim.SyncVisual(d)
	}
	m.ensureTextCursor()
	if updatePath {
		return m.refreshPathCompletion()
	}
	return nil
}
