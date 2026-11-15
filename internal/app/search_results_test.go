package app

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/elegit/internal/theme"
)

func TestSearchResultPresentationAndClipping(t *testing.T) {
	m := searchWorkspace(t)
	defer m.Close()
	os.MkdirAll(filepath.Join(m.repo.Root, "app", "apps", "cli"), 0755)
	path := filepath.Join(m.repo.Root, "app", "apps", "cli", "connection.go")
	os.WriteFile(path, []byte("    "+strings.Repeat("prefix ", 30)+"context.WithTimeout(context.Background()) trailing text\n猫 context 世界"), 0644)
	m.search.query.SetValue("context")
	drain(t, m, m.openSearch(false))
	if len(m.searchRows()) != 3 || m.search.result.Count != 3 {
		t.Fatal("same-line matches were not grouped without losing counts")
	}
	for _, colors := range []theme.Palette{theme.Dark(), theme.Light()} {
		m.colors = colors
		for _, nerd := range []bool{true, false} {
			m.nerd = nerd
			for _, w := range []int{24, 31, 48, 80} {
				rows := m.searchRows()
				header := m.searchResultRow(0, w, rows[0])
				content := m.searchResultRow(1, w, rows[1])
				if ansi.StringWidth(header) != w || ansi.StringWidth(content) != w {
					t.Fatal("result exceeded sidebar width")
				}
				if (w >= 31 && !strings.Contains(plain(header), "connection.go")) || !strings.Contains(plain(header), "3") {
					t.Fatalf("missing filename or count: %q", plain(header))
				}
				if w >= 48 && !strings.Contains(plain(header), "app/apps/cli") {
					t.Fatalf("missing parent path: %q", plain(header))
				}
				if !strings.Contains(plain(content), "context") || strings.Contains(plain(content), "1:") || !strings.Contains(plain(content), "…") {
					t.Fatalf("context was clipped or line numbers retained: %q", plain(content))
				}
				cells := uv.NewStyledString(content).Lines(ansi.GraphemeWidth)[0]
				highlighted := 0
				for _, cell := range cells {
					if sameColor(cell.Style.Bg, lipgloss.Color(colors.SearchMatch)) {
						highlighted++
					}
				}
				if highlighted < 7 {
					t.Fatal("match highlight was clipped or lost")
				}
			}
		}
	}
}
func TestSearchDismissKeepsFileAndExcludesReplacementTargets(t *testing.T) {
	for _, single := range []bool{false, true} {
		t.Run(map[bool]string{false: "all", true: "single"}[single], func(t *testing.T) {
			m := searchWorkspace(t)
			defer m.Close()
			path := filepath.Join(m.repo.Root, "file0.go")
			original := "old first\nold removed\nold last\n"
			os.WriteFile(path, []byte(original), 0644)
			m.search.query.SetValue("old")
			drain(t, m, m.openSearch(true))
			m.search.focus = 2
			m.search.selected = 2
			m.width = 120
			m.sidebarWidth = 48
			m.resize()
			m.hoverX = m.sideX() + m.sideWidth() - 1
			m.hoverY = m.searchResultY() + 2
			m.clickSearch(m.sideWidth()-1, m.searchResultY()+2)
			if m.search.result.Count != 4 {
				t.Fatal("dismiss did not update count")
			}
			data, _ := os.ReadFile(path)
			if string(data) != original || len(m.tabs) != 0 {
				t.Fatal("dismiss modified the file")
			}
			m.search.replacement.SetValue("new")
			// The selected row now points at the last line, despite the removed middle match.
			drain(t, m, m.replaceSearch(!single))
			data, _ = os.ReadFile(path)
			if string(data) != original {
				t.Fatal("replace unexpectedly saved")
			}
			var text string
			for _, tab := range m.tabs {
				if tab.doc.Path == path {
					text = tab.doc.Text
				}
			}
			expected := "new first\nold removed\nnew last\n"
			if single {
				expected = "old first\nold removed\nnew last\n"
			}
			if text != expected {
				t.Fatalf("dismiss changed replacement mapping: %q", text)
			}
		})
	}
}
func TestSearchHoverRefreshesResultAndDismissButton(t *testing.T) {
	m := searchWorkspace(t)
	defer m.Close()
	m.search.query.SetValue("old")
	drain(t, m, m.openSearch(false))
	m.View()
	m.Update(tea.MouseMotionMsg{X: m.sideX() + 6, Y: m.searchResultY() + 1})
	a := m.View().Content
	m.Update(tea.MouseMotionMsg{X: m.sideX() + 6, Y: m.searchResultY() + 3})
	b := m.View().Content
	if a == b {
		t.Fatal("hover reused stale search result rendering")
	}
	if m.search.selected != 0 {
		t.Fatal("hover changed result selection")
	}
	m.search.focus = 2
	m.search.selected = 1
	m.searchKey(tea.KeyPressMsg{Code: tea.KeyDelete})
	if len(m.search.result.Files) != 1 || m.search.result.Count != 2 {
		t.Fatal("keyboard dismiss did not remove all matches on the displayed line")
	}
}

