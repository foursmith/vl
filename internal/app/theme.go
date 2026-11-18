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
		Placeholder: theme.Style(m.colors.InputPlaceholder, m.colors.Input),
		Suggestion:  theme.Style(m.colors.InputPlaceholder, m.colors.Input),
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

type localThemesMsg struct {
	seq    uint64
	themes []*theme.Theme
	err    error
}

func (m *Model) loadLocalThemes() tea.Cmd {
	m.localThemesSeq++
	seq := m.localThemesSeq
	return func() tea.Msg {
		dir, err := theme.LocalDir()
		if err != nil {
			return localThemesMsg{seq: seq, err: err}
		}
		themes, err := theme.Discover(dir)
		return localThemesMsg{seq: seq, themes: themes, err: err}
	}
}

func (m *Model) applyLocalThemes(msg localThemesMsg) {
	if msg.seq != m.localThemesSeq {
		return
	}
	m.localThemes = msg.themes
	if msg.err != nil {
		m.notice = m.tr("Load local themes: %s", m.tr(msg.err.Error()))
	}
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

	var choices []action
	available := make([]*theme.Theme, 0, len(theme.BuiltinIDs)+len(m.localThemes))
	for _, id := range theme.BuiltinIDs {
		available = append(available, theme.Builtin(id))
	}
	available = append(available, m.localThemes...)
	// Keep the active local theme selectable while the first directory scan runs.
	if strings.HasPrefix(current.ID, "local:") && !slices.ContainsFunc(available, func(t *theme.Theme) bool { return t.ID == current.ID }) {
		available = append(available, current)
	}
	slices.SortStableFunc(available, func(a, b *theme.Theme) int {
		if a.IsLight() != b.IsLight() {
			if a.IsLight() {
				return 1
			}
			return -1
		}
		if n := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); n != 0 {
			return n
		}
		return strings.Compare(a.ID, b.ID)
	})
	for _, selected := range available {
		kind := m.tr("Dark")
		if selected.IsLight() {
			kind = m.tr("Light")
		}
		label := kind + " · " + selected.Name
		if strings.HasPrefix(selected.ID, "local:") {
			label += m.tr(" · Local")
		}
		choice := action{id: selected.ID, label: label, run: func() tea.Cmd { return m.chooseTheme(selected) }}
		if current.ID == selected.ID {
			choice.key = m.tr("Current")
		}
		choices = append(choices, choice)
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
	return tea.Batch(restore, cmd, m.loadLocalThemes())
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
	return rows[m.pickerIndex].run()
}

func (m *Model) cancelThemePreview() tea.Cmd {
	preview := m.themeBeforePreview
	if preview == nil {
		return nil
	}
	m.themeBeforePreview = nil
	return m.applyTheme(preview)
}

func (m *Model) restoreThemeSlot(id, kind string, light bool) {
	if id == "" {
		return
	}
	selected, err := theme.Load(id, kind)
	if err != nil {
		m.notice = m.tr("Load remembered theme: %s", m.tr(err.Error()))
		return
	}
	if selected.IsLight() != light {
		m.notice = m.tr("Remembered theme has changed appearance: %s", id)
		return
	}
	if light {
		m.lightTheme = selected
	} else {
		m.darkTheme = selected
	}
}
