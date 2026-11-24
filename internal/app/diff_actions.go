package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/diff"
	"github.com/foursmith/vl/internal/theme"
)

type diffButtons struct {
	hunk           int
	discard, stage diffButtonPosition
	visible        bool
}

type diffButtonPosition struct{ x, y, width int }

func (m *Model) hoveredDiffButtons() diffButtons {
	x, y := m.hoverX, m.hoverY
	if m.modal != "" || m.contextMenu != nil || m.busy || m.diffDrag != nil ||
		!m.diffOpen || m.editing() ||
		x < m.mainX() || x >= m.mainX()+m.mainWidth() || m.sidebarAt(x) ||
		y < m.contentRow() || y >= m.bodyHeight() || m.fileFindAt(x, y) {
		return diffButtons{}
	}
	top := m.viewport.YOffset()
	i := top + y - m.contentRow()
	if i >= len(m.diffRowHunks) || m.diffRowHunks[i] < 0 {
		return diffButtons{}
	}
	canStage := m.partialStageReason(m.diffStaged, true) == ""
	canDiscard := m.partialDiscardReason(true) == ""
	if !canStage && !canDiscard {
		return diffButtons{}
	}
	hunk := m.diffRowHunks[i]
	for i > top && m.diffRowHunks[i-1] == hunk {
		i--
	}
	limit := min(top+m.contentHeight(), len(m.diffRowHunks))
	start := i
	end, firstEdit, lastEdit := start, -1, -1
	for end < limit && m.diffRowHunks[end] == hunk {
		pair := m.diffRows[end]
		if pair.Left != nil && (pair.Left.Kind == diff.Added || pair.Left.Kind == diff.Deleted) ||
			pair.Right != nil && pair.Right.Kind == diff.Added {
			if firstEdit < 0 {
				firstEdit = end
			}
			lastEdit = end
		}
		end++
	}
	if start == end {
		return diffButtons{}
	}
	// Center the two rows over the visible changes in this hunk. Context
	// provides room for the buttons when only one changed row is visible.
	anchor := start
	if firstEdit >= 0 && end-start >= 2 {
		anchor = min(end-2, firstEdit+(lastEdit-firstEdit)/2)
	}
	column, width := m.diffGutter-1, 1
	if m.diffLayout.split {
		_, column = m.diffPane(false)
		width = diffSeparatorWidth
	}
	y = m.contentRow() + anchor - top
	if !canDiscard {
		return diffButtons{
			hunk: hunk, stage: diffButtonPosition{column, y, width}, visible: true,
		}
	}
	discard := diffButtonPosition{column, y, width}
	stage := diffButtonPosition{column, y + 1, width}
	if end-start == 1 {
		if !m.diffLayout.split {
			column = 0
		}
		discard = diffButtonPosition{column, y, 1}
		stage = diffButtonPosition{column + 1, y, max(1, width-1)}
	}
	if !canStage {
		stage = diffButtonPosition{}
	}
	return diffButtons{hunk: hunk, discard: discard, stage: stage, visible: true}
}

func (m *Model) diffButtonAt(pos diffButtonPosition, x, y int) bool {
	left := m.mainX() + pos.x
	return x >= left && x < left+pos.width && y == pos.y && !m.fileFindAt(left+pos.width-1, y)
}

func (m *Model) drawDiffButton(line string, y int, pos diffButtonPosition, glyph string) string {
	if pos.width == 0 || y != pos.y || m.fileFindAt(m.mainX()+pos.x+pos.width-1, y) {
		return line
	}
	fg, bg := m.colors.InputForeground, m.colors.Input
	if m.diffButtonAt(pos, m.hoverX, m.hoverY) {
		fg, bg = m.colors.Strong, theme.Composite(m.colors.Hover, bg)
	}
	if pos.width == diffSeparatorWidth {
		glyph = " " + glyph
	}
	x := pos.x
	return ansi.Cut(line, 0, x) + row(glyph, pos.width, fg, bg) + ansi.Cut(line, x+pos.width, m.mainWidth())
}

func (m *Model) diffButtonScope(hunk int) (func(*diff.Line) bool, bool) {
	selected := m.selectedDiffChanges()
	for line := range selected {
		if line.Hunk == hunk {
			return func(line *diff.Line) bool { return line.Hunk == hunk && selected[line] }, true
		}
	}
	return func(line *diff.Line) bool { return line.Hunk == hunk }, false
}

func (m *Model) diffButtonHint(buttons diffButtons) string {
	if !buttons.visible {
		return ""
	}
	discard := m.diffButtonAt(buttons.discard, m.hoverX, m.hoverY)
	if !discard && !m.diffButtonAt(buttons.stage, m.hoverX, m.hoverY) {
		return ""
	}
	_, ranges := m.diffButtonScope(buttons.hunk)
	switch {
	case discard && ranges:
		return m.tr("Discard Selected Ranges")
	case discard:
		return m.tr("Discard Hunk")
	case m.diffStaged && ranges:
		return m.tr("Unstage Selected Ranges")
	case m.diffStaged:
		return m.tr("Unstage Hunk")
	case ranges:
		return m.tr("Stage Selected Ranges")
	default:
		return m.tr("Stage Hunk")
	}
}

func (m *Model) clickDiffButton(msg tea.MouseClickMsg) (tea.Cmd, bool) {
	buttons := m.hoveredDiffButtons()
	if !buttons.visible {
		return nil, false
	}
	discard := m.diffButtonAt(buttons.discard, msg.X, msg.Y)
	if !discard && !m.diffButtonAt(buttons.stage, msg.X, msg.Y) {
		return nil, false
	}
	selected, ranges := m.diffButtonScope(buttons.hunk)
	m.focusEditor()
	m.diffHunk = buttons.hunk
	if discard {
		title := "Permanently discard this hunk in %s?"
		if ranges {
			title = "Permanently discard the selected edits to %s?"
		}
		return m.discardDiffLines(m.tr(title, m.diffPath), selected), true
	}
	title := "Stage Hunk"
	if ranges {
		title = "Stage Selected Ranges"
	}
	if m.diffStaged {
		title = "Unstage Hunk"
		if ranges {
			title = "Unstage Selected Ranges"
		}
	}
	return m.stageDiffLines(m.diffStaged, title, selected), true
}
