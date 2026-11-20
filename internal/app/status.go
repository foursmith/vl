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
	nextX := rightX
	for i := range right {
		right[i].x = nextX
		nextX += right[i].width
	}
	notice := m.notice
	if notice == "" && m.modal == "" && m.contextMenu == nil {
		switch {
		case m.sidebar && m.section == changes && m.current() != nil:
			notice = m.tr("%s/%s: stage or unstage · %s/%s: discard", gitKeys.Stage.Help().Key, gitKeys.StageScope.Help().Key, gitKeys.Discard.Help().Key, gitKeys.DiscardScope.Help().Key)
		case !m.sidebar && m.diffOpen && m.diffChange != nil:
			file, hunk := m.tr("Stage File"), m.tr("Stage Hunk")
			if m.diffStaged {
				file, hunk = m.tr("Unstage File"), m.tr("Unstage Hunk")
			}
			notice = gitKeys.StageScope.Help().Key + ": " + file
			if m.partialStageReason(m.diffStaged) == "" {
				start, end := m.diffSelection.bounds()
				if len(m.selectedDiffChanges()) > 0 {
					hunk = m.tr("Stage Selected Ranges")
					if m.diffStaged {
						hunk = m.tr("Unstage Selected Ranges")
					}
				} else if m.diffHunk < 0 || m.diffSelection.active && start != end {
					hunk = ""
				}
				if hunk != "" {
					notice = gitKeys.Stage.Help().Key + ": " + hunk + " · " + notice
				}
			}
			if !m.diffStaged {
				notice += " · " + m.tr("%s/%s: discard", gitKeys.Discard.Help().Key, gitKeys.DiscardScope.Help().Key)
			}
		}
	}
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
