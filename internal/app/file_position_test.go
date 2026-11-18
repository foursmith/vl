package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/foursmith/vl/internal/config"
	"github.com/foursmith/vl/internal/files"
	gitservice "github.com/foursmith/vl/internal/git"
)

func TestFilePositionsSurvivePreviewReplacementCloseAndRestart(t *testing.T) {
	m := tabWorkspace(t, 2)
	defer m.Close()
	store := &config.FilePositionStore{Dir: filepath.Join(t.TempDir(), "files")}
	m.SetFilePositionStore(store)
	for _, name := range []string{"file0.go", "file1.go"} {
		if err := os.WriteFile(filepath.Join(m.repo.Root, name), []byte(strings.Repeat("你好 cursor position\n", 100)), 0600); err != nil {
			t.Fatal(err)
		}
	}
	m.openPreviewFile("file0.go")
	m.document().SetPosition(40, 5)
	m.tabs[m.activeTab].top = 30
	want := documentPosition(m.tabs[m.activeTab])
	m.openPreviewFile("file1.go")
	m.document().SetPosition(20, 3)
	m.openPreviewFile("file0.go")
	if got := documentPosition(m.tabs[m.activeTab]); !got.Equal(want) {
		t.Fatalf("replaced preview position = %#v, want %#v", got, want)
	}
	m.closeAllTabs()
	saved := m.Session()
	if len(saved.Tabs) != 0 {
		t.Fatalf("workspace session retained file positions: %#v", saved)
	}
	for _, name := range []string{"file0.go", "file1.go"} {
		if _, ok, err := store.Load(filepath.Join(m.repo.Root, name)); err != nil || !ok {
			t.Fatalf("closed file position missing: %s, %v", name, err)
		}
	}
	path := filepath.Join(t.TempDir(), "session.json")
	if err := config.SaveSession(path, saved); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.LoadSession(path)
	if err != nil {
		t.Fatal(err)
	}
	restored := New(context.Background(), m.repo, false)
	defer restored.Close()
	restored.SetFilePositionStore(store)
	restored.SetSession(loaded)
	restored.restoreSession()
	if len(restored.tabs) != 0 {
		t.Fatal("position history reopened closed tabs")
	}
	restored.openFile("file0.go")
	if got := documentPosition(restored.tabs[restored.activeTab]); !got.Equal(want) {
		t.Fatalf("reopened position = %#v, want %#v", got, want)
	}
	restored.openFile("file1.go")
	if line, col := restored.document().Position(); line != 20 || col != 3 {
		t.Fatalf("second file position = %d:%d", line, col)
	}
	// External truncation clamps stale positions to a valid character.
	restored.closeAllTabs()
	if err := os.WriteFile(filepath.Join(m.repo.Root, "file0.go"), []byte("你\n"), 0600); err != nil {
		t.Fatal(err)
	}
	restored.openFile("file0.go")
	if line, col := restored.document().Position(); line != 1 || col != 0 {
		t.Fatalf("stale position = %d:%d", line, col)
	}
	if restored.document().Dirty() {
		t.Fatal("restoring position modified the file")
	}
}

func TestStandaloneRestoresPositionsWithoutWorkspaceTabs(t *testing.T) {
	m := tabWorkspace(t, 2)
	defer m.Close()
	store := &config.FilePositionStore{Dir: filepath.Join(t.TempDir(), "files")}
	m.SetFilePositionStore(store)
	m.openFile("file0.go")
	m.document().SetPosition(0, 5)
	m.openFile("file1.go")
	m.SaveFilePositions()
	standalone, err := NewEditor(context.Background(), filepath.Join(m.repo.Root, "file0.go"), false)
	if err != nil {
		t.Fatal(err)
	}
	defer standalone.Close()
	standalone.SetFilePositionStore(store)
	if len(standalone.tabs) != 1 {
		t.Fatal("standalone opened workspace tabs")
	}
	if line, col := standalone.document().Position(); line != 0 || col != 5 {
		t.Fatalf("standalone position = %d:%d", line, col)
	}
	standalone.closeTab()
	position, ok, err := store.Load(filepath.Join(m.repo.Root, "file0.go"))
	if err != nil || !ok || position.Column != 5 {
		t.Fatalf("standalone close lost file position: %#v, %v", position, err)
	}
}

