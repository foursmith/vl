package app

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/theme"
)

type statusSegment struct {
	id, text string
	x, width int
	// Offsets within the sync segment, including its left padding.
	pullOffset, pushOffset int
}

const (
	// These IDs are stored in user preferences independently of the labels.
	statusBarStandard = "vscode"
	statusBarPure     = "vim"
)

func (m *Model) openStatusBarStylePicker() tea.Cmd {
	m.palette = nil
	for _, style := range []struct{ id, label string }{
		{statusBarStandard, m.tr("Standard")},
		{statusBarPure, m.tr("Pure")},
	} {
		key := ""
		if m.statusBarStyle == style.id {
			key = m.tr("Current")
		}
		m.palette = append(m.palette, action{id: style.id, label: style.label, key: key, run: func() tea.Cmd {
			m.statusBarStyle = style.id
			return m.savePreferences()
		}})
	}
	return m.prompt("statusBarStyles", m.tr("Status Bar Style"), "")
}

func (m *Model) gitStatusSegments() []statusSegment {
	target := m.statusRepository()
	state := m.repositoryState(target)
	branch := state.Branch
	if branch == "" {
		branch = "…"
	}
	if state.Detached {
		branch = shortOID(state.Head) + m.tr(" (detached)")
	}
	staged, unstaged, untracked, conflicts := false, false, false, 0
	repositoryChanges := 0
	for _, change := range m.snapshot.Changes {
		if change.Repository != target {
			continue
		}
		repositoryChanges++
		staged = staged || change.Staged()
		unstaged = unstaged || change.Unstaged() && !change.Untracked()
		untracked = untracked || change.Untracked()
		if change.Conflicted() {
			conflicts++
		}
	}
	marks := ""
	if unstaged {
		marks += "*"
	}
	if staged {
		marks += "+"
	}
	if untracked {
		marks += "?"
	}
	branchIcon, githubIcon := m.symbol(symbolBranch), m.symbol(symbolGitHub)
	branchBudget := max(4, min(30, m.width/3)-ansi.StringWidth(marks)-3)
	prefix := ""
	if len(m.snapshot.Repositories) > 0 {
		branchBudget = max(7, min(48, m.width/3)-ansi.StringWidth(marks)-3)
		name := ansi.Truncate(safe(m.repositoryName(target)), max(1, min(18, (branchBudget-3)/2)), "…")
		prefix = name + " · "
	}
	branch = prefix + ansi.Truncate(safe(branch), max(1, branchBudget-ansi.StringWidth(prefix)), "…") + marks
	actions := m.repositoryRemoteActions(target, state)
	sync := statusSegment{id: "sync", text: actions[0].glyph}
	if len(actions) == 3 {
		sync.pullOffset = actions[0].displayWidth()
		sync.pushOffset = sync.pullOffset + actions[1].displayWidth() - 1
		sync.text += " " + actions[1].glyph + " " + actions[2].glyph
	}
	changeIcon, changeCount := m.symbol(symbolDiff), repositoryChanges
	if conflicts > 0 {
		changeIcon, changeCount = m.symbol(symbolWarning), conflicts
	}
	return []statusSegment{
		{id: "branch", text: branchIcon + " " + branch},
		sync,
		{id: "github", text: githubIcon},
		{id: "changes", text: fmt.Sprintf("%s %d", changeIcon, changeCount)},
	}
}

func (m *Model) showStatusFeedback(text string) {
	m.notice = ""
	m.statusFeedback = &statusFeedback{text: text}
}

func (m *Model) statusNotice() string {
	if m.notice != "" {
		return m.notice
	}
	if m.activeTab >= 0 && m.activeTab < len(m.tabs) && m.tabs[m.activeTab].diskNotice != "" {
		return m.tabs[m.activeTab].diskNotice
	}
	if m.operationProgress != "" {
		return m.operationProgress
	}
	if m.statusFeedback != nil {
		return m.statusFeedback.text
	}
	return ""
}

func (m *Model) statusHints() []string {
	if m.modal != "" || m.contextMenu != nil || m.fileFindFocused() || m.leaderActive() || m.flashValid() {
		return nil
	}
	if m.sidebar {
		entry := m.current()
		if m.section == changes && entry != nil && !m.busy {
			return m.changeStatusHints(entry)
		}
		if m.section == explorer && !m.outlineHasFocus() && m.activeTab >= 0 && m.tabs[m.activeTab].preview {
			var hints []string
			if entry != nil && entry.file != "" {
				path := entry.file
				if !filepath.IsAbs(path) {
					path = filepath.Join(m.repo.Root, path)
				}
				if m.tabs[m.activeTab].path() == path {
					hints = append(hints, sidebarKeys.Open.Help().Key+" "+m.tr("Keep Preview"))
				}
			}
			if key := m.commandShortcutHint(commandFocus); key != "" {
				hints = append(hints, key+" "+m.tr("Focus Editor"))
			}
			return hints
		}
		return nil
	}
	if !m.busy && !m.editing() && m.diffOpen && m.diffChange != nil &&
		m.activeTab >= 0 && !m.tabs[m.activeTab].vim.CommandPending() {
		var stage, discard []string
		for _, action := range m.reviewActions() {
			if action.key == "" || action.disabled != "" {
				continue
			}
			hint := action.key + " " + action.label
			if strings.HasPrefix(action.id, "diff.discard") {
				discard = append(discard, hint)
			} else {
				stage = append(stage, hint)
			}
		}
		return append(stage, discard...)
	}
	return nil
}

