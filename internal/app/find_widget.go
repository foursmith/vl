package app

import (
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/vl/internal/editor"
	searchservice "github.com/ourongxing/vl/internal/search"
	"github.com/ourongxing/vl/internal/theme"
)

type findWidgetState struct {
	visible, focused bool
	workspaceMatch   bool
	anchor           int
}

const (
	fileFindQueryRow = 1
	fileReplaceRow   = fileFindQueryRow + 2
)

func (m *Model) fileFindHeight() int {
	if !m.findWidget.visible || m.editorDocument() == nil || m.width < 70 && m.sidebar && m.sidebarVisible {
		return 0
	}
	return min(m.fileFindFrameHeight(), max(0, m.bodyHeight()-m.contentRow()))
}
func (m *Model) fileFindFrameHeight() int {
	if m.search.replace && m.editing() {
		return fileReplaceRow + 2
	}
	return fileFindQueryRow + 2
}
func (m *Model) fileFindFocused() bool {
	return m.findWidget.focused && !m.sidebar && m.fileFindHeight() > 0
}
func (m *Model) adoptSearchState() {
	s := &m.search
	options := searchservice.Options{Query: s.query.Value(), Case: s.options.Case, Word: s.options.Word, Regex: s.options.Regex}
	if m.findOptions != options {
		m.findHidden = false
	}
	m.findOptions = options
}
func (m *Model) presentFileFind() {
	if m.editorDocument() == nil {
		return
	}
	m.adoptSearchState()
	m.findWidget.workspaceMatch = false
	m.findHidden = false
	m.findWidget.visible = true
}
func (m *Model) openFileFind() tea.Cmd {
	if m.editorDocument() == nil {
		return nil
	}
	m.presentFileFind()
	m.findWidget.anchor = m.editorDocument().Cursor + 1
	m.tabs[m.activeTab].vim.CancelPending()
	return m.focusFileFind(0)
}
func (m *Model) openFileReplace() tea.Cmd {
	if !m.editing() {
		return nil
	}
	m.presentFileFind()
	m.search.replace = true
	m.findWidget.anchor = m.document().Cursor
	return m.focusFileFind(1)
}
func (m *Model) focusFileFind(focus int) tea.Cmd {
	m.focusEditor()
	m.findWidget.focused = true
	return m.focusSearch(focus)
}
func (m *Model) blurFileFind() {
	m.findWidget.focused = false
	m.search.query.Blur()
	m.search.replacement.Blur()
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
	options := m.findOptions
	cache := &m.tabs[m.activeTab].findRanges
	if cache.text != d.Text || cache.options != options {
		*cache = findRangeCache{text: d.Text, options: options}
		matcher, err := options.Compile()
		if err != nil {
			cache.err = err.Error()
		} else if m.findOptions.Query != "" {
			cache.ranges = matchedSearchRanges(d.Text, m.findOptions.Query, matcher)
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

// All entry points use these same cached matches, selection and centering rules.
func (m *Model) navigateFileFind(direction int) {
	d := m.editorDocument()
	if d == nil || m.findOptions.Query == "" {
		return
	}
	cache := m.fileFindCache()
	matches := cache.ranges
	if cache.err != "" {
		m.notice = cache.err
		return
	}
	if len(matches) == 0 {
		m.notice = searchText("No match")
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
	d.SelectRange(hit[1], hit[0])
	m.focusEditor()
	m.findHidden = false
	m.centerTextCursor()
}
func (m *Model) fileFindInput(msg tea.Msg) tea.Cmd {
	field := m.searchField(m.search.focus)
	if field == nil {
		return nil
	}
	before := field.Value()
	var cmd tea.Cmd
	*field, cmd = field.Update(msg)
	if before == field.Value() {
		return cmd
	}
	m.adoptSearchState()
	m.notice = ""
	if m.search.focus == 0 {
		m.editorDocument().ClearSelection()
		m.navigateFileFind(0)
	}
	if m.search.focus == 0 {
		return tea.Batch(cmd, m.refreshWorkspaceSearch())
	}
	return cmd
}
func (m *Model) refreshWorkspaceSearch() tea.Cmd {
	m.search.workspaceDirty = true
	if !m.editorOnly && m.section == search && m.sidebarVisible {
		return m.scheduleSearch(150 * time.Millisecond)
	}
	return nil
}
func (m *Model) toggleFileReplacement() tea.Cmd {
	if !m.editing() {
		return nil
	}
	m.search.replace = !m.search.replace
	if m.search.replace {
		return m.focusFileFind(1)
	}
	return m.focusFileFind(0)
}
func (m *Model) fileFindKey(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	k := msg.String()
	switch k {
	case "esc":
		m.closeFileFind()
		return true, nil
	case "ctrl+f":
		return true, m.focusFileFind(0)
	case "ctrl+shift+j", "alt+e":
		return true, m.openSearchOptions()
	case "alt+h":
		return true, m.toggleFileReplacement()
	case "f3", "down":
		m.navigateFileFind(1)
		return true, nil
	case "shift+f3", "up":
		m.navigateFileFind(-1)
		return true, nil
	case "tab", "shift+tab":
		if m.search.replace && m.editing() {
			return true, m.focusFileFind(1 - m.search.focus)
		}
		m.blurFileFind()
		return true, nil
	case "ctrl+enter":
		return true, m.replaceFileFind(false)
	case "ctrl+alt+enter":
		return true, m.replaceFileFind(true)
	case "enter":
		if m.search.focus == 1 {
			return true, m.replaceFileFind(false)
		}
		changed := m.findOptions.Query != m.search.query.Value()
		m.adoptSearchState()
		if changed || m.fileFindIndex() < 0 {
			m.navigateFileFind(0)
		} else {
			m.centerTextCursor()
		}
		m.blurFileFind()
		return true, nil
	case "ctrl+q", "ctrl+p", "ctrl+shift+p", "f1", "ctrl+1", "ctrl+2", "ctrl+3", "ctrl+4", "ctrl+l", "ctrl+g", "f5", "f6", "ctrl+j", "alt+left", "alt+right", "ctrl+shift+f", "ctrl+shift+h":
		m.blurFileFind()
		return false, nil
	case "ctrl+s":
		return false, nil
	}
	return true, m.fileFindInput(msg)
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
	hints := []string{"Match Case", "Match Whole Word", "Use Regular Expression", "Previous Match (Shift+F3)", "Next Match (F3 / n)"}
	ids = append(ids, "close")
	labels = append(labels, "×")
	hints = append(hints, searchText("Close Find (Esc)"))
	add(fileFindQueryRow, w-1, ids, labels, hints)
	if m.editing() {
		toggleRow := fileFindQueryRow
		if m.search.replace {
			toggleRow++
		}
		controls = append(controls, searchPanelControl{id: "replacement", label: m.treeChevron(m.search.replace), hint: "Toggle Replace (Alt+H)", x: 1, y: toggleRow, width: 1, quiet: true})
	}
	if m.search.replace && m.editing() {
		right := w - 1
		for _, control := range controls {
			if control.y == fileFindQueryRow && !control.embedded {
				right = min(right, control.x+1)
			}
		}
		add(fileReplaceRow, right, []string{"preserve", "replaceAll"}, []string{"AB", "↵"}, []string{"Preserve Case", "Replace All in This File (Ctrl+Alt+Enter)"})
	}
	return slices.DeleteFunc(controls, func(c searchPanelControl) bool { return c.y >= m.fileFindHeight() })
}

const searchFieldInset = 3

func (m *Model) frameSearchBox(lines []string, width, height int, title string) {
	bg := m.colors.Popup
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

func (m *Model) searchFieldLine(input *textinput.Model, active bool, textEnd, boxEnd int, placeholder, label string, x, y int) string {
	const inset = searchFieldInset
	// Only the focused surface owns input geometry. The other surface paints a
	// copy, so sidebar and editor widths remain independent.
	shown := *input
	shown.SetWidth(max(1, min(textEnd, boxEnd)-inset-2))
	shown.Placeholder = placeholder
	shown.SetStyles(m.input.Styles())
	if active {
		*input = shown
	}
	if !active || m.modal != "" {
		shown.Blur()
	}
	text := ansi.Truncate(m.inputView(shown, x+inset, y), max(0, textEnd-inset-1), "")
	bg := m.colors.Popup
	return row(label, inset, theme.Readable(m.colors.Muted, bg), bg) + row(text, boxEnd-inset, m.colors.InputForeground, bg)
}
func (m *Model) paintSearchControls(lines []string, w int, controls []searchPanelControl, originX, originY int, background string) {
	for _, c := range controls {
		if c.y < 0 || c.y >= len(lines) {
			continue
		}
		fg := m.colors.Accent
		bg := background
		if c.embedded {
			fg, bg = m.colors.InputForeground, m.colors.Popup
			if c.id == "replaceAll" {
				fg = m.colors.Accent
			}
		}
		if c.quiet || c.disabled {
			fg = m.colors.Muted
		}
		hovered := m.modal == "" && m.hoverY == originY+c.y && m.hoverX >= originX+c.x && m.hoverX < originX+c.x+c.width
		if c.quiet && hovered {
			fg = m.colors.SideForeground
		}
		selected := c.id == "case" && m.search.options.Case || c.id == "word" && m.search.options.Word || c.id == "regex" && m.search.options.Regex || c.id == "preserve" && m.search.options.PreserveCase
		selected = selected || c.id == "changed" && m.search.options.ChangedOnly || c.id == "open" && m.search.options.OnlyOpen || c.id == "ignore" && !m.search.options.IncludeIgnored
		if selected {
			fg = m.colors.Accent
		}
		label := theme.TextStyle(fg, bg).Underline(c.id == "word").Reverse(selected && m.monochrome).Render(c.label)
		lines[c.y] = row(ansi.Cut(lines[c.y], 0, c.x), c.x, m.colors.SideForeground, background) + row(label, c.width, fg, bg) + ansi.Cut(lines[c.y], c.x+c.width, w)
	}
}
func (m *Model) fileFindStatus() string {
	cache := m.fileFindCache()
	title := ""
	if m.findOptions.Query != "" {
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
	field := func(input *textinput.Model, focus, line int, placeholder string) {
		if line >= h {
			return
		}
		end := w - 1
		for _, control := range controls {
			if control.y == line && control.id != "replacement" {
				end = min(end, control.x)
			}
		}
		lines[line] = m.searchFieldLine(input, m.fileFindFocused() && m.search.focus == focus, end, w, placeholder, "", m.fileFindX(), m.contentRow()+line)
	}
	field(&m.search.query, 0, fileFindQueryRow, "Find")
	if m.search.replace && m.editing() {
		field(&m.search.replacement, 1, fileReplaceRow, "Replace")
	}
	m.paintSearchControls(lines, w, controls, m.fileFindX(), m.contentRow(), m.colors.Popup)
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
	for _, c := range m.fileFindControls(m.fileFindWidth()) {
		if y == c.y && x >= c.x && x < c.x+c.width {
			if c.disabled {
				return nil
			}
			switch c.id {
			case "case", "word", "regex", "preserve":
				return m.searchControlAction(c.id)
			case "close":
				m.closeFileFind()
			case "previous":
				m.navigateFileFind(-1)
			case "next":
				m.navigateFileFind(1)
			case "replacement":
				return m.toggleFileReplacement()
			case "replaceAll":
				return m.replaceFileFind(true)
			}
			return nil
		}
	}
	if y == fileReplaceRow && m.search.replace && m.editing() {
		return m.focusFileFind(1)
	}
	if y >= 0 && y < m.fileFindHeight() {
		return m.focusFileFind(0)
	}
	return nil
}
func (m *Model) replaceFileFind(all bool) tea.Cmd {
	d := m.document()
	if d == nil || d.ReadOnly || m.findOptions.Query == "" {
		return nil
	}
	m.adoptSearchState()
	// Replacement expansion and case preservation come from the same matcher as workspace replacement.
	options := m.findOptions
	options.PreserveCase = m.search.options.PreserveCase
	matcher, err := options.Compile()
	if err != nil {
		return nil
	}
	hits := matcher.Find(d.Text, m.search.replacement.Value())
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
	m.notice = searchText("Replaced %d matches in this file", len(hits))
	return m.refreshWorkspaceSearch()
}
