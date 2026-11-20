package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestFileSyntaxPickerPersistsAndResets(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	dir := t.TempDir()
	path := filepath.Join(dir, "script")
	if err := os.WriteFile(path, []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(dir, "script-link")
	if err := os.Symlink(path, alias); err != nil {
		t.Fatal(err)
	}
	open := func(path string) *Model {
		t.Helper()
		m, err := NewEditor(context.Background(), path, false)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(m.Close)
		drain(t, m, m.scheduleHighlight())
		return m
	}
	choose := func(m *Model, row string, click bool) {
		t.Helper()
		segment := statusPart(t, m, "language")
		m.Update(tea.MouseClickMsg{X: segment.x + 1, Y: m.height - 1, Button: tea.MouseLeft})
		if m.modal != "fileSyntax" {
			t.Fatal("clicking the language did not open the syntax picker")
		}
		for _, r := range row {
			m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		}
		if rows := m.filteredActions(); len(rows) != 1 || rows[0].label != row {
			t.Fatalf("filter %q returned %v", row, rows)
		}
		var cmd tea.Cmd
		if click {
			l := m.pickerLayout(m.filteredActions())
			_, cmd = m.Update(tea.MouseClickMsg{X: l.x + 2, Y: l.y + 3, Button: tea.MouseLeft})
		} else {
			_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		}
		drain(t, m, cmd)
	}

	m := open(path)
	if got := statusPart(t, m, "language").text; got != "Plain Text" {
		t.Fatalf("extensionless file detected as %q", got)
	}
	// An explicit language change takes effect even in Insert mode.
	m.tabs[m.activeTab].vim.InsertMode = true
	choose(m, "Go", true)
	if got := statusPart(t, m, "language").text; got != "Go" || len(m.tabs[0].spans) == 0 {
		t.Fatalf("Go syntax did not apply, status %q", got)
	}
	m.Close()

	m = open(alias)
	if got := statusPart(t, m, "language").text; got != "Go" || len(m.tabs[0].spans) == 0 {
		t.Fatalf("saved syntax was not restored through the symlink, status %q", got)
	}
	choose(m, "Plain Text", true)
	if got := m.activeSyntax().ID; got != "text" || len(m.tabs[0].spans) != 0 {
		t.Fatalf("plain text did not disable syntax, id %q", got)
	}
	m.Close()
	m = open(path)
	if m.activeSyntax().ID != "text" {
		t.Fatal("plain text override did not survive reopening")
	}
	choose(m, "Auto Detect", false)
	m.Close()
	if m = open(path); m.activeSyntax().ID != "" || len(m.tabs[0].spans) != 0 {
		t.Fatal("automatic detection was not restored")
	}
}

func TestFileSyntaxUpdatesEditorAndDiffTabs(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := tabWorkspace(t, 1)
	t.Cleanup(m.Close)
	m.openFile("file0.go")
	drain(t, m, m.scheduleHighlight())
	drain(t, m, m.requestDiff(diffTarget{path: "file0.go"}, func(context.Context) (string, error) {
		return diffTabText("review"), nil
	}))
	if len(m.diffSyntax) == 0 || len(m.tabs[0].spans) == 0 {
		t.Fatal("fixture has no syntax spans")
	}
	segment := statusPart(t, m, "language")
	m.Update(tea.MouseClickMsg{X: segment.x + 1, Y: m.height - 1, Button: tea.MouseLeft})
	drain(t, m, press(m, "down"))
	drain(t, m, press(m, "enter"))
	if len(m.diffSyntax) != 0 || len(m.tabs[0].spans) != 0 || m.tabs[0].syntax.ID != "text" {
		t.Fatal("plain text selection did not update both tabs")
	}
}

func TestFileSyntaxUsesShellShebangInAutoMode(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "script")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nif true; then echo hello; fi\n"), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := NewEditor(context.Background(), path, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	drain(t, m, m.scheduleHighlight())
	if got := statusPart(t, m, "language").text; got != "Shell" || len(m.tabs[0].spans) < 2 || len(m.tabs[0].spans[1]) == 0 {
		t.Fatalf("shell shebang did not color the editor, status %q", got)
	}
	m.openFileSyntaxPicker()
	drain(t, m, press(m, "down"))
	drain(t, m, press(m, "enter"))
	if got := statusPart(t, m, "language").text; got != "Plain Text" || len(m.tabs[0].spans) != 0 {
		t.Fatalf("manual plain text did not override the shebang, status %q", got)
	}
}
