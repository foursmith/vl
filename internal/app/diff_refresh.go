package app

import (
	"context"
	"path/filepath"
	"slices"

	tea "charm.land/bubbletea/v2"
	gitservice "github.com/foursmith/vl/internal/git"
)

func (d *diffState) tracksChanges() bool {
	return d != nil && d.read != nil && d.diffCommit == "" && d.diffPR.Number == 0
}

func (m *Model) invalidateDiffs() {
	for _, t := range m.tabs {
		if t.diff.tracksChanges() {
			t.diff.diffStale = true
		}
	}
}

func (m *Model) invalidateWorktreeDiffs(path string) {
	for _, t := range m.tabs {
		d := t.diff
		if !d.tracksChanges() || d.diffStaged {
			continue
		}
		file := filepath.Join(m.repo.Root, filepath.FromSlash(d.diffPath))
		if coversPath(path, file) || d.diffChange != nil && d.diffChange.Gitlink && coversPath(file, path) {
			d.diffStale = true
		}
		if d.diffOriginal != "" && coversPath(path, filepath.Join(m.repo.Root, filepath.FromSlash(d.diffOriginal))) {
			d.diffStale = true
		}
	}
}

func (m *Model) invalidateIndexDiffs(dir string) {
	repository := ""
	for path, state := range m.snapshot.RepositoryStates {
		if len(state.GitDirectories) > 0 && state.GitDirectories[0] == dir {
			repository = path
			break
		}
	}
	for _, t := range m.tabs {
		if d := t.diff; d.tracksChanges() && d.diffRepository == repository {
			d.diffStale = true
		}
	}
}

func (m *Model) reconcileChangeDiffs(previous gitservice.Snapshot) {
	for _, t := range m.tabs {
		d := t.diff
		if !d.tracksChanges() || d.diffChange == nil {
			continue
		}
		old := *d.diffChange
		change := old
		change.Index, change.Worktree = ' ', ' '
		if i := slices.IndexFunc(m.snapshot.Changes, func(c gitservice.Change) bool {
			return c.Path == d.diffPath && c.Repository == d.diffRepository
		}); i >= 0 {
			change = m.snapshot.Changes[i]
		}
		d.diffChange, d.diffOriginal = &change, change.Original
		if old.Index != change.Index || old.Original != change.Original || old.Gitlink != change.Gitlink ||
			!d.diffStaged && (old.Worktree != change.Worktree || old.GitlinkDirty != change.GitlinkDirty) {
			d.diffStale = true
		}
		if d.diffStaged {
			oldHead, head := previous.Head, m.snapshot.Head
			if d.diffRepository != "" {
				oldHead = previous.RepositoryStates[d.diffRepository].Head
				head = m.snapshot.RepositoryStates[d.diffRepository].Head
			}
			d.diffStale = d.diffStale || oldHead != head
		} else if change.Gitlink {
			d.diffStale = d.diffStale || previous.RepositoryStates[d.diffPath].Head != m.snapshot.RepositoryStates[d.diffPath].Head
		}
	}
}

func (m *Model) refreshActiveDiff() tea.Cmd {
	d := m.diffState
	if m.loading || m.busy || !d.tracksChanges() || !d.diffStale {
		return nil
	}
	if d.diffChange != nil {
		workspace, change, staged := m.repo, *d.diffChange, d.diffStaged
		d.read = func(ctx context.Context) (string, error) { return workspace.Diff(ctx, change, staged) }
	}
	return m.loadDiff(d, false)
}
