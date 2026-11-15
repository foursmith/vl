package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/elegit/internal/theme"
)

type rowAction struct {
	id, glyph, label, disabled string
	x, width                   int
}

func (m *Model) hoveredChangeRow() int {
	if m.section != changes {
		return -1
	}
	i := m.hoveredSidebarRow()
	if i < 0 || m.items[i].change == nil && m.items[i].folder == "" && !m.items[i].header {
		return -1
	}
	return i
}

func (m *Model) changeRowActions(entry item, w int) []rowAction {
	if entry.change == nil && !entry.header && entry.folder == "" {
		return nil
	}
	var actions []rowAction
	if entry.change != nil || entry.header {
		actions = append(actions, rowAction{id: "open", glyph: m.symbol(symbolOpen), label: "打开所有更改文件"})
	}
	if !entry.staged {
		actions = append(actions, rowAction{id: "discard", glyph: m.symbol(symbolDiscard), label: "丢弃此范围的全部更改"})
	}
	stage := rowAction{id: "stage", glyph: m.symbol(symbolAdd), label: "暂存此范围的全部更改"}
	if entry.staged {
		stage = rowAction{id: "unstage", glyph: m.symbol(symbolRemove), label: "取消暂存此范围的全部更改"}
	}
	actions = append(actions, stage)
	scope := m.scopedChanges(entry)
	for i := range actions {
		a := &actions[i]
		a.width, a.x = 3, w-3-(len(actions)-i)*3
		a.disabled = m.changeActionDisabled(entry, a.id, scope)
		if entry.change != nil {
			switch a.id {
			case "open":
				a.label = "打开文件"
			case "discard":
				a.label = "丢弃更改"
				if entry.change.Untracked() {
					a.label = "删除未跟踪文件"
				}
			case "stage":
				a.label = "暂存更改"
			case "unstage":
				a.label = "取消暂存"
			}
		}
	}
	return actions
}

func (m *Model) changeActionRow(entry item, w int, fg, bg string) string {
	actions := m.changeRowActions(entry, w)
	if len(actions) == 0 {
		text, _ := m.explorerRow(entry, w, bg)
		return text
	}
	start := actions[0].x
	label, _ := m.explorerRow(entry, start+3, bg)
	text := row(ansi.Cut(label, 0, start), start, fg, bg)
	hoverX := m.hoverX - m.sideX()
	for _, action := range actions {
		color := theme.Foreground
		if action.disabled != "" {
			color = theme.LineNumber
		} else if hoverX >= action.x && hoverX < action.x+action.width {
			color = "#FFFFFF"
		}
		text += row(" "+action.glyph+" ", action.width, color, bg)
	}
	full := row("", w, fg, bg)
	if !entry.header {
		full, _ = m.explorerRow(entry, w, bg)
	}
	text += row(ansi.Cut(full, w-3, w), 3, fg, bg)
	return text
}

func (m *Model) hoverHint() string {
	if m.resizingSidebar || m.sidebarDividerAt(m.hoverX, m.hoverY) {
		return "拖动调整侧边栏宽度"
	}
	i := m.hoveredChangeRow()
	if i < 0 {
		return ""
	}
	w := m.sideWidth()
	if !m.sideShown() {
		w = m.width
	}
	hoverX := m.hoverX - m.sideX()
	for _, action := range m.changeRowActions(m.items[i], w) {
		if hoverX >= action.x && hoverX < action.x+action.width {
			if action.disabled != "" {
				return action.disabled
			}
			return action.label
		}
	}
	return ""
}

func (m *Model) clickChangeAction(index, x, w int) (tea.Cmd, bool) {
	if index != m.hoveredChangeRow() {
		return nil, false
	}
	for _, action := range m.changeRowActions(m.items[index], w) {
		if x < action.x || x >= action.x+action.width {
			continue
		}
		m.selected = index
		m.sidebar = true
		if action.disabled != "" {
			m.notice = action.disabled
			return nil, true
		}
		if m.items[index].change == nil {
			return m.runScopeAction(m.items[index], action.id), true
		}
		switch action.id {
		case "open":
			return m.openFileInExplorer(m.items[index].change.Path), true
		case "discard":
			return m.discard(), true
		case "stage":
			return m.stageSelected(false), true
		case "unstage":
			return m.stageSelected(true), true
		}
	}
	return nil, false
}
