package app

import (
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/editor"
	searchservice "github.com/foursmith/vl/internal/search"
	"github.com/foursmith/vl/internal/theme"
)

type findWidgetState struct {
	searchForm
	visible bool
	focus   int
	anchor  int
}

func (s *findWidgetState) matchOptions() searchservice.Options {
	return searchservice.Options{Query: s.query.Value(), Case: s.options.Case, Word: s.options.Word, Regex: s.options.Regex}
}

func (s *findWidgetState) field() *textInput {
	if s.focus == 1 {
		return &s.replacement
	}
	return &s.query
}

const (
	fileFindQueryRow = 1
	fileReplaceRow   = fileFindQueryRow + 2
)

func (m *Model) fileFindHeight() int {
	if !m.findWidget.visible || m.editorDocument() == nil || m.mainShowsSidebar() {
		return 0
	}
	return min(m.fileFindFrameHeight(), max(0, m.bodyHeight()-m.contentRow()))
}
func (m *Model) fileFindFrameHeight() int {
	if m.findWidget.replace && m.editing() {
		return fileReplaceRow + 2
	}
	return fileFindQueryRow + 2
}
func (m *Model) fileFindFocused() bool {
	return !m.sidebar && m.findWidget.field().Focused() && m.fileFindHeight() > 0
}
func (m *Model) presentFileFind() {
	if m.editorDocument() == nil {
		return
	}
	m.findHidden = false
	m.findWidget.visible = true
}
func (m *Model) openFileFind() {
	if m.editorDocument() == nil {
		return
	}
	m.presentFileFind()
	m.findWidget.anchor = m.editorDocument().Cursor + 1
	m.tabs[m.activeTab].vim.CancelPending()
	m.focusFileFind(0)
}
func (m *Model) openFileReplace() tea.Cmd {
	if !m.editing() {
		return nil
	}
	m.presentFileFind()
	m.findWidget.replace = true
	m.findWidget.anchor = m.document().Cursor
	m.focusFileFind(1)
	return nil
}
func (m *Model) focusFileFind(focus int) {
	m.focusEditor()
	m.navigationState = navigationState{}
	m.blurFileFind()
	m.findWidget.focus = focus
	m.findWidget.field().Focus()
}
func (m *Model) blurFileFind() {
	m.findWidget.query.Blur()
	m.findWidget.replacement.Blur()
}
func (m *Model) closeFileFind() {
	m.blurFileFind()
	m.findWidget.visible = false
	m.findHidden = true
}
func (m *Model) fileFindCache() *findRangeCache {
	d := m.editorDocument()
	if d == nil || m.activeTab < 0 {
		return &findRangeCache{}
	}
	options := m.findWidget.matchOptions()
	cache := &m.tabs[m.activeTab].findRanges
	if cache.text != d.Text || cache.options != options {
		*cache = findRangeCache{text: d.Text, options: options}
		matcher, err := options.Compile()
		if err != nil {
			cache.err = err.Error()
		} else if options.Query != "" {
			cache.ranges = matchedSearchRanges(d.Text, options.Query, matcher)
		}
	}
	return cache
}
func (m *Model) fileFindIndex() int {
	d := m.editorDocument()
	if d == nil {
		return -1
	}
	a, b := d.Selection()
	matches := m.fileFindCache().ranges
	start := sort.Search(len(matches), func(i int) bool { return matches[i][1] >= a })
	for i := start; i < len(matches) && matches[i][0] <= b; i++ {
		hit := matches[i]
		if hit[0] >= a && hit[1] <= b && (a != b || hit[0] == a) {
			return i
		}
	}
	return -1
}

