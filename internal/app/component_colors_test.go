package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/vl/internal/highlight"
	"github.com/ourongxing/vl/internal/theme"
)

func TestComponentsDoNotBorrowEditorOrPickerColors(t *testing.T) {
	m := tabWorkspace(t, 1)
	filename := filepath.Join(t.TempDir(), "roles.json")
	raw := `{"colors":{"editor.foreground":"#111111","quickInput.foreground":"#222222","quickInput.background":"#101010","menu.foreground":"#334455","menu.background":"#111213","menu.selectionForeground":"#556677","menu.selectionBackground":"#141516","statusBar.foreground":"#778899","statusBar.background":"#171819","statusBar.noFolderBackground":"#FF0000","tab.activeForeground":"#99AABB","strongForeground":"#FFFFFF"}}`
	if err := os.WriteFile(filename, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	th, err := theme.Load(filename, "")
	if err != nil {
		t.Fatal(err)
	}
	m.SetColorTheme(th)
	m.openFile("file0.go")
	m.sidebar = true // keyboard focus does not change the single active editor group
	found := false
	for _, cell := range uv.NewStyledString(m.tabBar()).Lines(ansi.GraphemeWidth)[0] {
		if cell.Content == "f" {
			found = true
			if !sameColor(cell.Style.Fg, lipgloss.Color("#99AABB")) {
				t.Fatal("active tab borrowed strong/editor text")
			}
		}
	}
	if !found {
		t.Fatal("active tab label missing")
	}
	for _, noGit := range []bool{false, true} {
		m.snapshot.NoRepository = noGit
		for _, cell := range uv.NewStyledString(m.statusLine()).Lines(ansi.GraphemeWidth)[0] {
			if !sameColor(cell.Style.Bg, lipgloss.Color("#171819")) {
				t.Fatal("status bar borrowed sidebar/no-folder background")
			}
		}
	}
	m.contextMenu = &contextMenu{x: 2, y: 2, actions: []action{{label: "a selected"}, {label: "z normal"}}}
	base := make([]string, m.height)
	for i := range base {
		base[i] = strings.Repeat(" ", m.width)
	}
	l := m.contextMenuLayout()
	painted := m.contextMenuOverlay(base)
	for i, want := range []string{"#556677", "#334455"} {
		cells := uv.NewStyledString(painted[l.y+i+1]).Lines(ansi.GraphemeWidth)[0]
		if !sameColor(cells[l.x+2].Style.Fg, lipgloss.Color(want)) {
			t.Fatal("context menu borrowed quick input/editor foreground")
		}
	}
}

func TestTokenAlphaUsesActualDiffAndSelectionSurfaces(t *testing.T) {
	colors := theme.Builtin("dark_modern").Palette()
	colors.Background = "#000000"
	spans := []highlight.Span{{Start: 0, End: 1, Color: "#FFFFFF80", Background: "#00FF0080"}}
	rendered := diffCode(colors, "x", spans, 1, 0, colors.Background, "", diffRange{}, diffRange{})
	cell := uv.NewStyledString(rendered).Lines(ansi.GraphemeWidth)[0][0]
	if !sameColor(cell.Style.Bg, lipgloss.Color("#008000")) || !sameColor(cell.Style.Fg, lipgloss.Color("#80C080")) {
		t.Fatal("token alpha was composed before token/line background")
	}
	colors.EditorSelection = "#FF000080"
	rendered = diffCode(colors, "x", spans, 1, 0, colors.Background, "", diffRange{}, diffRange{0, 1})
	cell = uv.NewStyledString(rendered).Lines(ansi.GraphemeWidth)[0][0]
	if !sameColor(cell.Style.Bg, lipgloss.Color("#804000")) || !sameColor(cell.Style.Fg, lipgloss.Color("#C0A080")) {
		t.Fatal("selection/token foreground ignored actual compositing order")
	}
}
