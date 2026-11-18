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

func (m *Model) navigationKey(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if !m.sidebar && m.currentImage() != nil {
		k := msg.String()
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
	return m.sidebarNavigationKey(msg)
}
