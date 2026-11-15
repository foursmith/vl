package app

import (
	"context"
	"fmt"
	"image/color"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/elegit/internal/editor"
	gitservice "github.com/ourongxing/elegit/internal/git"
	"github.com/ourongxing/elegit/internal/highlight"
	"github.com/ourongxing/elegit/internal/theme"
)

func sameColor(a, b color.Color) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}

func TestTerminalBackgroundMatchesChromeWithAndWithoutFile(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.SetThemeMode("dark")
	for _, open := range []bool{false, true} {
		if open {
			m.openFile("file0.go")
		}
		if !sameColor(m.View().BackgroundColor, lipgloss.Color(theme.Dark().Panel)) {
			t.Fatalf("open file %v: uncovered terminal background differs from chrome", open)
		}
	}
	m.width, m.height = 20, 5
	if !sameColor(m.View().BackgroundColor, lipgloss.Color(theme.Dark().Panel)) {
		t.Fatal("small terminal background differs from chrome")
	}
}

func TestRowPaintsBackgroundAfterNestedStyleResets(t *testing.T) {
	text := theme.Style("#CE9178", theme.Dark().Background).Render("word") + " gap " + theme.Style(theme.Dark().Muted, theme.Dark().Background).Render("end")
	line := uv.NewStyledString(row(text, 30, theme.Dark().Foreground, theme.Dark().Background)).Lines(ansi.GraphemeWidth)[0]
	for i, cell := range line {
		if !sameColor(cell.Style.Bg, lipgloss.Color(theme.Dark().Background)) {
			t.Fatalf("cell %d (%q) has background %v, want %s", i, cell.Content, cell.Style.Bg, theme.Dark().Background)
		}
	}
}

func TestEditorCanvasAndCurrentLineBackgrounds(t *testing.T) {
	m := New(context.Background(), &gitservice.Repository{Root: "/tmp/repo"}, false)
	m.tabs = []tab{{doc: &editor.Document{Path: "/tmp/repo/test.go", Text: "package main\n\n// comment\n", Anchor: -1}}}
	m.activeTab = 0
	m.sidebar = true
	m.ensureEditor()
	for y, rendered := range m.mainLines()[editorContentRow:] {
		line := uv.NewStyledString(rendered).Lines(ansi.GraphemeWidth)[0]
		for x, cell := range line {
			want := theme.Dark().Background
			if y == 0 && x >= 7 {
				want = "#252526"
			}
			if !sameColor(cell.Style.Bg, lipgloss.Color(want)) {
				t.Fatalf("row %d cell %d (%q) has background %v, want %s", y, x, cell.Content, cell.Style.Bg, want)
			}
		}
	}
}

func TestSelectedFilePaintsIconTextAndPadding(t *testing.T) {
	m := New(context.Background(), &gitservice.Repository{Root: "/tmp/repo"}, true)
	m.items = []item{{file: "test.go", label: "test.go"}}
	m.selected = 0
	m.sidebar = true
	line := uv.NewStyledString(m.sidebarLines()[sidebarHeaderRows]).Lines(ansi.GraphemeWidth)[0]
	for i, cell := range line {
		if !sameColor(cell.Style.Bg, lipgloss.Color(theme.Dark().Selection)) {
			t.Fatalf("selected cell %d (%q) has background %v", i, cell.Content, cell.Style.Bg)
		}
	}
}

func TestRowPreservesExplicitHighlightBackgrounds(t *testing.T) {
	text := theme.Style(theme.Dark().Foreground, theme.Dark().AddedBackground).Render("+added") + " normal " + theme.Style(theme.Dark().Foreground, theme.Dark().Selection).Render("selected")
	line := uv.NewStyledString(row(text, 30, theme.Dark().Foreground, theme.Dark().Background)).Lines(ansi.GraphemeWidth)[0]
	for i, cell := range line {
		want := theme.Dark().Background
		if i < 6 {
			want = theme.Dark().AddedBackground
		}
		if i >= 14 && i < 22 {
			want = theme.Dark().Selection
		}
		if !sameColor(cell.Style.Bg, lipgloss.Color(want)) {
			t.Fatalf("cell %d (%q) background %v, want %s", i, cell.Content, cell.Style.Bg, want)
		}
	}
}

func TestEditorRunsKeepSyntaxSelectionCursorAndFocus(t *testing.T) {
	m := New(context.Background(), &gitservice.Repository{Root: "/tmp/repo"}, false)
	d := &editor.Document{Path: "/tmp/repo/test.go", Text: "ab\t中\nsecond", Cursor: 2, Anchor: 0}
	m.tabs = []tab{{doc: d}}
	m.activeTab = 0
	m.sidebar = false
	m.ensureEditor()
	m.tabs[0].highlightText = d.Text
	m.tabs[0].spans = [][]highlight.Span{{{Start: 0, End: 2, Color: theme.Dark().GitAdded, Font: highlight.Italic}}}
	first := uv.NewStyledString(m.editorLines()[0]).Lines(ansi.GraphemeWidth)[0]
	selected, cursor := false, false
	for _, cell := range first {
		if cell.Content == "a" {
			selected = true
			if !sameColor(cell.Style.Fg, lipgloss.Color(theme.Dark().GitAdded)) || !sameColor(cell.Style.Bg, lipgloss.Color(theme.Dark().Selection)) {
				t.Fatal("run rendering lost syntax or selection color")
			}
			if cell.Style.Attrs&uv.AttrItalic == 0 {
				t.Fatal("selected syntax lost its TextMate font style")
			}
		}
		cursor = cursor || cell.Style.Attrs&uv.AttrReverse != 0
	}
	if !selected || !cursor {
		t.Fatal("selected text or cursor missing")
	}
	m.sidebar = true
	second := uv.NewStyledString(m.editorLines()[0]).Lines(ansi.GraphemeWidth)[0]
	for _, cell := range second {
		if cell.Style.Attrs&uv.AttrReverse != 0 {
			t.Fatal("cached editor cursor remained after focus moved to sidebar")
		}
	}
	d.Anchor = -1
	m.editorLines()
	third := uv.NewStyledString(m.editorLines()[0]).Lines(ansi.GraphemeWidth)[0]
	for _, cell := range third {
		if cell.Content == "a" && sameColor(cell.Style.Bg, lipgloss.Color(theme.Dark().Selection)) {
			t.Fatal("cache kept a cleared selection")
		}
	}
}

func TestLargeOutputKeepsOnlyRecentLines(t *testing.T) {
	m := New(context.Background(), &gitservice.Repository{Root: "/tmp/repo"}, false)
	var text strings.Builder
	for i := 0; i < 1000; i++ {
		fmt.Fprintf(&text, "line %d\n", i)
	}
	m.addOutput(text.String())
	if len(m.output) != 200 || m.output[0] != "line 800" || m.output[199] != "line 999" {
		t.Fatal("output retained the wrong tail")
	}
	m.log("latest")
	if len(m.output) != 200 || !strings.Contains(m.output[199], "latest") {
		t.Fatal("output grew beyond its display history limit")
	}
}
