package app

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/theme"
)

type contextMenu struct {
	modal               string
	x, y, selected, top int
	actions             []action
	navigation          navigationState
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
	if m.modal != menu.modal {
		m.contextMenu = nil
		return nil, false
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg, tea.BlurMsg:
		m.contextMenu = nil
		return nil, false
	case tea.KeyPressMsg:
		m.navigationState = navigationState{}
		k := msg.String()
		if k == "h" {
			k = "esc"
		}
		if k == "l" {
			k = "enter"
		}
		if k != "d" && k != "y" {
			action, count, handled := menu.navigation.key(fmt.Sprintf("menu:%d", menu.selected), k)
			if handled {
				switch action {
				case "":
					return nil, true
				case "up":
					m.moveContextMenu(-count)
				case "down":
					m.moveContextMenu(count)
				case "first":
					menu.selected = min(len(menu.actions)-1, count-1)
					m.ensureContextMenu()
				case "last":
					menu.selected = len(menu.actions) - 1
					m.ensureContextMenu()
				case "pageUp":
					m.moveContextMenu(-count * m.contextMenuLayout().count)
				case "pageDown":
					m.moveContextMenu(count * m.contextMenuLayout().count)
				}
				return nil, true
			}
		} else {
			menu.navigation = navigationState{}
		}
		switch k {
		case "esc", "shift+f10":
			m.contextMenu = nil
		case "ctrl+q":
			m.contextMenu = nil
			return m.quit(), true
		case "tab":
			m.moveContextMenu(1)
		case "shift+tab":
			m.moveContextMenu(-1)
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
	menu.selected = ((menu.selected+delta)%len(menu.actions) + len(menu.actions)) % len(menu.actions)
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
	border := theme.Style(m.colors.MenuBorder, m.colors.MenuBackground)
	side := border.Render("│")
	box := []string{inputBorderRule(border, l.width, "╭", "╮")}
	for i := 0; i < l.count; i++ {
		index := menu.top + i
		a := menu.actions[index]
		fg, bg := m.colors.MenuForeground, m.colors.MenuBackground
		prefix := " "
		if index == menu.selected {
			fg, bg = m.colors.MenuSelectionForeground, m.colors.MenuSelection
			if m.monochrome {
				prefix = ">"
			}
		}
		if a.disabled != "" {
			fg = m.colors.Brand
		}
		text := prefix + ansi.Truncate(a.label, max(0, l.width-6-ansi.StringWidth(a.key)), "…")
		text += strings.Repeat(" ", max(1, l.width-3-ansi.StringWidth(text)-ansi.StringWidth(a.key))) + a.key + " "
		fg = theme.Readable(fg, bg)
		if a.disabled != "" {
			text = theme.Style(fg, bg).Italic(true).Render(text)
		}
		box = append(box, side+row(text, l.width-2, fg, bg)+side)
	}
	box = append(box, inputBorderRule(border, l.width, "╰", "╯"))
	return m.composeOverlay(base, box, l.x, l.y)
}

func (m *Model) openContextMenu(x, y int) tea.Cmd {
	m.leaderTarget = ""
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
	if m.sidebarAt(x) && y >= headerHeight && y < m.bodyHeight() {
		m.sidebar = true
		entry := m.selectSidebarRow(m.sidebarRowAt(y))
		if entry == nil {
			if m.section == search {
				return nil
			}
			m.selected = -1
		}
		section, root := m.section, m.repo.Root
		// Resolve the row again after refresh before running the action.
		bind := func(run func() tea.Cmd) func() tea.Cmd {
			return func() tea.Cmd {
				if m.section == section && m.repo.Root == root {
					if entry == nil {
						m.selected = -1
						return run()
					}
					if index := m.sidebarRowWithKey(entry.key); index >= 0 {
						*m.sidebarList().selected = index
						return run()
					}
				}
				m.notice = m.tr("Target changed; reopen the menu")
				return nil
			}
		}
		if entry != nil {
			if label := m.sidebarOpenLabel(*entry); label != "" {
				add(label, sidebarKeys.Open.Help().Key, bind(m.openSelected))
			}
		}
		if m.section == explorer {
			if entry != nil {
				if entry.folder != "" && strings.Contains(entry.label, "/") {
					add(m.tr("Split Folders"), sidebarKeys.SplitFolders.Help().Key, bind(m.splitFolderChain))
				}
				if !entry.header {
					add(m.tr("Copy"), "yy", bind(m.yankEntry))
				}
			}
			add(m.tr("Paste"), sidebarKeys.Paste.Help().Key, bind(m.pasteEntryPrompt))
			add(m.tr("New File…"), "", bind(func() tea.Cmd { return m.newEntry(false) }))
			add(m.tr("New Folder…"), "", bind(func() tea.Cmd { return m.newEntry(true) }))
			add(m.tr("Refresh"), workbenchKeys.SidebarRefresh.Help().Key, m.refreshView)
			add(m.tr("Collapse Folders"), "", m.collapseFolders)
		} else if m.section == changes && entry != nil {
			for _, a := range m.changeRowActions(*entry, m.sidebarContentWidth()) {
				var shortcut string
				switch a.id {
				case "stage", "unstage":
					shortcut = stageKey
					if entry.repoHeader && !m.stageScopeUnstages(entry) {
						shortcut += " / " + stageScopeKey
					}
				case "discard":
					shortcut = discardKey
				case "commit":
					shortcut = changesCommitKey
				case "refresh":
					shortcut = workbenchKeys.SidebarRefresh.Help().Key
				}
				actions = append(actions, action{label: a.label, key: shortcut, disabled: a.disabled, run: bind(func() tea.Cmd { return m.runChangeRowAction(m.selected, a.id) })})
			}
			if entry.repoHeader {
				scope := *entry
				scope.staged = true
				shortcut := ""
				if m.stageScopeUnstages(entry) {
					shortcut = stageScopeKey
				}
				actions = append(actions, action{
					label:    m.tr("Unstage All Changes"),
					key:      shortcut,
					disabled: m.changeActionDisabled(scope, "unstage", m.scopedChanges(scope)),
					run:      bind(func() tea.Cmd { return m.runScopeAction(scope, "unstage") }),
				})
				actions = append(actions, action{
					label:    m.tr("Discard All Changes"),
					key:      discardScopeKey,
					disabled: m.discardScopeReason(entry),
					run:      bind(func() tea.Cmd { return m.runScopeAction(*entry, "discard") }),
				})
			}
		} else if m.section == search && entry != nil {
			add(m.tr("Dismiss Result"), "dd / "+sidebarKeys.Delete.Help().Key, bind(func() tea.Cmd { m.dismissSearchResult(m.search.selected); return nil }))
		} else if m.section == pullRequests {
			if entry != nil {
				if entry.repoHeader {
					add(m.tr("Filter Pull Requests"), "", m.openPullRequestFilter)
				}
				if entry.pullRequest != nil {
					add(m.tr("Open on GitHub"), sidebarKeys.OpenPullRequest.Help().Key, func() tea.Cmd { return m.openBrowser(entry.pullRequest.URL) })
				}
			}
			add(m.tr("Refresh"), workbenchKeys.SidebarRefresh.Help().Key, m.refreshView)
		}
		if entry != nil {
			if m.selectedEntryPath() != "" {
				label := m.tr("Delete…")
				if entry.folder != "" {
					label = m.tr("Delete Folder…")
				}
				add(label, "dd / "+sidebarKeys.Delete.Help().Key, bind(m.deleteSelectedEntry))
			}
			path := entryPath(entry)
			if path != "" {
				actions = append(actions, m.copyPathActions(path)...)
			}
		}
	} else if x >= m.mainX() && x < m.mainX()+m.mainWidth() && !(m.width < 70 && m.sidebar && m.sidebarVisible) && y >= 0 && y < m.bodyHeight() {
		if !m.editorOnly && y == tabRow {
			actions = m.tabContextActions(x - m.mainX())
		} else {
			m.focusEditor()
			if m.activeTab >= 0 && m.activeTab < len(m.tabs) {
				if m.editing() {
					selected := m.document().SelectedText() != ""
					reason := m.tr("Select text first")
					if selected {
						reason = ""
					}
					actions = append(actions, action{label: m.tr("Copy"), key: workbenchKeys.Copy.Help().Key, disabled: reason, run: func() tea.Cmd { return m.copy(false) }}, action{label: m.tr("Cut"), key: "Ctrl+X", disabled: reason, run: func() tea.Cmd { return m.copy(true) }})
					add(m.tr("Paste"), "Ctrl+V", m.paste)
					add(m.tr("Select All"), "Ctrl+A", func() tea.Cmd { m.document().SelectAll(); return nil })
					add(m.tr("Undo"), "Ctrl+Z", func() tea.Cmd { m.document().Undo(); m.ensureEditor(); return nil })
					add(m.tr("Redo"), "Ctrl+Y", func() tea.Cmd { m.document().Redo(); m.ensureEditor(); return nil })
					add(m.tr("Save"), workbenchKeys.Save.Help().Key, m.save)
					add(m.tr("Find…"), workbenchKeys.Find.Help().Key, m.openFileFind)
				} else if m.diffOpen {
					if index := m.viewport.YOffset() + y - m.contentRow(); y >= m.contentRow() && index < len(m.diffRowHunks) {
						m.diffHunk = m.diffRowHunks[index]
					}
					actions = append(actions, m.reviewActions()...)
					reason := ""
					if m.diffSelectedText() == "" {
						reason = m.tr("Select text first")
					}
					actions = append(actions, action{label: m.tr("Copy"), key: workbenchKeys.Copy.Help().Key, disabled: reason, run: func() tea.Cmd { return m.copy(false) }})
					add(m.tr("Toggle Inline View"), workbenchKeys.DiffLayout.Help().Key, func() tea.Cmd { m.split = !m.split; m.renderDiff(); return nil })
					add(m.tr("Find…"), workbenchKeys.Find.Help().Key, m.openFileFind)
				} else if m.currentImage() != nil {
					add(m.tr("Fit to Window"), "0", func() tea.Cmd { m.resetImageZoom(); return nil })
				}
				actions = append(actions, m.copyPathActions(m.tabs[m.activeTab].path())...)
			}
		}
	}
	if len(actions) > 0 {
		m.contextMenu = &contextMenu{x: x, y: y, actions: actions}
	}
	return nil
}

func (m *Model) sidebarOpenLabel(entry item) string {
	switch m.sidebarActivation(entry) {
	case toggleFolder, toggleCommit, togglePR:
		return m.tr("Open / Expand")
	case loadMoreCommits:
		return m.tr("Load more commits…")
	case loadMorePRs:
		return m.tr("Load more pull requests…")
	case openFileRow:
		if entry.change != nil {
			return m.tr("Open Changes")
		}
		return m.tr("Open")
	}
	return ""
}

func (m *Model) tabContextActions(x int) []action {
	for _, slot := range m.tabLayout() {
		if slot.control != 0 || x < slot.x || x >= slot.x+slot.width {
			continue
		}
		target := m.tabs[slot.index]
		targetKey := target.clickKey()
		bind := func(run func(int) tea.Cmd) func() tea.Cmd {
			return func() tea.Cmd {
				for i, t := range m.tabs {
					if t.clickKey() == targetKey {
						return run(i)
					}
				}
				m.notice = m.tr("Target changed; reopen the menu")
				return nil
			}
		}
		actions := []action{
			{label: m.tr("Close"), key: workbenchKeys.Close.Help().Key, run: bind(func(i int) tea.Cmd {
				return m.requestCloseTabs(m.tabs[i : i+1])
			})},
			{label: m.tr("Close Others"), run: bind(m.closeOtherTabsExcept)},
			{label: m.tr("Close All"), run: m.closeAllTabs},
		}
		return append(actions, m.copyPathActions(target.path())...)
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
		return func() tea.Cmd { return writeClipboard(text, nil) }
	}
	return []action{{label: m.tr("Copy Path"), run: copy(absolute)}, {label: m.tr("Copy Relative Path"), run: copy(filepath.ToSlash(relative))}}
}
