package app

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/vl/internal/theme"
)

// SetColorTheme commits a selection. Preview changes never call this method.
func (m *Model) SetColorTheme(selected *theme.Theme) tea.Cmd {
	m.themePinned = m.appearance.source == "" || selected.IsLight() != m.appearance.light
	if selected.IsLight() {
		m.lightTheme = selected
	} else {
		m.darkTheme = selected
	}
	return tea.Batch(m.applyTheme(selected), m.savePreferences())
}

func (m *Model) committedTheme() *theme.Theme {
	if m.themeBeforePreview != nil {
		return m.themeBeforePreview
	}
	return m.colorTheme
}

// Appearance records the currently authoritative source for this terminal.
type appearanceState struct {
	source string
	light  bool
}

func (m *Model) externalColorScheme(light bool, source string) tea.Cmd {
	previous := m.appearance
	if previous.source == "terminal" && source == "system" {
		return nil
	}
	m.appearance = appearanceState{source: source, light: light}
	aligned := m.committedTheme().IsLight() == light
	if m.themePinned {
		// The first system sample can arrive before a terminal with a
		// different appearance replies. Let that terminal resolve the pin.
		if !aligned || (source == "system" && previous.source == "") {
			return nil
		}
		m.themePinned = false
		return m.savePreferences()
	}
	if aligned {
		return nil
	}
	selected := m.darkTheme
	if light {
		selected = m.lightTheme
	}
	if m.themeBeforePreview != nil {
		m.pickerFilter = pickerFilterCache{}
		m.themeBeforePreview = selected
		for i := range m.palette {
			m.palette[i].key = ""
			if m.palette[i].id == selected.ID {
				m.palette[i].key = m.tr("Current")
			}
		}
		return m.savePreferences()
	}
	return tea.Batch(m.applyTheme(selected), m.savePreferences())
}

func (m *Model) applyTheme(selected *theme.Theme) tea.Cmd {
	if m.colorTheme == selected {
		return nil
	}
	m.colorTheme = selected
	m.colors = selected.Palette()
	m.styleInput()
	m.hoverOnly = false
	m.hoverRender = hoverRenderCache{}
	m.pickerFilter = pickerFilterCache{}
	for i := range m.tabs {
		t := &m.tabs[i]
		if d := t.diff; d != nil {
			if d.diffSyntaxCancel != nil {
				d.diffSyntaxCancel()
				d.diffSyntaxCancel = nil
			}
			d.diffSyntax = nil
		}
		t.clearHighlight()
	}
	return tea.Batch(m.scheduleHighlight(), m.highlightDiff())
}

func (m *Model) styleInput() {
	state := textinput.StyleState{
		Text:        theme.Style(m.colors.InputForeground, m.colors.Input),
		Prompt:      theme.Style(m.colors.InputForeground, m.colors.Input),
		Placeholder: theme.TextStyle(m.colors.InputPlaceholder, m.colors.Input),
		Suggestion:  theme.TextStyle(m.colors.InputPlaceholder, m.colors.Input),
	}
	m.input.SetStyles(textinput.Styles{
		Focused: state, Blurred: state,
		Cursor: textinput.CursorStyle{Color: lipgloss.Color(theme.Composite(m.colors.InputForeground, m.colors.Input)), Shape: tea.CursorBar, Blink: true},
	})
}

// Probe before enabling so the caller can restore only the mode we changed.
func (m *Model) themeNotificationMode(msg tea.ModeReportMsg) tea.Cmd {
	if msg.Mode != ansi.ModeLightDark || m.themeNotifications {
		return nil
	}
	switch msg.Value {
	case ansi.ModeReset:
		m.themeNotifications, m.themeNotificationsOwned = true, true
		return tea.Raw(ansi.SetModeLightDark + ansi.RequestLightDarkReport)
	case ansi.ModeSet, ansi.ModePermanentlySet:
		m.themeNotifications = true
		return tea.Raw(ansi.RequestLightDarkReport)
	default:
		return nil // unsupported or permanently disabled; system fallback remains available
	}
}

// ThemeNotificationRestoreSequence is written after Bubble Tea releases the
// terminal, including on errors. Preserve subscriptions
// already enabled by the shell or a parent application.
func (m *Model) ThemeNotificationRestoreSequence() string {
	if m.themeNotificationsOwned {
		return ansi.ResetModeLightDark
	}
	return ""
}

type importedThemesMsg struct {
	seq    uint64
	themes []*theme.Theme
	err    error
}

const themeImportID = "theme.import"

type themesImportedMsg struct {
	count int
	err   error
}

func (m *Model) importThemes() tea.Cmd {
	if m.themeImporting {
		return nil
	}
	m.themeImporting = true
	m.refreshThemeChoices()
	return tea.Batch(m.applyTheme(m.committedTheme()), func() tea.Msg {
		dir, err := theme.ImportDir()
		if err != nil {
			return themesImportedMsg{err: err}
		}
		sources, err := theme.VSCodeExtensionDirs()
		if err != nil {
			return themesImportedMsg{err: err}
		}
		count, err := theme.ImportVSCode(dir, sources)
		return themesImportedMsg{count: count, err: err}
	})
}

