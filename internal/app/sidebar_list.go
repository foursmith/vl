package app

import (
	"fmt"
	"strings"

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
	delta := count
	if action == "pageUp" || action == "pageDown" {
		delta *= l.height
	}
	switch action {
	case "up", "pageUp":
		*l.selected = max(0, *l.selected-delta)
	case "down", "pageDown":
		*l.selected = min(l.count-1, max(-1, *l.selected)+delta)
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

func (m *Model) sidebarNavigationKey(k string) (bool, tea.Cmd) {
	l := m.sidebarList()
	key := ""
	if entry := m.sidebarEntry(); entry != nil {
		key = entry.key
	}
	scope := fmt.Sprintf("sidebar:%d:%s:%d:%s", m.section, m.repo.Root, *l.selected, key)
	if m.section == search {
		scope += fmt.Sprintf(":%d", m.search.seq)
	}
	action, count, handled := m.navigationState.key(scope, k)
	if m.section == search && (action == "dd" || k == "delete") {
		m.dismissSearchResult(*l.selected)
		return true, nil
	}
	if !handled {
		return false, nil
	}
	switch action {
	case "":
		return true, nil
	case "dd", "yy":
		if m.section != explorer {
			return true, nil
		}
		if count != 1 {
			m.notice = m.tr("File operations act on one selected entry")
			return true, nil
		}
		if action == "dd" {
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
		ancestor := false
		switch m.section {
		case changes:
			if entry.header {
				ancestor = parent.repoHeader
			} else if parent.staged == entry.staged {
				ancestor = parent.header && !parent.repoHeader || parent.folder != "" && strings.HasPrefix(path, parent.folder+"/")
			}
		case history:
			if entry.commit != nil {
				ancestor = parent.repoHeader
			} else {
				ancestor = parent.commit != nil && historyKey(parent.repository, parent.commit.ID) == entry.historyID || parent.historyID == entry.historyID && parent.folder != "" && strings.HasPrefix(path, parent.folder+"/")
			}
		default:
			ancestor = parent.folder != "" && strings.HasPrefix(path, parent.folder+"/")
		}
		if ancestor {
			m.selected = i
			m.ensureSidebar()
			return
		}
	}
}
