package app

import (
	"strconv"
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

const (
	headerHeight           = 2
	sidebarNavigationWidth = 4
)

func (m *Model) sidebarAt(x int) bool {
	if m.sideShown() {
		return x >= m.sideX() && x < m.sideX()+m.sideWidth()
	}
	return x >= 0 && x < m.width && m.sidebar && m.sidebarVisible
}

func (m *Model) navigation() []navigationItem {
	return []navigationItem{
		{0, sidebarNavigationWidth, m.symbol(symbolExplorer), explorer},
		{sidebarNavigationWidth, sidebarNavigationWidth, m.symbol(symbolSearch), search},
		{2 * sidebarNavigationWidth, sidebarNavigationWidth, m.symbol(symbolBranch), changes},
		{3 * sidebarNavigationWidth, sidebarNavigationWidth, m.symbol(symbolHistory), history},
		{4 * sidebarNavigationWidth, sidebarNavigationWidth, m.symbol(symbolPullRequest), pullRequests},
	}
}

func (m *Model) sidebarLines() []string { return m.sidebarLinesWidth(m.sideWidth()) }

func (m *Model) hoveredSidebarRow() int {
	if m.resizingSidebar || m.modal != "" || m.hoverX < 0 || !m.sidebarAt(m.hoverX) || m.hoverY < headerHeight || m.hoverY >= m.bodyHeight() {
		return -1
	}
	return m.sidebarRowAt(m.hoverY)
}

func (m *Model) sidebarLinesWidth(w int) []string {
	if w == 0 {
		return nil
	}
	h := m.bodyHeight()
	var nav, navBottom strings.Builder
	inactive := theme.Readable(m.colors.NavigationInactiveForeground, m.colors.Panel)
	for _, item := range m.navigation() {
		active := item.section == m.section
		fg, bg := inactive, m.colors.Panel
		if active {
			fg = theme.Readable(m.colors.NavigationForeground, bg)
			if !m.monochrome && fg == inactive {
				bg = m.colors.SidebarSelection(true)
				fg = theme.Readable(m.colors.NavigationForeground, bg)
			}
		}
		badge := ""
		count := 0
		switch item.section {
		case changes:
			count = len(m.snapshot.Changes)
		case pullRequests:
			if !m.snapshot.NoRepository {
				count = m.openPullRequestCount
			}
		}
		if count > 0 {
			badge = strconv.Itoa(count)
			if count > 99 {
				badge = "99+"
			}
			badgeBackground := m.colors.SidebarSelection(true)
			badgeForeground := m.colors.SelectionForeground
			if badgeForeground == "" {
				badgeForeground = m.colors.SideForeground
			}
			badge = theme.Style(theme.Readable(badgeForeground, badgeBackground), badgeBackground).Render(badge)
		}
		glyph := ansi.Truncate(item.glyph, item.width, "")
		label := theme.Style(fg, bg).Underline(active && m.monochrome).Render(glyph)
		nav.WriteString(row(label, item.width, fg, bg))
		navBottom.WriteString(row(badge, item.width, fg, bg))
	}
	lines := []string{
		row(nav.String(), w, m.colors.SideForeground, m.colors.Panel),
		row(navBottom.String(), w, m.colors.SideForeground, m.colors.Panel),
	}
	if m.section == search {
		return m.searchLines(lines, w)
	}
	if m.snapshot.NoRepository && (m.section == changes || m.section == history) {
		for _, text := range []string{"", m.tr("Not a Git repository"), "", m.tr("[Initialize Repository]"), m.tr("Enter: Initialize")} {
			lines = append(lines, row(text, w, m.colors.Muted, m.colors.Panel))
		}
		for len(lines) < h {
			lines = append(lines, row("", w, m.colors.SideForeground, m.colors.Panel))
		}
		return lines[:h]
	}
	if m.section == pullRequests && len(m.items) == 0 {
		for _, message := range m.pullRequestSidebarMessage() {
			for _, line := range strings.Split(ansi.Wrap(message, max(1, w-2), ""), "\n") {
				lines = append(lines, row(" "+line, w, m.colors.Muted, m.colors.Panel))
			}
		}
	}
	hovered := m.hoveredSidebarRow()
	sticky := m.sidebarStickyRows()
	for y := len(lines); y < h; y++ {
		i := m.sideTop + y - headerHeight
		if offset := y - headerHeight; offset < len(sticky) {
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
				bg = m.colors.SidebarSelection(m.sidebar)
			}
			state := treeRowState{selected: i == m.selected, hovered: i == hovered}
			if m.section == explorer || m.section == changes {
				text, fg = m.treeRow(entry, w, bg, state)
				if m.section == changes && (i == hovered || entry.repoHeader) {
					text = m.sidebarActionRow(entry, w, fg, bg, state, m.changeRowActions(entry, w))
				}
			} else {
				text, fg = m.historyRow(entry, w, bg)
				if m.section == pullRequests && entry.repoHeader {
					text = m.sidebarActionRow(entry, w, fg, bg, state, m.pullRequestRowActions(entry, w))
				}
			}
			if i == m.selected && m.monochrome && m.sidebar {
				text = ">" + ansi.Cut(text, 1, w)
			}
		}
		lines = append(lines, row(text, w, theme.Readable(fg, bg), bg))
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
		fg := theme.Readable(m.colors.Muted, bg)
		return row(strings.Repeat("  ", entry.depth)+m.treeChevron(m.folderExpanded(entry))+" "+entry.label, w, fg, bg), fg
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
	if m.section == changes {
		padding = " "
	}
	if m.section == explorer {
		padding = ""
		if m.monochrome && m.sidebar && state.selected {
			padding = " " // The selection marker occupies this cell.
		}
	}
	indent := padding + strings.Repeat("  ", min(entry.depth, 10))
	path := entryPath(&entry)
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
		text += strings.Repeat(" ", max(1, w-2-ansi.StringWidth(text))) + theme.TextStyle(markerColor, bg).Render(marker)
	}
	return text, theme.Readable(fg, bg)
}

