package app

import (
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/clipperhouse/uax29/v2/graphemes"
	"github.com/foursmith/vl/internal/editor"
	"github.com/foursmith/vl/internal/theme"
)

type inputArea struct{ x, y, width int }

func inputBorderRule(style lipgloss.Style, width int, left, right string) string {
	return style.Render(left + strings.Repeat("─", max(0, width-2)) + right)
}

func newInput() textInput {
	input := textinput.New()
	input.KeyMap.LineStart.SetKeys("home")
	input.KeyMap.Paste.SetEnabled(false)
	input.CharLimit = 2000
	input.SetVirtualCursor(false)
	return textInput{Model: input, anchor: -1}
}

func (input textInput) viewport(value string, width int) (left, cells int) {
	prefix := string([]rune(value)[:input.Position()])
	left = input.viewportLeft
	if left > len(prefix) || left > 0 && ansi.StringWidth(value) <= width {
		left = 0
	}
	cells = ansi.StringWidth(prefix[left:])
	iter := graphemes.FromString(prefix[left:])
	for cells > width && iter.Next() {
		cluster := iter.Value()
		left += len(cluster)
		cells -= ansi.StringWidth(cluster)
	}
	return left, cells
}

func (m *Model) clickInput(input *textInput, area inputArea, x, y int) {
	prompt := ansi.StringWidth(input.Prompt)
	if !input.Focused() || y != area.y || x < area.x || x >= area.x+prompt+area.width+1 {
		return
	}
	value := input.Value()
	left, _ := input.viewport(value, area.width)
	position := utf8.RuneCountInString(value[:left]) + textColumn(value[left:], x-area.x-prompt, nil)
	start, end := -1, position
	switch m.countTextClick(input, x, y) {
	case 2:
		start, end = m.mouseWordRange(&editor.Document{Text: value}, position)
	case 3:
		start, end = 0, utf8.RuneCountInString(value)
	}
	input.viewportLeft = left
	input.anchor = start
	input.SetCursor(end)
	input.revision++
}

// Paint text and the native caret from the same display-cell viewport.
func (m *Model) inputView(input textInput, area inputArea) string {
	width := area.width
	value := input.Value()
	left, cells := input.viewport(value, area.width)
	styles := input.Styles()
	state := styles.Blurred
	if input.Focused() {
		state = styles.Focused
		m.inputCaret = &tea.Cursor{
			Position: tea.Position{X: area.x + ansi.StringWidth(input.Prompt) + cells, Y: area.y},
			Shape:    styles.Cursor.Shape, Blink: styles.Cursor.Blink, Color: styles.Cursor.Color,
		}
	}
	paint := state.Text.Inline(true)
	text := paint.Render(value[left:])
	if value == "" {
		text = state.Placeholder.Inline(true).Render(input.Placeholder)
	} else if start, end := input.selection(); input.Focused() && start < end {
		runes := []rune(value)
		startByte := max(left, len(string(runes[:start])))
		endByte := max(left, len(string(runes[:end])))
		if startByte < endByte {
			background := m.colors.Panel
			if m.gitMessageDialog() {
				background = m.colors.Input
			}
			fill := m.colors.EditorSelection
			if fill == "" {
				fill = m.colors.PopupSelection
			}
			fg := m.colors.EditorSelectionForeground
			if fg == "" {
				fg = m.colors.InputForeground
			}
			selected := theme.TextStyle(fg, theme.Composite(fill, background)).Inline(true)
			if m.monochrome {
				selected = lipgloss.NewStyle().Reverse(true).Inline(true)
			}
			text = paint.Render(value[left:startByte]) + selected.Render(value[startByte:endByte]) + paint.Render(value[endByte:])
		}
	}
	text = ansi.Truncate(text, width+1, "")
	padding := max(0, width+1-ansi.StringWidth(text))
	return state.Prompt.Inline(true).Render(input.Prompt) + text + state.Text.Inline(true).Render(strings.Repeat(" ", padding))
}
