package app

import (
	"context"
	"path/filepath"
	"time"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	searchservice "github.com/foursmith/vl/internal/search"
)

type searchState struct {
	query, replacement textinput.Model
	include, exclude   textinput.Model
	replace            bool
	details            bool
	options            searchservice.Options
	result             searchservice.Result
	rows               []searchRow
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
	next   tea.Cmd
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
	return m.openSearchAt(0)
}

func (m *Model) openSearchAt(focus int) tea.Cmd {
	m.notice = ""
	if m.findWidget.visible {
		m.closeFileFind()
	}
	m.findWidget.focused = false
	m.selectSection(search)
	m.navigationState = navigationState{}
	cmd := m.focusSearch(focus)
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
	s.rows = nil
	s.err = ""
	s.top = 0
	s.selected = 0
	s.loading = s.options.Query != ""
	if !s.loading {
		return nil
	}
	matcher, err := s.options.Compile()
	if err != nil {
		s.err = err.Error()
		s.loading = false
		return nil
	}
	ctx, cancel := context.WithCancel(m.ctx)
	s.cancel = cancel
	seq, root := s.seq, m.repo.Root
	buffers := map[string]string{}
	for _, t := range m.tabs {
		if t.doc != nil {
			buffers[t.doc.Path] = t.doc.Text
		}
	}
	return func() tea.Msg {
		if delay > 0 {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return nil
			case <-timer.C:
			}
		}
		results := make(chan searchMsg)
		receive := func() tea.Msg {
			select {
			case <-ctx.Done():
				return nil
			case msg := <-results:
				return msg
			}
		}
		go func() {
			defer cancel()
			var batch searchservice.Result
			var last time.Time
			publish := func(next tea.Cmd, err error) {
				select {
				case <-ctx.Done():
				case results <- searchMsg{seq: seq, result: batch, err: err, next: next}:
				}
				batch = searchservice.Result{}
				last = time.Now()
			}
			unreadable, err := matcher.Scan(ctx, root, buffers, func(file searchservice.File) {
				batch.Count += len(file.Matches)
				batch.Files = append(batch.Files, file)
				if time.Since(last) >= 120*time.Millisecond {
					publish(receive, nil)
				}
			})
			batch.Unreadable = unreadable
			publish(nil, err)
		}()
		return receive()
	}
}

func (m *Model) applySearchResults(msg searchMsg) tea.Cmd {
	s := &m.search
	if s.seq != msg.seq {
		return nil
	}
	start := len(s.result.Files)
	s.result.Files = append(s.result.Files, msg.result.Files...)
	s.result.Count += msg.result.Count
	s.result.Unreadable += msg.result.Unreadable
	if s.rows != nil {
		s.appendRows(start)
	}
	if msg.next == nil {
		s.loading = false
		if msg.err != nil {
			s.err = msg.err.Error()
		}
	}
	m.clampSearch()
	return msg.next
}
func (m *Model) searchRows() []searchRow {
	s := &m.search
	if len(s.result.Files) == 0 {
		return nil
	}
	if s.rows == nil {
		s.appendRows(0)
	}
	return s.rows
}

func (m *Model) searchRowWithKey(key string) int {
	for index, row := range m.searchRows() {
		if m.searchEntry(row).key == key {
			return index
		}
	}
	return -1
}

func (s *searchState) appendRows(start int) {
	for i := start; i < len(s.result.Files); i++ {
		f := s.result.Files[i]
		s.rows = append(s.rows, searchRow{i, -1})
		if !s.folded[f.Path] {
			for j := range f.Matches {
				if j == 0 || f.Matches[j].Line != f.Matches[j-1].Line {
					s.rows = append(s.rows, searchRow{i, j})
				}
			}
		}
	}
}

func (s *searchState) setFolded(path string, folded bool) {
	s.folded[path] = folded
	s.rows = nil
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
	rows := m.searchRows()
	if index := m.search.selected; index >= 0 && index < len(rows) {
		return rows[index], true
	}
	return searchRow{}, false
}

func (m *Model) searchKey(msg tea.KeyPressMsg) tea.Cmd {
	s := &m.search
	k := msg.String()
	switch {
	case m.matchWorkbenchCommand(k, commandFind):
		return m.focusSearch(0)
	case key.Matches(msg, searchKeys.NextField, searchKeys.PreviousField):
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
		if key.Matches(msg, searchKeys.PreviousField) {
			step = -1
		}
		return m.focusSearch(order[(index+step+len(order))%len(order)])
	case key.Matches(msg, dismissKey):
		return m.focusSearch(2)
	case pageDelta(k, 1) != 0 && s.focus != 2:
		m.focusSearch(2)
		_, cmd := m.sidebarNavigationKey(msg)
		return cmd
	case key.Matches(msg, sidebarKeys.Open):
		if s.focus == 2 {
			return m.openSelected()
		}
		if s.focus == 1 {
			return m.replaceSearch(false)
		}
		m.focusSearch(2)
		return m.runSearch()
	case key.Matches(msg, pickerKeys.Down, pickerKeys.Up):
		if s.focus != 2 {
			if msg.Mod.Contains(tea.ModCtrl) {
				_, cmd := m.sidebarNavigationKey(msg)
				return cmd
			}
			return m.focusSearch(2)
		}
	}
	if s.focus != 2 {
		return m.searchInput(msg)
	}
	if key.Matches(msg, searchKeys.FocusQuery) {
		return m.focusSearch(0)
	}
	if m.matchWorkbenchCommand(k, commandContextMenu) {
		return m.openSidebarContextMenu()
	}
	_, cmd := m.sidebarNavigationKey(msg)
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
	m.search.setFolded(path, false)
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
		matches, current := f.ResolveMatches(d.Text)
		if !current {
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
		hit := matches[r.match]
		t := &m.tabs[m.activeTab]
		t.vim.Reset(d)
		t.vim.JumpTo(d, utf8.RuneCountInString(d.Text[:hit.Start]))
		d.SelectRange(utf8.RuneCountInString(d.Text[:hit.End]), d.Cursor)
		m.centerTextCursor()
		return nil, true
	}
	return nil, false
}
