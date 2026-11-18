package app

import (
	"context"
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/vl/internal/editor"
	gitservice "github.com/ourongxing/vl/internal/git"
	"github.com/ourongxing/vl/internal/highlight"
	"github.com/ourongxing/vl/internal/theme"
)

func sameColor(a, b color.Color) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}

func TestTerminalPaddingFollowsThemeChrome(t *testing.T) {
	m := tabWorkspace(t, 1)
	for _, id := range []string{"dark_modern", "light_modern", "dark_vs"} {
		m.setTestTheme(id)
		m.width, m.height = 120, 36
		for _, open := range []bool{false, true} {
			if open {
				m.openFile("file0.go")
			}
			if !sameColor(m.View().BackgroundColor, lipgloss.Color(m.colors.Panel)) {
				t.Fatalf("%s, open file %v: terminal padding differs from chrome", id, open)
			}
		}
		m.width, m.height = 20, 5
		if !sameColor(m.View().BackgroundColor, lipgloss.Color(m.colors.Panel)) {
			t.Fatalf("%s: small terminal padding differs from chrome", id)
		}
	}
}

func TestRowPaintsBackgroundAfterNestedStyleResets(t *testing.T) {
	text := theme.Style("#CE9178", theme.Builtin("dark_modern").Palette().Background).Render("word") + " gap " + theme.Style(theme.Builtin("dark_modern").Palette().Muted, theme.Builtin("dark_modern").Palette().Background).Render("end")
	line := uv.NewStyledString(row(text, 30, theme.Builtin("dark_modern").Palette().Foreground, theme.Builtin("dark_modern").Palette().Background)).Lines(ansi.GraphemeWidth)[0]
	for i, cell := range line {
		if !sameColor(cell.Style.Bg, lipgloss.Color(theme.Builtin("dark_modern").Palette().Background)) {
			t.Fatalf("cell %d (%q) has background %v, want %s", i, cell.Content, cell.Style.Bg, theme.Builtin("dark_modern").Palette().Background)
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
			want := theme.Builtin("dark_modern").Palette().Background
			if y == 0 && x >= 7 {
				want = theme.Builtin("dark_modern").Palette().CurrentLine
			}
			if !sameColor(cell.Style.Bg, lipgloss.Color(want)) {
				t.Fatalf("row %d cell %d (%q) has background %v, want %s", y, x, cell.Content, cell.Style.Bg, want)
			}
		}
	}
}

func TestEditorCurrentLineBorderUsesAdjacentRows(t *testing.T) {
	m := New(context.Background(), &gitservice.Repository{Root: "/tmp/repo"}, false)
	m.colors = theme.Builtin("hc_black").Palette()
	doc := &editor.Document{Path: "/tmp/repo/test.txt", Text: "above\n当前\nbelow", Anchor: -1}
	m.tabs = []tab{{doc: doc}}
	m.activeTab = 0
	m.ensureEditor()
	border := lipgloss.Color(theme.Composite(m.colors.CurrentLineBorder, m.colors.Background))
	for _, cursorRow := range []int{1, 1, 0} {
		doc.SetPosition(cursorRow, 0)
		for y, rendered := range m.editorLines() {
			if got := ansi.StringWidth(rendered); got != m.mainWidth() {
				t.Fatalf("row %d has width %d, want %d", y, got, m.mainWidth())
			}
			for x, cell := range uv.NewStyledString(rendered).Lines(ansi.GraphemeWidth)[0] {
				want := x >= 7 && (y == cursorRow || y == cursorRow-1)
				if got := cell.Style.Underline == uv.UnderlineSingle && sameColor(cell.Style.UnderlineColor, border); got != want {
					t.Fatalf("cursor row %d, rendered row %d, cell %d (%q) has border=%t, want %t", cursorRow, y, x, cell.Content, got, want)
				}
			}
		}
	}
}

