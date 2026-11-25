package app

import (
	tea "charm.land/bubbletea/v2"
	searchservice "github.com/foursmith/vl/internal/search"
)

func searchOption(options *searchservice.Options, id string) *bool {
	switch id {
	case "case":
		return &options.Case
	case "word":
		return &options.Word
	case "regex":
		return &options.Regex
	case "preserve":
		return &options.PreserveCase
	case "changed":
		return &options.ChangedOnly
	case "open":
		return &options.OnlyOpen
	case "ignore":
		return &options.IncludeIgnored
	default:
		return nil
	}
}

func searchOptionSelected(options *searchservice.Options, id string) bool {
	value := searchOption(options, id)
	return value != nil && *value != (id == "ignore")
}

func (m *Model) openSearchOptions(inFile bool) tea.Cmd {
	return m.showSearchOptions(inFile, "", 0)
}
func (m *Model) changeSearchOption(inFile bool, id string) tea.Cmd {
	value, index := m.input.Value(), m.pickerIndex
	var cmd tea.Cmd
	if inFile {
		m.toggleFileSearchOption(id)
	} else {
		cmd = m.searchControlAction(id)
	}
	return tea.Batch(cmd, m.showSearchOptions(inFile, value, index))
}
func (m *Model) showSearchOptions(inFile bool, filter string, index int) tea.Cmd {
	options := &m.search.options
	if inFile {
		options = &m.findWidget.options
	}
	m.palette = nil
	for _, option := range []struct {
		id, label string
	}{
		{"case", searchText("Match case")},
		{"word", searchText("Whole words")},
		{"regex", searchText("Regular expression")},
		{"preserve", searchText("Preserve case")},
		{"changed", searchText("Only changed files")},
		{"open", searchText("Only open files")},
		{"ignore", searchText("Respect ignore rules")},
	} {
		if inFile && (option.id == "changed" || option.id == "open" || option.id == "ignore") {
			continue
		}
		marker := "[ ] "
		if searchOptionSelected(options, option.id) {
			marker = "[x] "
		}
		id := option.id
		disabled := ""
		if id == "changed" && !m.repo.HasRepository() {
			disabled = searchText("This folder is not a Git repository")
		}
		m.palette = append(m.palette, action{id: "search.option." + id, label: marker + option.label, disabled: disabled, run: func() tea.Cmd { return m.changeSearchOption(inFile, id) }})
	}
	if inFile && m.editing() {
		marker := "[ ] "
		s := &m.findWidget
		if s.replace {
			marker = "[x] "
		}
		m.palette = append(m.palette, action{id: "search.replacement", label: marker + searchText("Show replacement"), run: func() tea.Cmd {
			value, index := m.input.Value(), m.pickerIndex
			s.replace = !s.replace
			if !s.replace && s.focus == 1 {
				m.focusFileFind(0)
			}
			return m.showSearchOptions(inFile, value, index)
		}})
	}
	m.prompt("searchOptions", searchText("Search options · Enter / Space to toggle"), filter)
	m.selectPickerIndex(index)
	return nil
}
