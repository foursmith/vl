package app

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/colorprofile"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/vl/internal/config"
	"github.com/ourongxing/vl/internal/diff"
	"github.com/ourongxing/vl/internal/editor"
	"github.com/ourongxing/vl/internal/files"
	gitservice "github.com/ourongxing/vl/internal/git"
	"github.com/ourongxing/vl/internal/highlight"
	"github.com/ourongxing/vl/internal/i18n"
	"github.com/ourongxing/vl/internal/imagepreview"
	"github.com/ourongxing/vl/internal/platform"
	searchservice "github.com/ourongxing/vl/internal/search"
	"github.com/ourongxing/vl/internal/theme"
	"github.com/ourongxing/vl/internal/watcher"
)

type section int

const (
	changes section = iota
	explorer
	history
	search
	pullRequests
)

type item struct {
	branch          string
	label, key      string
	header          bool
	repository      string
	repoHeader      bool
	change          *gitservice.Change
	staged          bool
	commit          *gitservice.Commit
	historyPage     *historyPageState
	historyID       string
	historyChange   *gitservice.Change
	pullRequest     *gitservice.PullRequest
	pullRequestFile *diff.FilePatch
	file            string
	folder          string
	depth           int
	ignored         bool
	status          byte
}

type tab struct {
	findRanges findRangeCache
	scopeText  string
	scopes     []codeScope
	folds      []codeScope
	foldRows   []int
	image      *imageState

	vim              editor.Vim
	doc              *editor.Document
	diff             *diffState
	reloadSeq        uint64
	diskNotice       string
	reloadWhenClean  bool
	preview          bool
	positionBaseline *config.FilePosition
	top, left        int
	scrollText       string
	scrollWidth      int
	highlightText    string
	highlightSource  []string
	spans            [][]highlight.Span
	highlightJob     highlightJob
	highlighter      *highlight.Session
	renderKey        editorRenderKey
	renderedRows     map[int]editorRenderedRow
	guides           *editorGuides
	markdown         markdownLayout
	markdownImages   *markdownImageCache
	topImageOffset   int
}
type snapshotMsg struct {
	seq      int
	snapshot gitservice.Snapshot
	err      error
}
type diffMsg struct {
	target    *diffState
	seq       int
	raw       string
	prVersion string
	err       error
	images    *diffImages
}
type operationMsg struct {
	title string
	err   error
}
type tickMsg time.Time
type clipboardMsg struct {
	text string
	err  error
	doc  *editor.Document
	cut  bool
}

