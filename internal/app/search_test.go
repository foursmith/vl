package app

import (
	tea "charm.land/bubbletea/v2"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/editor"
	"github.com/foursmith/vl/internal/theme"
)

func hasFindBackground(p theme.Palette, cell uv.Cell) bool {
	for _, bg := range []string{p.Background, p.CurrentLine} {
		_, passive, _ := p.FindColors(p.Foreground, bg, false)
		_, current, _ := p.FindColors(p.Foreground, theme.Composite(p.EditorSelection, bg), true)
		if sameColor(cell.Style.Bg, lipgloss.Color(passive)) || sameColor(cell.Style.Bg, lipgloss.Color(current)) {
			return true
		}
	}
	return false
}

func setTestFind(m *Model, query string) {
	m.findWidget.query.SetValue(query)
	m.findWidget.options.Case = true
	m.findHidden = false
}

func TestSearchRanges(t *testing.T) {
	for _, tt := range []struct {
		text, query string
		want        [][2]int
	}{
		{"one one", "one", [][2]int{{0, 3}, {4, 7}}},
		{"中🙂中🙂", "🙂", [][2]int{{1, 2}, {3, 4}}},
		{"aaaa", "aa", [][2]int{{0, 2}, {2, 4}}},
		{"word", "", nil}, {"word", "Word", nil},
	} {
		if got := searchRanges(tt.text, tt.query); !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("%q / %q: %v", tt.text, tt.query, got)
		}
	}
}

func TestSearchHighlightsAllAndClearsUntilNext(t *testing.T) {
	m := newHoverRenderModel(t)
	d := &editor.Document{Buffer: editor.Buffer{Text: "one one\none one", Anchor: -1}}
	m.tabs = []tab{{doc: d}}
	m.activeTab = 0
	m.sidebar = false

	setTestFind(m, "one")
	m.findNext()
	check := func(want int) {
		t.Helper()
		count := 0
		for _, row := range m.editorLines()[:2] {
			for _, cell := range uv.NewStyledString(row).Lines(ansi.GraphemeWidth)[0] {
				if hasFindBackground(m.colors, cell) {
					count++
				}
			}
		}
		if count != want {
			t.Fatalf("highlighted cells = %d, want %d", count, want)
		}
	}
	check(12)
	press(m, "ctrl+f")
	press(m, "esc")
	check(0)
	press(m, "n")
	check(12)
	// Query and text changes must invalidate the cached rows too.
	setTestFind(m, "two")
	check(0)
	d.Text = strings.ReplaceAll(d.Text, "one", "two")
	check(12)
}

func TestLocalSearchCentersCtrlFSlashAndNext(t *testing.T) {
	m := tabWorkspace(t, 1)
	t.Cleanup(m.Close)
	m.openFile("file0.go")
	lines := make([]string, 120)
	for i := range lines {
		lines[i] = "source"
	}
	for _, row := range []int{0, 45, 75, 119} {
		lines[row] = "needle"
	}
	d := m.document()
	d.Edit(strings.Join(lines, "\n"))
	d.SetPosition(0, 0)
	check := func(wantRow int) {
		t.Helper()
		row, _ := d.Position()
		tab := &m.tabs[m.activeTab]
		wantTop := max(0, wantRow-m.contentHeight()/2)
		if row != wantRow || tab.top != wantTop || d.SelectedText() != "needle" {
			t.Fatalf("row=%d top=%d, want %d / %d", row, tab.top, wantRow, wantTop)
		}
	}
	press(m, "ctrl+f")
	m.findWidget.query.SetValue("needle")
	press(m, "enter")
	check(45)
	press(m, "n")
	check(75)
	press(m, "/")
	m.findWidget.query.SetValue("needle")
	press(m, "enter")
	check(75)
	press(m, "n")
	check(119)
	press(m, "n")
	check(0)
}

func TestLocalSearchCentersWithFoldedRows(t *testing.T) {
	m := tabWorkspace(t, 1)
	t.Cleanup(m.Close)
	m.openFile("file0.go")
	lines := make([]string, 120)
	for i := range lines {
		lines[i] = "source"
	}
	lines[60] = "needle"
	d := m.document()
	d.Edit(strings.Join(lines, "\n"))
	d.SetPosition(0, 0)
	tab := &m.tabs[m.activeTab]
	tab.syncCodeScopes()
	tab.folds = []codeScope{{start: 5, end: 30}}
	tab.rebuildFoldRows()
	setTestFind(m, "needle")
	m.findNext()
	row, _ := d.Position()
	if tab.visibleRow(row)-tab.visibleRow(tab.top) != m.contentHeight()/2 {
		t.Fatal("search did not center using visible folded rows")
	}
}

