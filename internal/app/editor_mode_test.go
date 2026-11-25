package app

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/foursmith/vl/internal/config"
)

func TestEditorModeGoToFileListsCurrentDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"first.txt", "second.txt", ".hidden", "subdir/nested.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("first\nsecond\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	m, err := NewEditor(t.Context(), filepath.Join(dir, "first.txt"), false)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	drain(t, m, press(m, "ctrl+g"))
	var filenames []string
	for _, a := range m.filteredActions() {
		filenames = append(filenames, a.id)
	}
	if !slices.Equal(filenames, []string{".hidden", "first.txt", "second.txt"}) {
		t.Fatalf("Go to File listed %v", filenames)
	}
	press(m, "second.txt:2:3")
	press(m, "enter")
	line, column := m.document().Position()
	if filepath.Base(m.document().Path) != "second.txt" || line != 1 || column != 2 {
		t.Fatalf("opened %s at %d:%d", m.document().Path, line, column)
	}
	if !m.editorOnly || m.sideShown() || m.refreshSeq != 0 || m.treeSeq != 0 {
		t.Fatal("Go to File activated workspace UI or indexing")
	}

	drain(t, m, press(m, "ctrl+g"))
	press(m, "first.txt")
	press(m, "enter")
	if filepath.Base(m.document().Path) != "first.txt" || len(m.tabs) != 2 {
		t.Fatal("Go to File did not return to the existing editor")
	}
}

func TestEditorModeRefreshReloadsCleanFileAndKeepsUnsavedEdits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte("original\n"), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := NewEditor(t.Context(), path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	press(m, "ctrl+f")
	refresh := func(diskText string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(diskText), 0600); err != nil {
			t.Fatal(err)
		}
		_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyF5})
		drain(t, m, cmd)
	}
	refresh("updated\n")
	if m.document().Text != "updated\n" || !m.fileFindFocused() || m.refreshSeq != 0 {
		t.Fatal("F5 did not reload the current file within the standalone editor")
	}

	m.document().Insert("unsaved ")
	refresh("changed again\n")
	if m.document().Text != "unsaved updated\n" || !m.document().Dirty() ||
		!m.tabs[m.activeTab].reloadWhenClean {
		t.Fatal("F5 did not preserve unsaved edits after a disk change")
	}
}

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
		for _, k := range []string{"esc", "ctrl+1", "ctrl+2", "ctrl+3", "ctrl+r"} {
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

func TestEditorModeStatusHintsAdvertiseQuitAndCommands(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte("hello\n"), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := NewEditor(t.Context(), path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	m.SetTerminalSize(120, 24)
	m.notice = ""
	if got := statusPart(t, m, "notice").text; got != "Ctrl+Q Quit · Ctrl+P Show All Commands" {
		t.Fatalf("clean editor hint = %q", got)
	}

	keymap, err := buildWorkbenchKeymap([]workbenchKeyOverride{
		{id: string(commandQuit), binding: binding("F8", "", "f8")},
	})
	if err != nil {
		t.Fatal(err)
	}
	m.workbenchKeymap = keymap
	m.document().Insert("edited ")
	if got := statusPart(t, m, "notice").text; got != "F8 Quit · Ctrl+P Show All Commands" {
		t.Fatalf("modified editor hint = %q", got)
	}
	if got := statusPart(t, m, "editor").text; got != "file.txt *" {
		t.Fatalf("modified editor label = %q", got)
	}
	m.SetTerminalSize(22, 24)
	if got := plain(m.statusLine()); strings.Contains(got, "F8") {
		t.Fatalf("quit hint displayed without enough space: %q", got)
	}
	m.SetTerminalSize(120, 24)
	press(m, "ctrl+f")
	if hints := m.statusHints(); len(hints) != 0 {
		t.Fatalf("editing hints shown while Find has focus: %v", hints)
	}
}

func TestEditorStartupHighlightsWithSavedTheme(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := NewEditor(context.Background(), path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	m.SetLayout(config.Layout{Appearance: config.Appearance{Theme: config.ThemePreferences{Dark: "light_modern"}}})
	tab := &m.tabs[m.activeTab]
	before := tab.highlightJob.seq
	batch := m.Init()().(tea.BatchMsg)
	if !tab.highlightJob.pending || tab.highlightJob.seq == before {
		t.Fatal("startup did not replace the pending theme-restoration job")
	}
	drain(t, m, batch[len(batch)-1])
	if len(tab.spans) == 0 || len(tab.spans[0]) == 0 || tab.spans[0][0].Color != "#0000FF" {
		t.Fatal("startup did not apply Light Modern syntax colors")
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
			if !hasSaveConfirmation(m) || m.document() == nil {
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
	if !m.snapshot.NoRepository || m.historyStates != nil || m.historyPages != nil || m.commitDrafts != nil || m.sidebarCollapsed != nil || m.expanded != nil {
		t.Fatal("editor initialized workspace/Git state")
	}
	m.openPalette()
	refreshAvailable, deleteToStartAvailable, deleteToEndAvailable := false, false, false
	for _, a := range m.filteredActions() {
		if strings.HasPrefix(a.id, "git.") || strings.HasPrefix(a.id, "github.") || strings.HasPrefix(a.id, "view.") || a.id == "file.closeOthers" || a.id == "file.keepOpen" {
			t.Fatalf("non-editor command exposed: %s", a.id)
		}
		if a.id == string(commandRefresh) {
			refreshAvailable = a.key == "F5" && a.disabled == ""
		}
		if a.id == string(commandDeleteToLineStart) {
			deleteToStartAvailable = a.key == "Ctrl+Backspace" && a.disabled == ""
		}
		if a.id == string(commandDeleteToLineEnd) {
			deleteToEndAvailable = a.key == "Ctrl+Delete" && a.disabled == ""
		}
	}
	if !refreshAvailable {
		t.Fatal("editor palette did not expose the standalone refresh command")
	}
	if !deleteToStartAvailable || !deleteToEndAvailable {
		t.Fatal("editor palette did not expose the line boundary deletion shortcuts")
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
