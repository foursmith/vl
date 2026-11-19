package app

import (
	"context"
	"path/filepath"

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
	tabReason := ""
	if m.activeTab < 0 {
		tabReason = m.tr("No active tab")
	}
	return []action{
		{"theme.select", m.tr("Preferences: Color Theme"), "", "", m.openThemePicker},
		{"statusBar.style", m.tr("Preferences: Status Bar Style"), "", "", m.openStatusBarStylePicker},
		{"language.select", m.tr("Preferences: Configure Display Language"), "", "", m.openLanguagePicker},
		m.commandAction(commandSave, "File: Save", docReason, ""),
		m.commandAction(commandClose, "View: Close Editor", tabReason, ""),
		{"edit.replace", m.tr("Find: Replace Current View"), "", docReason, m.openFileReplace},
		m.commandAction(commandFind, "Find: Search Current View", "", ""),
		m.commandAction(commandQuit, "Quit", "", ""),
	}
}
