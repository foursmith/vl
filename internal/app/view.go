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
	"github.com/foursmith/vl/internal/diff"
	searchservice "github.com/foursmith/vl/internal/search"
	"github.com/foursmith/vl/internal/theme"
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

func underlineRow(rendered, color string, startColumn int) string {
	var out strings.Builder
	var pen uv.Style
	underline := lipgloss.Color(color)
	column := 0
	for _, cell := range uv.NewStyledString(rendered).Lines(ansi.GraphemeWidth)[0] {
		style := cell.Style
		if column >= startColumn && style.Underline == uv.UnderlineNone {
			style.Underline = uv.UnderlineSingle
			style.UnderlineColor = underline
		}
		out.WriteString(style.Diff(&pen))
		out.WriteString(cell.Content)
		pen = style
		column += cell.Width
	}
	out.WriteString("\x1b[0m")
	return out.String()
}

const tabRow = 0
const editorContentRow = tabRow + 2
const editorGutterWidth = 7

func (m *Model) contentRow() int {
	if m.editorOnly {
		return 0
	}
	return editorContentRow
}

func (m *Model) bodyHeight() int    { return max(1, m.height-1) }
func (m *Model) contentHeight() int { return max(1, m.bodyHeight()-m.contentRow()) }

func (m *Model) resize() {
	unchangedPosition := false
	if m.document() != nil {
		t := &m.tabs[m.activeTab]
		unchangedPosition = t.positionBaseline != nil && t.positionBaseline.Equal(documentPosition(*t))
	}
	m.hoverOnly = false
	m.hoverRender = hoverRenderCache{}
	m.pickerFilter = pickerFilterCache{}
	m.resizeInput()
	m.syncPicker()
	m.renderDiff()
	m.ensureSidebar()
	m.clampSearch()

	m.ensureEditor()
	// Layout adjustments alone do not make an untouched position worth saving.
	if unchangedPosition {
		m.tabs[m.activeTab].capturePosition()
	}
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
	row, outlineRow       int
	menu                  *contextMenu
	menuSelected, menuTop int
	picker                int
	findX                 int
	findY                 int
	hint                  string
	divider               bool
	diffButtons           diffButtons
}

