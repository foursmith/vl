package app

import (
	"fmt"
	"github.com/clipperhouse/uax29/v2/graphemes"
	"strings"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/elegit/internal/diff"
	"github.com/ourongxing/elegit/internal/highlight"
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
	m.hoverOnly = false
	m.hoverRender = hoverRenderCache{}
	m.viewport.SetWidth(m.mainWidth())
	m.viewport.SetHeight(m.contentHeight())
	m.resizeInput()
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
	hint    string
	divider bool
}

func (m *Model) View() tea.View {
	if m.width < 30 || m.height < 8 {
		v := tea.NewView(row("Resize terminal to at least 30 × 8", m.width, m.colors.Foreground, m.colors.Background))
		v.AltScreen = true
		if m.ThemeMode() != "auto" {
			v.BackgroundColor = lipgloss.Color(m.colors.Panel)
		}
		return v
	}
	key := hoverRenderKey{
		row: m.hoveredSidebarRow(), hint: m.hoverHint(),
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
			color := m.colors.Border
			if m.resizingSidebar || m.sidebarDividerAt(m.hoverX, m.hoverY) {
				color = m.colors.Accent
			}
			divider := row("│", 1, color, m.colors.Panel)
			if m.sidebarRight {
				line += divider + side[y]
			} else {
				line = side[y] + divider + line
			}
		}
		lines = append(lines, line)
	}
	if h := m.outputHeight(); h > 0 {
		lines = append(lines, row("  OUTPUT   ·   Git", m.width, m.colors.Foreground, m.colors.Panel))
		output := m.output
		start := max(0, len(output)-(h-1))
		for i := 0; i < h-1; i++ {
			text := ""
			if start+i < len(output) {
				text = "  " + safe(output[start+i])
			}
			lines = append(lines, row(text, m.width, m.colors.Muted, m.colors.Panel))
		}
	}
	lines = append(lines, m.statusLine())
	if m.modal != "" {
		lines = m.overlay(lines)
	}
	v := tea.NewView(strings.Join(lines, "\n"))
	v.AltScreen = true
	// Uncovered terminal padding should blend with the surrounding chrome.
	// Editor and diff cells paint their own backgrounds.
	if m.ThemeMode() != "auto" {
		v.BackgroundColor = lipgloss.Color(m.colors.Panel)
	}
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
	editing := m.showEditor && m.document() != nil
	if editing {
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
	w := m.mainWidth()
	key := diffLayoutKey{m.patch.Raw, m.diffPath, w, m.split && w >= 68}
	if m.diffRows != nil && m.diffLayout == key {
		return
	}
	m.diffLayout = key
	largest := 1
	for _, line := range m.patch.Lines {
		largest = max(largest, line.Old, line.New)
	}
	m.diffGutter = max(4, len(fmt.Sprint(largest))) + 2
	pairs := m.patch.Pairs()
	m.diffEmphasis = diffEmphasis(pairs)
	m.diffRows = make([]diff.Pair, 0, len(m.patch.Lines))
	m.diffRowHunks = make([]int, 0, len(m.patch.Lines))
	hidden := func(line *diff.Line) bool {
		if m.diffPath == "" || line == nil {
			return false
		}
		if line.Kind == diff.HunkHeader && len(m.patch.Hunks) == 1 && (strings.Contains(m.patch.Raw, "\nnew file mode ") || strings.Contains(m.patch.Raw, "\ndeleted file mode ")) {
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
	if len(m.patch.Hunks) == 0 {
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
	if m.activeTab < 0 || m.activeTab >= len(m.tabs) {
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
}

func (m *Model) editorLines() []string {
	t := &m.tabs[m.activeTab]
	d := t.doc
	source := d.Lines()
	line, col := d.Position()
	a, b := d.Selection()
	key := editorRenderKey{d.Text, d.Cursor, d.Anchor, m.mainWidth(), t.left, !m.sidebar}
	if t.renderedRows == nil || t.renderKey != key {
		t.renderKey = key
		t.renderedRows = make(map[int]string, m.contentHeight())
	}
	for i := range t.renderedRows {
		if i < t.top || i >= t.top+m.contentHeight() {
			delete(t.renderedRows, i)
		}
	}
	var lines []string
	for y := 0; y < m.contentHeight(); y++ {
		i := t.top + y
		if i >= len(source) {
			lines = append(lines, row("", m.mainWidth(), m.colors.Foreground, m.colors.Background))
			continue
		}
		if rendered, ok := t.renderedRows[i]; ok {
			lines = append(lines, rendered)
			continue
		}
		offset := d.LineOffset(i)
		bg := m.colors.Background
		if i == line {
			bg = m.colors.CurrentLine
		}
		var text, run strings.Builder
		runFG, runBG, runReverse := "", "", false
		var runFont highlight.FontStyle
		flush := func() {
			if run.Len() == 0 {
				return
			}
			style := syntaxStyle(runFG, runBG, runFont).Reverse(runReverse)
			text.WriteString(style.Render(run.String()))
			run.Reset()
		}
		appendRun := func(s, fg, bg string, reverse bool, font highlight.FontStyle) {
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
			var font highlight.FontStyle
			if t.highlightText == d.Text && i < len(t.spans) {
				spans := t.spans[i]
				for spanIndex < len(spans) && j >= spans[spanIndex].End {
					spanIndex++
				}
				if spanIndex < len(spans) && j >= spans[spanIndex].Start {
					fg = spans[spanIndex].Color
					font = spans[spanIndex].Font
				}
			}
			cellBG := bg
			if offset+j >= a && offset+j < b {
				cellBG = m.colors.Selection
			}
			appendRun(s, fg, cellBG, !m.sidebar && i == line && j == col, font)
		}
		if !m.sidebar && i == line && col == d.LineRuneCount(i) {
			appendRun(" ", m.colors.Foreground, bg, true, 0)
		}
		flush()
		numberColor := m.colors.LineNumber
		if i == line {
			numberColor = m.colors.Foreground
		}
		gutter := theme.Style(numberColor, m.colors.Background).Render(fmt.Sprintf("%5d  ", i+1))
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
	w := min(76, m.width-6)
	var content []string
	content = append(content, row(" "+safe(m.modalTitle), w-2, m.colors.Foreground, m.colors.Popup))
	if m.modal == "close" {
		content = append(content, row(" Unsaved changes will be lost.", w-2, m.colors.Muted, m.colors.Popup))
		buttons := theme.Style("#FFFFFF", m.colors.Accent).Render(" Save ") + theme.Style(m.colors.Foreground, m.colors.Border).Render(" Discard ") + theme.Style(m.colors.Foreground, m.colors.Popup).Render(" Cancel ")
		content = append(content, row(buttons, w-2, m.colors.Foreground, m.colors.Popup))
	} else if m.modal == "confirm" {
		if m.confirmationDetail != "" {
			content = append(content, row(" "+m.confirmationDetail, w-2, m.colors.Muted, m.colors.Popup))
		}
		content = append(content, row(" Enter to confirm · Esc to cancel", w-2, m.colors.Muted, m.colors.Popup))
	} else {
		content = append(content, row(" "+m.input.View(), w-2, m.colors.Foreground, m.colors.Input))
	}
	border := theme.Style(m.colors.Border, m.colors.Popup)
	box := []string{border.Render("┌" + strings.Repeat("─", w-2) + "┐")}
	for _, s := range content {
		box = append(box, border.Render("│")+s+border.Render("│"))
	}
	box = append(box, border.Render("└"+strings.Repeat("─", w-2)+"┘"))
	x, y := (m.width-w)/2, min(3, max(0, m.height-len(box)-1))
	for i, s := range box {
		if y+i >= len(base) {
			break
		}
		left := row(ansi.Cut(base[y+i], 0, x), x, m.colors.Foreground, m.colors.Panel)
		right := row(ansi.Cut(base[y+i], x+w, m.width), m.width-x-w, m.colors.Foreground, m.colors.Background)
		base[y+i] = left + s + right
	}
	return base
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
	if msg.Y == editorContentRow-1 && !m.showEditor && m.diffOpen {
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
	if m.showEditor && m.document() != nil && msg.Y >= editorContentRow && msg.Y < m.bodyHeight() {
		t := &m.tabs[m.activeTab]
		lines := t.doc.Lines()
		line := min(len(lines)-1, t.top+msg.Y-editorContentRow)
		goal := max(0, msg.X-m.mainX()-7+t.left)
		visual, col := 0, 0
		iter := graphemes.FromString(lines[line])
		for iter.Next() {
			cluster := iter.Value()
			width := ansi.StringWidth(displayCluster(cluster, visual))
			if visual+width > goal {
				break
			}
			visual += width
			col += utf8.RuneCountInString(cluster)
		}
		t.doc.SetPosition(line, col)
		m.ensureEditor()
	} else if !m.showEditor && msg.Y >= editorContentRow && msg.Y < m.bodyHeight() {
		i := m.viewport.YOffset() + msg.Y - editorContentRow
		hunk := -1
		if i < len(m.diffRowHunks) {
			hunk = m.diffRowHunks[i]
		}
		if hunk >= 0 {
			m.diffHunk = hunk
			m.renderDiff()
		}
	}
	return nil
}

func syntaxStyle(fg, bg string, font highlight.FontStyle) lipgloss.Style {
	return theme.Style(fg, bg).Bold(font&highlight.Bold != 0).Italic(font&highlight.Italic != 0).Underline(font&highlight.Underline != 0).Strikethrough(font&highlight.Strikethrough != 0)
}