func TestEditorCurrentLineHighlightYieldsToSelection(t *testing.T) {
	for _, id := range []string{"hc_black", "dark_2026"} {
		t.Run(id, func(t *testing.T) {
			m := New(context.Background(), &gitservice.Repository{Root: "/tmp/repo"}, false)
			m.colors = theme.Builtin(id).Palette()
			doc := &editor.Document{Path: "/tmp/repo/test.txt", Text: "above\ncurrent\nbelow", Anchor: -1}
			m.tabs = []tab{{doc: doc}}
			m.activeTab = 0
			m.ensureEditor()
			doc.SetPosition(1, 0)
			before := m.editorLines()

			doc.SelectVisual(doc.Cursor, doc.Cursor, false)
			during := m.editorLines()
			for y, rendered := range during[:2] {
				cells := uv.NewStyledString(rendered).Lines(ansi.GraphemeWidth)[0]
				for x, cell := range cells {
					want := m.colors.Background
					if y == 1 && x == 7 {
						want = theme.Composite(m.colors.EditorSelection, want)
					}
					if cell.Style.Underline != uv.UnderlineNone || !sameColor(cell.Style.Bg, lipgloss.Color(want)) {
						t.Fatalf("selected row %d cell %d has unexpected style %+v", y, x, cell.Style)
					}
				}
			}

			doc.ClearSelection()
			after := m.editorLines()
			if after[0] != before[0] || after[1] != before[1] {
				t.Fatal("clearing the selection did not restore the current-line highlight")
			}
		})
	}
}

func TestSelectedFilePaintsIconTextAndPadding(t *testing.T) {
	m := New(context.Background(), &gitservice.Repository{Root: "/tmp/repo"}, true)
	m.section = explorer
	m.items = []item{{file: "test.go", label: "test.go"}}
	m.selected = 0
	m.sidebar = true
	line := uv.NewStyledString(m.sidebarLines()[headerHeight]).Lines(ansi.GraphemeWidth)[0]
	for i, cell := range line {
		if !sameColor(cell.Style.Bg, lipgloss.Color(theme.Builtin("dark_modern").Palette().Selection)) {
			t.Fatalf("selected cell %d (%q) has background %v", i, cell.Content, cell.Style.Bg)
		}
	}
}

func TestRowPreservesExplicitHighlightBackgrounds(t *testing.T) {
	text := theme.Style(theme.Builtin("dark_modern").Palette().Foreground, theme.Builtin("dark_modern").Palette().AddedBackground).Render("+added") + " normal " + theme.Style(theme.Builtin("dark_modern").Palette().Foreground, theme.Builtin("dark_modern").Palette().Selection).Render("selected")
	line := uv.NewStyledString(row(text, 30, theme.Builtin("dark_modern").Palette().Foreground, theme.Builtin("dark_modern").Palette().Background)).Lines(ansi.GraphemeWidth)[0]
	for i, cell := range line {
		want := theme.Builtin("dark_modern").Palette().Background
		if i < 6 {
			want = theme.Builtin("dark_modern").Palette().AddedBackground
		}
		if i >= 14 && i < 22 {
			want = theme.Builtin("dark_modern").Palette().Selection
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
	m.tabs[0].spans = [][]highlight.Span{{{Start: 0, End: 2, Color: theme.Builtin("dark_modern").Palette().GitAdded, Font: theme.Italic}}}
	first := uv.NewStyledString(m.editorLines()[0]).Lines(ansi.GraphemeWidth)[0]
	selected := false
	for _, cell := range first {
		if cell.Content == "a" {
			selected = true
			p := theme.Builtin("dark_modern").Palette()
			if !sameColor(cell.Style.Fg, lipgloss.Color(theme.Readable(p.GitAdded, p.EditorSelection))) || !sameColor(cell.Style.Bg, lipgloss.Color(p.EditorSelection)) {
				t.Fatal("run rendering lost syntax or selection color")
			}
			if cell.Style.Attrs&uv.AttrItalic == 0 {
				t.Fatal("selected syntax lost its TextMate font style")
			}
		}
	}
	if !selected || m.editorCursor() == nil {
		t.Fatal("selected text or cursor missing")
	}
	m.sidebar = true
	if m.editorCursor() != nil {
		t.Fatal("editor cursor remained after focus moved to sidebar")
	}
	d.Anchor = -1
	m.editorLines()
	third := uv.NewStyledString(m.editorLines()[0]).Lines(ansi.GraphemeWidth)[0]
	for _, cell := range third {
		if cell.Content == "a" && sameColor(cell.Style.Bg, lipgloss.Color(theme.Builtin("dark_modern").Palette().EditorSelection)) {
			t.Fatal("cache kept a cleared selection")
		}
	}
}
