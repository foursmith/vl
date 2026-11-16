package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/elegit/internal/diff"
	"github.com/ourongxing/elegit/internal/editor"
	"github.com/ourongxing/elegit/internal/files"
	gitservice "github.com/ourongxing/elegit/internal/git"
)

func drain(t *testing.T, m *Model, cmd tea.Cmd) {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for i := 0; len(queue) > 0; i++ {
		if i > 100 {
			t.Fatal("command chain did not settle")
		}
		current := queue[0]
		queue = queue[1:]
		if current == nil {
			continue
		}
		msg := current()
		if batch, ok := msg.(tea.BatchMsg); ok {
			queue = append(queue, batch...)
			continue
		}
		_, next := m.Update(msg)
		if next != nil {
			queue = append(queue, next)
		}
	}
}
func press(m *Model, key string) tea.Cmd {
	msg := tea.KeyPressMsg{Code: rune(key[0]), Text: key}
	if strings.HasPrefix(key, "ctrl+") {
		msg = tea.KeyPressMsg{Code: rune(key[len(key)-1]), Mod: tea.ModCtrl}
	}
	if key == "enter" {
		msg = tea.KeyPressMsg{Code: tea.KeyEnter}
	}
	if key == "esc" {
		msg = tea.KeyPressMsg{Code: tea.KeyEscape}
	}
	_, cmd := m.Update(msg)
	return cmd
}

func TestAppEditStageCommitWorkflow(t *testing.T) {
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-b", "main", dir}, {"-C", dir, "config", "user.name", "Test"}, {"-C", dir, "config", "user.email", "test@example.test"}, {"-C", dir, "config", "commit.gpgsign", "false"}} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("%s: %v", out, err)
		}
	}
	path := filepath.Join(dir, "main.go")
	os.WriteFile(path, []byte("package main\n\nfunc main() {}\n"), 0644)
	r, err := gitservice.Open(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	m := New(context.Background(), r, false)
	drain(t, m, m.refresh())
	if m.current() == nil || m.current().change.Path != "main.go" || len(m.patch.Hunks) != 1 {
		t.Fatal("initial changed file not selected")
	}
	drain(t, m, m.editSelected())
	if !m.editing() || m.sidebar {
		t.Fatal("editor did not take focus")
	}
	press(m, "s")
	press(m, "ctrl+z")
	if m.document().Dirty() {
		t.Fatal("undo did not restore document")
	}
	m.document().Move("ctrl+end", false)
	press(m, "enter")
	press(m, "x")
	drain(t, m, press(m, "ctrl+s"))
	data, _ := os.ReadFile(path)
	if !strings.HasSuffix(string(data), "\n\nx") {
		t.Fatalf("save: %q", data)
	}
	drain(t, m, m.selectSection(changes))
	drain(t, m, m.stageSelected(false))
	if !m.current().staged {
		t.Fatal("staged file not selected")
	}
	drain(t, m, m.commit())
	m.input.SetValue("first commit")
	drain(t, m, press(m, "enter"))
	if len(m.snapshot.Changes) != 0 || len(m.snapshot.Commits) != 1 {
		t.Fatalf("commit failed: %+v / %s", m.snapshot, m.notice)
	}
	if cmd := press(m, "ctrl+q"); cmd == nil {
		t.Fatal("clean quit unavailable")
	}
}

func TestFrameGeometryAndControlCharacters(t *testing.T) {
	m := New(context.Background(), &gitservice.Repository{Root: "/tmp/中文-repo"}, true)
	m.items = []item{{label: "CHANGES", header: true}, {label: "file\n\x1b[31m.go", key: "file", file: "file.go"}}
	m.selected = 1
	m.tabs = []tab{{doc: &editor.Document{Path: "/tmp/file.go", Text: "package main\n\t// 中文🙂\n", Anchor: -1}}}
	m.activeTab = 0
	for _, width := range []int{30, 48, 70, 80, 120} {
		for _, height := range []int{8, 14, 36} {
			for _, modal := range []bool{false, true} {
				m.Update(tea.WindowSizeMsg{Width: width, Height: height})
				m.modal = ""
				if modal {
					m.openPalette()
				}
				lines := strings.Split(m.View().Content, "\n")
				if len(lines) != height {
					t.Fatalf("%dx%d: got %d rows", width, height, len(lines))
				}
				for y, line := range lines {
					if got := ansi.StringWidth(line); got != width {
						t.Fatalf("%dx%d row %d width %d: %q", width, height, y, got, ansi.Strip(line))
					}
				}
			}
		}
	}
	if safe("x\n\x1b[31m") != "x\\n[31m" {
		t.Fatal("unsafe terminal text")
	}
}

