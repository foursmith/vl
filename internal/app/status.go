package app

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

type statusSegment struct {
	id, text, color string
	x, width        int
}

func (m *Model) gitStatusSegments() []statusSegment {
	target := m.statusRepository()
	state := m.repositoryState(target)
	branch := state.Branch
	if branch == "" {
		branch = "…"
	}
	if state.Detached {
		branch = shortOID(state.Head) + " (分离)"
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
		sync = m.symbol(symbolPublish) + " 发布"
	} else if state.TrackingKnown {
		sync = fmt.Sprintf("%s %d%s %d%s", syncIcon, state.Behind, m.symbol(symbolArrowDown), state.Ahead, m.symbol(symbolUp))
	} else if state.Upstream != "" {
		sync += " " + m.symbol(symbolWarning)
	}
	githubColor := m.colors.Foreground
	if state.GitHubURL == "" {
		githubColor = m.colors.Muted
	}
	left := []statusSegment{{id: "branch", text: branchIcon + " " + branch, color: m.colors.Foreground}, {id: "sync", text: sync, color: m.colors.Foreground}, {id: "github", text: githubIcon, color: githubColor}, {id: "changes", text: fmt.Sprintf("%s %d", m.symbol(symbolDiff), repositoryChanges), color: m.colors.Muted}}
	if conflicts > 0 {
		left[3].text = fmt.Sprintf("%s %d", m.symbol(symbolWarning), conflicts)
		left[3].color = m.colors.Deleted
	}
	return left
}

func (m *Model) statusLayout() []statusSegment {
	left := []statusSegment{{id: "init", text: "初始化 Git 仓库", color: m.colors.Muted}}
	if !m.snapshot.NoRepository {
		left = m.gitStatusSegments()
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
	if d := m.document(); d != nil {
		line, col := d.Position()
		eol := "LF"
		if d.CRLF {
			eol = "CRLF"
		}
		language := m.language(d.Path).Name
		candidates = []statusSegment{{id: "position", text: fmt.Sprintf("行 %d, 列 %d", line+1, col+1)}, {id: "encoding", text: "UTF-8"}, {id: "eol", text: eol}, {id: "indent", text: "Tab: 4"}, {id: "language", text: language}}
	}
	if im := m.currentImage(); im != nil {
		b := im.Bounds()
		zoom := m.imageRegion().Scale
		candidates = []statusSegment{{id: "image", text: fmt.Sprintf("%s · %d × %d · %.0f%% · 只读", im.Format, b.Dx(), b.Dy(), zoom*100)}}
	}
	var right []statusSegment
	rightWidth := 0
	for _, segment := range candidates {
		segment.width = ansi.StringWidth(segment.text) + 2
		segment.color = m.colors.Foreground
		if x+rightWidth+segment.width+1 <= m.width {
			right = append(right, segment)
			rightWidth += segment.width
		}
	}
	rightX := m.width - rightWidth
	notice := m.notice
	if hint := m.hoverHint(); hint != "" {
		notice = hint
	}
	if gap := rightX - x; gap > 10 && notice != "" {
		text := ansi.Truncate(safe(notice), gap-2, "…")
		result = append(result, statusSegment{id: "notice", x: x, width: gap, text: text, color: m.colors.Muted})
	}
	for _, segment := range right {
		segment.x = rightX
		result = append(result, segment)
		rightX += segment.width
	}
	return result
}

func (m *Model) statusLine() string {
	text := ""
	x := 0
	for _, segment := range m.statusLayout() {
		text += row("", segment.x-x, m.colors.Foreground, m.colors.Panel) + row(" "+segment.text+" ", segment.width, segment.color, m.colors.Panel)
		x = segment.x + segment.width
	}
	return row(text, m.width, m.colors.Foreground, m.colors.Panel)
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
			return m.prompt("position", "转到行 · 行:列", "")
		}
	}
	return nil
}
