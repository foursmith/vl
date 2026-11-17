package app

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func parsePosition(value string) (line, column int, err error) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) > 2 {
		return 0, 0, fmt.Errorf("Enter a line number or line:column")
	}
	line, err = strconv.Atoi(parts[0])
	if err != nil || line < 1 {
		return 0, 0, fmt.Errorf("Line number must be greater than 0")
	}
	column = 1
	if len(parts) == 2 {
		column, err = strconv.Atoi(parts[1])
		if err != nil || column < 1 {
			return 0, 0, fmt.Errorf("Column number must be greater than 0")
		}
	}
	return line, column, nil
}

// An exact filename wins over the suffix syntax, including names containing ':'.
func (m *Model) quickPositionActions(value string) ([]action, bool) {
	if !strings.Contains(value, ":") {
		return nil, false
	}
	colon := -1
	for _, a := range m.palette {
		if a.id == value {
			return nil, false
		}
		if strings.HasPrefix(value, a.id+":") && len(a.id) > colon {
			colon = len(a.id)
		}
	}
	if colon < 0 {
		start := strings.LastIndexAny(value, "/\\") + 1
		if offset := strings.Index(value[start:], ":"); offset >= 0 {
			colon = start + offset
		}
	}
	if colon < 0 {
		return nil, false
	}
	query, position := strings.TrimSpace(value[:colon]), value[colon+1:]
	line, column, err := parsePosition(position)
	reason := ""
	if err != nil {
		reason = m.tr(err.Error())
	}
	key := ""
	if err == nil {
		key = m.tr("Ln %d, Col %d", line, column)
	}
	makeAction := func(path string, run func() tea.Cmd) action {
		return action{id: path, label: path, key: key, disabled: reason, run: run}
	}
	if query == "" {
		d := m.document()
		if d == nil {
			return []action{{label: m.tr("Current file"), disabled: m.tr("Open a text file first, then enter :line[:column]")}}, true
		}
		path, relErr := filepath.Rel(m.repo.Root, d.Path)
		if relErr != nil {
			path = d.Path
		}
		return []action{makeAction(path, func() tea.Cmd { return m.goToPosition(position) })}, true
	}
	var rows []action
	query = strings.ToLower(query)
	for _, a := range m.palette {
		if strings.Contains(strings.ToLower(a.id), query) {
			path := a.id
			rows = append(rows, makeAction(path, func() tea.Cmd {
				if !m.openDocument(path, false) {
					return nil
				}
				if d := m.document(); d != nil {
					d.SetPosition(line-1, column-1)
				} else {
					m.notice = m.tr("Go to line is unavailable in image previews")
				}
				cmd := m.revealDocument(path)
				m.ensureEditor()
				return cmd
			}))
		}
	}
	return rows, true
}

func (m *Model) goToPosition(value string) tea.Cmd {
	d := m.document()
	if d == nil {
		return nil
	}
	line, column, err := parsePosition(value)
	if err != nil {
		m.notice = m.tr(err.Error())
		return nil
	}
	d.SetPosition(line-1, column-1)
	m.sidebar = false
	m.ensureEditor()
	return nil
}
