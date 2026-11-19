package app

import (
	"path/filepath"
	"slices"
	"strings"
	"text/scanner"
)

const maxStickyRows = 5

// Sticky rows overlay the scrolled rows; offsets retain their source meaning.
// Leave at least one content row available even in a very short viewport.
func stickyLimit(height int) int { return min(maxStickyRows, max(0, height-1)) }

type codeScope struct{ start, end int }

func compareCodeScopes(a, b codeScope) int {
	if a.start == b.start {
		return b.end - a.end
	}
	return a.start - b.start
}

func codeScopes(path, text string) []codeScope {
	var scopes []codeScope
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".py" || ext == ".pyw" {
		source := strings.Split(text, "\n")
		var stack []int
		for i, line := range source {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			indent := len(line) - len(strings.TrimLeft(line, " \t"))
			for len(stack) > 0 {
				start := stack[len(stack)-1]
				if indent > len(source[start])-len(strings.TrimLeft(source[start], " \t")) {
					break
				}
				scopes = append(scopes, codeScope{start, i})
				stack = stack[:len(stack)-1]
			}
			if strings.HasSuffix(trimmed, ":") {
				stack = append(stack, i)
			}
		}
		for _, start := range stack {
			scopes = append(scopes, codeScope{start, len(source)})
		}
	} else {
		switch ext {
		case ".go", ".js", ".jsx", ".ts", ".tsx", ".c", ".h", ".cc", ".cpp", ".hpp", ".rs", ".java", ".cs", ".swift", ".css", ".scss", ".php":
		default:
			return nil
		}
		if !strings.ContainsRune(text, '{') {
			return nil
		}
		source := strings.Split(text, "\n")
		var s scanner.Scanner
		s.Init(strings.NewReader(text))
		s.Mode = scanner.ScanIdents | scanner.ScanStrings | scanner.ScanRawStrings | scanner.ScanChars | scanner.ScanComments | scanner.SkipComments
		s.Error = func(*scanner.Scanner, string) {} // Editing may leave incomplete tokens.
		var stack []int
		for token := s.Scan(); token != scanner.EOF; token = s.Scan() {
			line := s.Position.Line - 1
			switch token {
			case '{':
				start := line
				if strings.TrimSpace(source[line]) == "{" {
					for start > 0 && strings.TrimSpace(source[start-1]) == "" {
						start--
					}
					start = max(0, start-1)
				}
				stack = append(stack, start)
			case '}':
				if len(stack) > 0 {
					start := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					if line > start {
						scopes = append(scopes, codeScope{start, line})
					}
				}
			}
		}
		for _, start := range stack {
			scopes = append(scopes, codeScope{start, len(source)})
		}
	}
	slices.SortFunc(scopes, compareCodeScopes)
	return scopes
}

func (m *Model) editorStickyRows() []int {
	t := &m.tabs[m.activeTab]
	top := t.visibleRow(t.top)
	if len(t.scopes) == 0 {
		return nil
	}
	last := t.visibleRow(len(t.doc.Lines()) - 1)
	// Leave the last line visible when it has scrolled into the sticky area.
	limit := min(stickyLimit(m.contentHeight()), max(0, last-top))
	if len(t.folds) == 0 {
		return stickyRows(t.scopes, top, limit)
	}
	var scopes []codeScope
	for _, scope := range t.scopes {
		start, end := t.visibleRow(scope.start), t.visibleRow(scope.end)
		if t.sourceRow(start) == scope.start && end > start {
			scopes = append(scopes, codeScope{start, end})
		}
	}
	rows := stickyRows(scopes, top, limit)
	for i := range rows {
		rows[i] = t.sourceRow(rows[i])
	}
	return rows
}

func stickyRows(scopes []codeScope, top, limit int) []int {
	var rows []int
	for _, scope := range scopes {
		if len(rows) == limit {
			break
		}
		first := top + len(rows)
		if scope.start >= first {
			break
		}
		if scope.end > first && (len(rows) == 0 || rows[len(rows)-1] != scope.start) {
			rows = append(rows, scope.start)
		}
	}
	return rows
}

func (m *Model) editorRowAt(y int, includeImageRows bool) int {
	sticky := m.editorStickyRows()
	if y >= 0 && y < len(sticky) {
		return sticky[y]
	}
	t := &m.tabs[m.activeTab]
	line, image, _ := t.editorDisplayAt(t.editorDisplayTop() + y)
	if image != nil && !includeImageRows {
		return -1
	}
	return line
}

func (m *Model) sidebarStickyRows() []int {
	if m.section != explorer && m.section != changes {
		return nil
	}
	if len(m.items) == 0 {
		return nil
	}
	if len(m.stickyItems) != len(m.items) || &m.stickyItems[0] != &m.items[0] {
		m.stickyItems = m.items
		m.stickyTree = nil
		var stack []int
		depth := func(i int) int {
			entry := m.items[i]
			if m.section == changes && !entry.header {
				return entry.depth + 1
			}
			return entry.depth
		}
		for i, entry := range m.items {
			for len(stack) > 0 {
				scope := &m.stickyTree[stack[len(stack)-1]]
				if depth(scope.start) < depth(i) {
					break
				}
				scope.end = i
				stack = stack[:len(stack)-1]
			}
			if entry.folder != "" || entry.header {
				stack = append(stack, len(m.stickyTree))
				m.stickyTree = append(m.stickyTree, codeScope{i, len(m.items)})
			}
		}
	}
	return stickyRows(m.stickyTree, m.sideTop, stickyLimit(m.treeList().height))
}

func (m *Model) sidebarRowAt(y int) int {
	l := m.sidebarList()
	y -= l.start
	if y < 0 || y >= l.height {
		return -1
	}
	sticky := m.sidebarStickyRows()
	if y < len(sticky) {
		return sticky[y]
	}
	index := *l.top + y
	if index >= l.count {
		return -1
	}
	return index
}

func (m *Model) revealEditorRow(line int) {
	sticky := m.editorStickyRows()
	t := &m.tabs[m.activeTab]
	if line >= t.top && t.visibleRow(line) < t.visibleRow(t.top)+len(sticky) && !slices.Contains(sticky, line) {
		t.top = t.sourceRow(max(0, t.visibleRow(line)-stickyLimit(m.contentHeight())))
	}
}