type Model struct {
	findWidget                         findWidgetState
	fileSearchOptionsDialog            bool
	navigationState                    navigationState
	leaderTarget                       string
	explorerYankRoot, explorerYankPath string
	flash                              *flashState
	vimRegister                        string
	flashRevision                      uint64
	displayLanguage                    i18n.Language
	kittySupported, kittyProbeDone     bool
	cellWidth, cellHeight              int
	imageIDs                           map[int]bool
	imageRetired                       []int
	imagePan                           *imagePan

	colors                  theme.Palette
	monochrome              bool
	colorTheme              *theme.Theme
	themeBeforePreview      *theme.Theme
	importedThemes          []*theme.Theme
	importedThemesSeq       uint64
	themeImporting          bool
	darkTheme               *theme.Theme
	lightTheme              *theme.Theme
	themePinned             bool
	statusBarStyle          string
	appearance              appearanceState
	layoutStore             *config.LayoutStore
	themeNotifications      bool
	themeNotificationsOwned bool
	ctx                     context.Context
	cancel                  context.CancelFunc
	watcher                 *watcher.Watcher
	watchDirty              bool
	repo                    *gitservice.Repository
	width, height           int
	nerd                    bool
	section                 section
	editorOnly              bool
	sidebar, sidebarVisible bool
	sidebarWidth            int
	sidebarRight            bool
	resizingSidebar         bool
	textDrag                *editor.Document
	diffDrag                *diffState
	snapshot                gitservice.Snapshot
	stickyItems             []item
	stickyTree              []codeScope
	items                   []item
	selected, sideTop       int
	refreshSeq              int
	refreshCancel           context.CancelFunc
	loading, busy           bool
	*diffState              // Bound to the active diff tab; documents use emptyDiff.
	emptyDiff               *diffState
	split                   bool
	lastFileRepository      string
	historyStates           map[string]*historyState
	historyPages            map[string]*historyPageState
	historySeq              int
	pullRequests            []gitservice.PullRequest
	openPullRequestCount    int
	pullRequestsNextCursor  string
	pullRequestsPageError   error
	pullRequestsSeq         uint64
	pullRequestsCancel      context.CancelFunc
	pullRequestFilter       gitservice.PullRequestFilter
	pullRequestsError       error
	pullRequestsFetchedAt   time.Time
	pullRequestReview       *pullRequestReview
	pullRequestReviews      map[string]*pullRequestReview
	pendingSession          *config.Session
	filePositionStore       *config.FilePositionStore
	tabs                    []tab
	activeTab               int
	languagePath            string
	languageInfo            highlight.LanguageInfo
	notice                  string
	contextMenu             *contextMenu
	modal, modalTitle       string
	input                   textinput.Model
	inputCaret              *tea.Cursor
	palette                 []action
	lastCommand             string
	pickerIndex             int
	pickerTop               int
	confirm                 func() tea.Cmd
	confirmationDetail      string
	confirmationLabel       string
	confirmationKey         string
	confirmationButton      int
	dialogTop               int
	search                  searchState
	findOptions             searchservice.Options
	findMatcherOptions      searchservice.Options
	findMatcher             *searchservice.Matcher
	findHidden              bool
	expanded                map[string]bool
	uncompressed            map[string]bool
	treeEntries             []files.Entry
	treeSeq                 int
	revealFile              string
	tabStart                int
	closeQueue              []*editor.Document
	lastClickKey            string
	lastClickTime           time.Time
	sidebarCollapsed        map[string]bool
	branchStart             string
	branchRepository        string
	commitRepository        string
	stashRepository         string
	commitDrafts            map[string]string
	openURL                 func(context.Context, string) error
	hoverX, hoverY          int
	hoverOnly               bool
	hoverRender             hoverRenderCache
	welcomeOrbit            welcomeOrbit
	pickerFilter            pickerFilterCache
	workspaceSeq            int
	workspaces              []string
	openingRepository       bool
	nextRepository          *gitservice.Repository
}

func New(ctx context.Context, repo *gitservice.Repository, nerd bool) *Model {
	return newModel(ctx, repo, nerd, false)
}

func newModel(ctx context.Context, repo *gitservice.Repository, nerd, editorOnly bool) *Model {
	ctx, cancel := context.WithCancel(ctx)
	input := newInput()
	dark, light := theme.Builtin("dark_modern"), theme.Builtin("light_modern")
	m := &Model{displayLanguage: i18n.English, cellWidth: 8, cellHeight: 16, colors: dark.Palette(), colorTheme: dark, darkTheme: dark, lightTheme: light, ctx: ctx, cancel: cancel, repo: repo, width: 120, height: 36, nerd: nerd, editorOnly: editorOnly, sidebar: !editorOnly, sidebarVisible: !editorOnly, split: true, diffState: newDiffState(), input: input, activeTab: -1, notice: "Loading repository…"}
	m.statusBarStyle = statusBarStandard
	m.snapshot.NoRepository = editorOnly || !repo.HasRepository()
	if editorOnly {
		m.notice = ""
	}
	m.search = newSearch()
	m.emptyDiff = m.diffState
	m.styleInput()
	m.resize()
	m.imageIDs = map[int]bool{}
	if !editorOnly {
		m.expanded = map[string]bool{}
		m.uncompressed = map[string]bool{}
		m.sidebarCollapsed = map[string]bool{}
		m.commitDrafts = map[string]string{}
		m.historyStates = map[string]*historyState{}
		m.historyPages = map[string]*historyPageState{}
	}
	m.section = explorer
	m.pullRequestFilter = gitservice.PullRequestOpen
	m.selected = -1
	m.openURL = platform.OpenURL
	m.hoverX, m.hoverY = -1, -1
	return m
}

