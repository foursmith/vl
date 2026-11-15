package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	searchservice "github.com/ourongxing/elegit/internal/search"
)

func searchWorkspace(t *testing.T) *Model {
	t.Helper()
	m := tabWorkspace(t, 2)
	root, err := filepath.EvalSymlinks(m.repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	m.repo.Root = root
	if out, err := exec.Command("git", "init", root).CombinedOutput(); err != nil {
		t.Fatalf("%s %v", out, err)
	}
	for _, name := range []string{"file0.go", "file1.go"} {
		os.WriteFile(filepath.Join(root, name), []byte("猫 old old\n"), 0644)
	}
	return m
}
func TestSearchReplaceBuffersUndoAndSave(t *testing.T) {
	m := searchWorkspace(t)
	defer m.Close()
	m.openPreviewFile("file0.go")
	m.document().Insert("old ")
	original := m.document().Text
	m.search.query.SetValue("old")
	drain(t, m, m.openSearch(true))
	if m.search.result.Count != 5 {
		t.Fatalf("buffer not searched: %+v", m.search.result)
	}
	// Jump to a Unicode match using rune, rather than byte, offsets.
	m.search.selected = 1
	m.search.focus = 2
	drain(t, m, m.openSearchResult())
	if m.document().SelectedText() != "old" {
		t.Fatal("result selection failed")
	}
	m.search.replacement.SetValue("new")
	drain(t, m, m.replaceSearch(true))
	if len(m.tabs) != 2 {
		t.Fatalf("replacement tabs: %d", len(m.tabs))
	}
	for _, tab := range m.tabs {
		if strings.Contains(tab.doc.Text, "old") || !tab.doc.Dirty() || tab.preview {
			t.Fatal("replace failed to keep dirty buffer")
		}
		disk, _ := os.ReadFile(tab.doc.Path)
		if string(disk) != "猫 old old\n" {
			t.Fatal("replace wrote disk")
		}
	}
	m.tabs[0].doc.Undo()
	if m.tabs[0].doc.Text != original {
		t.Fatal("replace was not one undo step")
	}
	m.tabs[0].doc.Redo()
	m.sidebar = false
	drain(t, m, m.save())
	data, _ := os.ReadFile(m.document().Path)
	if strings.Contains(string(data), "old") {
		t.Fatal("save failed")
	}
}
func TestSearchStaleResultsAndAtomicReplacement(t *testing.T) {
	m := searchWorkspace(t)
	defer m.Close()
	m.search.query.SetValue("old")
	drain(t, m, m.openSearch(true))
	stale := searchMsg{seq: m.search.seq - 1, result: searchservice.Result{Count: 999}}
	m.Update(stale)
	if m.search.result.Count != 4 {
		t.Fatal("stale worker applied")
	}
	os.WriteFile(filepath.Join(m.repo.Root, "file1.go"), []byte("external change"), 0644)
	m.search.replacement.SetValue("new")
	drain(t, m, m.replaceSearch(true))
	if len(m.tabs) != 0 {
		t.Fatal("partially applied stale replacement")
	}
	if !strings.Contains(m.notice, "changed") {
		t.Fatalf("missing stale notice: %q", m.notice)
	}
	// Empty replacement is deletion; replacing a selected row affects just that occurrence.
	m.search.selected = 1
	m.search.replacement.SetValue("")
	drain(t, m, m.replaceSearch(false))
	if m.document().Text != "猫  old\n" {
		t.Fatalf("single deletion: %q", m.document().Text)
	}
}
func TestSearchLayoutKeyboardMouseAndIncompleteSearch(t *testing.T) {
	m := searchWorkspace(t)
	defer m.Close()
	m.Update(tea.KeyPressMsg{Code: 'h', Mod: tea.ModCtrl | tea.ModShift})
	if m.section != search || !m.search.replace {
		t.Fatal("search shortcut missed")
	}
	m.Update(tea.PasteMsg{Content: "old"})
	drain(t, m, m.runSearch())
	if m.search.result.Count != 4 {
		t.Fatal("paste not searched")
	}
	for _, right := range []bool{false, true} {
		m.sidebarRight = right
		for _, width := range []int{30, 70, 120} {
			m.Update(tea.WindowSizeMsg{Width: width, Height: 20})
			w := m.sideWidth()
			if !m.sideShown() {
				w = width
			}
			for _, line := range m.sidebarLinesWidth(w) {
				if ansi.StringWidth(line) != w {
					t.Fatalf("row width %d != %d", ansi.StringWidth(line), w)
				}
			}
			m.clickSidebar(tea.MouseClickMsg{X: m.sideX() + 2, Y: 3, Button: tea.MouseLeft})
			if !m.search.options.Case {
				t.Fatal("option hitbox missed")
			}
			m.search.options.Case = false
		}
	}
	m.search.result.Unreadable = 1
	m.search.replacement.SetValue("new")
	m.replaceSearch(true)
	if len(m.tabs) > 0 {
		t.Fatal("incomplete results replaced")
	}
	m.search.query.SetValue("[")
	m.search.options.Regex = true
	drain(t, m, m.runSearch())
	if m.search.err == "" || m.search.loading {
		t.Fatal("invalid regex state")
	}
}

func TestSearchResultScrollingAndReplacementHitboxes(t *testing.T) {
	m := searchWorkspace(t)
	defer m.Close()
	m.search.query.SetValue("old")
	drain(t, m, m.openSearch(true))
	m.search.replacement.SetValue("new")
	m.search.selected = 1
	m.search.focus = 2
	m.searchKey(tea.KeyPressMsg{Code: tea.KeyLeft})
	rows := m.searchRows()
	if rows[m.search.selected].match != -1 || rows[m.search.selected].file != 0 {
		t.Fatal("collapse lost parent selection")
	}
	selected := m.search.selected
	m.Update(tea.MouseWheelMsg{X: m.sideX() + 2, Y: 10, Button: tea.MouseWheelDown})
	if selected != m.search.selected || len(m.tabs) != 0 {
		t.Fatal("wheel selected or opened a result")
	}
	m.clickSearch(28, m.searchResultY()-2)
	if len(m.tabs) != 0 {
		t.Fatal("blank control space replaced files")
	}
	drain(t, m, m.clickSearch(3, m.searchResultY()-2))
	if len(m.tabs) != 1 || m.document().Text != "猫 new new\n" {
		t.Fatal("replace file button failed")
	}
	// The second file remains unchanged until the All button is used.
	drain(t, m, m.clickSearch(14, m.searchResultY()-2))
	if len(m.tabs) != 2 {
		t.Fatal("replace all button failed")
	}
}

func TestSearchMoreThanTwoThousandReplacementsAndIgnoredToggle(t *testing.T) {
	m := searchWorkspace(t)
	defer m.Close()
	body := strings.Repeat("old\n", 3000)
	os.WriteFile(filepath.Join(m.repo.Root, "file0.go"), []byte(body), 0644)
	os.WriteFile(filepath.Join(m.repo.Root, ".gitignore"), []byte("ignored.txt\n"), 0644)
	os.WriteFile(filepath.Join(m.repo.Root, "ignored.txt"), []byte("old"), 0644)
	m.search.query.SetValue("old")
	drain(t, m, m.openSearch(true))
	if m.search.result.Count != 3002 || m.search.result.Unreadable > 0 {
		t.Fatal("workspace results were truncated")
	}
	drain(t, m, m.clickSearch(20, 3))
	if !m.search.options.IncludeIgnored || m.search.result.Count != 3003 {
		t.Fatal("include ignored click did not widen search")
	}
	m.hoverX = m.sideX() + 20
	m.hoverY = 3
	if !strings.Contains(m.hoverHint(), "Ignored") {
		t.Fatal("ignore toggle missing hover description")
	}
	m.search.replacement.SetValue("new")
	drain(t, m, m.replaceSearch(true))
	if len(m.tabs) != 3 || m.search.result.Count != 0 {
		t.Fatal("not all matches replaced")
	}
	for _, tab := range m.tabs {
		if strings.Contains(tab.doc.Text, "old") {
			t.Fatal("replacement was truncated")
		}
		tab.doc.Undo()
	}
	if m.tabs[0].doc.Text != body {
		t.Fatal("full replacement cannot be undone")
	}
	m.sidebar = true
	drain(t, m, m.searchKey(tea.KeyPressMsg{Code: 'i', Mod: tea.ModAlt}))
	if m.search.options.IncludeIgnored || m.search.result.Count != 3002 {
		t.Fatal("ignore shortcut did not restore scope")
	}
}
