package app

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/clipperhouse/uax29/v2/graphemes"
	"github.com/ourongxing/elegit/internal/diff"
	"github.com/ourongxing/elegit/internal/theme"
)

func safe(text string) string {
	if strings.IndexFunc(text, unicode.IsControl) < 0 {
		return text
	}
	var b strings.Builder
	for _, r := range text {
		switch r {
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("    ")
		default:
			if !unicode.IsControl(r) {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}
func plain(s string) string { return ansi.Strip(s) }
func row(text string, width int, fg, bg string) string {
	fg = theme.Composite(fg, bg)
	width = max(0, width)
	text = ansi.Truncate(text, width, "")
	if !strings.ContainsRune(text, '\x1b') {
		return theme.Style(fg, bg).Render(text + strings.Repeat(" ", max(0, width-ansi.StringWidth(text))))
	}
	base := uv.Style{Fg: lipgloss.Color(fg), Bg: lipgloss.Color(bg)}
	var out strings.Builder
	var pen uv.Style
	write := func(content string, style uv.Style) {
		// Nested SGR resets must return to this surface, not the terminal's palette.
		if style.Fg == nil {
			style.Fg = base.Fg
		}
		if style.Bg == nil {
			style.Bg = base.Bg
		}
		out.WriteString(style.Diff(&pen))
		out.WriteString(content)
		pen = style
	}
	lines := uv.NewStyledString(text).Lines(ansi.GraphemeWidth)
	if len(lines) > 0 {
		for _, cell := range lines[0] {
			write(cell.Content, cell.Style)
		}
	}
	if padding := width - ansi.StringWidth(text); padding > 0 {
		write(strings.Repeat(" ", padding), base)
	}
	if out.Len() > 0 {
		out.WriteString("\x1b[0m")
	}
	return out.String()
}

func (m *Model) outputHeight() int {
	if m.outputVisible && m.height >= 18 {
		return 7
	}
	return 0
}

const headerHeight = 1
const tabRow = 0
const editorContentRow = headerHeight + 1

func (m *Model) bodyHeight() int    { return max(1, m.height-1-m.outputHeight()) }
func (m *Model) contentHeight() int { return max(1, m.bodyHeight()-editorContentRow) }

func (m *Model) resize() {
	m.clampWorkspaces()
	m.hoverOnly = false
	m.hoverRender = hoverRenderCache{}
	m.viewport.SetWidth(m.mainWidth())
	m.viewport.SetHeight(m.contentHeight())
	m.resizeInput()
	m.syncPicker()
	m.renderDiff()
	m.ensureSidebar()
	m.ensureEditor()
	m.ensureTabVisible()
}

// Reuse the previous frame only across passive mouse motion. All other messages
// invalidate it, including asynchronous file, Git and syntax updates.
type hoverRenderCache struct {
	main []string
	view tea.View
	key  hoverRenderKey
}

type hoverRenderKey struct {
	row     int
	picker  int
	hint    string
	divider bool
}

func (m *Model) View() tea.View {
	if m.width < 30 || m.height < 8 {
		v := tea.NewView(row("Resize terminal to at least 30 × 8", m.width, m.colors.Foreground, m.colors.Background))
		v.AltScreen = true
		v.BackgroundColor = lipgloss.Color(m.colors.Panel)
		return v
	}
	hoveredRow := m.hoveredSidebarRow()
	key := hoverRenderKey{
		row: hoveredRow, hint: m.hoverHint(),
		picker:  m.hoveredPickerIndex(),
		divider: m.sidebarDividerAt(m.hoverX, m.hoverY),
	}
	if m.hoverOnly && m.hoverRender.main != nil && m.hoverRender.key == key {
		return m.hoverRender.view
	}
	var lines []string
	side := m.sidebarLines()
	main := m.hoverRender.main
	// In narrow terminals the main surface itself contains the sidebar.
	if !m.hoverOnly || main == nil || m.width < 70 && m.sidebar && m.sidebarVisible {
		main = m.mainLines()
	}
	for y := 0; y < m.bodyHeight(); y++ {
		line := main[y]
		if m.sideShown() {
			color := m.colors.SideBarBorder
			if m.resizingSidebar || m.sidebarDividerAt(m.hoverX, m.hoverY) {
				color = m.colors.FocusBorder
			}
			glyph := "│"
			if color == "" {
				glyph = " "
			}
			divider := row(glyph, 1, color, m.colors.Panel)
			if m.sidebarRight {
				line += divider + side[y]
			} else {
				line = side[y] + divider + line
			}
		}
		lines = append(lines, line)
	}
	if h := m.outputHeight(); h > 0 {
		lines = append(lines, row("  OUTPUT   ·   Git", m.width, m.colors.OutputForeground, m.colors.OutputBackground))
		output := m.output
		start := max(0, len(output)-(h-1))
		for i := 0; i < h-1; i++ {
			text := ""
			if start+i < len(output) {
				text = "  " + safe(output[start+i])
			}
			lines = append(lines, row(text, m.width, m.colors.OutputForeground, m.colors.OutputBackground))
		}
	}
	lines = append(lines, m.statusLine())
	if m.modal != "" {
		lines = m.overlay(lines)
	}
	if m.contextMenu != nil {
		lines = m.contextMenuOverlay(lines)
	}
	v := tea.NewView(strings.Join(lines, "\n"))
	v.AltScreen = true
	// Terminal padding sits outside painted cells and should match the chrome.
	v.BackgroundColor = lipgloss.Color(m.colors.Panel)
	v.MouseMode = tea.MouseModeAllMotion
	v.ReportFocus = true
	m.hoverRender = hoverRenderCache{main: main, view: v, key: key}
	return v
}

func (m *Model) mainLines() []string {
	w, h := m.mainWidth(), m.bodyHeight()
	if m.width < 70 && m.sidebar && m.sidebarVisible {
		return m.sidebarLinesWidth(w)
	}
	lines := []string{m.tabBar()}
	lines = append(lines, m.contentToolbar())
	var content []string
	editing := m.editing()
	if m.currentImage() != nil {
		content = m.imageLines()
	} else if editing {
		content = m.editorLines()
	} else if m.patch.Raw != "" {
		content = m.diffView()
	} else if m.welcomeVisible() {
		content = m.welcomeLines()
	} else {
		message := "No changes in this diff"
		if m.section == changes && len(m.snapshot.Changes) == 0 {
			message = "Working tree clean"
		}
		if m.diffLoading {
			message = ""
		}
		content = make([]string, m.contentHeight())
		middle := max(0, m.contentHeight()/2-2)
		if message != "" {
			content[middle] = theme.Style(m.colors.Foreground, m.colors.Background).Render("  " + message)
		}
	}
	for i := 0; i < h-editorContentRow; i++ {
		text := ""
		if i < len(content) {
			text = content[i]
		}
		if editing {
			lines = append(lines, text)
		} else {
			lines = append(lines, row(text, w, m.colors.Foreground, m.colors.Background))
		}
	}
	return lines[:h]
}

type diffLayoutKey struct {
	raw, path string
	width     int
	split     bool
}

func (m *Model) renderDiff() {
	if m.images != nil {
		m.diffRows, m.diffRowHunks = nil, nil
		m.diffSelection = diffSelection{}
		m.diffHunk = -1
		m.viewport.SetContent("")
		return
	}
	w := m.mainWidth()
	key := diffLayoutKey{m.patch.Raw, m.diffPath, w, m.splitDiff()}
	if m.diffRows != nil && m.diffLayout == key {
		return
	}
	m.diffLayout = key
	m.diffSelection = diffSelection{}
	m.diffDrag = nil
	largest := 1
	for _, line := range m.patch.Lines {
		largest = max(largest, line.Old, line.New)
	}
	m.diffGutter = max(4, len(fmt.Sprint(largest))) + 2
	m.diffWidth = 0
	for _, line := range m.patch.Lines {
		if line.Kind == diff.Added || line.Kind == diff.Deleted || line.Kind == diff.Context {
			m.diffWidth = max(m.diffWidth, ansi.StringWidth(expandTabs(line.Text[1:])))
		}
	}
	m.clampDiffHorizontal()
	pairs := m.patch.Pairs()
	m.diffEmphasis = diffEmphasis(pairs)
	m.diffRows = make([]diff.Pair, 0, len(m.patch.Lines))
	m.diffRowHunks = make([]int, 0, len(m.patch.Lines))
	hidden := func(line *diff.Line) bool {
		if m.diffPath == "" || line == nil {
			return false
		}
		if line.Kind == diff.HunkHeader && len(m.patch.Hunks) == 1 && (m.patch.NewFile || m.patch.DeletedFile) {
			return true
		}
		if line.Kind != diff.Header {
			return false
		}
		for _, prefix := range []string{"diff --git ", "index ", "--- ", "+++ ", "new file mode ", "deleted file mode ", "old mode ", "new mode ", "similarity index ", "dissimilarity index ", "rename from ", "rename to ", "copy from ", "copy to "} {
			if strings.HasPrefix(line.Text, prefix) {
				return true
			}
		}
		return false
	}
	content := make([]string, 0, len(m.patch.Lines))
	appendRow := func(pair diff.Pair) {
		m.diffRows = append(m.diffRows, pair)
		m.diffRowHunks = append(m.diffRowHunks, pair.Hunk)
		text := diffSourceText(pair.Left)
		if key.split && pair.Right != nil && pair.Right != pair.Left {
			text += " " + diffSourceText(pair.Right)
		}
		content = append(content, ansi.Truncate(text, w, ""))
	}
	if key.split {
		for _, pair := range pairs {
			if !hidden(pair.Left) {
				appendRow(pair)
			}
		}
	} else {
		for i := range m.patch.Lines {
			line := &m.patch.Lines[i]
			if !hidden(line) {
				appendRow(diff.Pair{Left: line, Hunk: line.Hunk})
			}
		}
	}
	// Bubbles owns scrolling/search offsets; ANSI painting is limited to visible rows.
	m.viewport.SetContentLines(content)
}

func diffSourceText(line *diff.Line) string {
	if line == nil {
		return ""
	}
	if line.Kind == diff.HunkHeader {
		return hunkLabel(line.Text)
	}
	return safe(line.Text)
}

func (m *Model) nextHunk(delta int) {
	if m.images != nil || len(m.patch.Hunks) == 0 {
		return
	}
	m.diffHunk = min(max(0, m.diffHunk+delta), len(m.patch.Hunks)-1)
	for i, hunk := range m.diffRowHunks {
		if hunk == m.diffHunk {
			m.viewport.SetYOffset(i)
			break
		}
	}
}

func (m *Model) ensureEditor() {
	if m.document() == nil {
		return
	}
	t := &m.tabs[m.activeTab]
	line, col := t.doc.Position()
	t.doc.Lines()
	if line < t.top {
		t.top = line
	}
	if line >= t.top+m.contentHeight() {
		t.top = line - m.contentHeight() + 1
	}
	m.clampEditorScroll(t)
	m.revealEditorRow(line)
	visual := col
	if !t.doc.PlainASCII(line) {
		start, end := t.doc.ByteOffset(t.doc.LineOffset(line)), t.doc.ByteOffset(t.doc.Cursor)
		visual = ansi.StringWidth(expandTabs(t.doc.Text[start:end]))
	}
	w := max(1, m.mainWidth()-7)
	if visual < t.left {
		t.left = visual
	}
	if visual >= t.left+w {
		t.left = visual - w + 1
	}
	m.ensureTabVisible()
}

// Keep the final line within the last full screen, including after edits/resizes.
func (m *Model) clampEditorScroll(t *tab) {
	lines := len(t.doc.Lines())
	t.top = min(max(0, t.top), max(0, lines-m.contentHeight()))
}

// displayCluster is shared by painting, cursor scrolling and mouse hit testing.
func displayCluster(cluster string, visual int) string {
	if len(cluster) == 1 && cluster[0] >= 32 && cluster[0] < 127 {
		return cluster
	}
	if cluster == "\t" {
		return strings.Repeat(" ", 4-visual%4)
	}
	if strings.IndexFunc(cluster, unicode.IsControl) >= 0 {
		cluster = safe(cluster)
	}
	if cluster == "" {
		return " "
	}
	if ansi.StringWidth(cluster) == 0 {
		return "◌" + cluster
	}
	return cluster
}

func expandTabs(text string) string {
	var b strings.Builder
	col := 0
	iter := graphemes.FromString(text)
	for iter.Next() {
		cluster := iter.Value()
		display := displayCluster(cluster, col)
		b.WriteString(display)
		col += ansi.StringWidth(display)
	}
	return b.String()
}

// Cached rows are bounded to the current viewport; edits, selection, focus and
// horizontal layout changes invalidate them together.
type editorRenderKey struct {
	text                        string
	cursor, anchor, width, left int
	focused                     bool
	sticky                      [maxStickyRows]int
	stickyCount                 int
}

func (m *Model) editorLines() []string {
	t := &m.tabs[m.activeTab]
	d := t.doc
	source := d.Lines()
	line, col := d.Position()
	a, b := d.Selection()
	sticky := m.editorStickyRows()
	key := editorRenderKey{stickyCount: len(sticky), text: d.Text, cursor: d.Cursor, anchor: d.Anchor, width: m.mainWidth(), left: t.left, focused: !m.sidebar}
	copy(key.sticky[:], sticky)
	if t.renderedRows == nil || t.renderKey != key {
		t.renderKey = key
		t.renderedRows = make(map[int]string, m.contentHeight())
	}
	for i := range t.renderedRows {
		if !slices.Contains(sticky, i) && (i < t.top+len(sticky) || i >= t.top+m.contentHeight()) {
			delete(t.renderedRows, i)
		}
	}
	var lines []string
	for y := 0; y < m.contentHeight(); y++ {
		i := t.top + y
		if y < len(sticky) {
			i = sticky[y]
		}
		if i >= len(source) {
			lines = append(lines, row("", m.mainWidth(), m.colors.Foreground, m.colors.Background))
			continue
		}
		if rendered, ok := t.renderedRows[i]; ok {
			lines = append(lines, rendered)
			continue
		}
		offset := d.LineOffset(i)
		gutterBG := m.colors.Background
		if y < len(sticky) {
			gutterBG = m.colors.Panel
		}
		bg := gutterBG
		if i == line {
			bg = m.colors.CurrentLine
		}
		var text, run strings.Builder
		runFG, runBG, runReverse := "", "", false
		var runFont theme.FontStyle
		flush := func() {
			if run.Len() == 0 {
				return
			}
			style := syntaxStyle(runFG, runBG, runFont).Reverse(runReverse)
			text.WriteString(style.Render(run.String()))
			run.Reset()
		}
		appendRun := func(s, fg, bg string, reverse bool, font theme.FontStyle) {
			if fg != runFG || bg != runBG || reverse != runReverse || font != runFont {
				flush()
				runFG, runBG, runReverse, runFont = fg, bg, reverse, font
			}
			run.WriteString(s)
		}
		spanIndex := 0
		visual, runeIndex := 0, 0
		input := source[i]
		if d.PlainASCII(i) {
			start := min(t.left, len(input))
			input = input[start:]
			visual, runeIndex = start, start
		}
		iter := graphemes.FromString(input)
		for iter.Next() {
			cluster := iter.Value()
			j := runeIndex
			runeIndex += utf8.RuneCountInString(cluster)
			start := visual
			if start >= t.left+max(1, m.mainWidth()-7) {
				break
			}
			s := displayCluster(cluster, visual)
			visual += ansi.StringWidth(s)
			if visual <= t.left {
				continue
			}
			if start < t.left {
				s = strings.Repeat(" ", visual-t.left)
			}
			fg := m.colors.Foreground
			cellBG := bg
			var font theme.FontStyle
			if t.highlightText == d.Text && i < len(t.spans) {
				spans := t.spans[i]
				for spanIndex < len(spans) && j >= spans[spanIndex].End {
					spanIndex++
				}
				if spanIndex < len(spans) && j >= spans[spanIndex].Start {
					fg = spans[spanIndex].Color
					font = spans[spanIndex].Font
					if spans[spanIndex].Background != "" {
						cellBG = theme.Composite(spans[spanIndex].Background, bg)
					}
				}
			}
			if offset+j >= a && offset+j < b {
				if m.colors.EditorSelection != "" {
					cellBG = theme.Composite(m.colors.EditorSelection, cellBG)
				}
				if m.colors.EditorSelectionForeground != "" {
					fg = m.colors.EditorSelectionForeground
				}
			}
			appendRun(s, fg, cellBG, !m.sidebar && i == line && j == col, font)
		}
		if !m.sidebar && i == line && col == d.LineRuneCount(i) && visual >= t.left && visual < t.left+max(1, m.mainWidth()-7) {
			appendRun(" ", m.colors.Foreground, bg, true, 0)
		}
		flush()
		numberColor := m.colors.LineNumber
		if i == line {
			numberColor = m.colors.LineNumberActive
		}
		gutter := theme.Style(numberColor, gutterBG).Render(fmt.Sprintf("%5d  ", i+1))
		rendered := row(gutter+row(text.String(), max(1, m.mainWidth()-7), m.colors.Foreground, bg), m.mainWidth(), m.colors.Foreground, m.colors.Background)
		t.renderedRows[i] = rendered
		lines = append(lines, rendered)
	}
	return lines
}

func (m *Model) overlay(base []string) []string {
	if m.isPicker() {
		return m.pickerOverlay(base)
	}
	l := m.dialogLayout()
	w := l.width
	border := theme.Style(m.colors.Border, m.colors.Popup)
	tl, tr, bl, br := "┌", "┐", "└", "┘"
	if m.confirmationDialog() {
		tl, tr, bl, br = "╭", "╮", "╰", "╯"
	}
	box := []string{border.Render(tl + strings.Repeat("─", w-2) + tr)}
	for _, s := range l.content {
		box = append(box, border.Render("│")+s+border.Render("│"))
	}
	box = append(box, border.Render(bl+strings.Repeat("─", w-2)+br))
	return m.composeOverlay(base, box, l.x, l.y)
}

func (m *Model) click(msg tea.MouseClickMsg) tea.Cmd {
	if msg.Y >= 0 && msg.Y < headerHeight && msg.X >= m.mainX() && msg.X < m.mainX()+m.mainWidth() && !(m.width < 70 && m.sidebar && m.sidebarVisible) && (msg.Button == tea.MouseLeft || msg.Button == tea.MouseMiddle) {
		return m.clickTab(msg)
	}
	if msg.Button != tea.MouseLeft {
		return nil
	}
	if msg.Y == m.height-1 {
		return m.clickStatus(msg.X)
	}
	if m.sidebarAt(msg.X) {
		return m.clickSidebar(msg)
	}
	if msg.Y == editorContentRow-1 && !m.editing() && m.diffOpen {
		button := m.diffModeButton()
		if msg.X >= m.mainX()+button.x && msg.X < m.mainX()+button.x+button.width {
			if m.mainWidth() < 68 {
				m.notice = "编辑区至少需要 68 列才能并排显示"
				return nil
			}
			m.split = !m.split
			m.renderDiff()
		}
		return nil
	}
	m.sidebar = false
	m.lastClickKey = ""
	if m.welcomeVisible() {
		return m.clickWelcome(msg)
	}
	if msg.Y >= editorContentRow && msg.Y < m.bodyHeight() {
		m.beginTextDrag(msg)
		if !m.editing() && m.diffOpen {
			i := m.viewport.YOffset() + msg.Y - editorContentRow
			if i < len(m.diffRowHunks) && m.diffRowHunks[i] >= 0 {
				m.diffHunk = m.diffRowHunks[i]
			}
		}
	}
	return nil
}

func syntaxStyle(fg, bg string, font theme.FontStyle) lipgloss.Style {
	return theme.Style(fg, bg).Bold(font&theme.Bold != 0).Italic(font&theme.Italic != 0).Underline(font&theme.Underline != 0).Strikethrough(font&theme.Strikethrough != 0)
}
