package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/ourongxing/vl/internal/theme"
)

type pickerLayout struct {
	x, y, width, start, count int
}

func (m *Model) pickerHeight() int { return min(16, max(1, m.height-7)) }

func (l pickerLayout) indexAt(x, y int) int {
	i := y - l.y - 3
	if x < l.x || x >= l.x+l.width || i < 0 || i >= l.count {
		return -1
	}
	return l.start + i
}

func (m *Model) hoveredPickerIndex() int {
	if !m.isPicker() {
		return -1
	}
	return m.pickerLayout(m.filteredActions()).indexAt(m.hoverX, m.hoverY)
}

func (m *Model) pickerLayout(rows []action) pickerLayout {
	w := min(96, m.width-4)
	return pickerLayout{(m.width - w) / 2, 1, w, m.pickerTop, min(m.pickerHeight(), max(0, len(rows)-m.pickerTop))}
}

func (m *Model) pickerOverlay(base []string) []string {
	rows := m.filteredActions()
	l := m.pickerLayout(rows)
	bg := m.colors.Popup
	w := l.width - 2
	border := theme.Style(m.colors.PopupBorder, bg)
	box := []string{
		border.Render("╭" + strings.Repeat("─", w) + "╮"),
		border.Render("│") + row(" "+m.inputView(m.input, l.x+2, l.y+1), w, m.colors.InputForeground, bg) + border.Render("│"),
		border.Render("├" + strings.Repeat("─", w) + "┤"),
	}
	hovered := l.indexAt(m.hoverX, m.hoverY)
	for index := l.start; index < l.start+l.count; index++ {
		box = append(box, border.Render("│")+m.pickerRow(rows[index], index, l.width-2, hovered)+border.Render("│"))
	}

	if len(rows) == 0 {
		message := m.tr("No matches")
		if m.modal == "recent" && m.input.Value() == "" {
			message = m.tr("No recent workspaces")
		}
		box = append(box, border.Render("│")+row("  "+message, w, theme.Readable(m.colors.Muted, bg), bg)+border.Render("│"))
	}
	box = append(box, border.Render("╰"+strings.Repeat("─", w)+"╯"))
	return m.composeOverlay(base, box, l.x, l.y)
}

func (m *Model) clickPicker(msg tea.MouseClickMsg) tea.Cmd {
	rows := m.filteredActions()
	l := m.pickerLayout(rows)
	if msg.Button == tea.MouseRight && m.modal == "recent" {
		if index := l.indexAt(msg.X, msg.Y); index >= 0 {
			m.selectPickerIndex(index)
			folder := rows[index].id
			m.contextMenu = &contextMenu{x: msg.X, y: msg.Y, modal: m.modal, actions: []action{
				{label: m.tr("Remove from workspace list"), run: func() tea.Cmd { return m.removeWorkspace(folder) }},
			}}
		}
		return nil
	}
	if msg.Button == tea.MouseLeft && msg.X >= l.x && msg.X < l.x+l.width {
		if msg.Y >= l.y && msg.Y < l.y+3 {
			return nil
		}
		if index := l.indexAt(msg.X, msg.Y); index >= 0 {
			if m.pickerHeading(rows[index]) {
				return nil
			}
			m.selectPickerIndex(index)
			return m.selectPickerChoice()
		}
	}
	return m.updateModal(tea.KeyPressMsg{Code: tea.KeyEscape})
}

func (m *Model) isPicker() bool {
	switch m.modal {
	case "searchOptions", "quick", "remotes", "refs", "recent", "themes", "languages", "statusBarStyles":
		return true
	default:
		return false
	}
}

func (m *Model) selectPickerChoice() tea.Cmd {
	m.syncPicker()
	rows := m.filteredActions()
	if len(rows) == 0 {
		return nil
	}
	a := rows[m.pickerIndex]
	if a.run == nil {
		return nil
	}
	if a.disabled != "" {
		m.notice = a.disabled
		return nil
	}
	if m.commandPicker() {
		m.lastCommand = a.id
	}
	if m.modal == "themes" {
		if a.id == themeImportID {
			return a.run()
		}
		m.themeBeforePreview = nil
	}
	m.modal = ""
	m.input.Blur()
	return a.run()
}

func (m *Model) openQuickFiles() tea.Cmd { return m.openQuickPicker("") }

func (m *Model) commandPicker() bool {
	return m.modal == "quick" && strings.HasPrefix(m.input.Value(), ">")
}

func (m *Model) openQuickPicker(value string) tea.Cmd {
	restore := m.cancelThemePreview()
	m.palette = nil
	cmd := m.prompt("quick", "", value)
	m.syncQuickPicker()
	m.syncPicker()
	return tea.Batch(restore, cmd)
}

func (m *Model) syncQuickPicker() {
	if m.modal != "quick" {
		return
	}
	title := m.tr("Open File")
	if m.commandPicker() {
		title = m.tr("Commands")
	}
	if m.modalTitle == title && m.palette != nil {
		return
	}
	m.modalTitle = title
	m.pickerIndex = 0
	if m.commandPicker() {
		m.input.Placeholder = ""
		m.palette = m.commandActions()
		return
	}
	m.input.Placeholder = m.tr("Search files by name (append : to go to line)")
	m.palette = make([]action, 0, len(m.snapshot.Files))
	for _, path := range m.snapshot.Files {
		m.palette = append(m.palette, action{id: path, label: path, run: func() tea.Cmd { return m.openFileInExplorer(path) }})
	}
}
