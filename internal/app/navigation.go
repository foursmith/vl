package app

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// Navigation prefixes belong to one surface and one selected target.
// They are never fed to text inputs or the editor's Vim state.
type navigationState struct {
	scope, pending string
	count          int
}

func (n *navigationState) key(scope, k string) (string, int, bool) {
	if n.scope != scope {
		*n = navigationState{scope: scope}
	}
	if len(k) == 1 && k[0] >= '0' && k[0] <= '9' && (k != "0" || n.count > 0) && n.pending == "" {
		n.count = min(10000, n.count*10+int(k[0]-'0'))
		return "", 0, true
	}
	count := max(1, n.count)
	if n.pending != "" {
		pending := n.pending
		n.pending = ""
		n.count = 0
		if k == pending {
			if k == "g" {
				return "first", count, true
			}
			return k + k, count, true
		}
		// Cancel an unfinished operator, then handle this key normally.
		count = 1
	}
	switch k {
	case "g", "d", "y":
		n.pending = k
		return "", 0, true
	case "G":
		explicit := n.count > 0
		n.count = 0
		if explicit {
			return "first", count, true
		}
		return "last", 1, true
	case "j", "down":
		k = "down"
	case "k", "up":
		k = "up"
	case "h", "left":
		k = "left"
	case "l", "right":
		k = "right"
	case "pgup":
		k = "pageUp"
	case "pgdown":
		k = "pageDown"
	case "home":
		k = "first"
	case "end":
		k = "last"
	default:
		n.count = 0
		return k, 1, false
	}
	n.count = 0
	return k, count, true
}

func (m *Model) navigationKey(k string) (bool, tea.Cmd) {
	if !m.sidebar && m.currentImage() != nil {
		digit := len(k) == 1 && k[0] >= '0' && k[0] <= '9' && (k != "0" || m.navigationState.count > 0)
		switch k {
		case "h", "j", "k", "l", "left", "right", "up", "down":
		default:
			if !digit {
				m.navigationState = navigationState{}
				return false, nil
			}
		}
		action, count, _ := m.navigationState.key(fmt.Sprintf("image:%d", m.activeTab), k)
		switch action {
		case "left":
			m.panImage(-3*count, 0)
		case "right":
			m.panImage(3*count, 0)
		case "up":
			m.panImage(0, -3*count)
		case "down":
			m.panImage(0, 3*count)
		}
		return true, nil
	}
	if !m.sidebar {
		m.navigationState = navigationState{}
		return false, nil
	}
	scope := fmt.Sprintf("%d:%d:%d:%s", m.section, m.activeTab, m.selected, m.repo.Root)
	action, count, handled := m.navigationState.key(scope, k)
	if !handled {
		return false, nil
	}
	if action == "" {
		return true, nil
	}
	if action == "dd" || action == "yy" {
		if m.section == explorer {
			if count != 1 {
				m.notice = m.tr("File operations act on one selected entry")
				return true, nil
			}
			if action == "dd" {
				return true, m.deleteSelectedEntry()
			}
			return true, m.yankEntry()
		}
		return true, nil
	}
	page := max(1, m.bodyHeight()-m.sidebarHeaderRows())
	delta := count
	if action == "pageUp" || action == "pageDown" {
		delta *= page
	}
	switch action {
	case "up", "pageUp":
		m.selected = max(0, m.selected-delta)
	case "down", "pageDown":
		m.selected = min(len(m.items)-1, max(-1, m.selected)+delta)
	case "first":
		m.selected = min(len(m.items)-1, count-1)
	case "last":
		m.selected = len(m.items) - 1
	case "left":
		m.selectParentFolder()
		return true, nil
	case "right":
		return true, m.openSelected()
	}
	m.ensureSidebar()
	return true, m.preview()
}
