package app

import (
	"context"
	"path/filepath"
	"time"
	"unicode/utf8"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	searchservice "github.com/ourongxing/vl/internal/search"
)

type searchState struct {
	query, replacement textinput.Model
	include, exclude   textinput.Model
	replace            bool
	details            bool
	options            searchservice.Options
	result             searchservice.Result
	seq                int
	cancel             context.CancelFunc
	loading            bool
	workspaceDirty     bool
	focus              int // 0 query, 1 replacement, 2 results, 3 include, 4 exclude
	selected, top      int
	folded             map[string]bool
	err                string
}
type searchMsg struct {
	seq    int
	result searchservice.Result
	err    error
}
type searchRow struct{ file, match int }

func newSearch() searchState {
	field := func() textinput.Model {
		input := newInput()
		input.Prompt = ""
		return input
	}
	return searchState{query: field(), replacement: field(), include: field(), exclude: field(), folded: map[string]bool{}}
}
func (m *Model) openSearch() tea.Cmd {
	m.notice = ""
	if m.findWidget.visible {
		m.closeFileFind()
	}
	m.findWidget.focused = false
	m.selectSection(search)
	m.navigationState = navigationState{}
	cmd := m.focusSearch(0)
	if m.search.workspaceDirty {
		return tea.Batch(cmd, m.runSearch())
	}
	return cmd
}
func (m *Model) focusSearch(focus int) tea.Cmd {
	m.navigationState = navigationState{}
	m.search.focus = focus
	for _, field := range []*textinput.Model{&m.search.query, &m.search.replacement, &m.search.include, &m.search.exclude} {
		field.Blur()
	}
	if field := m.searchField(focus); field != nil {
		return field.Focus()
	}
	return nil
}
func (m *Model) searchField(focus int) *textinput.Model {
	switch focus {
	case 0:
		return &m.search.query
	case 1:
		return &m.search.replacement
	case 3:
		return &m.search.include
	case 4:
		return &m.search.exclude
	}
	return nil
}

func (m *Model) openReplace() tea.Cmd {
	m.notice = ""
	m.selectSection(search)
	m.search.replace = true
	m.resize()
	m.clampSearch()
	return m.focusSearch(1)
}
func (m *Model) toggleSearchReplace() tea.Cmd {
	m.search.replace = !m.search.replace
	m.resize()
	m.clampSearch()
	if m.search.replace {
		return m.focusSearch(1)
	}
	return m.focusSearch(0)
}
func (m *Model) runSearch() tea.Cmd { return m.scheduleSearch(0) }
func (m *Model) scheduleSearch(delay time.Duration) tea.Cmd {
	s := &m.search
	s.workspaceDirty = false
	if s.cancel != nil {
		s.cancel()
	}
	s.seq++
	m.adoptSearchState()
	s.options.Query = s.query.Value()
	s.options.Include = s.include.Value()
	s.options.Exclude = s.exclude.Value()
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
		if delay > 0 {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return nil
			case <-timer.C:
			}
		}
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
func (m *Model) searchResultY() int { return m.searchPanelLayout().results }
func (m *Model) clampSearch() {
	m.clampSearchList(m.searchList())
}

func (m *Model) clampSearchList(list sidebarList) {
	m.search.selected = min(max(0, m.search.selected), max(0, list.count-1))
	list.clampScroll()
}

func (m *Model) selectedSearchRow() (searchRow, bool) {
	if !m.search.loading {
		rows := m.searchRows()
		if index := m.search.selected; index >= 0 && index < len(rows) {
			return rows[index], true
		}
	}
	return searchRow{}, false
}

func (m *Model) searchKey(msg tea.KeyPressMsg) tea.Cmd {
	s := &m.search
	k := msg.String()
	switch k {
	case "ctrl+shift+j", "alt+e":
		return m.toggleSearchDetails()
	case "alt+h":
		return m.toggleSearchReplace()
	case "ctrl+enter":
		if s.replace {
			return m.replaceSearch(false)
		}
		return nil
	case "ctrl+alt+enter":
		if s.replace {
			return m.replaceSearch(true)
		}
		return nil
	case "ctrl+f", "ctrl+shift+f":
		return m.focusSearch(0)
	case "tab", "shift+tab":
		order := []int{0}
		if s.replace {
			order = append(order, 1)
		}
		if s.details {
			order = append(order, 3, 4)
		}

		order = append(order, 2)
		index := 0
		for i, v := range order {
			if v == s.focus {
				index = i
				break
			}
		}
		step := 1
		if k == "shift+tab" {
			step = -1
		}
		return m.focusSearch(order[(index+step+len(order))%len(order)])
	case "esc":
		return m.focusSearch(2)
	case "enter":
		if s.focus == 2 {
			return m.openSelected()
		}
		if s.focus == 1 {
			return m.replaceSearch(false)
		}
		m.focusSearch(2)
		return m.runSearch()
	case "down", "up":
		if s.focus != 2 {
			return m.focusSearch(2)
		}
	}
	if s.focus != 2 {
		return m.searchInput(msg)
	}
	if k == "/" || k == "i" {
		return m.focusSearch(0)
	}
	if k == "shift+f10" {
		return m.openSidebarContextMenu()
	}
	_, cmd := m.sidebarNavigationKey(k)
	return cmd
}

func (m *Model) openSearchRight() tea.Cmd {
	r, ok := m.selectedSearchRow()
	if !ok {
		return nil
	}
	path := m.search.result.Files[r.file].Path
	if !m.search.folded[path] {
		return m.openSelected()
	}
	m.search.folded[path] = false
	m.ensureSidebar()
	return nil
}

func (m *Model) searchInput(msg tea.Msg) tea.Cmd {
	s := &m.search
	field := m.searchField(s.focus)
	if field == nil {
		return nil
	}
	before := field.Value()
	var cmd tea.Cmd
	*field, cmd = field.Update(msg)
	if s.focus != 1 && before != field.Value() {
		return tea.Batch(cmd, m.scheduleSearch(150*time.Millisecond))
	}
	return cmd
}

func (m *Model) loadSearchResult() (tea.Cmd, bool) {
	r, ok := m.selectedSearchRow()
	if !ok {
		return nil, false
	}
	f := m.search.result.Files[r.file]
	if r.match < 0 {
		r.match = 0
	}

	if real, err := filepath.EvalSymlinks(f.Path); err != nil || real != f.Path {
		m.notice = searchText("Search file path changed; search again")
		return m.runSearch(), false
	}
	if m.openDocument(f.Path, true) {
		d := m.document()
		if d == nil {
			return nil, false
		}
		if d.Text != f.Text {
			m.notice = searchText("Search result changed; search again")
			m.sidebar = true
			return m.runSearch(), false
		}
		if m.findWidget.visible {
			m.closeFileFind()
		}
		m.findWidget.workspaceMatch = true
		m.adoptSearchState()
		m.findHidden = false
		hit := f.Matches[r.match]
		t := &m.tabs[m.activeTab]
		t.vim.Reset(d)
		t.vim.JumpTo(d, utf8.RuneCountInString(f.Text[:hit.Start]))
		d.SelectRange(utf8.RuneCountInString(f.Text[:hit.End]), d.Cursor)
		m.centerTextCursor()
		return nil, true
	}
	return nil, false
}
