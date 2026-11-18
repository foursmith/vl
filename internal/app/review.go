package app

import (
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/vl/internal/diff"
	"github.com/ourongxing/vl/internal/editor"
)

// Diff contributes source text and a mapping to painted rows. The editor's Vim
// engine owns cursor/selection commands for both editable and read-only text.
func (m *Model) editorDocument() *editor.Document {
	if d := m.document(); d != nil {
		return d
	}
	if m.diffOpen && m.images == nil {
		return m.review
	}
	return nil
}

func (m *Model) rebuildReview(right bool) {
	old := m.review
	oldRow, oldCol := 0, 0
	if old != nil {
		oldRow, oldCol = old.Position()
	}
	var oldLine *diff.Line
	for line, index := range m.reviewLine {
		if index == oldRow {
			oldLine = line
			break
		}
	}
	var lines []string
	m.reviewRows = nil
	m.reviewLine = map[*diff.Line]int{}
	for i, pair := range m.diffRows {
		line := pair.Left
		if right {
			line = pair.Right
		}
		if line == nil || (line.Kind != diff.Context && line.Kind != diff.Added && line.Kind != diff.Deleted) {
			continue
		}
		m.reviewLine[line] = len(lines)
		m.reviewRows = append(m.reviewRows, i)
		lines = append(lines, codeLine(line))
	}
	m.reviewRight = right
	m.review = &editor.Document{ReadOnly: true, Path: filepath.Join(m.repo.Root, m.diffPath), Text: strings.Join(lines, "\n"), Anchor: -1}
	if index, ok := m.reviewLine[oldLine]; ok {
		oldRow = index
	}
	m.review.SetPosition(oldRow, oldCol)
	if m.diffHunk < 0 && len(m.reviewRows) > 0 {
		row := m.reviewRows[min(oldRow, len(m.reviewRows)-1)]
		if row < len(m.diffRowHunks) {
			m.diffHunk = m.diffRowHunks[row]
		}
	}
	for i := range m.tabs {
		if m.tabs[i].diff == m.diffState {
			m.tabs[i].vim.Reset(m.review)
		}
	}
}

func documentPoint(d *editor.Document, offset int) textPoint {
	lines := d.Lines()
	row := sort.Search(len(lines), func(i int) bool { return d.LineOffset(i) > offset }) - 1
	row = max(0, row)
	return textPoint{row, offset - d.LineOffset(row)}
}

func (m *Model) syncReviewSelection() {
	d := m.review
	if d == nil || len(m.reviewRows) == 0 {
		return
	}
	a, b := d.Selection()
	if a == b {
		m.diffSelection = diffSelection{}
		return
	}
	start, end := documentPoint(d, a), documentPoint(d, b)
	m.diffSelection = diffSelection{
		active: true, right: m.reviewRight,
		anchor: textPoint{m.reviewRows[min(start.row, len(m.reviewRows)-1)], start.col},
		cursor: textPoint{m.reviewRows[min(end.row, len(m.reviewRows)-1)], end.col},
	}
}

func (m *Model) syncReviewMouse() {
	if m.images != nil || len(m.diffRows) == 0 {
		return
	}
	s := m.diffSelection
	if m.review == nil || s.right != m.reviewRight {
		m.rebuildReview(s.right)
	}
	if len(m.reviewRows) == 0 {
		return
	}
	offset := func(p textPoint) int {
		i := sort.SearchInts(m.reviewRows, p.row)
		i = min(i, len(m.reviewRows)-1)
		return m.review.LineOffset(i) + min(p.col, m.review.LineRuneCount(i))
	}
	m.tabs[m.activeTab].vim.Reset(m.review)
	m.review.SelectRange(offset(s.anchor), offset(s.cursor))
}

// Search navigation centers the matching row in both files and text diffs.
// Folded source lines and diff headers use their painted row coordinates.
func (m *Model) centerTextCursor() {
	d := m.editorDocument()
	if d == nil {
		return
	}
	line, _ := d.Position()
	if m.editing() {
		t := &m.tabs[m.activeTab]
		t.revealFold(line)
		t.top = t.sourceRow(max(0, t.visibleRow(line)-m.contentHeight()/2))
	} else if len(m.reviewRows) > 0 {
		row := m.reviewRows[min(line, len(m.reviewRows)-1)]
		m.viewport.SetYOffset(row - m.contentHeight()/2)
	}
	m.ensureTextCursor()
}

