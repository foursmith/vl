package app

import (
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/editor"
	"github.com/foursmith/vl/internal/theme"
)

type dialogLayout struct {
	x, y, width, top, maxTop, page int
	content                        []string
	buttons                        []dialogButton
}

type dialogButton struct{ x, y, width int }

type confirmationAction struct {
	label    string
	shortcut key.Binding
	run      func() tea.Cmd
}

func (m *Model) confirmationDialog() bool {
	return m.modal == "confirm" && len(m.confirmationActions) > 0
}

func (m *Model) showDialog(kind, title string) {
	if kind != "githubPublish" {
		m.githubPublish = nil
	}
	m.leaderTarget = ""
	m.resizingSidebar = false
	m.modal, m.modalTitle = kind, title
	m.dialogTop = 0
}

func (m *Model) openConfirmation(title, detail string, cancel func() tea.Cmd, actions ...confirmationAction) {
	m.showDialog("confirm", title)
	m.confirmationDetail = detail
	m.confirmationButton = 0
	m.confirmationActions = append(actions, confirmationAction{label: m.tr("Cancel"), shortcut: dismissKey, run: cancel})
	m.input.Blur()
}

func (m *Model) dismissDialog() {
	m.confirmationActions = nil
	m.githubPublish = nil
	m.modal = ""
	m.input.Blur()
}

func (m *Model) ask(title, label string, run func() tea.Cmd) tea.Cmd {
	cancel := func() tea.Cmd {
		m.dismissDialog()
		return nil
	}
	m.openConfirmation(title, "", cancel, confirmationAction{label: label, run: func() tea.Cmd {
		m.dismissDialog()
		return run()
	}})
	return nil
}

func (m *Model) askSaveChanges(title string, choose func(string) tea.Cmd) {
	cancel := func() tea.Cmd { return choose("cancel") }
	m.openConfirmation(title, m.tr("Unsaved changes will be lost."), cancel,
		confirmationAction{label: m.tr("Save"), shortcut: dialogKeys.Save, run: func() tea.Cmd { return choose("save") }},
		confirmationAction{label: m.tr("Don't Save"), shortcut: dialogKeys.Discard, run: func() tea.Cmd { return choose("discard") }},
	)
}

func (m *Model) saveDialogDocument(d *editor.Document) bool {
	if m.busy {
		m.confirmationDetail = m.tr("Wait for the current Git operation before saving")
		m.dialogTop = 0
		return false
	}
	if err := m.saveDocument(d); err != nil {
		m.confirmationDetail = m.tr("Failed to save %s: %s", filepath.Base(d.Path), m.tr(err.Error()))
		m.dialogTop = 0
		return false
	}
	return true
}

func (m *Model) confirmLeavingWorkspace(title string, leave func() tea.Cmd) tea.Cmd {
	m.askSaveChanges(title, func(choice string) tea.Cmd {
		if choice == "cancel" {
			m.dismissDialog()
			return nil
		}
		if m.busy {
			m.confirmationDetail = m.tr("Wait for the current operation to finish")
			m.dialogTop = 0
			return nil
		}
		if choice == "save" {
			for _, t := range m.tabs {
				if t.dirty() && !m.saveDialogDocument(t.doc) {
					return nil
				}
			}
		}
		m.dismissDialog()
		return leave()
	})
	return nil
}

func (m *Model) gitMessageDialog() bool { return m.modal == "commit" || m.modal == "stash" }

func (m *Model) gitMessageDialogWidth() int { return max(6, min(58, m.width-4)) }

func (m *Model) activateConfirmation(index int) tea.Cmd {
	m.confirmationButton = index
	return m.confirmationActions[index].run()
}

