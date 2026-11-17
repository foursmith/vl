package app

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/vl/internal/editor"
)

func setTestFind(m *Model, query string) {
	m.search.query.SetValue(query)
	m.search.options.Case = true
	m.adoptSearchState()
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
	d := &editor.Document{Text: "one one\none one", Anchor: -1}
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
				if sameColor(cell.Style.Bg, lipgloss.Color(m.colors.FindMatch)) || sameColor(cell.Style.Bg, lipgloss.Color(m.colors.FindMatchHighlight)) {
					count++
				}
			}
		}
		if count != want {
			t.Fatalf("highlighted cells = %d, want %d", count, want)
		}
	}
	check(12)
	press(m, " ")
	press(m, "enter")
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
		wantTop := min(max(0, wantRow-m.contentHeight()/2), 120-m.contentHeight())
		if row != wantRow || tab.top != wantTop || d.SelectedText() != "needle" {
			t.Fatalf("row=%d top=%d, want %d / %d", row, tab.top, wantRow, wantTop)
		}
	}
	press(m, "ctrl+f")
	m.search.query.SetValue("needle")
	press(m, "enter")
	check(45)
	press(m, "n")
	check(75)
	press(m, "/")
	m.search.query.SetValue("needle")
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
		if m.reviewRight {
			setTestFind(m, "new value 19")
		}
		m.findNext()
		if m.review.SelectedText() != m.findOptions.Query {
			t.Fatal("diff search did not find source text")
		}
		line, _ := m.review.Position()
		row := m.reviewRows[line]
		if row-m.viewport.YOffset() != m.contentHeight()/2 {
			t.Fatalf("split=%v: match row=%d scroll=%d", split, row, m.viewport.YOffset())
		}
	}
}

func TestWorkspaceResultReusesFileSearchHighlightsAndNavigation(t *testing.T) {
	for _, tc := range []struct {
		query, text string
		word, regex bool
		cells       int
		next        string
	}{
		{"needle", "needle NEEDLE needleX needle", true, false, 18, "NEEDLE"},
		{`id\d+`, "id1 ID22 idX", false, true, 7, "ID22"},
		{`id1\nID22`, "id1\nID22\nidX", false, true, 7, "id1\nID22"},
	} {
		m := tabWorkspace(t, 1)
		t.Cleanup(m.Close)
		if err := os.WriteFile(filepath.Join(m.repo.Root, "file0.go"), []byte(tc.text), 0644); err != nil {
			t.Fatal(err)
		}
		m.openSearch()
		m.search.query.SetValue(tc.query)
		m.search.options.Word = tc.word
		m.search.options.Regex = tc.regex
		drain(t, m, m.runSearch())
		m.findHidden = true
		m.clickSearch(5, m.searchResultY()+1)
		count := func() int {
			t.Helper()
			total := 0
			for _, line := range m.editorLines() {
				for _, cells := range uv.NewStyledString(line).Lines(ansi.GraphemeWidth) {
					for _, cell := range cells {
						if sameColor(cell.Style.Bg, lipgloss.Color(m.colors.FindMatch)) || sameColor(cell.Style.Bg, lipgloss.Color(m.colors.FindMatchHighlight)) {
							total++
						}
					}
				}
			}
			return total
		}
		if m.findOptions.Query != tc.query || m.findHidden || count() != tc.cells {
			t.Fatalf("%q: file search was not seeded, highlights=%d want=%d", tc.query, count(), tc.cells)
		}
		press(m, "n")
		if m.document().SelectedText() != tc.next {
			t.Fatalf("%q: n selected %q, want %q", tc.query, m.document().SelectedText(), tc.next)
		}
		press(m, " ")
		press(m, "enter")
		if count() != 0 {
			t.Fatal("clear highlight did not use shared state")
		}
		press(m, "n")
		if count() != tc.cells {
			t.Fatal("n did not restore all highlights")
		}
		press(m, "ctrl+f")
		m.search.query.SetValue("idX")
		press(m, "enter")
		if m.findOptions.Regex != tc.regex || m.findOptions.Word != tc.word {
			t.Fatal("local find did not preserve shared options")
		}
	}
}

func TestWorkspaceSearchNextVisitsAdjacentMatches(t *testing.T) {
	m := tabWorkspace(t, 1)
	t.Cleanup(m.Close)
	if err := os.WriteFile(filepath.Join(m.repo.Root, "file0.go"), []byte("id1ID22id3"), 0644); err != nil {
		t.Fatal(err)
	}
	m.openSearch()
	m.search.query.SetValue(`id\d+`)
	m.search.options.Regex = true
	drain(t, m, m.runSearch())
	m.clickSearch(5, m.searchResultY()+1)
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
	m.clickSearch(1, m.searchResultY())
	if len(m.tabs) != 0 || len(m.searchRows()) != 1 {
		t.Fatal("chevron opened a file instead of folding")
	}
	m.clickSearch(7, m.searchResultY())
	if m.document() == nil || m.sidebar || m.findOptions.Query != "package" || m.document().SelectedText() != "package" {
		t.Fatal("file name did not activate file search")
	}
	if len(m.searchRows()) != 1 {
		t.Fatal("file name changed the group fold state")
	}
}
