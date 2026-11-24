package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/theme"
)

type explorerPanelID uint8

const (
	explorerPanelAuto explorerPanelID = iota
	explorerPanelNone
	explorerPanelOutline
	explorerPanelTimeline
	explorerPanelEnd
)

const explorerPanelCount = explorerPanelEnd - explorerPanelOutline

type explorerPanelSpec struct {
	id        explorerPanelID
	title     string
	available func(*Model) bool
	focused   func(*Model) bool
	list      func(*Model) sidebarList
	render    func(*Model, []string, int, sidebarList) []string
	click     func(*Model, int, sidebarList) tea.Cmd
	blur      func(*Model)
}

// Register panels in their display order. The group owns their headers,
// viewport geometry and the single expanded panel.
func explorerPanelSpecs() [explorerPanelCount]explorerPanelSpec {
	return [...]explorerPanelSpec{
		{
			id: explorerPanelOutline, title: "Outline",
			available: (*Model).outlineAvailable, focused: (*Model).outlinePanelFocused,
			list: (*Model).outlineList, render: (*Model).appendOutline,
			click: (*Model).clickOutline, blur: (*Model).blurOutline,
		},
		{
			id: explorerPanelTimeline, title: "Timeline",
			available: (*Model).fileHistoryAvailable, focused: (*Model).fileHistoryPanelFocused,
			list: (*Model).fileHistoryList, render: (*Model).appendFileHistory,
			click: (*Model).clickFileHistory, blur: (*Model).blurFileHistory,
		},
	}
}

type explorerPanel struct {
	spec   explorerPanelSpec
	header int
	list   sidebarList
}

type explorerPanelGroup struct {
	panels [explorerPanelCount]explorerPanel
	count  int
}

func (m *Model) explorerPanels() explorerPanelGroup {
	end := m.bodyHeight()
	room := end - headerHeight
	var group explorerPanelGroup
	if m.section != explorer || m.editorOnly || room < 4 {
		return group
	}
	active := m.explorerPanel
	activeVisible := false
	for _, spec := range explorerPanelSpecs() {
		if spec.available(m) {
			group.panels[group.count].spec = spec
			group.count++
			activeVisible = activeVisible || spec.id == active
		}
	}
	if group.count == 0 || group.count >= room {
		return explorerPanelGroup{}
	}
	if !activeVisible && active != explorerPanelNone {
		active = group.panels[0].spec.id
	}
	height := min(room/3, room-int(explorerPanelCount)-1)
	if active != explorerPanelNone && height > 0 {
		// Give unused header rows to the expanded list without moving the group.
		height += int(explorerPanelCount) - group.count
	}
	for i := group.count - 1; i >= 0; i-- {
		panel := &group.panels[i]
		if panel.spec.id == active && height > 0 {
			panel.list = panel.spec.list(m)
			panel.list.height = height
			panel.list.start = end - height
		}
		panel.header = end - panel.list.height - 1
		end = panel.header
	}
	return group
}

func (m *Model) explorerPanelLayout(id explorerPanelID) (int, sidebarList) {
	group := m.explorerPanels()
	for _, panel := range group.panels[:group.count] {
		if panel.spec.id == id {
			return panel.header, panel.list
		}
	}
	return m.bodyHeight(), sidebarList{}
}

func (m *Model) explorerPanelAt(y int) (explorerPanel, bool) {
	group := m.explorerPanels()
	for _, panel := range group.panels[:group.count] {
		if y >= panel.header && y <= panel.header+panel.list.height {
			return panel, true
		}
	}
	return explorerPanel{}, false
}

func (m *Model) explorerPanelHoverY() int {
	if m.modal == "" && m.contextMenu == nil && !m.resizingSidebar &&
		m.hoverX >= 0 && m.sidebarAt(m.hoverX) {
		return m.hoverY
	}
	return -1
}

func (m *Model) hoveredExplorerPanelRow() int {
	y := m.explorerPanelHoverY()
	if y >= headerHeight {
		if _, ok := m.explorerPanelAt(y); ok {
			return y
		}
	}
	return -1
}

func (m *Model) setExplorerPanel(id explorerPanelID) {
	m.explorerPanel = id
	for _, spec := range explorerPanelSpecs() {
		if spec.id != id {
			spec.blur(m)
		}
	}
	m.clampSidebarScroll()
}

func (m *Model) clickExplorerPanel(panel explorerPanel, y int) tea.Cmd {
	m.sidebar = true
	if y != panel.header {
		if y-panel.list.start < panel.list.count {
			return panel.spec.click(m, y, panel.list)
		}
		return nil
	}
	m.lastClickKey = ""
	if panel.list.height > 0 {
		m.setExplorerPanel(explorerPanelNone)
	} else {
		m.setExplorerPanel(panel.spec.id)
	}
	return nil
}

func (m *Model) explorerPanelHeader(panel explorerPanel, width int) string {
	bg := m.colors.Panel
	prefix := m.sidebarHeaderIndent()
	if m.sidebar && panel.spec.focused(m) && panel.list.count == 0 {
		bg = m.colors.SidebarSelection(true)
		if m.monochrome {
			prefix = ">"
		}
	} else if m.explorerPanelHoverY() == panel.header {
		bg = m.colors.Hover
	}
	title := prefix + m.treeChevron(panel.list.height > 0) + " " + m.tr(panel.spec.title)
	fg := theme.Readable(m.colors.Strong, bg)
	rule := strings.Repeat("─", max(0, width-ansi.StringWidth(title)-2))
	text := theme.TextStyle(fg, bg).Bold(true).Render(title) + " " +
		theme.Style(theme.WithOpacity(m.colors.Muted, .45), bg).Render(rule)
	return row(text, width, fg, bg)
}