func (m *Model) changeStatusHints(entry *item) []string {
	stage, unstage, discard := "Stage File", "Unstage File", "Discard File"
	switch {
	case entry.repoHeader:
		stage, unstage = "Stage Repository", "Unstage Repository"
	case entry.folder != "":
		stage, unstage, discard = "Stage Folder", "Unstage Folder", "Discard Folder"
	case entry.header:
		stage, unstage, discard = "Stage Group", "Unstage Group", "Discard Group"
	case entry.change == nil:
		return nil
	}
	if entry.staged {
		stage = unstage
	}
	scope := "Stage Repository"
	if m.stageScopeUnstages(entry) {
		scope = "Unstage Repository"
	}
	stageKey, scopeKey := gitKeys.Stage.Help().Key, gitKeys.StageScope.Help().Key
	var hints []string
	if stage == scope {
		hints = append(hints, stageKey+"/"+scopeKey+" "+m.tr(stage))
	} else {
		if entry.change == nil || m.stageFileReason(*entry.change, entry.staged) == "" {
			hints = append(hints, stageKey+" "+m.tr(stage))
		}
		hints = append(hints, scopeKey+" "+m.tr(scope))
	}
	if !entry.staged {
		if !entry.repoHeader && (entry.change == nil || m.discardFileReason(*entry.change) == "") {
			if entry.change != nil && entry.change.Untracked() {
				discard = "Delete Untracked File"
			}
			hints = append(hints, gitKeys.Discard.Help().Key+" "+m.tr(discard))
		}
		hints = append(hints, gitKeys.DiscardScope.Help().Key+" "+m.tr("Discard Repository Changes"))
	}
	return hints
}

// Fit complete action hints in order before dropping the lower-priority ones.
func fitStatusHints(hints []string, width int) string {
	if len(hints) == 0 {
		return ""
	}
	text := hints[0]
	for _, hint := range hints[1:] {
		next := text + " · " + hint
		if ansi.StringWidth(next) > width {
			break
		}
		text = next
	}
	return text
}

func (m *Model) statusLayout() []statusSegment {
	left := []statusSegment{{id: "init", text: m.tr("Initialize Repository")}}
	if m.editorOnly {
		var hints []string
		for _, id := range [...]commandID{commandSave, commandClose, commandPalette} {
			if hint := m.commandShortcutHint(id); hint != "" {
				hints = append(hints, hint)
			}
		}
		shortcuts := strings.Join(hints, " · ")
		label := shortcuts
		if d := m.document(); d != nil {
			label = safe(filepath.Base(d.Path))
			if d.Dirty() {
				label += " *"
			}
			if m.width >= 100 && shortcuts != "" {
				label += " · " + shortcuts
			}
		}
		left = []statusSegment{{id: "editor", text: label}}
	} else if !m.snapshot.NoRepository {
		left = m.gitStatusSegments()
	}
	mode := ""
	if m.editorDocument() != nil {
		mode = m.tabs[m.activeTab].vim.Label()
	}
	if m.leaderActive() {
		mode = "LEADER"
	}
	if mode != "" {
		if flash := m.flashLabel(); flash != "" {
			mode = flash
		}
		left = append([]statusSegment{{id: "vim", text: mode}}, left...)
	}
	var result []statusSegment
	x := 0
	for _, segment := range left {
		segment.width = ansi.StringWidth(segment.text) + 2
		if x+segment.width > m.width {
			continue
		}
		segment.x = x
		result = append(result, segment)
		x += segment.width
	}
	notice := m.statusNotice()
	var hints []string
	if notice == "" {
		hints = m.statusHints()
	}
	rightLimit := m.width - x - 1
	hintWidth := 0
	if notice != "" {
		hintWidth = min(24, ansi.StringWidth(safe(notice))+2)
	} else if len(hints) > 0 {
		hintWidth = ansi.StringWidth(hints[0]) + 2
	}
	if hintWidth > 0 {
		rightLimit = min(rightLimit, max(0, m.width-x-hintWidth))
	}
	var candidates []statusSegment
	if d := m.editorDocument(); d != nil {
		line, col := d.Position()
		eol := "LF"
		if d.CRLF {
			eol = "CRLF"
		}
		language := m.tr(m.language(d.Path).Name)
		candidates = []statusSegment{{id: "position", text: m.tr("Ln %d, Col %d", line+1, col+1)}, {id: "encoding", text: "UTF-8"}, {id: "eol", text: eol}, {id: "language", text: language}}
		if d.ReadOnly {
			candidates = append([]statusSegment{{id: "readOnly", text: m.tr("Read-only")}}, candidates...)
		}
	}
	if im := m.currentImage(); im != nil {
		b := im.Bounds()
		zoom := m.imageRegion().Scale
		candidates = []statusSegment{{id: "image", text: m.tr("%s · %d × %d · %.0f%% · Read-only", im.Format, b.Dx(), b.Dy(), zoom*100)}}
	}
	var right []statusSegment
	rightWidth := 0
	for _, segment := range candidates {
		segment.width = ansi.StringWidth(segment.text) + 2
		if rightWidth+segment.width <= rightLimit {
			right = append(right, segment)
			rightWidth += segment.width
		}
	}
	rightX := m.width - rightWidth
	nextX := rightX
	for i := range right {
		right[i].x = nextX
		nextX += right[i].width
	}
	if notice == "" {
		notice = fitStatusHints(hints, rightX-x-2)
		if m.sidebar && m.section == search && notice == "" {
			notice = m.tr("%s: fields/results", searchKeys.NextField.Help().Key)
			if m.search.replace && m.search.focus == 1 {
				notice = m.tr("%s: replace", sidebarKeys.Open.Help().Key)
			}
		}
		if m.sidebar && m.sidebarVisible && m.modal == "" && m.contextMenu == nil && (m.section == history || m.section == pullRequests) {
			if title := m.current().revisionTitle(); title != "" {
				notice = title
			}
		}
		if hint := m.hoverHint(); hint != "" {
			notice = hint
		}
		if len(right) > 0 && m.hoveredStatusLanguage(right[len(right)-1]) {
			notice = m.tr("Change Syntax Highlighting")
		}
		if m.flashValid() {
			notice = m.tr("Type to search · label to jump · Tab next · Esc cancel")
		}
	}
	if gap := rightX - x; gap > 10 && notice != "" {
		text := ansi.Truncate(safe(notice), gap-2, "…")
		result = append(result, statusSegment{id: "notice", x: x, width: gap, text: text})
	}
	return append(result, right...)
}

