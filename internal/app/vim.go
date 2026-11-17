package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
	"github.com/ourongxing/elegit/internal/editor"
)

func (m *Model) toggleVim() tea.Cmd {
	m.vimEnabled = !m.vimEnabled
	for i := range m.tabs {
		t := &m.tabs[i]
		if t.doc != nil {
			t.vim.Reset(t.doc)
		}
	}
	m.ensureEditor()
	return m.savePreferences()
}

func (m *Model) vimKey(k string) (bool, tea.Cmd) {
	v := &m.tabs[m.activeTab].vim
	d := m.document()
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
			return true, m.prompt("find", m.tr("Find in current view"), m.find)
		}
		return true, m.findNext()
	}
	if !v.Handle(d, k) {
		return false, nil
	}
	m.ensureEditor()
	if text, ready := v.TakeClipboardCopy(); ready {
		return true, func() tea.Msg { return clipboardMsg{err: clipboard.WriteAll(text)} }
	}
	switch v.TakeAction() {
	case "clearSearch":
		m.findHidden = true
		d.ClearSelection()
		v.Reset(d)
	case "save":
		return true, m.save()
	case "close":
		return true, m.closeTab()
	case "saveClose":
		return true, m.saveEditor(true)
	case "focus":
		if m.editorOnly {
			return true, nil
		}
		m.sidebar = true
		m.sidebarVisible = true
		m.resize()
	case "clipboardPaste":
		return true, m.readVimClipboard(true)
	case "clipboardPasteBefore":
		return true, m.readVimClipboard(false)
	}
	return true, nil
}

type vimClipboardMsg struct {
	doc              *editor.Document
	source, text     string
	cursor, anchor   int
	visual, linewise bool
	after            bool
	err              error
}

func (m *Model) readVimClipboard(after bool) tea.Cmd {
	d := m.document()
	v := &m.tabs[m.activeTab].vim
	snapshot := vimClipboardMsg{after: after, doc: d, source: d.Text, cursor: d.Cursor, anchor: d.Anchor, visual: v.VisualMode, linewise: v.VisualLine}
	return func() tea.Msg {
		snapshot.text, snapshot.err = clipboard.ReadAll()
		return snapshot
	}
}
func (m *Model) applyVimClipboard(msg vimClipboardMsg) tea.Cmd {
	if msg.err != nil {
		m.log(msg.err.Error())
		return nil
	}
	if !m.vimEnabled || m.sidebar || m.modal != "" || !m.editing() || m.document() != msg.doc {
		return nil
	}
	d := m.document()
	v := &m.tabs[m.activeTab].vim
	if d.Text != msg.source || d.Cursor != msg.cursor || d.Anchor != msg.anchor || v.InsertMode || v.VisualMode != msg.visual || v.VisualLine != msg.linewise {
		return nil
	}
	v.PasteClipboard(d, msg.text, msg.after)
	m.ensureEditor()
	return nil
}
