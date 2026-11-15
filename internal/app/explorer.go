package app

import (
	"maps"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/ourongxing/elegit/internal/files"
)

type treeMsg struct {
	seq     int
	entries []files.Entry
	err     error
}

func (m *Model) openFileInExplorer(path string) tea.Cmd {
	if !m.openDocument(path, false) {
		return nil
	}
	return m.revealDocument(path)
}

func (m *Model) revealDocument(path string) tea.Cmd {
	m.section = explorer
	m.sidebarVisible = true
	m.rootCollapsed = false
	for parent := filepath.ToSlash(filepath.Dir(path)); parent != "." && parent != "/"; parent = filepath.ToSlash(filepath.Dir(parent)) {
		m.expanded[parent] = true
	}
	m.revealFile = path
	m.rebuildItems()
	m.resize()
	return m.refreshTree()
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
	// Index statuses once instead of scanning every change for every visible entry.
	statuses := make(map[string]byte, len(m.snapshot.Changes))
	directories := make(map[string]byte)
	for i := range m.snapshot.Changes {
		change := &m.snapshot.Changes[i]
		status := changeStatus(change, change.Worktree == ' ')
		statuses[change.Path] = status
		for parent := filepath.ToSlash(filepath.Dir(change.Path)); parent != "."; parent = filepath.ToSlash(filepath.Dir(parent)) {
			if statusPriority(status) > statusPriority(directories[parent]) {
				directories[parent] = status
			}
		}
	}
	result := make([]item, 0, len(m.treeEntries))
	for _, entry := range m.treeEntries {
		row := item{label: entry.Label, key: entry.Path, depth: entry.Depth, ignored: entry.Ignored, status: statuses[entry.Path]}
		if entry.Directory {
			row.folder = entry.Path
			if statusPriority(directories[entry.Path]) > statusPriority(row.status) {
				row.status = directories[entry.Path]
			}
		} else {
			row.file = entry.Path
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
