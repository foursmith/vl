package app

import (
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/elegit/internal/fileicon"
	"github.com/ourongxing/elegit/internal/theme"
)

const sidebarRootRow = headerHeight
const sidebarHeaderRows = headerHeight + 1

type navigationItem struct {
	x, width int
	glyph    string
	section  section
}
type sidebarButton struct {
	x              int
	glyph, command string
}

func (m *Model) sidebarAt(x int) bool {
	if m.sideShown() {
		return x >= m.sideX() && x < m.sideX()+m.sideWidth()
	}
	return x >= 0 && x < m.width && m.sidebar && m.sidebarVisible
}

func (m *Model) navigation() []navigationItem {
	return []navigationItem{
		{1, 4, m.symbol(symbolExplorer), explorer},
		{5, 4, m.symbol(symbolBranch), changes},
		{9, 4, m.symbol(symbolHistory), history},
	}
}

func (m *Model) rootButtons(w int) []sidebarButton {
	glyphs := []string{m.symbol(symbolNewFile), m.symbol(symbolNewFolder), m.symbol(symbolRefresh), m.symbol(symbolCollapse)}
	commands := []string{"newFile", "newFolder", "refresh", "collapse"}
	var result []sidebarButton
	for i := range glyphs {
		result = append(result, sidebarButton{w - 12 + i*3, glyphs[i], commands[i]})
	}
	return result
}

func (m *Model) sidebarLines() []string { return m.sidebarLinesWidth(m.sideWidth()) }

func (m *Model) hoveredSidebarRow() int {
	if m.resizingSidebar || m.modal != "" || m.hoverX < 0 || !m.sidebarAt(m.hoverX) || m.hoverY < sidebarHeaderRows || m.hoverY >= m.bodyHeight() {
		return -1
	}
	i := m.sideTop + m.hoverY - sidebarHeaderRows
	if i < 0 || i >= len(m.items) {
		return -1
	}
	return i
}

func (m *Model) sidebarLinesWidth(w int) []string {
	if w == 0 {
		return nil
	}
	h := m.bodyHeight()
	var nav strings.Builder
	x := 0
	for _, item := range m.navigation() {
		nav.WriteString(row("", item.x-x, theme.Foreground, theme.Panel))
		active := item.section == m.section
		fg := theme.Muted
		if active {
			fg = theme.Foreground
		}
		if item.section == changes && len(m.snapshot.Changes) > 0 {
			fg = theme.Accent
		}
		label := " " + item.glyph + " "
		label = ansi.Truncate(label, item.width, "")
		label += strings.Repeat(" ", max(0, item.width-ansi.StringWidth(label)))
		nav.WriteString(theme.Style(fg, theme.Panel).Underline(active).Render(label))
		x = item.x + item.width
	}
	chevron := m.treeChevron(!m.rootCollapsed)
	root := " " + chevron + " " + safe(strings.ToUpper(filepath.Base(m.repo.Root)))
	if m.section == explorer {
		root = ansi.Truncate(root, max(1, w-13), "…")
		root = theme.Style(theme.Foreground, theme.Panel).Bold(true).Render(root)
		for _, button := range m.rootButtons(w) {
			root += row("", button.x-ansi.StringWidth(root), theme.Foreground, theme.Panel) + row(" "+button.glyph, 3, theme.Muted, theme.Panel)
		}
	}
	lines := []string{row(nav.String(), w, theme.Foreground, theme.Panel), row(root, w, theme.Foreground, theme.Panel)}
	hovered := m.hoveredChangeRow()
	if m.section == explorer {
		hovered = m.hoveredSidebarRow()
	}
	for y := sidebarHeaderRows; y < h; y++ {
		i := m.sideTop + y - sidebarHeaderRows
		text := ""
		fg, bg := theme.Foreground, theme.Panel
		if i < len(m.items) {
			entry := m.items[i]
			if i == hovered {
				bg = theme.Hover
			}
			if i == m.selected {
				bg = theme.Selection
				if !m.sidebar {
					bg = theme.Border
				}
			}
			if m.section == explorer || m.section == changes {
				text, fg = m.explorerRow(entry, w, bg)
				if m.section == changes && i == hovered {
					text = m.changeActionRow(entry, w, fg, bg)
				}
			} else {
				text = "  " + safe(entry.label)
			}
		}
		lines = append(lines, row(text, w, fg, bg))
	}
	return lines[:h]
}

