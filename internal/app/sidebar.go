package app

import (
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/elegit/internal/fileicon"
	"github.com/ourongxing/elegit/internal/theme"
)

const sidebarTitleRow = headerHeight
const sidebarRootRow = headerHeight + 1
const sidebarHeaderRows = headerHeight + 2

type navigationItem struct {
	x, width int
	glyph    string
	section  section
	search   bool
}
type sidebarButton struct {
	x              int
	glyph, command string
}

func (m *Model) sidebarAt(x int) bool {
	return m.sideShown() && x < m.mainX() || !m.sideShown() && m.sidebar && m.sidebarVisible
}

func (m *Model) navigation() []navigationItem {
	glyphs := []string{m.symbol(symbolExplorer), m.symbol(symbolSearch), m.symbol(symbolBranch), m.symbol(symbolHistory), m.symbol(symbolCompare)}
	return []navigationItem{{1, 4, glyphs[0], explorer, false}, {5, 4, glyphs[1], explorer, true}, {9, 4, glyphs[2], changes, false}, {13, 4, glyphs[3], history, false}, {17, 4, glyphs[4], branches, false}}
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

func (m *Model) sidebarLinesWidth(w int) []string {
	if w == 0 {
		return nil
	}
	h := m.bodyHeight()
	var nav strings.Builder
	x := 0
	for _, item := range m.navigation() {
		nav.WriteString(row("", item.x-x, theme.Foreground, theme.Panel))
		active := !item.search && item.section == m.section
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
	titles := map[section]string{explorer: "资源管理器", changes: "源代码管理", history: "提交历史", branches: "分支"}
	title := "  " + titles[m.section]
	title += strings.Repeat(" ", max(1, w-ansi.StringWidth(title)-4)) + row(m.symbol(symbolMore), 3, theme.Muted, theme.Panel) + " "
	chevron := m.treeChevron(!m.rootCollapsed)
	root := " " + chevron + " " + safe(strings.ToUpper(filepath.Base(m.repo.Root)))
	if m.section == explorer {
		root = ansi.Truncate(root, max(1, w-13), "…")
		root = theme.Style(theme.Foreground, theme.Panel).Bold(true).Render(root)
		for _, button := range m.rootButtons(w) {
			root += row("", button.x-ansi.StringWidth(root), theme.Foreground, theme.Panel) + row(" "+button.glyph, 3, theme.Muted, theme.Panel)
		}
	}
	lines := []string{row(nav.String(), w, theme.Foreground, theme.Panel), row(title, w, theme.Muted, theme.Panel), row(root, w, theme.Foreground, theme.Panel)}
	hovered := m.hoveredChangeRow()
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
				if i == hovered {
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
	y := msg.Y
	if y >= 0 && y < headerHeight {
		for _, item := range m.navigation() {
			if msg.X >= item.x && msg.X < item.x+item.width {
				if item.search {
					return m.openQuickFiles()
				}
				return m.selectSection(item.section)
			}
		}
		return nil
	}
	w := m.sideWidth()
	if !m.sideShown() {
		w = m.width
	}
	if y == sidebarTitleRow && msg.X >= w-4 {
		cmd := m.openPalette()
		if m.section == explorer {
			m.input.SetValue("Explorer:")
		}
		return cmd
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
	if m.section == explorer || m.section == branches || entry.folder != "" {
		return m.openSelected()
	}
	return m.preview()
}

func (m *Model) openQuickFiles() tea.Cmd {
	m.palette = nil
	for _, path := range m.snapshot.Files {
		m.palette = append(m.palette, action{id: path, label: path, run: func() tea.Cmd { return m.openFile(path) }})
	}
	return m.prompt("palette", "打开文件", "")
}
