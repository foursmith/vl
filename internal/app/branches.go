package app

import (
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	gitservice "github.com/foursmith/vl/internal/git"
)

type browserMsg struct {
	target string
	err    error
}

func (m *Model) openBranchPicker(repoPath, mode string) tea.Cmd {
	m.branchRepository = repoPath
	suffix := ""
	if repoPath != "" {
		suffix = " · " + m.repositoryName(repoPath)
	}
	m.palette = nil
	if mode == "" {
		m.palette = append(m.palette,
			action{id: "branch.create", label: m.symbol(symbolAdd) + m.tr(" Create Branch…"), run: func() tea.Cmd {
				m.branchStart = ""
				m.branchRepository = repoPath
				return m.prompt("branch", m.tr("New branch name")+suffix, "")
			}},
			action{id: "branch.createFrom", label: m.symbol(symbolAdd) + m.tr(" Create Branch From…"), run: func() tea.Cmd { return m.openBranchPicker(repoPath, "base") }},
			action{id: "branch.detach", label: m.tr("Checkout to (Detached)…"), run: func() tea.Cmd { return m.openBranchPicker(repoPath, "detach") }})
	}
	for _, ref := range m.repositoryState(repoPath).Refs {
		kind, glyph := m.tr("Branch"), m.symbol(symbolBranch)
		if ref.Kind == gitservice.RemoteBranch {
			kind, glyph = m.tr("Remote Branch"), m.symbol(symbolCloud)
		}
		if ref.Kind == gitservice.Tag {
			kind, glyph = m.tr("Tag"), m.symbol(symbolTag)
		}
		mark := ""
		if ref.Current {
			mark = " " + m.symbol(symbolCheck)
		}
		label := glyph + " " + ref.Name + "  " + shortOID(ref.OID) + mark
		disabled := ""
		if m.hasDirtyTabsForCheckout(repoPath) && mode != "base" && (!ref.Current || mode == "detach") {
			disabled = m.tr("Save or close unsaved files first")
		}
		m.palette = append(m.palette, action{id: ref.FullName, label: label, key: kind, disabled: disabled, run: func() tea.Cmd {
			if mode == "base" {
				m.branchStart = ref.OID
				m.branchRepository = repoPath
				return m.prompt("branch", m.tr("New branch name · Based on %s", ref.Name)+suffix, "")
			}
			return m.checkout(repoPath, ref, mode == "detach")
		}})
	}
	title := m.tr("Select a branch or tag to check out")
	if mode == "base" {
		title = m.tr("Select a starting point for the new branch")
	}
	if mode == "detach" {
		title = m.tr("Select a ref to check out with detached HEAD")
	}
	title += suffix
	cmd := m.prompt("refs", title, "")
	m.input.Placeholder = title + m.tr(" · Enter a name or commit ID")
	return cmd
}

func shortOID(oid string) string { return oid[:min(7, len(oid))] }

func (m *Model) checkout(repoPath string, ref gitservice.Ref, detach bool) tea.Cmd {
	if ref.Current && !detach {
		m.notice = m.tr("Already on branch %s", ref.Name)
		return nil
	}
	if m.hasDirtyTabsForCheckout(repoPath) {
		m.notice = m.tr("Save or close unsaved files first")
		return nil
	}
	if detach {
		ref.Kind = gitservice.Tag
	}
	return m.operation("Switch branch", func() (string, error) {
		repo, err := m.repo.RepositoryAt(m.ctx, repoPath)
		if err != nil {
			return "", err
		}
		return repo.Switch(m.ctx, ref)
	})
}

func (m *Model) syncRepository() tea.Cmd {
	target := m.statusRepository()
	state := m.repositoryState(target)
	if state.Detached {
		m.notice = m.tr("Cannot sync a branch with detached HEAD")
		return nil
	}
	if m.hasDirtyTabs() {
		m.notice = m.tr("Save or close unsaved files first")
		return nil
	}
	if state.Upstream == "" {
		return m.publishBranchInRepository(target)
	}
	return m.operation("Sync", func() (string, error) {
		repo, err := m.repo.RepositoryAt(m.ctx, target)
		if err != nil {
			return "", err
		}
		return repo.Sync(m.ctx)
	})
}

func (m *Model) publishBranch() tea.Cmd {
	return m.publishBranchInRepository(m.statusRepository())
}

func (m *Model) publishBranchInRepository(target string) tea.Cmd {
	state := m.repositoryState(target)
	if state.Detached || state.Head == "" {
		m.notice = m.tr("Create a branch and make an initial commit first")
		return nil
	}
	if len(state.Remotes) == 0 {
		m.notice = m.tr("No remotes configured")
		return nil
	}
	m.palette = nil
	branch := state.Branch
	for _, remote := range state.Remotes {
		m.palette = append(m.palette, action{id: remote.Name, label: remote.Name, key: m.tr("Publish Branch"), run: func() tea.Cmd {
			if m.hasDirtyTabs() {
				m.notice = m.tr("Save or close unsaved files first")
				return nil
			}
			return m.operation("Publish", func() (string, error) {
				repo, err := m.repo.RepositoryAt(m.ctx, target)
				if err != nil {
					return "", err
				}
				return repo.Publish(m.ctx, remote.Name, branch)
			})
		}})
	}
	return m.prompt("remotes", m.tr("Select a remote to publish the branch"), "")
}

func (m *Model) openGitHub() tea.Cmd {
	target := m.repositoryState(m.statusRepository()).GitHubURL
	if target == "" {
		m.notice = m.tr("No GitHub remote found")
		return nil
	}
	return m.openBrowser(target)
}

func (m *Model) openBrowser(target string) tea.Cmd {
	open := m.openURL
	return func() tea.Msg { return browserMsg{target: target, err: open(m.ctx, target)} }
}

func (m *Model) createBranch(name string) tea.Cmd {
	target := m.branchRepository
	if m.hasDirtyTabsForCheckout(target) {
		m.notice = m.tr("Save or close unsaved files first")
		return nil
	}
	start := m.branchStart
	m.branchStart = ""
	return m.operation("Create branch", func() (string, error) {
		repo, err := m.repo.RepositoryAt(m.ctx, target)
		if err != nil {
			return "", err
		}
		return repo.CreateBranch(m.ctx, name, start)
	})
}

// Switching a repository may also update nested worktrees through Git's recurse
// setting, so protect dirty descendants while allowing unrelated parent edits.
func (m *Model) hasDirtyTabsForCheckout(repoPath string) bool {
	if repoPath == "" {
		return m.hasDirtyTabs()
	}
	root := filepath.Join(m.repo.Root, repoPath)
	for _, t := range m.tabs {
		if t.doc == nil || !t.doc.Dirty() {
			continue
		}
		relative, err := filepath.Rel(root, t.doc.Path)
		if err == nil && filepath.IsLocal(relative) {
			return true
		}
	}
	return false
}
