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

func clickSearchControl(t *testing.T, m *Model, id string) tea.Cmd {
	t.Helper()
	if m.modal != "" {
		m.updateModal(tea.KeyPressMsg{Code: tea.KeyEscape})
	}
	for _, c := range m.searchPanelControls(m.searchControlWidth()) {
		if c.id == id {
			return m.clickSearch(c.x, c.y)
		}
	}
	m.openSearchOptions()
	for i, a := range m.palette {
		if a.id == "search.option."+id {
			m.selectPickerIndex(i)
			return m.selectPickerChoice()
		}
	}

	t.Fatal("control missing", id)
	return nil
}

func TestSearchDetailsInlineFieldsAndFocus(t *testing.T) {
	m := replacementWorkspace(t)
	th, _ := fixtureTheme(t, "vs-dark", `{"colors":{"sideBar.background":"#181818","quickInput.background":"#222222"}}`)
	m.SetColorTheme(th)
	m.openReplace()
	m.toggleSearchDetails()
	m.search.options.Word = true
	if !m.search.details || m.modal != "" {
		t.Fatal("details must expand in the panel")
	}
	layout := m.searchPanelLayout()
	lines := m.searchLines(nil, 36)
	if layout.summary-layout.details-1 != 5 {
		t.Fatal("path filters should share a five-row box")
	}
	divider := strings.Repeat("─", 34-searchFieldInset) + "┤"
	if ansi.Cut(plain(lines[layout.query+1]), searchFieldInset, 35) != divider || ansi.Cut(plain(lines[layout.include+1]), searchFieldInset, 35) != divider {
		t.Fatal("paired fields must have a divider across the box")
	}
	for y := headerHeight; y < layout.summary; y++ {
		cells := uv.NewStyledString(lines[y]).Lines(ansi.GraphemeWidth)[0]
		for x, cell := range cells {
			if !sameColor(cell.Style.Bg, lipgloss.Color(m.colors.Panel)) {
				t.Fatalf("search panel background interrupted at (%d, %d): %#v", x, y, cell)
			}
		}
	}
	if !strings.HasPrefix(plain(lines[layout.details+1]), "╭") || !strings.HasPrefix(plain(lines[layout.summary-1]), "╰") {
		t.Fatal("path filter box must have rounded corners")
	}
	if !strings.HasPrefix(plain(lines[layout.include]), "│⊕ ") || !strings.HasPrefix(plain(lines[layout.exclude]), "│⊖ ") {
		t.Fatal("path filter symbols must be inset inside the box")
	}
	m.clickSearch(3, layout.include)
	m.Update(tea.PasteMsg{Content: "*.go"})
	if m.search.include.Value() != "*.go" {
		t.Fatal("include field did not receive input")
	}
	if line := plain(m.searchLines(nil, 36)[layout.include]); !strings.HasPrefix(line, "│⊕ *.go") {
		t.Fatalf("filter symbol needs a space before the input: %q", line)
	}
	m.searchKey(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.search.focus != 4 {
		t.Fatal("Tab must move from include to exclude")
	}
	m.toggleSearchDetails()
	if m.search.focus != 0 || m.search.include.Value() != "*.go" {
		t.Fatal("collapsing details lost filters or focus")
	}
}

func TestSearchFilterIconsKeepTheSidebarBackgroundWhenSelected(t *testing.T) {
	m := replacementWorkspace(t)
	m.nerd = true
	m.openSearch()
	m.toggleSearchDetails()
	m.search.options.ChangedOnly = true
	m.search.options.OnlyOpen = true
	lines := m.searchLines(nil, 36)
	for _, c := range m.searchPanelControls(36) {
		switch c.id {
		case "changed", "open", "ignore":
			cells := uv.NewStyledString(lines[c.y]).Lines(ansi.GraphemeWidth)[0]
			if cell := cells[c.x]; !sameColor(cell.Style.Fg, lipgloss.Color(m.colors.Accent)) || !sameColor(cell.Style.Bg, lipgloss.Color(m.colors.Panel)) {
				t.Fatalf("selected %s icon must use the accent on the sidebar background", c.id)
			}
		}
	}
}

func TestCompleteSearchPanelScopesAndPreserveCase(t *testing.T) {
	m := replacementWorkspace(t)
	m.openPreviewFile("file0.go")
	m.openReplace()
	m.search.query.SetValue("old")
	m.search.replacement.SetValue("new")
	// A staged file, an untracked file and an open file exercise independent scopes.
	if _, err := m.repo.Run(m.ctx, "add", "file0.go"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(m.repo.Root, "file0.go")
	if err := os.WriteFile(path, []byte("OLD Old old\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m.document().Edit("OLD Old old\n")
	m.search.include.SetValue("*.go")
	m.search.exclude.SetValue("file1.go")
	drain(t, m, m.runSearch())
	if m.search.result.Count != 3 {
		t.Fatal("path filters failed", m.search.result)
	}
	drain(t, m, clickSearchControl(t, m, "open"))
	if !m.search.options.OnlyOpen || m.search.result.Count != 3 {
		t.Fatal("open editor scope failed")
	}
	drain(t, m, clickSearchControl(t, m, "changed"))
	if !m.search.options.ChangedOnly || m.search.result.Count != 3 {
		t.Fatal("changed scope failed")
	}
	clickSearchControl(t, m, "preserve")
	drain(t, m, clickSearchControl(t, m, "replaceAll"))
	if m.document().Text != "NEW New new\n" || !m.search.options.PreserveCase {
		t.Fatal("preserve case replace all failed", m.document().Text)
	}
	m.document().Undo()
	if m.document().Text != "OLD Old old\n" {
		t.Fatal("not one undo step")
	}
	// Opening and closing options must retain the active scope.
	m.updateModal(tea.KeyPressMsg{Code: tea.KeyEscape})
	m.openSearchOptions()
	if m.modal != "searchOptions" || m.search.include.Value() != "*.go" {
		t.Fatal("options did not retain filters")
	}
	m.updateModal(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.modal != "" {
		t.Fatal("options did not dismiss")
	}

}
func TestSearchPanelFieldFocusMouseAndSmallSizes(t *testing.T) {
	m := replacementWorkspace(t)
	m.openReplace()
	for _, right := range []bool{false, true} {
		m.SetSidebarLayout(36, right)
		for _, height := range []int{8, 10, 12, 24} {
			for _, width := range []int{30, 69, 70, 120} {
				m.Update(tea.WindowSizeMsg{Width: width, Height: height})
				lines := m.sidebarLinesWidth(m.searchControlWidth())
				if len(lines) != m.bodyHeight() {
					t.Fatal("panel height overflow")
				}
				for _, line := range lines {
					if ansi.StringWidth(line) != m.searchControlWidth() {
						t.Fatal("panel width overflow")
					}
				}
				if len(strings.Split(m.View().Content, "\n")) != height {
					t.Fatal("frame height overflow")
				}
				for _, c := range m.searchPanelControls(m.searchControlWidth()) {
					if c.y >= m.bodyHeight() || c.x < 0 || c.x+c.width > m.searchControlWidth() {
						t.Fatal("invisible control", c)
					}
				}
			}
		}
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	m.toggleSearchDetails()
	layout := m.searchPanelLayout()
	m.clickSearch(3, layout.include)
	drain(t, m, m.searchInput(tea.PasteMsg{Content: "*.go"}))
	m.clickSearch(3, layout.exclude)
	drain(t, m, m.searchInput(tea.PasteMsg{Content: "file1.go"}))
	if m.search.result.Count != 2 || len(m.search.result.Files) != 1 {
		t.Fatal("field values were not applied")
	}
	// Invalid glob input reports an error rather than broadening the scope.
	m.search.include.SetValue("[bad")
	drain(t, m, m.runSearch())
	if m.search.err == "" || m.search.loading {
		t.Fatal("invalid glob not reported")
	}
}

func TestSearchUsesInlineFieldsAndActions(t *testing.T) {
	m := replacementWorkspace(t)
	m.openReplace()
	layout := m.searchPanelLayout()
	if layout.details-headerHeight != 5 {
		t.Fatal("expanded search and replacement should share a five-row box")
	}
	if layout.replacement != layout.query+2 {
		t.Fatal("search and replacement inputs need a blank row between them")
	}
	for _, control := range m.searchPanelControls(36) {
		switch control.id {
		case "replaceToggle":
			if control.y != layout.query+1 {
				t.Fatal("expanded toggle must sit between inputs")
			}
		case "case", "word", "regex", "preserve":
			if !control.embedded {
				t.Fatal("matching option must be inside its input")
			}
			m.hoverX, m.hoverY = m.sideX()+control.x, control.y
			line := m.searchLines(nil, 36)[control.y]
			cells := uv.NewStyledString(line).Lines(ansi.GraphemeWidth)[0]
			if underlined := cells[control.x].Style.Underline != uv.UnderlineNone; underlined != (control.id == "word") {
				t.Fatalf("hover changed the underline on %q", control.label)
			}
		case "details":
			if control.y <= layout.replacement {
				t.Fatal("more options must be below replacement")
			}
		case "replace":
			t.Fatal("workspace form only shows replace all beside replacement")
		}
	}
	for _, width := range []int{28, 36, 60} {
		m.search.focus = 0
		m.search.query.Focus()
		lines := m.searchLines(nil, width)
		m.search.focus = 1
		m.search.replacement.Focus()
		m.searchLines(nil, width)
		if m.search.query.Width() < 8 || m.search.replacement.Width() < 8 {
			t.Fatal("inline controls left insufficient input space")
		}
		query := plain(lines[m.searchPanelLayout().query])
		if !strings.Contains(query, "Aa") || !strings.Contains(query, ".*") {
			t.Fatal("inline matching options missing")
		}
		actions := plain(lines[m.searchPanelLayout().replacement])
		if !strings.Contains(actions, "AB") || !strings.Contains(actions, "↵") {
			t.Fatal("inline replacement actions missing", actions)
		}
	}
	// Filtering and toggling keep the same dialog and selected row.
	m.openSearchOptions()
	m.input.SetValue("case")
	m.selectPickerIndex(0)
	drain(t, m, m.selectPickerChoice())
	if m.modal != "searchOptions" || m.input.Value() != "case" || !m.search.options.Case || !strings.Contains(m.filteredActions()[0].label, "[x]") {
		t.Fatal("option toggle lost dialog state")
	}
}
