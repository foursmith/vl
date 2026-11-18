package app

import (
	"context"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/ourongxing/vl/internal/config"
	gitservice "github.com/ourongxing/vl/internal/git"
)

// SetSession defers tab restoration until repository discovery completes.
func (m *Model) SetSession(session config.Session) {
	if m.editorOnly {
		return
	}
	m.pendingSession = &session
}

func (m *Model) Session() config.Session {
	if m.editorOnly {
		return config.Session{Active: -1}
	}
	// Preserve the saved session if startup was interrupted before discovery.
	if m.pendingSession != nil {
		return *m.pendingSession
	}
	session := config.Session{Active: m.activeTab}
	for _, t := range m.tabs {
		saved := config.SessionTab{Kind: "file", Path: t.path(), Preview: t.preview}
		if t.image != nil {
			saved.Kind = "image"
		}
		if d := t.diff; d != nil {
			saved.Kind, saved.Repository, saved.Commit, saved.Staged = "diff", d.diffRepository, d.diffCommit, d.diffStaged
			saved.Original = d.diffOriginal
		}
		session.Tabs = append(session.Tabs, saved)
	}
	return session
}

func (m *Model) restoreSession() tea.Cmd {
	if m.pendingSession == nil {
		return nil
	}
	session := *m.pendingSession
	m.pendingSession = nil
	// Do not replace tabs opened while startup discovery was in progress.
	if len(m.tabs) != 0 {
		return nil
	}
	var commands []tea.Cmd
	active := -1
	previews := map[int]bool{}
	for i, saved := range session.Tabs {
		if saved.Path == "" {
			continue
		}
		switch saved.Kind {
		case "file", "image":
			if !m.openDocument(saved.Path, false) {
				continue
			}
		case "diff":
			// Deleted files still have valid diffs; only the owning repository must exist.
			if _, err := os.Stat(filepath.Join(m.repo.Root, saved.Repository)); err != nil {
				continue
			}
			c := gitservice.Change{Path: saved.Path, Repository: saved.Repository, Original: saved.Original, Index: ' ', Worktree: ' '}
			for _, current := range m.snapshot.Changes {
				if current.Path == c.Path && current.Repository == c.Repository && saved.Commit == "" {
					c = current
					break
				}
			}
			target := diffTarget{path: c.Path, repository: c.Repository, commit: saved.Commit, staged: saved.Staged, change: &c}
			commands = append(commands, m.requestDiff(target, func(ctx context.Context) (string, error) {
				if saved.Commit != "" {
					return m.repo.CommitFileDiff(ctx, saved.Commit, c)
				}
				return m.repo.Diff(ctx, c, saved.Staged)
			}))
		default:
			continue
		}
		previews[m.activeTab] = saved.Preview
		if i <= session.Active {
			active = m.activeTab
		}
	}
	// Apply preview flags after insertion, so restoration cannot replace a tab.
	for index, preview := range previews {
		m.tabs[index].preview = preview
	}
	if len(m.tabs) > 0 {
		index := max(0, active)
		if m.tabs[index].preview && m.sidebarVisible {
			m.sidebar = true
			m.selectTab(index)
		} else {
			m.activateTab(index)
		}
	}
	return tea.Batch(commands...)
}