func (m *Model) explorerRow(entry item, w int, bg string) (string, string) {
	if entry.header {
		return row(" "+m.treeChevron(m.folderExpanded(entry))+" "+entry.label, w, theme.Muted, bg), theme.Muted
	}
	fg := theme.Foreground
	switch entry.status {
	case 'U', 'A':
		fg = theme.GitAdded
	case 'M', 'R', 'C', 'T':
		fg = theme.GitModified
	case 'D', '!':
		fg = theme.Deleted
	}
	if entry.ignored {
		fg = theme.Ignored
	}
	indent := "  " + strings.Repeat("  ", min(entry.depth, 10))
	path := entry.file
	if entry.change != nil {
		path = entry.change.Path
	}
	icon := fileicon.For(path, m.nerd)
	chevron := "  "
	if entry.folder != "" {
		chevron = row(m.treeChevron(m.folderExpanded(entry)), 2, fg, bg)
		icon = fileicon.Folder(entry.folder, m.folderExpanded(entry), m.nerd)
	}
	iconColor := icon.Color
	if entry.ignored {
		iconColor = theme.Ignored
	}
	text := indent + chevron + row(icon.Glyph, 2, iconColor, bg) + safe(entry.label)
	marker := ""
	if entry.status != 0 && !entry.ignored {
		marker = string(entry.status)
		if entry.folder != "" {
			marker = m.symbol(symbolDot)
		}
	}
	text = ansi.Truncate(text, max(0, w-3), "…")
	if marker != "" {
		text += strings.Repeat(" ", max(1, w-2-ansi.StringWidth(text))) + marker
	}
	return text, fg
}

func (m *Model) clickSidebar(msg tea.MouseClickMsg) tea.Cmd {
	msg.X -= m.sideX()
	y := msg.Y
	if y >= 0 && y < headerHeight {
		for _, item := range m.navigation() {
			if msg.X >= item.x && msg.X < item.x+item.width {
				return m.selectSection(item.section)
			}
		}
		return nil
	}
	w := m.sideWidth()
	if !m.sideShown() {
		w = m.width
	}
	if y == sidebarRootRow && m.section == explorer {
		for _, button := range m.rootButtons(w) {
			if msg.X >= button.x && msg.X < button.x+3 {
				switch button.command {
				case "newFile":
					return m.newEntry(false)
				case "newFolder":
					return m.newEntry(true)
				case "refresh":
					return m.refresh()
				case "collapse":
					return m.collapseFolders()
				}
			}
		}
		m.rootCollapsed = !m.rootCollapsed
		m.rebuildItems()
		return nil
	}
	i := m.sideTop + y - sidebarHeaderRows
	if y < sidebarHeaderRows || i < 0 || i >= len(m.items) {
		return nil
	}
	if m.section == changes {
		if cmd, handled := m.clickChangeAction(i, msg.X, w); handled {
			return cmd
		}
	}
	if m.items[i].header {
		m.selected = i
		m.sidebar = true
		return m.openSelected()
	}
	m.selected = i
	m.sidebar = true
	entry := m.current()
	if m.section == explorer && entry.file != "" {
		if m.doubleClick("file:" + entry.file) {
			return m.openFile(entry.file)
		}
		cmd := m.openPreviewFile(entry.file)
		m.sidebar = m.sideShown()
		return cmd
	}
	m.lastClickKey = ""
	if m.section == explorer || entry.folder != "" {
		return m.openSelected()
	}
	return m.preview()
}