// File Find navigates its cached matches independently of workspace search.
func (m *Model) navigateFileFind(direction int) {
	d := m.editorDocument()
	if d == nil || m.findWidget.query.Value() == "" {
		return
	}
	cache := m.fileFindCache()
	matches := cache.ranges
	if cache.err != "" || len(matches) == 0 {
		// The Find box already shows the error or match count while open.
		if m.fileFindHeight() == 0 && m.notice == "" {
			text := cache.err
			if text == "" {
				text = searchText("No match")
			}
			m.showStatusFeedback(text)
		}
		return
	}
	index := m.fileFindIndex()
	if direction == 0 {
		index = sort.Search(len(matches), func(i int) bool { return matches[i][0] >= m.findWidget.anchor })
		if index == len(matches) {
			index = 0
		}
	} else if index >= 0 {
		index = (index + direction + len(matches)) % len(matches)
	} else if direction < 0 {
		index = sort.Search(len(matches), func(i int) bool { return matches[i][0] >= d.Cursor }) - 1
		if index < 0 {
			index = len(matches) - 1
		}
	} else {
		index = sort.Search(len(matches), func(i int) bool { return matches[i][0] > d.Cursor })
		if index == len(matches) {
			index = 0
		}
	}
	hit := matches[index]
	m.notice = ""
	m.focusEditor()
	m.findHidden = false
	m.revealSearchMatch(d, hit[0], hit[1])
}
func (m *Model) fileFindInput(msg tea.Msg) tea.Cmd {
	field := m.findWidget.field()
	before := field.Value()
	cmd := field.update(msg)
	if before == field.Value() {
		return cmd
	}
	m.notice = ""
	if m.findWidget.focus == 0 {
		m.findHidden = false
		m.editorDocument().ClearSelection()
		m.navigateFileFind(0)
	}
	return cmd
}
func (m *Model) toggleFileReplacement() {
	if !m.editing() {
		return
	}
	m.findWidget.replace = !m.findWidget.replace
	if m.findWidget.replace {
		m.focusFileFind(1)
	} else {
		m.focusFileFind(0)
	}
}

func (m *Model) toggleFileSearchOption(id string) {
	d := m.editorDocument()
	if d == nil {
		return
	}
	state := &m.findWidget
	value := searchOption(&state.options, id)
	if value == nil {
		return
	}
	*value = !*value
	if id == "preserve" {
		return
	}
	m.findHidden = false
	state.anchor = d.Cursor
	m.navigateFileFind(0)
}

func (m *Model) fileFindKey(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(msg, dismissKey):
		m.closeFileFind()
		return nil
	case isHalfPageKey(msg.String()):
		m.scrollTextHalfPage(msg.String())
		return nil
	case key.Matches(msg, pickerKeys.Down):
		m.navigateFileFind(1)
		return nil
	case key.Matches(msg, pickerKeys.Up):
		m.navigateFileFind(-1)
		return nil
	case key.Matches(msg, searchKeys.NextField, searchKeys.PreviousField):
		if m.findWidget.replace && m.editing() {
			m.focusFileFind(1 - m.findWidget.focus)
			return nil
		}
		m.blurFileFind()
		return nil
	case key.Matches(msg, pickerKeys.Accept):
		if m.findWidget.focus == 1 {
			return m.replaceFileFind(false)
		}
		if m.fileFindIndex() < 0 {
			m.navigateFileFind(0)
		} else {
			m.centerTextCursor()
		}
		m.blurFileFind()
		return nil
	}
	return m.fileFindInput(msg)
}
func (m *Model) fileFindControls(w int) []searchPanelControl {
	var controls []searchPanelControl
	hasMatches := len(m.fileFindCache().ranges) > 0
	add := func(y, right int, ids, labels, hints []string) {
		x := right + 1
		for _, label := range labels {
			x -= ansi.StringWidth(label) + 1
		}
		x = max(8, x)
		for i, id := range ids {
			label := labels[i]
			width := ansi.StringWidth(label)
			if x+width > w-1 {
				continue
			}
			disabled := !hasMatches && (id == "previous" || id == "next" || id == "replaceAll")
			embedded := id == "case" || id == "word" || id == "regex" || id == "preserve"
			controlX := x
			if embedded {
				controlX--
			}
			controls = append(controls, searchPanelControl{id: id, label: label, hint: hints[i], x: controlX, y: y, width: width, disabled: disabled, embedded: embedded, quiet: !embedded})
			x += width + 1
		}
	}
	ids := []string{"case", "word", "regex", "previous", "next"}
	labels := []string{"Aa", "ab", ".*", "↑", "↓"}
	hints := []string{"Match Case", "Match Whole Word", "Use Regular Expression", "Previous Match", "Next Match"}
	ids = append(ids, "close")
	labels = append(labels, "×")
	hints = append(hints, shortcutHint("Close Find", dismissKey))
	add(fileFindQueryRow, w-1, ids, labels, hints)
	if m.editing() {
		toggleRow := fileFindQueryRow
		if m.findWidget.replace {
			toggleRow++
		}
		controls = append(controls, searchPanelControl{id: "replacement", label: m.treeChevron(m.findWidget.replace), hint: "Toggle Replace", x: 1, y: toggleRow, width: 1, quiet: true})
	}
	if m.findWidget.replace && m.editing() {
		right := w - 1
		for _, control := range controls {
			if control.y == fileFindQueryRow && !control.embedded {
				right = min(right, control.x+1)
			}
		}
		add(fileReplaceRow, right, []string{"preserve", "replaceAll"}, []string{"AB", "↵"}, []string{"Preserve Case", "Replace All in This File"})
	}
	return slices.DeleteFunc(controls, func(c searchPanelControl) bool { return c.y >= m.fileFindHeight() })
}

