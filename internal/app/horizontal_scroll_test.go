package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/vl/internal/diff"
	"github.com/ourongxing/vl/internal/highlight"
	"github.com/ourongxing/vl/internal/theme"
)

func TestFileHorizontalWheelPreservesCursorAndVerticalScroll(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openFile("file0.go")
	m.document().Edit(strings.Repeat("a", 200) + "END\n" + strings.Repeat("line\n", 80))
	m.document().SetPosition(0, 0)
	m.ensureEditor()
	cursor := m.document().Cursor
	for i := 0; i < 100; i++ {
		m.Update(tea.MouseWheelMsg{X: m.mainX() + 8, Y: 5, Button: tea.MouseWheelRight})
	}
	state := &m.tabs[m.activeTab]
	if state.left != 203-max(1, m.mainWidth()-7) || state.top != 0 || m.document().Cursor != cursor {
		t.Fatalf("unexpected scroll/cursor: left=%d top=%d cursor=%d", state.left, state.top, m.document().Cursor)
	}
	if !strings.Contains(plain(m.editorLines()[0]), "END") {
		t.Fatal("file tail is unreachable")
	}
	for i := 0; i < 100; i++ {
		m.Update(tea.MouseWheelMsg{X: m.mainX() + 8, Y: 5, Button: tea.MouseWheelUp, Mod: tea.ModShift})
	}
	if state.left != 0 || state.top != 0 {
		t.Fatal("shift wheel did not return to beginning horizontally")
	}
	m.Update(tea.MouseWheelMsg{X: m.sideX() + 2, Y: 5, Button: tea.MouseWheelRight})
	if state.left != 0 {
		t.Fatal("sidebar wheel scrolled file")
	}
}

func TestDiffHorizontalScrollKeepsGuttersAndSplitBoundary(t *testing.T) {
	m := tabWorkspace(t, 0)
	m.sidebar = false
	showDiffFixture(m)
	m.diffPath = "long.go"
	m.patch = diff.Parse("@@ -1 +1 @@\n-" + strings.Repeat("a", 200) + "OLD\n+" + strings.Repeat("b", 200) + "NEW\n")
	for _, split := range []bool{false, true} {
		m.split = split
		m.renderDiff()
		press(m, "G")
		press(m, "L") // The read-only editor reveals the cursor at the source line's end.
		if m.diffLeft == 0 || m.viewport.YOffset() != 0 {
			t.Fatal("horizontal keys did not preserve vertical position")
		}
		lines := m.diffView()
		if !strings.Contains(plain(strings.Join(lines, "\n")), "NEW") {
			t.Fatal("diff tail is unreachable")
		}
		for _, line := range lines {
			if ansi.StringWidth(line) != m.mainWidth() {
				t.Fatal("scroll changed rendered width")
			}
		}
		line := &m.patch.Lines[2]
		painted := plain(m.diffRow(line, 40, true, diffRange{}))
		if !strings.Contains(painted[:m.diffGutter], "1+") {
			t.Fatal("horizontal scroll moved line number")
		}
		m.scrollHorizontal(-10000)
		if m.diffLeft != 0 {
			t.Fatal("scrolled before first column")
		}
	}
	m.scrollHorizontal(30)
	m.Update(tea.WindowSizeMsg{Width: 1000, Height: 30})
	if m.diffLeft != 0 {
		t.Fatal("resize left diff scrolled beyond content")
	}
}

func TestDiffHorizontalClipUsesDisplayColumns(t *testing.T) {
	colors := theme.Builtin("dark_modern").Palette()
	code := "\t你e\u0301TAIL"
	spans := []highlight.Span{{Start: 1, End: 8, Color: "#569CD6", Font: theme.Bold}}
	for _, left := range []int{0, 1, 4, 5, 6, 7, 8} {
		got := diffCode(colors, code, spans, 6, left, colors.AddedBackground, colors.AddedText, diffRange{2, 8}, diffRange{})
		want := plain(row(ansi.Cut(expandTabs(code), left, left+6), 6, colors.Foreground, colors.AddedBackground))
		if plain(got) != want || ansi.StringWidth(got) != 6 {
			t.Fatalf("column %d: got %q, want %q", left, plain(got), want)
		}
	}
}
