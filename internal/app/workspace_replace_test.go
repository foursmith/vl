package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/editor"
)

func replacementWorkspace(t *testing.T) *Model {
	t.Helper()
	m := tabWorkspace(t, 2)
	t.Cleanup(m.Close)
	for _, name := range []string{"file0.go", "file1.go"} {
		if err := os.WriteFile(filepath.Join(m.repo.Root, name), []byte("猫 old old\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	m.openSearch()
	m.search.query.SetValue("old")
	drain(t, m, m.runSearch())
	return m
}
func assertReplacementFile(t *testing.T, m *Model, name, want string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(m.repo.Root, name))
	if err != nil || string(data) != want {
		t.Fatalf("%s = %q, want %q (read error: %v)", name, data, want, err)
	}
}

func TestWorkspaceReplaceSavesAffectedFiles(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := replacementWorkspace(t)
	path := filepath.Join(m.repo.Root, "file2.go")
	if err := os.WriteFile(path, []byte("\ufeffold\r\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(m.repo.Root, "untouched.txt"), []byte("disk\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m.openFile("untouched.txt")
	untouched := m.document()
	untouched.Edit("pending\n")
	m.openFile("file1.go")
	clean := m.document()
	m.openPreviewFile("file0.go")
	dirty := m.document()
	m.tabs[m.activeTab].vim.InsertMode = true
	dirty.BeginEditGroup()
	dirty.Insert("old ")
	drain(t, m, m.runSearch())
	m.openReplace()
	m.search.replacement.SetValue("新")
	drain(t, m, m.replaceAllSearchResults())
	if len(m.tabs) != 3 || m.document() != dirty || !strings.Contains(m.statusNotice(), "6 matches; saved 3 files") {
		t.Fatal("unexpected tabs or replacement notice", m.notice)
	}
	assertReplacementFile(t, m, "file0.go", "新 猫 新 新\n")
	assertReplacementFile(t, m, "file1.go", "猫 新 新\n")
	assertReplacementFile(t, m, "file2.go", "\ufeff新\r\n")
	assertReplacementFile(t, m, "untouched.txt", "disk\n")
	if !untouched.Dirty() || untouched.Text != "pending\n" {
		t.Fatal("replacement changed an unrelated document", untouched.Text)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0755 {
		t.Fatal("replacement did not preserve file mode", info, err)
	}
	if dirty.Dirty() || dirty.Text != "新 猫 新 新\n" || m.tabs[m.activeTab].vim.InsertMode ||
		clean.Dirty() || clean.Text != "猫 新 新\n" {
		t.Fatal("incorrect open buffer state", dirty.Text, clean.Text)
	}
	dirty.Undo()
	clean.Undo()
	if dirty.Text != "新 猫 新 新\n" || clean.Text != "猫 新 新\n" || dirty.Dirty() || clean.Dirty() {
		t.Fatal("undo should stop at the workspace replacement", dirty.Text, clean.Text)
	}
	clean.Redo()
	dirty.Redo()
	if dirty.Text != "新 猫 新 新\n" || clean.Text != "猫 新 新\n" || dirty.Dirty() || clean.Dirty() {
		t.Fatal("redo changed a workspace replacement", dirty.Text, clean.Text)
	}
	m.openFile("file2.go")
	reopened := m.document()
	reopened.Undo()
	if reopened.Text != "新\n" || reopened.Dirty() {
		t.Fatal("replacement history leaked into a file opened after replacement", reopened.Text)
	}
}

func TestWorkspaceReplaceReportsWriteFailures(t *testing.T) {
	m := replacementWorkspace(t)
	path := filepath.Join(m.repo.Root, "file0.go")
	if err := os.Chmod(path, 0444); err != nil {
		t.Fatal(err)
	}
	m.openFile("file0.go")
	m.openReplace()
	m.search.replacement.SetValue("new")
	drain(t, m, m.replaceAllSearchResults())
	if len(m.tabs) != 1 || m.document().Dirty() || m.document().Text != "猫 old old\n" ||
		m.search.result.Count != 2 || !strings.Contains(m.notice, "saved 1 files") ||
		!strings.Contains(m.notice, "file0.go") {
		t.Fatal("incorrect partial failure state", m.notice)
	}
	assertReplacementFile(t, m, "file0.go", "猫 old old\n")
	assertReplacementFile(t, m, "file1.go", "猫 new new\n")
}

func TestWorkspaceReplacePreservesExternalEdits(t *testing.T) {
	m := replacementWorkspace(t)
	m.openFile("file0.go")
	d := m.document()
	d.Edit("pending old\n")
	drain(t, m, m.runSearch())
	if err := os.WriteFile(d.Path, []byte("external\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m.openReplace()
	m.search.replacement.SetValue("new")
	drain(t, m, m.replaceAllSearchResults())
	assertReplacementFile(t, m, "file0.go", "external\n")
	assertReplacementFile(t, m, "file1.go", "猫 new new\n")
	if d.Text != "pending old\n" || !d.Dirty() || !strings.Contains(m.statusNotice(), "file changed on disk") {
		t.Fatal("external change did not block replacement in the affected buffer", d.Text, m.notice)
	}
}

func TestWorkspaceReplaceChecksEncodedSizeBeforeWriting(t *testing.T) {
	m := replacementWorkspace(t)
	for name, content := range map[string]string{
		"file0.go": "old\n",
		"file1.go": "\ufeffold\r\n",
	} {
		if err := os.WriteFile(filepath.Join(m.repo.Root, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	drain(t, m, m.runSearch())
	m.openReplace()
	// Bypass the input length limit to exercise the file-size check.
	m.search.replacement.buffer.Text = strings.Repeat("x", editor.MaxFileSize-1)
	m.replaceAllSearchResults()
	assertReplacementFile(t, m, "file0.go", "old\n")
	assertReplacementFile(t, m, "file1.go", "\ufeffold\r\n")
	if !strings.Contains(m.notice, "size limit") {
		t.Fatal("missing encoded size limit notice", m.notice)
	}
}

func TestWorkspaceReplaceEnterAppliesRegexToAllRetainedResults(t *testing.T) {
	m := replacementWorkspace(t)
	for name, content := range map[string]string{
		"file0.go": "skip old old\nkeep old\n",
		"file2.go": "old\n",
	} {
		if err := os.WriteFile(filepath.Join(m.repo.Root, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	m.search.query.SetValue("(o)(ld)")
	m.search.options.Regex = true
	drain(t, m, m.runSearch())
	for index := len(m.searchRows()) - 1; index >= 0; index-- {
		row := m.searchRows()[index]
		name := filepath.Base(m.search.result.Files[row.file].Path)
		if name == "file0.go" && row.match == 0 || name == "file2.go" && row.match < 0 {
			m.dismissSearchResult(index)
		}
	}
	m.openReplace()
	m.search.replacement.SetValue("$2$1")
	m.search.selected = 0
	drain(t, m, press(m, "enter"))
	assertReplacementFile(t, m, "file0.go", "skip old old\nkeep ldo\n")
	assertReplacementFile(t, m, "file1.go", "猫 ldo ldo\n")
	assertReplacementFile(t, m, "file2.go", "old\n")
}

func TestWorkspaceReplaceRejectsStaleResultsBeforeWriting(t *testing.T) {
	stale := replacementWorkspace(t)
	stale.openReplace()
	stale.search.replacement.SetValue("new")
	if err := os.WriteFile(filepath.Join(stale.repo.Root, "file1.go"), []byte("external"), 0644); err != nil {
		t.Fatal(err)
	}
	drain(t, stale, stale.replaceAllSearchResults())
	if len(stale.tabs) != 0 || !strings.Contains(stale.notice, "changed") {
		t.Fatal("stale bulk replacement partially applied")
	}
	assertReplacementFile(t, stale, "file0.go", "猫 old old\n")
}
func TestWorkspaceReplaceControlsAndLayout(t *testing.T) {
	m := replacementWorkspace(t)
	for _, c := range m.searchPanelControls(m.sidebarContentWidth()) {
		if c.id == "replaceToggle" {
			m.clickSearch(c.x, c.y)
			break
		}
	}
	if m.section != search || !m.search.replace || m.search.focus != 1 {
		t.Fatal("replace toggle did not open replacement field")
	}
	m.Update(tea.PasteMsg{Content: "new"})
	if m.search.replacement.Value() != "new" || m.search.query.Value() != "old" {
		t.Fatal("paste went to wrong field")
	}
	for _, right := range []bool{false, true} {
		m.SetSidebarLayout(36, right)
		for _, width := range []int{30, 69, 70, 120} {
			m.Update(tea.WindowSizeMsg{Width: width, Height: 20})
			lines := m.sidebarLinesWidth(m.sidebarContentWidth())
			for _, line := range lines {
				if ansi.StringWidth(line) != m.sidebarContentWidth() {
					t.Fatal("bad replacement panel width")
				}
			}
			if len(lines) != m.bodyHeight() || !strings.Contains(plain(lines[m.searchPanelLayout().replacement]), "↵") || m.searchResultY() != m.searchPanelLayout().summary+1 {
				t.Fatal("replacement displaced results")
			}
		}
	}
	for _, focus := range []int{2, 0, 1} {
		m.searchKey(tea.KeyPressMsg{Code: tea.KeyTab})
		if m.search.focus != focus {
			t.Fatalf("Tab focus=%d, want %d", m.search.focus, focus)
		}
	}
	// Click the replace-all button using its rendered geometry.
	for _, c := range m.searchPanelControls(m.sidebarContentWidth()) {
		if c.id == "replaceAll" {
			drain(t, m, m.clickSearch(c.x, c.y))
			break
		}
	}
	if m.search.result.Count != 0 || len(m.tabs) != 0 {
		t.Fatal("replace all button failed")
	}
	for _, c := range m.searchPanelControls(m.sidebarContentWidth()) {
		if c.id == "replaceToggle" {
			m.clickSearch(c.x, c.y)
			break
		}
	}
	if m.search.replace || m.search.focus != 0 {
		t.Fatal("replace toggle did not collapse")
	}
}
