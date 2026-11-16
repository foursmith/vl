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
	if entry.repoHeader {
		scope := m.scopedChanges(entry)
		actions := []rowAction{
			{id: "refresh", glyph: m.symbol(symbolRefresh), label: "刷新仓库"},
			{id: "commit", glyph: m.symbol(symbolCheck), label: "提交此仓库的暂存更改", disabled: m.commitDisabled(entry.repository)},
			{id: "stage", glyph: m.symbol(symbolAdd), label: "暂存此仓库的全部更改", disabled: m.changeActionDisabled(entry, "stage", scope)},
		}
		branchWidth := ansi.StringWidth(m.repositoryBranchText(entry, w, len(actions)*3))
		for i := range actions {
			actions[i].width, actions[i].x = 3, w-branchWidth-(len(actions)-i)*3
			if m.busy {
				actions[i].disabled = "等待当前 Git 操作完成"
			}
		}
		return actions
	}
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
	labelWidth := start + 3
	labelEntry := entry
	if entry.repoHeader {
		labelWidth = start
		labelEntry.branch = ""
	}
	label, _ := m.explorerRow(labelEntry, labelWidth, bg)
	text := row(ansi.Cut(label, 0, start), start, fg, bg)
	hoverX := m.hoverX - m.sideX()
	for _, action := range actions {
		color := m.colors.Foreground
		if action.disabled != "" {
			color = m.colors.LineNumber
		} else if hoverX >= action.x && hoverX < action.x+action.width {
			color = m.colors.Strong
		}
		text += row(" "+action.glyph+" ", action.width, color, bg)
	}
	if entry.repoHeader {
		branch := m.repositoryBranchText(entry, w, len(actions)*3)
		return text + theme.Style(m.colors.Foreground, bg).Bold(true).Render(branch)
	}
	full := row("", w, fg, bg)
	if !entry.header {
		full, _ = m.explorerRow(entry, w, bg)
	}
	text += row(ansi.Cut(full, w-3, w), 3, fg, bg)
	return text
}

func (m *Model) hoverHint() string {
	if index := m.hoveredWorkspace(); index >= 0 {
		return "切换 Workspace · " + safe(m.workspaces[index])
	}
	if m.resizingSidebar || m.sidebarDividerAt(m.hoverX, m.hoverY) {
		return "拖动调整侧边栏宽度"
	}
	if m.modal == "" && m.section == search && m.sidebarAt(m.hoverX) && m.hoverY == 3+m.workspaceRows() {
		controls := m.search.controls()
		i := (m.hoverX - m.sideX()) / searchOptionWidth
		if i >= 0 && i < len(controls) {
			return controls[i].hint
		}
	}
	if m.section == search && m.searchHoveredRow() >= 0 {
		x := m.hoverX - m.sideX()
		if x == m.searchControlWidth()-1 {
			return "Remove from Search Results (Delete)"
		}
	}
	if index := m.hoveredSidebarRow(); index >= 0 {
		width := m.sideWidth()
		if !m.sideShown() {
			width = m.width
		}
		if m.repositoryBranchAt(m.items[index], m.hoverX-m.sideX(), width) {
			return "切换 " + m.repositoryName(m.items[index].repository) + " 的分支"
		}
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
	if !m.items[index].repoHeader && index != m.hoveredChangeRow() {
		return nil, false
	}
	for _, action := range m.changeRowActions(m.items[index], w) {
		if x < action.x || x >= action.x+action.width {
			continue
		}
		return m.runChangeRowAction(index, action.id), true
	}
	return nil, false
}

// Shared by inline buttons and the context menu, including all scope safeguards.
func (m *Model) runChangeRowAction(index int, id string) tea.Cmd {
	if index < 0 || index >= len(m.items) {
		return nil
	}
	entry := m.items[index]
	m.selected, m.sidebar = index, true
	for _, a := range m.changeRowActions(entry, m.sideWidth()) {
		if a.id == id && a.disabled != "" {
			m.notice = a.disabled
			return nil
		}
	}
	if entry.repoHeader {
		switch id {
		case "commit":
			return m.commitInRepository(entry.repository)
		case "refresh":
			return m.refresh()
		}
	}
	if entry.change == nil {
		return m.runScopeAction(entry, id)
	}
	switch id {
	case "open":
		return m.openFileInExplorer(entry.change.Path)
	case "discard":
		return m.discard()
	case "stage":
		return m.stageSelected(false)
	case "unstage":
		return m.stageSelected(true)
	}
	return nil
}