func TestExplorerClickRestoresFilePositions(t *testing.T) {
	m := tabWorkspace(t, 2)
	defer m.Close()
	store := &config.FilePositionStore{Dir: filepath.Join(t.TempDir(), "files")}
	m.SetFilePositionStore(store)
	for _, path := range []string{"file0.go", "file1.go"} {
		if err := os.WriteFile(filepath.Join(m.repo.Root, path), []byte(strings.Repeat("cursor position example\n", 100)), 0600); err != nil {
			t.Fatal(err)
		}
		m.treeEntries = append(m.treeEntries, files.Entry{Path: path, Label: path})
	}
	m.rebuildItems()
	click := func(index int) {
		t.Helper()
		m.click(tea.MouseClickMsg{X: m.sideX() + 5, Y: headerHeight + 1 + index, Button: tea.MouseLeft})
		if !m.sidebar || m.document() == nil || !m.tabs[m.activeTab].preview {
			t.Fatal("explorer click did not show a preview with sidebar focus")
		}
	}
	click(0)
	m.document().SetPosition(70, 8)
	m.ensureEditor()
	want := documentPosition(m.tabs[m.activeTab])
	click(1)
	click(0)
	if got := documentPosition(m.tabs[m.activeTab]); !got.Equal(want) {
		t.Fatalf("explorer preview position = %#v, want %#v", got, want)
	}
	m.closeAllTabs()
	saved := m.Session()
	m.SetSession(saved)
	m.restoreSession()
	click(0)
	if got := documentPosition(m.tabs[m.activeTab]); !got.Equal(want) {
		t.Fatalf("explorer reopened position = %#v, want %#v", got, want)
	}
}

func TestExplorerClickBeforeDiscoveryRestoresFilePosition(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	path := filepath.Join(m.repo.Root, "file0.go")
	if err := os.WriteFile(path, []byte(strings.Repeat("cursor position\n", 100)), 0600); err != nil {
		t.Fatal(err)
	}
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	store := &config.FilePositionStore{Dir: filepath.Join(t.TempDir(), "files")}
	if err := store.Save(real, config.FilePosition{Line: 70, Column: 8, Top: 50}); err != nil {
		t.Fatal(err)
	}
	m.SetFilePositionStore(store)
	m.SetSession(config.Session{})
	m.treeEntries = []files.Entry{{Path: "file0.go", Label: "file0.go"}}
	m.rebuildItems()
	m.click(tea.MouseClickMsg{X: m.sideX() + 5, Y: headerHeight + 1, Button: tea.MouseLeft})
	if line, col := m.document().Position(); line != 70 || col != 8 {
		t.Fatalf("early explorer click position = %d:%d", line, col)
	}
}

