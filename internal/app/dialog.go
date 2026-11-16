package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/elegit/internal/theme"
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
		return []confirmationAction{{"保存", "save"}, {"不保存", "discard"}, {"取消", "cancel"}}
	}
	return []confirmationAction{{m.confirmationLabel, "confirm"}, {"取消", "cancel"}}
}

func (m *Model) confirmationDialog() bool { return m.modal == "confirm" || m.modal == "close" }

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
	w := max(6, min(76, m.width-4))
	paint := func(text, fg string) string { return row(" "+text, w-2, fg, m.colors.Popup) }
	content := []string{
		paint(safe(m.modalTitle), m.colors.Foreground),
		row(" "+m.input.View(), w-2, m.colors.Foreground, m.colors.Input),
	}
	return dialogLayout{width: w, x: (m.width - w) / 2, y: min(3, max(0, m.height-len(content)-3)), content: content}
}

func (m *Model) confirmationLayout() dialogLayout {
	w := max(6, min(76, m.width-4))
	l := dialogLayout{width: w, x: (m.width - w) / 2}
	center := func(text, fg string) string {
		text = ansi.Truncate(text, w-4, "…")
		padding := max(0, (w-2-ansi.StringWidth(text))/2)
		return row(strings.Repeat(" ", padding)+text, w-2, fg, m.colors.Popup)
	}
	var message []string
	detail := m.confirmationDetail
	if m.modal == "close" {
		detail = "Unsaved changes will be lost."
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
		l.content = append(l.content, center("↑/↓ 滚动", m.colors.Muted))
	}
	l.content = append(l.content, blank)
	actions := m.confirmationActions()
	gap := 3
	buttonWidth := max(1, min(12, (w-2-gap*(len(actions)-1))/len(actions)))
	start := max(0, (w-2-len(actions)*buttonWidth-gap*(len(actions)-1))/2)
	text := strings.Repeat(" ", start)
	for i, action := range actions {
		label := action.label
		fg, bg := m.colors.Foreground, m.colors.Border
		selected := i == m.confirmationButton
		if selected {
			fg, bg = m.colors.SelectionForeground, m.colors.Selection
		}
		if selected && m.monochrome {
			label = "> " + label
		}
		label = ansi.Truncate(label, buttonWidth, "")
		left := max(0, (buttonWidth-ansi.StringWidth(label))/2)
		label = strings.Repeat(" ", left) + label
		label += strings.Repeat(" ", max(0, buttonWidth-ansi.StringWidth(label)))
		text += theme.Style(fg, bg).Bold(selected).Underline(selected).Render(label)
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
