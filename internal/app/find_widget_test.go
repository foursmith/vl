package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

func TestSharedFindComponentEntriesCountAndInput(t *testing.T) {
	m := replacementWorkspace(t)
	m.clickSearch(7, m.searchResultY())
	d := m.document()
	original := d.Text
	if m.fileFindHeight() != 0 || m.findHidden || m.fileFindIndex() != 0 || len(m.fileFindCache().ranges) != 2 {
		t.Fatal("workspace result must highlight matches without opening file find")
	}
	press(m, "enter")
	press(m, "n")
	if m.fileFindHeight() != 0 || m.fileFindIndex() != 1 {
		t.Fatal("next workspace match must stay in the editor")
	}
	press(m, "ctrl+f")
	m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if !strings.Contains(plain(m.fileFindLines()[0]), "1 of 2") {
		t.Fatal("match counter missing")
	}
	m.navigateFileFind(1)
	if m.fileFindIndex() != 1 {
		t.Fatal("next did not share selection")
	}
	m.navigateFileFind(-1)
	if m.fileFindIndex() != 0 {
		t.Fatal("previous did not share selection")
	}
	press(m, "ctrl+f")
	if m.modal != "" || !m.fileFindFocused() || m.search.query.Value() != "old" {
		t.Fatal("Ctrl+F did not reuse component")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	press(m, "enter")
	if m.fileFindFocused() || m.fileFindIndex() != 1 {
		t.Fatal("accept reset explicit match navigation")
	}
	press(m, "ctrl+f")
	m.search.query.SetValue("")
	m.Update(tea.PasteMsg{Content: "猫"})
	if m.findOptions.Query != "猫" || m.search.query.Value() != "猫" || len(m.fileFindCache().ranges) != 1 || d.Text != original {
		t.Fatal("query input was not shared or edited source")
	}
	press(m, "enter")
	if m.fileFindFocused() || d.SelectedText() != "猫" {
		t.Fatal("accept did not return to code")
	}
	m.closeFileFind()
	press(m, "/")
	if !m.fileFindFocused() || m.search.query.Value() != "猫" {
		t.Fatal("Vim search did not reopen same component")
	}
	press(m, "esc")
	press(m, "n")
	if !m.findWidget.visible || m.findHidden || m.fileFindFocused() {
		t.Fatal("n did not restore shared component")
	}
}

func TestSharedFindCompactRowsAndUnselectedCount(t *testing.T) {
	m := replacementWorkspace(t)
	m.clickSearch(7, m.searchResultY())
	m.search.replace = false
	m.openFileFind()
	if m.fileFindHeight() != 3 {
		t.Fatal("find should include the count in its three rows")
	}
	m.document().SelectRange(0, 0)
	line := plain(m.fileFindLines()[0])
	if strings.Contains(line, "0/2") || !strings.Contains(line, "2 matches") || !strings.HasPrefix(line, "╭") || !strings.HasSuffix(line, "╮") {
		t.Fatalf("unselected count is misleading: %s", line)
	}
	m.openFileReplace()
	if m.fileFindHeight() != 5 {
		t.Fatal("expanded find should include the count in its five rows")
	}
	lines := m.fileFindLines()
	divider := strings.Repeat("─", m.fileFindWidth()-searchFieldInset-1) + "┤"
	if ansi.Cut(plain(lines[fileFindQueryRow+1]), searchFieldInset, m.fileFindWidth()) != divider {
		t.Fatal("find and replace must have a divider beside the toggle")
	}
	if bottom := plain(lines[len(lines)-1]); !strings.HasPrefix(bottom, "╰") || !strings.HasSuffix(bottom, "╯") {
		t.Fatal("find box must have a rounded bottom border")
	}
	for y, line := range lines {
		cells := uv.NewStyledString(line).Lines(ansi.GraphemeWidth)[0]
		for x, cell := range cells {
			if !sameColor(cell.Style.Bg, lipgloss.Color(m.colors.Panel)) {
				t.Fatalf("find box background interrupted at (%d, %d)", x, y)
			}
		}
	}
}

func TestSharedFindEnglishControlsAndInlineOptions(t *testing.T) {
	m := replacementWorkspace(t)
	m.SetDisplayLanguage("zh-CN")
	m.clickSearch(7, m.searchResultY())
	m.openFileFind()
	if !strings.Contains(plain(m.fileFindLines()[0]), "of") {
		t.Fatal("search labels should remain English")
	}
	for _, control := range m.fileFindControls(m.fileFindWidth()) {
		if control.id == "case" {
			before := m.search.options.Case
			m.clickFileFind(m.mainWidth()-m.fileFindWidth()+control.x, control.y)
			if m.search.options.Case == before || m.findOptions.Case != m.search.options.Case {
				t.Fatal("inline option did not update shared matcher")
			}
			return
		}
	}
	t.Fatal("inline case control missing")
}
func TestSharedFindReplacesOnlyCurrentFileWithUndo(t *testing.T) {
	m := replacementWorkspace(t)
	first := filepath.Join(m.repo.Root, "file0.go")
	if err := os.WriteFile(first, []byte("OLD Old old\n"), 0644); err != nil {
		t.Fatal(err)
	}
	drain(t, m, m.runSearch())
	m.clickSearch(7, m.searchResultY())
	m.openFileReplace()
	m.search.replacement.SetValue("new")
	m.search.options.PreserveCase = true
	drain(t, m, m.replaceFileFind(true))
	d := m.document()
	if d.Text != "NEW New new\n" || !d.Dirty() || len(m.tabs) != 1 {
		t.Fatal("current-file replacement affected scope", d.Text)
	}
	for _, name := range []string{"file0.go", "file1.go"} {
		data, _ := os.ReadFile(filepath.Join(m.repo.Root, name))
		if strings.Contains(string(data), "new") || strings.Contains(string(data), "NEW") {
			t.Fatal("replacement saved automatically")
		}
	}
	d.Undo()
	if d.Text != "OLD Old old\n" || len(m.fileFindCache().ranges) != 3 {
		t.Fatal("undo did not restore counter and matches")
	}
	m.search.replacement.SetValue("")
	m.findWidget.anchor = 0
	m.navigateFileFind(0)
	drain(t, m, m.replaceFileFind(false))
	if d.Text != " Old old\n" || len(m.fileFindCache().ranges) != 2 {
		t.Fatal("single empty replacement failed", d.Text)
	}
}
func TestSharedFindOptionsStayInEditorAndReadonlyDiff(t *testing.T) {
	for _, readonly := range []bool{false, true} {
		m := tabWorkspace(t, 1)
		t.Cleanup(m.Close)
		if readonly {
			m.patch = benchmarkPatch()
			m.diffPath = "file.go"
			showDiffFixture(m)
			m.renderDiff()
		} else {
			m.openFile("file0.go")
		}
		d := m.editorDocument()
		original := d.Text
		m.openFileFind()
		m.search.query.SetValue("value")
		m.adoptSearchState()
		m.navigateFileFind(0)
		height := m.fileFindHeight()
		m.openSearchOptions()
		if !m.fileSearchOptionsDialog || m.sidebar || m.modal != "searchOptions" {
			t.Fatal("options escaped into workspace search")
		}
		for _, a := range m.palette {
			if a.id == "search.option.changed" || a.id == "search.option.open" || a.id == "search.option.ignore" {
				t.Fatal("workspace scope leaked into file options")
			}
		}
		for i, a := range m.palette {
			if a.id == "search.option.regex" {
				m.selectPickerIndex(i)
				drain(t, m, m.selectPickerChoice())
				break
			}
		}
		if !m.search.options.Regex || !m.findOptions.Regex {
			t.Fatal("options were not shared")
		}
		press(m, "esc")
		if readonly {
			m.search.replace = true
			m.resize()
			m.search.replacement.SetValue("MUTATED")
			m.replaceFileFind(true)
			if d.Text != original || m.fileFindHeight() != height {
				t.Fatal("read-only replacement was allowed")
			}
		}
	}
}
func TestSharedFindGeometryAndMouseOffsets(t *testing.T) {
	m := replacementWorkspace(t)
	m.clickSearch(7, m.searchResultY())
	m.openFileReplace()
	for _, right := range []bool{false, true} {
		m.SetSidebarLayout(36, right)
		for _, width := range []int{30, 69, 70, 120} {
			for _, height := range []int{8, 12, 24} {
				m.Update(tea.WindowSizeMsg{Width: width, Height: height})
				m.sidebar = false
				view := m.View()
				if len(strings.Split(view.Content, "\n")) != height {
					t.Fatal("widget changed frame height")
				}
				for _, line := range m.fileFindLines() {
					if ansi.StringWidth(line) != m.fileFindWidth() {
						t.Fatal("widget width overflow")
					}
				}
				if m.contentRow() != editorContentRow || m.contentHeight() != m.bodyHeight()-editorContentRow {
					t.Fatal("find overlay displaced the code viewport")
				}
				for _, c := range m.fileFindControls(m.fileFindWidth()) {
					if c.y >= m.fileFindHeight() || c.x+c.width > m.fileFindWidth() {
						t.Fatal("offscreen widget control")
					}
				}
				if line := plain(m.fileFindLines()[fileFindQueryRow]); !strings.Contains(line, "↑ ↓") || !strings.Contains(line, "×") {
					t.Fatalf("navigation and close must remain visible at width %d: %q", width, line)
				}
			}
		}
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	m.closeFileFind()
	beforeOpen := strings.Split(m.View().Content, "\n")[m.contentRow()]
	m.openFileReplace()
	afterOpen := strings.Split(m.View().Content, "\n")[m.contentRow()]
	if plain(ansi.Cut(beforeOpen, m.mainX(), m.fileFindX())) != plain(ansi.Cut(afterOpen, m.mainX(), m.fileFindX())) {
		t.Fatal("opening find changed the visible code beside it")
	}
	m.blurFileFind()
	m.sidebar = false
	m.click(tea.MouseClickMsg{X: m.fileFindX() + 2, Y: m.contentRow() + fileFindQueryRow, Button: tea.MouseLeft})
	if !m.fileFindFocused() {
		t.Fatal("click did not focus shared query")
	}
	before := m.document().Text
	press(m, "ctrl+w")
	if len(m.tabs) != 1 || m.document().Text != before {
		t.Fatal("query editing closed or changed source")
	}
	m.click(tea.MouseClickMsg{X: m.mainX() + 7, Y: m.contentRow(), Button: tea.MouseLeft})
	if m.fileFindFocused() || m.editorCursor() == nil {
		t.Fatal("click did not return focus to code")
	}
}

func TestSharedFindSwitchToNewDiffUpdatesViewportHeight(t *testing.T) {
	m := replacementWorkspace(t)
	m.clickSearch(7, m.searchResultY())
	m.openFileFind()
	m.patch = benchmarkPatch()
	m.diffPath = "file.go"
	showDiffFixture(m)
	m.renderDiff()
	if m.viewport.Height() != m.contentHeight() {
		t.Fatal("find overlay changed the diff viewport height")
	}
	last := len(m.review.Lines()) - 1
	m.review.SetPosition(last, 0)
	m.ensureTextCursor()
	row := m.reviewRows[last]
	if row >= m.viewport.YOffset()+m.contentHeight() {
		t.Fatal("last diff row cannot be reached with find widget visible")
	}
}

func TestSharedFindInputOwnerKeepsOwnWidth(t *testing.T) {
	m := replacementWorkspace(t)
	m.clickSearch(7, m.searchResultY())
	m.openFileFind()
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 24})
	m.search.query.SetValue(strings.Repeat("needle", 40))
	m.adoptSearchState()
	m.fileFindLines()
	inputWidth := m.search.query.Width()
	m.View()
	if m.search.query.Width() != inputWidth || inputWidth >= m.mainWidth()-4 {
		t.Fatal("sidebar changed editor find input width")
	}
	m.sidebar = true
	m.focusSearch(0)
	m.searchLines(nil, m.sidebarContentWidth())
	inputWidth = m.search.query.Width()
	m.View()
	if m.search.query.Width() != inputWidth {
		t.Fatal("editor changed sidebar find input width")
	}
}

func TestSharedFindUndoRefreshesWorkspaceResults(t *testing.T) {
	m := replacementWorkspace(t)
	m.clickSearch(7, m.searchResultY())
	m.openFileReplace()
	m.search.replacement.SetValue("new")
	for _, control := range m.fileFindControls(m.fileFindWidth()) {
		if control.id == "replaceAll" {
			drain(t, m, m.clickFileFind(m.mainWidth()-m.fileFindWidth()+control.x, control.y))
			break
		}
	}
	if m.search.result.Count != 2 {
		t.Fatal("file replacement did not refresh workspace scope")
	}
	press(m, "esc")
	drain(t, m, press(m, "u"))
	if m.search.result.Count != 4 || !m.findHidden {
		t.Fatal("undo left stale workspace results or restored hidden highlights")
	}
}
