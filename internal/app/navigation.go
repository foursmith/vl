package app

import (
	"fmt"
	"sort"

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
			if k == navigationFirstPrefix {
				return "first", count, true
			}
			return k + k, count, true
		}
		// Cancel an unfinished operator, then handle this key normally.
		count = 1
	}
	switch {
	case k == navigationFirstPrefix, k == navigationDeletePrefix, k == navigationCopyPrefix:
		n.pending = k
		return "", 0, true
	case matchesShortcut(k, navigationKeys.Last):
		if n.count > 0 {
			k = "first"
		} else {
			k = "last"
		}
	case matchesShortcut(k, navigationKeys.Down):
		k = "down"
	case matchesShortcut(k, navigationKeys.Up):
		k = "up"
	case matchesShortcut(k, navigationKeys.Left):
		k = "left"
	case matchesShortcut(k, navigationKeys.Right):
		k = "right"
	default:
		k = listNavigationKey(k)
	}
	n.count = 0
	return k, count, k != ""
}

func (m *Model) navigationKey(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if !m.sidebar && m.currentImage() != nil {
		k := msg.String()
		digit := len(k) == 1 && k[0] >= '0' && k[0] <= '9' && (k != "0" || m.navigationState.count > 0)
		if !digit && !matchesShortcut(k, navigationKeys.Left) && !matchesShortcut(k, navigationKeys.Right) &&
			!matchesShortcut(k, navigationKeys.Up) && !matchesShortcut(k, navigationKeys.Down) && !isHalfPageKey(k) {
			m.navigationState = navigationState{}
			return false, nil
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
		case "ctrl+u", "ctrl+d":
			m.panImage(0, pageDelta(action, m.contentHeight())*count)
		}
		return true, nil
	}
	if !m.sidebar {
		m.navigationState = navigationState{}
		return false, nil
	}
	return m.sidebarNavigationKey(msg)
}

func (m *Model) scrollTextHalfPage(k string) {
	d := m.editorDocument()
	if d == nil {
		return
	}
	t := &m.tabs[m.activeTab]
	height := m.contentHeight()
	revealCursor := true
	targetLine := func(line, delta int) int {
		if m.editing() {
			m.syncMarkdownImageRows(t)
			top := t.editorDisplayTop()
			cursor := min(top+height-1, max(top, t.editorDisplayRow(line)))
			t.setEditorDisplayTop(top + delta)
			m.clampEditorScroll(t)
			target, image, _ := t.editorDisplayAt(cursor + delta)
			// An embedded image can fill the viewport without a text row to reveal.
			if image != nil && t.editorDisplayRow(target) < t.editorDisplayTop() {
				next, _, _ := t.editorDisplayAt(image.first + image.region.Rows)
				if next < len(d.Lines()) && t.editorDisplayRow(next) < t.editorDisplayTop()+height {
					return next
				}
				revealCursor = false
			}
			return target
		}
		if len(m.reviewRows) == 0 {
			return line
		}
		top := m.viewport.YOffset()
		cursor := m.reviewRows[min(line, len(m.reviewRows)-1)]
		target := min(top+height-1, max(top, cursor)) + delta
		m.viewport.SetYOffset(top + delta)
		index := sort.SearchInts(m.reviewRows, target)
		if delta < 0 && index > 0 && (index == len(m.reviewRows) || m.reviewRows[index] != target) {
			index--
		}
		return min(index, len(m.reviewRows)-1)
	}
	delta := pageDelta(k, height)
	if m.vimDisabled {
		line, col := d.Position()
		d.SetPosition(targetLine(line, delta), col)
	} else {
		t.vim.ScrollPage(d, delta, targetLine)
	}
	if revealCursor {
		m.ensureTextCursor()
	}
}
