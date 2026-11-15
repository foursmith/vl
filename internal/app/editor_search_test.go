package app

import (
	"os"
	"path/filepath"
	"testing"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

func editorBackgroundCount(m *Model, bg string) int {
	count := 0
	for _, row := range m.editorLines() {
		for _, cell := range uv.NewStyledString(row).Lines(ansi.GraphemeWidth)[0] {
			if sameColor(cell.Style.Bg, lipgloss.Color(bg)) {
				count++
			}
		}
	}
	return count
}

func TestEditorHighlightsAllFileSearchMatchesAndPreservesActiveSelection(t *testing.T) {
	m := searchWorkspace(t)
	defer m.Close()
	os.WriteFile(filepath.Join(m.repo.Root, "file0.go"), []byte("猫 old old\nold 世界\n"), 0644)
	m.search.query.SetValue("old")
	drain(t, m, m.openSearch(false))
	m.search.selected = 1
	drain(t, m, m.openSearchResult())
	for _, mode := range []string{"dark", "light"} {
		drain(t, m, m.SetThemeMode(mode))
		if got := editorBackgroundCount(m, m.colors.SearchMatch); got != 6 {
			t.Fatalf("%s: only %d nonselected match cells highlighted, want 6", mode, got)
		}
		if m.document().SelectedText() != "old" {
			t.Fatal("all-match highlighting disturbed active selection")
		}
	}
	d := m.document()
	d.SelectRange(0, 0)
	if got := editorBackgroundCount(m, m.colors.SearchMatch); got != 9 {
		t.Fatalf("clearing selection did not reveal every match: %d", got)
	}
	d.Move("ctrl+end", false)
	d.Insert("old")
	if got := editorBackgroundCount(m, m.colors.SearchMatch); got != 12 {
		t.Fatalf("editing left stale match positions: %d", got)
	}
	m.search.query.SetValue("世界")
	drain(t, m, m.runSearch())
	if got := editorBackgroundCount(m, m.colors.SearchMatch); got != 2 {
		t.Fatalf("changed Unicode query retained old highlights: %d", got)
	}
	drain(t, m, m.selectSection(explorer))
	if got := editorBackgroundCount(m, m.colors.SearchMatch); got != 0 {
		t.Fatalf("leaving Search kept %d highlighted cells", got)
	}
}

func TestEditorSearchHighlightsFollowHorizontalScrollAndRegexOptions(t *testing.T) {
	m := searchWorkspace(t)
	defer m.Close()
	os.WriteFile(filepath.Join(m.repo.Root, "file0.go"), []byte("aaa e\u0301 e\u0301 OLD old tail\nold\n"), 0644)
	m.search.query.SetValue("old")
	drain(t, m, m.openSearch(false))
	m.search.selected = 1
	drain(t, m, m.openSearchResult())
	m.document().SelectRange(0, 0)
	m.tabs[m.activeTab].left = 7
	if got := editorBackgroundCount(m, m.colors.SearchMatch); got != 6 {
		t.Fatalf("scroll lost match byte offsets: %d", got)
	}
	m.search.options.Case = true
	drain(t, m, m.runSearch())
	if got := editorBackgroundCount(m, m.colors.SearchMatch); got != 3 {
		t.Fatalf("case option did not invalidate highlights: %d", got)
	}
	m.tabs[m.activeTab].left = 0
	m.search.options.Regex = true
	m.search.query.SetValue("^old$")
	drain(t, m, m.runSearch())
	if got := editorBackgroundCount(m, m.colors.SearchMatch); got != 3 {
		t.Fatalf("multiline regex highlighting: %d", got)
	}
	m.search.query.SetValue("[")
	drain(t, m, m.runSearch())
	if got := editorBackgroundCount(m, m.colors.SearchMatch); got != 0 {
		t.Fatal("invalid query retained highlights")
	}
	m.search.options.Regex = false
	m.search.options.Case = false
	m.search.query.SetValue("\u0301")
	drain(t, m, m.runSearch())
	if got := editorBackgroundCount(m, m.colors.SearchMatch); got != 2 {
		t.Fatalf("combining matches did not highlight whole graphemes: %d", got)
	}
}
