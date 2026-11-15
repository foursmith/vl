package app

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/elegit/internal/theme"
)

type statusSegment struct {
	id, text, color string
	x, width        int
}

func (m *Model) statusLayout() []statusSegment {
	branch := m.snapshot.Branch
	if branch == "" {
		branch = "…"
	}
	if m.snapshot.Detached {
		branch = shortOID(m.snapshot.Head) + " (分离)"
	}
	staged, unstaged, untracked, conflicts := false, false, false, 0
	for _, change := range m.snapshot.Changes {
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
	branch = ansi.Truncate(safe(branch), max(4, min(30, m.width/3)-ansi.StringWidth(marks)-3), "…") + marks
	sync := syncIcon
	if m.busy {
		sync = "…"
	} else if m.snapshot.Upstream == "" && !m.snapshot.Detached {
		sync = m.symbol(symbolPublish) + " 发布"
	} else if m.snapshot.TrackingKnown {
		sync = fmt.Sprintf("%s %d%s %d%s", syncIcon, m.snapshot.Behind, m.symbol(symbolArrowDown), m.snapshot.Ahead, m.symbol(symbolUp))
	} else if m.snapshot.Upstream != "" {
		sync += " " + m.symbol(symbolWarning)
	}
	githubColor := theme.Foreground
	if m.snapshot.GitHubURL == "" {
		githubColor = theme.Muted
	}
	left := []statusSegment{{id: "branch", text: branchIcon + " " + branch, color: theme.Foreground}, {id: "sync", text: sync, color: theme.Foreground}, {id: "github", text: githubIcon, color: githubColor}, {id: "changes", text: fmt.Sprintf("%s %d", m.symbol(symbolDiff), len(m.snapshot.Changes)), color: theme.Muted}}
	if conflicts > 0 {
		left[3].text = fmt.Sprintf("%s %d", m.symbol(symbolWarning), conflicts)
		left[3].color = theme.Deleted
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
	if d := m.document(); m.showEditor && d != nil {
		line, col := d.Position()
		eol := "LF"
		if d.CRLF {
			eol = "CRLF"
		}
		language := m.language(d.Path).Name
		candidates = []statusSegment{{id: "position", text: fmt.Sprintf("行 %d, 列 %d", line+1, col+1)}, {id: "encoding", text: "UTF-8"}, {id: "eol", text: eol}, {id: "indent", text: "Tab: 4"}, {id: "language", text: language}}
	}
	var right []statusSegment
	rightWidth := 0
	for _, segment := range candidates {
		segment.width = ansi.StringWidth(segment.text) + 2
		segment.color = theme.Foreground
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
		result = append(result, statusSegment{id: "notice", x: x, width: gap, text: text, color: theme.Muted})
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
		text += row("", segment.x-x, theme.Foreground, theme.Panel) + row(" "+segment.text+" ", segment.width, segment.color, theme.Panel)
		x = segment.x + segment.width
	}
	return row(text, m.width, theme.Foreground, theme.Panel)
}

func (m *Model) clickStatus(x int) tea.Cmd {
	for _, segment := range m.statusLayout() {
		if x < segment.x || x >= segment.x+segment.width {
			continue
		}
		switch segment.id {
		case "branch":
			return m.openBranchPicker("")
		case "sync":
			return m.syncRepository()
		case "github":
			return m.openGitHub()
		case "changes":
			return m.selectSection(changes)
		case "position":
			return m.prompt("position", "转到行 · 行:列", "")
		}
	}
	return nil
}
