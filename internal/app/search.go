package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/ourongxing/elegit/internal/editor"
	searchservice "github.com/ourongxing/elegit/internal/search"
)

type searchState struct {
	query, replacement textinput.Model
	options            searchservice.Options
	result             searchservice.Result
	seq                int
	cancel             context.CancelFunc
	loading, replace   bool
	focus              int // 0 search, 1 replace, 2 results
	selected, top      int
	folded             map[string]bool
	err                string
}

const searchOptionWidth = 6

type searchOption struct {
	label, key, hint string
	enabled          *bool
}

func (s *searchState) controls() [4]searchOption {
	return [4]searchOption{
		{"Aa", "alt+c", "Match Case (Alt+C)", &s.options.Case},
		{"ab", "alt+w", "Match Whole Word (Alt+W)", &s.options.Word},
		{".*", "alt+r", "Use Regular Expression (Alt+R)", &s.options.Regex},
		{"ig", "alt+i", "Include Ignored Files (Alt+I)", &s.options.IncludeIgnored},
	}
}

type searchMsg struct {
	seq    int
	result searchservice.Result
	err    error
}
type searchRow struct{ file, match int }

func newSearch() searchState {
	q, r := textinput.New(), textinput.New()
	q.Prompt = " "
	q.Placeholder = "Search"
	q.CharLimit = 2000
	r.Prompt = " "
	r.Placeholder = "Replace"
	r.CharLimit = 2000
	return searchState{query: q, replacement: r, folded: map[string]bool{}}
}
func (m *Model) openSearch(replace bool) tea.Cmd {
	m.selectSection(search)
	m.search.replace = m.search.replace || replace
	m.search.focus = 0
	m.search.replacement.Blur()
	m.search.query.Focus()
	return m.runSearch()
}
func (m *Model) runSearch() tea.Cmd {
	s := &m.search
	if s.cancel != nil {
		s.cancel()
	}
	s.seq++
	s.options.Query = s.query.Value()
	s.result = searchservice.Result{}
	s.err = ""
	s.top = 0
	s.selected = 0
	s.loading = s.options.Query != ""
	if !s.loading {
		return nil
	}
	if _, err := s.options.Compile(); err != nil {
		s.err = err.Error()
		s.loading = false
		return nil
	}
	ctx, cancel := context.WithCancel(m.ctx)
	s.cancel = cancel
	seq, root, options := s.seq, m.repo.Root, s.options
	buffers := map[string]string{}
	for _, t := range m.tabs {
		if t.doc != nil {
			buffers[t.doc.Path] = t.doc.Text
		}
	}
	return func() tea.Msg {
		defer cancel()
		result, err := searchservice.Run(ctx, root, options, buffers)
		return searchMsg{seq, result, err}
	}
}
func (m *Model) searchRows() []searchRow {
	var rows []searchRow
	for i, f := range m.search.result.Files {
		rows = append(rows, searchRow{i, -1})
		if !m.search.folded[f.Path] {
			for j := range f.Matches {
				if j == 0 || f.Matches[j].Line != f.Matches[j-1].Line {
					rows = append(rows, searchRow{i, j})
				}
			}
		}
	}
	return rows
}
func (m *Model) searchResultY() int {
	if m.search.replace {
		return 7
	}
	return 6
}
func (m *Model) clampSearch() {
	s := &m.search
	n := len(m.searchRows())
	s.selected = min(max(0, s.selected), max(0, n-1))
	s.top = min(max(0, s.top), max(0, n-max(1, m.bodyHeight()-m.searchResultY())))
}
func (m *Model) searchLines(nav string, w int) []string {
	s := &m.search
	s.query.SetWidth(max(1, w-2))
	s.replacement.SetWidth(max(1, w-2))
	styles := m.input.Styles()
	s.query.SetStyles(styles)
	s.replacement.SetStyles(styles)
	field := func(input textinput.Model, index int) string {
		input.Blur()
		if m.sidebar && s.focus == index {
			input.Focus()
		}
		return input.View()
	}
	toggle := " > Search"
	if s.replace {
		toggle = " v Replace"
	}
	lines := []string{nav, toggle, field(s.query, 0)}
	opts := ""
	for _, control := range s.controls() {
		fg := m.colors.Muted
		if *control.enabled {
			fg = m.colors.Accent
		}
		opts += row(" ["+control.label+"]", searchOptionWidth, fg, m.colors.Panel)
	}
	lines = append(lines, opts)
	if s.replace {
		lines = append(lines, field(s.replacement, 1))
	}
	controls := " Enter: search"
	if s.replace {
		controls = " [Replace] [Replace All]"
	}
	lines = append(lines, controls)
	summary := fmt.Sprintf(" %d files · %d results", len(s.result.Files), s.result.Count)
	if s.loading {
		summary = " Searching…"
	} else if s.err != "" {
		summary = " " + s.err
	} else if s.options.Query == "" {
		summary = " Search workspace files"
	} else if s.result.Unreadable > 0 {
		summary = fmt.Sprintf(" %d results; %d unreadable files", s.result.Count, s.result.Unreadable)
	}
	lines = append(lines, summary)
	for i := range lines {
		lines[i] = row(lines[i], w, m.colors.Foreground, m.colors.Panel)
	}
	m.clampSearch()
	rows := m.searchRows()
	for len(lines) < m.bodyHeight() {
		index := s.top + len(lines) - m.searchResultY()
		text := row("", w, m.colors.Foreground, m.colors.Panel)
		if index >= 0 && index < len(rows) {
			text = m.searchResultRow(index, w, rows[index])
		}
		lines = append(lines, text)
	}
	return lines[:m.bodyHeight()]
}
func (m *Model) searchKey(msg tea.KeyPressMsg) tea.Cmd {
	s := &m.search
	k := msg.String()
	for _, control := range s.controls() {
		if k == control.key {
			*control.enabled = !*control.enabled
			return m.runSearch()
		}
	}
	switch k {
	case "delete":
		if s.focus == 2 {
			m.dismissSearchResult(s.selected)
			return nil
		}
		return m.searchInput(msg)
	case "ctrl+f":
		s.focus = 0
		return nil
	case "ctrl+h":
		s.replace = !s.replace
		if !s.replace && s.focus == 1 {
			s.focus = 0
		}
		return nil
	case "ctrl+enter":
		return m.replaceSearch(false)
	case "ctrl+alt+enter":
		return m.replaceSearch(true)
	case "tab", "shift+tab":
		step := 1
		if k == "shift+tab" {
			step = -1
		}
		s.focus = (s.focus + step + 3) % 3
		if !s.replace && s.focus == 1 {
			s.focus = (s.focus + step + 3) % 3
		}
		return nil
	case "esc":
		s.focus = 2
		return nil
	case "enter":
		if s.focus == 2 {
			return m.openSearchResult()
		}
		s.focus = 2
		return m.runSearch()
	case "down", "up":
		if s.focus != 2 {
			s.focus = 2
			return nil
		}
		step := 1
		if k == "up" {
			step = -1
		}
		s.selected += step
		m.clampSearch()
		h := max(1, m.bodyHeight()-m.searchResultY())
		if s.selected < s.top {
			s.top = s.selected
		}
		if s.selected >= s.top+h {
			s.top = s.selected - h + 1
		}
		return nil
	case "left", "right":
		if s.focus == 2 {
			rows := m.searchRows()
			if len(rows) > 0 {
				file := rows[s.selected].file
				s.folded[s.result.Files[file].Path] = k == "left"
				for i, r := range m.searchRows() {
					if r.file == file && r.match < 0 {
						s.selected = i
						break
					}
				}
				m.clampSearch()
			}
			return nil
		}
	}
	return m.searchInput(msg)
}
func (m *Model) searchInput(msg tea.Msg) tea.Cmd {
	s := &m.search
	if s.focus == 2 {
		return nil
	}
	if s.focus == 0 {
		before := s.query.Value()
		s.query.Focus()
		s.query, _ = s.query.Update(msg)
		if before != s.query.Value() {
			return m.runSearch()
		}
	} else {
		s.replacement.Focus()
		s.replacement, _ = s.replacement.Update(msg)
	}
	return nil
}
func (m *Model) clickSearch(x, y int) tea.Cmd {
	s := &m.search
	m.sidebar = true
	switch {
	case y == 1:
		s.replace = !s.replace
		if !s.replace && s.focus == 1 {
			s.focus = 0
		}
	case y == 2:
		s.focus = 0
	case y == 3:
		controls := s.controls()
		i := x / searchOptionWidth
		if x < 0 || i >= len(controls) {
			return nil
		}
		*controls[i].enabled = !*controls[i].enabled
		return m.runSearch()
	case s.replace && y == 4:
		s.focus = 1
	case y == m.searchResultY()-2:
		if s.replace {
			if x >= 1 && x < 10 {
				return m.replaceSearch(false)
			}
			if x >= 11 && x < 24 {
				return m.replaceSearch(true)
			}
			return nil
		}
		return m.runSearch()
	case y >= m.searchResultY():
		i := s.top + y - m.searchResultY()
		rows := m.searchRows()
		if i < len(rows) {
			if x == m.searchControlWidth()-1 && m.searchRowAction(i) {
				m.dismissSearchResult(i)
				return nil
			}
			s.selected = i
			s.focus = 2
			return m.openSearchResult()
		}
	}
	return nil
}
func (m *Model) openSearchResult() tea.Cmd {
	rows := m.searchRows()
	s := &m.search
	if len(rows) == 0 {
		return nil
	}
	m.clampSearch()
	r := rows[s.selected]
	f := s.result.Files[r.file]
	if r.match < 0 {
		s.folded[f.Path] = !s.folded[f.Path]
		m.clampSearch()
		return nil
	}
	if real, err := filepath.EvalSymlinks(f.Path); err != nil || real != f.Path {
		m.notice = "Search file path changed; search again"
		return m.runSearch()
	}
	if m.openDocument(f.Path, true) {
		d := m.document()
		if d.Text != f.Text {
			m.notice = "Search result changed; search again"
			return m.runSearch()
		}
		hit := f.Matches[r.match]
		d.SelectRange(utf8.RuneCountInString(f.Text[:hit.Start]), utf8.RuneCountInString(f.Text[:hit.End]))
		m.ensureEditor()
	}
	return nil
}

