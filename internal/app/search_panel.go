package app

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

type searchPanelGeometry struct {
	query, replacement, summary, results int
	details, include, exclude            int
}

func (m *Model) searchPanelLayout() searchPanelGeometry {
	l := searchPanelGeometry{query: 1, replacement: -1, summary: 3, results: 4}
	if m.search.replace {
		l.replacement = 3
		l.summary = 5
		l.results = 6
	}
	l.details = l.summary - 1
	l.include, l.exclude = -1, -1
	if m.search.details {
		l.include = l.details + 1
		l.exclude = l.include + 2
		l.summary = l.exclude + 1
		l.results = l.summary + 1
	}
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
	label := m.treeChevron(m.search.replace)
	toggleY := l.query
	if m.search.replace {
		toggleY = l.query + 1
	}
	controls = append(controls, searchPanelControl{id: "replaceToggle", label: label, hint: "Toggle Replace (Alt+H)", x: 1, y: toggleY, width: 1, quiet: true})
	add := func(y int, ids, labels, hints []string) {
		right := w - 2
		for index := len(ids) - 1; index >= 0; index-- {
			id := ids[index]
			width := max(2, ansi.StringWidth(labels[index]))
			x := right - width
			if x < 8 {
				break
			}
			disabled := (id == "replace" || id == "replaceAll") && (m.search.loading || m.search.err != "" || m.search.result.Count == 0)
			controls = append(controls, searchPanelControl{id: id, label: labels[index], hint: hints[index], x: x, y: y, width: width, disabled: disabled, embedded: id == "case" || id == "word" || id == "regex" || id == "preserve"})
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
		add(l.include, []string{"changed", "open"}, []string{"Changed", "Tab"}, []string{"Only Changed Files", "Only Open Tabs"})
		add(l.exclude, []string{"ignore"}, []string{"Ignore rules"}, []string{"Respect .gitignore and .ignore"})
		for index := range controls {
			if controls[index].id == "changed" || controls[index].id == "open" || controls[index].id == "ignore" {
				controls[index].embedded = true
			}
		}
	}
	return controls
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
func (m *Model) searchLines(nav string, w int) []string {
	s := &m.search
	l := m.searchPanelLayout()
	lines := make([]string, max(m.bodyHeight(), l.results))
	lines[0] = nav
	indent := strings.Repeat(" ", searchFieldInset)
	controls := m.searchPanelControls(w)
	field := func(input *textinput.Model, focus, y int, placeholder, label string) {
		end := w
		for _, control := range controls {
			if control.y == y && control.id != "replaceToggle" {
				end = min(end, control.x)
			}
		}
		boxEnd := w - 1
		if focus == 1 {
			boxEnd = w - 5
		}
		lines[y] = m.searchFieldLine(input, m.sidebar && s.focus == focus, end, boxEnd, placeholder, label, m.colors.Panel)
	}
	field(&s.query, 0, l.query, searchText("Search in files"), "")
	if s.replace {
		field(&s.replacement, 1, l.replacement, "Replace", "")
	}
	if s.details {
		field(&s.include, 3, l.include, "", "In")
		field(&s.exclude, 4, l.exclude, "", "Ex")
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
		if ansi.StringWidth(filtered) <= w-searchFieldInset-1 {
			summary = filtered
		}
	}
	lines[l.summary] = row(indent+ansi.Truncate(strings.TrimSpace(safe(summary)), max(0, w-searchFieldInset-1), "…"), w, m.colors.Muted, m.colors.Panel)
	for i := range lines {
		lines[i] = row(lines[i], w, m.colors.SideForeground, m.colors.Panel)
	}
	m.paintSearchControls(lines, w, controls, m.sideX(), 0, m.colors.Panel)
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
	for _, c := range m.searchPanelControls(m.searchControlWidth()) {
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
	case l.query:
		return m.focusSearch(0)
	case l.replacement:
		if y >= 0 {
			return m.focusSearch(1)
		}

	}
	if y >= l.results {
		i := m.search.top + y - l.results
		rows := m.searchRows()
		if i >= 0 && i < len(rows) {
			if x == m.searchControlWidth()-1 && m.searchRowAction(i) {
				m.dismissSearchResult(i)
				return nil
			}
			m.search.selected = i
			m.focusSearch(2)
			return m.openSearchResultFile(rows[i].match < 0 && x >= 3)
		}
	}
	return nil
}
func (m *Model) searchHoverHint(x, y int) string {
	for _, c := range m.searchPanelControls(m.searchControlWidth()) {
		if y == c.y && x >= c.x && x < c.x+c.width {
			return c.hint
		}
	}
	if m.searchHoveredRow() >= 0 && x == m.searchControlWidth()-1 {
		return searchText("Remove from Search Results (Delete)")
	}
	return ""
}
