package app

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/elegit/internal/theme"
)

// ThemeMode returns the preference, independently of the currently resolved palette.
func (m *Model) ThemeMode() string {
	if m.themeMode == "" {
		return "auto"
	}
	return m.themeMode
}

// SetThemeMode selects an explicit palette or follows terminal color scheme notifications.
func (m *Model) SetThemeMode(mode string) tea.Cmd {
	m.hoverOnly = false
	m.hoverRender = hoverRenderCache{}
	switch mode {
	case "light", "dark":
		m.themeMode = mode
		return m.applyLightMode(mode == "light")
	default:
		m.themeMode = "auto"
		return m.applyLightMode(m.terminalLight)
	}
}

// applyLightMode switches presentation without changing editor or repository state.
func (m *Model) applyLightMode(light bool) tea.Cmd {
	if m.light == light {
		return nil
	}
	m.light = light
	if light {
		m.colors = theme.Light()
	} else {
		m.colors = theme.Dark()
	}
	m.styleInput()
	m.hoverOnly = false
	m.hoverRender = hoverRenderCache{}
	for i := range m.tabs {
		t := &m.tabs[i]
		t.highlightJob.stop()
		t.highlightText, t.spans, t.renderedRows = "", nil, nil
	}
	return tea.Batch(m.scheduleHighlight(), m.highlightDiff())
}

func (m *Model) styleInput() {
	state := textinput.StyleState{
		Text:        theme.Style(m.colors.Foreground, m.colors.Input),
		Prompt:      theme.Style(m.colors.Foreground, m.colors.Input),
		Placeholder: theme.Style(m.colors.Muted, m.colors.Input),
		Suggestion:  theme.Style(m.colors.Muted, m.colors.Input),
	}
	m.input.SetStyles(textinput.Styles{
		Focused: state, Blurred: state,
		Cursor: textinput.CursorStyle{Color: lipgloss.Color(m.colors.Foreground), Shape: tea.CursorBlock, Blink: true},
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
		return nil // unsupported or permanently disabled: no automatic theme updates
	}
}

func (m *Model) terminalColorScheme(light bool) tea.Cmd {
	m.terminalLight = light
	if m.ThemeMode() != "auto" {
		return nil
	}
	return m.applyLightMode(light)
}

// ThemeNotificationRestoreSequence is written after Bubble Tea releases the
// terminal, including on errors and repository switches. Preserve subscriptions
// already enabled by the shell or a parent application.
func (m *Model) ThemeNotificationRestoreSequence() string {
	if m.themeNotificationsOwned {
		return ansi.ResetModeLightDark
	}
	return ""
}