const searchFieldInset = 3

func (m *Model) frameSearchBox(lines []string, width, height int, title string) {
	bg := m.colors.Panel
	border := theme.Style(m.colors.PopupBorder, bg)
	if title = ansi.Truncate(title, max(0, width-6), "…"); title == "" {
		lines[0] = inputBorderRule(border, width, "╭", "╮")
	} else {
		gap := width - 5 - ansi.StringWidth(title)
		lines[0] = border.Render("╭"+strings.Repeat("─", gap)+" ") +
			theme.TextStyle(m.colors.Muted, bg).Render(title) + border.Render(" ─╮")
	}
	side := border.Render("│")
	for y := 1; y < len(lines); y++ {
		switch {
		case y == height-1:
			lines[y] = inputBorderRule(border, width, "╰", "╯")
		case height == 5 && y == 2:
			gutter := row(ansi.Cut(lines[y], 1, searchFieldInset), searchFieldInset-1, m.colors.InputForeground, bg)
			rule := strings.Repeat("─", max(0, width-searchFieldInset-1))
			lines[y] = side + gutter + border.Render(rule+"┤")
		default:
			lines[y] = side + ansi.Cut(lines[y], 1, width-1) + side
		}
	}
}

func searchInputArea(x, y, textEnd, boxEnd int) inputArea {
	return inputArea{x: x + searchFieldInset, y: y, width: max(1, min(textEnd, boxEnd)-searchFieldInset-2)}
}

func (m *Model) searchFieldLine(input *textInput, active bool, textEnd, boxEnd int, placeholder, label string, x, y int) string {
	const inset = searchFieldInset
	area := searchInputArea(x, y, textEnd, boxEnd)
	shown := *input
	shown.focused = input.Focused() && active && m.modal == ""
	shown.Placeholder = placeholder
	bg := m.colors.Panel
	shown.styles = m.input.styles
	if label != "" {
		shown.styles.placeholder = theme.TextStyle(theme.WithOpacity(m.colors.InputPlaceholder, .75), bg)
	}
	text := ansi.Truncate(m.inputView(shown, area), max(0, textEnd-inset-1), "")
	return row(label, inset, theme.Readable(m.colors.Muted, bg), bg) + row(text, boxEnd-inset, m.colors.InputForeground, bg)
}
func (m *Model) paintSearchControls(lines []string, w int, controls []searchPanelControl, originX, originY int, options *searchservice.Options) {
	background := m.colors.Panel
	for _, c := range controls {
		if c.y < 0 || c.y >= len(lines) {
			continue
		}
		hovered := m.modal == "" && m.hoverY == originY+c.y && m.hoverX >= originX+c.x && m.hoverX < originX+c.x+c.width
		selected := searchOptionSelected(options, c.id)
		lines[c.y] = row(ansi.Cut(lines[c.y], 0, c.x), c.x, m.colors.SideForeground, background) +
			m.searchControlView(c, background, selected, hovered) + ansi.Cut(lines[c.y], c.x+c.width, w)
	}
}

