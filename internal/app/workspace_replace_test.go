package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/vl/internal/editor"
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
func TestWorkspaceReplaceBuffersUndoAndSave(t *testing.T) {
	m := replacementWorkspace(t)
	m.openPreviewFile("file0.go")
	m.tabs[m.activeTab].vim.InsertMode = true
	m.document().BeginEditGroup()
	m.document().Insert("old ")
	original := m.document().Text
	drain(t, m, m.runSearch())
	m.openReplace()
	m.search.replacement.SetValue("新")
	drain(t, m, m.replaceSearch(true))
	if len(m.tabs) != 2 || !strings.Contains(m.notice, "5 matches") {
		t.Fatal("missing changed files or count", m.notice)
	}
	for i := range m.tabs {
		tab := &m.tabs[i]
		if tab.preview || !tab.doc.Dirty() || strings.Contains(tab.doc.Text, "old") || tab.vim.InsertMode {
			t.Fatal("bad replacement buffer", tab.doc.Text)
		}
		data, _ := os.ReadFile(tab.doc.Path)
		if string(data) != "猫 old old\n" {
			t.Fatal("replacement saved without request")
		}
		tab.doc.Undo()
		want := "猫 old old\n"
		if filepath.Base(tab.doc.Path) == "file0.go" {
			want = original
		}
		if tab.doc.Text != want {
			t.Fatal("replacement not a separate undo step", tab.doc.Text)
		}
		tab.doc.Redo()
		if err := tab.doc.Save(); err != nil {
			t.Fatal(err)
		}
		data, _ = os.ReadFile(tab.doc.Path)
		if string(data) != tab.doc.Text {
			t.Fatal("save did not write replacement")
		}
	}
}
func TestWorkspaceReplaceSelectedRegexDismissAndStale(t *testing.T) {
	m := replacementWorkspace(t)
	m.openReplace()
	m.search.replacement.SetValue("")
	m.search.selected = 1
	drain(t, m, m.replaceSearch(false))
	if m.document().Text != "猫  old\n" || m.search.result.Count != 3 {
		t.Fatal("single or empty replacement failed", m.document().Text)
	}
	m.search.query.SetValue("(o)(ld)")
	m.search.options.Regex = true
	drain(t, m, m.runSearch())
	m.search.replacement.SetValue("$2$1")
	// Removing a file excludes it from bulk replacement.
	m.dismissSearchResult(0)
	drain(t, m, m.replaceSearch(true))
	if len(m.tabs) != 2 {
		t.Fatal("expected two buffers")
	}
	if m.tabs[0].doc.Text != "猫  old\n" || m.tabs[1].doc.Text != "猫 ldo ldo\n" {
		t.Fatal("dismissed results or capture groups were not respected")
	}

	stale := replacementWorkspace(t)
	stale.openReplace()
	stale.search.replacement.SetValue("new")
	if err := os.WriteFile(filepath.Join(stale.repo.Root, "file1.go"), []byte("external"), 0644); err != nil {
		t.Fatal(err)
	}
	drain(t, stale, stale.replaceSearch(true))
	if len(stale.tabs) != 0 || !strings.Contains(stale.notice, "changed") {
		t.Fatal("stale bulk replacement partially applied")
	}
}
func TestWorkspaceReplaceControlsAndLayout(t *testing.T) {
	m := replacementWorkspace(t)
	m.Update(tea.KeyPressMsg{Code: 'h', Mod: tea.ModCtrl | tea.ModShift})
	if m.section != search || !m.search.replace || m.search.focus != 1 {
		t.Fatal("replace shortcut did not open replacement field")
	}
	m.Update(tea.PasteMsg{Content: "new"})
	if m.search.replacement.Value() != "new" || m.search.query.Value() != "old" {
		t.Fatal("paste went to wrong field")
	}
	for _, right := range []bool{false, true} {
		m.SetSidebarLayout(36, right)
		for _, width := range []int{30, 69, 70, 120} {
			m.Update(tea.WindowSizeMsg{Width: width, Height: 20})
			lines := m.sidebarLinesWidth(m.searchControlWidth())
			for _, line := range lines {
				if ansi.StringWidth(line) != m.searchControlWidth() {
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
	for _, c := range m.searchPanelControls(m.searchControlWidth()) {
		if c.id == "replaceAll" {
			drain(t, m, m.clickSearch(c.x, c.y))
			break
		}
	}
	if m.search.result.Count != 0 || len(m.tabs) != 2 {
		t.Fatal("replace all button failed")
	}
	for _, c := range m.searchPanelControls(m.searchControlWidth()) {
		if c.id == "replaceToggle" {
			m.clickSearch(c.x, c.y)
			break
		}
	}
	if m.search.replace || m.search.focus != 0 {
		t.Fatal("replace toggle did not collapse")
	}

	m.openReplace()
	m.search.query.SetValue("new")
	drain(t, m, m.runSearch())
	m.search.replacement.CharLimit = 0
	m.search.replacement.SetValue(strings.Repeat("x", editor.MaxFileSize))
	before := m.tabs[0].doc.Text
	m.replaceSearch(true)
	if m.tabs[0].doc.Text != before || !strings.Contains(m.notice, "size limit") {
		t.Fatal("oversized replacement partially applied")
	}
}
