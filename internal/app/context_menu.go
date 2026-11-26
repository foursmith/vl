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
		if cmd, handled := m.runContextMenuShortcut(k); handled {
			return cmd, true
		}
		left, right := matchesShortcut(k, navigationKeys.Left), matchesShortcut(k, navigationKeys.Right)
		if !left && !right {
			action, count, handled := menu.navigation.key(fmt.Sprintf("menu:%d", menu.selected), k)
			if handled {
				switch action {
				case "":
					return nil, true
				case sidebarDeleteSequence, sidebarCopySequence:
					if count == 1 {
						cmd, _ := m.runContextMenuShortcut(action)
						return cmd, true
					}
				case "up":
					m.moveContextMenu(-count)
				case "down":
					m.moveContextMenu(count)
				default:
					height := m.contextMenuLayout().count
					index, scroll, moved := moveListSelection(action, menu.selected, len(menu.actions), height, count)
					if moved {
						menu.selected = index
						menu.top = min(len(menu.actions)-height, max(0, menu.top+scroll))
						m.ensureContextMenu()
					}
				}
				return nil, true
			}
		} else {
			menu.navigation = navigationState{}
		}
		switch {
		case matchesShortcut(k, dismissKey), left, m.matchWorkbenchCommand(k, commandContextMenu):
			m.contextMenu = nil
		case matchesShortcut(k, searchKeys.NextField):
			m.moveContextMenu(1)
		case matchesShortcut(k, searchKeys.PreviousField):
			m.moveContextMenu(-1)
		case matchesShortcut(k, dialogKeys.Accept), right:
			return m.runContextMenu(), true
		}
		if cmd, handled := m.runOverlayWorkbenchShortcut(k); handled {
			return cmd, true
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

func (m *Model) runContextMenuShortcut(name string) (tea.Cmd, bool) {
	menu := m.contextMenu
	for i, a := range menu.actions {
		if a.key == "" {
			continue
		}
		matches := a.id != "" && m.matchWorkbenchCommand(name, commandID(a.id))
		if !matches && !m.activeWorkbenchKeymap().byID[a.id].has(diffInput) {
			matches = menuShortcutMatches(name, a.key)
		}
		if matches {
			menu.selected = i
			return m.runContextMenu(), true
		}
	}
	return nil, false
}

func menuShortcutMatches(name, hint string) bool {
	for label := range strings.SplitSeq(hint, " / ") {
		switch {
		case len(label) == 1:
			if name == label || label[0] >= 'A' && label[0] <= 'Z' && name == "shift+"+strings.ToLower(label) {
				return true
			}
		case label == sidebarKeys.Delete.Help().Key:
			if matchesShortcut(name, sidebarKeys.Delete) {
				return true
			}
		case strings.EqualFold(name, label):
			return true
		}
	}
	return false
}

func (m *Model) runContextMenu() tea.Cmd {
	a := m.contextMenu.actions[m.contextMenu.selected]
	if a.disabled != "" {
		m.notice = a.disabled
		m.contextMenu.navigation = navigationState{}
		m.ensureContextMenu()
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
	border := theme.Style(m.colors.MenuBorder, m.colors.Panel)
	side := border.Render("│")
	box := []string{inputBorderRule(border, l.width, "╭", "╮")}
	for i := 0; i < l.count; i++ {
		index := menu.top + i
		a := menu.actions[index]
		fg, bg := m.colors.MenuForeground, m.colors.Panel
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
	m.cancelLeader()
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
	if m.fileFindAt(x, y) {
		m.focusEditor()
		add(m.tr("Search Options"), "", func() tea.Cmd { return m.openSearchOptions(true) })
	} else if m.section == search && m.sidebarAt(x) && y >= headerHeight && y < min(m.bodyHeight(), m.searchPanelLayout().results) {
		m.sidebar = true
		add(m.tr("Search Options"), "", func() tea.Cmd { return m.openSearchOptions(false) })
	} else if m.sidebarAt(x) && y >= headerHeight && y < m.bodyHeight() {
		if m.section == explorer && y >= headerHeight+m.treeList().height {
			return nil
		}
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
		if entry != nil && entry.repoHeader && (section == changes || section == history || section == pullRequests) {
			add(m.tr("Switch Branch…"), "", bind(func() tea.Cmd { return m.openBranchPicker(entry.repository, "") }))
			if section == changes || section == history {
				for _, a := range m.repositoryRemoteActions(entry.repository, m.repositoryState(entry.repository)) {
					actions = append(actions, action{
						label:    a.label,
						disabled: a.disabled,
						run:      bind(func() tea.Cmd { return m.runRepositoryRemoteAction(entry.repository, a.id) }),
					})
				}
			}
		}
		if m.section == explorer {
			if entry != nil {
				if entry.folder != "" && strings.Contains(entry.label, "/") {
					add(m.tr("Split Folders"), sidebarKeys.SplitFolders.Help().Key, bind(m.splitFolderChain))
				}
				if !entry.header {
					add(m.tr("Copy"), sidebarCopySequence, bind(m.yankEntry))
				}
			}
			add(m.tr("Paste"), sidebarKeys.Paste.Help().Key, bind(m.pasteEntryPrompt))
			add(m.tr("New File…"), "", bind(func() tea.Cmd { return m.newEntry(false) }))
			add(m.tr("New Folder…"), "", bind(func() tea.Cmd { return m.newEntry(true) }))
			actions = append(actions, m.commandAction(commandRefresh, "Refresh", "", ""))
			add(m.tr("Collapse Folders"), "", m.collapseFolders)
		} else if m.section == changes && entry != nil {
			for _, a := range m.changeRowActions(*entry, m.sidebarContentWidth()) {
				actions = append(actions, action{label: a.label, key: m.changeRowShortcut(entry, a.id), disabled: a.disabled, run: bind(func() tea.Cmd { return m.runChangeRowAction(m.selected, a.id) })})
			}
			if entry.repoHeader {
				scope := *entry
				scope.staged = true
				shortcut := ""
				if m.stageScopeUnstages(entry) {
					shortcut = gitKeys.StageScope.Help().Key
				}
				actions = append(actions, action{
					label:    m.tr("Unstage All Changes"),
					key:      shortcut,
					disabled: m.changeActionDisabled(scope, "unstage", m.scopedChanges(scope)),
					run:      bind(func() tea.Cmd { return m.runScopeAction(scope, "unstage") }),
				})
				actions = append(actions, action{
					label:    m.tr("Discard All Changes"),
					key:      gitKeys.DiscardScope.Help().Key,
					disabled: m.discardScopeReason(entry),
					run:      bind(func() tea.Cmd { return m.runScopeAction(*entry, "discard") }),
				})
			}
		} else if m.section == search && entry != nil {
			add(m.tr("Dismiss Result"), sidebarDeleteSequence+" / "+sidebarKeys.Delete.Help().Key, bind(func() tea.Cmd { m.dismissSearchResult(m.search.selected); return nil }))
		} else if m.section == pullRequests {
			if entry != nil {
				if entry.repoHeader {
					add(m.tr("Filter Pull Requests"), "", m.openPullRequestFilter)
				}
				if entry.pullRequest != nil {
					add(m.tr("Open on GitHub"), sidebarKeys.OpenPullRequest.Help().Key, func() tea.Cmd { return m.openBrowser(entry.pullRequest.URL) })
				}
			}
			actions = append(actions, m.commandAction(commandRefresh, "Refresh", "", ""))
		}
		if m.section == changes && !m.snapshot.NoRepository {
			target := m.selectedRepository()
			add(m.tr("New Stash…"), "", func() tea.Cmd { return m.openStashModes(target) })
			add(m.tr("Manage Stashes…"), "", func() tea.Cmd { return m.openStashes(target) })
		}
		if entry != nil {
			if m.selectedEntryPath() != "" {
				label := m.tr("Delete…")
				if entry.folder != "" {
					label = m.tr("Delete Folder…")
				}
				add(label, sidebarDeleteSequence+" / "+sidebarKeys.Delete.Help().Key, bind(m.deleteSelectedEntry))
			}
			path := entryPath(entry)
			if path != "" {
				if entry.isSidebarFile() && !m.gitUnavailable {
					a := m.fileHistoryAction(path)
					a.run = bind(a.run)
					actions = append(actions, a)
				}
				actions = append(actions, m.copyPathActions(path)...)
			}
		}
	} else if x >= m.mainX() && x < m.mainX()+m.mainWidth() && !m.mainShowsSidebar() && y >= 0 && y < m.bodyHeight() {
		if !m.editorOnly && y == tabRow {
			actions = m.tabContextActions(x - m.mainX())
		} else {
			m.focusEditor()
			if m.activeTab >= 0 && m.activeTab < len(m.tabs) {
				if m.editing() {
					actions = append(actions, m.textEditorActions()...)
					actions = append(actions, m.commandAction(commandSave, "Save", "", ""), m.commandAction(commandFind, "Find…", "", ""))
				} else if m.diffOpen {
					if index := m.viewport.YOffset() + y - m.contentRow(); y >= m.contentRow() && index < len(m.diffRowHunks) {
						m.diffHunk = m.diffRowHunks[index]
					}
					actions = append(actions, m.reviewActions()...)
					actions = append(actions, m.textEditorActions()...)
					actions = append(actions, m.commandAction(commandDiffLayout, "Toggle Inline View", "", ""), m.commandAction(commandFind, "Find…", "", ""))
				} else if m.currentImage() != nil {
					add(m.tr("Fit to Window"), imageKeys.Fit.Help().Key, func() tea.Cmd { m.resetImageZoom(); return nil })
				}
				path := m.tabs[m.activeTab].path()
				if !m.editorOnly && !m.gitUnavailable {
					actions = append(actions, m.fileHistoryAction(path))
				}
				actions = append(actions, m.copyPathActions(path)...)
			}
		}
	}
	if len(actions) > 0 {
		m.contextMenu = &contextMenu{x: x, y: y, actions: actions}
	}
	return nil
}

func (m *Model) tabContextActions(x int) []action {
	for _, slot := range m.tabLayout() {
		if slot.control != 0 || x < slot.x || x >= slot.x+slot.width {
			continue
		}
		target := m.tabs[slot.index]
		targetKey := target.clickKey()
		actions := []action{
			m.commandAction(commandClose, "Close", "", targetKey),
			m.commandAction(commandCloseOthers, "Close Others", "", targetKey),
			m.commandAction(commandCloseAll, "Close All", "", ""),
		}
		if !m.gitUnavailable {
			actions = append(actions, m.fileHistoryAction(target.path()))
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
