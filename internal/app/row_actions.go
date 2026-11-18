package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/vl/internal/theme"
)

type rowAction struct {
	id, glyph, label, disabled string
	x, width                   int
}

const repositoryRowActionWidth = 3

func (m *Model) layoutRepositoryActions(entry item, w int, actions []rowAction) []rowAction {
	branchWidth := ansi.StringWidth(m.repositoryBranchText(entry, w, len(actions)*repositoryRowActionWidth))
	for i := range actions {
		actions[i].width = repositoryRowActionWidth
		actions[i].x = w - branchWidth - (len(actions)-i)*repositoryRowActionWidth
	}
	return actions
}

func (m *Model) pullRequestRowActions(entry item, w int) []rowAction {
	if !entry.repoHeader {
		return nil
	}
	return m.layoutRepositoryActions(entry, w, []rowAction{
		{id: "filter", glyph: m.symbol(symbolFilter), label: m.tr("Filter Pull Requests") + " · " + m.pullRequestFilterLabel(m.pullRequestFilter)},
		{id: "refresh", glyph: m.symbol(symbolRefresh), label: m.tr("Refresh") + " · " + workbenchKeys.Refresh.Help().Key},
	})
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
			{id: "refresh", glyph: m.symbol(symbolRefresh), label: m.tr("Refresh")},
			{id: "commit", glyph: m.symbol(symbolCheck), label: m.tr("Commit Staged"), disabled: m.commitDisabled(entry.repository)},
			{id: "stage", glyph: m.symbol(symbolAdd), label: m.tr("Stage All Changes"), disabled: m.changeActionDisabled(entry, "stage", scope)},
		}
		for i := range actions {
			if m.busy {
				actions[i].disabled = m.tr("Wait for the current Git operation to finish")
			}
		}
		return m.layoutRepositoryActions(entry, w, actions)
	}
	if entry.change == nil && !entry.header && entry.folder == "" {
		return nil
	}
	var actions []rowAction
	if entry.change != nil || entry.header {
		actions = append(actions, rowAction{id: "open", glyph: m.symbol(symbolOpen), label: m.tr("Open All Changed Files")})
	}
	if !entry.staged {
		actions = append(actions, rowAction{id: "discard", glyph: m.symbol(symbolDiscard), label: m.tr("Discard All Changes")})
	}
	stage := rowAction{id: "stage", glyph: m.symbol(symbolAdd), label: m.tr("Stage All Changes")}
	if entry.staged {
		stage = rowAction{id: "unstage", glyph: m.symbol(symbolRemove), label: m.tr("Unstage All Changes")}
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
				a.label = m.tr("Open File")
			case "discard":
				a.label = m.tr("Discard Changes")
				if entry.change.Untracked() {
					a.label = m.tr("Delete Untracked File")
				}
			case "stage":
				a.label = m.tr("Stage Changes")
			case "unstage":
				a.label = m.tr("Unstage Changes")
			}
		}
	}
	return actions
}

func (m *Model) sidebarActionRow(entry item, w int, fg, bg string, state treeRowState, actions []rowAction) string {
	if len(actions) == 0 {
		text, _ := m.treeRow(entry, w, bg, state)
		return text
	}
	start := actions[0].x
	labelWidth := start + 3
	labelEntry := entry
	if entry.repoHeader {
		labelWidth = start
		labelEntry.branch = ""
	}
	label, _ := m.treeRow(labelEntry, labelWidth, bg, state)
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
		branch := m.repositoryBranchText(entry, w, len(actions)*repositoryRowActionWidth)
		return text + theme.Style(m.colors.Foreground, bg).Bold(true).Render(branch)
	}
	full := row("", w, fg, bg)
	if !entry.header {
		full, _ = m.treeRow(entry, w, bg, state)
	}
	text += row(ansi.Cut(full, w-3, w), 3, fg, bg)
	return text
}

func (m *Model) hoverHint() string {
	if m.modal == "" && m.fileFindAt(m.hoverX, m.hoverY) {
		for _, c := range m.fileFindControls(m.fileFindWidth()) {
			if m.hoverY == m.contentRow()+c.y && m.hoverX >= m.fileFindX()+c.x && m.hoverX < m.fileFindX()+c.x+c.width {
				return c.hint
			}
		}
	}
	if m.resizingSidebar || m.sidebarDividerAt(m.hoverX, m.hoverY) {
		return m.tr("Drag to resize sidebar")
	}
	if m.modal == "" && m.hoverY >= 0 && m.hoverY < headerHeight && m.sidebarAt(m.hoverX) {
		for _, nav := range m.navigation() {
			x := m.hoverX - m.sideX()
			if nav.section == pullRequests && x >= nav.x && x < nav.x+nav.width {
				return m.tr("Pull Requests") + " · " + workbenchKeys.PullRequests.Help().Key
			}
		}
	}
	if m.modal == "" && m.section == search && m.sidebarAt(m.hoverX) {
		return m.searchHoverHint(m.hoverX-m.sideX(), m.hoverY)
	}

	if index := m.hoveredSidebarRow(); index >= 0 {
		entry := m.items[index]
		if m.contextMenu == nil {
			if title := entry.revisionTitle(); title != "" {
				return title
			}
		}
		width := m.sideWidth()
		if !m.sideShown() {
			width = m.width
		}
		if m.section == pullRequests {
			for _, action := range m.pullRequestRowActions(entry, width) {
				if x := m.hoverX - m.sideX(); x >= action.x && x < action.x+action.width {
					return action.label
				}
			}
		}
		if m.repositoryBranchAt(entry, m.hoverX-m.sideX(), width) {
			return m.tr("Switch branch · %s", m.repositoryName(entry.repository))
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
