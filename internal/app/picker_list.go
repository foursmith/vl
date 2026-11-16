package app

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/elegit/internal/fileicon"
	"github.com/ourongxing/elegit/internal/theme"
)

func (a action) FilterValue() string { return a.label }

type pickerDelegate struct{ model *Model }

func (pickerDelegate) Height() int                         { return 1 }
func (pickerDelegate) Spacing() int                        { return 0 }
func (pickerDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }
func (d pickerDelegate) Render(out io.Writer, l list.Model, index int, entry list.Item) {
	m, a := d.model, entry.(action)
	fg, bg := m.colors.Foreground, m.colors.Popup
	selected := index == l.Index()
	if selected {
		fg, bg = m.colors.SelectionForeground, m.colors.Selection
	}
	if a.disabled != "" {
		fg = m.colors.Muted
	}
	w := l.Width() - 2
	label := " " + safe(a.label)
	if m.modal == "quick" && !m.commandPicker() && a.id != "" {
		icon := fileicon.For(a.id, m.nerd)
		label = ansi.Truncate(" "+row(icon.Glyph, 2, icon.Color, bg)+safe(filepath.Base(a.id)), w-2, "…")
		if dir := filepath.Dir(a.id); dir != "." {
			if remaining := w - 2 - ansi.StringWidth(label); remaining > 1 {
				label += theme.Style(m.colors.Muted, bg).Render(ansi.Truncate(" "+safe(dir), remaining, "…"))
			}
		}
	}
	if a.key != "" {
		hint := " " + a.key + " "
		label = ansi.Truncate(label, max(1, w-ansi.StringWidth(hint)-2), "…")
		label += strings.Repeat(" ", max(1, w-ansi.StringWidth(label)-ansi.StringWidth(hint)-1)) + theme.Style(m.colors.Muted, bg).Render(hint)
	}
	marker := " "
	if m.monochrome && selected {
		marker = ">"
	}
	fmt.Fprint(out, row(marker+row(label, w, fg, bg)+" ", l.Width(), fg, bg))
}

func newPickerList(m *Model) list.Model {
	l := list.New(nil, pickerDelegate{m}, min(96, m.width-4), min(16, max(1, m.height-7)))
	l.SetShowTitle(false)
	l.SetShowFilter(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	// The shared input also handles command/file modes and line:column syntax.
	l.SetFilteringEnabled(false)
	l.DisableQuitKeybindings()
	l.KeyMap.CursorUp, l.KeyMap.CursorDown = pickerKeys.Up, pickerKeys.Down
	l.KeyMap.PrevPage, l.KeyMap.NextPage = pickerKeys.PageUp, pickerKeys.PageDown
	l.KeyMap.GoToStart, l.KeyMap.GoToEnd = pickerKeys.First, pickerKeys.Last
	return l
}

// Synchronize in event handlers, never during rendering. Filtering is disabled
// in the list, so SetItems completes synchronously and produces no command.
func (m *Model) syncPickerList() {
	if !m.isPicker() {
		return
	}
	rows := m.filteredActions()
	items := make([]list.Item, len(rows))
	for i, a := range rows {
		items[i] = a
	}
	selected := m.pickerList.Index()
	m.pickerList.SetSize(min(96, m.width-4), min(16, max(1, m.height-7)))
	m.pickerList.SetItems(items)
	m.pickerList.Select(min(selected, max(0, len(rows)-1)))
}
func (m *Model) selectPickerIndex(index int) {
	m.syncPickerList()
	m.pickerList.Select(min(max(0, index), max(0, len(m.pickerList.Items())-1)))
}
func (m *Model) pickerListKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	if !key.Matches(msg, pickerKeys.Up, pickerKeys.Down, pickerKeys.PageUp, pickerKeys.PageDown, pickerKeys.First, pickerKeys.Last) {
		return nil, false
	}
	updated, cmd := m.pickerList.Update(msg)
	m.pickerList = updated
	return cmd, true
}
