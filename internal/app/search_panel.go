package app

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/theme"
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
	controls := []searchPanelControl{{id: "details", label: detailsIcon, hint: "Toggle Search Details", x: max(1, w-2-detailsWidth), y: l.details, width: detailsWidth, quiet: true}}
	label := " " + m.treeChevron(m.search.replace)
	toggleY := l.query
	if m.search.replace {
		toggleY++
	}
	controls = append(controls, searchPanelControl{id: "replaceToggle", label: label, hint: "Toggle Replace", x: 0, y: toggleY, width: sidebarSearchFieldInset, embedded: true, quiet: true})
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
		add(l.replacement, []string{"preserve", "replaceAll"}, []string{"AB", "↵"}, []string{"Preserve Case", "Replace All"})
	}
	if m.search.details {
		changed := "Changed"
		if m.nerd {
			changed = m.symbol(symbolDiff)
		}
		add(l.include, []string{"changed", "open"}, []string{changed, m.symbol(symbolOpenTabs)}, []string{"Only Changed Files", "Only Open Tabs"})
		add(l.exclude, []string{"ignore"}, []string{m.symbol(symbolIgnoreRules)}, []string{"Respect ignore rules"})
	}
	return slices.DeleteFunc(controls, func(c searchPanelControl) bool { return c.y >= m.bodyHeight() })
}
func (m *Model) toggleSearchDetails() {
	m.search.details = !m.search.details
	if !m.search.details && m.search.focus > 2 {
		m.focusSearch(0)
	}
	m.clampSearch()
}
func (m *Model) searchControlAction(id string) tea.Cmd {
	s := &m.search
	switch id {
	case "replaceToggle":
		m.setSearchReplace(!s.replace)
		return nil
	case "details":
		m.toggleSearchDetails()
		return nil
	case "replaceAll":
		return m.replaceAllSearchResults()
	}
	value := searchOption(&s.options, id)
	if value == nil || id == "changed" && !m.repo.HasRepository() {
		return nil
	}
	*value = !*value
	if id == "preserve" {
		return nil
	}
	return m.runSearch()
}
func searchFieldEnd(controls []searchPanelControl, y, end int) int {
	for _, c := range controls {
		if c.y == y && c.id != "replaceToggle" && c.id != "replacement" {
			end = min(end, c.x)
		}
	}
	return end
}

func (m *Model) searchLines(nav []string, w int) []string {
	s := &m.search
	l := m.searchPanelLayout()
	lines := make([]string, max(m.bodyHeight(), l.results))
	copy(lines, nav)
	indent := strings.Repeat(" ", sidebarSearchFieldInset)
	controls := m.searchPanelControls(w)
	field := func(input *textInput, focus, y int, placeholder, label string) {
		end := searchFieldEnd(controls, y, w)
		lines[y] = m.searchFieldLine(input, m.sidebar && s.focus == focus, end, w-1, placeholder, label, m.sideX(), y)
	}
	field(&s.query, 0, l.query, searchText("Search in files"), "")
	if s.replace {
		field(&s.replacement, 1, l.replacement, "Replace", "")
	}
	if s.details {
		field(&s.include, 3, l.include, "Include paths", " +")
		field(&s.exclude, 4, l.exclude, "Exclude paths", " -")
	}
	summary := searchText("%d results in %d files", s.result.Count, len(s.result.Files))
	switch {
	case s.loading:
		summary = searchText("Searching…")
		if s.result.Count > 0 {
			summary = searchText("%d results · Searching…", s.result.Count)
		}
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
	m.paintSearchControls(lines, w, controls, m.sideX(), 0, &s.options)
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
	w := m.sidebarContentWidth()
	controls := m.searchPanelControls(w)
	for _, c := range controls {
		if y == c.y && x >= c.x && x < c.x+c.width {
			if c.disabled {
				return nil
			}
			return m.searchControlAction(c.id)
		}
	}
	focus := -1
	if y >= 0 {
		switch y {
		case l.query:
			focus = 0
		case l.replacement:
			focus = 1
		case l.include:
			focus = 3
		case l.exclude:
			focus = 4
		}
	}
	if focus >= 0 {
		m.focusSearch(focus)
		area := searchInputArea(m.sideX(), y, searchFieldEnd(controls, y, w), w-1)
		m.clickInput(m.searchField(focus), area, m.sideX()+x, y)
		return nil
	}
	if y >= headerHeight && y < l.details {
		m.focusSearch(0)
		return nil
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
		m.search.setFolded(path, !m.search.folded[path])
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
