package app

import (
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/diff"
	"github.com/foursmith/vl/internal/editor"
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

func (m *Model) rebuildReview() {
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
		if m.diffLayout.split {
			line = pair.Right
		}
		if line == nil || (line.Kind != diff.Context && line.Kind != diff.Added && line.Kind != diff.Deleted) {
			continue
		}
		m.reviewLine[line] = len(lines)
		m.reviewRows = append(m.reviewRows, i)
		lines = append(lines, codeLine(line))
	}
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
		active: true, right: m.diffLayout.split,
		anchor: textPoint{m.reviewRows[min(start.row, len(m.reviewRows)-1)], start.col},
		cursor: textPoint{m.reviewRows[min(end.row, len(m.reviewRows)-1)], end.col},
	}
}

func (m *Model) syncReviewMouse() {
	if m.review == nil {
		return
	}
	s := m.diffSelection
	m.tabs[m.activeTab].vim.Reset(m.review)
	// Left-side selections are painted and copied directly from the diff rows.
	if s.right != m.diffLayout.split || len(m.reviewRows) == 0 {
		return
	}
	offset := func(p textPoint) int {
		i := sort.SearchInts(m.reviewRows, p.row)
		i = min(i, len(m.reviewRows)-1)
		return m.review.LineOffset(i) + min(p.col, m.review.LineRuneCount(i))
	}
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
	_, width := m.diffPane(m.diffLayout.split)
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
	pane, width := m.diffPane(m.diffLayout.split)
	left := m.mainX() + pane
	x := left + m.diffGutter + ansi.StringWidth(expandTabs(d.Text[start:end])) - m.diffLeft
	if y < 0 || y >= m.contentHeight() || x < left+m.diffGutter || x >= left+width || m.fileFindAt(x, m.contentRow()+y) {
		return nil
	}
	return &tea.Cursor{Position: tea.Position{X: x, Y: m.contentRow() + y}, Shape: tea.CursorBlock, Color: lipgloss.Color(m.colors.Foreground)}
}

func (m *Model) reviewToRow(row int) {
	if m.review == nil {
		return
	}
	m.tabs[m.activeTab].vim.Reset(m.review)
	m.diffSelection = diffSelection{}
	i := sort.SearchInts(m.reviewRows, row)
	// A deletion-only hunk still gets selected and scrolled into view.
	if i == len(m.reviewRows) || m.diffRowHunks[m.reviewRows[i]] != m.diffHunk {
		return
	}
	m.review.SetPosition(i, 0)
	m.ensureTextCursor()
}

