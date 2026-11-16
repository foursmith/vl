package app

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/elegit/internal/theme"
)

type dialogLayout struct {
	x, y, width, top, maxTop, page int
	content                        []string
}

var closeActions = [...]struct{ label, choice string }{
	{" Save ", "save"},
	{" Discard ", "discard"},
	{" Cancel ", "cancel"},
}

// Rendering and button hit testing share the same wrapped, height-bounded layout.
func (m *Model) dialogLayout() dialogLayout {
	w := max(6, min(76, m.width-4))
	l := dialogLayout{width: w, x: (m.width - w) / 2}
	paint := func(text, fg string) string {
		return row(" "+text, w-2, fg, m.colors.Popup)
	}
	wrap := func(text, fg string) []string {
		var lines []string
		for _, line := range strings.Split(ansi.Hardwrap(safe(text), w-4, true), "\n") {
			lines = append(lines, paint(line, fg))
		}
		return lines
	}
	if m.modal != "confirm" && m.modal != "close" {
		l.content = []string{
			paint(safe(m.modalTitle), m.colors.Foreground),
			row(" "+m.input.View(), w-2, m.colors.Foreground, m.colors.Input),
		}
	} else {
		message := wrap(m.modalTitle, m.colors.Foreground)
		var footer []string
		if m.modal == "close" {
			message = append(message, wrap("Unsaved changes will be lost.", m.colors.Muted)...)
			var buttons strings.Builder
			for i, action := range closeActions {
				fg, bg := m.colors.Foreground, m.colors.Popup
				switch i {
				case 0:
					fg, bg = "#FFFFFF", m.colors.Accent
				case 1:
					bg = m.colors.Border
				}
				buttons.WriteString(theme.Style(fg, bg).Render(action.label))
			}
			footer = []string{row(buttons.String(), w-2, m.colors.Foreground, m.colors.Popup)}
		} else {
			if m.confirmationDetail != "" {
				message = append(message, wrap(m.confirmationDetail, m.colors.Muted)...)
			}
			footer = wrap("Enter: confirm · Esc: cancel", m.colors.Muted)
		}
		// Keep actions visible even when a path needs more than one screen.
		l.page = max(1, m.height-3-len(footer))
		if len(message) > l.page {
			l.page = max(1, l.page-1)
			footer = append([]string{paint("Up/Down: scroll", m.colors.Muted)}, footer...)
		}
		l.maxTop = max(0, len(message)-l.page)
		l.top = min(max(0, m.dialogTop), l.maxTop)
		l.content = append(l.content, message[l.top:min(len(message), l.top+l.page)]...)
		l.content = append(l.content, footer...)
	}
	l.y = min(3, max(0, m.height-len(l.content)-3))
	return l
}
