package app

import tea "charm.land/bubbletea/v2"

func (m *Model) openSearchOptions() tea.Cmd {
	m.fileSearchOptionsDialog = m.fileFindHeight() > 0 && !m.sidebar
	return m.showSearchOptions("", 0)
}
func (m *Model) changeSearchOption(id string) tea.Cmd {
	value, index := m.input.Value(), m.pickerIndex
	cmd := m.searchControlAction(id)
	return tea.Batch(cmd, m.showSearchOptions(value, index))
}
func (m *Model) showSearchOptions(filter string, index int) tea.Cmd {
	s := &m.search
	m.palette = nil
	for _, option := range []struct {
		id, label string
		enabled   bool
	}{
		{"case", searchText("Match case"), s.options.Case},
		{"word", searchText("Whole words"), s.options.Word},
		{"regex", searchText("Regular expression"), s.options.Regex},
		{"preserve", searchText("Preserve case"), s.options.PreserveCase},
		{"changed", searchText("Only changed files"), s.options.ChangedOnly},
		{"open", searchText("Only open files"), s.options.OnlyOpen},
		{"ignore", searchText("Respect ignore rules"), !s.options.IncludeIgnored},
	} {
		if m.fileSearchOptionsDialog && (option.id == "changed" || option.id == "open" || option.id == "ignore") {
			continue
		}
		marker := "[ ] "
		if option.enabled {
			marker = "[x] "
		}
		id := option.id
		disabled := ""
		if id == "changed" && !m.repo.HasRepository() {
			disabled = searchText("This folder is not a Git repository")
		}
		m.palette = append(m.palette, action{id: "search.option." + id, label: marker + option.label, disabled: disabled, run: func() tea.Cmd { return m.changeSearchOption(id) }})
	}
	if m.fileSearchOptionsDialog && m.editing() {
		marker := "[ ] "
		if s.replace {
			marker = "[x] "
		}
		m.palette = append(m.palette, action{id: "search.replacement", label: marker + searchText("Show replacement"), run: func() tea.Cmd {
			value, index := m.input.Value(), m.pickerIndex
			s.replace = !s.replace
			if !s.replace && s.focus == 1 {
				m.focusSearch(0)
			}
			return m.showSearchOptions(value, index)
		}})
	}
	m.prompt("searchOptions", searchText("Search options · Enter / Space to toggle"), filter)
	m.selectPickerIndex(index)
	return nil
}
