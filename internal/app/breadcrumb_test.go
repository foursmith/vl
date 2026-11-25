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
	"github.com/foursmith/vl/internal/fileicon"
)

func TestEditorAndDiffShareFileIconAndBreadcrumbs(t *testing.T) {
	m := tabWorkspace(t, 0)
	m.nerd = true
	root, err := filepath.EvalSymlinks(m.repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	m.repo.Root = root
	if err = os.Mkdir(filepath.Join(root, "app"), 0755); err != nil {
		t.Fatal(err)
	}
	path := "app/eslint.config.mjs"
	if err = os.WriteFile(filepath.Join(root, path), []byte("export default []\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m.openPreviewFile(path)
	text := plain(m.contentToolbar())
	if !strings.Contains(text, "app "+m.symbol(symbolRight)) || !strings.Contains(text, "eslint.config.mjs") || strings.Contains(text, "javascript") {
		t.Fatalf("breadcrumb: %s", text)
	}
	icon := fileicon.For(path, true)
	for _, rendered := range []string{m.tabBar(), m.contentToolbar()} {
		found := false
		for _, cell := range uv.NewStyledString(rendered).Lines(ansi.GraphemeWidth)[0] {
			if cell.Content == icon.Glyph {
				found = true
				if !sameColor(cell.Style.Fg, lipgloss.Color(icon.Color)) {
					t.Fatal("file icon color differs between tab and breadcrumb")
				}
			}
		}
		if !found {
			t.Fatal("file icon missing")
		}
	}
	showDiffFixture(m)
	m.diffPath = path
	if !strings.Contains(m.tabBar(), icon.Glyph) || strings.Contains(m.tabBar(), "⇄") {
		t.Fatal("diff tab does not use the file's icon")
	}
	if !strings.Contains(plain(m.contentToolbar()), "eslint.config.mjs") {
		t.Fatal("diff lacks file breadcrumb")
	}
}

func TestBreadcrumbTruncatesAncestorsBeforeFile(t *testing.T) {
	m := tabWorkspace(t, 0)
	path := "very/long/ancestor/with/extra/names/component.jsx"
	for _, width := range []int{1, 8, 20, 35, 80} {
		if got := ansi.StringWidth(m.breadcrumb(path, width)); got != width {
			t.Fatalf("width %d: got %d", width, got)
		}
	}
	text := plain(m.breadcrumb(path, 35))
	if !strings.Contains(text, "component.jsx") {
		t.Fatalf("file lost before ancestors: %s", text)
	}
}

func TestHomeConfigurationBreadcrumb(t *testing.T) {
	m := tabWorkspace(t, 0)
	defer m.Close()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	realHome, err := filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(realHome, ".config", "vl", "vl.config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	m.openFile(path)
	prefix := strings.Join([]string{"~", ".config", "vl", ""}, " "+m.symbol(symbolRight)+" ")
	if rendered := plain(m.contentToolbar()); !strings.HasPrefix(rendered, " "+prefix) ||
		!strings.Contains(rendered, "vl.config.json") {
		t.Fatalf("configuration breadcrumb = %q", rendered)
	}
	for _, x := range []int{1, 1 + ansi.StringWidth(prefix) + 2} {
		m.Update(tea.MouseClickMsg{X: m.mainX() + x, Y: m.contentRow() - 1, Button: tea.MouseLeft})
		if m.modal != "" || m.document() == nil || m.document().Path != path {
			t.Fatalf("external breadcrumb click at column %d changed the current view", x)
		}
	}
}

func TestBreadcrumbOpensQuickPickerAtParentDirectory(t *testing.T) {
	m := tabWorkspace(t, 0)
	defer m.Close()
	root, err := filepath.EvalSymlinks(m.repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	m.repo.Root = root
	path := "内部/app/main.go"
	other := "内部/app/other.go"
	if err := os.MkdirAll(filepath.Join(root, filepath.Dir(path)), 0755); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{path, other} {
		if err := os.WriteFile(filepath.Join(root, file), []byte("package main\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	m.snapshot.Files = []string{path, other, "内部/util.go", "nested/" + path}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	m.openPreviewFile(path)
	// The parent directory begins after the full display width of 内部.
	appX := 1 + ansi.StringWidth("内部 "+m.symbol(symbolRight)+" ")
	click := func(x int) {
		m.Update(tea.MouseClickMsg{X: m.mainX() + x, Y: m.contentRow() - 1, Button: tea.MouseLeft})
	}
	click(2)
	if rows := m.filteredActions(); m.modal != "quick" || m.input.Value() != "" || len(rows) != 4 {
		t.Fatalf("ancestor click: query %q, rows %v", m.input.Value(), rows)
	}
	press(m, "esc")
	click(appX)
	if rows := m.filteredActions(); m.input.Value() != "内部/" || len(rows) != 3 {
		t.Fatalf("parent click: query %q, rows %v", m.input.Value(), rows)
	}
	press(m, "esc")
	click(appX + ansi.StringWidth("app "+m.symbol(symbolRight)+" ") + 2)
	if rows := m.filteredActions(); m.modal != "quick" || m.input.Value() != "内部/app/" || len(rows) != 2 {
		t.Fatalf("filename click: query %q, rows %v", m.input.Value(), rows)
	}
	drain(t, m, press(m, "down"))
	drain(t, m, press(m, "enter"))
	if d := m.document(); d == nil || !strings.HasSuffix(d.Path, other) {
		t.Fatal("picker did not open the selected sibling file")
	}
}