func (m *Model) View() tea.View {
	if m.width < 30 || m.height < 8 {
		v := tea.NewView(row(m.tr("Resize terminal to at least 30 × 8"), m.width, m.colors.Foreground, m.colors.Background))
		v.AltScreen = true
		v.BackgroundColor = lipgloss.Color(m.colors.Panel)
		return v
	}
	key := hoverRenderKey{
		row: m.hoveredSidebarRow(), outlineRow: m.hoveredOutlineRow(), hint: m.hoverHint(),
		picker:      m.hoveredPickerIndex(),
		divider:     m.sidebarDividerAt(m.hoverX, m.hoverY),
		diffButtons: m.hoveredDiffButtons(),
	}
	if menu := m.contextMenu; menu != nil {
		key.menu, key.menuSelected, key.menuTop = menu, menu.selected, menu.top
	}
	if m.fileFindAt(m.hoverX, m.hoverY) {
		key.findX, key.findY = m.hoverX+1, m.hoverY+1
	}
	if m.hoverOnly && m.hoverRender.main != nil && m.hoverRender.key == key {
		return m.hoverRender.view
	}
	var lines []string
	if !m.hoverOnly {
		m.inputCaret = nil
	}
	side := m.sidebarLines()
	main := m.hoverRender.main
	// In narrow terminals the main surface itself contains the sidebar.
	if !m.hoverOnly || main == nil || m.width < 70 && m.sidebar && m.sidebarVisible {
		main = m.mainLines()
	}
	find := m.fileFindLines()
	findOffset := m.fileFindX() - m.mainX()
	for y := 0; y < m.bodyHeight(); y++ {
		line := main[y]
		if key.diffButtons.visible {
			glyph := "+"
			if m.diffStaged {
				glyph = "−"
			}
			line = m.drawDiffButton(line, y, key.diffButtons.discard, "→")
			line = m.drawDiffButton(line, y, key.diffButtons.stage, glyph)
		}
		if index := y - m.contentRow(); index >= 0 && index < len(find) {
			line = row(ansi.Cut(line, 0, findOffset), findOffset, m.colors.Foreground, m.colors.Background) + find[index]
		}
		if m.sideShown() {
			color := m.colors.SideBarBorder
			if m.colors.Accent != "" && (m.resizingSidebar || m.sidebarDividerAt(m.hoverX, m.hoverY)) {
				color = m.colors.Accent
			}
			glyph := " "
			if color != "" {
				glyph = "│"
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
	lines = append(lines, m.statusLine())
	cursor := m.editorCursor()
	if m.pathCompletion != nil {
		lines = m.pathCompletionOverlay(lines, cursor)
	}
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
	v.Cursor = cursor
	if m.inputCaret != nil && m.contextMenu == nil {
		v.Cursor = m.inputCaret
	}
	m.hoverRender = hoverRenderCache{main: main, view: v, key: key}
	return v
}

func (m *Model) editorCursor() *tea.Cursor {
	if !m.editing() && m.diffOpen {
		return m.reviewCursor()
	}
	if !m.editing() || m.sidebar || m.fileFindFocused() || m.modal != "" || m.contextMenu != nil {
		return nil
	}
	t := &m.tabs[m.activeTab]
	line, col := t.doc.Position()
	y := t.editorDisplayRow(line) - t.editorDisplayTop()
	sticky := m.editorStickyRows()
	if index := slices.Index(sticky, line); index >= 0 {
		y = index
	} else if y < len(sticky) {
		return nil
	}
	x := 7 + t.editorColumn(line, col) - t.left
	if x < 7 || x >= m.mainWidth() || y < 0 || y >= m.contentHeight() || m.fileFindAt(m.mainX()+x, m.contentRow()+y) {
		return nil
	}
	shape, blink := tea.CursorBar, true
	if !t.vim.InsertMode {
		shape, blink = tea.CursorBlock, false
	}
	return &tea.Cursor{Position: tea.Position{X: m.mainX() + x, Y: m.contentRow() + y}, Shape: shape, Blink: blink, Color: lipgloss.Color(m.colors.Foreground)}
}

func (m *Model) mainLines() []string {
	w, h := m.mainWidth(), m.bodyHeight()
	if m.width < 70 && m.sidebar && m.sidebarVisible {
		return m.sidebarLinesWidth(w)
	}
	var lines []string
	if !m.editorOnly {
		lines = append(lines, m.tabBar())
		lines = append(lines, m.contentToolbar())
	}
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
		message := m.tr("No changes in this diff")
		if m.section == changes && len(m.snapshot.Changes) == 0 {
			message = m.tr("Working tree clean")
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
	for i := 0; i < h-m.contentRow(); i++ {
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
	w, height := m.mainWidth(), m.contentHeight()
	m.viewport.SetWidth(w)
	heightChanged := m.viewport.Height() != height
	m.viewport.SetHeight(height)
	if m.images != nil {
		m.diffRows, m.diffRowHunks = nil, nil
		m.diffSelection = diffSelection{}
		m.diffHunk = -1
		m.viewport.SetContent("")
		return
	}
	key := diffLayoutKey{m.patch.Raw, m.diffPath, w, m.splitDiff()}
	if m.diffRows != nil && m.diffLayout == key {
		if heightChanged {
			m.setDiffScrollRows()
		}
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
		if m.diffPR.Number != 0 && len(m.patch.Hunks) == 0 {
			return strings.HasPrefix(line.Text, "diff --git ")
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
	appendRow := func(pair diff.Pair) {
		m.diffRows = append(m.diffRows, pair)
		m.diffRowHunks = append(m.diffRowHunks, pair.Hunk)
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
	m.setDiffScrollRows()
	m.rebuildReview()
}

func (m *Model) nextHunk(delta int) {
	if m.images != nil || len(m.patch.Hunks) == 0 {
		return
	}
	m.diffHunk = min(max(0, m.diffHunk+delta), len(m.patch.Hunks)-1)
	for i, hunk := range m.diffRowHunks {
		if hunk == m.diffHunk {
			m.viewport.SetYOffset(i)
			m.reviewToRow(i)
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
	t.revealFold(line)
	m.syncMarkdownImageRows(t)
	cursorRow, top := t.editorDisplayRow(line), t.editorDisplayTop()
	if cursorRow < top {
		t.setEditorDisplayTop(cursorRow)
	} else if cursorRow >= top+m.contentHeight() {
		t.setEditorDisplayTop(cursorRow - m.contentHeight() + 1)
	}
	m.clampEditorScroll(t)
	m.revealEditorRow(line)
	visual := t.editorColumn(line, col)
	w := max(1, m.mainWidth()-editorGutterWidth)
	if visual < t.left {
		t.left = visual
	}
	if visual >= t.left+w {
		t.left = visual - w + 1
	}
	m.ensureTabVisible()
}

// Allow scrolling the last text row to the middle while keeping any trailing image reachable.
func (m *Model) clampEditorScroll(t *tab) {
	height := m.contentHeight()
	lastRow := t.editorDisplayRow(len(t.doc.Lines()) - 1)
	maxTop := max(0, lastRow-height/2)
	if cache := t.markdownImages; cache != nil && len(cache.blocks) > 0 {
		last := cache.blocks[len(cache.blocks)-1]
		maxTop = max(maxTop, last.first+last.region.Rows-height)
	}
	t.setEditorDisplayTop(min(t.editorDisplayTop(), maxTop))
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

// Document-wide changes invalidate the cache. Cursor and sticky styling
// are keyed per row so movement can reuse the rest of the viewport.
type editorRenderKey struct {
	flashRevision                             uint64
	findOptions                               searchservice.Options
	text                                      string
	selectionStart, selectionEnd, width, left int
}

type editorRowKey struct {
	current, sticky, stickyBorder bool
	guideColumn                   int
	guideColor                    string
}

type editorRenderedRow struct {
	key  editorRowKey
	text string
}

func (m *Model) editorLines() []string {
	width, height := m.mainWidth(), m.contentHeight()
	textWidth := max(1, width-editorGutterWidth)
	t := &m.tabs[m.activeTab]
	d := t.doc
	source := d.Lines()
	highlights := t.highlightRows(source)
	tableRows := t.tableRows()
	line, _ := d.Position()
	a, b := d.Selection()
	guides := t.editorGuides(source)
	flash := m.flash
	if !m.flashValid() {
		flash = nil
	}
	var bracket bracketGuide
	if a == b && flash == nil {
		bracket = guides.activeBracket(d.Cursor, m.colors)
	}
	sticky := m.editorStickyRows()
	findOptions := m.findOptions
	if m.findHidden {
		findOptions = searchservice.Options{}
	}
	key := editorRenderKey{findOptions: findOptions, flashRevision: m.flashRevision, text: d.Text, width: width, left: t.left}
	if a != b {
		key.selectionStart, key.selectionEnd = a, b
	}
	if t.renderedRows == nil || t.renderKey != key {
		t.renderKey = key
		t.renderedRows = make(map[int]editorRenderedRow, height)
	}
	top := t.editorDisplayTop()
	for i := range t.renderedRows {
		display := t.editorDisplayRow(i)
		if !slices.Contains(sticky, i) && (display < top+len(sticky) || display >= top+height) {
			delete(t.renderedRows, i)
		}
	}
	lines := make([]string, 0, height)
	var blank string
	currentRow := -1
	for y := 0; y < height; y++ {
		i, image, imageOffset := t.editorDisplayAt(top + y)
		isSticky := y < len(sticky)
		if isSticky {
			i = sticky[y]
		} else if image != nil {
			content := ""
			if image.image == nil {
				content = m.markdownRemoteImageRow(t.markdownImages, image, imageOffset)
			} else if mdRows := image.image.regionRows(image.region); imageOffset <= len(mdRows) {
				content = mdRows[imageOffset-1]
			}
			lines = append(lines, row("", editorGutterWidth, m.colors.Foreground, m.colors.Background)+row(content, textWidth, m.colors.Foreground, m.colors.Background))
			continue
		}
		if i >= len(source) {
			if blank == "" {
				blank = row("", width, m.colors.Foreground, m.colors.Background)
			}
			lines = append(lines, blank)
			continue
		}
		highlightCurrentLine := i == line && a == b
		if highlightCurrentLine && !isSticky {
			currentRow = y
		}
		rowKey := editorRowKey{current: i == line, sticky: isSticky, stickyBorder: isSticky && y == len(sticky)-1}
		var rowBracket bracketGuide
		if !isSticky && bracket.color != "" && bracket.column >= t.left && i > bracket.start && i < bracket.end {
			rowBracket = bracket
			rowKey.guideColumn, rowKey.guideColor = bracket.column, bracket.color
		}
		if rendered, ok := t.renderedRows[i]; ok && rendered.key == rowKey {
			lines = append(lines, rendered.text)
			continue
		}
		offset := d.LineOffset(i)
		gutterBG := m.colors.Background
		if isSticky {
			gutterBG = m.colors.StickyBackground
		}
		bg := gutterBG
		if highlightCurrentLine {
			bg = m.colors.CurrentLine
		}
		foldedLines := 0
		for _, fold := range t.folds {
			if fold.start == i {
				foldedLines = fold.end - fold.start
				bg = theme.Composite(m.colors.FoldBackground, bg)
				gutterBG = bg
				break
			}
		}
		var text, run strings.Builder
		runFG, runBG, runUnderline := "", "", ""
		var runFont theme.FontStyle
		flush := func() {
			if run.Len() == 0 {
				return
			}
			style := theme.CodeStyle(runFG, runBG, runUnderline, runFont)
			text.WriteString(style.Render(run.String()))
			run.Reset()
		}
		appendRun := func(s, fg, bg, underline string, font theme.FontStyle) {
			if fg != runFG || bg != runBG || underline != runUnderline || font != runFont {
				flush()
				runFG, runBG, runUnderline, runFont = fg, bg, underline, font
			}
			run.WriteString(s)
		}
		matches := m.fileSearchRowRanges(d, i)
		matchIndex := 0
		spans := highlights.at(i)
		spanIndex := 0
		visual, runeIndex := 0, 0
		input := source[i]
		padding := tableRows[i]
		if len(padding) == 0 && d.PlainASCII(i) {
			start := min(t.left, len(input))
			input = input[start:]
			visual, runeIndex = start, start
		}
		right := t.left + textWidth
		iter := graphemes.FromString(input)
		for iter.Next() {
			cluster := iter.Value()
			j := runeIndex
			runeIndex += utf8.RuneCountInString(cluster)
			gapStart := visual
			if gapStart >= right {
				break
			}
			visual += padding[iter.Start()]
			start := visual
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
			if depth, ok := guides.depths[offset+j]; ok {
				if color := m.colors.BracketColor(depth); color != "" {
					fg = color
				}
			}
			for matchIndex < len(matches) && matches[matchIndex][1] <= j {
				matchIndex++
			}
			searchMatch := matchIndex < len(matches) && matches[matchIndex][0] < runeIndex
			selected := offset+j >= a && offset+j < b
			if selected {
				if m.colors.EditorSelection != "" {
					cellBG = theme.Composite(m.colors.EditorSelection, cellBG)
				}
				if m.colors.EditorSelectionForeground != "" {
					fg = m.colors.EditorSelectionForeground
				}
			}
			underline := ""
			if searchMatch {
				fg, cellBG, underline = m.colors.FindColors(fg, cellBG, selected)
			}
			if flash != nil && flash.matches[offset+j] {
				cellBG = m.colors.PopupSelection
				if label := flash.marks[offset+j]; label != "" {
					s = label + strings.Repeat(" ", max(0, ansi.StringWidth(s)-1))
					fg = m.colors.Background
					cellBG = m.colors.Accent
					font = theme.Bold
				}
			}
			if gap := min(start, right) - max(gapStart, t.left); gap > 0 {
				appendRun(strings.Repeat(" ", gap), fg, cellBG, underline, font)
			}
			if start >= right {
				break
			}
			appendRun(s, fg, cellBG, underline, font)
		}
		selectedEOL := offset+d.LineRuneCount(i) >= a && offset+d.LineRuneCount(i) < b
		if selectedEOL && visual >= t.left && visual < right {
			selectionBG := bg
			if m.colors.EditorSelection != "" {
				selectionBG = theme.Composite(m.colors.EditorSelection, bg)
			}
			fg := m.colors.Foreground
			if m.colors.EditorSelectionForeground != "" {
				fg = m.colors.EditorSelectionForeground
			}
			appendRun(" ", fg, selectionBG, "", 0)
		}
		flush()
		if foldedLines > 0 {
			text.WriteString(theme.TextStyle(m.colors.Accent, bg).Render(" … " + m.tr("%d lines", foldedLines)))
		}
		numberColor := m.colors.LineNumber
		if i == line {
			numberColor = m.colors.LineNumberActive
		}
		marker := " "
		if foldedLines > 0 {
			marker = ">"
			numberColor = m.colors.Accent
		}
		gutter := theme.TextStyle(numberColor, gutterBG).Render(fmt.Sprintf("%5d%s", i+1, marker))
		gutter += row("", 1, numberColor, gutterBG)
		rendered := gutter + row(text.String(), textWidth, m.colors.Foreground, bg)
		// Both segments already have a full background and the expected width.
		// Six-digit line numbers extend the gutter and still need clipping.
		if i >= 99999 || width < editorGutterWidth+1 {
			rendered = row(rendered, width, m.colors.Foreground, m.colors.Background)
		}
		rendered = guides.decorate(rendered, i, editorGutterWidth, t.left, rowBracket, m.colors)
		if rowKey.stickyBorder {
			rendered = underlineRow(rendered, theme.Composite(m.colors.StickyBorder, m.colors.StickyBackground), 0)
		}
		t.renderedRows[i] = editorRenderedRow{key: rowKey, text: rendered}
		lines = append(lines, rendered)
	}

	if currentRow >= 0 && m.colors.CurrentLineBorder != "" {
		border := theme.Composite(m.colors.CurrentLineBorder, m.colors.Background)
		lines[currentRow] = underlineRow(lines[currentRow], border, editorGutterWidth)
		// The previous text row's underline draws the top edge without moving content.
		if currentRow > len(sticky) {
			if _, image, _ := t.editorDisplayAt(top + currentRow - 1); image == nil {
				lines[currentRow-1] = underlineRow(lines[currentRow-1], border, editorGutterWidth)
			}
		}
	}
	return lines
}

func (m *Model) overlay(base []string) []string {
	if m.isPicker() {
		return m.pickerOverlay(base)
	}
	l := m.dialogLayout()
	w := l.width
	border := theme.Style(m.colors.PopupBorder, m.colors.Popup)
	box := []string{inputBorderRule(border, w, "╭", "╮")}
	for _, s := range l.content {
		box = append(box, border.Render("│")+s+border.Render("│"))
	}
	box = append(box, inputBorderRule(border, w, "╰", "╯"))
	return m.composeOverlay(base, box, l.x, l.y)
}

func (m *Model) click(msg tea.MouseClickMsg) tea.Cmd {
	if !m.editorOnly && msg.Y == tabRow && msg.X >= m.mainX() && msg.X < m.mainX()+m.mainWidth() && !(m.width < 70 && m.sidebar && m.sidebarVisible) && (msg.Button == tea.MouseLeft || msg.Button == tea.MouseMiddle) {
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
	if m.fileFindAt(msg.X, msg.Y) {
		return m.clickFileFind(msg.X-m.mainX(), msg.Y-m.contentRow())
	}
	if msg.Y == m.contentRow()-1 && !m.editing() && m.diffOpen {
		button := m.diffModeButton()
		if msg.X >= m.mainX()+button.x && msg.X < m.mainX()+button.x+button.width {
			if m.mainWidth() < 68 {
				m.notice = m.tr("The editor needs at least 68 columns for side-by-side diff")
				return nil
			}
			m.split = !m.split
			m.renderDiff()
		}
		return nil
	}
	if msg.Y < 0 || msg.Y >= m.bodyHeight() {
		return nil
	}
	if cmd, handled := m.clickDiffButton(msg); handled {
		return cmd
	}
	m.focusEditor()
	m.blurFileFind()
	m.lastClickKey = ""
	if m.welcomeVisible() {
		return m.clickWelcome(msg)
	}
	if msg.Y >= m.contentRow() && msg.Y < m.bodyHeight() {
		if msg.Mod == 0 {
			if cmd, handled := m.clickMarkdownRemoteImage(msg.X-m.mainX(), msg.Y-m.contentRow()); handled {
				return cmd
			}
		}
		m.beginTextDrag(msg)
		if !m.editing() && m.diffOpen {
			i := m.viewport.YOffset() + msg.Y - m.contentRow()
			if i < len(m.diffRowHunks) && m.diffRowHunks[i] >= 0 {
				m.diffHunk = m.diffRowHunks[i]
			}
		}
	}
	return nil
}