// Replace buffers only after validating every target. Each file gets one undo step.
func (m *Model) replaceSearch(all bool) tea.Cmd {
	s := &m.search
	if s.loading || s.err != "" || len(s.result.Files) == 0 {
		return nil
	}
	m.clampSearch()
	rows := m.searchRows()
	selected := rows[s.selected]
	if s.result.Unreadable > 0 && (all || selected.match < 0) {
		m.notice = "Search is incomplete; resolve unreadable files before bulk replacement"
		return nil
	}
	matcher, err := s.options.Compile()
	if err != nil {
		m.notice = err.Error()
		return nil
	}
	type edit struct {
		doc      *editor.Document
		text     string
		existing bool
	}
	var edits []edit
	count := 0
	for fi, f := range s.result.Files {
		if !all && fi != selected.file {
			continue
		}
		var d *editor.Document
		for _, t := range m.tabs {
			if t.doc != nil && t.doc.Path == f.Path {
				d = t.doc
				break
			}
		}
		existing := d != nil
		if d == nil {
			if real, err := filepath.EvalSymlinks(f.Path); err != nil || real != f.Path {
				m.notice = "Search file path changed; search again"
				return m.runSearch()
			}
			var err error
			d, err = editor.Open(f.Path)
			if err != nil {
				m.notice = err.Error()
				return nil
			}
		}
		if d.Text != f.Text {
			m.notice = "Search result changed; search again before replacing"
			return m.runSearch()
		}
		matches := matcher.Find(f.Text, s.replacement.Value())
		targets := f.Matches
		if !all && selected.match >= 0 {
			targets = targets[selected.match : selected.match+1]
		}
		retained := matches[:0]
		next := 0
		for _, match := range matches {
			if next < len(targets) && match.Start == targets[next].Start && match.End == targets[next].End {
				retained = append(retained, match)
				next++
			}
		}
		if next != len(targets) {
			return m.runSearch()
		}
		matches = retained
		var b strings.Builder
		pos := 0
		for _, hit := range matches {
			b.WriteString(f.Text[pos:hit.Start])
			b.WriteString(hit.Replacement)
			pos = hit.End
			count++
		}
		b.WriteString(f.Text[pos:])
		if b.Len() > editor.MaxFileSize {
			m.notice = "Replacement exceeds editor file size limit"
			return nil
		}
		edits = append(edits, edit{d, b.String(), existing})
	}
	for _, e := range edits {
		e.doc.Edit(e.text)
		e.doc.SelectRange(0, 0)
		if !e.existing {
			m.tabs = append(m.tabs, tab{doc: e.doc})
		}
	}
	if len(edits) > 0 {
		for i := range m.tabs {
			if m.tabs[i].doc == edits[0].doc {
				m.activateTab(i)
				break
			}
		}
	}
	m.keepEditedTabs()
	m.syncWatch()
	m.sidebar = true
	m.notice = fmt.Sprintf("Replaced %d matches; save edited files with Ctrl+S (Ctrl+Z to undo)", count)
	return m.runSearch()
}
