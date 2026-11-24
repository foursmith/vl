package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
	"github.com/foursmith/vl/internal/editor"
)

type clipboardApply func(*Model, string) tea.Cmd

type clipboardMsg struct {
	text  string
	err   error
	apply clipboardApply
}

func readClipboard(apply clipboardApply) tea.Cmd {
	return func() tea.Msg {
		text, err := clipboard.ReadAll()
		return clipboardMsg{text: text, err: err, apply: apply}
	}
}

func writeClipboard(text string, apply clipboardApply) tea.Cmd {
	if text == "" {
		return nil
	}
	return func() tea.Msg {
		return clipboardMsg{text: text, err: clipboard.WriteAll(text), apply: apply}
	}
}

func (m *Model) applyClipboard(msg clipboardMsg) tea.Cmd {
	if msg.err != nil {
		m.notice = m.tr(msg.err.Error())
		return nil
	}
	if msg.apply != nil && msg.text != "" {
		return msg.apply(m, msg.text)
	}
	return nil
}

func (i *textInput) clipboardTarget(cut bool) clipboardApply {
	revision := i.revision
	return func(m *Model, text string) tea.Cmd {
		field, update := m.focusedTextInput()
		if field != i || i.revision != revision {
			return nil
		}
		if cut {
			return update(inputCutMsg{})
		}
		return update(tea.PasteMsg{Content: text})
	}
}

func (m *Model) editableDocumentFocused(d *editor.Document) bool {
	return d != nil && !d.ReadOnly && m.document() == d &&
		m.modal == "" && m.contextMenu == nil && !m.sidebar && !m.fileFindFocused()
}

// Capture the selection before the clipboard operation starts. A delayed
// result cannot edit a different buffer or a selection the user has moved.
func documentClipboardTarget(d *editor.Document, cut bool) clipboardApply {
	value := d.Text
	start, end := d.Selection()
	return func(m *Model, text string) tea.Cmd {
		if !m.editableDocumentFocused(d) || d.Text != value {
			return nil
		}
		if currentStart, currentEnd := d.Selection(); currentStart != start || currentEnd != end {
			return nil
		}
		if cut {
			text = ""
		}
		m.insertClipboardText(text)
		return nil
	}
}

func (m *Model) copy(cut bool) tea.Cmd {
	d := m.document()
	if cut {
		if !m.editableDocumentFocused(d) {
			return nil
		}
		return writeClipboard(d.SelectedText(), documentClipboardTarget(d, true))
	}
	if d != nil {
		return writeClipboard(d.SelectedText(), nil)
	}
	if m.diffOpen {
		return writeClipboard(m.diffSelectedText(), nil)
	}
	return nil
}

func (m *Model) paste() tea.Cmd {
	d := m.document()
	if !m.editableDocumentFocused(d) {
		return nil
	}
	return readClipboard(documentClipboardTarget(d, false))
}

func (m *Model) insertClipboardText(text string) {
	d := m.document()
	v := &m.tabs[m.activeTab].vim
	if !v.InsertMode {
		v.CancelPending()
	}
	d.Insert(text)
	if v.VisualMode {
		v.Reset(d)
	}
	m.ensureEditor()
}
