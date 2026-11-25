package app

import (
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/clipperhouse/uax29/v2/graphemes"
	"github.com/foursmith/vl/internal/diff"
	"github.com/foursmith/vl/internal/editor"
)

const multiClickInterval = 400 * time.Millisecond

type textClick struct {
	target      any
	x, y, count int
	at          time.Time
}

func (m *Model) countTextClick(target any, x, y int) int {
	now := time.Now()
	previous := m.textClick
	count := 1
	if previous.target == target && previous.y == y && previous.x == x &&
		now.Sub(previous.at) < multiClickInterval && previous.count < 3 {
		count = previous.count + 1
	}
	m.textClick = textClick{target: target, x: x, y: y, count: count, at: now}
	return count
}

func (m *Model) mouseWordRange(d *editor.Buffer, offset int) (start, end int) {
	return d.InnerWordRange(offset, m.hanDictionary)
}

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
	m.inputDrag = nil
}

func (m *Model) dragText(x, y int) {
	if (m.textDrag != nil || m.diffDrag != nil || m.inputDrag != nil) && (x != m.textClick.x || y != m.textClick.y) {
		m.textClick = textClick{}
	}
	if m.inputDrag != nil {
		m.dragInput(x)
		return
	}
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
	extend := msg.Mod.Contains(tea.ModShift)
	if m.editing() {
		d := m.document()
		m.positionEditor(msg.X, msg.Y, extend)
		if msg.Mod == 0 {
			switch m.countTextClick(d, msg.X, msg.Y) {
			case 2:
				from, to := m.mouseWordRange(&d.Buffer, d.Cursor)
				d.SelectRange(from, to)
				m.ensureEditor()
				return
			case 3:
				line, _ := d.Position()
				from := d.LineOffset(line)
				to := from + d.LineRuneCount(line)
				if line+1 < len(d.Lines()) {
					to = d.LineOffset(line + 1)
				}
				d.SelectRange(from, to)
				m.ensureEditor()
				return
			}
		}
		m.textDrag = d
	} else if m.diffOpen {
		m.positionDiff(msg.X, msg.Y, extend && m.diffSelection.active)
		if msg.Mod == 0 && m.diffSelection.active {
			s := &m.diffSelection
			line := codeLine(m.selectionLine(s.cursor.row, s.right))
			switch m.countTextClick(m.diffState, msg.X, msg.Y) {
			case 2:
				s.anchor.col, s.cursor.col = m.mouseWordRange(&editor.Buffer{Text: line}, s.cursor.col)
				m.syncReviewMouse()
				return
			case 3:
				s.anchor.col, s.cursor.col = 0, utf8.RuneCountInString(line)
				m.syncReviewMouse()
				return
			}
		}
		m.diffDrag = m.diffState
	}
}
