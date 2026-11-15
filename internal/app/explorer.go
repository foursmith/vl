package app

import (
	"maps"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/ourongxing/elegit/internal/files"
)

type treeMsg struct {
	seq     int
	entries []files.Entry
	err     error
}

func (m *Model) refreshTree() tea.Cmd {
	m.treeSeq++
	seq := m.treeSeq
	expanded := maps.Clone(m.expanded)
	return func() tea.Msg {
		entries, err := files.ReadTree(m.ctx, m.repo.Root, expanded)
		if err != nil {
			return treeMsg{seq: seq, err: err}
		}
		paths := make([]string, len(entries))
		for i, entry := range entries {
			paths[i] = entry.Path
		}
		ignored, err := m.repo.Ignored(m.ctx, paths)
		for i := range entries {
			entries[i].Ignored = ignored[entries[i].Path]
		}
		return treeMsg{seq: seq, entries: entries, err: err}
	}
}

func (m *Model) explorerItems() []item {
	if m.rootCollapsed {
		return nil
	}
	var result []item
	for _, entry := range m.treeEntries {
		row := item{label: filepath.Base(entry.Path), key: entry.Path, depth: entry.Depth, ignored: entry.Ignored}
		if entry.Directory {
			row.folder = entry.Path
		} else {
			row.file = entry.Path
		}
		for _, change := range m.snapshot.Changes {
			if change.Path == entry.Path {
				row.status = change.Worktree
				if row.status == ' ' {
					row.status = change.Index
				}
				if change.Untracked() {
					row.status = 'U'
				}
				if change.Conflicted() {
					row.status = '!'
				}
			} else if entry.Directory && strings.HasPrefix(change.Path, entry.Path+"/") {
				if row.status == 0 {
					row.status = 'M'
				}
				if change.Untracked() {
					row.status = 'U'
				}
				if change.Conflicted() {
					row.status = '!'
				}
			}
		}
		result = append(result, row)
	}
	return result
}

func (m *Model) collapseFolders() tea.Cmd {
	m.expanded = map[string]bool{}
	return m.refreshTree()
}

func (m *Model) newEntry(directory bool) tea.Cmd {
	path := ""
	if entry := m.current(); entry != nil {
		if entry.folder != "" {
			path = entry.folder + "/"
		} else if entry.file != "" && filepath.Dir(entry.file) != "." {
			path = filepath.ToSlash(filepath.Dir(entry.file)) + "/"
		}
	}
	kind, title := "newFile", "New File"
	if directory {
		kind, title = "newFolder", "New Folder"
	}
	return m.prompt(kind, title, path)
}

func (m *Model) createEntry(path string, directory bool) tea.Cmd {
	if err := files.Create(m.repo.Root, path, directory); err != nil {
		m.log(err.Error())
		return nil
	}
	m.rootCollapsed = false
	for parent := filepath.ToSlash(filepath.Dir(path)); parent != "." && parent != "/"; parent = filepath.ToSlash(filepath.Dir(parent)) {
		m.expanded[parent] = true
	}
	if !directory {
		m.openFile(path)
	}
	return m.refresh()
}
