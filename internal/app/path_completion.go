package app

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/vl/internal/editor"
)

type pathCompletion struct {
	doc      *editor.Document
	text     string
	cursor   int
	dir      string
	prefix   string
	quote    rune
	entries  []string
	matches  []string
	selected int
}

type pathEntriesMsg struct {
	request *pathCompletion
	entries []string
}

func relativePathPrefix(before, after string) (path string, quote rune) {
	relative := func(p string) bool {
		return strings.HasPrefix(p, "./") || strings.HasPrefix(p, "../")
	}
	isQuote := func(r rune) bool { return r == '"' || r == '\'' || r == '`' }
	next, _ := utf8.DecodeRuneInString(after)
	if i := strings.LastIndexFunc(before, isQuote); i >= 0 {
		q := rune(before[i])
		if p := before[i+1:]; relative(p) {
			if after == "" || next == q {
				return p, q
			}
			return "", 0
		}
	}
	if after != "" && !pathDelimiter(next) {
		return "", 0
	}
	if i := strings.LastIndexFunc(before, pathDelimiter); i >= 0 {
		_, size := utf8.DecodeRuneInString(before[i:])
		before = before[i+size:]
	}
	if relative(before) {
		return before, 0
	}
	return "", 0
}

func pathDelimiter(r rune) bool {
	return unicode.IsSpace(r) || unicode.IsControl(r) || strings.ContainsRune("\"'`<>()[]{}=,;|&", r)
}

func (m *Model) pathCompletionDocument() *editor.Document {
	if !m.editing() || m.sidebar || m.modal != "" || m.contextMenu != nil || m.fileFindFocused() ||
		!m.tabs[m.activeTab].vim.InsertMode {
		return nil
	}
	d := m.document()
	if d.ReadOnly || d.Path == "" || d.Anchor >= 0 {
		return nil
	}
	return d
}

func (m *Model) refreshPathCompletion() tea.Cmd {
	d := m.pathCompletionDocument()
	if d == nil {
		m.pathCompletion = nil
		return nil
	}
	i := d.ByteOffset(d.Cursor)
	before := d.Text[strings.LastIndexByte(d.Text[:i], '\n')+1 : i]
	after, _, _ := strings.Cut(d.Text[i:], "\n")
	path, quote := relativePathPrefix(before, after)
	if path == "" {
		m.pathCompletion = nil
		return nil
	}
	slash := strings.LastIndexByte(path, '/')
	dir := filepath.Join(filepath.Dir(d.Path), filepath.FromSlash(path[:slash+1]))
	c := m.pathCompletion
	var cmd tea.Cmd
	if c == nil || c.doc != d || c.dir != dir {
		c = &pathCompletion{doc: d, dir: dir}
		m.pathCompletion = c
		cmd = func() tea.Msg {
			return pathEntriesMsg{request: c, entries: readPathEntries(dir)}
		}
	}
	c.text, c.cursor, c.prefix, c.quote = d.Text, d.Cursor, path[slash+1:], quote
	c.filter()
	return cmd
}

func readPathEntries(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		isDir := entry.IsDir()
		if entry.Type()&os.ModeSymlink != 0 {
			info, err := os.Stat(filepath.Join(dir, name))
			isDir = err == nil && info.IsDir()
		}
		if isDir {
			name += "/"
		}
		names = append(names, name)
	}
	return names
}

func (c *pathCompletion) filter() {
	c.matches = c.matches[:0]
	c.selected = 0
	for _, name := range c.entries {
		if name == c.prefix || !strings.HasPrefix(name, c.prefix) ||
			c.prefix == "" && strings.HasPrefix(name, ".") || !utf8.ValidString(name) {
			continue
		}
		if strings.ContainsFunc(name, func(r rune) bool {
			return unicode.IsControl(r) || r == '\\' || r == c.quote || c.quote == 0 && pathDelimiter(r)
		}) {
			continue
		}
		c.matches = append(c.matches, name)
	}
}

func (m *Model) validPathCompletion() bool {
	c := m.pathCompletion
	d := m.pathCompletionDocument()
	return c != nil && d == c.doc && d.Cursor == c.cursor && d.Text == c.text
}

func (m *Model) pathCompletionKey(k string) (bool, tea.Cmd) {
	c := m.pathCompletion
	if c == nil || len(c.matches) == 0 || (k != "up" && k != "down" && k != "tab") {
		return false, nil
	}
	if !m.validPathCompletion() || m.editorCursor() == nil || m.width < 30 || m.height < 8 {
		return false, nil
	}
	switch k {
	case "up":
		c.selected = max(0, c.selected-1)
	case "down":
		c.selected = min(len(c.matches)-1, c.selected+1)
	case "tab":
		suffix := c.matches[c.selected][len(c.prefix):]
		c.doc.BeginEditGroup()
		c.doc.Insert(suffix)
		m.ensureTextCursor()
		if strings.HasSuffix(suffix, "/") {
			return true, m.refreshPathCompletion()
		}
		m.pathCompletion = nil
	}
	return true, nil
}

func (m *Model) pathCompletionOverlay(base []string, cursor *tea.Cursor) []string {
	c := m.pathCompletion
	if cursor == nil || !m.validPathCompletion() || len(c.matches) == 0 {
		return base
	}
	above, below := cursor.Y-m.contentRow(), m.bodyHeight()-cursor.Y-1
	count := min(6, len(c.matches))
	up := below < count && above > below
	y := cursor.Y + 1
	if up {
		count = min(count, above)
		y = cursor.Y - count
	} else {
		count = min(count, below)
	}
	if count <= 0 {
		return base
	}
	start := max(0, c.selected-count+1)
	width := 16
	for _, name := range c.matches[start : start+count] {
		width = max(width, ansi.StringWidth(name)+8)
	}
	width = min(width, 44, m.mainWidth())
	x := min(cursor.X, m.mainX()+m.mainWidth()-width)
	lines := make([]string, 0, count)
	for i, name := range c.matches[start : start+count] {
		fg, bg := m.colors.PopupForeground, m.colors.Popup
		label := "  " + name
		if start+i == c.selected {
			fg, bg = m.colors.PopupSelectionForeground, m.colors.PopupSelection
			if m.monochrome {
				label = "> " + name
			}
			label = row(label, width-5, fg, bg) + " Tab "
		}
		lines = append(lines, row(label, width, fg, bg))
	}
	return m.composeOverlay(base, lines, x, y)
}
