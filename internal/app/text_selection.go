package app

import (
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/clipperhouse/uax29/v2/graphemes"
	"github.com/foursmith/vl/internal/diff"
)

// Mouse columns are terminal cells; selection columns are source rune offsets.
func textColumn(text string, goal int, gaps tablePadding) int {
	goal = max(0, goal)
	visual, col := 0, 0
	iter := graphemes.FromString(text)
	for iter.Next() {
		cluster := iter.Value()
		visual += gaps[iter.Start()]
		width := ansi.StringWidth(displayCluster(cluster, visual))
		if visual+width > goal {
			break
		}
		visual += width
		col += utf8.RuneCountInString(cluster)
	}
	return col
}

func (m *Model) positionEditor(x, y int, extend bool) {
	t := &m.tabs[m.activeTab]
	d := t.doc
	if t.vim.VisualMode {
		t.vim.Reset(d)
	}
	line := min(len(d.Lines())-1, max(0, m.editorRowAt(y-m.contentRow(), true)))
	col := textColumn(d.Lines()[line], x-m.mainX()-7+t.left, t.tableRows()[line])
	anchor := d.Anchor
	if anchor < 0 {
		anchor = d.Cursor
	}
	d.SetPosition(line, col)
	if extend {
		d.Anchor = anchor
	}
	m.ensureEditor()
}

type textPoint struct{ row, col int }
type diffSelection struct {
	anchor, cursor textPoint
	right          bool
	active         bool
}

func codeLine(line *diff.Line) string {
	if line != nil && (line.Kind == diff.Context || line.Kind == diff.Added || line.Kind == diff.Deleted) {
		return line.Text[1:]
	}
	return ""
}

func (m *Model) selectionLine(i int, right bool) *diff.Line {
	if i < 0 || i >= len(m.diffRows) {
		return nil
	}
	if right {
		return m.diffRows[i].Right
	}
	return m.diffRows[i].Left
}

func (m *Model) positionDiff(x, y int, extend bool) {
	if len(m.diffRows) == 0 {
		return
	}
	s := &m.diffSelection
	local := x - m.mainX()
	if !extend {
		right, _ := m.diffPane(true)
		s.right = m.diffLayout.split && local >= right
	}
	start, width := m.diffPane(s.right)
	local -= start
	i := min(len(m.diffRows)-1, max(0, m.viewport.YOffset()+y-m.contentRow()))
	code := codeLine(m.selectionLine(i, s.right))
	gutter := min(width, m.diffGutter)
	goal := min(max(0, local-gutter), width-gutter) + m.diffLeft
	p := textPoint{i, textColumn(code, goal, nil)}
	if !extend {
		s.anchor, s.active = p, true
	}
	s.cursor = p
	m.syncReviewMouse()
}

func (s diffSelection) bounds() (textPoint, textPoint) {
	a, b := s.anchor, s.cursor
	if a.row > b.row || a.row == b.row && a.col > b.col {
		a, b = b, a
	}
	return a, b
}

func (m *Model) diffSelectedRange(i int, right bool) diffRange {
	s := m.diffSelection
	if !s.active || s.right != right {
		return diffRange{}
	}
	a, b := s.bounds()
	if i < a.row || i > b.row {
		return diffRange{}
	}
	start, end := 0, utf8.RuneCountInString(codeLine(m.selectionLine(i, right)))
	if i == a.row {
		start = a.col
	}
	if i == b.row {
		end = b.col
	}
	return diffRange{start, end}
}

func (m *Model) diffSelectedText() string {
	s := m.diffSelection
	if !s.active {
		return ""
	}
	a, b := s.bounds()
	var lines []string
	for i := a.row; i <= b.row && i < len(m.diffRows); i++ {
		line := m.selectionLine(i, s.right)
		if line == nil || line.Kind != diff.Context && line.Kind != diff.Added && line.Kind != diff.Deleted {
			continue
		}
		code := []rune(codeLine(line))
		r := m.diffSelectedRange(i, s.right)
		lines = append(lines, string(code[min(r.start, len(code)):min(r.end, len(code))]))
	}
	return strings.Join(lines, "\n")
}

func (m *Model) stopTextDrag() {
	m.textDrag = nil
	m.diffDrag = nil
}

func (m *Model) dragText(x, y int) {
	if m.modal != "" {
		m.stopTextDrag()
		return
	}
	if m.textDrag != nil && m.editing() && m.document() == m.textDrag {
		m.positionEditor(x, y, true)
	} else if m.diffDrag != nil && !m.editing() && m.diffState == m.diffDrag {
		if y < m.contentRow() {
			m.viewport.ScrollUp(1)
		} else if y >= m.bodyHeight() {
			m.viewport.ScrollDown(1)
		}
		m.positionDiff(x, min(max(y, m.contentRow()), m.bodyHeight()-1), true)
	} else {
		m.stopTextDrag()
	}
}

func (m *Model) beginTextDrag(msg tea.MouseClickMsg) {
	if m.editing() {
		m.positionEditor(msg.X, msg.Y, msg.Mod.Contains(tea.ModShift))
		m.textDrag = m.document()
	} else if m.diffOpen {
		m.positionDiff(msg.X, msg.Y, msg.Mod.Contains(tea.ModShift) && m.diffSelection.active)
		m.diffDrag = m.diffState
	}
}
