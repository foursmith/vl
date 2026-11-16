package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/ourongxing/elegit/internal/theme"
)

type pickerLayout struct {
	x, y, width, start, count int
}

func (m *Model) pickerLayout(rows []action) pickerLayout {
	w := min(96, m.width-4)
	count := min(16, max(1, m.height-7))
	start := m.pickerList.Paginator.Page * m.pickerList.Paginator.PerPage
	return pickerLayout{(m.width - w) / 2, 1, w, start, min(count, max(0, len(rows)-start))}
}

func (m *Model) pickerOverlay(base []string) []string {
	rows := m.filteredActions()
	l := m.pickerLayout(rows)
	bg := m.colors.Popup
	inputBG := m.colors.Input
	w := l.width - 2
	border := theme.Style(m.colors.Accent, bg)
	box := []string{
		border.Render("╭" + strings.Repeat("─", w) + "╮"),
		border.Render("│") + row(" "+m.input.View(), w, m.colors.Foreground, inputBG) + border.Render("│"),
		border.Render("╰" + strings.Repeat("─", w) + "╯"),
	}
	if len(rows) > 0 {
		lines := strings.Split(m.pickerList.View(), "\n")
		box = append(box, lines[:l.count]...)
	}

	if len(rows) == 0 {
		message := "无匹配结果"
		if m.modal == "recent" && m.input.Value() == "" {
			message = "没有最近打开的文件夹"
		}
		box = append(box, row("  "+message, l.width, m.colors.Muted, bg))
	} else if reason := rows[min(m.pickerList.Index(), len(rows)-1)].disabled; reason != "" {
		box = append(box, row("  "+safe(reason), l.width, m.colors.Muted, bg))
	}
	return m.composeOverlay(base, box, l.x, l.y)
}

func (m *Model) clickPicker(msg tea.MouseClickMsg) tea.Cmd {
	m.syncPickerList()
	rows := m.filteredActions()
	l := m.pickerLayout(rows)
	if msg.Button == tea.MouseLeft && msg.X >= l.x && msg.X < l.x+l.width {
		if msg.Y >= l.y && msg.Y < l.y+3 {
			return nil
		}
		if index := msg.Y - l.y - 3; index >= 0 && index < l.count {
			m.selectPickerIndex(l.start + index)
			return m.selectPickerChoice()
		}
	}
	return m.updateModal(tea.KeyPressMsg{Code: tea.KeyEscape})
}

func (m *Model) isPicker() bool {
	return m.modal == "quick" || m.modal == "remotes" || m.modal == "refs" || m.modal == "recent"
}

func (m *Model) selectPickerChoice() tea.Cmd {
	rows := m.filteredActions()
	if len(rows) == 0 {
		return nil
	}
	m.syncPickerList()
	a := rows[min(m.pickerList.Index(), len(rows)-1)]
	if a.disabled != "" {
		m.notice = a.disabled
		return nil
	}
	if m.commandPicker() {
		m.lastCommand = a.id
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
	m.palette = nil
	cmd := m.prompt("quick", "", value)
	m.syncQuickPicker()
	m.syncPickerList()
	return cmd
}

func (m *Model) syncQuickPicker() {
	if m.modal != "quick" {
		return
	}
	title := "打开文件"
	if m.commandPicker() {
		title = "Commands"
	}
	if m.modalTitle == title && m.palette != nil {
		return
	}
	m.modalTitle = title
	m.pickerList.ResetSelected()
	if m.commandPicker() {
		m.input.Placeholder = ""
		m.palette = m.commandActions()
		return
	}
	m.input.Placeholder = "按名称搜索文件（追加 : 跳转行）"
	m.palette = make([]action, 0, len(m.snapshot.Files))
	for _, path := range m.snapshot.Files {
		m.palette = append(m.palette, action{id: path, label: path, run: func() tea.Cmd { return m.openFileInExplorer(path) }})
	}
}