func (m *Model) vimModeBackground() string {
	if m.editorDocument() == nil {
		return ""
	}
	// VSCodeVim's defaults with statusBarColorControl enabled.
	vim := &m.tabs[m.activeTab].vim
	switch {
	case vim.InsertMode:
		return "#5F0000"
	case vim.VisualMode && vim.VisualLine:
		return "#005F87"
	case vim.VisualMode:
		return "#5F00AF"
	default:
		return "#005F5F"
	}
}

func (m *Model) statusLine() string {
	background := m.colors.Panel
	foreground := theme.Readable(m.colors.SideForeground, background)
	modeBackground := m.vimModeBackground()
	pureStyle := m.statusBarStyle == statusBarPure
	if !pureStyle && modeBackground != "" {
		foreground, background = "#FFFFFF", modeBackground
	}
	var text strings.Builder
	x := 0
	for _, segment := range m.statusLayout() {
		if segment.x > x {
			text.WriteString(row("", segment.x-x, foreground, background))
		}
		fg, bg := foreground, background
		if pureStyle && segment.id == "vim" && modeBackground != "" {
			fg, bg = "#FFFFFF", modeBackground
		}
		if m.hoveredStatusLanguage(segment) {
			fg, bg = theme.Readable(m.colors.HoverForeground, m.colors.Hover), m.colors.Hover
		}
		text.WriteString(row(" "+segment.text+" ", segment.width, fg, bg))
		x = segment.x + segment.width
	}
	if x < m.width {
		text.WriteString(row("", m.width-x, foreground, background))
	}
	return text.String()
}

func (m *Model) hoveredStatusLanguage(segment statusSegment) bool {
	return segment.id == "language" && m.modal == "" && m.contextMenu == nil &&
		m.hoverY == m.height-1 && m.hoverX >= segment.x && m.hoverX < segment.x+segment.width &&
		m.canSetFileSyntax()
}

func (m *Model) clickStatus(x int) tea.Cmd {
	for _, segment := range m.statusLayout() {
		if x < segment.x || x >= segment.x+segment.width {
			continue
		}
		switch segment.id {
		case "init":
			return m.initializeRepository()
		case "branch":
			return m.openBranchPicker(m.statusRepository(), "")
		case "sync":
			offset := x - segment.x
			action := "sync"
			if segment.pushOffset > 0 && offset >= segment.pushOffset {
				action = "push"
			} else if segment.pullOffset > 0 && offset >= segment.pullOffset {
				action = "pull"
			}
			return m.runRepositoryRemoteAction(m.statusRepository(), action)
		case "github":
			return m.openGitHub()
		case "changes":
			return m.selectSectionInRepository(changes, m.statusRepository())
		case "language":
			return m.openFileSyntaxPicker()
		}
	}
	return nil
}
