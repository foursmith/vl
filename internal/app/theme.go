package app

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/theme"
)

// SetColorTheme commits a selection. Preview changes never call this method.
func (m *Model) SetColorTheme(selected *theme.Theme) tea.Cmd {
	m.startupTheme = nil
	if m.appearance.light {
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
	if m.appearance.source == "terminal" && source == "system" {
		return nil
	}
	m.appearance = appearanceState{source: source, light: light}
	if m.modal == "themes" {
		m.modalTitle = m.themePickerTitle()
		m.input.Placeholder = m.modalTitle
	}
	return m.followAppearance()
}

// SetStartupTheme applies the explicit --theme choice for this session.
func (m *Model) SetStartupTheme(selected *theme.Theme) {
	m.startupTheme = selected
	m.applyTheme(selected)
}

func (m *Model) followAppearance() tea.Cmd {
	selected := m.darkTheme
	if m.appearance.light {
		selected = m.lightTheme
	}
	if m.startupTheme != nil {
		selected = m.startupTheme
	}
	if selected == m.committedTheme() {
		return nil
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
		return nil
	}
	return m.applyTheme(selected)
}

func (m *Model) applyTheme(selected *theme.Theme) tea.Cmd {
	if m.colorTheme == selected {
		return nil
	}
	m.colorTheme = selected
	m.colors = selected.Palette()
	m.styleInput()
	m.hoverOnly = false
	m.renderFrame = frameCache{}
	m.pickerFilter = pickerFilterCache{}
	for i := range m.tabs {
		t := &m.tabs[i]
		if d := t.diff; d != nil {
			d.clearSyntax()
		}
		t.clearHighlight()
	}
	return tea.Batch(m.scheduleHighlight(), m.highlightDiff())
}

func (m *Model) styleInput() {
	background := m.colors.Panel
	m.input.styles = inputStyles{
		text:        theme.Style(m.colors.InputForeground, background),
		placeholder: theme.TextStyle(m.colors.InputPlaceholder, background),
		cursor:      tea.Cursor{Color: lipgloss.Color(theme.Composite(m.colors.InputForeground, background)), Shape: tea.CursorBar, Blink: true},
	}
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
		m.showStatusFeedback(m.tr("Imported %d VS Code color themes", msg.count))
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

func (m *Model) themePickerTitle() string {
	if m.appearance.light {
		return m.tr("Select Color Theme for Light Appearance")
	}
	return m.tr("Select Color Theme for Dark Appearance")
}

func (m *Model) openThemePicker() tea.Cmd {
	restore := m.cancelThemePreview()
	m.themeBeforePreview = m.colorTheme
	m.palette = m.themeChoices()
	m.prompt("themes", m.themePickerTitle(), "")
	for i, choice := range m.palette {
		if choice.key == m.tr("Current") {
			m.selectPickerIndex(i)
			break
		}
	}
	return tea.Batch(restore, m.loadImportedThemes())
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

func (m *Model) restoreThemeSlot(id, fallback string) *theme.Theme {
	if id != "" {
		selected, err := theme.Load(id)
		if err == nil {
			return selected
		}
		m.notice = m.tr("Load remembered theme: %s", m.tr(err.Error()))
	}
	return theme.Builtin(fallback)
}
