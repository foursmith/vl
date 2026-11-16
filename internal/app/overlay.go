package app

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Keep a full-screen base layer so floating layers retain their coordinates.
// Clip before composition to prevent a popup from enlarging the terminal frame.
func (m *Model) composeOverlay(base, popup []string, x, y int) []string {
	if x < 0 || y < 0 || x >= m.width || y >= len(base) {
		return base
	}
	popup = popup[:min(len(popup), len(base)-y)]
	clipped := make([]string, len(popup))
	for i, line := range popup {
		clipped[i] = ansi.Truncate(line, m.width-x, "")
	}
	composed := lipgloss.NewCompositor(
		lipgloss.NewLayer(strings.Join(base, "\n")),
		lipgloss.NewLayer(strings.Join(clipped, "\n")).X(x).Y(y).Z(1),
	).Render()
	lines := strings.Split(composed, "\n")
	// Canvas output omits trailing blanks; restore the frame's exact cell size.
	for i := range lines {
		lines[i] = row(lines[i], m.width, m.colors.Foreground, m.colors.Background)
	}
	return lines
}
