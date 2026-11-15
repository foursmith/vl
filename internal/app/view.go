package app

import (
	"fmt"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/elegit/internal/diff"
	"github.com/ourongxing/elegit/internal/highlight"
	"github.com/ourongxing/elegit/internal/theme"
)

func safe(text string) string {
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

func (m *Model) sideShown() bool { return m.sidebarVisible && m.width >= 70 }
func (m *Model) sideWidth() int {
	if !m.sideShown() {
		return 0
	}
	return min(44, max(28, m.width/4))
}
func (m *Model) mainX() int {
	if m.sideShown() {
		return 1 + m.sideWidth()
	}
	return 0
}
func (m *Model) mainWidth() int { return max(1, m.width-m.mainX()) }
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
	m.viewport.SetWidth(m.mainWidth())
	m.viewport.SetHeight(m.contentHeight())
	m.input.SetWidth(max(10, min(66, m.width-14)))
	m.renderDiff()
	m.ensureSidebar()
	m.ensureEditor()
	m.ensureTabVisible()
}

func (m *Model) View() tea.View {
	if m.width < 30 || m.height < 8 {
		v := tea.NewView(row("Resize terminal to at least 30 × 8", m.width, theme.Foreground, theme.Background))
		v.AltScreen = true
		v.BackgroundColor = lipgloss.Color(theme.Background)
		return v
	}
	var lines []string
	side := m.sidebarLines()
	main := m.mainLines()
	for y := 0; y < m.bodyHeight(); y++ {
		line := ""
		if m.sideShown() {
			line = side[y] + row("│", 1, theme.Border, theme.Panel)
		}
		line += main[y]
		lines = append(lines, line)
	}
	if h := m.outputHeight(); h > 0 {
		lines = append(lines, row("  OUTPUT   ·   Git", m.width, theme.Foreground, theme.Panel))
		var output []string
		for _, s := range m.output {
			output = append(output, strings.Split(s, "\n")...)
		}
		start := max(0, len(output)-(h-1))
		for i := 0; i < h-1; i++ {
			text := ""
			if start+i < len(output) {
				text = "  " + safe(output[start+i])
			}
			lines = append(lines, row(text, m.width, theme.Muted, theme.Panel))
		}
	}
	lines = append(lines, m.statusLine())
	if m.modal != "" {
		lines = m.overlay(lines)
	}
	v := tea.NewView(strings.Join(lines, "\n"))
	v.AltScreen = true
	v.BackgroundColor = lipgloss.Color(theme.Background)
	v.MouseMode = tea.MouseModeAllMotion
	v.ReportFocus = true
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
	if m.showEditor && m.document() != nil {
		content = m.editorLines()
	} else if m.patch.Raw != "" {
		content = strings.Split(m.viewport.View(), "\n")
	} else {
		message := "Select a file to preview"
		if m.section == changes && len(m.snapshot.Changes) == 0 {
			message = "Working tree clean"
		}
		if m.diffLoading {
			message = "Loading diff…"
		}
		content = make([]string, m.contentHeight())
		middle := max(0, m.contentHeight()/2-2)
		content[middle] = theme.Style(theme.Foreground, theme.Background).Render("  " + message)
	}
	for i := 0; i < h-editorContentRow; i++ {
		text := ""
		if i < len(content) {
			text = content[i]
		}
		lines = append(lines, row(text, w, theme.Foreground, theme.Background))
	}
	return lines[:h]
}

func (m *Model) renderDiff() {
	w := m.mainWidth()
	var lines []string
	m.diffRowHunks = nil
	appendLine := func(text string, hunk int) {
		lines = append(lines, text)
		m.diffRowHunks = append(m.diffRowHunks, hunk)
	}
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
	if m.split && w >= 68 {
		left := (w - 1) / 2
		right := w - left - 1
		for _, pair := range m.patch.Pairs() {
			if hidden(pair.Left) {
				continue
			}
			if pair.Left != nil && (pair.Left.Kind == diff.Header || pair.Left.Kind == diff.HunkHeader) {
				appendLine(m.diffRow(pair.Left, w, false), pair.Hunk)
				continue
			}
			appendLine(m.diffRow(pair.Left, left, false)+row("│", 1, theme.Border, theme.Background)+m.diffRow(pair.Right, right, true), pair.Hunk)
		}
	} else {
		for i := range m.patch.Lines {
			if hidden(&m.patch.Lines[i]) {
				continue
			}
			appendLine(m.diffRow(&m.patch.Lines[i], w, false), m.patch.Lines[i].Hunk)
		}
	}
	m.viewport.SetContent(strings.Join(lines, "\n"))
}

