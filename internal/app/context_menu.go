package app

import (
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/x/ansi"
)

type contextMenu struct {
	x, y, selected, top int
	actions             []action
}

type contextMenuLayout struct{ x, y, width, count int }

func (m *Model) contextMenuLayout() contextMenuLayout {
	menu := m.contextMenu
	width := 24
	for _, a := range menu.actions {
		width = max(width, ansi.StringWidth(a.label)+ansi.StringWidth(a.key)+6)
	}
	width = min(width, m.width)
	count := min(len(menu.actions), max(1, m.height-2))
	return contextMenuLayout{min(max(0, menu.x), m.width-width), min(max(0, menu.y), max(0, m.height-count-2)), width, count}
}

// While a menu is open, input belongs to it. Background data updates still run.
func (m *Model) contextMenuInput(msg tea.Msg) (tea.Cmd, bool) {
	menu := m.contextMenu
	if menu == nil {
		return nil, false
	}
	if m.modal != "" {
		m.contextMenu = nil
		return nil, false
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg, tea.BlurMsg:
		m.contextMenu = nil
		return nil, false
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "shift+f10":
			m.contextMenu = nil
		case "ctrl+q":
			m.contextMenu = nil
			return m.quit(), true
		case "up", "k":
			m.moveContextMenu(-1)
		case "down", "j", "tab":
			m.moveContextMenu(1)
		case "shift+tab":
			m.moveContextMenu(-1)
		case "home":
			menu.selected = 0
			m.ensureContextMenu()
		case "end":
			menu.selected = len(menu.actions) - 1
			m.ensureContextMenu()
		case "enter", "space":
			return m.runContextMenu(), true
		}
		return nil, true
	case tea.MouseClickMsg:
		if msg.Button == tea.MouseRight {
			m.contextMenu = nil
			return nil, false
		}
		if msg.Button == tea.MouseLeft {
			l := m.contextMenuLayout()
			if msg.X > l.x && msg.X < l.x+l.width-1 && msg.Y > l.y && msg.Y <= l.y+l.count {
				menu.selected = menu.top + msg.Y - l.y - 1
				return m.runContextMenu(), true
			}
			m.contextMenu = nil
		}
		return nil, true
	case tea.MouseMotionMsg:
		l := m.contextMenuLayout()
		if msg.X > l.x && msg.X < l.x+l.width-1 && msg.Y > l.y && msg.Y <= l.y+l.count {
			menu.selected = menu.top + msg.Y - l.y - 1
		}
		return nil, true
	case tea.MouseWheelMsg:
		if msg.Button == tea.MouseWheelUp {
			m.moveContextMenu(-1)
		} else if msg.Button == tea.MouseWheelDown {
			m.moveContextMenu(1)
		}
		return nil, true
	case tea.MouseReleaseMsg, tea.PasteMsg:
		return nil, true
	}
	return nil, false
}

func (m *Model) moveContextMenu(delta int) {
	menu := m.contextMenu
	menu.selected = (menu.selected + delta + len(menu.actions)) % len(menu.actions)
	m.ensureContextMenu()
}
func (m *Model) ensureContextMenu() {
	menu := m.contextMenu
	count := m.contextMenuLayout().count
	menu.top = min(menu.top, menu.selected)
	menu.top = max(menu.top, menu.selected-count+1)
}
func (m *Model) runContextMenu() tea.Cmd {
	a := m.contextMenu.actions[m.contextMenu.selected]
	if a.disabled != "" {
		m.notice = a.disabled
		return nil
	}
	m.contextMenu = nil
	if a.run != nil {
		return a.run()
	}
	return nil
}

