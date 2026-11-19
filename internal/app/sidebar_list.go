package app

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// A view of the active list state, not another copy of its selection or scroll.
// Search owns different rows and header geometry; list interactions are shared.
type sidebarList struct {
	selected, top        *int
	count, start, height int
}

func (m *Model) treeList() sidebarList {
	return sidebarList{&m.selected, &m.sideTop, len(m.items), headerHeight, max(1, m.bodyHeight()-headerHeight)}
}
func (m *Model) searchList() sidebarList {
	start := m.searchResultY()
	return sidebarList{&m.search.selected, &m.search.top, len(m.searchRows()), start, max(1, m.bodyHeight()-start)}
}
func (m *Model) sidebarList() sidebarList {
	if m.section == search {
		return m.searchList()
	}
	return m.treeList()
}
func (l sidebarList) clampScroll() {
	*l.top = min(max(0, *l.top), max(0, l.count-l.height))
}
func (l sidebarList) ensureSelected() {
	if *l.selected < *l.top {
		*l.top = *l.selected
	}
	if *l.selected >= *l.top+l.height {
		*l.top = *l.selected - l.height + 1
	}
	*l.top = max(0, *l.top)
}
func (l sidebarList) move(action string, count int) bool {
	switch action {
	case "up":
		*l.selected = max(0, *l.selected-count)
	case "down":
		*l.selected = min(l.count-1, max(-1, *l.selected)+count)
	case "pgup", "pgdown", "ctrl+u", "ctrl+d":
		delta := pageDelta(action, l.height) * count
		*l.selected = min(l.count-1, max(0, max(-1, *l.selected)+delta))
		if isHalfPageKey(action) {
			*l.top += delta
			l.clampScroll()
		}
	case "first":
		*l.selected = min(l.count-1, count-1)
	case "last":
		*l.selected = l.count - 1
	default:
		return false
	}
	if l.count == 0 {
		*l.selected = -1
	}
	return true
}

func (m *Model) sidebarNavigationKey(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	l := m.sidebarList()
	entryKey := ""
	if entry := m.sidebarEntry(); entry != nil {
		entryKey = entry.key
	}
	scope := fmt.Sprintf("sidebar:%d:%s:%d:%s", m.section, m.repo.Root, *l.selected, entryKey)
	if m.section == search {
		scope += fmt.Sprintf(":%d", m.search.seq)
	}
	action, count, handled := m.navigationState.key(scope, msg.String())
	if m.section == search && (action == sidebarDeleteSequence || key.Matches(msg, sidebarKeys.Delete)) {
		m.dismissSearchResult(*l.selected)
		return true, nil
	}
	if !handled {
		return false, nil
	}
	switch action {
	case "":
		return true, nil
	case sidebarDeleteSequence, sidebarCopySequence:
		if m.section != explorer {
			return true, nil
		}
		if count != 1 {
			m.notice = m.tr("File operations act on one selected entry")
			return true, nil
		}
		if action == sidebarDeleteSequence {
			return true, m.deleteSelectedEntry()
		}
		return true, m.yankEntry()
	case "left":
		m.selectParentFolder()
		return true, nil
	case "right":
		if m.section == search {
			return true, m.openSearchRight()
		}
		return true, m.openSelected()
	}
	if l.move(action, count) {
		m.ensureSidebar()
		return true, m.preview()
	}
	return true, nil
}

func (m *Model) selectSidebarRow(index int) *item {
	l := m.sidebarList()
	if index < 0 || index >= l.count {
		return nil
	}
	*l.selected = index
	m.sidebar = true
	if m.section == search {
		m.focusSearch(2)
	}
	entry := m.sidebarEntry()
	if entry == nil {
		return nil
	}
	copy := *entry
	return &copy
}

func (m *Model) sidebarRowWithKey(key string) int {
	if m.section == search {
		if !m.search.loading {
			for index, row := range m.searchRows() {
				if m.searchEntry(row).key == key {
					return index
				}
			}
		}
		return -1
	}
	for index, entry := range m.items {
		if entry.key == key {
			return index
		}
	}
	return -1
}

func (m *Model) sidebarSelectionY() int {
	l := m.sidebarList()
	for y, index := range m.sidebarStickyRows() {
		if index == *l.selected {
			return l.start + y
		}
	}
	return l.start + max(0, *l.selected-*l.top)
}

func (m *Model) sidebarCollapsedFolders(entry item) map[string]bool {
	switch {
	case entry.repoHeader || m.section == changes:
		return m.sidebarCollapsed
	case m.section == pullRequests && entry.pullRequest != nil:
		if review := m.pullRequestReviews[entry.pullRequest.URL]; review != nil {
			return review.collapsed
		}
	case m.section == history:
		if state := m.historyStates[entry.historyID]; state != nil {
			return state.collapsed
		}
	}
	return nil
}

func (m *Model) folderExpanded(entry item) bool {
	if m.section == explorer && !entry.repoHeader {
		return m.expanded[entry.folder]
	}
	collapsed := m.sidebarCollapsedFolders(entry)
	return collapsed != nil && !collapsed[entry.key]
}

func (m *Model) selectParentFolder() {
	if m.section == search {
		rows := m.searchRows()
		if m.search.selected < 0 || m.search.selected >= len(rows) {
			return
		}
		selected := rows[m.search.selected]
		for i := m.search.selected; i >= 0; i-- {
			if rows[i].file == selected.file && rows[i].match < 0 {
				m.search.selected = i
				m.ensureSidebar()
				return
			}
		}
		return
	}
	entry := m.current()
	if entry == nil || entry.repoHeader {
		return
	}
	path := entryPath(entry)
	// Visible ancestors handle compact folder chains without inventing hidden rows.
	for i := m.selected - 1; i >= 0; i-- {
		parent := m.items[i]
		if parent.repository != entry.repository {
			continue
		}
		inFolder := parent.folder != "" && strings.HasPrefix(path, parent.folder+"/")
		ancestor := false
		switch m.section {
		case pullRequests:
			if entry.isPullRequest() {
				ancestor = parent.repoHeader
			} else if entry.pullRequest != nil && parent.pullRequest == entry.pullRequest {
				ancestor = parent.isPullRequest() || inFolder
			}
		case changes:
			if entry.header {
				ancestor = parent.repoHeader
			} else if parent.staged == entry.staged {
				ancestor = parent.header && !parent.repoHeader || inFolder
			}
		case history:
			if entry.commit != nil {
				ancestor = parent.repoHeader
			} else {
				ancestor = parent.commit != nil && historyKey(parent.repository, parent.commit.ID) == entry.historyID || parent.historyID == entry.historyID && inFolder
			}
		default:
			ancestor = parent.repoHeader || inFolder
		}
		if ancestor {
			m.selected = i
			m.ensureSidebar()
			return
		}
	}
}
