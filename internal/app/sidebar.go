package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/vl/internal/fileicon"
	"github.com/ourongxing/vl/internal/theme"
)

type navigationItem struct {
	x, width int
	glyph    string
	section  section
}

func (m *Model) sidebarAt(x int) bool {
	if m.sideShown() {
		return x >= m.sideX() && x < m.sideX()+m.sideWidth()
	}
	return x >= 0 && x < m.width && m.sidebar && m.sidebarVisible
}

func (m *Model) navigation() []navigationItem {
	return []navigationItem{
		{1, 4, m.symbol(symbolExplorer), explorer},
		{5, 4, m.symbol(symbolBranch), changes},
		{9, 4, m.symbol(symbolHistory), history},
	}
}

func (m *Model) sidebarLines() []string { return m.sidebarLinesWidth(m.sideWidth()) }

func (m *Model) hoveredSidebarRow() int {
	if m.resizingSidebar || m.modal != "" || m.hoverX < 0 || !m.sidebarAt(m.hoverX) || m.hoverY < m.sidebarHeaderRows() || m.hoverY >= m.bodyHeight() {
		return -1
	}
	i := m.sidebarRowAt(m.hoverY)
	if i < 0 || i >= len(m.items) {
		return -1
	}
	return i
}

func (m *Model) sidebarLinesWidth(w int) []string {
	if w == 0 {
		return nil
	}
	h := m.bodyHeight()
	var nav strings.Builder
	x := 0
	for _, item := range m.navigation() {
		nav.WriteString(row("", item.x-x, m.colors.SideForeground, m.colors.Panel))
		active := item.section == m.section
		fg := m.colors.NavigationInactiveForeground
		if active {
			fg = m.colors.NavigationForeground
		}
		label := item.glyph + " "
		label = ansi.Truncate(label, item.width, "")
		label += strings.Repeat(" ", max(0, item.width-ansi.StringWidth(label)))
		nav.WriteString(theme.Style(fg, m.colors.Panel).Underline(m.monochrome && active).Render(label))
		x = item.x + item.width
	}
	lines := m.workspaceLines(w)
	lines = append(lines, row(nav.String(), w, m.colors.SideForeground, m.colors.Panel))
	if m.snapshot.NoRepository && (m.section == changes || m.section == history) {
		for _, text := range []string{"", m.tr(" Not a Git repository"), "", m.tr(" [Initialize Repository]"), m.tr(" Enter: Initialize")} {
			lines = append(lines, row(text, w, m.colors.Muted, m.colors.Panel))
		}
		for len(lines) < h {
			lines = append(lines, row("", w, m.colors.SideForeground, m.colors.Panel))
		}
		return lines[:h]
	}
	hovered := m.hoveredSidebarRow()
	sticky := m.sidebarStickyRows()
	for y := m.sidebarHeaderRows(); y < h; y++ {
		i := m.sideTop + y - m.sidebarHeaderRows()
		if offset := y - m.sidebarHeaderRows(); offset < len(sticky) {
			i = sticky[offset]
		}
		text := ""
		fg, bg := m.colors.SideForeground, m.colors.Panel
		if i < len(m.items) {
			entry := m.items[i]
			if i == hovered {
				bg = m.colors.Hover
			}
			if i == m.selected {
				bg = m.colors.Selection
				if !m.sidebar {
					bg = m.colors.InactiveSelection
				}
			}
			if m.section == explorer || m.section == changes {
				text, fg = m.treeRow(entry, w, bg, treeRowState{selected: i == m.selected, hovered: i == hovered})
				if m.section == changes && (i == hovered || entry.repoHeader) {
					text = m.changeActionRow(entry, w, fg, bg, treeRowState{selected: i == m.selected, hovered: i == hovered})
				}
			} else {
				text, fg = m.historyRow(entry, w, bg)
			}
			if m.monochrome && i == m.selected && m.sidebar {
				text = ">" + ansi.Cut(text, 1, w)
			}
		}
		lines = append(lines, row(text, w, fg, bg))
	}
	return lines[:h]
}

type treeRowState struct{ selected, hovered bool }

func (m *Model) explorerRow(entry item, w int, bg string) (string, string) {
	return m.treeRow(entry, w, bg, treeRowState{})
}

