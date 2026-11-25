package app

import (
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/clipperhouse/uax29/v2/graphemes"
	"github.com/foursmith/vl/internal/theme"
)

type inputArea struct{ x, y, width int }

type inputDrag struct {
	field     *textInput
	area      inputArea
	anchor, x int
	revision  uint64
}

type inputStyles struct {
	text, placeholder lipgloss.Style
	cursor            tea.Cursor
}

func inputBorderRule(style lipgloss.Style, width int, left, right string) string {
	return style.Render(left + strings.Repeat("─", max(0, width-2)) + right)
}

func newInput() textInput {
	var input textInput
	input.buffer.ClearSelection()
	return input
}

func (input *textInput) viewport(width int) (left, cells int) {
	value := input.Value()
	prefix := value[:input.buffer.ByteOffset(input.Position())]
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
	left, _ := input.viewport(area.width)
	position := input.positionAt(left, x-area.x-prompt)
	input.viewportLeft = left
	input.revision++
	switch m.countTextClick(input, x, y) {
	case 2:
		start, end := m.mouseWordRange(&input.buffer, position)
		input.buffer.SelectRange(start, end)
	case 3:
		input.buffer.SelectAll()
	default:
		input.buffer.SelectRange(position, position)
		m.inputDrag = &inputDrag{field: input, area: area, anchor: input.Position(), x: x, revision: input.revision}
	}
}

func (input *textInput) positionAt(left, column int) int {
	value := input.Value()
	if column < 0 {
		prefix := value[:left]
		return textColumn(prefix, ansi.StringWidth(prefix)+column, nil)
	}
	return utf8.RuneCountInString(value[:left]) + textColumn(value[left:], column, nil)
}

func (m *Model) dragInput(x int) {
	drag := m.inputDrag
	input := drag.field
	if m.focusedInput() != input || input.revision != drag.revision {
		m.inputDrag = nil
		return
	}
	if x == drag.x {
		return
	}
	left, _ := input.viewport(drag.area.width)
	position := input.positionAt(left, x-drag.area.x-ansi.StringWidth(input.Prompt))
	input.buffer.SelectRange(drag.anchor, position)
	input.viewportLeft = min(left, input.buffer.ByteOffset(input.Position()))
	input.revision++
	drag.x, drag.revision = x, input.revision
}

// Paint text and the native caret from the same display-cell viewport.
func (m *Model) inputView(input textInput, area inputArea) string {
	width := area.width
	value := input.Value()
	left, cells := input.viewport(width)
	styles := input.styles
	startByte, endByte := left, left
	if input.Focused() {
		cursor := styles.cursor
		cursor.Position = tea.Position{X: area.x + ansi.StringWidth(input.Prompt) + cells, Y: area.y}
		m.inputCaret = &cursor
		start, end := input.buffer.Selection()
		if start < end {
			startByte = max(left, input.buffer.ByteOffset(start))
			endByte = max(left, input.buffer.ByteOffset(end))
		}
	}
	paint := styles.text.Inline(true)
	var text string
	switch {
	case value == "":
		text = styles.placeholder.Inline(true).Render(input.Placeholder)
	case startByte < endByte:
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
	default:
		text = paint.Render(value[left:])
	}
	text = ansi.Truncate(text, width+1, "")
	padding := max(0, width+1-ansi.StringWidth(text))
	return paint.Render(input.Prompt) + text + paint.Render(strings.Repeat(" ", padding))
}
