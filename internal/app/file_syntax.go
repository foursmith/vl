package app

import (
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/foursmith/vl/internal/config"
	"github.com/foursmith/vl/internal/highlight"
)

func (m *Model) restoreFileSyntax(t *tab) {
	if t.image != nil {
		return
	}
	path := t.path()
	if !filepath.IsAbs(path) {
		path = filepath.Join(m.repo.Root, path)
	}
	record, err := config.FileSyntaxPath(path)
	if err != nil {
		m.notice = m.tr("Load syntax highlighting: %s", err.Error())
		return
	}
	t.syntaxRecord = record
	id, err := config.LoadFileSyntax(record)
	if err != nil {
		m.notice = m.tr("Load syntax highlighting: %s", err.Error())
		return
	}
	t.syntax = highlight.LanguageByID(id)
}

func (m *Model) canSetFileSyntax() bool {
	return m.activeTab >= 0 && m.activeTab < len(m.tabs) &&
		m.tabs[m.activeTab].syntaxRecord != "" && m.editorDocument() != nil
}

func (m *Model) openFileSyntaxPicker() tea.Cmd {
	if !m.canSetFileSyntax() {
		return nil
	}
	t := &m.tabs[m.activeTab]
	record := t.syntaxRecord
	languages := highlight.Languages()
	slices.SortFunc(languages[1:], func(a, b highlight.LanguageInfo) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	languages = append([]highlight.LanguageInfo{{}}, languages...)
	m.palette = make([]action, 0, len(languages))
	current := 0
	for i, language := range languages {
		label := language.Name
		if language.ID == "" {
			label = "Auto Detect"
		}
		key := ""
		if language.ID == t.syntax.ID {
			key = m.tr("Current")
			current = i
		}
		m.palette = append(m.palette, action{id: language.ID, label: m.tr(label), key: key, run: func() tea.Cmd {
			m.setFileSyntax(record, language)
			return nil
		}})
	}
	m.prompt("fileSyntax", m.tr("Syntax Highlighting"), "")
	m.selectPickerIndex(current)
	return nil
}

func (m *Model) setFileSyntax(record string, language highlight.LanguageInfo) {
	// Save before applying so an immediate quit retains the selection and
	// a failed write leaves the visible setting consistent with disk.
	if err := config.SaveFileSyntax(record, language.ID); err != nil {
		m.notice = m.tr("Save syntax highlighting: %s", err.Error())
		return
	}
	for i := range m.tabs {
		t := &m.tabs[i]
		if t.syntaxRecord != record || t.syntax == language {
			continue
		}
		t.syntax = language
		if t.diff != nil {
			t.diff.clearSyntax()
		} else {
			t.clearHighlight()
		}
	}
}

func (m *Model) activeSyntax() highlight.LanguageInfo {
	if m.activeTab >= 0 && m.activeTab < len(m.tabs) {
		return m.tabs[m.activeTab].syntax
	}
	return highlight.LanguageInfo{}
}

func (t *tab) documentLanguage() highlight.LanguageInfo {
	if t.syntax.ID != "" {
		return t.syntax
	}
	return highlight.Language(t.doc.Path).WithShebang(t.doc.Text)
}

// Cache filename detection for the footer.
func (m *Model) language(path string) highlight.LanguageInfo {
	if language := m.activeSyntax(); language.ID != "" {
		return language
	}
	if m.languagePath != path || m.languageInfo.ID == "" {
		m.languagePath = path
		m.languageInfo = highlight.Language(path)
	}
	if d := m.document(); d != nil {
		return m.languageInfo.WithShebang(d.Text)
	}
	return m.languageInfo
}
