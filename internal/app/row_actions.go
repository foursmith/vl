package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

type rowAction struct {
	id, glyph, label, disabled string
	x, width                   int
}

const sidebarRowActionWidth = 3

func layoutRowActions(actions []rowAction, end int) {
	for i := range actions {
		actions[i].width = sidebarRowActionWidth
		actions[i].x = end - (len(actions)-i)*sidebarRowActionWidth
	}
}

func (m *Model) repositoryRowActions(entry item) []rowAction {
	switch m.section {
	case changes:
		actions := []rowAction{
			{id: "refresh", glyph: m.symbol(symbolRefresh), label: m.tr("Refresh")},
			{id: "commit", glyph: m.symbol(symbolCheck), label: m.tr("Commit Staged"), disabled: m.commitDisabled(entry.repository)},
			{id: "stage", glyph: m.symbol(symbolAdd), label: m.tr("Stage All Changes"), disabled: m.changeActionDisabled(entry, "stage", m.scopedChanges(entry))},
		}
		if m.busy {
			for i := range actions {
				actions[i].disabled = m.tr("Wait for the current Git operation to finish")
			}
		}
		return actions
	case pullRequests:
		return []rowAction{
			{id: "filter", glyph: m.symbol(symbolFilter), label: m.tr("Filter Pull Requests") + " · " + m.pullRequestFilterLabel(m.pullRequestFilter)},
			{id: "refresh", glyph: m.symbol(symbolRefresh), label: m.tr("Refresh") + " · " + workbenchKeys.SidebarRefresh.Help().Key},
		}
	}
	return nil
}

func (m *Model) changeRowActions(entry item, w int) []rowAction {
	if entry.repoHeader {
		return m.layoutRepositoryRow(entry, w).actions
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
	layoutRowActions(actions, w-sidebarRowActionWidth)
	scope := m.scopedChanges(entry)
	for i := range actions {
		a := &actions[i]
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

func (m *Model) changeActionRow(entry item, text string, w int, fg, bg string, state treeRowState, actions []rowAction) string {
	if len(actions) == 0 {
		return text
	}
	start := actions[0].x
	label, _ := m.treeRow(entry, start+sidebarRowActionWidth, bg, state)
	marker := ""
	if !entry.header {
		marker = ansi.Cut(row(text, w, fg, bg), w-sidebarRowActionWidth, w)
	}
	return row(ansi.Cut(label, 0, start), start, fg, bg) +
		m.renderRowActions(actions, bg) + row(marker, sidebarRowActionWidth, fg, bg)
}

func (m *Model) renderRowActions(actions []rowAction, bg string) string {
	var text string
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
		width := m.sidebarContentWidth()
		x := m.hoverX - m.sideX()
		var actions []rowAction
		if entry.repoHeader {
			layout := m.layoutRepositoryRow(entry, width)
			if layout.branchAt(x) {
				return m.tr("Switch branch · %s", m.repositoryName(entry.repository))
			}
			actions = layout.actions
		} else if m.section == changes {
			actions = m.changeRowActions(entry, width)
		}
		if action := rowActionAt(actions, x); action != nil {
			if action.disabled != "" {
				return action.disabled
			}
			return action.label
		}
	}
	return ""
}

func rowActionAt(actions []rowAction, x int) *rowAction {
	for i := range actions {
		if action := &actions[i]; x >= action.x && x < action.x+action.width {
			return action
		}
	}
	return nil
}

// Shared by inline buttons and the context menu, including all scope safeguards.
func (m *Model) runChangeRowAction(index int, id string) tea.Cmd {
	if index < 0 || index >= len(m.items) {
		return nil
	}
	entry := m.items[index]
	m.selected, m.sidebar = index, true
	for _, a := range m.changeRowActions(entry, m.sidebarContentWidth()) {
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
