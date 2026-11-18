package app

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// sidebarEntry adapts search results to the same file target as tree rows.
func (m *Model) sidebarEntry() *item {
	if m.section != search {
		return m.current()
	}
	row, ok := m.selectedSearchRow()
	if !ok {
		return nil
	}
	entry := m.searchEntry(row)
	return &entry
}

func (m *Model) searchEntry(row searchRow) item {
	file := m.search.result.Files[row.file]
	if row.match < 0 {
		return item{key: "file:" + file.Path, file: file.Path}
	}
	hit := file.Matches[row.match]
	key := fmt.Sprintf("match:%d:%d:%s", hit.Start, hit.End, file.Path)
	return item{key: key, file: file.Path}
}

func entryPath(entry *item) string {
	if entry == nil || entry.header || entry.repoHeader {
		return ""
	}
	if entry.folder != "" {
		return entry.folder
	}
	if entry.change != nil {
		return entry.change.Path
	}
	if entry.historyChange != nil {
		return entry.historyChange.Path
	}
	if entry.pullRequestFile != nil {
		return entry.pullRequestFile.Path
	}
	return entry.file
}

func (m *Model) openSidebarContextMenu() tea.Cmd {
	m.ensureSidebar()
	return m.openContextMenu(m.sideX()+2, m.sidebarSelectionY())
}

// Sources load content; this shared path owns preview promotion and editor focus.
func (m *Model) openSidebarFile(preview bool) tea.Cmd {
	entry := m.sidebarEntry()
	if entry == nil {
		return nil
	}
	return m.openSidebarEntry(*entry, preview)
}

func (m *Model) openSidebarEntry(entry item, preview bool) tea.Cmd {
	cmd, opened := m.loadSidebarFile(entry)
	if opened && !preview {
		m.focusEditor()
	}
	return cmd
}

func (m *Model) loadSidebarFile(entry item) (tea.Cmd, bool) {
	if m.section == search {
		return m.loadSearchResult()
	}
	if entry.file != "" {
		return nil, m.openDocument(entry.file, true)
	}
	if entry.historyChange != nil {
		return m.requestHistoryDiff(entry), true
	}
	if entry.pullRequestFile != nil {
		return m.requestPullRequestDiff(entry), true
	}
	if entry.change == nil {
		return nil, false
	}
	c, staged := *entry.change, entry.staged
	return m.requestDiff(diffTarget{path: c.Path, repository: c.Repository, staged: staged, change: &c, preview: true}, func(ctx context.Context) (string, error) {
		return m.repo.Diff(ctx, c, staged)
	}), true
}

func (m *Model) clickSidebarFile() tea.Cmd {
	entry := m.sidebarEntry()
	if entry == nil {
		return nil
	}
	key := fmt.Sprintf("sidebar:%d:%s:%s", m.section, m.repo.Root, entry.key)
	double := m.doubleClick(key)
	return m.openSidebarEntry(*entry, !double)
}
