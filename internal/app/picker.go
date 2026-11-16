package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/ourongxing/elegit/internal/theme"
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
	inputBG := m.colors.Input
	w := l.width - 2
	border := theme.Style(m.colors.Accent, bg)
	box := []string{
		border.Render("╭" + strings.Repeat("─", w) + "╮"),
		border.Render("│") + row(" "+m.input.View(), w, m.colors.Foreground, inputBG) + border.Render("│"),
		border.Render("╰" + strings.Repeat("─", w) + "╯"),
	}
	hovered := l.indexAt(m.hoverX, m.hoverY)
	for index := l.start; index < l.start+l.count; index++ {
		box = append(box, m.pickerRow(rows[index], index, l.width, hovered))
	}

	if len(rows) == 0 {
		message := "无匹配结果"
		if m.modal == "recent" && m.input.Value() == "" {
			message = "没有最近打开的文件夹"
		}
		box = append(box, row("  "+message, l.width, m.colors.Muted, bg))
	}
	return m.composeOverlay(base, box, l.x, l.y)
}

func (m *Model) clickPicker(msg tea.MouseClickMsg) tea.Cmd {
	m.syncPicker()
	rows := m.filteredActions()
	l := m.pickerLayout(rows)
	if msg.Button == tea.MouseLeft && msg.X >= l.x && msg.X < l.x+l.width {
		if msg.Y >= l.y && msg.Y < l.y+3 {
			return nil
		}
		if index := l.indexAt(msg.X, msg.Y); index >= 0 {
			m.selectPickerIndex(index)
			return m.selectPickerChoice()
		}
	}
	return m.updateModal(tea.KeyPressMsg{Code: tea.KeyEscape})
}

func (m *Model) isPicker() bool {
	return m.modal == "quick" || m.modal == "remotes" || m.modal == "refs" || m.modal == "recent"
}

func (m *Model) selectPickerChoice() tea.Cmd {
	m.syncPicker()
	rows := m.filteredActions()
	if len(rows) == 0 {
		return nil
	}
	a := rows[m.pickerIndex]
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
	m.syncPicker()
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
	m.pickerIndex = 0
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
