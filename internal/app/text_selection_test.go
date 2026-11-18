package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/vl/internal/diff"
)

func TestEditorMouseSelectionAndRelease(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	m.openFile("file0.go")
	d := m.document()
	d.Edit("A👨‍👩‍👧‍👦e\u0301\tZ\nsecond")
	d.SetPosition(0, 0)
	x := m.mainX() + 7
	m.Update(tea.MouseClickMsg{X: x + 1, Y: editorContentRow, Button: tea.MouseLeft})
	before := m.View().Content
	m.Update(tea.MouseMotionMsg{X: x + 4, Y: editorContentRow, Button: tea.MouseLeft})
	if d.SelectedText() != "👨‍👩‍👧‍👦e\u0301" {
		t.Fatalf("selection split graphemes: %q", d.SelectedText())
	}
	if m.View().Content == before {
		t.Fatal("drag reused stale frame")
	}
	m.Update(tea.MouseReleaseMsg{X: x + 3, Y: editorContentRow + 1, Button: tea.MouseLeft})
	if d.SelectedText() != "👨‍👩‍👧‍👦e\u0301\tZ\nsec" {
		t.Fatalf("release did not extend selection: %q", d.SelectedText())
	}
	m.Update(tea.MouseMotionMsg{X: x + 6, Y: editorContentRow + 1})
	if d.SelectedText() != "👨‍👩‍👧‍👦e\u0301\tZ\nsec" || m.textDrag != nil {
		t.Fatal("selection continued after release")
	}
	m.Update(tea.MouseClickMsg{X: x + 5, Y: editorContentRow + 1, Button: tea.MouseLeft, Mod: tea.ModShift})
	if !strings.HasSuffix(d.SelectedText(), "\nsecon") {
		t.Fatalf("shift click lost anchor: %q", d.SelectedText())
	}
}

func TestDiffMouseSelectionCopyAndPainting(t *testing.T) {
	for _, split := range []bool{false, true} {
		t.Run(map[bool]string{false: "unified", true: "split"}[split], func(t *testing.T) {
			m := tabWorkspace(t, 0)
			defer m.Close()
			m.width = 180
			showDiffFixture(m)
			m.diffPath = "file.go"
			m.split = split
			m.patch = diff.Parse("@@ -1,2 +1,3 @@\n-old\n+new\n+extra\n same\n")
			m.renderDiff()
			first := -1
			for i, pair := range m.diffRows {
				line := pair.Left
				if split {
					line = pair.Right
				}
				if line != nil && line.Kind == diff.Added {
					first = i
					break
				}
			}
			x := m.mainX() + m.diffGutter
			if split {
				x += (m.mainWidth()-1)/2 + 1
			}
			m.Update(tea.MouseClickMsg{X: x + 1, Y: editorContentRow + first, Button: tea.MouseLeft})
			m.View()
			m.Update(tea.MouseMotionMsg{X: x + 3, Y: editorContentRow + first + 1, Button: tea.MouseLeft})
			if got := m.diffSelectedText(); got != "ew\next" {
				t.Fatalf("source text contains gutters/other side: %q", got)
			}
			if m.hoverOnly {
				t.Fatal("drag marked as passive hover")
			}
			found := false
			for _, rendered := range m.diffView() {
				for _, cell := range uv.NewStyledString(rendered).Lines(ansi.GraphemeWidth)[0] {
					if sameColor(cell.Style.Bg, lipgloss.Color(m.colors.EditorSelection)) {
						found = true
					}
				}
			}
			if !found {
				t.Fatal("diff selection has no highlight")
			}
			m.Update(tea.MouseReleaseMsg{X: x + 3, Y: editorContentRow + first + 1, Button: tea.MouseLeft})
			if cmd := m.key(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}); cmd == nil {
				t.Fatal("copy did not produce a clipboard command")
			}
			m.Update(tea.MouseClickMsg{X: x + 3, Y: editorContentRow + first + 1, Button: tea.MouseLeft})
			m.Update(tea.MouseMotionMsg{X: x + 1, Y: editorContentRow + first, Button: tea.MouseLeft})
			if m.diffSelectedText() != "ew\next" {
				t.Fatal("reverse selection changed text")
			}
			m.split = !split
			m.renderDiff()
			if m.diffSelectedText() != "" || m.diffDrag != nil {
				t.Fatal("layout change retained stale row coordinates")
			}
			if cmd := m.key(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}); cmd != nil {
				t.Fatal("Ctrl+C without selection must not quit a diff")
			}
		})
	}
}

func TestDiffSelectionSkipsMissingRowsAndStaysOnSide(t *testing.T) {
	m := tabWorkspace(t, 0)
	defer m.Close()
	m.width = 180
	showDiffFixture(m)
	m.diffPath = "file.go"
	m.patch = diff.Parse("@@ -1,2 +1,3 @@\n-old\n+new\n+extra\n same\n")
	m.renderDiff()
	first := 0
	for i, p := range m.diffRows {
		if p.Left != nil && p.Left.Kind == diff.Deleted {
			first = i
			break
		}
	}
	x := m.mainX() + m.diffGutter
	m.Update(tea.MouseClickMsg{X: x, Y: editorContentRow + first, Button: tea.MouseLeft})
	m.Update(tea.MouseMotionMsg{X: m.mainX() + m.mainWidth() - 1, Y: editorContentRow + first + 2, Button: tea.MouseLeft})
	if got := m.diffSelectedText(); got != "old\nsame" {
		t.Fatalf("cross-column drag switched sides or copied placeholder: %q", got)
	}
	m.Update(tea.BlurMsg{})
	if m.diffDrag != nil {
		t.Fatal("blur retained active drag")
	}
}

func TestTextSelectionUsesDisplayCellsAndScrollOffsets(t *testing.T) {
	for _, tc := range []struct {
		text      string
		cell, col int
	}{
		{"中文x", 1, 0}, {"中文x", 2, 1}, {"中文x", 4, 2},
		{"a\tb", 3, 1}, {"a\tb", 4, 2}, {"e\u0301x", 1, 2},
	} {
		if got := textColumn(tc.text, tc.cell, nil); got != tc.col {
			t.Fatalf("%q cell %d: rune %d, want %d", tc.text, tc.cell, got, tc.col)
		}
	}
	m := tabWorkspace(t, 1)
	defer m.Close()
	m.openFile("file0.go")
	d := m.document()
	d.Edit(strings.Repeat("中文abcdef\n", 50))
	m.tabs[m.activeTab].top = 10
	m.tabs[m.activeTab].left = 4
	x := m.mainX() + 7
	m.Update(tea.MouseClickMsg{X: x, Y: editorContentRow, Button: tea.MouseLeft})
	m.Update(tea.MouseReleaseMsg{X: x + 3, Y: editorContentRow, Button: tea.MouseLeft})
	if got := d.SelectedText(); got != "abc" {
		t.Fatalf("scrolled selection: %q", got)
	}
	row, _ := d.Position()
	if row != 10 {
		t.Fatalf("selected row %d, want 10", row)
	}
}