func (m *Model) Init() tea.Cmd {
	commands := []tea.Cmd{m.scheduleMarkdownImages(), m.tick(), m.startWatcher(), m.probeImages(), m.pollSystemAppearance(false), tea.Raw(ansi.RequestModeLightDark)}
	if m.editing() {
		// Theme restoration can prepare highlighting before the event loop
		// starts. Queue a fresh request so startup always runs the command.
		m.tabs[m.activeTab].highlightJob.stop()
		commands = append(commands, m.scheduleHighlight())
	}
	if !m.editorOnly {
		commands = append(commands, m.refresh())
	}
	return tea.Batch(commands...)
}
func (m *Model) tick() tea.Cmd {
	delay := 3 * time.Second
	if m.watcher != nil {
		delay = 30 * time.Second
	}
	delay = m.pullRequestTickDelay(delay)
	return delayedMessage(m.ctx, delay, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *Model) refresh() tea.Cmd {
	if m.editorOnly {
		return nil
	}
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
	if m.snapshot.NoRepository && (m.section == changes || m.section == history || m.section == pullRequests) {
		m.selected = -1
		m.sideTop = 0
		return
	}
	switch m.section {
	case changes:
		m.items = m.changeRepositoryItems()
	case explorer:
		m.items = m.explorerItems()
	case history:
		m.items = m.historyRepositoryItems()
	case pullRequests:
		m.items = m.pullRequestItems()
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

func (m *Model) ensureSidebar() {
	l := m.sidebarList()
	if m.section == search {
		m.clampSearchList(l)
		l.ensureSelected()
		return
	}
	l.ensureSelected()
	h := l.height
	sticky := m.sidebarStickyRows()
	if m.selected >= m.sideTop && m.selected < m.sideTop+len(sticky) {
		m.sideTop = max(0, m.selected-stickyLimit(h))
	}
}

func (m *Model) clampSidebarScroll() {
	m.treeList().clampScroll()
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
				state.expanded = false
			}
			state.loading = false
			state.seq = 0
		}
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
	if s == explorer {
		return m.refreshTree()
	}
	if s == pullRequests {
		return m.refreshPullRequests(false)
	}
	return nil
}

func (m *Model) preview() tea.Cmd {
	entry := m.sidebarEntry()
	if entry == nil {
		return nil
	}
	cmd, _ := m.loadSidebarFile(*entry)
	return cmd
}

func (m *Model) Update(msg tea.Msg) (model tea.Model, next tea.Cmd) {
	// Normalize the Vim escape chord before any input handler sees it.
	if key, ok := msg.(tea.KeyPressMsg); ok && key.Mod.Contains(tea.ModCtrl) && key.String() == "ctrl+[" {
		msg = tea.KeyPressMsg{Code: tea.KeyEscape}
	}
	beforeDocument := m.document()
	beforeText := ""
	beforeSearchSeq := m.search.seq
	if beforeDocument != nil {
		beforeText = beforeDocument.Text
	}
	_, motion := msg.(tea.MouseMotionMsg)
	m.hoverOnly = motion && !m.resizingSidebar && !m.welcomeOrbit.dragging && m.textDrag == nil && m.diffDrag == nil && m.imagePan == nil
	if !m.hoverOnly {
		m.hoverRender = hoverRenderCache{}
		m.pickerFilter = pickerFilterCache{}
	}
	defer func() {
		if cmd := m.scheduleMarkdownImages(); cmd != nil {
			next = tea.Batch(next, cmd)
		}
		if m.editing() {
			m.syncMarkdownImageRows(&m.tabs[m.activeTab])
		}
		if cmd := m.flushRetiredImages(); cmd != nil {
			next = tea.Batch(next, cmd)
		}
		m.keepEditedTabs()
		if beforeDocument != nil && m.search.seq == beforeSearchSeq {
			after := m.document()
			changed := beforeDocument.Text != beforeText || after != nil && after.Path == beforeDocument.Path && after.Text != beforeText
			if changed {
				if cmd := m.refreshWorkspaceSearch(); cmd != nil {
					next = tea.Batch(next, cmd)
				}
			}
		}
		if !m.hoverOnly {
			if cmd := m.loadVisibleHistory(); cmd != nil {
				next = tea.Batch(next, cmd)
			}
			if cmd := m.loadVisiblePullRequests(); cmd != nil {
				next = tea.Batch(next, cmd)
			}
		}
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
		if m.diffOpen && !m.diffLoading && m.patch.Raw != "" && m.diffSyntax == nil && m.diffSyntaxCancel == nil {
			cmd := m.highlightDiff()
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
		if m.flash != nil && !m.flashValid() {
			m.cancelFlash()
		}
		// Maintenance can change visible state even during a mouse message.
		if next != nil {
			m.hoverOnly = false
			m.hoverRender = hoverRenderCache{}
		}
	}()
	if cmd, handled := m.contextMenuInput(msg); handled {
		return m, cmd
	}
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
		return m, nil

	case layoutSavedMsg:
		if msg.err != nil {
			m.notice = m.tr("Save preferences: %s", m.tr(msg.err.Error()))
		}
	case importedThemesMsg:
		m.applyImportedThemes(msg)
	case themesImportedMsg:
		return m, m.finishThemeImport(msg)
	case historyPageMsg:
		m.applyHistoryPage(msg)
	case cachedPullRequestsMsg:
		return m, m.applyCachedPullRequests(msg)
	case pullRequestsMsg:
		m.applyPullRequests(msg)
	case pullRequestDiffMsg:
		return m, m.applyPullRequestDiff(msg)
	case tea.ColorProfileMsg:
		m.monochrome = msg.Profile <= colorprofile.ASCII
	case markdownImageLoadedMsg:
		m.finishMarkdownImage(msg)
	case uv.KittyGraphicsEvent:
		m.imageCapability(msg)
	case imageProbeTimeoutMsg:
		m.kittyProbeDone = true
	case uv.CellSizeEvent:
		m.imagePan = nil
		if msg.Width > 0 && msg.Height > 0 && msg.Width <= 256 && msg.Height <= 256 &&
			(msg.Width != m.cellWidth || msg.Height != m.cellHeight) {
			m.cellWidth, m.cellHeight = msg.Width, msg.Height
			for i := range m.tabs {
				if im := m.tabs[i].image; im != nil {
					im.rows = nil
				}
				if d := m.tabs[i].diff; d != nil && d.images != nil {
					for _, im := range d.images.sides {
						if im != nil {
							im.rows = nil
						}
					}
				}
			}
		}
	case tea.ModeReportMsg:
		return m, m.themeNotificationMode(msg)
	case systemAppearanceMsg:
		var change tea.Cmd
		if msg.known {
			change = m.externalColorScheme(msg.light, "system")
		}
		return m, tea.Batch(change, m.pollSystemAppearance(true))
	case uv.DarkColorSchemeEvent:
		return m, m.externalColorScheme(false, "terminal")
	case uv.LightColorSchemeEvent:
		return m, m.externalColorScheme(true, "terminal")
	case workspacesMsg:
		if msg.seq != m.workspaceSeq {
			break
		}
		if msg.err != nil {
			if m.modal == "recent" {
				m.modal = ""
				m.input.Blur()
			}
			m.notice = m.tr("Failed to update workspace list: %s", m.tr(msg.err.Error()))
		} else {
			m.workspaces = msg.folders
			if m.modal == "recent" {
				m.populateRecentWorkspaces()
			}
		}
	case repositoryOpenedMsg:
		return m, m.finishOpenRepository(msg)
	case watchReadyMsg:
		if msg.err != nil {
			m.notice = m.tr("File watcher unavailable: %s", m.tr(msg.err.Error()))
			return m, nil
		}
		m.watcher = msg.watcher
		m.syncWatch()
		return m, m.nextWatch()
	case watchMsg:
		if !msg.open {
			m.watcher = nil
			return m, nil
		}
		return m, tea.Batch(m.handleWatch(msg.batch), m.nextWatch())
	case imageReloadMsg:
		m.finishImageReload(msg)
	case fileReloadMsg:
		m.finishReload(msg)
	case highlightStartMsg:
		return m, m.startHighlight(msg.request)
	case highlightResultMsg:
		m.finishHighlight(msg)
	case diffSyntaxMsg:
		if d := msg.target; d != nil && msg.seq == d.diffSeq && msg.raw == d.patch.Raw && msg.ctx.Err() == nil {
			d.diffSyntax = msg.spans
		}
	case tea.MouseMotionMsg:
		if m.welcomeOrbit.dragging {
			if msg.Button == tea.MouseLeft {
				m.dragWelcomeArt(msg.X, msg.Y)
			} else {
				m.welcomeOrbit.dragging = false
			}
			return m, nil
		}
		if m.imagePan != nil {
			if msg.Button == tea.MouseLeft && m.modal == "" {
				m.dragImage(msg.X, msg.Y)
			} else {
				m.imagePan = nil
			}
			return m, nil
		}
		m.hoverX, m.hoverY = msg.X, msg.Y
		if msg.Button == tea.MouseLeft {
			m.dragText(msg.X, msg.Y)
		} else {
			m.stopTextDrag()
		}
		if m.resizingSidebar {
			if msg.Button == tea.MouseLeft && m.modal == "" && m.sideShown() {
				m.dragSidebar(msg.X)
			} else {
				m.resizingSidebar = false
			}
		}
	case tea.MouseReleaseMsg:
		if m.welcomeOrbit.dragging {
			if msg.Button == tea.MouseLeft || msg.Button == tea.MouseNone {
				m.dragWelcomeArt(msg.X, msg.Y)
			}
			m.welcomeOrbit.dragging = false
			return m, nil
		}
		if m.imagePan != nil {
			if msg.Button == tea.MouseLeft || msg.Button == tea.MouseNone {
				m.dragImage(msg.X, msg.Y)
			}
			m.imagePan = nil
			return m, nil
		}
		if msg.Button == tea.MouseLeft || msg.Button == tea.MouseNone {
			m.dragText(msg.X, msg.Y)
		}
		m.stopTextDrag()
		if m.resizingSidebar {
			if msg.Button == tea.MouseLeft || msg.Button == tea.MouseNone {
				m.dragSidebar(msg.X)
			}
			m.resizingSidebar = false
		}
	case tea.BlurMsg:
		m.welcomeOrbit.dragging = false
		m.leaderTarget = ""
		m.imagePan = nil
		m.stopTextDrag()
		m.resizingSidebar = false
		m.hoverX, m.hoverY = -1, -1
	case browserMsg:
		if msg.err != nil {
			m.notice = m.tr("Failed to open browser: %s", m.tr(msg.err.Error()))
		} else {
			m.notice = m.tr("Opened %s", msg.target)
		}
	case treeMsg:
		if msg.seq != m.treeSeq {
			return m, nil
		}
		if msg.err != nil {
			m.revealFile = ""
			m.notice = m.tr(msg.err.Error())
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
		m.imagePan = nil
		m.resizingSidebar = false
		m.width, m.height = max(1, msg.Width), max(1, msg.Height)
		m.resize()
		return m, tea.Raw(ansi.WindowOp(16))
	case tickMsg:
		pullRequestRefresh := m.refreshPullRequests(false)
		if !m.loading && !m.busy {
			return m, tea.Batch(m.refresh(), m.reloadWatchedFiles(nil), pullRequestRefresh, m.tick())
		}
		return m, tea.Batch(pullRequestRefresh, m.tick())
	case snapshotMsg:
		if msg.seq != m.refreshSeq {
			return m, nil
		}
		m.loading = false
		if msg.err != nil {
			m.notice = m.tr(msg.err.Error())
			return m, nil
		}
		wasNoRepository := m.snapshot.NoRepository
		m.snapshot = msg.snapshot
		m.reconcileHistoryRepositories()
		m.syncWatch()
		m.rebuildItems()
		initial := m.notice == m.tr("Loading repository…")
		if initial {
			m.notice = ""
		}
		var commands []tea.Cmd
		if m.section == explorer {
			commands = append(commands, m.refreshTree())
		}
		commands = append(commands, m.refreshPullRequests(m.section == pullRequests && wasNoRepository))
		commands = append(commands, m.refreshDiffs(), m.restoreSession())
		if initial && len(m.tabs) == 0 && m.section == changes {
			commands = append(commands, m.preview())
		}
		return m, tea.Batch(commands...)

	case historyFilesMsg:
		m.applyHistoryFiles(msg)
	case diffMsg:
		return m, m.finishDiff(msg)

	case operationMsg:
		m.busy = false
		if msg.err != nil {
			m.notice = m.tr(msg.err.Error())
		} else {
			m.notice = m.tr("%s completed", m.tr(msg.title))
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
			m.notice = m.tr(msg.err.Error())
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
		m.welcomeOrbit.dragging = false
		m.leaderTarget = ""
		m.navigationState = navigationState{}
		m.cancelFlash()
		if m.editing() {
			m.tabs[m.activeTab].vim.CancelPending()
		}
		m.imagePan = nil
		m.stopTextDrag()
		if m.modal != "" {
			return m, m.clickModal(msg)
		}
		m.resizingSidebar = false
		if msg.Button == tea.MouseRight {
			if m.fileFindAt(msg.X, msg.Y) {
				return m, nil
			}
			return m, m.openContextMenu(msg.X, msg.Y)
		}
		if m.sidebarDividerAt(msg.X, msg.Y) {
			m.resizingSidebar = msg.Button == tea.MouseLeft
			return m, nil
		}
		if m.currentImage() != nil && msg.Button == tea.MouseLeft && m.imageContentAt(msg.X, msg.Y) {
			m.beginImagePan(msg.X, msg.Y)
			return m, nil
		}
		return m, m.click(msg)
	case tea.MouseWheelMsg:
		m.cancelFlash()
		if m.modal != "" {
			if m.isPicker() {
				m.scrollPicker(msg)
			}
			return m, nil
		}
		if m.currentImage() != nil && m.imageContentAt(msg.X, msg.Y) {
			if msg.Button == tea.MouseWheelUp {
				m.zoomImage(1.2, msg.X, msg.Y)
			} else if msg.Button == tea.MouseWheelDown {
				m.zoomImage(1/1.2, msg.X, msg.Y)
			}
			return m, nil
		}
		if msg.Button == tea.MouseWheelLeft || msg.Button == tea.MouseWheelRight ||
			msg.Mod.Contains(tea.ModShift) && (msg.Button == tea.MouseWheelUp || msg.Button == tea.MouseWheelDown) {
			if !m.sidebarAt(msg.X) {
				delta := 3
				if msg.Button == tea.MouseWheelLeft || msg.Button == tea.MouseWheelUp {
					delta = -delta
				}
				m.scrollHorizontal(delta)
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
			l := m.sidebarList()
			if msg.Y >= l.start {
				*l.top += delta
				l.clampScroll()
			}
		} else if m.editing() && m.activeTab >= 0 {
			delta := 3
			if msg.Button == tea.MouseWheelUp {
				delta = -3
			}
			t := &m.tabs[m.activeTab]
			t.setEditorDisplayTop(t.editorDisplayTop() + delta)
			m.clampEditorScroll(t)
		} else {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	case tea.KeyPressMsg:
		m.welcomeOrbit.dragging = false
		if m.flash != nil {
			return m, m.flashKey(msg)
		}
		if m.modal != "" {
			m.leaderTarget = ""
			m.navigationState = navigationState{}
			return m, m.updateModal(msg)
		}
		return m, m.key(msg)
	case tea.PasteMsg:
		if m.modal == "" && m.fileFindFocused() {
			return m, m.fileFindInput(msg)
		}
		if m.modal == "" && m.sidebar && m.section == search {
			return m, m.searchInput(msg)
		}
		if m.flash != nil {
			return m, m.flashText(msg.Content)
		}
		if m.modal != "" {
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			m.syncQuickPicker()
			m.selectPickerIndex(0)
			return m, cmd
		}
		if !m.sidebar && m.editing() && m.tabs[m.activeTab].vim.InsertMode {
			m.document().Insert(msg.Content)
			m.ensureEditor()
		}
	}
	return m, nil
}

func (m *Model) key(msg tea.KeyPressMsg) tea.Cmd {
	m.stopTextDrag()
	if m.sidebar && (m.section == history || m.section == pullRequests) {
		m.hoverX, m.hoverY = -1, -1
	}
	k := msg.String()
	if m.fileFindFocused() {
		if handled, cmd := m.fileFindKey(msg); handled {
			return cmd
		}
	}
	if !m.sidebar && m.fileFindHeight() > 0 && k == "alt+h" {
		return m.toggleFileReplacement()
	}
	if !m.sidebar && k == "shift+f3" {
		if !m.findWidget.workspaceMatch {
			m.presentFileFind()
		}
		m.navigateFileFind(-1)
		return nil
	}
	if m.editorOnly && key.Matches(msg, workbenchKeys.Recent, workbenchKeys.Search, workbenchKeys.Replace, workbenchKeys.Explorer, workbenchKeys.Changes, workbenchKeys.History, workbenchKeys.PullRequests, workbenchKeys.Sidebar, workbenchKeys.Focus, workbenchKeys.DiffLayout) {
		return nil
	}
	if m.sidebar && m.section == search {
		switch k {
		case "ctrl+q", "ctrl+p", "ctrl+h", "ctrl+shift+p", "f1", "ctrl+1", "ctrl+2", "ctrl+3", "ctrl+4", "ctrl+5", "ctrl+shift+f", "ctrl+shift+h", "ctrl+l", "f6", "ctrl+s", "ctrl+w", "alt+left", "alt+right":
		default:
			return m.searchKey(msg)
		}
	}
	if m.editing() && m.sidebar {
		m.tabs[m.activeTab].vim.CancelPending()
	}
	if handled, cmd := m.leaderKey(k); handled {
		return cmd
	}
	if m.rotateWelcomeArt(k) {
		return nil
	}
	if !m.sidebar && m.editorDocument() != nil {
		if handled, cmd := m.vimKey(k); handled {
			return cmd
		}
	}
	if m.sidebar && m.section == explorer && k == "p" {
		m.navigationState = navigationState{}
		return m.pasteEntryPrompt()
	}
	if m.sidebar && m.section == changes {
		if handled, cmd := m.changesKey(k); handled {
			m.navigationState = navigationState{}
			return cmd
		}
	}
	if handled, cmd := m.navigationKey(k); handled {
		return cmd
	}
	if key.Matches(msg, workbenchKeys.ContextMenu) {
		if m.sidebar {
			return m.openSidebarContextMenu()
		}
		if cursor := m.editorCursor(); cursor != nil {
			return m.openContextMenu(cursor.X, cursor.Y)
		}
		return m.openContextMenu(m.mainX()+2, m.contentRow())
	}
	switch {
	case key.Matches(msg, workbenchKeys.Quit, workbenchKeys.Copy):
		if key.Matches(msg, workbenchKeys.Copy) && !m.sidebar && (m.editing() || m.diffOpen) {
			return m.copy(false)
		}
		return m.quit()
	case key.Matches(msg, workbenchKeys.Files):
		return m.openQuickFiles()
	case key.Matches(msg, workbenchKeys.Recent):
		return m.openRecentWorkspaces()
	case key.Matches(msg, workbenchKeys.Commands):
		return m.openPalette()
	case key.Matches(msg, workbenchKeys.Position):
		return m.prompt("position", m.tr("Go to line · line:column"), "")
	case key.Matches(msg, workbenchKeys.Replace):
		return m.openReplace()
	case key.Matches(msg, workbenchKeys.Search):
		return m.openSearch()
	case key.Matches(msg, workbenchKeys.Explorer):
		return m.selectSection(explorer)
	case key.Matches(msg, workbenchKeys.Changes):
		return m.selectSection(changes)
	case key.Matches(msg, workbenchKeys.History):
		return m.selectSection(history)
	case key.Matches(msg, workbenchKeys.PullRequests):
		return m.selectSection(pullRequests)
	case key.Matches(msg, workbenchKeys.Sidebar):
		return m.toggleSidebar()
	case key.Matches(msg, workbenchKeys.Focus):
		if m.sidebar {
			m.blurFileFind()
			m.focusEditor()
		} else {
			m.sidebar = true
			m.sidebarVisible = true
			m.resize()
		}
		return nil
	case key.Matches(msg, workbenchKeys.Refresh), m.sidebar && key.Matches(msg, workbenchKeys.SidebarRefresh):
		return m.refreshView()
	case key.Matches(msg, workbenchKeys.DiffLayout):
		m.split = !m.split
		m.renderDiff()
		return nil
	case key.Matches(msg, workbenchKeys.Save):
		return m.runShortcut(shortcut{action: shortcutSave})
	case key.Matches(msg, workbenchKeys.Close):
		return m.runShortcut(shortcut{action: shortcutClose})
	case key.Matches(msg, workbenchKeys.NextTab):
		return m.switchTab(1)
	case key.Matches(msg, workbenchKeys.PreviousTab):
		return m.switchTab(-1)
	case key.Matches(msg, workbenchKeys.Find):
		return m.openFileFind()
	case key.Matches(msg, workbenchKeys.FindNext):
		return m.findNext()
	case key.Matches(msg, workbenchKeys.Escape):
		if m.editorOnly {
			return nil
		}
		m.sidebar = true
		m.sidebarVisible = true
		m.resize()
		return nil
	}
	if m.sidebar {
		if m.section == pullRequests && k != "enter" {
			if entry := m.current(); k == "o" && entry != nil && entry.pullRequest != nil {
				return m.openBrowser(entry.pullRequest.URL)
			}
			return nil
		}
		switch k {
		case "delete":
			return m.deleteSelectedEntry()
		case "enter":
			return m.openSelected()
		case "L", "shift+l":
			return m.splitFolderChain()
		case "e":
			return m.editSelected()
		}
		if m.section != changes {
			switch k {
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
		}
		return nil
	}
	if m.currentImage() != nil {
		switch k {
		case "+", "=", "-":
			factor := 1.25
			if k == "-" {
				factor = 1 / factor
			}
			m.zoomImage(factor, m.mainX()+m.mainWidth()/2, m.contentRow()+m.contentHeight()/2)
		case "0":
			m.resetImageZoom()
		case "enter":
			return m.keepTab()
		}
		return nil
	}
	if d := m.editorDocument(); d != nil {
		if m.tabs[m.activeTab].vim.InsertMode {
			d.BeginEditGroup()
		}
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
			if d.ReadOnly {
				return nil
			}
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
		if m.tabs[m.activeTab].vim.VisualMode {
			m.tabs[m.activeTab].vim.SyncVisual(d)
		}
		m.ensureTextCursor()
		return nil
	}
	if k == "enter" {
		return m.keepTab()
	}
	return nil
}

func (m *Model) document() *editor.Document {
	if m.activeTab < 0 || m.activeTab >= len(m.tabs) {
		return nil
	}
	return m.tabs[m.activeTab].doc
}

func (m *Model) save() tea.Cmd { return m.saveEditor(false) }

func (m *Model) saveEditor(closeAfter bool) tea.Cmd {
	if m.busy {
		m.notice = m.tr("Wait for the current Git operation before saving")
		return nil
	}
	d := m.document()
	if d == nil {
		m.notice = m.tr("Open a file to edit first")
		return nil
	}
	if err := d.Save(); err != nil {
		m.notice = m.tr(err.Error())
		return nil
	}
	m.notice = m.tr("Saved %s", filepath.Base(d.Path))
	refresh := m.refresh()
	if closeAfter {
		return tea.Batch(refresh, m.closeTab())
	}
	return refresh
}

func (m *Model) reloadCleanTabs() {
	var kept []tab
	newActive := -1
	for i, t := range m.tabs {
		wasActive := i == m.activeTab
		if t.image != nil {
			fresh, err := imagepreview.Open(m.ctx, t.image.Path)
			if err != nil {
				m.notice = m.tr(err.Error())
			} else {
				t.image.Image, t.image.rows = fresh, nil
			}
		}
		if t.doc != nil && !t.doc.Dirty() {
			t.clearHighlight()
			d, err := editor.Open(t.doc.Path)
			if err != nil {
				m.notice = m.tr("Closed %s: %s", filepath.Base(t.doc.Path), m.tr(err.Error()))
				continue
			}
			line, col := t.doc.Position()
			d.SetPosition(line, col)
			t.doc = d
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
		m.diffState = m.emptyDiff
	} else {
		m.selectTab(m.activeTab)
	}
	m.ensureEditor()
}

func (m *Model) copy(cut bool) tea.Cmd {
	d := m.document()
	text := ""
	if d != nil {
		text = d.SelectedText()
	} else if m.diffOpen && !cut {
		text = m.diffSelectedText()
	}
	if !cut {
		d = nil
	}
	return writeClipboard(text, d)
}

func writeClipboard(text string, cut *editor.Document) tea.Cmd {
	if text == "" {
		return nil
	}
	return func() tea.Msg {
		return clipboardMsg{text: text, err: clipboard.WriteAll(text), doc: cut, cut: cut != nil}
	}
}

func (m *Model) paste() tea.Cmd {
	d := m.document()
	return func() tea.Msg { text, err := clipboard.ReadAll(); return clipboardMsg{text: text, err: err, doc: d} }
}
