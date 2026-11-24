package app

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/theme"
)

type outlineEntry struct {
	label       string
	line, depth int
}

func (m *Model) outlineAvailable() bool {
	doc := m.document()
	return doc != nil && isMarkdownPath(doc.Path)
}

func (m *Model) outlineLayout() (int, sidebarList) {
	return m.explorerPanelLayout(explorerPanelOutline)
}

func (m *Model) outlineList() sidebarList {
	t := &m.tabs[m.activeTab]
	t.syncMarkdown()
	cache := &t.markdown
	return sidebarList{selected: &cache.outlineSelected, top: &cache.outlineTop, count: len(cache.headings)}
}

func (m *Model) outlinePanelFocused() bool { return m.outlineFocused }
func (m *Model) blurOutline()              { m.outlineFocused = false }

func (m *Model) outlineHasFocus() bool {
	if !m.sidebar || !m.outlineFocused {
		return false
	}
	header, _ := m.outlineLayout()
	return header < m.bodyHeight()
}

func (m *Model) appendOutline(lines []string, width int, l sidebarList) []string {
	if l.height == 0 {
		return lines
	}
	focused := m.sidebar && m.outlineFocused
	hovered := m.explorerPanelHoverY()
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
	for i := top; i < min(l.count, top+l.height); i++ {
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

func (m *Model) clickOutline(y int, l sidebarList) tea.Cmd {
	m.lastClickKey = ""
	l.clampScroll()
	m.jumpToOutline(*l.top + y - l.start)
	return nil
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
		if m.outlineFocused {
			m.setExplorerPanel(explorerPanelOutline)
			_, l = m.outlineLayout()
			if l.count > 0 {
				l.clampScroll()
				l.ensureSelected()
			}
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
		if l.height > 0 {
			m.explorerPanel = explorerPanelNone
			m.clampSidebarScroll()
		}
		return true
	case matchesShortcut(k, pickerKeys.Accept), matchesShortcut(k, navigationKeys.Right):
		if l.count > 0 {
			m.jumpToOutline(*l.selected)
		} else {
			m.setExplorerPanel(explorerPanelOutline)
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
