package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
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
	m.openReplace()
	m.toggleSearchDetails()
	if !m.search.details || m.modal != "" {
		t.Fatal("details must expand in the panel")
	}
	layout := m.searchPanelLayout()
	lines := m.searchLines(nil, 36)
	if !strings.HasPrefix(plain(lines[layout.include]), "In ") || !strings.HasPrefix(plain(lines[layout.exclude]), "Ex ") {
		t.Fatal("path field labels must start in the left gutter")
	}
	m.clickSearch(3, layout.include)
	m.Update(tea.PasteMsg{Content: "*.go"})
	if m.search.include.Value() != "*.go" {
		t.Fatal("include field did not receive input")
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
