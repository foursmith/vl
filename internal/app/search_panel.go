package app

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/vl/internal/theme"
)

const sidebarSearchFieldInset = 2

type searchPanelGeometry struct {
	query, replacement, summary, results int
	details, include, exclude            int
}

func (m *Model) searchPanelLayout() searchPanelGeometry {
	l := searchPanelGeometry{query: headerHeight + 1, replacement: -1, include: -1, exclude: -1}
	l.details = l.query + 2
	if m.search.replace {
		l.replacement = l.query + 2
		l.details = l.replacement + 2
	}
	l.summary = l.details + 1
	if m.search.details {
		l.include = l.details + 2
		l.exclude = l.include + 2
		l.summary = l.exclude + 2
	}
	l.results = l.summary + 1
	return l
}

type searchPanelControl struct {
	id, label, hint string
	x, y, width     int
	disabled        bool
	embedded        bool
	quiet           bool
}

func (m *Model) searchPanelControls(w int) []searchPanelControl {
	l := m.searchPanelLayout()
	detailsIcon := m.symbol(symbolSearchDetails)
	detailsWidth := max(2, ansi.StringWidth(detailsIcon))
	controls := []searchPanelControl{{id: "details", label: detailsIcon, hint: "Toggle Search Details (Ctrl+Shift+J)", x: max(1, w-2-detailsWidth), y: l.details, width: detailsWidth, quiet: true}}
	label := " " + m.treeChevron(m.search.replace)
	toggleY := l.query
	if m.search.replace {
		toggleY++
	}
	controls = append(controls, searchPanelControl{id: "replaceToggle", label: label, hint: "Toggle Replace (Alt+H)", x: 0, y: toggleY, width: sidebarSearchFieldInset, embedded: true, quiet: true})
	add := func(y int, ids, labels, hints []string) {
		right := w - 2
		for index := len(ids) - 1; index >= 0; index-- {
			id := ids[index]
			width := max(2, ansi.StringWidth(labels[index]))
			x := right - width
			if x < 8 {
				break
			}
			disabled := id == "replaceAll" && (m.search.loading || m.search.err != "" || m.search.result.Count == 0)
			controls = append(controls, searchPanelControl{id: id, label: labels[index], hint: hints[index], x: x, y: y, width: width, disabled: disabled, embedded: true})
			right = x - 1
			if id == "replaceAll" {
				right--
			}
		}
	}
	add(l.query, []string{"case", "word", "regex"}, []string{"Aa", "ab", ".*"}, []string{"Match Case", "Match Whole Word", "Use Regular Expression"})
	if m.search.replace {
		add(l.replacement, []string{"preserve", "replaceAll"}, []string{"AB", "↵"}, []string{"Preserve Case", "Replace All (Ctrl+Alt+Enter)"})
	}
	if m.search.details {
		changed := "Changed"
		if m.nerd {
			changed = m.symbol(symbolDiff)
		}
		add(l.include, []string{"changed", "open"}, []string{changed, m.symbol(symbolOpenTabs)}, []string{"Only Changed Files", "Only Open Tabs"})
		add(l.exclude, []string{"ignore"}, []string{m.symbol(symbolIgnoreRules)}, []string{"Respect .gitignore and .ignore"})
	}
	return slices.DeleteFunc(controls, func(c searchPanelControl) bool { return c.y >= m.bodyHeight() })
}
func (m *Model) toggleSearchDetails() tea.Cmd {
	m.search.details = !m.search.details
	if !m.search.details && m.search.focus > 2 {
		m.focusSearch(0)
	}
	m.clampSearch()
	return nil
}
func (m *Model) searchControlAction(id string) tea.Cmd {
	s := &m.search
	switch id {
	case "replaceToggle":
		return m.toggleSearchReplace()
	case "details":
		return m.toggleSearchDetails()
	case "case":
		s.options.Case = !s.options.Case
	case "word":
		s.options.Word = !s.options.Word
	case "regex":
		s.options.Regex = !s.options.Regex
	case "preserve":
		s.options.PreserveCase = !s.options.PreserveCase
		return nil
	case "open":
		s.options.OnlyOpen = !s.options.OnlyOpen
	case "changed":
		if !m.repo.HasRepository() {
			return nil
		}
		s.options.ChangedOnly = !s.options.ChangedOnly
	case "ignore":
		s.options.IncludeIgnored = !s.options.IncludeIgnored
	case "replaceAll":
		return m.replaceSearch(true)
	}
	if m.fileFindHeight() > 0 && !m.sidebar {
		m.adoptSearchState()
		m.findWidget.anchor = m.editorDocument().Cursor
		m.navigateFileFind(0)
		return m.refreshWorkspaceSearch()
	}
	return m.runSearch()
}
func (m *Model) searchLines(nav []string, w int) []string {
	s := &m.search
	l := m.searchPanelLayout()
	lines := make([]string, max(m.bodyHeight(), l.results))
	copy(lines, nav)
	indent := strings.Repeat(" ", sidebarSearchFieldInset)
	controls := m.searchPanelControls(w)
	field := func(input *textinput.Model, focus, y int, placeholder, label string) {
		end := w
		for _, control := range controls {
			if control.y == y && control.id != "replaceToggle" {
				end = min(end, control.x)
			}
		}
		lines[y] = m.searchFieldLine(input, m.sidebar && s.focus == focus, end, w-1, placeholder, label, m.sideX(), y)
	}
	field(&s.query, 0, l.query, searchText("Search in files"), "")
	if s.replace {
		field(&s.replacement, 1, l.replacement, "Replace", "")
	}
	if s.details {
		field(&s.include, 3, l.include, "", " ⊕")
		field(&s.exclude, 4, l.exclude, "", " ⊖")
	}
	summary := searchText("%d results in %d files", s.result.Count, len(s.result.Files))
	switch {
	case s.loading:
		summary = searchText("Searching…")
	case s.err != "":
		summary = s.err
	case s.options.Query == "":
		summary = ""
	case s.result.Count == 0:
		summary = searchText("No results found")
	case s.result.Unreadable > 0:
		summary = searchText("%d results · %d unreadable files", s.result.Count, s.result.Unreadable)
	}
	if summary != "" && !s.details && (s.options.Include != "" || s.options.Exclude != "" || s.options.OnlyOpen || s.options.ChangedOnly || s.options.IncludeIgnored) {
		filtered := summary + " · Filters"
		if ansi.StringWidth(filtered) <= w-sidebarSearchFieldInset-1 {
			summary = filtered
		}
	}
	lines[l.summary] = row(indent+ansi.Truncate(strings.TrimSpace(safe(summary)), max(0, w-sidebarSearchFieldInset-1), "…"), w, theme.Readable(m.colors.Muted, m.colors.Panel), m.colors.Panel)
	m.paintSearchControls(lines, w, controls, m.sideX(), 0, m.colors.Panel)
	searchBox := lines[headerHeight:l.details]
	m.frameSearchBox(searchBox, w-1, len(searchBox), "")
	if s.details {
		filterBox := lines[l.details+1 : l.summary]
		m.frameSearchBox(filterBox, w-1, len(filterBox), "")
	}
	for i := range lines {
		lines[i] = row(lines[i], w, m.colors.SideForeground, m.colors.Panel)
	}
	m.clampSearch()
	rows := m.searchRows()
	for y := l.results; y < m.bodyHeight(); y++ {
		index := s.top + y - l.results
		if index >= 0 && index < len(rows) {
			lines[y] = m.searchResultRow(index, w, rows[index])
		}
	}
	return lines[:m.bodyHeight()]
}
func (m *Model) clickSearch(x, y int) tea.Cmd {
	m.sidebar = true
	l := m.searchPanelLayout()
	for _, c := range m.searchPanelControls(m.sidebarContentWidth()) {
		if y == c.y && x >= c.x && x < c.x+c.width {
			if c.disabled {
				return nil
			}
			return m.searchControlAction(c.id)
		}
	}
	switch y {
	case l.include:
		if y >= 0 {
			return m.focusSearch(3)
		}
	case l.exclude:
		if y >= 0 {
			return m.focusSearch(4)
		}
	case l.replacement:
		if y >= 0 {
			return m.focusSearch(1)
		}
	}
	if y >= headerHeight && y < l.details {
		return m.focusSearch(0)
	}
	index := m.sidebarRowAt(y)
	if index < 0 {
		return nil
	}
	if x == m.sidebarContentWidth()-1 && m.searchRowAction(index) {
		m.lastClickKey = ""
		m.dismissSearchResult(index)
		return nil
	}
	if m.selectSidebarRow(index) == nil {
		return nil
	}
	if row, ok := m.selectedSearchRow(); ok && row.match < 0 && x < m.searchResultChevronX()+2 {
		m.lastClickKey = ""
		path := m.search.result.Files[row.file].Path
		m.search.folded[path] = !m.search.folded[path]
		m.clampSearch()
		return nil
	}
	return m.clickSidebarEntry()
}
func (m *Model) searchHoverHint(x, y int) string {
	for _, c := range m.searchPanelControls(m.sidebarContentWidth()) {
		if y == c.y && x >= c.x && x < c.x+c.width {
			return c.hint
		}
	}
	if m.hoveredSidebarRow() >= 0 && x == m.sidebarContentWidth()-1 {
		return searchText("Remove from Search Results (Delete)")
	}
	return ""
}
