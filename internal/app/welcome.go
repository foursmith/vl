package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/vl/internal/theme"
)

type welcomeAction struct {
	label, key, command string
}

var welcomeActions = []welcomeAction{
	{"Show All Commands", workbenchKeys.Commands.Help().Key, "commands"},
	{"Go to File", workbenchKeys.Files.Help().Key, "files"},
	{"Open Recent Workspace", workbenchKeys.Recent.Help().Key, "recent"},
}

func (m *Model) welcomeVisible() bool {
	return !m.editing() && m.currentImage() == nil && !m.diffOpen && !m.diffLoading && m.patch.Raw == ""
}

func (m *Model) welcomeArtWidth() int { return min(94, max(1, m.mainWidth()-4)) }

func (m *Model) welcomeCellAspect() float64 {
	return min(.85, max(.3, float64(m.cellWidth)/float64(max(1, m.cellHeight))))
}

// Rendering and clicks share the same centered block, including compact mode.
func (m *Model) welcomeLayout() (x, y, width, logoRows int) {
	minimumWidth := 2
	for _, a := range welcomeActions {
		minimumWidth = max(minimumWidth, ansi.StringWidth(m.tr(a.label))+1+ansi.StringWidth(a.key))
	}
	width = min(max(42, minimumWidth), max(1, m.mainWidth()-4))
	width = min(max(width, minimumWidth), max(1, m.mainWidth()))
	artWidth := m.welcomeArtWidth()
	artRows := min(29, m.contentHeight()-len(welcomeActions)-4, int(float64(artWidth)*m.welcomeCellAspect()*.72))
	logoRows = 1
	if artWidth >= 28 && artRows >= 7 {
		logoRows = artRows
	}
	height := logoRows + m.welcomeGap(logoRows) + len(welcomeActions)
	x = max(0, (m.mainWidth()-width)/2)
	y = max(0, (m.contentHeight()-height)/2)
	return
}

func (m *Model) welcomeGap(logoRows int) int {
	if logoRows > 1 {
		return 2
	}
	if m.contentHeight() < logoRows+1+len(welcomeActions) {
		return 0
	}
	return 1
}

func (m *Model) welcomeLines() []string {
	x, y, width, logoRows := m.welcomeLayout()
	lines := make([]string, m.contentHeight())
	if logoRows > 1 {
		artWidth := m.welcomeArtWidth()
		indent := strings.Repeat(" ", (m.mainWidth()-artWidth)/2)
		for i, line := range m.renderWelcomeArt(artWidth, logoRows) {
			lines[y+i] = indent + line
		}
		hint := m.tr("Drag to rotate")
		lines[y+logoRows] = strings.Repeat(" ", max(0, (m.mainWidth()-ansi.StringWidth(hint))/2)) +
			theme.Style(m.colors.Muted, m.colors.Background).Render(hint)
	} else {
		lines[y] = strings.Repeat(" ", x+(width-2)/2) + theme.Style(m.colors.Brand, m.colors.Background).Render("vl")
	}
	for i, a := range welcomeActions {
		actionRow := y + logoRows + m.welcomeGap(logoRows) + i
		if actionRow >= len(lines) {
			break
		}
		key := ansi.Truncate(a.key, width, "")
		label := ansi.Truncate(m.tr(a.label), max(0, width-ansi.StringWidth(key)-1), "…")
		padding := width - ansi.StringWidth(label) - ansi.StringWidth(key)
		lines[actionRow] = strings.Repeat(" ", x) +
			theme.Style(m.colors.Muted, m.colors.Background).Render(label) + strings.Repeat(" ", padding) +
			theme.TextStyle(m.colors.InputForeground, m.colors.Input).Render(key)
	}
	return lines
}

func (m *Model) clickWelcome(msg tea.MouseClickMsg) tea.Cmd {
	x, y, width, logoRows := m.welcomeLayout()
	artWidth := m.welcomeArtWidth()
	artX := m.mainX() + (m.mainWidth()-artWidth)/2
	artY := m.contentRow() + y
	if logoRows > 1 && msg.X >= artX && msg.X < artX+artWidth && msg.Y >= artY && msg.Y < artY+logoRows {
		m.welcomeOrbit.dragging = true
		m.welcomeOrbit.x, m.welcomeOrbit.y = msg.X, msg.Y
		return nil
	}
	x += m.mainX()
	index := msg.Y - m.contentRow() - y - logoRows - m.welcomeGap(logoRows)
	if msg.X < x || msg.X >= x+width || index < 0 || index >= len(welcomeActions) {
		return nil
	}
	switch welcomeActions[index].command {
	case "commands":
		return m.openPalette()
	case "files":
		return m.openQuickFiles()
	case "recent":
		return m.openRecentWorkspaces()
	}
	return nil
}

func (m *Model) dragWelcomeArt(x, y int) {
	o := &m.welcomeOrbit
	if m.modal != "" || !m.welcomeVisible() || m.width < 30 || m.height < 8 || m.width < 70 && m.sidebar && m.sidebarVisible {
		o.dragging = false
		return
	}
	o.rotate(float64(x-o.x)*.065, float64(o.y-y)*.065/m.welcomeCellAspect())
	o.x, o.y = x, y
}

func (m *Model) rotateWelcomeArt(key string) bool {
	if m.sidebar || !m.welcomeVisible() {
		return false
	}
	_, _, _, logoRows := m.welcomeLayout()
	if logoRows <= 1 {
		return false
	}
	switch key {
	case "left":
		m.welcomeOrbit.rotate(-.16, 0)
	case "right":
		m.welcomeOrbit.rotate(.16, 0)
	case "up":
		m.welcomeOrbit.rotate(0, .16)
	case "down":
		m.welcomeOrbit.rotate(0, -.16)
	default:
		return false
	}
	return true
}
