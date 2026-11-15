package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	gitservice "github.com/ourongxing/elegit/internal/git"
)

func explorerWorkspace(t *testing.T) *Model {
	t.Helper()
	root := t.TempDir()
	if out, err := exec.Command("git", "init", "-b", "main", root).CombinedOutput(); err != nil {
		t.Fatalf("%s %v", out, err)
	}
	for _, dir := range []string{"bin", "cmd", "examples"} {
		os.Mkdir(filepath.Join(root, dir), 0755)
	}
	os.WriteFile(filepath.Join(root, ".gitignore"), []byte("bin/\nexamples/\n"), 0644)
	os.WriteFile(filepath.Join(root, "cmd", "main.go"), []byte("package main\n"), 0644)
	os.WriteFile(filepath.Join(root, "README.md"), []byte("# Workspace\n"), 0644)
	r, err := gitservice.Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	m := New(context.Background(), r, true)
	drain(t, m, m.refresh())
	return m
}

func TestExplorerShowsDiskFoldersGitDecorationsAndLazyExpansion(t *testing.T) {
	m := explorerWorkspace(t)
	if m.selected != -1 || m.section != explorer {
		t.Fatal("explorer should start without selecting a file")
	}
	if len(m.items) != 5 || m.items[0].folder != "bin" || !m.items[0].ignored || m.items[1].folder != "cmd" || m.items[1].status != 'U' || !m.items[2].ignored {
		t.Fatalf("root entries: %+v", m.items)
	}
	for i, entry := range m.items {
		if entry.folder == "cmd" {
			m.selected = i
			break
		}
	}
	drain(t, m, m.openSelected())
	if !m.expanded["cmd"] || len(m.items) != 6 || m.items[2].file != "cmd/main.go" || m.items[2].status != 'U' {
		t.Fatalf("expanded entries: %+v", m.items)
	}
	drain(t, m, m.collapseFolders())
	if len(m.items) != 5 || len(m.expanded) != 0 {
		t.Fatal("collapse did not restore root-only tree")
	}
}

func TestExplorerToolbarCreatesFilesAndFolders(t *testing.T) {
	m := explorerWorkspace(t)
	button := m.rootButtons(m.sideWidth())[0]
	drain(t, m, m.click(tea.MouseClickMsg{X: button.x + 1, Y: sidebarRootRow, Button: tea.MouseLeft}))
	if m.modal != "newFile" {
		t.Fatal("new file button missed its target")
	}
	m.input.SetValue("created.go")
	drain(t, m, press(m, "enter"))
	if m.document() == nil || filepath.Base(m.document().Path) != "created.go" {
		t.Fatal("new file was not opened")
	}
	m.newEntry(true)
	m.input.SetValue("new-folder")
	drain(t, m, press(m, "enter"))
	info, err := os.Stat(filepath.Join(m.repo.Root, "new-folder"))
	if err != nil || !info.IsDir() {
		t.Fatalf("new folder: %v", err)
	}
}

func TestHorizontalNavigationPreservesViewAndQuickOpen(t *testing.T) {
	m := explorerWorkspace(t)
	for _, nav := range m.navigation() {
		if nav.section == changes && !nav.search {
			drain(t, m, m.click(tea.MouseClickMsg{X: nav.x + 1, Y: tabRow, Button: tea.MouseLeft}))
			if m.section != changes {
				t.Fatal("source control navigation missed")
			}
			break
		}
	}
	m.openQuickFiles()
	m.input.SetValue("README.md")
	drain(t, m, press(m, "enter"))
	if m.document() == nil || !strings.HasSuffix(m.document().Path, "README.md") {
		t.Fatal("file search did not open selected file")
	}
}
