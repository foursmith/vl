package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/elegit/internal/files"
	gitservice "github.com/ourongxing/elegit/internal/git"
)

func tabWorkspace(t *testing.T, count int) *Model {
	t.Helper()
	root := t.TempDir()
	for i := 0; i < count; i++ {
		path := filepath.Join(root, fmt.Sprintf("file%d.go", i))
		if err := os.WriteFile(path, []byte("package main\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	m := New(context.Background(), &gitservice.Repository{Root: root}, false)
	m.section = explorer
	m.notice = ""
	return m
}

func TestPreviewReplacesOnlyCleanPreviewAndPromotesOnEdit(t *testing.T) {
	m := tabWorkspace(t, 4)
	m.openPreviewFile("file0.go")
	first := m.document()
	m.openPreviewFile("file1.go")
	if len(m.tabs) != 1 || m.document() == first || !m.tabs[0].preview {
		t.Fatal("single click did not replace the clean preview")
	}
	press(m, "x")
	press(m, "ctrl+z")
	if m.tabs[0].preview || m.document().Dirty() {
		t.Fatal("edit should permanently keep the tab even after undo")
	}
	m.openPreviewFile("file2.go")
	m.openPreviewFile("file3.go")
	if len(m.tabs) != 2 || filepath.Base(m.tabs[0].doc.Path) != "file1.go" || filepath.Base(m.tabs[1].doc.Path) != "file3.go" {
		t.Fatal("opening previews replaced a fixed tab")
	}
	m.openFile("file3.go")
	if len(m.tabs) != 2 || m.tabs[1].preview {
		t.Fatal("explicit open should keep the existing preview")
	}
	m.openPreviewFile("file1.go")
	if len(m.tabs) != 2 || m.activeTab != 0 || m.tabs[0].preview {
		t.Fatal("existing fixed file duplicated or became a preview")
	}
}

func TestFileIdentityAndFailurePreservePreview(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openPreviewFile("file0.go")
	doc := m.document()
	m.openPreviewFile("missing.go")
	if len(m.tabs) != 1 || m.document() != doc {
		t.Fatal("failed open destroyed preview")
	}
	alias := filepath.Join(m.repo.Root, "alias.go")
	if err := os.Symlink(doc.Path, alias); err != nil {
		t.Skip(err)
	}
	m.openFile("alias.go")
	if len(m.tabs) != 1 || m.document() != doc || m.tabs[0].preview {
		t.Fatal("symlink created a duplicate file tab")
	}
}

func TestExplorerSingleAndDoubleClick(t *testing.T) {
	m := tabWorkspace(t, 3)
	m.snapshot.Files = []string{"file0.go", "file1.go", "file2.go"}
	for _, path := range m.snapshot.Files {
		m.treeEntries = append(m.treeEntries, files.Entry{Path: path})
	}
	m.rebuildItems()
	click := func(index int) {
		m.click(tea.MouseClickMsg{X: 5, Y: sidebarHeaderRows + index, Button: tea.MouseLeft})
	}
	click(0)
	click(1)
	if len(m.tabs) != 1 || filepath.Base(m.document().Path) != "file1.go" || !m.tabs[0].preview || !m.sidebar {
		t.Fatal("single-click browsing did not reuse preview and preserve explorer focus")
	}
	click(1)
	if m.tabs[0].preview || m.sidebar {
		t.Fatal("double click did not keep file and move focus to editor")
	}
	click(2)
	if len(m.tabs) != 2 {
		t.Fatal("double-clicked file was replaced")
	}
}

func TestTabCloseHitboxesAndInactiveMiddleClose(t *testing.T) {
	m := tabWorkspace(t, 3)
	for i := 0; i < 3; i++ {
		m.openFile(fmt.Sprintf("file%d.go", i))
	}
	active := m.document()
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 30})
	var slot tabSlot
	for _, s := range m.tabLayout() {
		if s.index == 0 && s.control == 0 {
			slot = s
			break
		}
	}
	m.click(tea.MouseClickMsg{X: m.mainX() + slot.x + 2, Y: tabRow, Button: tea.MouseMiddle})
	if len(m.tabs) != 2 || m.document() != active || m.activeTab != 1 {
		t.Fatal("closing inactive tab changed active document")
	}
	for _, s := range m.tabLayout() {
		if s.index == m.activeTab && s.control == 0 {
			slot = s
			break
		}
	}
	m.click(tea.MouseClickMsg{X: m.mainX() + slot.x + slot.width - 3, Y: tabRow, Button: tea.MouseLeft})
	if len(m.tabs) != 1 || filepath.Base(m.document().Path) != "file1.go" {
		t.Fatal("visible close button missed tab")
	}
}

func TestCloseDirtySaveDiscardCancelAndSaveFailure(t *testing.T) {
	m := tabWorkspace(t, 2)
	m.openPreviewFile("file0.go")
	press(m, "x")
	first := m.document()
	m.closeTab()
	if m.modal != "close" || len(m.tabs) != 1 {
		t.Fatal("dirty close did not offer save/discard/cancel")
	}
	press(m, "esc")
	if m.modal != "" || m.document() != first {
		t.Fatal("cancel closed dirty tab")
	}
	m.closeTab()
	press(m, "s")
	data, _ := os.ReadFile(first.Path)
	if string(data) != "xpackage main\n" || len(m.tabs) != 0 || m.modal != "" {
		t.Fatal("save and close failed")
	}
	m.openFile("file1.go")
	press(m, "x")
	os.WriteFile(m.document().Path, []byte("external\n"), 0644)
	m.closeTab()
	press(m, "s")
	if m.modal != "close" || len(m.tabs) != 1 {
		t.Fatal("save failure dismissed dirty close")
	}
	press(m, "d")
	if len(m.tabs) != 0 {
		t.Fatal("discard did not close tab")
	}
	data, _ = os.ReadFile(filepath.Join(m.repo.Root, "file1.go"))
	if string(data) != "external\n" {
		t.Fatal("discard overwrote external changes")
	}
}

func TestOverflowKeepsActiveTabAndCloseVisible(t *testing.T) {
	m := tabWorkspace(t, 15)
	for i := 0; i < 15; i++ {
		m.openFile(fmt.Sprintf("file%d.go", i))
	}
	for _, width := range []int{30, 48, 80, 120} {
		m.Update(tea.WindowSizeMsg{Width: width, Height: 24})
		found := false
		previousEnd := 0
		for _, slot := range m.tabLayout() {
			if slot.x < previousEnd || slot.x+slot.width > m.mainWidth() {
				t.Fatal("tab hitboxes overlap or overflow")
			}
			previousEnd = slot.x + slot.width
			if slot.control == 0 && slot.index == m.activeTab {
				found = true
				if slot.width < 7 {
					t.Fatal("close button not visible")
				}
			}
		}
		if !found {
			t.Fatalf("active tab invisible at width %d", width)
		}
		if ansi.StringWidth(m.tabBar()) != m.mainWidth() {
			t.Fatal("tab bar overflow")
		}
	}
}

func TestClosedDiffStaysClosedAndClickPinsPreview(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openPreviewFile("file0.go")
	for i := 0; i < 2; i++ {
		slot := m.tabLayout()[0]
		m.click(tea.MouseClickMsg{X: m.mainX() + slot.x + 4, Y: tabRow, Button: tea.MouseLeft})
	}
	if m.tabs[0].preview {
		t.Fatal("double-clicking tab did not keep preview")
	}
	m.section = changes
	m.diffOpen = true
	m.showEditor = false
	m.diffSeq = 1
	m.closeTab()
	seq := m.diffSeq
	m.Update(diffMsg{seq: seq - 1, raw: "stale"})
	m.Update(snapshotMsg{seq: m.refreshSeq, snapshot: gitservice.Snapshot{Changes: []gitservice.Change{{Path: "file0.go", Index: ' ', Worktree: 'M'}}}})
	if m.diffOpen || m.patch.Raw != "" || !m.showEditor {
		t.Fatal("background refresh reopened closed diff")
	}
	m.lastClickTime = time.Now().Add(-time.Second)
	if m.doubleClick(m.lastClickKey) {
		t.Fatal("unrelated delayed click treated as double click")
	}
}

func TestCloseOthersKeepsActiveAndGuardsDirtyFiles(t *testing.T) {
	m := tabWorkspace(t, 3)
	m.openFile("file0.go")
	press(m, "x")
	m.openFile("file1.go")
	m.openFile("file2.go")
	active := m.document()
	m.closeOtherTabs()
	if m.modal != "close" {
		t.Fatal("close others bypassed dirty file")
	}
	press(m, "d")
	if len(m.tabs) != 1 || m.document() != active {
		t.Fatal("close others did not keep active document")
	}
	m.closeAllTabs()
	if len(m.tabs) != 0 || m.diffOpen {
		t.Fatal("close all left tabs behind")
	}
}

func TestEditingKeepsDirtyActiveTabVisibleAtOverflowBoundary(t *testing.T) {
	m := tabWorkspace(t, 2)
	m.openFile("file0.go")
	m.openFile("file1.go")
	m.Update(tea.WindowSizeMsg{Width: 32, Height: 24})
	m.sidebar = false
	press(m, "x")
	found := false
	for _, slot := range m.tabLayout() {
		if slot.control == 0 && slot.index == m.activeTab {
			found = true
		}
	}
	if !found {
		t.Fatal("editing pushed active tab out of the visible strip")
	}
	if !strings.Contains(ansi.Strip(m.tabBar()), m.symbol(symbolDot)) {
		t.Fatal("dirty indicator was truncated")
	}
}