func TestSearchFilesGroupByParentBeforeDescendants(t *testing.T) {
	m := searchWorkspace(t)
	defer m.Close()
	os.MkdirAll(filepath.Join(m.repo.Root, "app", "src"), 0755)
	for _, name := range []string{"app/AGENTS.md", "app/bun.lock", "app/src/a.go"} {
		os.WriteFile(filepath.Join(m.repo.Root, name), []byte("context"), 0644)
	}
	m.search.query.SetValue("context")
	drain(t, m, m.openSearch(false))
	files := m.search.result.Files
	if len(files) != 3 || filepath.Base(files[0].Path) != "AGENTS.md" || filepath.Base(files[1].Path) != "bun.lock" || filepath.Base(files[2].Path) != "a.go" {
		t.Fatal("parent files were split by descendant matches")
	}
}

func TestSearchDismissButtonNeverReflowsTextOrBadges(t *testing.T) {
	m := searchWorkspace(t)
	defer m.Close()
	os.MkdirAll(filepath.Join(m.repo.Root, "long", "parent"), 0755)
	os.WriteFile(filepath.Join(m.repo.Root, "long", "parent", "a_long_filename.go"), []byte(strings.Repeat("prefix ", 8)+"context.WithTimeout(context.Background()) and more text"), 0644)
	m.search.query.SetValue("context")
	drain(t, m, m.openSearch(false))
	rows := m.searchRows()
	for _, w := range []int{24, 31, 48, 80} {
		for _, nerd := range []bool{false, true} {
			m.nerd = nerd
			for index, r := range rows {
				m.hoverX, m.hoverY = -1, -1
				m.search.focus = 0
				before := m.searchResultRow(index, w, r)
				if r.match < 0 {
					badge := strings.TrimSpace(plain(ansi.Cut(before, w-5, w-2)))
					if badge != strconv.Itoa(len(m.search.result.Files[r.file].Matches)) {
						t.Fatal("count badge left an oversized trailing gap")
					}
				} else if plain(ansi.Cut(before, w-3, w-2)) != "…" {
					t.Fatal("long snippet did not fill the available row")
				}
				m.hoverX = m.sideX() + 5
				m.hoverY = m.searchResultY() + index
				hovered := m.searchResultRow(index, w, r)
				m.hoverX, m.hoverY = -1, -1
				m.search.focus = 2
				m.search.selected = index
				selected := m.searchResultRow(index, w, r)
				content := plain(ansi.Cut(before, 0, w-1))
				for _, after := range []string{hovered, selected} {
					if plain(ansi.Cut(after, 0, w-1)) != content {
						t.Fatalf("icons=%v width=%d row=%d: button reflowed text or badge", nerd, w, index)
					}
					gap := uv.NewStyledString(ansi.Cut(after, w-2, w-1)).Lines(ansi.GraphemeWidth)[0]
					if len(gap) != 1 || gap[0].Content != " " || sameColor(gap[0].Style.Bg, lipgloss.Color(m.colors.DiffPlaceholder)) {
						t.Fatal("file and match rows must share the same gap before close")
					}

					if ansi.StringWidth(after) != w || plain(ansi.Cut(after, w-1, w)) != m.symbol(symbolClose) {
						t.Fatal("close button moved outside its fixed slot")
					}
				}
			}
		}
	}
}