func (m *Model) clickSidebar(msg tea.MouseClickMsg) tea.Cmd {
	msg.X -= m.sideX()
	y := msg.Y
	if y >= 0 && y < headerHeight {
		for _, item := range m.navigation() {
			if msg.X >= item.x && msg.X < item.x+item.width {
				if item.section == search {
					return m.openSearch()
				}
				return m.selectSection(item.section)
			}
		}
		return nil
	}
	if m.snapshot.NoRepository && (m.section == changes || m.section == history) {
		if y == headerHeight+3 && msg.X >= 0 && msg.X < ansi.StringWidth(m.tr("[Initialize Repository]")) {
			return m.initializeRepository()
		}
		return nil
	}
	if m.section == search {
		return m.clickSearch(msg.X, y)
	}
	w := m.sideWidth()
	if !m.sideShown() {
		w = m.width
	}
	i := m.sidebarRowAt(y)
	if y < headerHeight || i < 0 || i >= len(m.items) {
		return nil
	}
	if m.section == pullRequests {
		for _, action := range m.pullRequestRowActions(m.items[i], w) {
			if msg.X >= action.x && msg.X < action.x+action.width {
				m.lastClickKey = ""
				if action.id == "filter" {
					return m.openPullRequestFilter()
				}
				return m.forceRefreshPullRequests()
			}
		}
	}
	entry := m.selectSidebarRow(i)
	if entry == nil {
		return nil
	}
	if m.repositoryBranchAt(*entry, msg.X, w) {
		m.lastClickKey = ""
		return m.openBranchPicker(m.items[i].repository, "")
	}
	if m.section == changes {
		if cmd, handled := m.clickChangeAction(i, msg.X, w); handled {
			m.lastClickKey = ""
			return cmd
		}
	}
	if entry.header {
		m.lastClickKey = ""
		return m.openSelected()
	}
	if entry.file != "" || entry.change != nil || entry.historyChange != nil || entry.pullRequestFile != nil {
		return m.clickSidebarFile()
	}
	m.lastClickKey = ""
	if m.section == history && entry.commit != nil {
		return m.openHistoryRow(*entry, false)
	}
	if m.section == pullRequests {
		return m.openPullRequestRow(*entry, false)
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
