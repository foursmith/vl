package app

import (
	"context"
	"slices"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/foursmith/vl/internal/diff"
	gitservice "github.com/foursmith/vl/internal/git"
)

type diffFold struct {
	diff.ContextGap
	start int
	lines []diff.Line
}

func (m *Model) expandedDiffFoldStart(line *diff.Line) *diffFold {
	if line == nil || line.Kind != diff.Context || line.Hunk >= 0 {
		return nil
	}
	for i := range m.contextFolds {
		fold := &m.contextFolds[i]
		if len(fold.lines) > 0 && line == &fold.lines[0] {
			return fold
		}
	}
	return nil
}

func (m *Model) diffFoldHint(fold *diffFold) string {
	if fold.lines != nil {
		if fold.Count == 1 {
			return m.tr("Hide %d unchanged line", fold.Count)
		}
		return m.tr("Hide %d unchanged lines", fold.Count)
	}
	if fold.Count == 1 {
		return m.tr("Show %d unchanged line", fold.Count)
	}
	return m.tr("Show %d unchanged lines", fold.Count)
}

func readDiffContext(ctx context.Context, workspace *gitservice.Repository, repository, raw string) string {
	const maxContextBytes = 1 << 20
	id := diff.ContextBase(raw)
	if id == "" {
		return ""
	}
	repo, err := workspace.RepositoryAt(ctx, repository)
	if err != nil {
		return ""
	}
	data, err := repo.Blob(ctx, id, maxContextBytes)
	if err != nil {
		return ""
	}
	return string(data)
}

func (d *diffState) initContextFolds(source string) {
	d.contextSource, d.contextFolds = nil, nil
	if source == "" {
		return
	}
	lines := strings.Split(strings.TrimSuffix(source, "\n"), "\n")
	gaps := d.patch.ContextGaps(len(lines))
	if !slices.ContainsFunc(gaps, func(gap diff.ContextGap) bool { return gap.Count > 0 }) {
		return
	}
	d.contextSource = lines
	d.contextFolds = make([]diffFold, len(gaps))
	for i, gap := range gaps {
		d.contextFolds[i].ContextGap = gap
	}
}

func (m *Model) toggleDiffFold(fold *diffFold, keyboard bool) tea.Cmd {
	start := fold.start
	opening := fold.lines == nil
	top := m.viewport.YOffset()
	cursor, _ := m.review.Position()
	cursorInFold := !opening && cursor < len(m.reviewRows) &&
		m.reviewRows[cursor] >= start && m.reviewRows[cursor] < start+len(fold.lines)
	if opening {
		fold.lines = make([]diff.Line, fold.Count)
		for i := range fold.lines {
			fold.lines[i] = diff.Line{
				Kind: diff.Context, Hunk: -1, Old: fold.Old + i, New: fold.New + i,
				Text: " " + m.contextSource[fold.Old-1+i],
			}
		}
	} else {
		if top > start {
			top = max(start, top-len(fold.lines)+1)
		}
		fold.lines = nil
	}
	m.diffLayout = diffLayoutKey{}
	m.renderDiff()
	m.viewport.SetYOffset(top)
	if keyboard && opening || cursorInFold {
		index := sort.SearchInts(m.reviewRows, start)
		m.review.SetPosition(min(index, len(m.reviewRows)-1), 0)
		m.ensureTextCursor()
	}
	return m.highlightDiff()
}

func (m *Model) diffFoldAt(x, y int) *diffFold {
	if !m.diffOpen || len(m.contextFolds) == 0 || y < m.contentRow() || y >= m.bodyHeight() {
		return nil
	}
	column, width := x-m.mainX(), m.mainWidth()
	if column < 0 || column >= width {
		return nil
	}
	row := m.viewport.YOffset() + y - m.contentRow()
	line := m.selectionLine(row, false)
	if fold := m.expandedDiffFoldStart(line); fold != nil {
		_, leftWidth := m.diffPane(false)
		if column == 0 && leftWidth >= m.diffGutter {
			return fold
		}
		if m.diffLayout.split {
			rightStart, rightWidth := m.diffPane(true)
			if column == rightStart && rightWidth >= m.diffGutter {
				return fold
			}
		}
	}
	if line != nil && line.Kind == diff.Fold {
		return &m.contextFolds[line.Hunk]
	}
	return nil
}

func (m *Model) clickDiffFold(msg tea.MouseClickMsg) (tea.Cmd, bool) {
	fold := m.diffFoldAt(msg.X, msg.Y)
	if msg.Mod != 0 || fold == nil {
		return nil, false
	}
	m.focusEditor()
	m.blurFileFind()
	return m.toggleDiffFold(fold, false), true
}

func (m *Model) toggleNearestDiffFold() (bool, tea.Cmd) {
	if len(m.contextFolds) == 0 || len(m.reviewRows) == 0 {
		return false, nil
	}
	cursor, _ := m.review.Position()
	row := m.reviewRows[cursor]
	var nearest *diffFold
	distance := len(m.diffRows) + 1
	for i := range m.contextFolds {
		fold := &m.contextFolds[i]
		if fold.Count == 0 {
			continue
		}
		delta := row - fold.start
		if delta >= 0 && delta < len(fold.lines) {
			return true, m.toggleDiffFold(fold, true)
		}
		if delta >= 0 {
			delta -= len(fold.lines)
		}
		delta = max(delta, -delta)
		if delta < distance {
			nearest, distance = fold, delta
		}
	}
	if nearest == nil {
		return false, nil
	}
	return true, m.toggleDiffFold(nearest, true)
}
