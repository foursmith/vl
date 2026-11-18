package app

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

func TestOverlayPreservesFrameAndStyledCells(t *testing.T) {
	m := &Model{width: 30, height: 8}
	base := make([]string, m.height)
	for i := range base {
		base[i] = row("ab中文 👨‍👩‍👧‍👦 text", m.width, "#eeeeee", "#222222")
	}
	for _, pos := range [][2]int{{3, 2}, {28, 6}, {0, 0}} {
		popup := []string{row("选项 one", 12, "#ffffff", "#004488"), row("two", 12, "#ffffff", "#004488"), row("three", 12, "#ffffff", "#004488")}
		x, y := pos[0], pos[1]
		popup = popup[:min(len(popup), len(base)-y)]
		clipped := make([]string, len(popup))
		for i, line := range popup {
			clipped[i] = ansi.Truncate(line, m.width-x, "")
		}
		want := lipgloss.NewCompositor(lipgloss.NewLayer(strings.Join(base, "\n")), lipgloss.NewLayer(strings.Join(clipped, "\n")).X(x).Y(y).Z(1)).Render()
		expectedLines := append([]string(nil), base...)
		rendered := strings.Split(want, "\n")
		for i := range popup {
			expectedLines[y+i] = row(rendered[y+i], m.width, m.colors.Foreground, m.colors.Background)
		}
		want = strings.Join(expectedLines, "\n")
		got := m.composeOverlay(base, popup, x, y)
		for i := range base {
			if (i < y || i >= y+len(popup)) && got[i] != base[i] {
				t.Fatal("overlay rewrote an uncovered row")
			}
		}
		actual, expected := uv.NewScreenBuffer(m.width, m.height), uv.NewScreenBuffer(m.width, m.height)
		uv.NewStyledString(strings.Join(got, "\n")).Draw(actual, actual.Bounds())
		uv.NewStyledString(want).Draw(expected, expected.Bounds())
		for cy := 0; cy < m.height; cy++ {
			for cx := 0; cx < m.width; cx++ {
				if !actual.CellAt(cx, cy).Equal(expected.CellAt(cx, cy)) {
					t.Fatalf("overlay at %v changed cell %d,%d: got %#v want %#v", pos, cx, cy, actual.CellAt(cx, cy), expected.CellAt(cx, cy))
				}
			}
		}
	}
}
