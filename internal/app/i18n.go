package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/foursmith/vl/internal/i18n"
)

func (m *Model) tr(message string, args ...any) string {
	return i18n.Text(m.displayLanguage, message, args...)
}

func searchText(message string, args ...any) string {
	return i18n.Text(i18n.English, message, args...)
}

// SetDisplayLanguage changes presentation without changing command IDs or data.
// Invalid saved values fall back to English.
func (m *Model) SetDisplayLanguage(value string) {
	language, _ := i18n.Parse(value)
	if m.displayLanguage == language {
		return
	}
	loading := m.notice == m.tr("Loading repository…")
	m.displayLanguage = language
	m.hoverOnly = false
	m.renderFrame = frameCache{}
	m.pickerFilter = pickerFilterCache{}
	m.contextMenu = nil
	m.notice = ""
	m.statusFeedback = nil
	m.rebuildItems()
	if loading {
		m.notice = m.tr("Loading repository…")
	}
}

func (m *Model) openLanguagePicker() tea.Cmd {
	m.palette = nil
	for _, choice := range []struct {
		language i18n.Language
		name     string
	}{{i18n.English, "English"}, {i18n.Chinese, "简体中文"}} {
		language := choice.language
		key := ""
		if language == m.displayLanguage {
			key = m.tr("Current")
		}
		m.palette = append(m.palette, action{id: string(language), label: choice.name, key: key, run: func() tea.Cmd {
			m.SetDisplayLanguage(string(language))
			m.showStatusFeedback(m.tr("Display language changed"))
			return m.savePreferences()
		}})
	}
	m.prompt("languages", m.tr("Configure Display Language"), "")
	return nil
}
