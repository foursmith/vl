package app

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/elegit/internal/diff"
	"github.com/ourongxing/elegit/internal/editor"
	"github.com/ourongxing/elegit/internal/files"
	gitservice "github.com/ourongxing/elegit/internal/git"
	"github.com/ourongxing/elegit/internal/highlight"
	"github.com/ourongxing/elegit/internal/platform"
	"github.com/ourongxing/elegit/internal/theme"
	"github.com/ourongxing/elegit/internal/watcher"
)

type section int

const (
	changes section = iota
	explorer
	history
	search
)

type item struct {
	branch        string
	label, key    string
	header        bool
	repository    string
	repoHeader    bool
	change        *gitservice.Change
	staged        bool
	commit        *gitservice.Commit
	historyID     string
	historyChange *gitservice.Change
	file          string
	folder        string
	depth         int
	ignored       bool
	status        byte
}

type tab struct {
	doc             *editor.Document
	reloadSeq       uint64
	diskNotice      string
	reloadWhenClean bool
	preview         bool
	top, left       int
	highlightText   string
	spans           [][]highlight.Span
	highlightJob    highlightJob
	searchHighlight editorSearchCache
	renderKey       editorRenderKey
	renderedRows    map[int]string
}
type snapshotMsg struct {
	seq      int
	snapshot gitservice.Snapshot
	err      error
}
type diffMsg struct {
	seq       int
	path, raw string
	err       error
}
type operationMsg struct {
	title, output string
	err           error
}
type tickMsg time.Time
type clipboardMsg struct {
	text string
	err  error
	doc  *editor.Document
	cut  bool
}

type Model struct {
	colors                  theme.Palette
	light                   bool
	themeMode               string
	terminalLight           bool
	themeNotifications      bool
	themeNotificationsOwned bool
	ctx                     context.Context
	cancel                  context.CancelFunc
	watcher                 *watcher.Watcher
	watchMetadata           []string
	watchDirty              bool
	repo                    *gitservice.Repository
	width, height           int
	nerd                    bool
	section                 section
	sidebar, sidebarVisible bool
	sidebarWidth            int
	sidebarRight            bool
	resizingSidebar         bool
	snapshot                gitservice.Snapshot
	items                   []item
	selected, sideTop       int
	refreshSeq, diffSeq     int
	refreshCancel           context.CancelFunc
	loading, busy           bool
	patch                   diff.Patch
	diffPath                string
	diffRepository          string
	lastFileRepository      string
	diffChange              *gitservice.Change
	diffStaged              bool
	diffHunk                int
	diffRowHunks            []int
	diffRows                []diff.Pair
	diffGutter              int
	diffLayout              diffLayoutKey
	diffLoading, split      bool
	diffCancel              context.CancelFunc
	diffSyntaxCancel        context.CancelFunc
	diffSyntax              map[*diff.Line][]highlight.Span
	diffEmphasis            map[*diff.Line]diffRange
	diffOpen                bool
	diffCommit              string
	historyCommit           string
	historyStates           map[string]*historyState
	historySeq              int
	viewport                viewport.Model
	tabs                    []tab
	activeTab               int
	showEditor              bool
	languagePath            string
	languageInfo            highlight.LanguageInfo
	notice                  string
	output                  []string
	outputVisible           bool
	modal, modalTitle       string
	input                   textinput.Model
	choice                  int
	palette                 []action
	confirm                 func() tea.Cmd
	confirmationDetail      string
	search                  searchState
	find                    string
	expanded                map[string]bool
	treeEntries             []files.Entry
	treeSeq                 int
	revealFile              string
	rootCollapsed           bool
	tabStart                int
	closeQueue              []*editor.Document
	lastClickKey            string
	lastClickTime           time.Time
	changeCollapsed         map[string]bool
	branchStart             string
	branchRepository        string
	commitRepository        string
	commitDrafts            map[string]string
	openURL                 func(context.Context, string) error
	hoverX, hoverY          int
	hoverOnly               bool
	hoverRender             hoverRenderCache
	recentSeq               int
	openingRepository       bool
	nextRepository          *gitservice.Repository
}

