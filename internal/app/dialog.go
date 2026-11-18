package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/vl/internal/theme"
)

type dialogLayout struct {
	x, y, width, top, maxTop, page int
	content                        []string
	buttons                        []dialogButton
}

type dialogButton struct{ x, y, width int }

type confirmationAction struct{ label, choice string }

func (m *Model) confirmationActions() []confirmationAction {
	if m.modal == "close" {
		return []confirmationAction{{m.tr("Save"), "save"}, {m.tr("Don't Save"), "discard"}, {m.tr("Cancel"), "cancel"}}
	}
	return []confirmationAction{{m.confirmationLabel, "confirm"}, {m.tr("Cancel"), "cancel"}}
}

func (m *Model) confirmationDialog() bool { return m.modal == "confirm" || m.modal == "close" }

func (m *Model) gitMessageDialog() bool { return m.modal == "commit" || m.modal == "stash" }

func (m *Model) gitMessageDialogWidth() int { return max(6, min(58, m.width-4)) }

func (m *Model) chooseConfirmation() tea.Cmd {
	actions := m.confirmationActions()
	choice := actions[min(max(0, m.confirmationButton), len(actions)-1)].choice
	if m.modal == "close" {
		return m.chooseClose(choice)
	}
	run := m.confirm
	m.confirm = nil
	m.modal = ""
	m.input.Blur()
	if choice == "confirm" && run != nil {
		return run()
	}
	return nil
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
	paint := func(text, fg string) string { return row(" "+text, w-2, fg, m.colors.Popup) }
	x, y := (m.width-w)/2, min(3, max(0, m.height-5))
	content := []string{
		paint(safe(m.modalTitle), m.colors.Foreground),
		row(" "+m.inputView(m.input, x+2, y+2), w-2, m.colors.InputForeground, m.colors.Popup),
	}
	return dialogLayout{width: w, x: x, y: y, content: content}
}

func (m *Model) gitMessageLayout() dialogLayout {
	w := m.gitMessageDialogWidth()
	l := dialogLayout{width: w, x: (m.width - w) / 2, y: max(0, (m.height-8)/2)}
	popup, field := m.colors.Popup, m.colors.Input
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
		return row(strings.Repeat(" ", padding)+text, w-2, theme.Readable(fg, m.colors.Popup), m.colors.Popup)
	}
	var message []string
	detail := m.confirmationDetail
	if m.modal == "close" {
		detail = m.tr("Unsaved changes will be lost.")
	}
	for i, text := range []string{m.modalTitle, detail} {
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
	blank := row("", w-2, m.colors.Foreground, m.colors.Popup)
	for range padding {
		l.content = append(l.content, blank)
	}
	l.content = append(l.content, message[l.top:min(len(message), l.top+l.page)]...)
	if l.maxTop > 0 {
		l.content = append(l.content, center(m.tr("↑/↓ Scroll"), m.colors.Muted))
	}
	l.content = append(l.content, blank)
	actions := m.confirmationActions()
	gap := 3
	buttonWidth := max(1, min(12, (w-2-gap*(len(actions)-1))/len(actions)))
	start := max(0, (w-2-len(actions)*buttonWidth-gap*(len(actions)-1))/2)
	text := strings.Repeat(" ", start)
	for i, action := range actions {
		label := action.label
		fg, bg := m.colors.InputForeground, m.colors.Popup
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
	l.content = append(l.content, row(text, w-2, m.colors.Foreground, m.colors.Popup))
	for range padding {
		l.content = append(l.content, blank)
	}
	l.y = max(0, (m.height-len(l.content)-2)/2)
	return l
}
