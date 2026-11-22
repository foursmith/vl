package app

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/theme"
)

type outlineEntry struct {
	label       string
	line, depth int
}

// The header and list share geometry across rendering and input. An
// unavailable outline starts after the body, leaving all rows to the tree.
func (m *Model) outlineLayout() (header int, list sidebarList) {
	end := m.bodyHeight()
	available := end - headerHeight
	doc := m.document()
	if m.section != explorer || m.editorOnly || available < 4 || doc == nil || !isMarkdownPath(doc.Path) {
		return end, sidebarList{}
	}
	if m.outlineCollapsed {
		return end - 1, sidebarList{}
	}

	t := &m.tabs[m.activeTab]
	t.syncMarkdown()
	cache := &t.markdown
	height := max(1, min(len(cache.headings), available/3))
	list = sidebarList{&cache.outlineSelected, &cache.outlineTop, len(cache.headings), end - height, height}
	return list.start - 1, list
}

func (m *Model) outlineHasFocus() bool {
	if !m.sidebar || !m.outlineFocused {
		return false
	}
	header, _ := m.outlineLayout()
	return header < m.bodyHeight()
}

func (m *Model) appendOutline(lines []string, width, header int, l sidebarList) []string {
	if header == m.bodyHeight() {
		return lines
	}
	bg := m.colors.Panel
	focused := m.sidebar && m.outlineFocused
	hovered := m.hoveredOutlineRowAt(header)
	titlePrefix := " "
	if focused && l.count == 0 {
		bg = m.colors.SidebarSelection(true)
		titlePrefix = "›"
	} else if hovered == header {
		bg = m.colors.Hover
	}
	title := titlePrefix + m.treeChevron(!m.outlineCollapsed) + " " + m.tr("Outline")
	rule := strings.Repeat("─", max(0, width-ansi.StringWidth(title)-2))
	ruleColor := theme.WithOpacity(m.colors.Muted, .45)
	headerText := theme.TextStyle(m.colors.Strong, bg).Bold(true).Render(title) + " " +
		theme.Style(ruleColor, bg).Render(rule)
	lines = append(lines, row(headerText, width, m.colors.Strong, bg))
	if l.height == 0 {
		return lines
	}
	if l.count == 0 {
		label := m.tr("No headings")
		return append(lines, row("    "+label, width, theme.Readable(m.colors.Muted, m.colors.Panel), m.colors.Panel))
	}

	top := min(*l.top, max(0, l.count-l.height))
	t := &m.tabs[m.activeTab]
	cursor, _ := t.doc.Position()
	entries := t.markdown.headings
	active := sort.Search(len(entries), func(i int) bool { return entries[i].line > cursor }) - 1
	numberWidth := len(strconv.Itoa(entries[len(entries)-1].line + 1))
	for i := top; i < top+l.height; i++ {
		line := m.outlineRow(entries[i], width, numberWidth, i == active, focused && i == *l.selected, hovered == l.start+i-top)
		lines = append(lines, line)
	}
	return lines
}

func (m *Model) outlineRow(entry outlineEntry, width, numberWidth int, active, selected, hovered bool) string {
	fill, foreground := m.colors.Panel, m.colors.SideForeground
	switch {
	case selected:
		fill = m.colors.SidebarSelection(true)
		if m.colors.SelectionForeground != "" {
			foreground = m.colors.SelectionForeground
		}
	case hovered:
		fill = m.colors.Hover
		if m.colors.HoverForeground != "" {
			foreground = m.colors.HoverForeground
		}
	case active:
		fill = m.colors.SidebarSelection(false)
		if m.colors.InactiveSelectionForeground != "" {
			foreground = m.colors.InactiveSelectionForeground
		}
	}

	marker := " "
	if selected {
		marker = "›"
	} else if active {
		marker = "▎"
	}
	detail := theme.TextStyle(m.colors.Muted, fill)
	accent := theme.TextStyle(m.colors.Accent, fill)
	guide := theme.Style(theme.WithOpacity(m.colors.Muted, .55), fill)
	if selected {
		detail, accent, guide = theme.TextStyle(foreground, fill), theme.TextStyle(foreground, fill), theme.Style(foreground, fill)
	}
	depth := min(entry.depth, max(0, (width-numberWidth-12)/2))
	prefix := accent.Render(marker) + " " + guide.Render(strings.Repeat("│ ", depth)) +
		accent.Render("#") + " "
	labelWidth := max(0, width-ansi.StringWidth(prefix)-numberWidth-2)
	label := ansi.Truncate(safe(entry.label), labelWidth, "…")
	body := theme.TextStyle(foreground, fill).Bold(active).Render(label)
	number := detail.Render(fmt.Sprintf("%*d ", numberWidth, entry.line+1))
	gap := strings.Repeat(" ", labelWidth-ansi.StringWidth(label)+1)
	return row(prefix+body+gap+number, width, foreground, fill)
}

func (m *Model) hoveredOutlineRow() int {
	header, _ := m.outlineLayout()
	return m.hoveredOutlineRowAt(header)
}

func (m *Model) hoveredOutlineRowAt(header int) int {
	if m.modal == "" && !m.resizingSidebar && m.sidebarAt(m.hoverX) && m.hoverY >= header && m.hoverY < m.bodyHeight() {
		return m.hoverY
	}
	return -1
}

func (m *Model) clickOutline(y, header int, l sidebarList) {
	m.lastClickKey = ""
	if y == header {
		m.outlineFocused = false
		m.toggleOutline()
		return
	}
	if l.count > 0 {
		l.clampScroll()
		m.jumpToOutline(*l.top + y - l.start)
	}
}

func (m *Model) toggleOutline() {
	m.outlineCollapsed = !m.outlineCollapsed
	m.clampSidebarScroll()
}

func (m *Model) jumpToOutline(index int) {
	t := &m.tabs[m.activeTab]
	if index >= 0 && index < len(t.markdown.headings) {
		t.markdown.outlineSelected = index
		t.doc.SetPosition(t.markdown.headings[index].line, 0)
		m.focusEditor()
		m.centerTextCursor()
	}
}

func (m *Model) outlineKey(k string) bool {
	header, l := m.outlineLayout()
	if header == m.bodyHeight() {
		return false
	}
	if matchesShortcut(k, sidebarKeys.FocusOutline) {
		m.outlineFocused = !m.outlineFocused
		m.navigationState = navigationState{}
		if m.outlineFocused && l.count > 0 {
			l.clampScroll()
			l.ensureSelected()
		}
		return true
	}
	if !m.outlineFocused {
		return false
	}
	switch {
	case k == "esc":
		m.outlineFocused = false
		return true
	case matchesShortcut(k, navigationKeys.Left):
		if !m.outlineCollapsed {
			m.toggleOutline()
		}
		return true
	case matchesShortcut(k, pickerKeys.Accept), matchesShortcut(k, navigationKeys.Right):
		if l.count > 0 {
			m.jumpToOutline(*l.selected)
		} else {
			m.toggleOutline()
		}
		return true
	}
	if l.count == 0 {
		return false
	}
	action, count, handled := m.navigationState.key(fmt.Sprintf("outline:%d:%d", m.activeTab, *l.selected), k)
	if handled && l.move(action, count) {
		l.clampScroll()
		l.ensureSelected()
	}
	return handled
}
