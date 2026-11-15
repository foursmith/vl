package app

import (
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/elegit/internal/fileicon"
	"github.com/ourongxing/elegit/internal/theme"
)

type pickerLayout struct {
	x, y, width, start, count int
}

func (m *Model) pickerLayout(rows []action) pickerLayout {
	w := min(96, m.width-4)
	count := min(16, max(1, m.height-7))
	start := max(0, m.choice-count+1)
	return pickerLayout{(m.width - w) / 2, 1, w, start, min(count, max(0, len(rows)-start))}
}

func (m *Model) pickerOverlay(base []string) []string {
	rows := m.filteredActions()
	l := m.pickerLayout(rows)
	const bg = "#222222"
	const inputBG = "#313131"
	w := l.width - 2
	border := theme.Style(theme.Accent, bg)
	box := []string{
		border.Render("╭" + strings.Repeat("─", w) + "╮"),
		border.Render("│") + row(" "+m.input.View(), w, theme.Foreground, inputBG) + border.Render("│"),
		border.Render("╰" + strings.Repeat("─", w) + "╯"),
	}
	for i := l.start; i < l.start+l.count; i++ {
		a := rows[i]
		fg, surface := theme.Foreground, bg
		if i == m.choice {
			fg, surface = "#FFFFFF", theme.Selection
		}
		if a.disabled != "" {
			fg = theme.Muted
		}
		label := " " + safe(a.label)
		if m.modal == "files" {
			icon := fileicon.For(a.id, m.nerd)
			name := " " + row(icon.Glyph, 2, icon.Color, surface) + safe(filepath.Base(a.id))
			label = ansi.Truncate(name, w-2, "…")
			if dir := filepath.Dir(a.id); dir != "." {
				remaining := w - 2 - ansi.StringWidth(label)
				if remaining > 1 {
					label += theme.Style("#737373", surface).Render(ansi.Truncate(" "+safe(dir), remaining, "…"))
				}
			}
		} else if a.key != "" {
			key := " " + a.key + " "
			label = ansi.Truncate(label, max(1, w-ansi.StringWidth(key)-2), "…")
			label += strings.Repeat(" ", max(1, w-ansi.StringWidth(label)-ansi.StringWidth(key)-1))
			label += theme.Style(theme.Muted, surface).Render(key)
		}
		box = append(box, row(" "+row(label, w, fg, surface)+" ", l.width, fg, surface))
	}
	if len(rows) == 0 {
		message := "无匹配结果"
		if m.modal == "recent" {
			if m.palette == nil {
				message = "读取最近文件夹…"
			} else if m.input.Value() == "" {
				message = "没有最近打开的文件夹"
			}
		}
		box = append(box, row("  "+message, l.width, theme.Muted, bg))
	} else if reason := rows[min(m.choice, len(rows)-1)].disabled; reason != "" {
		box = append(box, row("  "+safe(reason), l.width, theme.Muted, bg))
	}
	for i, line := range box {
		if l.y+i >= len(base) {
			break
		}
		original := base[l.y+i]
		base[l.y+i] = ansi.Cut(original, 0, l.x) + line + ansi.Cut(original, l.x+l.width, m.width)
	}
	return base
}

func (m *Model) clickPicker(msg tea.MouseClickMsg) tea.Cmd {
	rows := m.filteredActions()
	l := m.pickerLayout(rows)
	if msg.Button == tea.MouseLeft && msg.X >= l.x && msg.X < l.x+l.width {
		if msg.Y >= l.y && msg.Y < l.y+3 {
			return nil
		}
		if index := msg.Y - l.y - 3; index >= 0 && index < l.count {
			m.choice = l.start + index
			return m.selectPickerChoice()
		}
	}
	return m.updateModal(tea.KeyPressMsg{Code: tea.KeyEscape})
}

func (m *Model) isPicker() bool {
	return m.modal == "palette" || m.modal == "files" || m.modal == "refs" || m.modal == "recent"
}

func (m *Model) selectPickerChoice() tea.Cmd {
	rows := m.filteredActions()
	if len(rows) == 0 {
		return nil
	}
	a := rows[min(m.choice, len(rows)-1)]
	if a.disabled != "" {
		m.notice = a.disabled
		return nil
	}
	m.modal = ""
	m.input.Blur()
	return a.run()
}

func (m *Model) openQuickFiles() tea.Cmd {
	m.palette = nil
	for _, path := range m.snapshot.Files {
		m.palette = append(m.palette, action{id: path, label: path, run: func() tea.Cmd { return m.openFileInExplorer(path) }})
	}
	return m.prompt("files", "打开文件", "")
}
