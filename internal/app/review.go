package app

import (
	"path/filepath"
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/diff"
	"github.com/foursmith/vl/internal/editor"
)

// Diff contributes source text and a mapping to painted rows for the shared
// text cursor and selection commands.
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
	m.review = &editor.Document{Buffer: editor.Buffer{ReadOnly: true, Text: strings.Join(lines, "\n"), Anchor: -1}, Path: filepath.Join(m.repo.Root, m.diffPath)}
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
		if !m.vimDisabled && m.tabs[i].diff == m.diffState {
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

// Center the cursor using displayed rows, including folds, images and diff headers.
func (m *Model) centerTextCursor() {
	d := m.editorDocument()
	if d == nil {
		return
	}
	line, _ := d.Position()
	if m.editing() {
		t := &m.tabs[m.activeTab]
		t.revealFold(line)
		m.syncMarkdownImageRows(t)
		t.setEditorDisplayTop(t.editorDisplayRow(line) - m.contentHeight()/2)
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
	if row < len(m.diffRowHunks) && m.diffRowHunks[row] >= 0 {
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
	shape := tea.CursorBlock
	if m.vimDisabled {
		shape = tea.CursorBar
	}
	return &tea.Cursor{Position: tea.Position{X: x, Y: m.contentRow() + y}, Shape: shape, Color: lipgloss.Color(m.colors.Foreground)}
}

func (d *diffState) reviewHunkStartLine() int {
	if d.review == nil || len(d.reviewRows) == 0 || d.images != nil {
		return 0
	}
	row, _ := d.review.Position()
	paintedRow := d.reviewRows[min(row, len(d.reviewRows)-1)]
	hunk := d.diffRowHunks[paintedRow]
	if hunk < 0 {
		hunk = d.diffHunk
	}
	return d.patch.HunkStartLine(hunk)
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

func (m *Model) partialStageReason(reverse, checkRefresh bool) string {
	if m.diffChange == nil || m.editing() || m.diffStaged != reverse {
		return m.tr("Open a working-tree or index diff first")
	}
	if checkRefresh && (m.diffLoading || m.diffStale) {
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

func (m *Model) partialDiscardReason(checkRefresh bool) string {
	if m.busy {
		return m.tr("Wait for the current Git operation to finish")
	}
	if m.diffChange == nil || m.diffStaged || m.editing() || checkRefresh && (m.diffLoading || m.diffStale) {
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
	return m.reviewActionsForKey("", true)
}

func (m *Model) reviewActionsForKey(pressed string, checkRefresh bool) []action {
	var actions []action
	add := func(a action, shortcut key.Binding) {
		if pressed == "" || matchesShortcut(pressed, shortcut) {
			a.key = shortcut.Help().Key
			actions = append(actions, a)
		}
	}
	sourceReason := ""
	if m.diffChange == nil {
		sourceReason = m.tr("Open a working-tree or index diff first")
	} else if checkRefresh && (m.diffLoading || m.diffStale) {
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
		partialReason := m.partialStageReason(reverse, checkRefresh)
		discardReason := m.partialDiscardReason(checkRefresh)
		addDiscard := func(id, label string, shortcut key.Binding, selected func(*diff.Line) bool, prompt string, args ...any) {
			if reverse {
				return
			}
			add(action{id: id, label: m.tr(label), disabled: discardReason,
				run: bind("Diff changed; refresh before discarding", func() tea.Cmd {
					return m.discardDiffLines(m.tr(prompt, args...), selected)
				}),
			}, shortcut)
		}
		selected := m.selectedDiffChanges()
		hunkStageKey, hunkDiscardKey := gitKeys.Stage, gitKeys.Discard
		if start, end := m.diffSelection.bounds(); m.diffSelection.active && start != end {
			hunkStageKey, hunkDiscardKey = key.Binding{}, key.Binding{}
		}
		if len(selected) != 0 {
			inRange := func(line *diff.Line) bool { return selected[line] }
			add(action{id: prefix + "Ranges", label: m.tr(ranges), disabled: partialReason, run: stage(func() tea.Cmd {
				return m.stageDiffLines(reverse, ranges, inRange)
			})}, gitKeys.Stage)
			addDiscard("diff.discardRanges", "Discard Selected Ranges", gitKeys.Discard, inRange,
				"Permanently discard %d selected changed lines in %s?", len(selected), m.diffPath)
		}
		if index := m.diffHunk; index >= 0 && index < len(m.patch.Hunks) {
			inHunk := func(line *diff.Line) bool { return line.Hunk == index }
			add(action{id: prefix + "Hunk", label: m.tr(hunk), disabled: partialReason, run: stage(func() tea.Cmd {
				return m.stageDiffLines(reverse, hunk, inHunk)
			})}, hunkStageKey)
			addDiscard("diff.discardHunk", "Discard Hunk", hunkDiscardKey, inHunk,
				"Permanently discard hunk %d of %d in %s?", index+1, len(m.patch.Hunks), m.diffPath)
		}
		fileReason := sourceReason
		if fileReason == "" {
			fileReason = m.stageFileReason(*m.diffChange, reverse)
		}
		add(action{id: prefix + "File", label: m.tr(file), disabled: fileReason, run: stage(func() tea.Cmd {
			return m.stageFile(*m.diffChange, reverse)
		})}, gitKeys.StageScope)
		if !reverse {
			reason := sourceReason
			if reason == "" && raw == "" {
				reason = m.tr("Open a working-tree diff first")
			}
			if reason == "" {
				reason = m.discardFileReason(*m.diffChange)
			}
			add(action{id: "diff.discardFile", label: m.tr("Discard File"), disabled: reason,
				run: bind("Diff changed; refresh before discarding", func() tea.Cmd { return m.discardFile(*m.diffChange, raw) })}, gitKeys.DiscardScope)
		}
	}
	add(action{id: "diff.edit", label: m.tr("Open Working Tree File"), disabled: sourceReason, run: m.editDiff}, key.Binding{})
	return actions
}

func (m *Model) reviewKey(k string) (bool, tea.Cmd) {
	v := &m.tabs[m.activeTab].vim
	if v.CommandPending() || v.VisualMode {
		return false, nil
	}
	switch k {
	case "enter":
		return m.toggleNearestDiffFold()
	case "[":
		m.nextHunk(-1)
	case "]":
		m.nextHunk(1)
	default:
		return false, nil
	}
	return true, nil
}

func (m *Model) diffChangeShortcut(k string) (bool, tea.Cmd) {
	if m.sidebar || !m.diffOpen || m.diffChange == nil {
		return false, nil
	}
	if !isDiffGitShortcut(k) {
		return false, nil
	}
	if m.tabs[m.activeTab].vim.CommandPending() {
		return false, nil
	}
	if actions := m.reviewActionsForKey(k, true); len(actions) != 0 {
		action := actions[0]
		if action.disabled != "" {
			m.notice = action.disabled
			return true, nil
		}
		return true, action.run()
	}
	if matchesShortcut(k, gitKeys.Discard) || matchesShortcut(k, gitKeys.DiscardScope) {
		if m.diffStaged {
			m.notice = m.tr("Open a working-tree diff first")
		} else if start, end := m.diffSelection.bounds(); m.diffSelection.active && start != end {
			m.notice = m.tr("Select changed lines to discard")
		} else {
			m.notice = m.tr("Use %s to discard changes to the whole file", gitKeys.DiscardScope.Help().Key)
		}
	} else if start, end := m.diffSelection.bounds(); m.diffSelection.active && start != end {
		m.notice = m.tr("Select changed lines to stage or unstage")
	} else {
		m.notice = m.tr("Use %s to stage or unstage the whole file", gitKeys.StageScope.Help().Key)
	}
	return true, nil
}
