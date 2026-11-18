package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/ourongxing/vl/internal/config"
	gitservice "github.com/ourongxing/vl/internal/git"
)

var sidebarSessionNames = [...]string{
	explorer:     "explorer",
	search:       "search",
	changes:      "changes",
	history:      "history",
	pullRequests: "pull-requests",
}

type restoredPullRequest struct {
	version string
	patches map[string]string
}

// SetSession selects the sidebar before discovery and defers editor tab restoration.
func (m *Model) SetSession(session config.Session) {
	if m.editorOnly {
		return
	}
	for index, name := range sidebarSessionNames {
		if session.SidebarTab == name {
			m.section = section(index)
			break
		}
	}
	m.pendingSession = &session
}

func (m *Model) Session() config.Session {
	if m.editorOnly {
		return config.Session{Active: -1}
	}
	sidebarTab := sidebarSessionNames[m.section]
	// Preserve editor tabs if startup was interrupted before discovery.
	if m.pendingSession != nil {
		session := *m.pendingSession
		session.SidebarTab = sidebarTab
		return session
	}
	session := config.Session{Active: m.activeTab, SidebarTab: sidebarTab}
	for _, t := range m.tabs {
		saved := config.SessionTab{Kind: "file", Path: t.path(), Preview: t.preview}
		if d := t.diff; d != nil {
			if d.diffPR.Number != 0 {
				saved.Kind = "pr-diff"
				saved.PullRequest = &config.SessionPullRequest{Number: d.diffPR.Number, URL: d.diffPR.URL, Version: d.diffPRVersion}
			} else {
				saved.Kind, saved.Repository, saved.Commit, saved.Staged = "diff", d.diffRepository, d.diffCommit, d.diffStaged
				saved.Original = d.diffOriginal
			}
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
	prReaders := make(map[config.SessionPullRequest]func() (restoredPullRequest, error))
	for i, saved := range session.Tabs {
		if saved.Path == "" {
			continue
		}
		switch saved.Kind {
		case "file":
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
		case "pr-diff":
			if saved.PullRequest == nil || saved.PullRequest.Number <= 0 || saved.PullRequest.URL == "" {
				continue
			}
			source := *saved.PullRequest
			read := prReaders[source]
			if read == nil {
				repo, workspaceContext := m.repo, m.ctx
				// All tabs for this PR share one read of the workspace's cache.
				read = sync.OnceValues(func() (restoredPullRequest, error) {
					files, version, err := readPullRequestFiles(workspaceContext, repo, source.URL, source.Version)
					if err != nil {
						return restoredPullRequest{}, err
					}
					patches := make(map[string]string, len(files))
					for _, file := range files {
						patches[file.Path] = file.Raw
					}
					return restoredPullRequest{version: version, patches: patches}, nil
				})
				prReaders[source] = read
			}
			target := diffTarget{path: saved.Path, pr: gitservice.PullRequest{Number: source.Number, URL: source.URL}, prVersion: source.Version}
			tab := m.openDiffTab(target)
			commands = append(commands, m.loadVersionedDiff(tab, true, func(ctx context.Context) (string, string, error) {
				if err := ctx.Err(); err != nil {
					return "", "", err
				}
				data, err := read()
				return strings.Clone(data.patches[saved.Path]), data.version, err
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
	if m.section == search {
		commands = append(commands, m.focusSearch(m.search.focus))
	}
	return tea.Batch(commands...)
}