func TestNestedFileSharesPositionAcrossWorkspaceAndStandalone(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	store := &config.FilePositionStore{Dir: filepath.Join(t.TempDir(), "files")}
	m.SetFilePositionStore(store)
	nested := filepath.Join(m.repo.Root, "nested")
	if err := os.Mkdir(nested, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(nested, "file.go")
	if err := os.WriteFile(path, []byte(strings.Repeat("shared cursor position\n", 100)), 0600); err != nil {
		t.Fatal(err)
	}
	m.openFile("nested/file.go")
	m.document().SetPosition(70, 8)
	m.ensureEditor()
	m.SaveFilePositions()
	standalone, err := NewEditor(context.Background(), path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer standalone.Close()
	standalone.SetFilePositionStore(store)
	if line, col := standalone.document().Position(); line != 70 || col != 8 {
		t.Fatalf("standalone position = %d:%d", line, col)
	}
	standalone.document().SetPosition(30, 4)
	standalone.closeTab()
	subworkspace := New(context.Background(), &gitservice.Repository{Root: nested}, false)
	defer subworkspace.Close()
	subworkspace.SetFilePositionStore(store)
	subworkspace.openPreviewFile("file.go")
	if line, col := subworkspace.document().Position(); line != 30 || col != 4 {
		t.Fatalf("nested workspace position = %d:%d", line, col)
	}

}

func TestUntouchedFilesDoNotCreatePositionRecords(t *testing.T) {
	store := &config.FilePositionStore{Dir: filepath.Join(t.TempDir(), "files")}
	m := tabWorkspace(t, 2)
	defer m.Close()
	m.SetFilePositionStore(store)
	m.openPreviewFile("file0.go")
	m.openPreviewFile("file1.go")
	if err := m.SaveFilePositions(); err != nil {
		t.Fatal(err)
	}
	m.closeAllTabs()
	standalone, err := NewEditor(context.Background(), filepath.Join(m.repo.Root, "file0.go"), false)
	if err != nil {
		t.Fatal(err)
	}
	defer standalone.Close()
	standalone.SetFilePositionStore(store)
	if err := standalone.SaveFilePositions(); err != nil {
		t.Fatal(err)
	}
	standalone.closeTab()
	if _, err := os.Stat(store.Dir); !os.IsNotExist(err) {
		t.Fatalf("untouched files created position records: %v", err)
	}
}

func TestUnchangedFilePositionsDoNotRewriteRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte(strings.Repeat("cursor position example\n", 100)), 0600); err != nil {
		t.Fatal(err)
	}
	store := &config.FilePositionStore{Dir: filepath.Join(t.TempDir(), "files")}
	// A larger viewport may clamp the restored scroll offset. Opening alone
	// still must not rewrite the existing record.
	original := config.FilePosition{Line: 70, Column: 5, Top: 69}
	if err := store.Save(path, original); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(store.Dir)
	if err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(store.Dir, entries[0].Name())
	oldTime := time.Unix(100, 0)
	if err := os.Chtimes(record, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	m, err := NewEditor(context.Background(), path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	m.SetFilePositionStore(store)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 80})
	if err := m.SaveFilePositions(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(record)
	if err != nil || !info.ModTime().Equal(oldTime) {
		t.Fatalf("opening rewrote position record: %v, %v", info, err)
	}
	if got, _, err := store.Load(path); err != nil || !got.Equal(original) {
		t.Fatalf("opening changed saved position: %#v, %v", got, err)
	}
	m.document().SetPosition(40, 3)
	m.ensureEditor()
	if err := m.SaveFilePositions(); err != nil {
		t.Fatal(err)
	}
	if got, _, err := store.Load(path); err != nil || got.Line != 40 || got.Column != 3 {
		t.Fatalf("changed position not saved: %#v, %v", got, err)
	}
	if err := os.Chtimes(record, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := m.SaveFilePositions(); err != nil {
		t.Fatal(err)
	}
	m.closeTab()
	info, err = os.Stat(record)
	if err != nil || !info.ModTime().Equal(oldTime) {
		t.Fatalf("unchanged position was saved again: %v, %v", info, err)
	}
}

func TestReopeningFileDoesNotRestoreDeletedPosition(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	store := &config.FilePositionStore{Dir: filepath.Join(t.TempDir(), "files")}
	m.SetFilePositionStore(store)
	m.openFile("file0.go")
	m.document().SetPosition(0, 5)
	m.closeTab()
	if err := os.RemoveAll(store.Dir); err != nil {
		t.Fatal(err)
	}
	m.openFile("file0.go")
	if line, col := m.document().Position(); line != 0 || col != 0 {
		t.Fatalf("deleted position restored: %d:%d", line, col)
	}
	m.closeTab()
	if _, err := os.Stat(store.Dir); !os.IsNotExist(err) {
		t.Fatalf("deleted position recreated: %v", err)
	}
}

func TestFoldStatePersistsAcrossWorkspaceAndStandalone(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	store := &config.FilePositionStore{Dir: filepath.Join(t.TempDir(), "files")}
	m.SetFilePositionStore(store)
	path := filepath.Join(m.repo.Root, "file0.go")
	text := "func outer() {\n if true {\n  inner()\n }\n other()\n}\nlast"
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	m.openFile("file0.go")
	m.document().SetPosition(1, 0)
	press(m, "z")
	press(m, "a")
	m.document().SetPosition(0, 0)
	press(m, "z")
	press(m, "a")
	want := documentPosition(m.tabs[m.activeTab])
	m.closeTab()
	m.openFile("file0.go")
	if got := documentPosition(m.tabs[m.activeTab]); !got.Equal(want) || len(got.Folds) != 2 {
		t.Fatalf("workspace did not restore nested folds: %#v", got)
	}
	m.closeTab()
	standalone, err := NewEditor(context.Background(), path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer standalone.Close()
	standalone.SetFilePositionStore(store)
	state := &standalone.tabs[0]
	if got := documentPosition(*state); !got.Equal(want) || state.sourceRow(1) != 6 {
		t.Fatalf("standalone did not restore folded display: %#v", got)
	}
	// Opening an unchanged file preserves both the record and its timestamp.
	entries, err := os.ReadDir(store.Dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("file records: %v, %v", entries, err)
	}
	record := filepath.Join(store.Dir, entries[0].Name())
	oldTime := time.Unix(100, 0)
	if err := os.Chtimes(record, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := standalone.SaveFilePositions(); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(record); err != nil || !info.ModTime().Equal(oldTime) {
		t.Fatal("unchanged restored folds rewrote the record", err)
	}
	// Only unfolding changes: cursor and scroll remain at zero.
	press(standalone, "z")
	press(standalone, "a")
	if err := standalone.SaveFilePositions(); err != nil {
		t.Fatal(err)
	}
	saved, _, err := store.Load(path)
	if err != nil || len(saved.Folds) != 1 || saved.Folds[0].Start != 1 {
		t.Fatalf("fold-only change was not saved: %#v, %v", saved, err)
	}
	// Changes inside a block keep its fold when the boundary lines still match.
	if err := os.WriteFile(path, []byte(strings.Replace(text, "inner()", "changed()", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	m.openFile("file0.go")
	if len(m.tabs[m.activeTab].folds) != 1 {
		t.Fatal("body-only edit discarded a fold with matching boundary lines")
	}
	m.closeTab()
	// Shifted boundary lines no longer match at the saved coordinates.
	if err := os.WriteFile(path, []byte("// new content\n"+text), 0600); err != nil {
		t.Fatal(err)
	}
	m.openFile("file0.go")
	if len(m.tabs[m.activeTab].folds) != 0 {
		t.Fatal("restored stale folds after an external edit")
	}
}
