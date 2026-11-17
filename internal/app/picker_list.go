package app

import (
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/elegit/internal/fileicon"
	"github.com/ourongxing/elegit/internal/theme"
)

func (m *Model) pickerRow(a action, index, width, hovered int) string {
	fg, bg := m.colors.PopupForeground, m.colors.Popup
	selected := index == m.pickerIndex
	if selected {
		fg, bg = m.colors.PopupSelectionForeground, m.colors.PopupSelection
	} else if index == hovered {
		bg = m.colors.Hover
	}
	if a.disabled != "" {
		fg = m.colors.Brand
	}
	opacity := .7
	if m.colorTheme.IsLight() {
		opacity = .95
	}
	secondary := theme.WithOpacity(fg, opacity)
	if selected {
		secondary = fg
	}
	w := width - 2
	label := " " + safe(a.label)
	if m.modal == "quick" && !m.commandPicker() && a.id != "" {
		icon := fileicon.For(a.id, m.nerd)
		label = ansi.Truncate(" "+row(icon.Glyph, 2, icon.Color, bg)+safe(filepath.Base(a.id)), w-2, "…")
		if dir := filepath.Dir(a.id); dir != "." {
			if remaining := w - 2 - ansi.StringWidth(label); remaining > 1 {
				label += theme.Style(secondary, bg).Render(ansi.Truncate(" "+safe(dir), remaining, "…"))
			}
		}
	}
	if a.key != "" {
		hint := " " + a.key + " "
		label = ansi.Truncate(label, max(1, w-ansi.StringWidth(hint)-2), "…")
		label += strings.Repeat(" ", max(1, w-ansi.StringWidth(label)-ansi.StringWidth(hint)-1)) + theme.Style(secondary, bg).Render(hint)
	}
	marker := " "
	if m.monochrome && selected {
		marker = ">"
	}
	return row(marker+row(label, w, fg, bg)+" ", width, fg, bg)
}

// Synchronize selection and scroll position in event handlers, never rendering.
func (m *Model) syncPicker() {
	if !m.isPicker() {
		return
	}
	count := len(m.filteredActions())
	height := m.pickerHeight()
	m.pickerIndex = min(max(0, m.pickerIndex), max(0, count-1))
	m.pickerTop = min(m.pickerTop, m.pickerIndex)
	m.pickerTop = max(m.pickerTop, m.pickerIndex-height+1)
	m.pickerTop = min(max(0, m.pickerTop), max(0, count-height))
}

func (m *Model) selectPickerIndex(index int) {
	m.pickerIndex = index
	m.syncPicker()
}

func (m *Model) scrollPicker(msg tea.MouseWheelMsg) {
	rows := m.filteredActions()
	if m.pickerLayout(rows).indexAt(msg.X, msg.Y) < 0 {
		return
	}
	delta := 1
	switch msg.Button {
	case tea.MouseWheelUp:
		delta = -1
	case tea.MouseWheelDown:
	default:
		return
	}
	m.pickerTop = min(max(0, m.pickerTop+delta), max(0, len(rows)-m.pickerHeight()))
}

func (m *Model) pickerKey(msg tea.KeyPressMsg) bool {
	index := m.pickerIndex
	switch {
	case key.Matches(msg, pickerKeys.Up):
		index--
	case key.Matches(msg, pickerKeys.Down):
		index++
	case key.Matches(msg, pickerKeys.PageUp):
		index -= m.pickerHeight()
	case key.Matches(msg, pickerKeys.PageDown):
		index += m.pickerHeight()
	case key.Matches(msg, pickerKeys.First):
		index = 0
	case key.Matches(msg, pickerKeys.Last):
		index = len(m.filteredActions()) - 1
	default:
		return false
	}
	m.selectPickerIndex(index)
	return true
}
