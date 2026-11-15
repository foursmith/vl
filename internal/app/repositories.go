package app

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	gitservice "github.com/ourongxing/elegit/internal/git"
)

func repositoryKey(path string) string { return "repository:" + path }
func repositoryPrefix(path string) string {
	return repositoryKey(path) + "\x00"
}

func (m *Model) repositoryName(path string) string {
	if path == "" {
		return filepath.Base(m.repo.Root)
	}
	return path
}

func (m *Model) repositoryBranch(path string) string {
	return m.historyData(path).Branch
}

func (m *Model) changeRepositoryItems() []item {
	repositories := append([]string{""}, m.snapshot.Repositories...)
	var result []item
	for _, repo := range repositories {
		key := repositoryKey(repo)
		result = append(result, m.repositoryHeader(repo, key))
		if m.changeCollapsed[key] {
			continue
		}
		for _, staged := range []bool{true, false} {
			rows, count := m.repositoryChangesTree(repo, staged)
			if count == 0 {
				continue
			}
			label := "CHANGES"
			if staged {
				label = "STAGED CHANGES"
			}
			key := changeKey(repo, staged, "group", "")
			result = append(result, item{label: fmt.Sprintf("%s  %d", label, count), key: key, header: true, staged: staged, repository: repo, depth: 1})
			if !m.changeCollapsed[key] {
				for i := range rows {
					rows[i].depth++
				}
				result = append(result, rows...)
			}
		}
	}
	return result
}

func (m *Model) selectedRepository() string {
	if m.section == changes || m.section == history {
		if entry := m.current(); entry != nil {
			return entry.repository
		}
	}
	return ""
}

func (m *Model) commitDisabled(repo string) string {
	if m.busy {
		return "等待当前 Git 操作完成"
	}
	staged := false
	for _, c := range m.snapshot.Changes {
		if c.Repository != repo {
			continue
		}
		if c.Conflicted() {
			return "先解决此仓库的全部冲突"
		}
		staged = staged || c.Staged()
	}
	if !staged {
		return "此仓库没有暂存更改"
	}
	return ""
}

func (m *Model) commitInRepository(repo string) tea.Cmd {
	if reason := m.commitDisabled(repo); reason != "" {
		m.notice = reason
		return nil
	}
	m.commitRepository = repo
	title := "Commit staged changes · " + m.repositoryName(repo)
	if branch := m.repositoryBranch(repo); branch != "" {
		title += " [" + branch + "]"
	}
	return m.prompt("commit", title, m.commitDrafts[repo])
}

func (m *Model) rememberCommitDraft() {
	if m.modal == "commit" {
		m.commitDrafts[m.commitRepository] = m.input.Value()
	}
}

// Gitlink paths stay in their parent's group; only actual child files are
// displayed relative to the child repository.
func repositoryFile(path, repo string) string {
	if repo == "" {
		return path
	}
	return strings.TrimPrefix(path, repo+"/")
}

func (m *Model) hasDirtyTabsInRepository(repo string) bool {
	for _, t := range m.tabs {
		if t.doc == nil || !t.doc.Dirty() {
			continue
		}
		path, err := filepath.Rel(m.repo.Root, t.doc.Path)
		if err != nil {
			continue
		}
		path = filepath.ToSlash(path)
		owner := ""
		for _, candidate := range m.snapshot.Repositories {
			if len(candidate) > len(owner) && strings.HasPrefix(path, candidate+"/") {
				owner = candidate
			}
		}
		if owner == repo {
			return true
		}
	}
	return false
}

func (m *Model) repositoryHeader(repo, key string) item {
	mark := ""
	for _, c := range m.snapshot.Changes {
		if c.Repository == repo {
			mark = "*"
			break
		}
	}
	return item{label: m.repositoryName(repo), key: key, header: true, repoHeader: true, repository: repo, branch: m.repositoryBranch(repo) + mark}
}

func (m *Model) repositoryHeaderPrefix(entry item) string {
	return " " + m.treeChevron(m.folderExpanded(entry)) + " " + m.symbol(symbolRepository) + " "
}

func (m *Model) repositoryBranchText(entry item, width, buttons int) string {
	if entry.branch == "" {
		return ""
	}
	budget := max(0, min(18, width-ansi.StringWidth(m.repositoryHeaderPrefix(entry))-2-buttons-1))
	branch := ansi.Truncate(" "+safe(entry.branch), budget, "…")
	if branch == "" {
		return ""
	}
	return branch + " "
}

func (m *Model) repositoryBranchAt(entry item, x, width int) bool {
	if !entry.repoHeader {
		return false
	}
	buttons := 0
	if m.section == changes {
		buttons = 9
	}
	branchWidth := ansi.StringWidth(m.repositoryBranchText(entry, width, buttons))
	return branchWidth > 0 && x >= width-branchWidth && x < width
}

// File ownership uses a directory boundary and the deepest known repository.
// A gitlink itself belongs to its parent, rather than the child it represents.
func (m *Model) fileRepository(path string) string {
	if filepath.IsAbs(path) {
		var err error
		path, err = filepath.Rel(m.repo.Root, path)
		if err != nil {
			return ""
		}
	}
	path = filepath.ToSlash(filepath.Clean(path))
	owner := ""
	for _, candidate := range m.snapshot.Repositories {
		if len(candidate) > len(owner) && strings.HasPrefix(path, candidate+"/") {
			owner = candidate
		}
	}
	return owner
}

func (m *Model) statusRepository() string {
	if m.editing() {
		return m.fileRepository(m.document().Path)
	}
	if m.diffOpen {
		return m.diffRepository
	}
	for _, repo := range m.snapshot.Repositories {
		if repo == m.lastFileRepository {
			return repo
		}
	}
	return ""
}

func (m *Model) repositoryState(repo string) gitservice.Snapshot {
	if repo == "" {
		return m.snapshot
	}
	return m.snapshot.RepositoryStates[repo]
}
