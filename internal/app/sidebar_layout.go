package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/foursmith/vl/internal/config"
	"github.com/foursmith/vl/internal/editor"
	"github.com/foursmith/vl/internal/i18n"
	"github.com/foursmith/vl/internal/theme"
)

const (
	minSidebarWidth    = 28
	minMainWidth       = 40
	minDockedWidth     = 70
	sidebarLeftPadding = 1
)

func (m *Model) sideShown() bool {
	return !m.editorOnly && m.sidebarVisible && m.width >= minDockedWidth
}

func (m *Model) mainShowsSidebar() bool {
	return m.width < minDockedWidth && m.sidebar && m.sidebarVisible
}

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
func (m *Model) sidebarContentWidth() int {
	if !m.sideShown() {
		return m.width
	}
	return m.sideWidth() - sidebarLeftPadding
}

// sideX is the screen column where sidebar content begins.
func (m *Model) sideX() int {
	if !m.sideShown() {
		return 0
	}
	if m.sidebarRight {
		return m.dividerX() + 1 + sidebarLeftPadding
	}
	return sidebarLeftPadding
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

// Layout captures user preferences independently of the current terminal size.
func (m *Model) Layout() config.Layout {
	var appearance config.Appearance
	if m.darkTheme.ID != theme.DefaultDarkID {
		appearance.Theme.Dark = m.darkTheme.ID
	}
	if m.lightTheme.ID != theme.DefaultLightID {
		appearance.Theme.Light = m.lightTheme.ID
	}
	if m.statusBarStyle != statusBarStandard {
		appearance.StatusBarStyle = m.statusBarStyle
	}
	appearance.Sidebar.Width = m.sidebarWidth
	appearance.Sidebar.Hidden = m.sidebarHidden
	if m.sidebarRight {
		appearance.Sidebar.Side = "right"
	}
	var language string
	if m.displayLanguage != i18n.English {
		language = string(m.displayLanguage)
	}
	return config.Layout{
		Language:              language,
		Appearance:            appearance,
		Vim:                   config.VimPreferences{EnableJieba: m.vimHanDictionary != nil},
		AutoSwitchInputMethod: m.autoSwitchInputMethod,
	}
}

func (m *Model) LastCommand() string      { return m.lastCommand }
func (m *Model) SetLastCommand(id string) { m.lastCommand = id }

func (m *Model) SetLayout(layout config.Layout) {
	m.autoSwitchInputMethod = layout.AutoSwitchInputMethod
	m.vimHanDictionary = nil
	if layout.Vim.EnableJieba {
		m.vimHanDictionary = editor.InstalledHanDictionary
	}
	m.SetDisplayLanguage(layout.Language)
	m.statusBarStyle = statusBarStandard
	appearance := layout.Appearance
	if appearance.StatusBarStyle == statusBarPure {
		m.statusBarStyle = statusBarPure
	}
	m.sidebarHidden = appearance.Sidebar.Hidden
	m.sidebarVisible = !m.editorOnly && !m.sidebarHidden
	if !m.sidebarVisible {
		m.sidebar = false
	}
	m.SetSidebarLayout(appearance.Sidebar.Width, appearance.Sidebar.Side == "right")
	m.darkTheme = m.restoreThemeSlot(appearance.Theme.Dark, theme.DefaultDarkID)
	m.lightTheme = m.restoreThemeSlot(appearance.Theme.Light, theme.DefaultLightID)
	m.followAppearance()
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
	m.sidebarHidden = !m.sidebarVisible
	if m.sidebarVisible {
		m.sidebar = true
	} else {
		m.focusEditor()
	}
	m.resize()
	return m.savePreferences()
}

// SetLayoutStore enables immediate persistence after a preference selection.
// Startup restoration runs before attaching the shared store.
func (m *Model) SetLayoutStore(store *config.LayoutStore) { m.layoutStore = store }

type layoutSavedMsg struct{ err error }

func (m *Model) savePreferences() tea.Cmd {
	if m.layoutStore == nil {
		return nil
	}
	save := m.layoutStore.Prepare(m.Layout())
	return func() tea.Msg { return layoutSavedMsg{err: save()} }
}
