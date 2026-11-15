package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/elegit/internal/fileicon"
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
	if !strings.Contains(text, "app "+m.symbol(symbolRight)) || !strings.Contains(text, "eslint.config.mjs") || !strings.Contains(text, "javascript") {
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

func TestBreadcrumbTruncatesAncestorsBeforeFileAndLanguage(t *testing.T) {
	m := tabWorkspace(t, 0)
	path := "very/long/ancestor/with/extra/names/component.jsx"
	for _, width := range []int{1, 8, 20, 35, 80} {
		if got := ansi.StringWidth(m.breadcrumb(path, width)); got != width {
			t.Fatalf("width %d: got %d", width, got)
		}
	}
	text := plain(m.breadcrumb(path, 35))
	if !strings.Contains(text, "component.jsx") || !strings.Contains(text, "jsx") {
		t.Fatalf("file or language lost before ancestors: %s", text)
	}
}
