package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func clickScopeAction(t *testing.T, m *Model, folder string, header, staged bool, id string) tea.Cmd {
	t.Helper()
	for i, entry := range m.items {
		if entry.repoHeader || entry.header != header || entry.staged != staged || entry.folder != folder {
			continue
		}
		m.selected = i
		m.ensureSidebar()
		y := headerHeight + i - m.sideTop
		for _, a := range m.changeRowActions(entry, m.sideWidth()) {
			if a.id != id {
				continue
			}
			m.Update(tea.MouseMotionMsg{X: a.x + 1, Y: y})
			for _, line := range m.sidebarLines() {
				if ansi.StringWidth(line) != m.sideWidth() {
					t.Fatal("hover actions overflow sidebar")
				}
			}
			_, cmd := m.Update(tea.MouseClickMsg{X: a.x + 1, Y: y, Button: tea.MouseLeft})
			return cmd
		}
		t.Fatalf("scope missing action %s", id)
	}
	t.Fatal("scope missing from tree")
	return nil
}

func TestFolderHoverActionsIncludeCollapsedDescendantsAndKeepOtherPaths(t *testing.T) {
	m := explorerWorkspace(t)
	for _, dir := range []string{"cmd/nested", "cmd-other"} {
		if err := os.MkdirAll(filepath.Join(m.repo.Root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{"cmd/nested/child.go", "cmd-other/keep.go"} {
		if err := os.WriteFile(filepath.Join(m.repo.Root, path), []byte("package main\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	drain(t, m, m.refresh())
	drain(t, m, m.selectSection(changes))
	m.sidebarCollapsed[changeKey("", false, "directory", "cmd")] = true
	m.rebuildItems()
	drain(t, m, clickScopeAction(t, m, "cmd", false, false, "stage"))
	for _, c := range m.snapshot.Changes {
		if strings.HasPrefix(c.Path, "cmd/") {
			if !c.Staged() {
				t.Fatalf("descendant not staged: %s", c.Path)
			}
		} else if c.Staged() {
			t.Fatalf("outside path staged: %s", c.Path)
		}
	}
	drain(t, m, clickScopeAction(t, m, "cmd", false, true, "unstage"))
	for _, c := range m.snapshot.Changes {
		if c.Staged() {
			t.Fatalf("still staged: %s", c.Path)
		}
	}
}

func TestFolderDiscardConfirmsAndNeverDeletesOutsideScope(t *testing.T) {
	m := explorerWorkspace(t)
	drain(t, m, m.selectSection(changes))
	clickScopeAction(t, m, "cmd", false, false, "discard")
	if m.modal != "confirm" || !strings.Contains(m.modalTitle, "cmd") {
		t.Fatal("folder discard missing scope confirmation")
	}
	m.Update(tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft})
	if _, err := os.Stat(filepath.Join(m.repo.Root, "cmd/main.go")); err != nil {
		t.Fatal("cancel deleted file")
	}
	clickScopeAction(t, m, "cmd", false, false, "discard")
	drain(t, m, press(m, "enter"))
	if _, err := os.Stat(filepath.Join(m.repo.Root, "cmd/main.go")); !os.IsNotExist(err) {
		t.Fatal("confirmed folder discard did not delete descendant")
	}
	if _, err := os.Stat(filepath.Join(m.repo.Root, "README.md")); err != nil {
		t.Fatal("folder discard touched outside scope")
	}
	if info, err := os.Stat(filepath.Join(m.repo.Root, "cmd")); err != nil || !info.IsDir() {
		t.Fatal("folder discard removed directory itself")
	}
}

func TestGroupHoverStageUnstageAndOpenAll(t *testing.T) {
	m := explorerWorkspace(t)
	drain(t, m, m.selectSection(changes))
	drain(t, m, clickScopeAction(t, m, "", true, false, "open"))
	if len(m.tabs) != 3 {
		t.Fatalf("open all created %d tabs", len(m.tabs))
	}
	drain(t, m, clickScopeAction(t, m, "", true, false, "stage"))
	for _, c := range m.snapshot.Changes {
		if !c.Staged() {
			t.Fatalf("group missed %s", c.Path)
		}
	}
	drain(t, m, clickScopeAction(t, m, "", true, true, "unstage"))
	for _, c := range m.snapshot.Changes {
		if c.Staged() {
			t.Fatalf("group unstage missed %s", c.Path)
		}
	}
}

func TestFolderActionsDisableForUnsavedDescendant(t *testing.T) {
	m := explorerWorkspace(t)
	drain(t, m, m.selectSection(changes))
	m.openFile("cmd/main.go")
	m.document().Insert("// edit\n")
	for _, id := range []string{"stage", "discard"} {
		cmd := clickScopeAction(t, m, "cmd", false, false, id)
		if cmd != nil || m.busy || m.modal != "" {
			t.Fatalf("%s operated on unsaved descendant", id)
		}
	}
	for _, c := range m.snapshot.Changes {
		if c.Staged() {
			t.Fatal("unsaved file was staged")
		}
	}
}