func (m *Model) treeRow(entry item, w int, bg string, state treeRowState) (string, string) {
	if entry.repoHeader {
		prefix := m.repositoryHeaderPrefix(entry)
		branch := m.repositoryBranchText(entry, w, 0)
		nameWidth := max(0, w-ansi.StringWidth(prefix)-ansi.StringWidth(branch))
		text := prefix + row(safe(entry.label), nameWidth, m.colors.SideForeground, bg) + branch
		return row(theme.Style(m.colors.SideForeground, bg).Bold(true).Render(text), w, m.colors.SideForeground, bg), m.colors.SideForeground
	}
	if entry.header {
		return row(" "+strings.Repeat("  ", entry.depth)+m.treeChevron(m.folderExpanded(entry))+" "+entry.label, w, m.colors.Muted, bg), m.colors.Muted
	}
	decoration := m.gitDecorationColor(entry)
	fg := m.colors.SideForeground
	if state.hovered && !state.selected && m.colors.HoverForeground != "" {
		fg = m.colors.HoverForeground
	}
	if state.selected {
		if m.sidebar {
			if m.colors.SelectionForeground != "" {
				fg = m.colors.SelectionForeground
			}
		} else if m.colors.InactiveSelectionForeground != "" {
			fg = m.colors.InactiveSelectionForeground
		}
	}
	rowForeground := fg
	// iconlabel.css clears decorations only for a focused selected list. An
	// inactive selection/hover changes the row's inherited color, not its label's
	// explicitly declared Git decoration.
	if m.section == explorer && (entry.status != 0 || entry.ignored) && !(state.selected && m.sidebar) {
		fg = decoration
	}

	padding := "  "
	if m.section == explorer {
		padding = " "
	}
	indent := padding + strings.Repeat("  ", min(entry.depth, 10))
	path := entry.file
	if entry.change != nil {
		path = entry.change.Path
	} else if entry.historyChange != nil {
		path = entry.historyChange.Path
	}
	icon := fileicon.For(path, m.nerd)
	chevron := "  "
	if entry.folder != "" {
		chevronColor := rowForeground
		chevron = row(m.treeChevron(m.folderExpanded(entry)), 2, chevronColor, bg)
		icon = fileicon.Folder(entry.folder, m.folderExpanded(entry), m.nerd)
	}
	iconColor := icon.Color
	text := indent + chevron + row(icon.Glyph, 2, iconColor, bg) + safe(entry.label)
	marker := ""
	if entry.status != 0 && !entry.ignored {
		marker = string(entry.status)
		if entry.change != nil && entry.change.Gitlink {
			marker = "S"
		}
		if entry.folder != "" {
			marker = m.symbol(symbolDot)
		}
	}
	text = ansi.Truncate(text, max(0, w-3), "…")
	if marker != "" {
		markerColor := decoration
		if state.selected && m.sidebar && m.section == explorer {
			markerColor = fg
		}
		if entry.folder != "" {
			markerColor = theme.DecorationBubble(markerColor, bg)
		} else if m.section == explorer {
			markerColor = theme.DecorationBadge(markerColor, bg)
		}
		text += strings.Repeat(" ", max(1, w-2-ansi.StringWidth(text))) + theme.Style(markerColor, bg).Render(marker)
	}
	return text, fg
}

func (m *Model) clickSidebar(msg tea.MouseClickMsg) tea.Cmd {
	msg.X -= m.sideX()
	y := msg.Y
	if y >= m.workspaceRows() && y < m.workspaceRows()+headerHeight {
		for _, item := range m.navigation() {
			if msg.X >= item.x && msg.X < item.x+item.width {
				return m.selectSection(item.section)
			}
		}
		return nil
	}
	if y >= 0 && y < m.workspaceRows() {
		if y == 0 && msg.Button == tea.MouseLeft {
			m.workspaceCollapsed = !m.workspaceCollapsed
			m.resize()
			return nil
		}
		if index := m.workspaceTop + y - 1; y > 0 && index < len(m.workspaces) && msg.Button == tea.MouseLeft {
			return m.openRepository(m.workspaces[index])
		}
		return nil
	}
	if m.snapshot.NoRepository && (m.section == changes || m.section == history) {
		if y == m.sidebarHeaderRows()+3 && msg.X >= 1 && msg.X < ansi.StringWidth(m.tr(" [Initialize Repository]")) {
			return m.initializeRepository()
		}
		return nil
	}
	w := m.sideWidth()
	if !m.sideShown() {
		w = m.width
	}
	i := m.sidebarRowAt(y)
	if y < m.sidebarHeaderRows() || i < 0 || i >= len(m.items) {
		return nil
	}
	if m.repositoryBranchAt(m.items[i], msg.X, w) {
		m.selected = i
		m.sidebar = true
		return m.openBranchPicker(m.items[i].repository, "")
	}
	if m.section == changes {
		if cmd, handled := m.clickChangeAction(i, msg.X, w); handled {
			return cmd
		}
	}
	if m.items[i].header {
		m.selected = i
		m.sidebar = true
		return m.openSelected()
	}
	m.selected = i
	m.sidebar = true
	entry := m.current()
	if m.section == explorer && entry.file != "" {
		if m.doubleClick("file:" + entry.file) {
			return m.openFile(entry.file)
		}
		return m.openPreviewFile(entry.file)
	}
	if entry.change != nil || entry.historyChange != nil {
		double := m.doubleClick("diff:" + entry.key)
		cmd := m.preview()
		if double {
			m.keepTab()
			m.sidebar = false
		}
		return cmd
	}
	m.lastClickKey = ""
	if m.section == history && entry.commit != nil {
		return m.openHistory(*entry)
	}
	if m.section == explorer || entry.folder != "" {
		return m.openSelected()
	}
	return m.preview()
}

func (m *Model) gitDecorationColor(entry item) string {
	if entry.ignored {
		return m.colors.Ignored
	}
	if entry.status == '!' {
		return m.colors.GitConflicting
	}
	if entry.change != nil && entry.change.Gitlink {
		return m.colors.GitSubmodule
	}
	switch entry.status {
	case 'U':
		return m.colors.GitUntracked
	case 'A':
		return m.colors.GitAdded
	case 'R', 'C':
		return m.colors.GitRenamed
	case 'M', 'T':
		if entry.staged {
			return m.colors.GitStageModified
		}
		return m.colors.GitModified
	case 'D':
		if entry.staged {
			return m.colors.GitStageDeleted
		}
		return m.colors.GitDeleted
	}
	return m.colors.SideForeground
}
