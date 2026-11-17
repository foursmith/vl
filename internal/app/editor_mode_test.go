package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/ourongxing/vl/internal/config"
)

func TestEditorModeIsolatedAndFullWidth(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(path, []byte("hello 世界\n"), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := NewEditor(context.Background(), path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	for _, width := range []int{120, 50, 30} {
		m.Update(tea.WindowSizeMsg{Width: width, Height: 24})
		for _, k := range []string{"esc", "ctrl+1", "ctrl+2", "ctrl+3", "ctrl+l", "ctrl+j", "ctrl+h"} {
			press(m, k)
		}
		if m.sidebar || m.sidebarVisible || m.sideShown() || m.mainWidth() != width || m.modal != "" {
			t.Fatal("editor escaped into workspace UI")
		}
		if m.contentRow() != 0 || !strings.Contains(plain(m.mainLines()[0]), "hello 世界") {
			t.Fatal("editor content did not move up after removing tabs")
		}
		if strings.Contains(plain(m.mainLines()[0]), " x ") {
			t.Fatal("tab close control still visible")
		}
		m.click(tea.MouseClickMsg{X: 8, Y: m.contentRow(), Button: tea.MouseLeft})
		if m.document().Cursor == 0 {
			t.Fatal("mouse position ignored editor-only row offset")
		}
		if !strings.Contains(plain(m.View().Content), "hello 世界") {
			t.Fatal("file content missing")
		}
	}
	for _, a := range m.actions() {
		if strings.HasPrefix(a.id, "git.") || a.id == "workspace.openRecent" || strings.HasPrefix(a.id, "view.sidebar") {
			t.Fatalf("workspace action exposed: %s", a.id)
		}
	}
	for _, s := range m.statusLayout() {
		if s.id == "init" || s.id == "branch" || s.id == "changes" {
			t.Fatalf("Git status exposed: %s", s.id)
		}
	}
	if m.refresh() != nil || m.loading {
		t.Fatal("editor started Git refresh")
	}

	press(m, "c")
	press(m, "w")
	if m.sidebar || m.sidebarVisible {
		t.Fatal("Vim focus exposed sidebar")
	}
}

func TestEditorCloseProtectsEditsAndExits(t *testing.T) {
	for _, choice := range []string{"save", "discard", "cancel"} {
		t.Run(choice, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "file.txt")
			if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			m, err := NewEditor(context.Background(), path, false)
			if err != nil {
				t.Fatal(err)
			}
			defer m.Close()
			m.document().Insert("new ")
			m.closeTab()
			if m.modal != "close" || m.document() == nil {
				t.Fatal("dirty close skipped prompt")
			}
			cmd := m.chooseClose(choice)
			if choice == "cancel" {
				if cmd != nil || m.document() == nil || !m.document().Dirty() {
					t.Fatal("cancel lost buffer")
				}
				return
			}
			if cmd == nil {
				t.Fatal("last close did not quit")
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatal("last close did not return quit")
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			want := "original"
			if choice == "save" {
				want = "new original"
			}
			if string(data) != want {
				t.Fatalf("disk = %q", data)
			}
		})
	}
}

func TestEditorInsideRepositoryNeedsNoGitState(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "file.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	m, err := NewEditor(context.Background(), path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if !m.snapshot.NoRepository || m.historyStates != nil || m.historyPages != nil || m.commitDrafts != nil || m.changeCollapsed != nil || m.expanded != nil {
		t.Fatal("editor initialized workspace/Git state")
	}
	m.openPalette()
	for _, a := range m.filteredActions() {
		if strings.HasPrefix(a.id, "git.") || strings.HasPrefix(a.id, "github.") || strings.HasPrefix(a.id, "view.") || a.id == "file.closeOthers" || a.id == "file.keepOpen" {
			t.Fatalf("non-editor command exposed: %s", a.id)
		}
	}
	m.modal = ""
	m.document().Insert("// no git needed\n")
	if cmd := m.save(); cmd != nil || m.document().Dirty() {
		t.Fatal("save scheduled Git work or failed")
	}
	if cmd := m.initializeRepository(); cmd != nil {
		t.Fatal("editor can initialize a Git repository")
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "config")); !os.IsNotExist(err) {
		t.Fatal("editor touched Git metadata")
	}
	if !strings.Contains(plain(m.View().Content), "no git needed") {
		t.Fatal("editor render failed without Git")
	}
}

func TestEditorModePersistsOnlyFilePosition(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", "")
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte("first\nsecond line\n"), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := NewEditor(context.Background(), path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	store, err := config.NewFilePositionStore()
	if err != nil {
		t.Fatal(err)
	}
	m.SetFilePositionStore(store)
	// Editor-only models do not accept or expose workspace sessions.
	m.SetSession(config.Session{Tabs: []config.SessionTab{{Kind: "file", Path: "/other/file"}}, Active: 0})
	if m.pendingSession != nil || m.restoreSession() != nil {
		t.Fatal("editor accepted workspace restoration")
	}
	if session := m.Session(); len(session.Tabs) != 0 || session.Active != -1 {
		t.Fatalf("editor exposed a workspace session: %#v", session)
	}
	m.document().SetPosition(1, 4)
	if err := m.SaveFilePositions(); err != nil {
		t.Fatal(err)
	}
	position, exists, err := store.Load(path)
	if err != nil || !exists || position.Line != 1 || position.Column != 4 {
		t.Fatalf("file position = %#v, %v, %v", position, exists, err)
	}
	entries, err := os.ReadDir(filepath.Join(home, ".local", "state", "vl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "files" {
		t.Fatalf("editor created workspace state: %v", entries)
	}
	m.closeTab()
	entries, err = os.ReadDir(filepath.Join(home, ".local", "state", "vl"))
	if err != nil || len(entries) != 1 || entries[0].Name() != "files" {
		t.Fatalf("editor close created workspace state: %v, %v", entries, err)
	}
}
