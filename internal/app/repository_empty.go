package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/theme"
)

type repositoryEmptyState struct {
	title, detail, label string
	action               func() tea.Cmd
}

type repositoryEmptyLayout struct {
	x, titleY   int
	title       string
	detail      []string
	button      dialogButton
	buttonLabel string
}

func (m *Model) repositoryEmptyState() repositoryEmptyState {
	if m.section != changes && m.section != history && m.section != pullRequests {
		return repositoryEmptyState{}
	}
	if m.snapshot.NoRepository {
		if m.gitUnavailable {
			detail := "Install Git to view changes and commit history."
			if m.section == pullRequests {
				detail = "Install Git to work with pull requests."
			}
			return repositoryEmptyState{title: "Git is not available", detail: detail}
		}
		detail := "Track changes and browse commits in this folder."
		if m.section == pullRequests {
			detail = "Initialize this folder to start working with pull requests."
		}
		return repositoryEmptyState{"No Git repository", detail, "Initialize Repository", m.initializeRepository}
	}
	if m.section == pullRequests && m.hasRepositoryWithoutRemotes() {
		detail := "Publish this repository to GitHub to view its pull requests here."
		if blocker := publishBranchBlocker(m.snapshot); blocker != "" {
			detail = blocker
		}
		return repositoryEmptyState{"No Git remote", detail, "Publish to GitHub", func() tea.Cmd {
			return m.publishBranchInRepository("")
		}}
	}
	return repositoryEmptyState{}
}

func (m *Model) hasRepositoryWithoutRemotes() bool {
	// Branch is populated once the first repository snapshot arrives.
	return !m.snapshot.NoRepository && m.snapshot.Branch != "" && len(m.snapshot.Remotes) == 0
}

func (m *Model) repositoryEmptyLayout(state repositoryEmptyState, width int) repositoryEmptyLayout {
	space := max(0, m.bodyHeight()-headerHeight)
	x := min(2, max(0, (width-1)/4))
	inner := max(1, width-2*x)
	reserved := 1
	if state.action != nil {
		reserved++
	}
	top := min(2, max(0, space-reserved))
	l := repositoryEmptyLayout{x: x, titleY: headerHeight + top, title: ansi.Truncate(m.tr(state.title), inner, "…")}
	detailLines := strings.Split(ansi.Wrap(m.tr(state.detail), inner, ""), "\n")
	detailSpace := max(0, space-top-reserved-1)
	l.detail = detailLines[:min(len(detailLines), detailSpace)]
	if state.action != nil && space-top >= reserved {
		y := l.titleY + 1 + len(l.detail)
		if y+1 < m.bodyHeight() {
			y++
		}
		label := " " + ansi.Truncate(m.tr(state.label), max(1, inner-4), "…") + " ↵ "
		l.buttonLabel = ansi.Truncate(label, inner, "")
		l.button = dialogButton{x: x, y: y, width: ansi.StringWidth(l.buttonLabel)}
	}
	return l
}

func (m *Model) repositoryEmptyButtonAt(state repositoryEmptyState, x, y int) bool {
	if state.action == nil {
		return false
	}
	button := m.repositoryEmptyLayout(state, m.sidebarContentWidth()).button
	return y == button.y && x >= button.x && x < button.x+button.width
}

func (m *Model) renderRepositoryEmptyState(state repositoryEmptyState, lines []string, width int) []string {
	l := m.repositoryEmptyLayout(state, width)
	bg := m.colors.Panel
	fg := theme.Readable(m.colors.SideForeground, bg)
	muted := theme.Readable(m.colors.Muted, bg)
	inset := strings.Repeat(" ", l.x)
	for y := len(lines); y < m.bodyHeight(); y++ {
		text := ""
		switch {
		case y == l.titleY:
			text = inset + theme.TextStyle(fg, bg).Bold(true).Render(l.title)
		case y > l.titleY && y <= l.titleY+len(l.detail):
			text = inset + theme.TextStyle(muted, bg).Render(l.detail[y-l.titleY-1])
		case l.button.width > 0 && y == l.button.y:
			buttonFG, buttonBG := m.colors.PopupSelectionForeground, m.colors.PopupSelection
			button := theme.TextStyle(buttonFG, buttonBG).Bold(true).Render(l.buttonLabel)
			text = inset + button
		}
		lines = append(lines, row(text, width, fg, bg))
	}
	return lines[:m.bodyHeight()]
}