func (m *Model) contextMenuOverlay(base []string) []string {
	l := m.contextMenuLayout()
	menu := m.contextMenu
	box := []string{row("┌"+strings.Repeat("─", max(0, l.width-2))+"┐", l.width, m.colors.Border, m.colors.Popup)}
	for i := 0; i < l.count; i++ {
		index := menu.top + i
		a := menu.actions[index]
		fg, bg := m.colors.Foreground, m.colors.Popup
		prefix := " "
		if index == menu.selected {
			fg, bg = m.colors.SelectionForeground, m.colors.Selection
			if m.monochrome {
				prefix = ">"
			}
		}
		if a.disabled != "" {
			fg = m.colors.Muted
		}
		text := prefix + ansi.Truncate(a.label, max(0, l.width-6-ansi.StringWidth(a.key)), "…")
		text += strings.Repeat(" ", max(1, l.width-3-ansi.StringWidth(text)-ansi.StringWidth(a.key))) + a.key + " "
		box = append(box, row("│", 1, m.colors.Border, m.colors.Popup)+row(text, l.width-2, fg, bg)+row("│", 1, m.colors.Border, m.colors.Popup))
	}
	box = append(box, row("└"+strings.Repeat("─", max(0, l.width-2))+"┘", l.width, m.colors.Border, m.colors.Popup))
	return m.composeOverlay(base, box, l.x, l.y)
}