func (m *Model) ensureTextCursor() {
	if m.editing() {
		m.ensureEditor()
		return
	}
	if m.review == nil || len(m.reviewRows) == 0 {
		return
	}
	d := m.review
	line, _ := d.Position()
	row := m.reviewRows[min(line, len(m.reviewRows)-1)]
	if row < m.viewport.YOffset() {
		m.viewport.SetYOffset(row)
	}
	if row >= m.viewport.YOffset()+m.contentHeight() {
		m.viewport.SetYOffset(row - m.contentHeight() + 1)
	}
	visual := ansi.StringWidth(expandTabs(d.Text[d.ByteOffset(d.LineOffset(line)):d.ByteOffset(d.Cursor)]))
	width := m.mainWidth()
	if m.diffLayout.split {
		width = (width - 1) / 2
	}
	width = max(1, width-m.diffGutter)
	if visual < m.diffLeft {
		m.diffLeft = visual
	}
	if visual >= m.diffLeft+width {
		m.diffLeft = visual - width + 1
	}
	m.clampDiffHorizontal()
	if row < len(m.diffRowHunks) {
		m.diffHunk = m.diffRowHunks[row]
	}
	m.syncReviewSelection()
	m.ensureTabVisible()
}

func (m *Model) reviewCursor() *tea.Cursor {
	if m.review == nil || len(m.reviewRows) == 0 || m.sidebar || m.fileFindFocused() || m.modal != "" || m.contextMenu != nil {
		return nil
	}
	d := m.review
	line, _ := d.Position()
	y := m.reviewRows[min(line, len(m.reviewRows)-1)] - m.viewport.YOffset()
	start, end := d.ByteOffset(d.LineOffset(line)), d.ByteOffset(d.Cursor)
	x := m.mainX() + m.diffGutter + ansi.StringWidth(expandTabs(d.Text[start:end])) - m.diffLeft
	width := m.mainWidth()
	if m.diffLayout.split {
		width = (width - 1) / 2
		if m.reviewRight {
			x += width + 1
		}
	}
	left := m.mainX()
	if m.diffLayout.split && m.reviewRight {
		left += width + 1
	}
	if y < 0 || y >= m.contentHeight() || x < left+m.diffGutter || x >= left+width || m.fileFindAt(x, m.contentRow()+y) {
		return nil
	}
	return &tea.Cursor{Position: tea.Position{X: x, Y: m.contentRow() + y}, Shape: tea.CursorBlock, Color: lipgloss.Color(m.colors.Foreground)}
}

func (m *Model) reviewToRow(row int) {
	if m.review == nil || len(m.reviewRows) == 0 {
		return
	}
	i := min(sort.SearchInts(m.reviewRows, row), len(m.reviewRows)-1)
	if m.diffLayout.split && m.diffRowHunks[m.reviewRows[i]] != m.diffHunk {
		m.rebuildReview(!m.reviewRight)
		if len(m.reviewRows) == 0 {
			return
		}
		i = min(sort.SearchInts(m.reviewRows, row), len(m.reviewRows)-1)
	}
	m.review.SetPosition(i, 0)
	m.tabs[m.activeTab].vim.Reset(m.review)
	m.ensureTextCursor()
}

func (m *Model) textViewport() (top, left, width, gutter int) {
	if m.editing() {
		t := &m.tabs[m.activeTab]
		return t.editorDisplayTop(), t.left, m.mainWidth(), 7
	}
	width = m.mainWidth()
	if m.diffLayout.split {
		width = (width - 1) / 2
	}
	return m.viewport.YOffset(), m.diffLeft, width, m.diffGutter
}

func (m *Model) textRowAt(y int) int {
	if m.editing() {
		return m.editorRowAt(y, false)
	}
	row := m.viewport.YOffset() + y
	i := sort.SearchInts(m.reviewRows, row)
	if i < len(m.reviewRows) && m.reviewRows[i] == row {
		return i
	}
	return -1
}

func (m *Model) reviewActions() []action {
	sourceReason := ""
	if m.diffChange == nil || m.diffLoading {
		sourceReason = m.tr("Open a working-tree or index diff first")
	}
	stageReason, unstageReason := sourceReason, sourceReason
	if m.diffStaged {
		stageReason = m.tr("Open a working-tree diff first")
	} else {
		unstageReason = m.tr("Open an index diff first")
	}
	actions := []action{
		{id: "diff.stageHunk", label: m.tr("Stage Hunk"), disabled: stageReason, run: func() tea.Cmd { return m.stageHunk(false) }},
		{id: "diff.unstageHunk", label: m.tr("Unstage Hunk"), disabled: unstageReason, run: func() tea.Cmd { return m.stageHunk(true) }},
		{id: "diff.edit", label: m.tr("Open Working Tree File"), disabled: sourceReason, run: m.editDiff},
	}
	if m.diffLayout.split && m.review != nil {
		actions = append(actions, action{id: "diff.otherSide", label: m.tr("Focus Other Diff Side"), run: func() tea.Cmd {
			m.cancelFlash()
			m.rebuildReview(!m.reviewRight)
			m.ensureTextCursor()
			return nil
		}})
	}
	return actions
}