func (m *Model) diffRow(line *diff.Line, width int, right bool) string {
	if line == nil {
		return row("", width, theme.Muted, theme.Panel)
	}
	fg, bg := theme.Foreground, theme.Background
	text := safe(line.Text)
	switch line.Kind {
	case diff.Header:
		fg = theme.Muted
		if m.diffPath != "" && strings.HasPrefix(line.Text, "Binary files ") {
			text = "  二进制文件已更改"
		}
	case diff.HunkHeader:
		fg, bg = theme.Muted, theme.Panel
		marker := " "
		if line.Hunk == m.diffHunk {
			marker = m.symbol(symbolRight)
			fg = theme.Foreground
		}
		text = " " + marker + " " + hunkLabel(line.Text)
	case diff.Added:
		bg = theme.AddedBackground
	case diff.Deleted:
		bg = theme.DeletedBackground
	}
	if line.Kind == diff.Added || line.Kind == diff.Deleted || line.Kind == diff.Context {
		number := line.Old
		if right || line.Kind == diff.Added {
			number = line.New
		}
		text = theme.Style(theme.LineNumber, bg).Render(fmt.Sprintf("%4d ", number)) + text
	}
	return row(text, width, fg, bg)
}

func (m *Model) nextHunk(delta int) {
	if len(m.patch.Hunks) == 0 {
		return
	}
	m.diffHunk = min(max(0, m.diffHunk+delta), len(m.patch.Hunks)-1)
	m.renderDiff()
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
	if t.highlightText != t.doc.Text {
		t.highlightText = t.doc.Text
		t.spans = highlight.Lines(t.doc.Path, t.doc.Text)
	}
	line, col := t.doc.Position()
	lines := strings.Split(t.doc.Text, "\n")
	if line < t.top {
		t.top = line
	}
	if line >= t.top+m.contentHeight() {
		t.top = line - m.contentHeight() + 1
	}
	visual := ansi.StringWidth(expandTabs(string([]rune(lines[line])[:col])))
	w := max(1, m.mainWidth()-7)
	if visual < t.left {
		t.left = visual
	}
	if visual >= t.left+w {
		t.left = visual - w + 1
	}
	m.ensureTabVisible()
}

func expandTabs(text string) string {
	var b strings.Builder
	col := 0
	for _, r := range text {
		if r == '\t' {
			n := 4 - col%4
			b.WriteString(strings.Repeat(" ", n))
			col += n
		} else {
			b.WriteRune(r)
			col += ansi.StringWidth(string(r))
		}
	}
	return b.String()
}

func (m *Model) editorLines() []string {
	t := &m.tabs[m.activeTab]
	d := t.doc
	source := strings.Split(d.Text, "\n")
	line, col := d.Position()
	a, b := d.Selection()
	var lines []string
	offset := 0
	for i := 0; i < t.top && i < len(source); i++ {
		offset += len([]rune(source[i])) + 1
	}
	for y := 0; y < m.contentHeight(); y++ {
		i := t.top + y
		if i >= len(source) {
			lines = append(lines, "")
			continue
		}
		bg := theme.Background
		if i == line {
			bg = "#252526"
		}
		var text strings.Builder
		visual := 0
		for j, r := range []rune(source[i]) {
			start := visual
			if start >= t.left+max(1, m.mainWidth()-7) {
				break
			}
			s := safe(string(r))
			if r == '\t' {
				s = strings.Repeat(" ", 4-visual%4)
			}
			visual += ansi.StringWidth(s)
			if visual <= t.left {
				continue
			}
			if start < t.left {
				s = strings.Repeat(" ", visual-t.left)
			}
			fg := theme.Foreground
			if i < len(t.spans) {
				for _, span := range t.spans[i] {
					if j >= span.Start && j < span.End {
						fg = span.Color
						break
					}
				}
			}
			style := theme.Style(fg, bg)
			if offset+j >= a && offset+j < b {
				style = style.Background(lipgloss.Color(theme.Selection))
			}
			if !m.sidebar && i == line && j == col {
				style = style.Reverse(true)
			}
			text.WriteString(style.Render(s))
		}
		if !m.sidebar && i == line && col == len([]rune(source[i])) {
			text.WriteString(theme.Style(theme.Foreground, bg).Reverse(true).Render(" "))
		}
		numberColor := theme.LineNumber
		if i == line {
			numberColor = theme.Foreground
		}
		gutter := theme.Style(numberColor, theme.Background).Render(fmt.Sprintf("%5d  ", i+1))
		lines = append(lines, gutter+row(text.String(), max(1, m.mainWidth()-7), theme.Foreground, bg))
		offset += len([]rune(source[i])) + 1
	}
	return lines
}