func (m *Model) textViewport() (top, left, width, gutter int) {
	if m.editing() {
		t := &m.tabs[m.activeTab]
		return t.editorDisplayTop(), t.left, m.mainWidth(), 7
	}
	_, width = m.diffPane(m.diffLayout.split)
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

func (m *Model) partialStageReason(reverse bool) string {
	if m.diffChange == nil || m.editing() || m.diffStaged != reverse {
		return m.tr("Open a working-tree or index diff first")
	}
	if m.diffLoading || m.diffStale {
		return m.tr("Wait for the diff to finish refreshing")
	}
	c := *m.diffChange
	if reason := m.stageFileReason(c, reverse); reason != "" {
		return reason
	}
	if m.images != nil || c.Gitlink || c.Conflicted() {
		return m.tr("Stage this file as a whole")
	}
	if err := m.patch.PartialStagingError(); err != nil {
		return m.tr(err.Error())
	}
	return ""
}

func (m *Model) partialDiscardReason() string {
	if m.busy {
		return m.tr("Wait for the current Git operation to finish")
	}
	if m.diffChange == nil || m.diffStaged || m.diffLoading || m.diffStale || m.editing() {
		return m.tr("Open a working-tree diff first")
	}
	c := *m.diffChange
	if c.Conflicted() || c.Gitlink || !c.Unstaged() {
		return m.tr("Select a non-conflicted working-tree file to discard")
	}
	if m.fileDirty(c.Path) {
		return m.tr("Save or close edited files before discarding")
	}
	if m.images != nil || m.patch.PartialStagingError() != nil {
		return m.tr("No text changes to discard partially")
	}
	return ""
}

func (m *Model) selectedDiffChanges() map[*diff.Line]bool {
	s := m.diffSelection
	start, end := s.bounds()
	if !s.active || start == end {
		return nil
	}
	last := end.row
	if end.col == 0 {
		last--
	}
	selected := make(map[*diff.Line]bool)
	for i := max(0, start.row); i <= last && i < len(m.diffRows); i++ {
		line := m.selectionLine(i, s.right)
		if line == nil || line.Kind != diff.Added && line.Kind != diff.Deleted {
			continue
		}
		selected[line] = true
		if m.diffLayout.split {
			if other := m.selectionLine(i, !s.right); other != nil && (other.Kind == diff.Added || other.Kind == diff.Deleted) {
				selected[other] = true
			}
		}
	}
	return selected
}

func (m *Model) reviewActions() []action {
	var actions []action
	sourceReason := ""
	if m.diffChange == nil {
		sourceReason = m.tr("Open a working-tree or index diff first")
	} else if m.diffLoading || m.diffStale {
		sourceReason = m.tr("Wait for the diff to finish refreshing")
	}
	if m.diffChange != nil {
		reverse := m.diffStaged
		prefix, file, hunk, ranges := "diff.stage", "Stage File", "Stage Hunk", "Stage Selected Ranges"
		if reverse {
			prefix, file, hunk, ranges = "diff.unstage", "Unstage File", "Unstage Hunk", "Unstage Selected Ranges"
		}
		target, raw := m.diffState, m.patch.Raw
		bind := func(stale string, run func() tea.Cmd) func() tea.Cmd {
			return func() tea.Cmd {
				if m.diffState != target || m.diffChange == nil || m.diffStaged != reverse || m.diffLoading || m.diffStale || m.patch.Raw != raw {
					m.notice = m.tr(stale)
					return nil
				}
				return run()
			}
		}
		stage := func(run func() tea.Cmd) func() tea.Cmd { return bind("Diff changed; refresh before staging", run) }
		partialReason := m.partialStageReason(reverse)
		discardReason := m.partialDiscardReason()
		addDiscard := func(id, label, prompt, shortcut string, selected func(*diff.Line) bool) {
			if reverse {
				return
			}
			actions = append(actions, action{id: id, label: m.tr(label), key: shortcut, disabled: discardReason,
				run: bind("Diff changed; refresh before discarding", func() tea.Cmd {
					return m.discardDiffLines(m.tr(prompt, m.diffPath), selected)
				}),
			})
		}
		selected := m.selectedDiffChanges()
		hunkStageKey, hunkDiscardKey := stageKey, discardKey
		if start, end := m.diffSelection.bounds(); m.diffSelection.active && start != end {
			hunkStageKey, hunkDiscardKey = "", ""
		}
		if len(selected) != 0 {
			inRange := func(line *diff.Line) bool { return selected[line] }
			actions = append(actions, action{id: prefix + "Ranges", label: m.tr(ranges), key: stageKey, disabled: partialReason, run: stage(func() tea.Cmd {
				return m.stageDiffLines(reverse, ranges, inRange)
			})})
			addDiscard("diff.discardRanges", "Discard Selected Ranges", "Permanently discard the selected edits to %s?", discardKey, inRange)
		}
		if index := m.diffHunk; index >= 0 && index < len(m.patch.Hunks) {
			inHunk := func(line *diff.Line) bool { return line.Hunk == index }
			actions = append(actions, action{id: prefix + "Hunk", label: m.tr(hunk), key: hunkStageKey, disabled: partialReason, run: stage(func() tea.Cmd {
				return m.stageDiffLines(reverse, hunk, inHunk)
			})})
			addDiscard("diff.discardHunk", "Discard Hunk", "Permanently discard this hunk in %s?", hunkDiscardKey, inHunk)
		}
		fileReason := sourceReason
		if fileReason == "" {
			fileReason = m.stageFileReason(*m.diffChange, reverse)
		}
		actions = append(actions, action{id: prefix + "File", label: m.tr(file), key: stageScopeKey, disabled: fileReason, run: stage(func() tea.Cmd {
			return m.stageFile(*m.diffChange, reverse)
		})})
		if !reverse {
			reason := sourceReason
			if reason == "" && raw == "" {
				reason = m.tr("Open a working-tree diff first")
			}
			if reason == "" {
				reason = m.discardFileReason(*m.diffChange)
			}
			actions = append(actions, action{id: "diff.discardFile", label: m.tr("Discard File"), key: discardScopeKey, disabled: reason,
				run: bind("Diff changed; refresh before discarding", func() tea.Cmd { return m.discardFile(*m.diffChange, raw) })})
		}
	}
	actions = append(actions, action{id: "diff.edit", label: m.tr("Open Working Tree File"), disabled: sourceReason, run: m.editDiff})
	return actions
}

func (m *Model) diffChangeShortcut(k string) (bool, tea.Cmd) {
	if m.sidebar || !m.diffOpen || m.diffChange == nil {
		return false, nil
	}
	switch k {
	case "shift+s":
		k = stageScopeKey
	case "shift+d":
		k = discardScopeKey
	case stageKey, stageScopeKey, discardKey, discardScopeKey:
	default:
		return false, nil
	}
	if m.tabs[m.activeTab].vim.CommandPending() {
		return false, nil
	}
	for _, action := range m.reviewActions() {
		if action.key != k {
			continue
		}
		if action.disabled != "" {
			m.notice = action.disabled
			return true, nil
		}
		return true, action.run()
	}
	if k == discardKey || k == discardScopeKey {
		if m.diffStaged {
			m.notice = m.tr("Open a working-tree diff first")
		} else if start, end := m.diffSelection.bounds(); m.diffSelection.active && start != end {
			m.notice = m.tr("Select changed lines to discard")
		} else {
			m.notice = m.tr("Use %s to discard changes to the whole file", discardScopeKey)
		}
	} else if start, end := m.diffSelection.bounds(); m.diffSelection.active && start != end {
		m.notice = m.tr("Select changed lines to stage or unstage")
	} else {
		m.notice = m.tr("Use %s to stage or unstage the whole file", stageScopeKey)
	}
	return true, nil
}
