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
	branchIcon, syncIcon, githubIcon := m.symbol(symbolBranch), m.symbol(symbolSync), m.symbol(symbolGitHub)
	branchBudget := max(4, min(30, m.width/3)-ansi.StringWidth(marks)-3)
	prefix := ""
	if len(m.snapshot.Repositories) > 0 {
		branchBudget = max(7, min(48, m.width/3)-ansi.StringWidth(marks)-3)
		name := ansi.Truncate(safe(m.repositoryName(target)), max(1, min(18, (branchBudget-3)/2)), "…")
		prefix = name + " · "
	}
	branch = prefix + ansi.Truncate(safe(branch), max(1, branchBudget-ansi.StringWidth(prefix)), "…") + marks
	sync := syncIcon
	if m.busy {
		sync = "…"
	} else if state.Upstream == "" && !state.Detached {
		sync = m.symbol(symbolPublish) + m.tr(" Publish")
	} else if state.TrackingKnown {
		sync = fmt.Sprintf("%s %d%s %d%s", syncIcon, state.Behind, m.symbol(symbolArrowDown), state.Ahead, m.symbol(symbolUp))
	} else if state.Upstream != "" {
		sync += " " + m.symbol(symbolWarning)
	}
	changeIcon, changeCount := m.symbol(symbolDiff), repositoryChanges
	if conflicts > 0 {
		changeIcon, changeCount = m.symbol(symbolWarning), conflicts
	}
	return []statusSegment{
		{id: "branch", text: branchIcon + " " + branch},
		{id: "sync", text: sync},
		{id: "github", text: githubIcon},
		{id: "changes", text: fmt.Sprintf("%s %d", changeIcon, changeCount)},
	}
}

func (m *Model) statusLayout() []statusSegment {
	left := []statusSegment{{id: "init", text: m.tr("Initialize Repository")}}
	if m.editorOnly {
		shortcuts := workbenchKeys.Save.Help().Key + " · " + workbenchKeys.Close.Help().Key + " · " + workbenchKeys.Commands.Help().Key
		label := shortcuts
		if d := m.document(); d != nil {
			label = safe(filepath.Base(d.Path))
			if d.Dirty() {
				label += " *"
			}
			if m.width >= 100 {
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
		if x+rightWidth+segment.width+1 <= m.width {
			right = append(right, segment)
			rightWidth += segment.width
		}
	}
	rightX := m.width - rightWidth
	notice := m.notice
	if m.sidebar && m.section == search && notice == "" {
		notice = m.tr("Tab: fields/results · Ctrl+Shift+J: options")
		if m.search.replace && m.search.focus == 1 {
			notice = m.tr("Enter: replace · Ctrl+Alt+Enter: replace all")
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
	if m.flashValid() {
		notice = m.tr("Type to search · label to jump · Tab next · Esc cancel")
	}
	if gap := rightX - x; gap > 10 && notice != "" {
		text := ansi.Truncate(safe(notice), gap-2, "…")
		result = append(result, statusSegment{id: "notice", x: x, width: gap, text: text})
	}
	for _, segment := range right {
		segment.x = rightX
		result = append(result, segment)
		rightX += segment.width
	}
	return result
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
		text.WriteString(row(" "+segment.text+" ", segment.width, fg, bg))
		x = segment.x + segment.width
	}
	if x < m.width {
		text.WriteString(row("", m.width-x, foreground, background))
	}
	return text.String()
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
			return m.syncRepository()
		case "github":
			return m.openGitHub()
		case "changes":
			return m.selectSectionInRepository(changes, m.statusRepository())
		}
	}
	return nil
}
