package app

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	gitservice "github.com/foursmith/vl/internal/git"
	"github.com/foursmith/vl/internal/highlight"
	"github.com/foursmith/vl/internal/theme"
)

func TestComponentsDoNotBorrowEditorOrPickerColors(t *testing.T) {
	m := tabWorkspace(t, 1)
	raw := `{"colors":{"editor.foreground":"#111111","quickInput.foreground":"#222222","quickInput.background":"#101010","menu.foreground":"#7799BB","menu.background":"#111213","menu.selectionForeground":"#8596A7","menu.selectionBackground":"#141516","sideBar.foreground":"#222324","sideBar.background":"#171819","statusBar.foreground":"#CC0000","statusBar.background":"#FF0000","tab.activeForeground":"#99AABB","strongForeground":"#FFFFFF"}}`
	th, _ := fixtureTheme(t, "vs-dark", raw)
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
	assertStatusColors := func(id, fg, bg string) {
		t.Helper()
		segment := statusPart(t, m, id)
		cells := uv.NewStyledString(m.statusLine()).Lines(ansi.GraphemeWidth)[0]
		cell := cells[segment.x+1]
		if !sameColor(cell.Style.Fg, lipgloss.Color(fg)) || !sameColor(cell.Style.Bg, lipgloss.Color(bg)) {
			t.Fatalf("status item %s did not use %s on %s: %+v", id, fg, bg, cell.Style)
		}
	}
	sidebarFG := theme.Readable("#222324", "#171819")
	m.snapshot = gitservice.Snapshot{Branch: "main"}
	vim := &m.tabs[m.activeTab].vim
	for _, tc := range []struct {
		label, bg            string
		insert, visual, line bool
	}{
		{label: "NORMAL", bg: "#005F5F"},
		{label: "INSERT", bg: "#5F0000", insert: true},
		{label: "VISUAL", bg: "#5F00AF", visual: true},
		{label: "VISUAL LINE", bg: "#005F87", visual: true, line: true},
	} {
		vim.InsertMode, vim.VisualMode, vim.VisualLine = tc.insert, tc.visual, tc.line
		if mode := statusPart(t, m, "vim").text; mode != tc.label {
			t.Fatalf("expected %s status label, got %q", tc.label, mode)
		}
		assertStatusColors("vim", "#FFFFFF", tc.bg)
		assertStatusColors("branch", "#FFFFFF", tc.bg)
		assertStatusColors("changes", "#FFFFFF", tc.bg)
		m.statusBarStyle = statusBarPure
		assertStatusColors("vim", "#FFFFFF", tc.bg)
		assertStatusColors("branch", sidebarFG, "#171819")
		assertStatusColors("changes", sidebarFG, "#171819")
		m.statusBarStyle = statusBarStandard
	}
	vim.VisualMode, vim.VisualLine = false, false
	m.snapshot.Changes = []gitservice.Change{{Path: "file0.go", Index: 'U', Worktree: 'U'}}
	if got, want := statusPart(t, m, "changes").text, m.symbol(symbolWarning)+" 1"; got != want {
		t.Fatalf("conflict indicator = %q, want %q", got, want)
	}
	assertStatusColors("changes", "#FFFFFF", "#005F5F")
	m.statusBarStyle = statusBarPure
	assertStatusColors("changes", sidebarFG, "#171819")
	m.statusBarStyle = statusBarStandard
	m.snapshot.NoRepository = true
	assertStatusColors("init", "#FFFFFF", "#005F5F")
	m.editorOnly = true
	assertStatusColors("editor", "#FFFFFF", "#005F5F")
	assertStatusColors("vim", "#FFFFFF", "#005F5F")
	m.statusBarStyle = statusBarPure
	assertStatusColors("editor", sidebarFG, "#171819")
	assertStatusColors("vim", "#FFFFFF", "#005F5F")
	m.statusBarStyle = statusBarStandard
	activeTab := m.activeTab
	m.activeTab = -1
	assertStatusColors("editor", sidebarFG, "#171819")
	m.editorOnly = false
	assertStatusColors("init", sidebarFG, "#171819")
	m.activeTab = activeTab
	m.contextMenu = &contextMenu{x: 2, y: 2, actions: []action{{label: "a selected"}, {label: "z normal"}}}
	base := make([]string, m.height)
	for i := range base {
		base[i] = strings.Repeat(" ", m.width)
	}
	l := m.contextMenuLayout()
	painted := m.contextMenuOverlay(base)
	for i, want := range []string{"#8596A7", "#7799BB"} {
		cells := uv.NewStyledString(painted[l.y+i+1]).Lines(ansi.GraphemeWidth)[0]
		if !sameColor(cells[l.x+2].Style.Fg, lipgloss.Color(want)) {
			t.Fatal("context menu borrowed quick input/editor foreground")
		}
	}
}