func (m *Model) openContextMenu(x, y int) tea.Cmd {
	if m.modal != "" || m.width < 30 || m.height < 8 {
		return nil
	}
	m.stopTextDrag()
	m.imagePan = nil
	m.resizingSidebar = false
	m.lastClickKey = ""
	var actions []action
	add := func(label, key string, run func() tea.Cmd) {
		actions = append(actions, action{label: label, key: key, run: run})
	}
	if m.sidebarAt(x) && y > 0 && y < m.workspaceRows() {
		index := m.workspaceTop + y - 1
		if index >= 0 && index < len(m.workspaces) {
			folder := m.workspaces[index]
			add("打开", "", func() tea.Cmd { return m.openRepository(folder) })
			add("从工作区列表移除", "", func() tea.Cmd { return m.removeWorkspace(folder) })
		}
	} else if m.sidebarAt(x) && y >= m.workspaceRows()+headerHeight && y < m.bodyHeight() {
		m.sidebar = true
		index := m.sideTop + y - m.sidebarHeaderRows()
		var entry *item
		if y >= m.sidebarHeaderRows() && index >= 0 && index < len(m.items) {
			m.selected = index
			copy := m.items[index]
			entry = &copy
		} else {
			m.selected = -1
		}
		// Resolve the row again after refresh; never apply an action to a replacement row.
		bind := func(run func() tea.Cmd) func() tea.Cmd {
			return func() tea.Cmd {
				if entry != nil {
					for i := range m.items {
						if m.items[i].key == entry.key {
							m.selected = i
							return run()
						}
					}
					m.notice = "目标已更改，请重新打开菜单"
					return nil
				}
				m.selected = -1
				return run()
			}
		}
		if m.section == explorer {
			if entry != nil {
				add("打开", "Enter", bind(m.openSelected))
			}
			add("新建文件…", "", bind(func() tea.Cmd { return m.newEntry(false) }))
			add("新建文件夹…", "", bind(func() tea.Cmd { return m.newEntry(true) }))
			add("刷新", workbenchKeys.Refresh.Help().Key, m.refresh)
			add("折叠所有文件夹", "", m.collapseFolders)
		} else if m.section == changes && entry != nil {
			for _, a := range m.changeRowActions(*entry, m.sideWidth()) {
				actions = append(actions, action{label: a.label, disabled: a.disabled, run: bind(func() tea.Cmd { return m.runChangeRowAction(m.selected, a.id) })})
			}
			if entry.change != nil {
				add("打开差异", "Enter", bind(m.openSelected))
			}
		} else if entry != nil {
			add("打开 / 展开", "Enter", bind(m.openSelected))
			if entry.repoHeader {
				repo := entry.repository
				add("切换分支…", "", func() tea.Cmd { return m.openBranchPicker(repo, "") })
			}
		}
		if entry != nil {
			path := entry.file
			if path == "" {
				path = entry.folder
			}
			if entry.change != nil {
				path = entry.change.Path
			}
			if entry.historyChange != nil {
				path = entry.historyChange.Path
			}
			if path != "" {
				actions = append(actions, m.copyPathActions(path)...)
			}
		}
	} else if x >= m.mainX() && x < m.mainX()+m.mainWidth() && !(m.width < 70 && m.sidebar && m.sidebarVisible) && y >= 0 && y < m.bodyHeight() {
		if y == tabRow {
			found := false
			for _, slot := range m.tabLayout() {
				if slot.control == 0 && x-m.mainX() >= slot.x && x-m.mainX() < slot.x+slot.width {
					m.activateTab(slot.index)
					found = true
					break
				}
			}
			if !found {
				return nil
			}
		} else {
			m.sidebar = false
		}
		if m.activeTab >= 0 && m.activeTab < len(m.tabs) {
			if y == tabRow {
				add("关闭", workbenchKeys.Close.Help().Key, m.closeTab)
				add("关闭其他标签页", "", m.closeOtherTabs)
				add("关闭所有标签页", "", m.closeAllTabs)
				if m.tabs[m.activeTab].preview {
					add("保持打开", "", m.keepTab)
				}
			} else if m.editing() {
				selected := m.document().SelectedText() != ""
				reason := "请先选择文本"
				if selected {
					reason = ""
				}
				actions = append(actions, action{label: "复制", key: workbenchKeys.Copy.Help().Key, disabled: reason, run: func() tea.Cmd { return m.copy(false) }}, action{label: "剪切", key: "Ctrl+X", disabled: reason, run: func() tea.Cmd { return m.copy(true) }})
				add("粘贴", "Ctrl+V", m.paste)
				add("全选", "Ctrl+A", func() tea.Cmd { m.document().SelectAll(); return nil })
				add("撤销", "Ctrl+Z", func() tea.Cmd { m.document().Undo(); m.ensureEditor(); return nil })
				add("重做", "Ctrl+Y", func() tea.Cmd { m.document().Redo(); m.ensureEditor(); return nil })
				add("保存", workbenchKeys.Save.Help().Key, m.save)
				add("查找…", workbenchKeys.Find.Help().Key, func() tea.Cmd { return m.prompt("find", "Find in current view", m.find) })
			} else if m.diffOpen {
				reason := ""
				if m.diffSelectedText() == "" {
					reason = "请先选择文本"
				}
				actions = append(actions, action{label: "复制", key: workbenchKeys.Copy.Help().Key, disabled: reason, run: func() tea.Cmd { return m.copy(false) }})
				add("切换差异布局", workbenchKeys.DiffLayout.Help().Key, func() tea.Cmd { m.split = !m.split; m.renderDiff(); return nil })
				add("查找…", workbenchKeys.Find.Help().Key, func() tea.Cmd { return m.prompt("find", "Find in current view", m.find) })
			} else if m.currentImage() != nil {
				add("恢复适应窗口", "0", func() tea.Cmd { m.resetImageZoom(); return nil })
			}
			actions = append(actions, m.copyPathActions(m.tabs[m.activeTab].path())...)
		}
	}
	if len(actions) > 0 {
		m.contextMenu = &contextMenu{x: x, y: y, actions: actions}
	}
	return nil
}

func (m *Model) copyPathActions(path string) []action {
	absolute := path
	if !filepath.IsAbs(absolute) {
		absolute = filepath.Join(m.repo.Root, path)
	}
	relative, err := filepath.Rel(m.repo.Root, absolute)
	if err != nil {
		relative = path
	}
	copy := func(text string) func() tea.Cmd {
		return func() tea.Cmd { return func() tea.Msg { return clipboardMsg{err: clipboard.WriteAll(text)} } }
	}
	return []action{{label: "复制路径", run: copy(absolute)}, {label: "复制相对路径", run: copy(filepath.ToSlash(relative))}}
}
