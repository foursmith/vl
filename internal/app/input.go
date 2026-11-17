package app

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/clipperhouse/uax29/v2/graphemes"
)

func newInput() textinput.Model {
	input := textinput.New()
	input.CharLimit = 2000
	input.SetVirtualCursor(false)
	return input
}

// Paint text and the native caret from the same display-cell viewport.
func (m *Model) inputView(input textinput.Model, x, y int) string {
	width := max(1, input.Width())
	value := input.Value()
	prefix := string([]rune(value)[:input.Position()])
	left := 0
	cells := ansi.StringWidth(prefix)
	iter := graphemes.FromString(prefix)
	for cells > width && iter.Next() {
		cluster := iter.Value()
		left += len(cluster)
		cells -= ansi.StringWidth(cluster)
	}
	styles := input.Styles()
	state := styles.Blurred
	if input.Focused() {
		state = styles.Focused
		m.inputCaret = &tea.Cursor{
			Position: tea.Position{X: x + ansi.StringWidth(input.Prompt) + cells, Y: y},
			Shape:    styles.Cursor.Shape, Blink: styles.Cursor.Blink, Color: styles.Cursor.Color,
		}
	}
	text := state.Text.Inline(true).Render(value[left:])
	if value == "" {
		text = state.Placeholder.Inline(true).Render(input.Placeholder)
	}
	text = ansi.Truncate(text, width+1, "")
	padding := max(0, width+1-ansi.StringWidth(text))
	return state.Prompt.Inline(true).Render(input.Prompt) + text + state.Text.Inline(true).Render(strings.Repeat(" ", padding))
}
