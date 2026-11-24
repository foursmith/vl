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
	"github.com/foursmith/vl/internal/config"
	"github.com/foursmith/vl/internal/diff"
	"github.com/foursmith/vl/internal/editor"
	"github.com/foursmith/vl/internal/files"
	gitservice "github.com/foursmith/vl/internal/git"
	"github.com/foursmith/vl/internal/highlight"
	"github.com/foursmith/vl/internal/i18n"
	"github.com/foursmith/vl/internal/imagepreview"
	"github.com/foursmith/vl/internal/platform"
	searchservice "github.com/foursmith/vl/internal/search"
	"github.com/foursmith/vl/internal/theme"
	"github.com/foursmith/vl/internal/watcher"
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
	syntax           highlight.LanguageInfo
	syntaxRecord     string
	syntaxPending    bool
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
type workspaceFilesMsg struct {
	seq   int
	paths []string
	err   error
}
type diffMsg struct {
	target    *diffState
	seq       int
	raw       string
	prVersion string
	err       error
	images    *diffImages
	oldSource string
}
type operationMsg struct {
	title      string
	err        error
	indexDiffs []*diffState
}
type statusFeedback struct{ text string }

type statusFeedbackExpiredMsg struct{ feedback *statusFeedback }

type tickMsg time.Time
type clipboardMsg struct {
	text string
	err  error
	doc  *editor.Document
	cut  bool
}