func New(ctx context.Context, repo *gitservice.Repository, nerd bool) *Model {
	ctx, cancel := context.WithCancel(ctx)
	input := textinput.New()
	input.Prompt = "> "
	input.CharLimit = 2000
	m := &Model{colors: theme.Dark(), ctx: ctx, cancel: cancel, repo: repo, width: 120, height: 36, nerd: nerd, sidebar: true, sidebarVisible: true, split: true, viewport: viewport.New(), input: input, diffHunk: -1, activeTab: -1, notice: "Loading repository…"}
	m.search = newSearch()
	m.styleInput()
	m.resize()
	m.expanded = map[string]bool{}
	m.changeCollapsed = map[string]bool{}
	m.commitDrafts = map[string]string{}
	m.historyStates = map[string]*historyState{}
	m.section = explorer
	m.selected = -1
	m.openURL = platform.OpenURL
	m.hoverX, m.hoverY = -1, -1
	return m
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.refresh(), m.tick(), m.startWatcher(), tea.Raw(ansi.RequestModeLightDark))
}
func (m *Model) tick() tea.Cmd {
	delay := 3 * time.Second
	if m.watcher != nil {
		delay = 30 * time.Second
	}
	return tea.Tick(delay, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *Model) refresh() tea.Cmd {
	if m.refreshCancel != nil {
		m.refreshCancel()
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.refreshCancel = cancel
	m.refreshSeq++
	seq := m.refreshSeq
	m.loading = true
	return func() tea.Msg { defer cancel(); s, err := m.repo.Snapshot(ctx); return snapshotMsg{seq, s, err} }
}

func (m *Model) rebuildItems() {
	key := ""
	selectedPath := ""
	if m.selected >= 0 && m.selected < len(m.items) {
		key = m.items[m.selected].key
		if c := m.items[m.selected].change; c != nil {
			selectedPath = c.Path
		}
	}
	m.items = nil
	switch m.section {
	case changes:
		m.items = m.changeRepositoryItems()
	case explorer:
		m.items = m.explorerItems()
	case history:
		m.items = m.historyRepositoryItems()
	}
	if m.selected >= 0 || m.section != explorer {
		m.selected = min(max(0, m.selected), max(0, len(m.items)-1))
	}
	found := false
	for i := range m.items {
		if m.items[i].key == key && key != "" {
			m.selected = i
			found = true
			break
		}
	}
	if !found && selectedPath != "" {
		for i, row := range m.items {
			if row.change != nil && row.change.Path == selectedPath {
				m.selected = i
				break
			}
		}
	}
	if !found && m.selected >= 0 && len(m.items) > 0 && m.items[m.selected].header {
		// Keep the initial file selection; explicitly selected group headers remain focused.
		for i, entry := range m.items {
			if !entry.header {
				m.selected = i
				break
			}
		}
	}
	if found {
		m.clampSidebarScroll()
	} else {
		m.ensureSidebar()
	}
}

func (m *Model) current() *item {
	if m.selected < 0 || m.selected >= len(m.items) {
		return nil
	}
	return &m.items[m.selected]
}

func (m *Model) moveSelection(delta int) {
	if m.selected < 0 && len(m.items) > 0 {
		m.selected = 0
		m.ensureSidebar()
		return
	}
	if i := m.selected + delta; i >= 0 && i < len(m.items) {
		m.selected = i
	}
	m.ensureSidebar()
}

func (m *Model) ensureSidebar() {
	h := max(1, m.bodyHeight()-sidebarHeaderRows)
	if m.selected < m.sideTop {
		m.sideTop = m.selected
	}
	if m.selected >= m.sideTop+h {
		m.sideTop = m.selected - h + 1
	}
	m.sideTop = max(0, m.sideTop)
}

func (m *Model) clampSidebarScroll() {
	height := max(1, m.bodyHeight()-sidebarHeaderRows)
	m.sideTop = min(max(0, m.sideTop), max(0, len(m.items)-height))
}

func (m *Model) selectSection(s section) tea.Cmd {
	return m.selectSectionInRepository(s, "")
}

func (m *Model) selectSectionInRepository(s section, repository string) tea.Cmd {
	m.revealFile = ""
	if m.section == history && s != history {
		for _, state := range m.historyStates {
			if state.cancel != nil {
				state.cancel()
				state.cancel = nil
			}
			state.loading = false
			state.seq = 0
		}
		m.historyCommit = ""
	}
	m.section = s
	m.sidebar = true
	m.sidebarVisible = true
	m.selected = -1
	m.sideTop = 0
	m.rebuildItems()
	if repository != "" {
		for i, entry := range m.items {
			if entry.repoHeader && entry.repository == repository {
				m.selected = i
				break
			}
		}
	}
	m.resize()
	m.ensureSidebar()
	if s == changes || s == history {
		return m.preview()
	}
	if s == explorer {
		return m.refreshTree()
	}
	return nil
}

func (m *Model) preview() tea.Cmd {
	return m.previewAt(true)
}

func (m *Model) previewAt(reset bool) tea.Cmd {
	entry := m.current()
	if entry == nil {
		return nil
	}
	if m.section == history {
		return m.previewHistory(*entry, reset)
	}
	if entry.change == nil || !reset && !m.diffOpen {
		return nil
	}
	c, staged := *entry.change, entry.staged
	m.diffCommit = ""
	m.diffChange = &c
	m.diffRepository = c.Repository
	m.diffStaged = staged
	return m.requestDiff(c.Path, reset, func(ctx context.Context) (string, error) {
		return m.repo.Diff(ctx, c, staged)
	})
}

func (m *Model) Update(msg tea.Msg) (model tea.Model, next tea.Cmd) {
	_, motion := msg.(tea.MouseMotionMsg)
	m.hoverOnly = motion && !m.resizingSidebar
	if !m.hoverOnly {
		m.hoverRender = hoverRenderCache{}
	}
	defer func() {
		m.keepEditedTabs()
		if cmd := m.reloadCleanConflicts(); cmd != nil {
			if next == nil {
				next = cmd
			} else {
				next = tea.Batch(next, cmd)
			}
		}
		if m.watchDirty && !m.loading && !m.busy {
			m.watchDirty = false
			cmd := m.refresh()
			if next == nil {
				next = cmd
			} else {
				next = tea.Batch(next, cmd)
			}
		}
		if cmd := m.scheduleHighlight(); cmd != nil {
			if next == nil {
				next = cmd
			} else {
				next = tea.Batch(next, cmd)
			}
		}
		// Maintenance can change visible state even during a mouse message.
		if next != nil {
			m.hoverOnly = false
			m.hoverRender = hoverRenderCache{}
		}
	}()
	switch msg := msg.(type) {
	case searchMsg:
		if msg.seq == m.search.seq {
			m.search.loading = false
			m.search.result = msg.result
			m.sortSearchFiles()
			if msg.err != nil {
				m.search.err = msg.err.Error()
			}
			m.clampSearch()
		}
	case tea.ModeReportMsg:
		return m, m.themeNotificationMode(msg)
	case uv.DarkColorSchemeEvent:
		return m, m.terminalColorScheme(false)
	case uv.LightColorSchemeEvent:
		return m, m.terminalColorScheme(true)
	case recentFoldersMsg:
		m.applyRecentFolders(msg)
	case repositoryOpenedMsg:
		return m, m.finishOpenRepository(msg)
	case watchReadyMsg:
		if msg.err != nil {
			m.log("文件监视不可用: " + msg.err.Error())
			return m, nil
		}
		m.watcher, m.watchMetadata = msg.watcher, msg.metadata
		m.syncWatch()
		return m, m.nextWatch()
	case watchMsg:
		if !msg.open {
			m.watcher = nil
			return m, nil
		}
		return m, tea.Batch(m.handleWatch(msg.batch), m.nextWatch())
	case fileReloadMsg:
		m.finishReload(msg)
	case highlightStartMsg:
		return m, m.startHighlight(msg.request)
	case highlightResultMsg:
		m.finishHighlight(msg)
	case diffSyntaxMsg:
		if msg.seq == m.diffSeq && msg.raw == m.patch.Raw && msg.ctx.Err() == nil {
			m.diffSyntax = msg.spans
		}
	case tea.MouseMotionMsg:
		m.hoverX, m.hoverY = msg.X, msg.Y
		if m.resizingSidebar {
			if msg.Button == tea.MouseLeft && m.modal == "" && m.sideShown() {
				m.dragSidebar(msg.X)
			} else {
				m.resizingSidebar = false
			}
		}
	case tea.MouseReleaseMsg:
		if m.resizingSidebar {
			if msg.Button == tea.MouseLeft || msg.Button == tea.MouseNone {
				m.dragSidebar(msg.X)
			}
			m.resizingSidebar = false
		}
	case tea.BlurMsg:
		m.resizingSidebar = false
		m.hoverX, m.hoverY = -1, -1
	case browserMsg:
		if msg.err != nil {
			m.log("打开浏览器失败: " + msg.err.Error())
		} else {
			m.notice = "已打开 " + msg.target
		}
	case treeMsg:
		if msg.seq != m.treeSeq {
			return m, nil
		}
		if msg.err != nil {
			m.revealFile = ""
			m.log(msg.err.Error())
			return m, nil
		}
		m.treeEntries = msg.entries
		m.syncWatch()
		if m.section == explorer {
			m.rebuildItems()
			if m.revealFile != "" {
				for i, entry := range m.items {
					if entry.file == m.revealFile {
						m.selected = i
						m.ensureSidebar()
						break
					}
				}
			}
		}
		m.revealFile = ""
	case tea.WindowSizeMsg:
		m.resizingSidebar = false
		m.width, m.height = max(1, msg.Width), max(1, msg.Height)
		m.resize()
	case tickMsg:
		if !m.loading && !m.busy {
			return m, tea.Batch(m.refresh(), m.reloadWatchedFiles(nil), m.tick())
		}
		return m, m.tick()
	case snapshotMsg:
		if msg.seq != m.refreshSeq {
			return m, nil
		}
		m.loading = false
		if msg.err != nil {
			m.log(msg.err.Error())
			return m, nil
		}
		changed := !reflect.DeepEqual(m.snapshot.Changes, msg.snapshot.Changes) || !reflect.DeepEqual(m.snapshot.Commits, msg.snapshot.Commits) || !reflect.DeepEqual(m.snapshot.RepositoryHistory, msg.snapshot.RepositoryHistory)
		m.snapshot = msg.snapshot
		m.reconcileHistoryRepositories()
		m.syncWatch()
		m.rebuildItems()
		initial := m.notice == "Loading repository…"
		if initial {
			m.notice = ""
		}
		if m.section == explorer {
			return m, m.refreshTree()
		}
		if (m.diffOpen || initial) && (m.section == changes || m.section == history) {
			if m.current() == nil {
				m.diffSeq++
				m.diffLoading = false
				m.diffChange = nil
				m.clearDiffContent()
			} else if changed || m.section == changes {
				wasEditor := m.showEditor
				cmd := m.previewAt(changed || initial)
				m.showEditor = wasEditor
				m.ensureTabVisible()
				return m, cmd
			}
		}
	case historyFilesMsg:
		return m, m.applyHistoryFiles(msg)
	case diffMsg:
		if msg.seq != m.diffSeq {
			return m, nil
		}
		m.diffLoading = false
		if msg.err != nil {
			m.log(msg.err.Error())
			return m, nil
		}
		if msg.raw == m.patch.Raw {
			m.diffPath = msg.path
			m.renderDiff()
			if m.diffSyntax == nil {
				return m, m.highlightDiff()
			}
			return m, nil
		}
		m.patch = diff.Parse(msg.raw)
		m.diffPath = msg.path
		if len(m.patch.Hunks) > 0 {
			m.diffHunk = min(max(0, m.diffHunk), len(m.patch.Hunks)-1)
		} else {
			m.diffHunk = -1
		}
		m.renderDiff()
		return m, m.highlightDiff()
	case operationMsg:
		m.busy = false
		if msg.err != nil {
			m.log(msg.err.Error())
		} else {
			m.log(msg.title + " completed")
			if strings.TrimSpace(msg.output) != "" {
				m.addOutput(msg.output)
			}
		}
		if msg.title == "Commit" && msg.err == nil {
			delete(m.commitDrafts, m.commitRepository)
		}
		if msg.title == "Switch branch" || msg.title == "Create branch" || msg.title == "Pull" || msg.title == "Sync" || msg.title == "Stash" || msg.title == "Stash pop" || msg.title == "Discard" {
			m.reloadCleanTabs()
		}
		return m, m.refresh()
	case clipboardMsg:
		if msg.err != nil {
			m.log(msg.err.Error())
		} else if msg.doc != nil && m.document() == msg.doc {
			if msg.cut {
				if msg.doc.SelectedText() == msg.text {
					msg.doc.Insert("")
				}
			} else {
				msg.doc.Insert(msg.text)
			}
			m.ensureEditor()
		}
	case tea.MouseClickMsg:
		if m.modal != "" {
			return m, m.clickModal(msg)
		}
		m.resizingSidebar = false
		if m.sidebarDividerAt(msg.X, msg.Y) {
			m.resizingSidebar = msg.Button == tea.MouseLeft
			return m, nil
		}
		return m, m.click(msg)
	case tea.MouseWheelMsg:
		if m.modal != "" {
			if m.isPicker() {
				delta := 1
				if msg.Button == tea.MouseWheelUp {
					delta = -1
				}
				m.choice = min(max(0, len(m.filteredActions())-1), max(0, m.choice+delta))
			}
			return m, nil
		}
		if msg.Button != tea.MouseWheelUp && msg.Button != tea.MouseWheelDown {
			return m, nil
		}
		if m.sidebarAt(msg.X) {
			delta := 3
			if msg.Button == tea.MouseWheelUp {
				delta = -3
			}
			if m.section == search {
				m.search.top += delta
				m.clampSearch()
				return m, nil
			}
			m.sideTop += delta
			m.clampSidebarScroll()
		} else if m.showEditor && m.activeTab >= 0 {
			delta := 3
			if msg.Button == tea.MouseWheelUp {
				delta = -3
			}
			t := &m.tabs[m.activeTab]
			t.top += delta
			m.clampEditorScroll(t)
		} else {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	case tea.KeyPressMsg:
		if m.modal != "" {
			return m, m.updateModal(msg)
		}
		return m, m.key(msg)
	case tea.PasteMsg:
		if m.modal == "" && m.sidebar && m.section == search {
			return m, m.searchInput(msg)
		}
		if m.modal != "" {
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			m.choice = 0
			return m, cmd
		}
		if !m.sidebar && m.showEditor && m.document() != nil {
			m.document().Insert(msg.Content)
			m.ensureEditor()
		}
	}
	return m, nil
}

func (m *Model) key(msg tea.KeyPressMsg) tea.Cmd {
	k := msg.String()
	if m.sidebar && m.section == search {
		switch k {
		case "ctrl+q", "ctrl+c", "ctrl+p", "ctrl+r", "ctrl+shift+p", "f1", "ctrl+1", "ctrl+2", "ctrl+3", "ctrl+4", "ctrl+shift+f", "ctrl+shift+h", "ctrl+b", "f6", "ctrl+s", "ctrl+w", "alt+left", "alt+right":
		default:
			return m.searchKey(msg)
		}
	}
	switch k {
	case "ctrl+q", "ctrl+c":
		if k == "ctrl+c" && !m.sidebar && m.showEditor {
			return m.copy(false)
		}
		return m.quit()
	case "ctrl+p":
		return m.openQuickFiles()
	case "ctrl+r":
		return m.openRecentFolders()
	case "ctrl+shift+p", "f1":
		return m.openPalette()
	case "ctrl+g":
		return m.prompt("position", "转到行 · 行:列", "")
	case "ctrl+shift+f", "ctrl+4":
		return m.openSearch(false)
	case "ctrl+shift+h":
		return m.openSearch(true)
	case "ctrl+1":
		return m.selectSection(explorer)
	case "ctrl+2":
		return m.selectSection(changes)
	case "ctrl+3":
		return m.selectSection(history)
	case "ctrl+b":
		m.sidebarVisible = !m.sidebarVisible
		if !m.sidebarVisible {
			m.sidebar = false
		}
		m.resize()
		return nil
	case "f6":
		m.sidebar = !m.sidebar
		if m.sidebar {
			m.sidebarVisible = true
			m.resize()
		}
		return nil
	case "ctrl+j":
		m.outputVisible = !m.outputVisible
		m.resize()
		return nil
	case "f5":
		if !m.busy {
			return m.refresh()
		}
		return nil
	case "ctrl+d":
		m.split = !m.split
		m.renderDiff()
		return nil
	case "ctrl+s":
		return m.save()
	case "ctrl+w":
		return m.closeTab()
	case "alt+right":
		return m.switchTab(1)
	case "alt+left":
		return m.switchTab(-1)
	case "ctrl+f":
		return m.prompt("find", "Find in current view", m.find)
	case "f3":
		return m.findNext()
	case "esc":
		m.sidebar = true
		m.sidebarVisible = true
		m.resize()
		return nil
	}
	if m.sidebar {
		switch k {
		case "up", "k":
			m.moveSelection(-1)
			return m.preview()
		case "down", "j":
			m.moveSelection(1)
			return m.preview()
		case "enter":
			return m.openSelected()
		case "right":
			if entry := m.current(); m.section == history && entry != nil && entry.commit != nil && m.historyExpanded(historyKey(entry.repository, entry.commit.ID)) {
				return nil
			}
			if entry := m.current(); entry != nil && (entry.header || entry.folder != "") && m.folderExpanded(*entry) {
				return nil
			}
			return m.openSelected()
		case "left":
			if entry := m.current(); m.section == history && entry != nil && entry.commit != nil {
				if m.historyExpanded(historyKey(entry.repository, entry.commit.ID)) {
					return m.openHistory(*entry)
				}
				m.selectHistoryParent(*entry)
				return nil
			}
			if entry := m.current(); entry != nil && (entry.header || entry.folder != "") && m.folderExpanded(*entry) {
				return m.openSelected()
			}
			m.selectParentFolder()
			return nil
		case "e":
			return m.editSelected()
		case "space":
			entry := m.current()
			return m.stageSelected(entry != nil && entry.staged)
		case "s":
			return m.stageSelected(false)
		case "u":
			return m.stageSelected(true)
		case "c":
			return m.commit()
		case "a":
			return m.stageAll()
		}
		return nil
	}
	if m.showEditor && m.document() != nil {
		d := m.document()
		switch k {
		case "ctrl+z":
			d.Undo()
		case "ctrl+y", "ctrl+shift+z":
			d.Redo()
		case "ctrl+a":
			d.SelectAll()
		case "ctrl+x":
			return m.copy(true)
		case "ctrl+v":
			return m.paste()
		case "backspace":
			d.Delete(true)
		case "delete":
			d.Delete(false)
		case "enter":
			d.Insert("\n")
		case "tab":
			d.Insert("\t")
		case "pgup", "pgdown":
			dir := "up"
			if k == "pgdown" {
				dir = "down"
			}
			for i := 0; i < m.contentHeight(); i++ {
				d.Move(dir, false)
			}
		default:
			base := strings.ReplaceAll(k, "shift+", "")
			switch base {
			case "left", "right", "up", "down", "home", "end", "ctrl+home", "ctrl+end", "ctrl+left", "ctrl+right":
				d.Move(base, strings.Contains(k, "shift+"))
			default:
				if msg.Text != "" {
					d.Insert(msg.Text)
				}
			}
		}
		m.ensureEditor()
		return nil
	}
	switch k {
	case "]", "n":
		m.nextHunk(1)
		return nil
	case "[", "p":
		m.nextHunk(-1)
		return nil
	case "s":
		return m.stageHunk(false)
	case "u":
		return m.stageHunk(true)
	case "e":
		return m.editDiff()
	case "tab":
		m.split = !m.split
		m.renderDiff()
		return nil
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return cmd
}

func (m *Model) document() *editor.Document {
	if m.activeTab < 0 || m.activeTab >= len(m.tabs) {
		return nil
	}
	return m.tabs[m.activeTab].doc
}

func (m *Model) save() tea.Cmd {
	if m.busy {
		m.notice = "Wait for the current Git operation before saving"
		return nil
	}
	d := m.document()
	if !m.showEditor || d == nil {
		m.notice = "Open a file to edit first"
		return nil
	}
	if err := d.Save(); err != nil {
		m.log(err.Error())
		return nil
	}
	m.notice = "Saved " + filepath.Base(d.Path)
	return m.refresh()
}

func (m *Model) reloadCleanTabs() {
	active := m.document()
	var kept []tab
	newActive := -1
	for _, t := range m.tabs {
		wasActive := t.doc == active
		if !t.doc.Dirty() {
			t.highlightJob.stop()
			d, err := editor.Open(t.doc.Path)
			if err != nil {
				m.log("Closed " + filepath.Base(t.doc.Path) + ": " + err.Error())
				continue
			}
			line, col := t.doc.Position()
			d.SetPosition(line, col)
			t.doc = d
			t.highlightText = ""
			t.spans = nil
		}
		if wasActive {
			newActive = len(kept)
		}
		kept = append(kept, t)
	}
	m.tabs = kept
	m.activeTab = newActive
	if m.activeTab < 0 && len(kept) > 0 {
		m.activeTab = 0
	}
	if len(kept) == 0 {
		m.showEditor = false
	}
	if m.showEditor && m.document() != nil {
		m.lastFileRepository = m.fileRepository(m.document().Path)
	}
	m.ensureEditor()
}

func (m *Model) log(text string) {
	m.notice = text
	m.addOutput(time.Now().Format("15:04:05") + "  " + text)
}

// Bound stored output by displayed lines, and detach the retained tail from
// large command buffers so scrolling never rescans an entire command log.
func (m *Model) addOutput(text string) {
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if len(lines) > 200 {
		lines = lines[len(lines)-200:]
	}
	for _, line := range lines {
		m.output = append(m.output, strings.Clone(line))
	}
	if len(m.output) > 200 {
		drop := len(m.output) - 200
		clear(m.output[:drop])
		m.output = m.output[drop:]
	}
}

func (m *Model) copy(cut bool) tea.Cmd {
	d := m.document()
	if d == nil {
		return nil
	}
	text := d.SelectedText()
	if text == "" {
		return nil
	}
	if !cut {
		d = nil
	}
	return func() tea.Msg { return clipboardMsg{text: text, err: clipboard.WriteAll(text), doc: d, cut: cut} }

}
func (m *Model) paste() tea.Cmd {
	d := m.document()
	return func() tea.Msg { text, err := clipboard.ReadAll(); return clipboardMsg{text: text, err: err, doc: d} }
}
