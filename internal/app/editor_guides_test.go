package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

func TestEditorBracketColorsStayVisibleWhileHighlightRefreshes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.go")
	source := "package main\nfunc f() {\n  call(\"字\", n)\n}\n"
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := NewEditor(context.Background(), path, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	drain(t, m, m.scheduleHighlight())

	check := func(stage string, closingRow int) {
		t.Helper()
		rows := m.editorLines()
		for _, endpoint := range []struct {
			row   int
			glyph byte
		}{{2, '('}, {closingRow, ')'}} {
			row := rows[endpoint.row]
			plain := ansi.Strip(row)
			index := strings.IndexByte(plain, endpoint.glyph)
			if index < 0 {
				t.Fatalf("%s: bracket %q missing from %q", stage, endpoint.glyph, plain)
			}
			column := ansi.StringWidth(plain[:index])
			cell := uv.NewStyledString(ansi.Cut(row, column, column+1)).Lines(ansi.GraphemeWidth)[0][0]
			if cell.Content != string(endpoint.glyph) || !sameColor(cell.Style.Fg, lipgloss.Color(m.colors.BracketColor(1))) {
				t.Fatalf("%s: bracket %q lost its nesting color at column %d: %+v", stage, endpoint.glyph, column, cell)
			}
		}
	}
	check("initial render", 2)
	d := m.document()
	d.SetPosition(2, 13)
	d.Insert("界\n  ")
	check("after edit", 3)
	request := m.scheduleHighlight()().(highlightStartMsg).request
	brackets := m.finishHighlight(m.startHighlight(request)().(highlightResultMsg))
	check("after viewport highlight", 3)
	drain(t, m, brackets)
	check("after bracket refresh", 3)
}

func TestEditorGuidesFollowActiveBracketAndIndentation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "page.tsx")
	source := "function Page() {\n  return (\n    <main>\n      {\")]}\"}\n\n    </main>\n  );\n}\n"
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := NewEditor(context.Background(), path, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	drain(t, m, m.scheduleHighlight())
	d := m.document()
	d.SetPosition(1, 9)
	rows := m.editorLines()
	cell := func(y, x int) uv.Cell {
		return uv.NewStyledString(rows[y]).Lines(ansi.GraphemeWidth)[0][x]
	}
	color := lipgloss.Color(m.colors.BracketGuide(1))
	for _, y := range []int{2, 3, 4, 5} {
		if c := cell(y, 7); c.Content != "│" || sameColor(c.Style.Fg, color) {
			t.Fatalf("row %d missing ordinary indentation guide: %+v", y, c)
		}
		if c := cell(y, 9); c.Content != "│" || !sameColor(c.Style.Fg, color) {
			t.Fatalf("row %d missing active bracket guide: %+v", y, c)
		}
	}
	for _, y := range []int{1, 6} {
		for _, c := range uv.NewStyledString(rows[y]).Lines(ansi.GraphemeWidth)[0] {
			if sameColor(c.Style.UnderlineColor, color) || c.Content == "│" && sameColor(c.Style.Fg, color) {
				t.Fatalf("unexpected bracket guide on endpoint row %d: %+v", y, c)
			}
		}
	}
	for _, x := range []int{20, 21, 23} {
		c := cell(0, x)
		if !sameColor(c.Style.Fg, lipgloss.Color(m.colors.BracketColor(0))) {
			t.Fatalf("inactive bracket lost its nesting color: %+v", c)
		}
	}
	var endpoints []uv.Cell
	for _, p := range [][2]int{{1, 16}, {6, 9}} {
		c := cell(p[0], p[1])
		if !sameColor(c.Style.Fg, lipgloss.Color(m.colors.BracketColor(1))) {
			t.Fatalf("incorrect nested bracket color at %v: %+v", p, c)
		}
		endpoints = append(endpoints, c)
	}
	d.SetPosition(8, 0)
	rows = m.editorLines()
	for i, p := range [][2]int{{1, 16}, {6, 9}} {
		c := cell(p[0], p[1])
		if !sameColor(c.Style.Fg, endpoints[i].Style.Fg) || !sameColor(c.Style.Bg, endpoints[i].Style.Bg) {
			t.Fatalf("active guide changed bracket colors at %v: %+v", p, endpoints[i])
		}
	}
	if c := cell(2, 9); c.Content != "│" || sameColor(c.Style.Fg, color) {
		t.Fatalf("inactive bracket guide remained on the indentation line: %+v", c)
	}
}
