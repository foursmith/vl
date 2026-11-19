package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	gitservice "github.com/foursmith/vl/internal/git"
	"github.com/foursmith/vl/internal/i18n"
)

// Scope membership comes from the full snapshot, including collapsed descendants.
func (m *Model) scopedChanges(entry item) []gitservice.Change {
	if entry.change != nil {
		return []gitservice.Change{*entry.change}
	}
	var result []gitservice.Change
	for _, c := range m.snapshot.Changes {
		if c.Repository != entry.repository {
			continue
		}
		if entry.staged && !c.Staged() || !entry.staged && !c.Unstaged() {
			continue
		}
		if entry.folder != "" && !strings.HasPrefix(c.Path, entry.folder+"/") {
			continue
		}
		result = append(result, c)
	}
	return result
}

func (m *Model) discardScopeReason(entry *item) string {
	if entry == nil || entry.staged {
		return m.tr("Select an unstaged item or repository to discard changes")
	}
	scope := item{repoHeader: true, repository: entry.repository}
	return m.changeActionDisabled(scope, "discard", m.scopedChanges(scope))
}

func (m *Model) changeActionDisabled(entry item, id string, scope []gitservice.Change) string {
	if m.busy {
		return m.tr("Wait for the current Git operation to finish")
	}
	if entry.change != nil && (id == "stage" || id == "unstage") {
		return m.stageFileReason(*entry.change, id == "unstage")
	}
	if id == "stage" && entry.repoHeader && m.hasDirtyTabsInRepository(entry.repository) {
		return m.tr("Save edited files before staging all changes")
	}
	if len(scope) == 0 {
		return m.tr("No changes in this scope")
	}
	switch id {
	case "open":
		if entry.change != nil && entry.change.Gitlink {
			return m.tr("Submodule commit pointer; view the diff or select files in the submodule")
		}
		if c := entry.change; c != nil && (c.Worktree == 'D' || c.Index == 'D' && c.Worktree == ' ') {
			return m.tr("File deleted")
		}
	case "discard":
		for _, c := range scope {
			if c.Gitlink {
				return m.tr("Cannot discard a repository directory as a file")
			}
			if c.Conflicted() {
				return m.tr("Resolve file conflicts first")
			}
		}
		if m.hasDirtyTabsInRepository(entry.repository) {
			return m.tr("Save or close unsaved files first")
		}
	case "stage":
		stageable := false
		for _, c := range scope {
			stageable = stageable || !c.GitlinkDirty
		}
		if !stageable {
			return m.tr("Submodule has file changes; stage and commit them in its repository")
		}
		for _, c := range scope {
			if !c.GitlinkDirty && m.fileDirty(c.Path) {
				return m.tr("Save files in this scope first")
			}
		}
	}
	return ""
}

func (m *Model) runScopeAction(entry item, id string) tea.Cmd {
	scope := m.scopedChanges(entry)
	if reason := m.changeActionDisabled(entry, id, scope); reason != "" {
		m.notice = reason
		return nil
	}
	if id == "stage" {
		stageable := scope[:0]
		for _, c := range scope {
			if !c.GitlinkDirty {
				stageable = append(stageable, c)
			}
		}
		return m.stageChanges(stageable, false, "Stage")
	}
	if id == "unstage" {
		return m.stageChanges(scope, true, "Unstage Changes")
	}
	if id == "open" {
		for _, c := range scope {
			if !c.Gitlink && c.Worktree != 'D' && !(c.Index == 'D' && c.Worktree == ' ') {
				m.openFile(c.Path)
			}
		}
		return nil
	}
	if id != "discard" {
		return nil
	}
	run := func() tea.Cmd {
		// Recheck editor state when a confirmation is accepted.
		if reason := m.changeActionDisabled(entry, id, scope); reason != "" {
			m.notice = reason
			return nil
		}
		language := m.displayLanguage
		return m.operation("Discard", func() (string, error) {
			for i, c := range scope {
				if _, err := m.repo.Discard(m.ctx, c); err != nil {
					return "", fmt.Errorf(i18n.Text(language, "Completed %d/%d, %s: %w"), i, len(scope), c.Path, err)
				}
			}
			return "", nil
		})
	}
	name := entry.folder
	if name == "" {
		name = m.tr("%s changes", m.repositoryName(entry.repository))
	}
	title := m.tr("Discard changes to %d files in %s?", len(scope), name)
	if entry.repoHeader {
		title = m.tr("Discard all unstaged changes in %s (%d files)?", m.repositoryName(entry.repository), len(scope))
	}
	cmd := m.askDiscard(title, run)
	if entry.repoHeader {
		m.confirmationButton = 1
	}
	m.confirmationDetail = m.tr("Only affects this scope. Untracked files will be deleted and cannot be recovered by Git.")
	return cmd
}
