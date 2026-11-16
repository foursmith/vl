package app

import (
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/elegit/internal/config"
)

type workspacesMsg struct {
	seq     int
	folders []string
	err     error
}

func (m *Model) loadWorkspaces() tea.Cmd {
	m.workspaceSeq++
	seq, ctx := m.workspaceSeq, m.ctx
	return func() tea.Msg {
		folders, err := config.RecentFolders()
		if ctx.Err() != nil {
			return nil
		}
		return workspacesMsg{seq: seq, folders: folders, err: err}
	}
}

func (m *Model) workspaceRows() int {
	if len(m.workspaces) == 0 {
		return 0
	}
	// Leave room for the current section; longer histories scroll independently.
	return 1 + min(len(m.workspaces), max(1, m.bodyHeight()-9))
}

func (m *Model) sidebarHeaderRows() int { return sidebarHeaderRows + m.workspaceRows() }

func (m *Model) clampWorkspaces() {
	m.workspaceTop = min(max(0, m.workspaceTop), max(0, len(m.workspaces)-(m.workspaceRows()-1)))
}

func (m *Model) hoveredWorkspace() int {
	if m.modal != "" || m.resizingSidebar || !m.sidebarAt(m.hoverX) || m.hoverY < 1 || m.hoverY >= m.workspaceRows() {
		return -1
	}
	return m.workspaceTop + m.hoverY - 1
}

func (m *Model) workspaceLabel(folder string) string {
	name := filepath.Base(folder)
	parent := filepath.Dir(folder)
	duplicate := false
	for _, other := range m.workspaces {
		if other == folder || filepath.Base(other) != name {
			continue
		}
		duplicate = true
		if filepath.Base(filepath.Dir(other)) == filepath.Base(parent) {
			return name + " · " + parent
		}
	}
	if duplicate {
		return name + " · " + filepath.Base(parent)
	}
	return name
}

func (m *Model) workspaceLines(w int) []string {
	n := m.workspaceRows()
	if n == 0 {
		return nil
	}
	header := " WORKSPACES"
	if n-1 < len(m.workspaces) {
		header += " ↕"
	}
	lines := []string{row(header, w, m.colors.Muted, m.colors.Panel)}
	for i := m.workspaceTop; i < m.workspaceTop+n-1; i++ {
		folder := m.workspaces[i]
		fg, bg := m.colors.Foreground, m.colors.Panel
		marker := ""
		if folder == m.repo.Root {
			marker, fg = m.symbol(symbolDot), m.colors.Accent
		}
		if i == m.hoveredWorkspace() {
			bg = m.colors.Hover
		}
		gutter := " " + row(marker, 2, fg, bg)
		label := gutter + ansi.Truncate(safe(m.workspaceLabel(folder)), max(0, w-4), "…")
		lines = append(lines, row(label, w, fg, bg))
	}
	return lines
}
