package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/ourongxing/vl/internal/files"
	gitservice "github.com/ourongxing/vl/internal/git"
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
	m.section = explorer
	drain(t, m, m.refresh())
	return m
}

func TestExplorerShowsDiskFoldersGitDecorationsAndLazyExpansion(t *testing.T) {
	m := explorerWorkspace(t)
	if m.selected != -1 || m.section != explorer {
		t.Fatal("explorer should start without selecting a file")
	}
	if len(m.items) != 6 || m.items[1].folder != "bin" || !m.items[1].ignored || m.items[2].folder != "cmd" || m.items[2].status != 'U' || !m.items[3].ignored {
		t.Fatalf("root entries: %+v", m.items)
	}
	for i, entry := range m.items {
		if entry.folder == "cmd" {
			m.selected = i
			break
		}
	}
	drain(t, m, m.openSelected())
	if !m.expanded["cmd"] || len(m.items) != 7 || m.items[3].file != "cmd/main.go" || m.items[3].status != 'U' {
		t.Fatalf("expanded entries: %+v", m.items)
	}
	drain(t, m, m.collapseFolders())
	if len(m.items) != 6 || len(m.expanded) != 0 {
		t.Fatal("collapse did not restore root-only tree")
	}
}

func TestExplorerWorkspaceRoot(t *testing.T) {
	m := explorerWorkspace(t)
	defer m.Close()
	root := m.items[0]
	if !root.header || root.label != filepath.Base(m.repo.Root) || !m.folderExpanded(root) || m.items[1].depth != 1 {
		t.Fatalf("workspace root: %+v", m.items)
	}
	m.selected = 1
	m.selectParentFolder()
	if m.selected != 0 || m.selectedEntryPath() != "" {
		t.Fatal("workspace root should be the parent of top-level entries")
	}
	drain(t, m, m.openSelected())
	drain(t, m, m.refresh())
	if len(m.items) != 1 || m.selected != 0 || m.folderExpanded(root) {
		t.Fatalf("collapsed root after refresh: %+v", m.items)
	}
	drain(t, m, m.revealDocument("cmd/main.go"))
	if !m.folderExpanded(root) || m.current() == nil || m.current().file != "cmd/main.go" {
		t.Fatalf("revealed file: %+v", m.current())
	}
}

func TestExplorerCreatesFilesAndFolders(t *testing.T) {
	m := explorerWorkspace(t)
	m.newEntry(false)
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

func TestHorizontalNavigationAndFileSearch(t *testing.T) {
	m := explorerWorkspace(t)
	for _, nav := range m.navigation() {
		if nav.section == changes {
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

func TestExplorerStatusAggregationKeepsConflictPriority(t *testing.T) {
	m := explorerWorkspace(t)
	m.treeEntries = []files.Entry{{Path: "src", Label: "src", Directory: true}, {Path: "src/nested", Label: "nested", Directory: true}, {Path: "src/nested/typed.go", Label: "typed.go"}}
	m.snapshot.Changes = []gitservice.Change{
		{Path: "src/conflict.go", Index: 'U', Worktree: 'U'},
		{Path: "src/new.go", Index: '?', Worktree: '?'},
		{Path: "src/nested/typed.go", Index: ' ', Worktree: 'T'},
	}
	rows := m.explorerItems()
	if rows[1].status != '!' || rows[2].status != 'T' || rows[3].status != 'T' {
		t.Fatalf("status aggregation lost severity: %+v", rows)
	}
}

func TestExplorerDeleteConfirmsCancelsAndRefreshes(t *testing.T) {
	m := explorerWorkspace(t)
	defer m.Close()
	for i, entry := range m.items {
		if entry.file == "README.md" {
			m.selected = i
		}
	}
	path := filepath.Join(m.repo.Root, "README.md")
	m.Update(tea.KeyPressMsg{Code: tea.KeyDelete})
	if m.modal != "confirm" {
		t.Fatal("delete did not ask for confirmation")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("file removed before confirmation")
	}
	press(m, "esc")
	if _, err := os.Stat(path); err != nil {
		t.Fatal("cancel removed file")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyDelete})
	drain(t, m, press(m, "enter"))
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file still exists: %v", err)
	}
	for _, entry := range m.items {
		if entry.file == "README.md" {
			t.Fatal("deleted file remains in explorer")
		}
	}
}

func TestExplorerDeleteProtectsDirtyFilesAndOtherSections(t *testing.T) {
	m := explorerWorkspace(t)
	defer m.Close()
	m.openFile("README.md")
	for i, entry := range m.items {
		if entry.file == "README.md" {
			m.selected = i
		}
	}
	m.sidebar = true
	m.document().Insert("unsaved")
	m.Update(tea.KeyPressMsg{Code: tea.KeyDelete})
	if m.modal != "" || !strings.Contains(m.notice, "unsaved") {
		t.Fatal("dirty file deletion was allowed")
	}
	m.document().Undo()
	m.Update(tea.KeyPressMsg{Code: tea.KeyDelete})
	if m.modal != "confirm" {
		t.Fatal("clean file could not be deleted")
	}
	m.document().Insert("changed while confirming")
	drain(t, m, press(m, "enter"))
	if _, err := os.Stat(filepath.Join(m.repo.Root, "README.md")); err != nil {
		t.Fatal("confirmation did not recheck dirty buffer")
	}
	drain(t, m, m.selectSection(changes))
	m.selected = len(m.items) - 1
	m.Update(tea.KeyPressMsg{Code: tea.KeyDelete})
	if m.modal != "" || m.busy {
		t.Fatal("delete applied outside explorer")
	}
}

func TestExplorerDeleteFolderConfirmsAndProtectsDirtyDescendants(t *testing.T) {
	m := explorerWorkspace(t)
	defer m.Close()
	m.openFile("cmd/main.go")
	for i, entry := range m.items {
		if entry.folder == "cmd" {
			m.selected = i
		}
	}
	m.sidebar = true
	m.document().Insert("unsaved")
	m.Update(tea.KeyPressMsg{Code: tea.KeyDelete})
	if m.modal != "" || !strings.Contains(m.notice, "unsaved") {
		t.Fatal("folder containing dirty file could be deleted")
	}
	m.document().Undo()
	press(m, "shift+f10")
	chooseContextAction(t, m, "Delete Folder…")
	if m.modal != "confirm" || !strings.Contains(m.confirmationDetail, "all its contents") {
		t.Fatal("folder deletion did not explain recursive removal")
	}
	m.document().Insert("changed while confirming")
	drain(t, m, press(m, "enter"))
	path := filepath.Join(m.repo.Root, "cmd/main.go")
	if _, err := os.Stat(path); err != nil {
		t.Fatal("folder confirmation failed to recheck dirty descendants")
	}
	m.document().Undo()
	m.Update(tea.KeyPressMsg{Code: tea.KeyDelete})
	press(m, "esc")
	if _, err := os.Stat(path); err != nil {
		t.Fatal("cancel deleted folder contents")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyDelete})
	drain(t, m, press(m, "enter"))
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatalf("folder remains after confirmation: %v", err)
	}
	for _, entry := range m.items {
		if entry.folder == "cmd" || strings.HasPrefix(entry.file, "cmd/") {
			t.Fatal("deleted folder remains in explorer")
		}
	}
}
