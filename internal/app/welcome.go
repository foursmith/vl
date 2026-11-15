package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/elegit/internal/theme"
)

// Block lettering uses a shared cell grid so every row stays aligned.
var welcomeLogo = []string{
	"██████  ██      ██████   ████   ██████  ██████",
	"██      ██      ██      ██  ██    ██      ██  ",
	"██      ██      ██      ██        ██      ██  ",
	"█████   ██      █████   ██ ███    ██      ██  ",
	"██      ██      ██      ██  ██    ██      ██  ",
	"██      ██      ██      ██  ██    ██      ██  ",
	"██████  ██████  ██████   ████   ██████    ██  ",
}

func welcomeLogoWidth() int {
	width := 0
	for _, line := range welcomeLogo {
		width = max(width, ansi.StringWidth(line))
	}
	return width
}

type welcomeAction struct {
	label, key, command string
}

var welcomeActions = []welcomeAction{
	{"Show All Commands", "F1", "commands"},
	{"Go to File", "Ctrl+P", "files"},
	{"Open Recent Folder", "Ctrl+R", "recent"},
	{"Changes", "Ctrl+2", "changes"},
}

func (m *Model) welcomeVisible() bool {
	return !m.editing() && !m.diffOpen && !m.diffLoading && m.patch.Raw == ""
}

// Rendering and clicks share the same centered block, including compact mode.
func (m *Model) welcomeLayout() (x, y, width, logoRows int) {
	width = min(max(42, welcomeLogoWidth()), max(1, m.mainWidth()-4))
	logoRows = 1
	if width >= welcomeLogoWidth() && m.contentHeight() >= len(welcomeLogo)+len(welcomeActions)+3 {
		logoRows = len(welcomeLogo)
	}
	height := logoRows + m.welcomeGap(logoRows) + len(welcomeActions)
	x = max(0, (m.mainWidth()-width)/2)
	y = max(0, (m.contentHeight()-height)/2)
	return
}

func (m *Model) welcomeGap(logoRows int) int {
	if m.contentHeight() < logoRows+1+len(welcomeActions) {
		return 0
	}
	return 1
}

func (m *Model) welcomeLines() []string {
	x, y, width, logoRows := m.welcomeLayout()
	lines := make([]string, m.contentHeight())
	logo := []string{"ELEGIT"}
	if logoRows > 1 {
		logo = welcomeLogo
	}
	logoWidth := ansi.StringWidth(logo[0])
	if logoRows > 1 {
		logoWidth = welcomeLogoWidth()
	}
	padding := max(0, (width-logoWidth)/2)
	for i, text := range logo {
		lines[y+i] = strings.Repeat(" ", x+padding) + theme.Style(m.colors.Brand, m.colors.Background).Render(text)
	}
	for i, a := range welcomeActions {
		padding := max(2, width-ansi.StringWidth(a.label)-ansi.StringWidth(a.key))
		lines[y+logoRows+m.welcomeGap(logoRows)+i] = strings.Repeat(" ", x) +
			theme.Style(m.colors.Muted, m.colors.Background).Render(a.label) + strings.Repeat(" ", padding) +
			theme.Style(m.colors.Foreground, m.colors.Border).Render(a.key)
	}
	return lines
}

func (m *Model) clickWelcome(msg tea.MouseClickMsg) tea.Cmd {
	x, y, width, logoRows := m.welcomeLayout()
	x += m.mainX()
	index := msg.Y - editorContentRow - y - logoRows - m.welcomeGap(logoRows)
	if msg.X < x || msg.X >= x+width || index < 0 || index >= len(welcomeActions) {
		return nil
	}
	switch welcomeActions[index].command {
	case "commands":
		return m.openPalette()
	case "files":
		return m.openQuickFiles()
	case "recent":
		return m.openRecentFolders()
	case "changes":
		return m.selectSection(changes)
	}
	return nil
}
