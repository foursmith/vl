package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	gitservice "github.com/ourongxing/vl/internal/git"
	"github.com/ourongxing/vl/internal/i18n"
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

func (m *Model) changeActionDisabled(entry item, id string, scope []gitservice.Change) string {
	if m.busy {
		return m.tr("Wait for the current Git operation to finish")
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
			if m.fileDirty(c.Path) {
				return m.tr("Save files in this scope first")
			}
		}
	}
	return ""
}

func (m *Model) runScopeAction(entry item, id string) tea.Cmd {
	scope := m.scopedChanges(entry)
	if len(scope) == 0 {
		return nil
	}
	if id == "stage" {
		stageable := scope[:0]
		for _, c := range scope {
			if !c.GitlinkDirty {
				stageable = append(stageable, c)
			}
		}
		if len(stageable) == 0 {
			m.notice = m.tr("Submodule has file changes; stage and commit them in its repository")
			return nil
		}
		scope = stageable
	}
	if id == "open" {
		for _, c := range scope {
			if !c.Gitlink && c.Worktree != 'D' && !(c.Index == 'D' && c.Worktree == ' ') {
				m.openFile(c.Path)
			}
		}
		return nil
	}
	run := func() tea.Cmd {
		// Recheck editor state when a confirmation is accepted.
		if reason := m.changeActionDisabled(entry, id, scope); reason != "" {
			m.notice = reason
			return nil
		}
		title := map[string]string{"stage": "Stage", "unstage": "Unstage Changes", "discard": "Discard"}[id]
		language := m.displayLanguage
		return m.operation(title, func() (string, error) {
			for i, c := range scope {
				var err error
				switch id {
				case "stage":
					_, err = m.repo.Stage(m.ctx, c)
				case "unstage":
					_, err = m.repo.Unstage(m.ctx, c)
				case "discard":
					_, err = m.repo.Discard(m.ctx, c)
				}
				if err != nil {
					return "", fmt.Errorf(i18n.Text(language, "Completed %d/%d, %s: %w"), i, len(scope), c.Path, err)
				}
			}
			return "", nil
		})
	}
	if id != "discard" {
		return run()
	}
	name := entry.folder
	if name == "" {
		name = m.tr("%s changes", m.repositoryName(entry.repository))
	}
	cmd := m.ask(m.tr("Discard changes to %d files in %s?", len(scope), name), run)
	m.confirmationDetail = m.tr("Only affects this scope. Untracked files will be deleted and cannot be recovered by Git.")
	return cmd
}