func (m *Model) searchControlView(c searchPanelControl, background string, selected, hovered bool) string {
	fg, bg := m.colors.Accent, background
	if c.embedded {
		fg, bg = m.colors.InputForeground, m.colors.Panel
		if c.id == "replaceAll" {
			fg = m.colors.Accent
		}
	}
	if c.quiet || c.disabled {
		fg = m.colors.Muted
	}
	if c.quiet && hovered {
		fg = m.colors.SideForeground
	}
	if selected {
		fg = m.colors.Accent
	}
	label := theme.TextStyle(fg, bg).Underline(c.id == "word").Reverse(selected && m.monochrome).Render(c.label)
	return row(label, c.width, fg, bg)
}
func (m *Model) fileFindStatus() string {
	cache := m.fileFindCache()
	title := ""
	if m.findWidget.query.Value() != "" {
		if index := m.fileFindIndex(); index >= 0 {
			title = searchText("%d of %d", index+1, len(cache.ranges))
		} else {
			title = searchText("%d matches", len(cache.ranges))
		}
	}
	if cache.err != "" {
		title = searchText("Invalid pattern")
	}
	return title
}
func (m *Model) fileFindLines() []string {
	h, w := m.fileFindHeight(), m.fileFindWidth()
	if h == 0 {
		return nil
	}
	lines := make([]string, h)
	controls := m.fileFindControls(w)
	field := func(input *textInput, focus, line int, placeholder string) {
		if line >= h {
			return
		}
		end := searchFieldEnd(controls, line, w-1)
		lines[line] = m.searchFieldLine(input, m.fileFindFocused() && m.findWidget.focus == focus, end, w, placeholder, "", m.fileFindX(), m.contentRow()+line)
	}
	field(&m.findWidget.query, 0, fileFindQueryRow, "Find")
	if m.findWidget.replace && m.editing() {
		field(&m.findWidget.replacement, 1, fileReplaceRow, "Replace")
	}
	m.paintSearchControls(lines, w, controls, m.fileFindX(), m.contentRow(), &m.findWidget.options)
	m.frameSearchBox(lines, w, m.fileFindFrameHeight(), m.fileFindStatus())
	return lines
}
func (m *Model) fileFindWidth() int { return min(m.mainWidth(), 64, max(30, m.mainWidth()-16)) }
func (m *Model) fileFindX() int     { return m.mainX() + m.mainWidth() - m.fileFindWidth() }
func (m *Model) fileFindAt(x, y int) bool {
	return x >= m.fileFindX() && x < m.mainX()+m.mainWidth() && y >= m.contentRow() && y < m.contentRow()+m.fileFindHeight()
}
func (m *Model) clickFileFind(x, y int) tea.Cmd {
	x -= m.mainWidth() - m.fileFindWidth()
	if x < 0 || x >= m.fileFindWidth() {
		m.blurFileFind()
		return nil
	}
	m.focusEditor()
	w := m.fileFindWidth()
	controls := m.fileFindControls(w)
	for _, c := range controls {
		if y == c.y && x >= c.x && x < c.x+c.width {
			if c.disabled {
				return nil
			}
			switch c.id {
			case "case", "word", "regex", "preserve":
				m.toggleFileSearchOption(c.id)
			case "close":
				m.closeFileFind()
			case "previous":
				m.navigateFileFind(-1)
			case "next":
				m.navigateFileFind(1)
			case "replacement":
				m.toggleFileReplacement()
				return nil
			case "replaceAll":
				return m.replaceFileFind(true)
			}
			return nil
		}
	}
	focus := 0
	if y == fileReplaceRow && m.findWidget.replace && m.editing() {
		focus = 1
	}
	if y >= 0 && y < m.fileFindHeight() {
		m.focusFileFind(focus)
		if y == fileFindQueryRow || focus == 1 {
			origin := m.fileFindX()
			area := searchInputArea(origin, m.contentRow()+y, searchFieldEnd(controls, y, w-1), w)
			m.clickInput(m.findWidget.field(), area, origin+x, area.y)
		}
	}
	return nil
}
func (m *Model) replaceFileFind(all bool) tea.Cmd {
	d := m.document()
	if d == nil || d.ReadOnly || m.findWidget.query.Value() == "" {
		return nil
	}
	// Replacement expansion and case preservation come from the same matcher as workspace replacement.
	options := m.findWidget.matchOptions()
	options.PreserveCase = m.findWidget.options.PreserveCase
	matcher, err := options.Compile()
	if err != nil {
		return nil
	}
	hits := matcher.Find(d.Text, m.findWidget.replacement.Value())
	if len(hits) == 0 {
		return nil
	}
	if !all {
		index := m.fileFindIndex()
		if index < 0 {
			m.navigateFileFind(0)
			index = m.fileFindIndex()
		}
		if index < 0 || index >= len(hits) {
			return nil
		}
		hits = hits[index : index+1]
	}
	text := searchservice.ReplaceText(d.Text, hits)
	if len(text) > editor.MaxFileSize {
		m.notice = searchText("Replacement exceeds editor file size limit")
		return nil
	}
	offset := utf8.RuneCountInString(d.Text[:hits[0].Start]) + utf8.RuneCountInString(hits[0].Replacement)
	m.tabs[m.activeTab].vim.Reset(d)
	d.Edit(text)
	d.SelectRange(offset, offset)
	m.findWidget.anchor = d.Cursor
	m.navigateFileFind(0)
	m.keepEditedTabs()
	m.showStatusFeedback(searchText("Replaced %d matches in this file", len(hits)))
	return m.refreshWorkspaceSearch()
}
