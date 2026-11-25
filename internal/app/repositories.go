package app

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	gitservice "github.com/foursmith/vl/internal/git"
	"github.com/foursmith/vl/internal/theme"
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
	state := m.repositoryState(path)
	if state.Detached {
		return shortOID(state.Head) + m.tr(" (detached)")
	}
	return state.Branch
}

func (m *Model) changeRepositoryItems() []item {
	repositories := append([]string{""}, m.snapshot.Repositories...)
	var result []item
	for _, repo := range repositories {
		key := repositoryKey(repo)
		result = append(result, m.repositoryHeader(repo, key))
		if m.sidebarCollapsed[key] {
			continue
		}
		for _, staged := range []bool{true, false} {
			rows, count := m.repositoryChangesTree(repo, staged)
			if count == 0 {
				continue
			}
			label := m.tr("CHANGES")
			if staged {
				label = m.tr("STAGED CHANGES")
			}
			key := changeKey(repo, staged, "group", "")
			result = append(result, item{label: fmt.Sprintf("%s  %d", label, count), key: key, header: true, staged: staged, repository: repo, depth: 1})
			if !m.sidebarCollapsed[key] {
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
		return m.tr("Wait for the current Git operation to finish")
	}
	staged := false
	for _, c := range m.snapshot.Changes {
		if c.Repository != repo {
			continue
		}
		if c.Conflicted() {
			return m.tr("Resolve all conflicts in this repository first")
		}
		staged = staged || c.Staged()
	}
	if !staged {
		return m.tr("No staged changes in this repository")
	}
	return ""
}

func (m *Model) commitInRepository(repo string) tea.Cmd {
	if reason := m.commitDisabled(repo); reason != "" {
		m.notice = reason
		return nil
	}
	m.commitRepository = repo
	m.prompt("commit", m.tr("Commit staged changes"), m.commitDrafts[repo])
	return nil
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
		if m.fileRepository(t.doc.Path) == repo {
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
	return m.sidebarHeaderIndent() + m.treeChevron(m.folderExpanded(entry)) + " " + m.symbol(symbolRepository) + " "
}

type repositoryRowLayout struct {
	prefix, branch string
	actions        []rowAction
	labelWidth     int
	branchStart    int
	width          int
}

func (m *Model) layoutRepositoryRow(entry item, width int) repositoryRowLayout {
	layout := repositoryRowLayout{prefix: m.repositoryHeaderPrefix(entry), width: width, actions: m.repositoryRowActions(entry)}
	buttonWidth := 0
	for _, action := range layout.actions {
		buttonWidth += action.displayWidth()
	}
	labelMinWidth := ansi.StringWidth(layout.prefix) + 2
	if m.section == history && width < labelMinWidth+buttonWidth {
		layout.actions, buttonWidth = nil, 0
	}
	if entry.branch != "" {
		budget := max(0, min(18, width-labelMinWidth-buttonWidth-1))
		if branch := ansi.Truncate(" "+safe(entry.branch), budget, "…"); branch != "" {
			layout.branch = branch + " "
		}
	}
	layout.branchStart = width - ansi.StringWidth(layout.branch)
	layout.labelWidth = layout.branchStart - buttonWidth
	layoutRowActions(layout.actions, layout.branchStart)
	return layout
}

func (layout repositoryRowLayout) branchAt(x int) bool {
	return x >= layout.branchStart && x < layout.width
}

func (m *Model) repositoryRow(entry item, width int, bg string) (string, string) {
	layout := m.layoutRepositoryRow(entry, width)
	fg := m.colors.SideForeground
	label := row(layout.prefix+safe(entry.label), layout.labelWidth, fg, bg)
	return theme.Style(fg, bg).Bold(true).Render(label) +
		m.renderRowActions(layout.actions, bg) +
		theme.Style(m.colors.Foreground, bg).Bold(true).Render(layout.branch), fg
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