func (m *Model) finishThemeImport(msg themesImportedMsg) tea.Cmd {
	m.themeImporting = false
	m.refreshThemeChoices()
	switch {
	case msg.err == nil:
		m.notice = m.tr("Imported %d VS Code color themes", msg.count)
	case msg.count > 0:
		m.notice = m.tr("Imported %d VS Code color themes with errors: %s", msg.count, m.tr(msg.err.Error()))
	default:
		m.notice = m.tr("Import VS Code themes: %s", m.tr(msg.err.Error()))
	}
	return m.loadImportedThemes()
}

func (m *Model) loadImportedThemes() tea.Cmd {
	m.importedThemesSeq++
	seq := m.importedThemesSeq
	return func() tea.Msg {
		dir, err := theme.ImportDir()
		if err != nil {
			return importedThemesMsg{seq: seq, err: err}
		}
		themes, err := theme.DiscoverImports(dir)
		return importedThemesMsg{seq: seq, themes: themes, err: err}
	}
}

func (m *Model) applyImportedThemes(msg importedThemesMsg) {
	if msg.seq != m.importedThemesSeq {
		return
	}
	m.importedThemes = msg.themes
	if msg.err != nil {
		m.notice = m.tr("Load imported themes: %s", m.tr(msg.err.Error()))
	}
	m.refreshThemeChoices()
}

func (m *Model) refreshThemeChoices() {
	if m.modal != "themes" {
		return
	}
	selected := ""
	rows := m.filteredActions()
	if m.pickerIndex >= 0 && m.pickerIndex < len(rows) {
		selected = rows[m.pickerIndex].id
	}
	m.palette = m.themeChoices()
	for i, row := range m.filteredActions() {
		if row.id == selected {
			m.pickerIndex = i
			break
		}
	}
	m.syncPicker()
}

func (m *Model) themeChoices() []action {
	current := m.committedTheme()

	importAction := action{id: themeImportID, label: m.tr("+ Import Themes from VS Code"), run: m.importThemes}
	if m.themeImporting {
		importAction.key = m.tr("Importing…")
	}
	choices := []action{importAction}
	available := make([]*theme.Theme, 0, len(theme.BuiltinIDs)+len(m.importedThemes))
	for _, id := range theme.BuiltinIDs {
		available = append(available, theme.Builtin(id))
	}
	available = append(available, m.importedThemes...)
	// Keep the active theme selectable during discovery.
	if !slices.ContainsFunc(available, func(t *theme.Theme) bool { return t.ID == current.ID }) {
		available = append(available, current)
	}
	slices.SortStableFunc(available, func(a, b *theme.Theme) int {
		if n := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); n != 0 {
			return n
		}
		return strings.Compare(a.ID, b.ID)
	})
	groups := []struct{ kind, label string }{
		{"vs-dark", m.tr("Dark Themes")},
		{"vs", m.tr("Light Themes")},
		{"hc-black", m.tr("Dark High Contrast Themes")},
		{"hc-light", m.tr("Light High Contrast Themes")},
	}
	for _, group := range groups {
		start := len(choices)
		for _, selected := range available {
			if selected.Type != group.kind {
				continue
			}
			if len(choices) == start {
				choices = append(choices, action{label: group.label})
			}
			choice := action{id: selected.ID, label: selected.Name, run: func() tea.Cmd { return m.chooseTheme(selected) }}
			if current.ID == selected.ID {
				choice.key = m.tr("Current")
			}
			choices = append(choices, choice)
		}
	}
	return choices
}

func (m *Model) openThemePicker() tea.Cmd {
	restore := m.cancelThemePreview()
	m.themeBeforePreview = m.colorTheme
	m.palette = m.themeChoices()
	cmd := m.prompt("themes", m.tr("Select Color Theme"), "")
	for i, choice := range m.palette {
		if choice.key == m.tr("Current") {
			m.selectPickerIndex(i)
			break
		}
	}
	return tea.Batch(restore, cmd, m.loadImportedThemes())
}

// Preferences remain unchanged until Enter; only presentation follows the
// highlighted choice. The snapshot also survives an asynchronous catalog refresh.
func (m *Model) chooseTheme(selected *theme.Theme) tea.Cmd {
	if m.themeBeforePreview != nil {
		return m.applyTheme(selected)
	}
	return m.SetColorTheme(selected)
}

func (m *Model) previewSelectedTheme() tea.Cmd {
	if m.modal != "themes" || m.themeBeforePreview == nil {
		return nil
	}
	rows := m.filteredActions()
	if len(rows) == 0 {
		return nil
	}
	a := rows[m.pickerIndex]
	if a.id == themeImportID {
		return m.applyTheme(m.committedTheme())
	}
	if a.run == nil {
		return nil
	}
	return a.run()
}

func (m *Model) cancelThemePreview() tea.Cmd {
	preview := m.themeBeforePreview
	if preview == nil {
		return nil
	}
	m.themeBeforePreview = nil
	return m.applyTheme(preview)
}

func (m *Model) restoreThemeSlot(id string, light bool) bool {
	if id == "" {
		return false
	}
	selected, err := theme.Load(id)
	if err != nil {
		m.notice = m.tr("Load remembered theme: %s", m.tr(err.Error()))
		return false
	}
	if selected.IsLight() != light {
		m.notice = m.tr("Remembered theme has changed appearance: %s", id)
		return false
	}
	if light {
		m.lightTheme = selected
	} else {
		m.darkTheme = selected
	}
	return true
}