func (m *Model) updateConfirmation(k string) tea.Cmd {
	count := len(m.confirmationActions)
	switch {
	case matchesShortcut(k, navigationKeys.Right), matchesShortcut(k, searchKeys.NextField):
		m.confirmationButton = (m.confirmationButton + 1) % count
		return nil
	case matchesShortcut(k, navigationKeys.Left), matchesShortcut(k, searchKeys.PreviousField):
		m.confirmationButton = (m.confirmationButton + count - 1) % count
		return nil
	case matchesShortcut(k, dialogKeys.Accept):
		return m.activateConfirmation(m.confirmationButton)
	}
	for i, action := range m.confirmationActions {
		if matchesShortcut(k, action.shortcut) {
			return m.activateConfirmation(i)
		}
	}

	l := m.confirmationLayout()
	top := l.top
	delta := pageDelta(k, l.page)
	switch {
	case matchesShortcut(k, navigationKeys.Up):
		top--
	case matchesShortcut(k, navigationKeys.Down):
		top++
	case delta != 0:
		top += delta
	case matchesShortcut(k, pickerKeys.First):
		top = 0
	case matchesShortcut(k, pickerKeys.Last):
		top = l.maxTop
	}
	m.dialogTop = min(max(0, top), l.maxTop)
	return nil
}

func (m *Model) clickModal(msg tea.MouseClickMsg) tea.Cmd {
	if m.isPicker() {
		return m.clickPicker(msg)
	}
	l := m.dialogLayout()
	if msg.Button == tea.MouseLeft {
		for i, button := range l.buttons {
			x, y := l.x+button.x, l.y+button.y
			if msg.Y != y || msg.X < x || msg.X >= x+button.width {
				continue
			}
			if m.confirmationDialog() {
				return m.activateConfirmation(i)
			}
			if i == 1 {
				return m.updateModal(tea.KeyPressMsg{Code: tea.KeyEnter})
			}
			return m.updateModal(tea.KeyPressMsg{Code: tea.KeyEscape})
		}
	}
	if msg.X >= l.x && msg.X < l.x+l.width && msg.Y >= l.y && msg.Y < l.y+len(l.content)+2 {
		return nil
	}
	return m.updateModal(tea.KeyPressMsg{Code: tea.KeyEscape})
}

// Rendering and button hit testing share the same wrapped, height-bounded layout.
func (m *Model) dialogLayout() dialogLayout {
	if m.confirmationDialog() {
		return m.confirmationLayout()
	}
	if m.gitMessageDialog() {
		return m.gitMessageLayout()
	}
	w := max(6, min(76, m.width-4))
	paint := func(text, fg string) string { return row(" "+text, w-2, fg, m.colors.Panel) }
	x, y := (m.width-w)/2, min(3, max(0, m.height-5))
	content := []string{
		paint(safe(m.modalTitle), m.colors.Foreground),
		row(" "+m.inputView(m.input, x+2, y+2), w-2, m.colors.InputForeground, m.colors.Panel),
	}
	return dialogLayout{width: w, x: x, y: y, content: content}
}

func (m *Model) gitMessageLayout() dialogLayout {
	w := m.gitMessageDialogWidth()
	l := dialogLayout{width: w, x: (m.width - w) / 2, y: max(0, (m.height-8)/2)}
	popup, field := m.colors.Panel, m.colors.Input
	inner := w - 2
	paint := func(text, fg string) string { return row(text, inner, fg, popup) }

	target, submit := m.stashRepository, m.tr("Stash")
	disabled := false
	if m.modal == "commit" {
		target, submit = m.commitRepository, m.tr("Commit")
		disabled = strings.TrimSpace(m.input.Value()) == ""
	}
	destination := m.repositoryName(target)
	if branch := m.repositoryBranch(target); branch != "" {
		destination += " [" + branch + "]"
	}
	title := theme.TextStyle(m.colors.PopupForeground, popup).Bold(true).Render(safe(m.modalTitle))
	subtitle := ansi.Truncate(safe(destination), max(0, inner-4), "…")
	blank := paint("", m.colors.PopupForeground)
	input := m.input
	styles := input.Styles()
	styles.Focused.Text = theme.TextStyle(m.colors.InputForeground, field)
	styles.Focused.Placeholder = theme.TextStyle(m.colors.InputPlaceholder, field)
	input.SetStyles(styles)
	message := row(" "+m.inputView(input, l.x+4, l.y+4), inner-4, m.colors.InputForeground, field)
	l.content = []string{
		paint("  "+title, m.colors.PopupForeground),
		paint("  "+subtitle, theme.Readable(m.colors.Muted, popup)),
		blank,
		paint("  "+message, m.colors.PopupForeground),
		blank,
	}
	cancelLabel, submitLabel := " "+m.tr("Cancel")+" Esc ", " "+submit+" ↵ "
	cancelWidth, submitWidth := ansi.StringWidth(cancelLabel), ansi.StringWidth(submitLabel)
	start := max(0, inner-cancelWidth-submitWidth-3)
	fg, bg := m.colors.PopupSelectionForeground, m.colors.PopupSelection
	if disabled {
		fg, bg = theme.Readable(m.colors.Muted, popup), popup
	}
	button := theme.TextStyle(fg, bg).Bold(!disabled).Underline(m.monochrome && !disabled).Render(submitLabel)
	l.content = append(l.content, paint(strings.Repeat(" ", start)+cancelLabel+" "+button, m.colors.PopupForeground))
	l.buttons = []dialogButton{
		{x: 1 + start, y: 6, width: cancelWidth},
		{x: 2 + start + cancelWidth, y: 6, width: submitWidth},
	}
	return l
}