func TestLocalSearchCentersDiffRows(t *testing.T) {
	for _, split := range []bool{false, true} {
		m := tabWorkspace(t, 0)
		t.Cleanup(m.Close)
		m.split = split
		m.patch = benchmarkPatch()
		m.diffPath = "file.go"
		showDiffFixture(m)
		m.renderDiff()
		setTestFind(m, "old value 19")
		if m.diffLayout.split {
			setTestFind(m, "new value 19")
		}
		m.findNext()
		if m.review.SelectedText() != m.findWidget.query.Value() {
			t.Fatal("diff search did not find source text")
		}
		line, _ := m.review.Position()
		row := m.reviewRows[line]
		if row-m.viewport.YOffset() != m.contentHeight()/2 {
			t.Fatalf("split=%v: match row=%d scroll=%d", split, row, m.viewport.YOffset())
		}
	}
}

func TestWorkspaceResultKeepsFileFindIndependent(t *testing.T) {
	for _, test := range []struct {
		query, text, match string
		word, regex        bool
	}{
		{"needle", "needle NEEDLE needleX", "needle", true, false},
		{`id\d+`, "id1 ID22 idX", "id1", false, true},
		{`id1\nID22`, "id1\nID22\nidX", "id1\nID22", false, true},
	} {
		m := tabWorkspace(t, 1)
		t.Cleanup(m.Close)
		if err := os.WriteFile(filepath.Join(m.repo.Root, "file0.go"), []byte(test.text+"\nlocal local"), 0644); err != nil {
			t.Fatal(err)
		}
		m.openFile("file0.go")
		m.openFileFind()
		m.Update(tea.PasteMsg{Content: "local"})
		press(m, "esc")

		m.openSearch()
		m.search.query.SetValue(test.query)
		m.search.options.Word = test.word
		m.search.options.Regex = test.regex
		drain(t, m, m.runSearch())
		m.clickSearch(5, m.searchResultY()+1)
		press(m, "enter")
		if m.document().SelectedText() != test.match || !m.findHidden || m.findWidget.visible ||
			m.findWidget.query.Value() != "local" || m.findWidget.options.Word || m.findWidget.options.Regex {
			t.Fatal("opening a workspace result changed file find")
		}
		press(m, "n")
		if m.document().SelectedText() != "local" || m.findHidden || !m.findWidget.visible {
			t.Fatal("n did not use the file query")
		}
	}
}

func TestLocalSearchNextVisitsAdjacentMatches(t *testing.T) {
	m := tabWorkspace(t, 1)
	t.Cleanup(m.Close)
	if err := os.WriteFile(filepath.Join(m.repo.Root, "file0.go"), []byte("id1ID22id3"), 0644); err != nil {
		t.Fatal(err)
	}
	m.openFile("file0.go")
	m.openFileFind()
	m.toggleFileSearchOption("regex")
	m.findWidget.anchor = 0
	m.Update(tea.PasteMsg{Content: `id\d+`})
	press(m, "enter")
	for _, want := range []string{"ID22", "id3", "id1"} {
		press(m, "n")
		if m.document().SelectedText() != want {
			t.Fatalf("n selected %q, want %q", m.document().SelectedText(), want)
		}
	}
}

func TestSearchFileNameOpensFileAndChevronOnlyFolds(t *testing.T) {
	m := tabWorkspace(t, 1)
	t.Cleanup(m.Close)
	m.openSearch()
	m.search.query.SetValue("package")
	drain(t, m, m.runSearch())
	m.clickSearch(0, m.searchResultY())
	if len(m.tabs) != 0 || len(m.searchRows()) != 1 {
		t.Fatal("chevron opened a file instead of folding")
	}
	m.clickSearch(2, m.searchResultY())
	if m.document() == nil || !m.sidebar || m.findWidget.query.Value() != "" || m.document().SelectedText() != "package" {
		t.Fatal("file name did not select its result independently")
	}
	if len(m.searchRows()) != 1 {
		t.Fatal("file name changed the group fold state")
	}
}
