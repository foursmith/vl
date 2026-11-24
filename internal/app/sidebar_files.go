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
	key := fmt.Sprintf("match:%d:%d:%d:%s", hit.Line, hit.Start, hit.End, file.Path)
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
	if m.outlineHasFocus() || m.fileHistoryHasFocus() {
		return nil
	}
	m.ensureSidebar()
	return m.openContextMenu(m.sideX()+2, m.sidebarSelectionY())
}

func (entry item) isSidebarFile() bool {
	return entry.file != "" || entry.change != nil || entry.historyChange != nil || entry.pullRequestFile != nil
}

func (m *Model) activateSidebarEntry(entry item, preview bool) tea.Cmd {
	switch {
	case entry.header || entry.folder != "":
		return m.toggleSidebarFolder(entry)
	case entry.historyPage != nil:
		return m.loadHistoryPage(entry.historyPage)
	case m.section == pullRequests && entry.key == pullRequestMoreKey:
		return m.loadMorePullRequests()
	case entry.commit != nil:
		return m.toggleHistoryCommit(entry)
	case entry.isPullRequest():
		return m.togglePullRequest(entry)
	case entry.isSidebarFile():
		return m.openSidebarEntry(entry, preview)
	}
	return nil
}

func (m *Model) toggleSidebarFolder(entry item) tea.Cmd {
	if m.section == explorer && !entry.repoHeader {
		m.expanded[entry.folder] = !m.expanded[entry.folder]
		return m.refreshTree()
	}
	if collapsed := m.sidebarCollapsedFolders(entry); collapsed != nil {
		collapsed[entry.key] = !collapsed[entry.key]
		m.rebuildItems()
	}
	return nil
}

// Sources load content; this shared path owns preview promotion and editor focus.
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

func (m *Model) clickSidebarEntry() tea.Cmd {
	entry := m.sidebarEntry()
	if entry == nil {
		return nil
	}
	if !entry.isSidebarFile() {
		m.lastClickKey = ""
		return m.activateSidebarEntry(*entry, false)
	}
	key := fmt.Sprintf("sidebar:%d:%s:%s", m.section, m.repo.Root, entry.key)
	double := m.doubleClick(key)
	return m.activateSidebarEntry(*entry, !double)
}