func (m *Model) confirmationLayout() dialogLayout {
	w := max(6, min(76, m.width-4))
	l := dialogLayout{width: w, x: (m.width - w) / 2}
	center := func(text, fg string) string {
		text = ansi.Truncate(text, w-4, "…")
		padding := max(0, (w-2-ansi.StringWidth(text))/2)
		return row(strings.Repeat(" ", padding)+text, w-2, theme.Readable(fg, m.colors.Panel), m.colors.Panel)
	}
	var message []string
	for i, text := range []string{m.modalTitle, m.confirmationDetail} {
		if text == "" {
			continue
		}
		fg := m.colors.Foreground
		if i == 1 {
			fg = m.colors.Muted
		}
		for _, line := range strings.Split(ansi.Hardwrap(safe(text), max(1, w-6), true), "\n") {
			message = append(message, center(line, fg))
		}
	}
	// Reserve space for the buttons; long targets remain scrollable.
	padding := 1
	if m.height < 10 {
		padding = 0
	}
	l.page = max(1, m.height-4-2*padding)
	if len(message) > l.page {
		l.page = max(1, l.page-1)
	}
	l.maxTop = max(0, len(message)-l.page)
	l.top = min(max(0, m.dialogTop), l.maxTop)
	blank := row("", w-2, m.colors.Foreground, m.colors.Panel)
	for range padding {
		l.content = append(l.content, blank)
	}
	l.content = append(l.content, message[l.top:min(len(message), l.top+l.page)]...)
	if l.maxTop > 0 {
		l.content = append(l.content, center(m.tr("↑/↓ Scroll"), m.colors.Muted))
	}
	l.content = append(l.content, blank)
	actions := m.confirmationActions
	gap := 3
	buttonWidth := max(1, min(12, (w-2-gap*(len(actions)-1))/len(actions)))
	start := max(0, (w-2-len(actions)*buttonWidth-gap*(len(actions)-1))/2)
	text := strings.Repeat(" ", start)
	for i, action := range actions {
		label := action.label
		fg, bg := m.colors.InputForeground, m.colors.Panel
		selected := i == m.confirmationButton
		if selected {
			fg, bg = m.colors.PopupSelectionForeground, m.colors.PopupSelection
		}
		if selected && m.monochrome {
			label = "> " + label
		}
		label = ansi.Truncate(label, buttonWidth, "")
		left := max(0, (buttonWidth-ansi.StringWidth(label))/2)
		label = strings.Repeat(" ", left) + label
		label += strings.Repeat(" ", max(0, buttonWidth-ansi.StringWidth(label)))
		text += theme.TextStyle(fg, bg).Bold(selected).Underline(selected).Render(label)
		l.buttons = append(l.buttons, dialogButton{x: 1 + start + i*(buttonWidth+gap), y: 1 + len(l.content), width: buttonWidth})
		if i < len(actions)-1 {
			text += strings.Repeat(" ", gap)
		}
	}
	l.content = append(l.content, row(text, w-2, m.colors.Foreground, m.colors.Panel))
	for range padding {
		l.content = append(l.content, blank)
	}
	l.y = max(0, (m.height-len(l.content)-2)/2)
	return l
}