func (m *Model) overlay(base []string) []string {
	w := min(76, m.width-6)
	var content []string
	content = append(content, row(" "+safe(m.modalTitle), w-2, theme.Foreground, "#222222"))
	if m.modal == "close" {
		content = append(content, row(" Unsaved changes will be lost.", w-2, theme.Muted, "#222222"))
		buttons := theme.Style("#FFFFFF", theme.Accent).Render(" Save ") + theme.Style(theme.Foreground, theme.Border).Render(" Discard ") + theme.Style(theme.Foreground, "#222222").Render(" Cancel ")
		content = append(content, row(buttons, w-2, theme.Foreground, "#222222"))
	} else if m.modal == "confirm" {
		if m.confirmationDetail != "" {
			content = append(content, row(" "+m.confirmationDetail, w-2, theme.Muted, "#222222"))
		}
		content = append(content, row(" Enter to confirm · Esc to cancel", w-2, theme.Muted, "#222222"))
	} else {
		content = append(content, row(" "+m.input.View(), w-2, theme.Foreground, "#313131"))
	}
	if m.isPicker() {
		rows := m.filteredActions()
		count := min(max(1, m.height-9), 12)
		start := max(0, m.choice-count+1)
		for i := start; i < min(len(rows), start+count); i++ {
			a := rows[i]
			bg, fg := "#222222", theme.Foreground
			if i == m.choice {
				bg = theme.Selection
			}
			if a.disabled != "" {
				fg = theme.Muted
			}
			label := " " + a.label
			if a.key != "" {
				if m.modal == "refs" {
					label = ansi.Truncate(label, max(1, w-ansi.StringWidth(a.key)-5), "…")
					label += strings.Repeat(" ", max(1, w-3-ansi.StringWidth(label)-ansi.StringWidth(a.key))) + a.key
				} else {
					label += "  " + a.key
				}
			}
			content = append(content, row(label, w-2, fg, bg))
		}
		if len(rows) > 0 && rows[min(m.choice, len(rows)-1)].disabled != "" {
			content = append(content, row(" "+rows[min(m.choice, len(rows)-1)].disabled, w-2, theme.Muted, "#222222"))
		}
	}
	border := theme.Style(theme.Border, "#222222")
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
		left := row(ansi.Cut(base[y+i], 0, x), x, theme.Foreground, theme.Panel)
		right := row(ansi.Cut(base[y+i], x+w, m.width), m.width-x-w, theme.Foreground, theme.Background)
		base[y+i] = left + s + right
	}
	return base
}

func (m *Model) click(msg tea.MouseClickMsg) tea.Cmd {
	if msg.Y >= 0 && msg.Y < headerHeight && msg.X >= m.mainX() && !(m.width < 70 && m.sidebar && m.sidebarVisible) && (msg.Button == tea.MouseLeft || msg.Button == tea.MouseMiddle) {
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
	if m.showEditor && m.document() != nil && msg.Y >= editorContentRow && msg.Y < m.bodyHeight() {
		t := &m.tabs[m.activeTab]
		lines := strings.Split(t.doc.Text, "\n")
		line := min(len(lines)-1, t.top+msg.Y-editorContentRow)
		goal := max(0, msg.X-m.mainX()-7+t.left)
		visual, col := 0, 0
		for _, r := range []rune(lines[line]) {
			width := ansi.StringWidth(string(r))
			if r == '\t' {
				width = 4 - visual%4
			}
			if visual+width > goal {
				break
			}
			visual += width
			col++
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
