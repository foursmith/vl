package app

import (
	"fmt"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/vl/internal/theme"
)

type statusSegment struct {
	id, text, color, background string
	x, width                    int
}

func (m *Model) gitStatusSegments() []statusSegment {
	foreground := m.colors.StatusForeground
	dim := theme.WithOpacity(foreground, .6)
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
	githubColor := foreground
	if state.GitHubURL == "" {
		githubColor = dim
	}
	left := []statusSegment{{id: "branch", text: branchIcon + " " + branch, color: foreground}, {id: "sync", text: sync, color: foreground}, {id: "github", text: githubIcon, color: githubColor}, {id: "changes", text: fmt.Sprintf("%s %d", m.symbol(symbolDiff), repositoryChanges), color: dim}}
	if conflicts > 0 {
		left[3].text = fmt.Sprintf("%s %d", m.symbol(symbolWarning), conflicts)
		left[3].color = m.colors.Deleted
	}
	return left
}

func (m *Model) statusLayout() []statusSegment {
	foreground := m.colors.StatusForeground
	dim := theme.WithOpacity(foreground, .6)
	left := []statusSegment{{id: "init", text: m.tr("Initialize Repository"), color: dim}}
	if m.editorOnly {
		label := "Ctrl+S · Ctrl+W · " + workbenchKeys.Commands.Help().Key
		if d := m.document(); d != nil {
			label = safe(filepath.Base(d.Path))
			if d.Dirty() {
				label += " *"
			}
			if m.width >= 100 {
				label += " · Ctrl+S · Ctrl+W · " + workbenchKeys.Commands.Help().Key
			}
		}
		left = []statusSegment{{id: "editor", text: label, color: dim}}
	} else if !m.snapshot.NoRepository {
		left = m.gitStatusSegments()
	}
	if m.editorDocument() != nil {
		v := &m.tabs[m.activeTab].vim
		label, color := v.Label(), m.colors.Accent
		if v.InsertMode {
			color = m.colors.GitAdded
		} else if v.VisualMode || v.CommandPending() {
			color = m.colors.GitModified
		}
		if m.leaderActive() {
			label, color = "LEADER", m.colors.GitModified
		}
		if flash := m.flashLabel(); flash != "" {
			label, color = flash, m.colors.GitModified
		}
		left = append([]statusSegment{{id: "vim", text: label, color: m.colors.Background, background: color}}, left...)
	} else if m.leaderActive() {
		left = append([]statusSegment{{id: "vim", text: "LEADER", color: m.colors.Background, background: m.colors.GitModified}}, left...)
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
		segment.color = foreground
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
	if hint := m.hoverHint(); hint != "" {
		notice = hint
	}
	if m.flashValid() {
		notice = m.tr("Type to search · label to jump · Tab next · Esc cancel")
	}
	if gap := rightX - x; gap > 10 && notice != "" {
		text := ansi.Truncate(safe(notice), gap-2, "…")
		result = append(result, statusSegment{id: "notice", x: x, width: gap, text: text, color: dim})
	}
	for _, segment := range right {
		segment.x = rightX
		result = append(result, segment)
		rightX += segment.width
	}
	return result
}

func (m *Model) statusLine() string {
	foreground := m.colors.StatusForeground
	text := ""
	x := 0
	for _, segment := range m.statusLayout() {
		background := segment.background
		if background == "" {
			background = m.colors.StatusBackground
		}
		text += row("", segment.x-x, foreground, m.colors.StatusBackground) + row(" "+segment.text+" ", segment.width, segment.color, background)
		x = segment.x + segment.width
	}
	return row(text, m.width, foreground, m.colors.StatusBackground)
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
		case "position":
			return m.prompt("position", m.tr("Go to line · line:column"), "")
		}
	}
	return nil
}
