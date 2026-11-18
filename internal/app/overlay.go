package app

import (
	"strings"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// Preserve the full frame while composing only the rows touched by a popup.
// Clip before composition to prevent a popup from enlarging the terminal frame.
func (m *Model) composeOverlay(base, popup []string, x, y int) []string {
	if x < 0 || y < 0 || x >= m.width || y >= len(base) {
		return base
	}
	popup = popup[:min(len(popup), len(base)-y)]
	clipped := make([]string, len(popup))
	popupWidth := 0
	for i, line := range popup {
		clipped[i] = ansi.Truncate(line, m.width-x, "")
		popupWidth = max(popupWidth, ansi.StringWidth(clipped[i]))
	}
	// Only compose the rows covered by the popup. Rebuilding the full canvas
	// reparses every image placeholder and pixel upload on each menu movement.
	canvas := lipgloss.NewCanvas(m.width, len(popup))
	uv.NewStyledString(strings.Join(base[y:y+len(popup)], "\n")).Draw(canvas, canvas.Bounds())
	uv.NewStyledString(strings.Join(clipped, "\n")).Draw(canvas, uv.Rect(x, 0, popupWidth, len(popup)))
	composed := strings.Split(canvas.Render(), "\n")
	lines := append([]string(nil), base...)
	// Canvas output omits trailing blanks; restore only the affected rows.
	for i := range popup {
		text := ""
		if i < len(composed) {
			text = composed[i]
		}
		lines[y+i] = row(text, m.width, m.colors.Foreground, m.colors.Background)
	}
	return lines
}
