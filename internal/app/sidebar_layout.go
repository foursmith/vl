package app

import tea "charm.land/bubbletea/v2"

const (
	minSidebarWidth = 28
	minMainWidth    = 40
)

func (m *Model) sideShown() bool { return m.sidebarVisible && m.width >= 70 }
func (m *Model) sideWidth() int {
	if !m.sideShown() {
		return 0
	}
	w := m.sidebarWidth
	if w == 0 {
		w = min(44, max(minSidebarWidth, m.width/4))
	}
	return m.clampSidebarWidth(w)
}
func (m *Model) sideX() int {
	if m.sideShown() && m.sidebarRight {
		return m.mainWidth() + 1
	}
	return 0
}
func (m *Model) dividerX() int {
	if m.sidebarRight {
		return m.mainWidth()
	}
	return m.sideWidth()
}
func (m *Model) mainX() int {
	if m.sideShown() && !m.sidebarRight {
		return 1 + m.sideWidth()
	}
	return 0
}
func (m *Model) mainWidth() int {
	if m.sideShown() {
		return max(1, m.width-m.sideWidth()-1)
	}
	return max(1, m.width)
}
func (m *Model) clampSidebarWidth(width int) int {
	return min(max(minSidebarWidth, width), m.width-minMainWidth-1)
}

func (m *Model) sidebarDividerAt(x, y int) bool {
	return m.modal == "" && m.sideShown() && x == m.dividerX() && y >= 0 && y < m.bodyHeight()
}

// SetSidebarLayout restores the preferred width in terminal cells and side.
// A zero width follows the terminal size automatically.
func (m *Model) SetSidebarLayout(width int, right bool) {
	if width > 0 {
		width = max(minSidebarWidth, width)
	} else {
		width = 0
	}
	m.sidebarWidth, m.sidebarRight = width, right
	m.resizingSidebar = false
	m.hoverX, m.hoverY = -1, -1
	m.resize()
}

func (m *Model) SidebarLayout() (width int, right bool) {
	return m.sidebarWidth, m.sidebarRight
}

func (m *Model) dragSidebar(x int) {
	w := x
	if m.sidebarRight {
		w = m.width - x - 1
	}
	w = m.clampSidebarWidth(w)
	if w != m.sideWidth() {
		m.sidebarWidth = w
		m.resize()
	}
}

func (m *Model) toggleSidebar() tea.Cmd {
	m.stopTextDrag()
	m.resizingSidebar = false
	m.hoverX, m.hoverY = -1, -1
	m.sidebarVisible = !m.sidebarVisible
	m.sidebar = m.sidebarVisible
	m.resize()
	return nil
}
