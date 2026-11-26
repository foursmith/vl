package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/foursmith/vl/internal/editor"
)

type textEditAction struct{ command commandID }

func isTextEditCommand(id commandID) bool {
	switch id {
	case commandCopy, commandCut, commandPaste, commandSelectAll, commandDeleteToLineStart, commandDeleteToLineEnd, commandUndo, commandRedo, commandToggleComment:
		return true
	default:
		return false
	}
}

func (m *Model) textEditFocused() bool {
	return m.focusedInput() != nil || m.modal == "" && !m.sidebar && !m.fileFindFocused() && m.editorDocument() != nil
}

func (m *Model) commentFocused() bool {
	d := m.document()
	return d != nil && !d.ReadOnly && m.modal == "" && !m.sidebar && !m.fileFindFocused()
}

func (m *Model) executeTextEdit(command commandID) tea.Cmd {
	if _, update := m.focusedTextInput(); update != nil {
		m.navigationState = navigationState{}
		return update(textEditAction{command})
	}
	if m.sidebar || m.modal != "" || m.fileFindFocused() {
		return nil
	}
	d := m.editorDocument()
	if d == nil && (command != commandCopy || !m.diffOpen) {
		return nil
	}
	m.cancelFlash()
	m.stopTextDrag()
	m.cancelLeader()
	m.pathCompletion = nil
	m.navigationState = navigationState{}
	if command == commandCopy {
		if d != nil && !m.tabs[m.activeTab].vim.InsertMode {
			m.tabs[m.activeTab].vim.CancelPending()
		}
		return m.copy(false)
	}

	v := &m.tabs[m.activeTab].vim
	v.CancelPending()
	if command == commandSelectAll {
		v.SelectAll(d)
		m.ensureTextCursor()
		return nil
	}
	if d.ReadOnly {
		return nil
	}
	switch command {
	case commandCut:
		return m.copy(true)
	case commandPaste:
		return m.paste()
	case commandDeleteToLineStart, commandDeleteToLineEnd:
		if d.DeleteToLineBoundary(command == commandDeleteToLineStart) && !m.vimDisabled && !v.InsertMode {
			v.Reset(d)
		}
	case commandUndo:
		d.Undo()
	case commandRedo:
		d.Redo()
	case commandToggleComment:
		d.EndEditGroup()
		d.ToggleComments()
		if v.VisualMode {
			v.Reset(d)
		} else {
			d.ClearSelection()
		}
	}
	if v.VisualMode {
		v.SyncVisual(d)
	}
	m.ensureTextCursor()
	return nil
}

func (m *Model) textEditorActions() []action {
	d := m.editorDocument()
	if m.sidebar || d == nil && !m.diffOpen {
		return nil
	}
	var selected string
	if m.editing() {
		selected = d.SelectedText()
	} else {
		selected = m.diffSelectedText()
	}
	reason := ""
	if selected == "" {
		reason = m.tr("Select text first")
	}
	actions := []action{m.commandAction(commandCopy, "Copy", reason, "")}
	if d == nil {
		return actions
	}
	selectAll := m.commandAction(commandSelectAll, "Select All", "", "")
	if d.ReadOnly {
		return append(actions, selectAll)
	}
	return append(actions,
		m.commandAction(commandCut, "Cut", reason, ""),
		m.commandAction(commandPaste, "Paste", "", ""),
		selectAll,
		m.commandAction(commandDeleteToLineStart, "Delete to Line Start", "", ""),
		m.commandAction(commandDeleteToLineEnd, "Delete to Line End", "", ""),
		m.commandAction(commandUndo, "Undo", "", ""),
		m.commandAction(commandRedo, "Redo", "", ""),
		m.commandAction(commandToggleComment, "Toggle Comment", "", ""),
	)
}

func (m *Model) focusedInput() *textInput {
	switch {
	case m.modal != "":
		if m.input.Focused() {
			return &m.input
		}
	case m.fileFindFocused():
		return m.findWidget.field()
	case m.sidebar && m.section == search:
		if field := m.searchField(m.search.focus); field != nil && field.Focused() {
			return field
		}
	}
	return nil
}

func (m *Model) focusedTextInput() (*textInput, func(tea.Msg) tea.Cmd) {
	field := m.focusedInput()
	switch {
	case field == nil:
		return nil, nil
	case field == &m.input:
		return field, m.updateDialogInput
	case m.fileFindFocused():
		return field, m.fileFindInput
	default:
		return field, m.searchInput
	}
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

func editBufferKey(buffer *editor.Buffer, key string, dictionary func() *editor.HanDictionary) bool {
	switch key {
	case "backspace", "delete":
		buffer.Delete(key == "backspace")
	default:
		move := strings.ReplaceAll(key, "shift+", "")
		switch move {
		case leftArrow, rightArrow, "home", "end", "ctrl+left", "ctrl+right":
			buffer.MoveWithDictionary(move, move != key, dictionary)
		default:
			return false
		}
	}
	return true
}

func (m *Model) editDocument(d *editor.Document, msg tea.KeyPressMsg, k string, updatePath bool) tea.Cmd {
	vim := &m.tabs[m.activeTab].vim
	if vim.InsertMode {
		d.BeginEditGroup()
	}
	switch {
	case m.vimDisabled && matchesShortcut(k, dismissKey):
		d.ClearSelection()
	case editBufferKey(&d.Buffer, k, m.hanDictionary):
	case matchesShortcut(k, pickerKeys.Accept):
		if !continueMarkdownLine(d) {
			d.Insert("\n")
		}
	case k == "tab":
		d.InsertIndent()
	case k == "shift+tab":
		d.Outdent()
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
		case "up", "down", "ctrl+home", "ctrl+end":
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
