package app

import (
	"context"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/foursmith/vl/internal/editor"
	gitservice "github.com/foursmith/vl/internal/git"
)

// NewEditor opens a single text file without repository discovery or workspace state.
func NewEditor(ctx context.Context, path string, nerd bool) (*Model, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	doc, err := editor.Open(abs)
	if err != nil {
		return nil, err
	}
	m := newModel(ctx, &gitservice.Repository{Root: filepath.Dir(doc.Path)}, nerd, true)
	m.activateTab(m.insertTab(tab{doc: doc}))
	return m, nil
}

// Shared editing/preferences commands; standalone mode adds no workspace commands.
func (m *Model) editorActions() []action {
	docReason := ""
	if !m.editing() {
		docReason = m.tr("Open a text file first")
	}
	syntaxReason := ""
	if !m.canSetFileSyntax() {
		syntaxReason = m.tr("Open a text file first")
	}
	tabReason := ""
	if m.activeTab < 0 {
		tabReason = m.tr("No active tab")
	}
	actions := []action{
		m.commandAction(commandOpenConfig, "Preferences: Open Configuration File", "", ""),
		m.commandAction(commandOpenKeybindings, "Preferences: Open Keyboard Shortcuts File", "", ""),
		{"theme.select", m.tr("Appearance: Color Theme"), "", "", m.openThemePicker},
		{"statusBar.style", m.tr("Appearance: Status Bar Style"), "", "", m.openStatusBarStylePicker},
		{"editor.toggleVim", m.tr("Preferences: Toggle Vim Mode"), "", "", func() tea.Cmd {
			m.setVimDisabled(!m.vimDisabled)
			return m.savePreferences()
		}},
		{"language.select", m.tr("Preferences: Configure Display Language"), "", "", m.openLanguagePicker},
		{"file.syntax", m.tr("Change Syntax Highlighting"), "", syntaxReason, m.openFileSyntaxPicker},
		m.commandAction(commandSave, "File: Save", docReason, ""),
		m.commandAction(commandClose, "View: Close Editor", tabReason, ""),
		m.commandAction(commandCloseOthers, "View: Close Other Editors", tabReason, ""),
		m.commandAction(commandPreviousTab, "View: Previous Editor", "", ""),
		m.commandAction(commandNextTab, "View: Next Editor", "", ""),
		{"edit.replace", m.tr("Find: Replace Current View"), "", docReason, m.openFileReplace},
		m.commandAction(commandFind, "Find: Search Current View", "", ""),
		m.commandAction(commandRefresh, "Refresh", "", ""),
		m.commandAction(commandReload, "Reload VL", "", ""),
		m.commandAction(commandQuit, "Quit", "", ""),
	}
	return append(actions, m.textEditorActions()...)
}