func TestStaleResultsAndDirtyQuit(t *testing.T) {
	m := New(context.Background(), &gitservice.Repository{Root: "/tmp/repo"}, false)
	m.diffSeq = 2
	m.Update(diffMsg{target: m.diffState, seq: 1, raw: "stale"})
	if m.patch.Raw != "" {
		t.Fatal("stale diff applied")
	}
	m.refreshSeq = 3
	m.Update(snapshotMsg{seq: 2, snapshot: gitservice.Snapshot{Branch: "stale"}})
	if m.snapshot.Branch != "" {
		t.Fatal("stale snapshot applied")
	}
	m.tabs = []tab{{doc: &editor.Document{Path: "/tmp/a", Text: "unsaved", Anchor: -1}}}
	m.activeTab = 0
	if cmd := m.quit(); cmd == nil || m.modal != "confirm" {
		t.Fatal("unsaved changes were not guarded")
	}
	press(m, "esc")
	if m.modal != "" {
		t.Fatal("cancel did not dismiss dialog")
	}
	m.busy = true
	if cmd := m.quit(); cmd != nil {
		t.Fatal("quit during Git operation")
	}
}

func TestDiffRefreshPreservesScrollAndTypedKeys(t *testing.T) {
	m := New(context.Background(), &gitservice.Repository{Root: "/tmp/repo"}, false)
	showDiffFixture(m)
	m.sidebar = false
	m.split = false
	raw := "diff --git a/a b/a\n--- a/a\n+++ b/a\n@@ -1,50 +1,50 @@\n" + strings.Repeat(" same\n", 50)
	m.patch = diff.Parse(raw)
	m.renderDiff()
	m.viewport.SetYOffset(12)
	m.diffSeq = 2
	m.Update(diffMsg{target: m.diffState, seq: 2, raw: raw})
	if m.viewport.YOffset() != 12 {
		t.Fatal("refresh reset scroll")
	}
	m.tabs = []tab{{doc: &editor.Document{Path: "/tmp/a.go", Text: "\t中文\n", Anchor: -1}}}
	m.activeTab = 0
	press(m, "s")
	if m.document().Text != "s\t中文\n" || m.busy {
		t.Fatal("editor key was routed to Git")
	}
}

func TestReloadCleanTabsAndPreserveUnsavedEdits(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.go"), filepath.Join(dir, "b.go")
	os.WriteFile(a, []byte("old\n"), 0644)
	os.WriteFile(b, []byte("before\n"), 0644)
	first, _ := editor.Open(a)
	second, _ := editor.Open(b)
	second.Insert("unsaved ")
	m := New(context.Background(), &gitservice.Repository{Root: dir}, false)
	m.tabs = []tab{{doc: first}, {doc: second}}
	m.activeTab = 0
	os.WriteFile(a, []byte("new branch\n"), 0644)
	os.WriteFile(b, []byte("external\n"), 0644)
	m.reloadCleanTabs()
	if m.document().Text != "new branch\n" || m.tabs[1].doc.Text != "unsaved before\n" {
		t.Fatal("clean reload lost or retained the wrong content")
	}
	m.busy = true
	if cmd := m.save(); cmd != nil || !strings.Contains(m.notice, "Wait") {
		t.Fatal("save was allowed during a Git operation")
	}
}

func TestExplorerDirectoryReplacementAndCollapse(t *testing.T) {
	m := New(context.Background(), &gitservice.Repository{Root: "/tmp/repo"}, false)
	m.treeEntries = []files.Entry{{Path: "replaced", Label: "replaced", Directory: true}, {Path: "replaced/child.go", Label: "child.go", Depth: 1}, {Path: "z.go", Label: "z.go"}}
	m.expanded["replaced"] = true
	m.section = explorer
	m.rebuildItems()
	if len(m.items) != 3 || m.items[0].folder != "replaced" {
		t.Fatalf("file-to-directory replacement: %+v", m.items)
	}
	m.selected = 0
	m.openSelected()
	if m.expanded["replaced"] {
		t.Fatal("folder did not collapse")
	}
}

func TestNewDiffRequestCancelsPreviousPreview(t *testing.T) {
	m := explorerWorkspace(t)
	drain(t, m, m.selectSection(changes))
	for i, entry := range m.items {
		if entry.change != nil {
			m.selected = i
			break
		}
	}
	previous := m.preview()
	current := m.preview()
	if previous == nil || current == nil {
		t.Fatal("preview did not schedule Git work")
	}
	stale := previous().(diffMsg)
	if stale.err == nil {
		t.Fatal("old Git request was not cancelled")
	}
	m.Update(stale)
	drain(t, m, current)
	if m.patch.Raw == "" {
		t.Fatal("latest preview did not load")
	}
}

func TestRepeatedRefreshCancelsObsoleteSnapshot(t *testing.T) {
	m := explorerWorkspace(t)
	old := m.refresh()
	latest := m.refresh()
	stale := old().(snapshotMsg)
	if stale.err == nil {
		t.Fatal("superseded snapshot kept running")
	}
	m.Update(stale)
	drain(t, m, latest)
	if m.snapshot.Branch != "main" {
		t.Fatal("latest snapshot did not load", m.notice)
	}
}