type Model struct {
	workbenchKeymap                    *workbenchKeymap
	findWidget                         findWidgetState
	fileSearchOptionsDialog            bool
	navigationState                    navigationState
	leaderTarget                       string
	explorerYankRoot, explorerYankPath string
	flash                              *flashState
	pathCompletion                     *pathCompletion
	vimRegister                        string
	flashRevision                      uint64
	displayLanguage                    i18n.Language
	kittySupported, kittyProbeDone     bool
	cellWidth, cellHeight              int
	imageIDs                           map[int]bool
	imageRetired                       []int
	imagePan                           *imagePan
	imageRaster                        *imageRasterRequest
	imageUse                           uint64

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
	watchFilesDirty         bool
	autoFetch               autoFetchState
	terminalBlurred         bool
	autoSwitchInputMethod   config.InputMethodSwitch
	vimHanDictionary        func() *editor.HanDictionary
	repo                    *gitservice.Repository
	width, height           int
	nerd                    bool
	section                 section
	editorOnly              bool
	sidebar, sidebarVisible bool
	sidebarWidth            int
	sidebarRight            bool
	explorerPanel           explorerPanelID
	outlineFocused          bool
	resizingSidebar         bool
	textDrag                *editor.Document
	diffDrag                *diffState
	snapshot                gitservice.Snapshot
	gitUnavailable          bool
	stickyItems             []item
	stickyTree              []codeScope
	items                   []item
	selected, sideTop       int
	refreshSeq              int
	refreshCancel           context.CancelFunc
	fileListSeq             int
	fileListCancel          context.CancelFunc
	fileListLoading         bool
	fileListIncludeIgnored  bool
	loading, busy           bool
	remoteProgress          *remoteProgress
	operationProgress       string
	*diffState              // Bound to the active diff tab; documents use emptyDiff.
	emptyDiff               *diffState
	split                   bool
	lastFileRepository      string
	historyStates           map[string]*historyState
	fileHistory             *fileHistoryState
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
	statusFeedback          *statusFeedback
	contextMenu             *contextMenu
	modal, modalTitle       string
	input                   textinput.Model
	inputCaret              *tea.Cursor
	palette                 []action
	lastCommand             string
	pickerIndex             int
	pickerTop               int
	confirmationActions     []confirmationAction
	confirmationDetail      string
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
	changeFilePositionSeq   int
	revealFile              string
	tabStart                int
	closeQueue              []*editor.Document
	lastClickKey            string
	lastClickTime           time.Time
	sidebarCollapsed        map[string]bool
	branchStart             string
	branchRepository        string
	githubPublish           *githubPublishPicker
	commitRepository        string
	stashRepository         string
	stashOption             string
	stashPickerSeq          int
	commitDrafts            map[string]string
	openURL                 func(context.Context, string) error
	hoverX, hoverY          int
	hoverOnly               bool
	renderFrame             frameCache
	welcomeOrbit            welcomeOrbit
	welcomeArt              welcomeArtCache
	pickerFilter            pickerFilterCache
	quickFilesFilter        quickFilesFilter
	workspaceSeq            int
	removingWorkspace       bool
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
	m.gitUnavailable = !editorOnly && !gitservice.Available()
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
		m.autoFetch.next = time.Now()
		commands = append(commands, m.refresh())
		if m.pendingSession != nil && len(m.pendingSession.Tabs) > 0 {
			commands = append(commands, func() tea.Msg { return restoreSessionMsg{} })
		}
	}
	return tea.Batch(commands...)
}
func (m *Model) tick() tea.Cmd {
	delay := 3 * time.Second
	if m.watcher != nil {
		delay = 30 * time.Second
	}
	if remaining := time.Until(m.autoFetch.next); remaining > 0 && !m.terminalBlurred {
		delay = min(delay, remaining)
	}
	delay = m.pullRequestTickDelay(delay)
	return delayedMessage(m.ctx, delay, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *Model) refresh() tea.Cmd {
	m.invalidateDiffs()
	return m.refreshRepository(true)
}

func (m *Model) refreshRepository(includeFiles bool) tea.Cmd {
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
	repo := m.repo
	commands := []tea.Cmd{
		func() tea.Msg {
			defer cancel()
			snapshot, err := repo.StatusSnapshot(ctx)
			return snapshotMsg{seq, snapshot, err}
		},
	}
	if includeFiles {
		commands = append(commands, m.refreshWorkspaceFiles())
	}
	if m.section == explorer {
		// Explorer reads the visible directories. The snapshot supplies Git
		// decorations separately when the full worktree scan finishes.
		commands = append(commands, m.refreshTree())
	}
	return tea.Batch(commands...)
}

func (m *Model) rebuildItems() {
	var previous item
	if current := m.current(); current != nil {
		previous = *current
	}
	m.items = nil
	if m.repositoryEmptyState().title != "" {
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
		if m.items[i].key == previous.key && previous.key != "" {
			m.selected = i
			found = true
			break
		}
	}
	if !found && previous.key != "" && m.section == changes {
		if index := m.changesSelectionAfterRefresh(previous); index >= 0 {
			m.selected = index
			m.ensureSidebar()
			return
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
	if s != explorer {
		m.stopFileHistory()
	}
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
	m.outlineFocused = false
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
	_, wheel := msg.(tea.MouseWheelMsg)
	var scrolled scrollRegion
	var timelineContent []string
	var timelineTab string
	if click, ok := msg.(tea.MouseClickMsg); ok && click.Button == tea.MouseLeft &&
		m.modal == "" && m.contextMenu == nil && m.flash == nil && m.textDrag == nil && m.diffDrag == nil &&
		m.sidebarAt(click.X) && (m.editing() && m.tabs[m.activeTab].markdownImages == nil ||
		m.diffOpen && m.diffCommit != "" && m.images == nil) {
		if panel, ok := m.explorerPanelAt(click.Y); ok && panel.spec.id == explorerPanelTimeline {
			timelineContent = m.renderFrame.content
			timelineTab = m.fileHistoryAnchor()
		}
	}
	if !m.hoverOnly {
		if !wheel {
			m.renderFrame = frameCache{}
		}
		m.pickerFilter = pickerFilterCache{}
	}
	defer func() {
		if m.pathCompletion != nil && !m.validPathCompletion() {
			m.pathCompletion = nil
		}
		if cmd := m.scheduleMarkdownImages(); cmd != nil {
			next = tea.Batch(next, cmd)
		}
		if m.editing() {
			m.syncMarkdownImageRows(&m.tabs[m.activeTab])
		}
		if cmd := m.flushRetiredImages(); cmd != nil {
			next = tea.Batch(next, cmd)
		}
		if m.keepEditedTabs() {
			scrolled = 0
		}
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
			next = tea.Batch(next, m.syncFileHistory(), m.startAutoFetch())
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
		if m.watchDirty && !m.loading && !m.busy && (!m.watchFilesDirty || !m.fileListLoading) {
			next = tea.Batch(next, m.refreshRepository(m.watchFilesDirty))
			m.watchDirty, m.watchFilesDirty = false, false
		}
		if cmd := m.refreshActiveDiff(); cmd != nil {
			next = tea.Batch(next, cmd)
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
		if m.notice != "" {
			m.statusFeedback = nil
		}
		// Maintenance can change visible state even during a mouse message.
		if next != nil {
			m.hoverOnly = false
			m.renderFrame = frameCache{}
		} else if wheel {
			m.renderFrame.afterScroll(scrolled)
		}
		if timelineContent != nil && !m.mainShowsSidebar() {
			if m.fileHistoryAnchor() == timelineTab {
				m.renderFrame = frameCache{content: timelineContent, reuseRegions: true}
			}
		}
	}()
	if cmd, handled := m.contextMenuInput(msg); handled {
		return m, cmd
	}
	switch msg := msg.(type) {
	case searchMsg:
		return m, m.applySearchResults(msg)

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
	case fileHistoryMsg:
		m.applyFileHistory(msg)
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
	case markdownRemoteImageMsg:
		m.finishMarkdownRemoteImage(msg)
	case markdownRemoteCacheMsg:
		m.finishMarkdownRemoteCache(msg)
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
		m.removingWorkspace = false
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
	case imageRasterResult:
		m.finishImageRaster(msg)
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
	case tea.FocusMsg:
		m.terminalBlurred = false
	case tea.BlurMsg:
		m.terminalBlurred = true
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
			m.showStatusFeedback(m.tr("Opened %s", msg.target))
		}
	case restoreSessionMsg:
		return m, m.restoreSession()
	case treeMsg:
		if msg.seq != m.treeSeq {
			return m, nil
		}
		if msg.err != nil {
			m.revealFile = ""
			m.notice = m.tr(msg.err.Error())
			return m, nil
		}
		// Keep existing decorations visible until this read's Git reply arrives.
		ignored := make(map[string]bool)
		for _, entry := range m.treeEntries {
			if entry.Ignored {
				ignored[entry.Path] = true
			}
		}
		m.treeEntries = msg.entries
		for i := range m.treeEntries {
			m.treeEntries[i].Ignored = ignored[m.treeEntries[i].Path]
		}
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
		return m, m.refreshTreeIgnored(msg.seq)
	case treeIgnoredMsg:
		if msg.seq != m.treeSeq {
			return m, nil
		}
		if msg.err != nil {
			m.notice = m.tr(msg.err.Error())
			return m, nil
		}
		changed := false
		for i := range m.treeEntries {
			entry := &m.treeEntries[i]
			ignored := msg.ignored[entry.Path]
			changed = changed || entry.Ignored != ignored
			entry.Ignored = ignored
		}
		if changed && m.section == explorer {
			m.rebuildItems()
		}
	case tea.WindowSizeMsg:
		m.SetTerminalSize(msg.Width, msg.Height)
		return m, tea.Raw(ansi.WindowOp(16))
	case autoFetchFinishedMsg:
		m.finishAutoFetch(msg)
	case tickMsg:
		pullRequestRefresh := m.refreshPullRequests(false)
		if !m.loading && !m.fileListLoading && !m.busy {
			return m, tea.Batch(m.refresh(), m.reloadWatchedFiles(nil), pullRequestRefresh, m.tick())
		}
		return m, tea.Batch(pullRequestRefresh, m.tick())
	case workspaceFilesMsg:
		if msg.seq != m.fileListSeq {
			return m, nil
		}
		m.fileListLoading = false
		if msg.err != nil {
			m.notice = m.tr(msg.err.Error())
			return m, nil
		}
		first := len(m.snapshot.Files) == 0
		m.snapshot.Files = msg.paths
		if first && len(msg.paths) > 0 && m.modal == "quick" && !m.commandPicker() {
			m.palette = nil
			m.syncQuickPicker()
			m.syncPicker()
		}
	case snapshotMsg:
		if msg.seq != m.refreshSeq {
			return m, nil
		}
		m.loading = false
		if msg.err != nil {
			for _, tab := range m.tabs {
				if d := tab.diff; d != nil && d.waitingForSnapshot {
					d.diffLoading = false
				}
			}
			m.notice = m.tr(msg.err.Error())
			return m, nil
		}
		previous := m.snapshot
		msg.snapshot.Files = previous.Files
		m.snapshot = msg.snapshot
		m.reconcileChangeDiffs(previous)
		m.reconcileIndexDiffTabs()
		m.reconcileHistoryRepositories()
		m.syncWatch()
		m.rebuildItems()
		initial := m.notice == m.tr("Loading repository…")
		if initial {
			m.notice = ""
		}
		remoteAdded := (previous.NoRepository || previous.Branch != "") && len(previous.Remotes) == 0 && len(m.snapshot.Remotes) > 0
		commands := []tea.Cmd{m.refreshPullRequests(m.section == pullRequests && remoteAdded), m.restoreSession()}
		for _, tab := range m.tabs {
			if d := tab.diff; d != nil && d.waitingForSnapshot {
				commands = append(commands, m.loadRestoredWorktreeDiff(d))
			}
		}
		if initial && len(m.tabs) == 0 && m.section == changes {
			commands = append(commands, m.preview())
		}
		return m, tea.Batch(commands...)

	case historyFilesMsg:
		m.applyHistoryFiles(msg)
	case diffMsg:
		if m.fileHistory != nil && m.fileHistory.pending != nil && m.fileHistory.pending == msg.target {
			return m, m.finishFileHistoryDiff(msg)
		}
		return m, m.finishDiff(msg)
	case changeFilePositionMsg:
		m.finishChangeFilePosition(msg)

	case remoteProgressFrameMsg:
		if progress := msg.progress; progress != nil && m.remoteProgress == progress {
			progress.frame++
			return m, progress.tick(120 * time.Millisecond)
		}
	case statusFeedbackExpiredMsg:
		if msg.feedback == m.statusFeedback {
			m.statusFeedback = nil
		}
	case stashPickerMsg:
		return m, m.finishStashPicker(msg)
	case githubPublishOwnerMsg:
		m.finishGitHubPublishOwner(msg)
	case operationMsg:
		m.busy = false
		m.operationProgress = ""
		if progress := m.remoteProgress; progress != nil {
			progress.cancel()
			m.remoteProgress = nil
		}
		if msg.err != nil {
			m.notice = m.tr(msg.err.Error())
		} else {
			m.showStatusFeedback(m.tr("%s completed", m.tr(msg.title)))
		}
		for _, d := range msg.indexDiffs {
			if d.diffOpen {
				d.followIndex = true
			}
		}
		if msg.title == "Commit" && msg.err == nil {
			delete(m.commitDrafts, m.commitRepository)
		}
		switch msg.title {
		case "Switch branch", "Create branch", "Pull", "Sync", "Stash", "Stash pop", "Stash apply", "Discard":
			m.reloadCleanTabs()
		}
		return m, m.refresh()
	case clipboardMsg:
		if msg.err != nil {
			m.notice = m.tr(msg.err.Error())
		} else if msg.doc != nil && m.document() == msg.doc {
			if msg.cut {
				if msg.doc.SelectedText() == msg.text {
					m.insertClipboardText("")
				}
			} else if msg.text != "" {
				m.insertClipboardText(msg.text)
			}
		}
	case pathEntriesMsg:
		if m.pathCompletion == msg.request && m.validPathCompletion() {
			m.pathCompletion.entries = msg.entries
			m.pathCompletion.filter()
		}
	case tea.MouseClickMsg:
		m.pathCompletion = nil
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
		// These text and sidebar scroll paths only move their own viewport.
		// Dismissing transient UI or preparing another frame's work can affect
		// more regions and requires a full refresh.
		editing := m.editing()
		textDiff := !editing && m.diffOpen && m.images == nil
		localScroll := (editing || textDiff) && !m.mainShowsSidebar() && m.modal == "" &&
			m.contextMenu == nil && m.pathCompletion == nil && m.flash == nil
		// An inline image can update the editor layout during maintenance.
		stableText := localScroll && (!editing || m.tabs[m.activeTab].markdownImages == nil)
		m.pathCompletion = nil
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
				changed := m.scrollHorizontal(delta)
				if localScroll {
					scrolled = scrollEditor
					if textDiff {
						scrolled = scrollDiff
					}
					if stableText && !changed {
						scrolled = scrollUnchanged
					}
				}
			} else if stableText {
				scrolled = scrollUnchanged
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
			if stableText {
				scrolled = scrollUnchanged
			}
			l := m.sidebarList()
			if panel, ok := m.explorerPanelAt(msg.Y); ok {
				l = panel.list
			}
			if l.count > 0 && msg.Y >= l.start && msg.Y < l.start+l.height {
				top := *l.top
				*l.top += delta
				l.clampScroll()
				if stableText && *l.top != top {
					scrolled = scrollSidebar
				}
			}
		} else if editing {
			delta := 3
			if msg.Button == tea.MouseWheelUp {
				delta = -3
			}
			t := &m.tabs[m.activeTab]
			top := t.editorDisplayTop()
			t.setEditorDisplayTop(top + delta)
			m.clampEditorScroll(t)
			if localScroll {
				scrolled = scrollEditor
				if stableText && t.editorDisplayTop() == top {
					scrolled = scrollUnchanged
				}
			}
		} else {
			top := m.viewport.YOffset()
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			if localScroll {
				scrolled = scrollDiff
				if m.viewport.YOffset() == top {
					scrolled = scrollUnchanged
				}
			}
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
		m.pathCompletion = nil
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
		if !m.sidebar && m.editing() && msg.Content != "" {
			m.insertClipboardText(msg.Content)
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
	if handled, cmd := m.pathCompletionKey(k); handled {
		return cmd
	}
	updatePath := msg.Text != "" || matchesShortcut(k, eraseCharacter) || k == "delete"
	if !updatePath {
		m.pathCompletion = nil
	}
	route := m.resolveWorkbenchShortcut(k)
	if route.has(beforeInput) {
		m.leaderTarget = ""
		m.navigationState = navigationState{}
		return m.executeCommand(route.command, "")
	}
	if m.fileFindFocused() {
		if !route.has(fromFileFind) {
			return m.fileFindKey(msg)
		}
		if route.has(blurFileFind) {
			m.blurFileFind()
		}
	}
	if m.editorOnly && route.has(workspaceOnly) {
		return nil
	}
	if handled, cmd := m.sidebarTabKey(k); handled {
		return cmd
	}
	if m.sidebar && m.section == search && !route.has(fromSidebarSearch) {
		return m.searchKey(msg)
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
	if handled, cmd := m.diffChangeShortcut(k); handled {
		return cmd
	}
	if route.command == commandCopy || route.command == commandSave {
		if m.editorDocument() != nil && !m.tabs[m.activeTab].vim.InsertMode {
			m.tabs[m.activeTab].vim.CancelPending()
		}
		return m.executeCommand(route.command, "")
	}
	if d := m.editorDocument(); !m.sidebar && d != nil {
		if matchesShortcut(k, editorKeys.Cut) || matchesShortcut(k, editorKeys.Paste) {
			if d.ReadOnly {
				return nil
			}
			m.tabs[m.activeTab].vim.CancelPending()
			if matchesShortcut(k, editorKeys.Cut) {
				return m.copy(true)
			}
			return m.paste()
		}
		if matchesShortcut(k, pickerKeys.Accept) {
			if cmd, handled := m.enterMarkdownRemoteImage(); handled {
				return cmd
			}
		}
		if handled, cmd := m.vimKey(k); handled {
			return cmd
		}
	}
	if m.sidebar && m.section == explorer {
		if cmd, handled := m.fileHistoryKey(k); handled {
			return cmd
		}
		if m.outlineKey(k) {
			return nil
		}
		if m.outlineHasFocus() || m.fileHistoryHasFocus() {
			return m.executeCommand(route.command, "")
		}
	}
	if m.sidebar && m.section == explorer && key.Matches(msg, sidebarKeys.Paste) {
		m.navigationState = navigationState{}
		return m.pasteEntryPrompt()
	}
	if m.sidebar && m.section == changes {
		if handled, cmd := m.changesKey(k); handled {
			m.navigationState = navigationState{}
			return cmd
		}
	}
	if handled, cmd := m.navigationKey(msg); handled {
		return cmd
	}
	if route.command != "" {
		return m.executeCommand(route.command, "")
	}
	if m.sidebar {
		if m.section == pullRequests && !key.Matches(msg, sidebarKeys.Open) {
			if entry := m.current(); key.Matches(msg, sidebarKeys.OpenPullRequest) && entry != nil && entry.pullRequest != nil {
				return m.openBrowser(entry.pullRequest.URL)
			}
			return nil
		}
		switch {
		case key.Matches(msg, sidebarKeys.Delete):
			return m.deleteSelectedEntry()
		case key.Matches(msg, sidebarKeys.Open):
			return m.openSelected()
		case key.Matches(msg, sidebarKeys.SplitFolders):
			return m.splitFolderChain()
		case key.Matches(msg, sidebarKeys.Edit):
			return m.editSelected()
		}
		return nil
	}
	if m.currentImage() != nil {
		switch {
		case matchesShortcut(k, imageKeys.ZoomIn), matchesShortcut(k, imageKeys.ZoomOut):
			factor := 1.25
			if matchesShortcut(k, imageKeys.ZoomOut) {
				factor = 1 / factor
			}
			m.zoomImage(factor, m.mainX()+m.mainWidth()/2, m.contentRow()+m.contentHeight()/2)
		case matchesShortcut(k, imageKeys.Fit):
			m.resetImageZoom()
		}
		return nil
	}
	if d := m.editorDocument(); d != nil {
		if m.tabs[m.activeTab].vim.InsertMode {
			d.BeginEditGroup()
		}
		switch {
		case matchesShortcut(k, editorKeys.Undo):
			d.Undo()
		case matchesShortcut(k, editorKeys.Redo):
			d.Redo()
		case matchesShortcut(k, editorKeys.SelectAll):
			d.SelectAll()
		case matchesShortcut(k, eraseCharacter):
			d.Delete(true)
		case k == "delete":
			d.Delete(false)
		case matchesShortcut(k, pickerKeys.Accept):
			d.Insert("\n")
		case k == "tab":
			d.Insert("\t")
		case k == "pgup", k == "pgdown":
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
		if updatePath {
			return m.refreshPathCompletion()
		}
		return nil
	}
	if matchesShortcut(k, pickerKeys.Accept) {
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
	if err := m.saveDocument(d); err != nil {
		m.notice = m.tr(err.Error())
		return nil
	}
	m.showStatusFeedback(m.tr("Saved %s", filepath.Base(d.Path)))
	refresh := m.refreshRepository(true)
	if closeAfter {
		return tea.Batch(refresh, m.closeTab())
	}
	return refresh
}

func (m *Model) saveDocument(d *editor.Document) error {
	if err := d.Save(); err != nil {
		return err
	}
	for i := range m.tabs {
		if t := &m.tabs[i]; t.doc == d {
			t.reloadSeq++
			t.diskNotice, t.reloadWhenClean = "", false
			break
		}
	}
	m.invalidateWorktreeDiffs(d.Path)
	return nil
}

func (m *Model) reloadCleanTabs() {
	var kept []tab
	newActive := -1
	for i, t := range m.tabs {
		wasActive := i == m.activeTab
		if t.image != nil && t.image.HasPixels() {
			fresh, err := imagepreview.Open(m.ctx, t.image.Path)
			if err != nil {
				m.notice = m.tr(err.Error())
			} else {
				t.diskNotice = ""
				if !fresh.SameContent(t.image.Image) {
					t.image.replacePixels(fresh)
				}
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
			t.diskNotice, t.reloadWhenClean = "", false
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
	if image := m.currentImage(); image != nil && !image.HasPixels() {
		im := m.tabs[m.activeTab].image
		im.renderErr, im.rows = nil, nil
	}
	m.trimImageTabs()
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

func (m *Model) insertClipboardText(text string) {
	d := m.document()
	v := &m.tabs[m.activeTab].vim
	if !v.InsertMode {
		v.CancelPending()
	}
	d.Insert(text)
	if v.VisualMode {
		v.Reset(d)
	}
	m.ensureEditor()
}
