package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/foursmith/vl/internal/config"
	"github.com/foursmith/vl/internal/editor"
	"github.com/foursmith/vl/internal/i18n"
	"github.com/foursmith/vl/internal/theme"
)

// Preferences captures user preferences independently of the current terminal size.
func (m *Model) Preferences() config.Preferences {
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
	var vimMode *bool
	if m.vimDisabled {
		vimMode = new(false)
	}
	return config.Preferences{
		Language:   language,
		Appearance: appearance,
		Features: config.Features{
			VimMode:                 vimMode,
			ChineseWordSegmentation: m.hanDictionary != nil,
			AutoSwitchInputMethod:   m.autoSwitchInputMethod,
		},
	}
}

func (m *Model) SetPreferences(preferences config.Preferences) {
	features := preferences.Features
	m.autoSwitchInputMethod = features.AutoSwitchInputMethod
	m.setVimDisabled(!features.VimEnabled())
	m.hanDictionary = nil
	if features.ChineseWordSegmentation {
		m.hanDictionary = editor.NewInstalledHanDictionaryLoader()
	}
	m.SetDisplayLanguage(preferences.Language)
	m.statusBarStyle = statusBarStandard
	appearance := preferences.Appearance
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

// SetPreferencesStore enables immediate persistence after a preference selection.
// Startup restoration runs before attaching the shared store.
func (m *Model) SetPreferencesStore(store *config.PreferencesStore) { m.preferencesStore = store }

type preferencesSavedMsg struct{ err error }

func (m *Model) savePreferences() tea.Cmd {
	if m.preferencesStore == nil {
		return nil
	}
	save := m.preferencesStore.Prepare(m.Preferences())
	return func() tea.Msg { return preferencesSavedMsg{err: save()} }
}

func (m *Model) openConfig() tea.Cmd {
	var path string
	var err error
	if m.preferencesStore != nil {
		path, err = m.preferencesStore.EnsureFile()
	} else {
		path, err = config.PreferencesPath()
		if err == nil {
			err = config.EnsurePreferencesFile(path)
		}
	}
	if err != nil {
		m.notice = m.tr("Open configuration file: %s", m.tr(err.Error()))
		return nil
	}
	return m.openFile(path)
}
