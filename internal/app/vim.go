package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/foursmith/vl/internal/editor"
)

func (m *Model) textInserting() bool {
	return m.editing() && (m.vimDisabled || m.tabs[m.activeTab].vim.InsertMode)
}

func (m *Model) setVimDisabled(disabled bool) {
	if m.vimDisabled == disabled {
		return
	}
	m.vimDisabled = disabled
	m.cancelFlash()
	m.leaderTarget = ""
	m.pathCompletion = nil
	m.renderFrame = frameCache{}
	for i := range m.tabs {
		t := &m.tabs[i]
		d := t.doc
		if d == nil && t.diff != nil {
			d = t.diff.review
		}
		if d == nil {
			continue
		}
		if !disabled {
			t.vim.Reset(d)
			continue
		}
		d.EndEditGroup()
		if t.vim.VisualMode {
			start, end := d.Selection()
			if d.Cursor < d.Anchor {
				start, end = end, start
			}
			d.SelectRange(start, end)
		}
		t.vim = editor.Vim{}
	}
	if m.editorDocument() != nil {
		m.ensureTextCursor()
	}
}

func (m *Model) quickVimCommand(query string) action {
	var label string
	var run func() tea.Cmd
	switch strings.TrimSpace(query) {
	case ":w", ":w!":
		label, run = "Save", m.save
	case ":q":
		label, run = "Close Editor", func() tea.Cmd { return m.vimQuit(false) }
	case ":q!":
		label, run = "Discard changes and close", func() tea.Cmd { return m.vimQuit(true) }
	case ":wq", ":wq!":
		label, run = "Save and close", func() tea.Cmd { return m.saveEditor(true) }
	default:
		return action{}
	}
	return action{label: m.tr(label), run: run}
}

func (m *Model) vimQuit(discard bool) tea.Cmd {
	if m.activeTab < 0 || m.activeTab >= len(m.tabs) {
		return m.quit()
	}
	if !discard && m.tabs[m.activeTab].dirty() {
		m.notice = m.tr("Unsaved changes. Use :wq to save or :q! to discard.")
		return nil
	}
	m.removeTabAt(m.activeTab)
	return m.finishClose()
}

func (m *Model) vimKey(k string) (bool, tea.Cmd) {
	v := &m.tabs[m.activeTab].vim
	d := m.editorDocument()
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
			m.openFileFind()
			return true, nil
		}
		return true, m.findNext()
	}
	before, _ := d.Position()
	vertical := !v.InsertMode && !v.CommandPending() && (k == "j" || k == "k" || k == "up" || k == "down")
	v.HanDictionary = m.hanDictionary
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
	if action := v.TakeFoldAction(); action != "" && m.editing() {
		m.editorFold(action)
	}
	return true, nil
}

func (m *Model) vimClipboardWrite(key string) tea.Cmd {
	d := m.editorDocument()
	if d == nil {
		return nil
	}
	v := &m.tabs[m.activeTab].vim
	if !v.VisualMode {
		v.Handle(d, key, m.vimRegister)
	}
	_, text := v.Handle(d, key, m.vimRegister)
	m.ensureTextCursor()
	if text != "" {
		m.vimRegister = text
	}
	return writeClipboard(text, nil)
}

func (m *Model) vimClipboardPut(d *editor.Document, key string) tea.Cmd {
	if !m.editableDocumentFocused(d) {
		return nil
	}
	v := &m.tabs[m.activeTab].vim
	visual, linewise := v.VisualMode, v.VisualLine
	return readClipboard(guardDocumentClipboard(d, func(m *Model, text string) tea.Cmd {
		v := &m.tabs[m.activeTab].vim
		if m.vimDisabled || m.leaderActive() || m.flash != nil || v.InsertMode || v.CommandPending() ||
			v.VisualMode != visual || v.VisualLine != linewise {
			return nil
		}
		v.Handle(d, key, text)
		m.ensureTextCursor()
		return nil
	}))
}