func TestTokenAlphaUsesActualDiffAndSelectionSurfaces(t *testing.T) {
	colors := theme.Builtin("dark_modern").Palette()
	colors.Background = "#000000"
	spans := []highlight.Span{{Start: 0, End: 1, Color: "#FFFFFFC0", Background: "#00004080"}}
	rendered := diffCode(colors, "x", spans, 1, 0, colors.Background, "", diffRange{}, diffRange{})
	cell := uv.NewStyledString(rendered).Lines(ansi.GraphemeWidth)[0][0]
	if !sameColor(cell.Style.Bg, lipgloss.Color("#000020")) || !sameColor(cell.Style.Fg, lipgloss.Color("#C0C0C8")) {
		t.Fatal("token alpha was composed before token/line background")
	}
	colors.EditorSelection = "#80000080"
	rendered = diffCode(colors, "x", spans, 1, 0, colors.Background, "", diffRange{}, diffRange{0, 1})
	cell = uv.NewStyledString(rendered).Lines(ansi.GraphemeWidth)[0][0]
	if !sameColor(cell.Style.Bg, lipgloss.Color("#400010")) || !sameColor(cell.Style.Fg, lipgloss.Color("#D0C0C4")) {
		t.Fatal("selection/token foreground ignored actual compositing order")
	}
}

func TestHighContrastDecorationsRemainVisible(t *testing.T) {
	for _, id := range []string{"hc_black", "hc_light"} {
		t.Run(id, func(t *testing.T) {
			m := tabWorkspace(t, 1)
			t.Cleanup(m.Close)
			drain(t, m, m.SetColorTheme(theme.Builtin(id)))
			drain(t, m, m.openFile("file0.go"))
			firstLine := func() []uv.Cell {
				return uv.NewStyledString(m.editorLines()[0]).Lines(ansi.GraphemeWidth)[0]
			}
			cells := firstLine()
			code := cells[7]
			border := lipgloss.Color(theme.Composite(m.colors.CurrentLineBorder, m.colors.Background))
			if cells[6].Style.Underline != uv.UnderlineNone || code.Style.Underline != uv.UnderlineSingle || !sameColor(code.Style.UnderlineColor, border) {
				t.Fatal("current-line border did not start at the first text cell")
			}
			if !sameColor(code.Style.Bg, lipgloss.Color(m.colors.Background)) || sameColor(code.Style.Fg, code.Style.Bg) {
				t.Fatalf("current-line border erased the syntax token: %+v", code)
			}

			setTestFind(m, "package")
			drain(t, m, m.findNext())
			match := firstLine()[7]
			if match.Style.Underline == 0 || sameColor(match.Style.Fg, match.Style.Bg) {
				t.Fatalf("search hit lost its underline or text: %+v", match)
			}

			m.pickerIndex = 0
			choice := action{label: "Same label"}
			focused, regular := m.pickerRow(choice, 0, 30, -1), m.pickerRow(choice, 1, 30, -1)
			readCell := func(s string, x int) uv.Cell {
				return uv.NewStyledString(s).Lines(ansi.GraphemeWidth)[0][x]
			}
			if plain(focused) != plain(regular) || sameColor(readCell(focused, 0).Style.Bg, readCell(regular, 0).Style.Bg) {
				t.Fatal("picker focus should use a distinct background with the same label")
			}
			m.contextMenu = &contextMenu{actions: []action{choice, choice}}
			l := m.contextMenuLayout()
			menu := m.contextMenuOverlay(m.mainLines())
			focusedCell, regularCell := readCell(menu[l.y+1], l.x+1), readCell(menu[l.y+2], l.x+1)
			if focusedCell.Content != " " || sameColor(focusedCell.Style.Bg, regularCell.Style.Bg) {
				t.Fatal("context menu focus should use a distinct background with an empty prefix")
			}

			p := m.colors
			diff := diffCode(p, "changed", nil, 7, 0, p.AddedBackground, p.AddedText, diffRange{0, 7},
				diffRange{}, diffCodeOverlay{changeBorder: p.AddedBorder})
			change := uv.NewStyledString(diff).Lines(ansi.GraphemeWidth)[0][0]
			if change.Style.Underline == 0 || sameColor(change.Style.Fg, change.Style.Bg) {
				t.Fatalf("inline addition lost its underline or text: %+v", change)
			}
		})
	}
}
