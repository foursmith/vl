package app

import (
	"maps"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/ourongxing/vl/internal/files"
)

type treeMsg struct {
	seq     int
	entries []files.Entry
	err     error
}

const explorerRootKey = "."

func (m *Model) openFileInExplorer(path string) tea.Cmd {
	if !m.openDocument(path, false) {
		return nil
	}
	return m.revealDocument(path)
}

func (m *Model) revealDocument(path string) tea.Cmd {
	if m.editorOnly {
		return nil
	}
	m.section = explorer
	m.sidebarVisible = true
	delete(m.sidebarCollapsed, explorerRootKey)
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
	uncompressed := maps.Clone(m.uncompressed)
	return func() tea.Msg {
		entries, err := files.ReadTree(m.ctx, m.repo.Root, expanded, uncompressed)
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
	root := m.repositoryHeader("", explorerRootKey)
	if m.sidebarCollapsed[root.key] {
		return []item{root}
	}
	// Index statuses once instead of scanning every change for every visible entry.
	statuses := make(map[string]byte, len(m.snapshot.Changes))
	directories := make(map[string]byte)
	staged := make(map[string]bool)
	directoryStaged := make(map[string]bool)
	for i := range m.snapshot.Changes {
		change := &m.snapshot.Changes[i]
		status := changeStatus(change, change.Worktree == ' ')
		statuses[change.Path] = status
		staged[change.Path] = change.Worktree == ' '
		for parent := filepath.ToSlash(filepath.Dir(change.Path)); parent != "."; parent = filepath.ToSlash(filepath.Dir(parent)) {
			if statusPriority(status) > statusPriority(directories[parent]) {
				directories[parent] = status
				directoryStaged[parent] = staged[change.Path]
			}
		}
	}
	result := make([]item, 1, len(m.treeEntries)+1)
	result[0] = root
	for _, entry := range m.treeEntries {
		row := item{label: entry.Label, key: entry.Path, depth: entry.Depth + 1, ignored: entry.Ignored, status: statuses[entry.Path], staged: staged[entry.Path]}
		if entry.Directory {
			row.folder = entry.Path
			if statusPriority(directories[entry.Path]) > statusPriority(row.status) {
				row.status = directories[entry.Path]
				row.staged = directoryStaged[entry.Path]
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

func (m *Model) splitFolderChain() tea.Cmd {
	entry := m.current()
	if m.section != explorer || entry == nil || entry.folder == "" || !strings.Contains(entry.label, "/") {
		return nil
	}
	// Keep the final directory's open state; reveal each previously hidden parent.
	path := strings.TrimSuffix(entry.folder, entry.label)
	parts := strings.Split(entry.label, "/")
	for _, part := range parts[:len(parts)-1] {
		path += part
		m.uncompressed[path] = true
		m.expanded[path] = true
		path += "/"
	}
	return m.refreshTree()
}

func (m *Model) selectedEntryParent() string {
	if entry := m.current(); entry != nil {
		if entry.folder != "" {
			return entry.folder
		}
		if entry.file != "" {
			return filepath.Dir(entry.file)
		}
	}
	return "."
}

func (m *Model) newEntry(directory bool) tea.Cmd {
	path := ""
	if parent := m.selectedEntryParent(); parent != "." {
		path = filepath.ToSlash(parent) + "/"
	}
	kind, title := "newFile", m.tr("New File")
	if directory {
		kind, title = "newFolder", m.tr("New Folder")
	}
	return m.prompt(kind, title, path)
}

func (m *Model) createEntry(path string, directory bool) tea.Cmd {
	if err := files.Create(m.repo.Root, path, directory); err != nil {
		m.notice = m.tr(err.Error())
		return nil
	}
	for parent := filepath.ToSlash(filepath.Dir(path)); parent != "." && parent != "/"; parent = filepath.ToSlash(filepath.Dir(parent)) {
		m.expanded[parent] = true
	}
	if !directory {
		m.openFile(path)
	}
	return m.refresh()
}

func (m *Model) selectedEntryPath() string {
	entry := m.current()
	if m.section != explorer || entry == nil {
		return ""
	}
	if entry.folder != "" {
		return entry.folder
	}
	return entry.file
}

func (m *Model) deletePathDirty(path string) bool {
	absolute := filepath.Join(m.repo.Root, path)
	for _, tab := range m.tabs {
		if tab.doc != nil && tab.doc.Dirty() && (tab.doc.Path == absolute || strings.HasPrefix(tab.doc.Path, absolute+string(filepath.Separator))) {
			return true
		}
	}
	return false
}

func (m *Model) deleteSelectedEntry() tea.Cmd {
	path := m.selectedEntryPath()
	if path == "" {
		return nil
	}
	if m.busy {
		m.notice = m.tr("Wait for the current operation to finish")
		return nil
	}
	if m.deletePathDirty(path) {
		m.notice = m.tr("Save or close unsaved changes in this path first")
		return nil
	}
	root := m.repo.Root
	operation := "Delete file"
	title := m.tr("Delete file %s?", path)
	detail := m.tr("This permanently deletes the file from disk. Uncommitted content cannot be recovered by Git.")
	if m.current().folder != "" {
		operation = "Delete folder"
		title = m.tr("Delete folder %s?", path)
		detail = m.tr("This permanently deletes the folder and all its contents from disk. Uncommitted content cannot be recovered by Git.")
	}
	cmd := m.ask(title, func() tea.Cmd {
		if m.repo.Root != root || m.busy || m.deletePathDirty(path) {
			m.notice = m.tr("Path state changed; try again")
			return nil
		}
		return m.startOperation(operation, func() (string, error) {
			return "", files.Delete(root, path)
		})
	})
	m.confirmationDetail = detail
	m.confirmationLabel = m.tr("Delete")
	return cmd
}

// File yanks are separate from the text clipboard used by the editor.
func (m *Model) yankEntry() tea.Cmd {
	path := m.selectedEntryPath()
	if path == "" {
		return nil
	}
	if m.deletePathDirty(path) {
		m.notice = m.tr("Save or close unsaved changes in this path first")
		return nil
	}
	m.explorerYankRoot, m.explorerYankPath = m.repo.Root, path
	m.notice = m.tr("Copied %s", path)
	return nil
}

func (m *Model) pasteEntryPrompt() tea.Cmd {
	if m.explorerYankPath == "" || m.explorerYankRoot != m.repo.Root {
		m.notice = m.tr("Copy an entry in this workspace with yy first")
		return nil
	}
	parent := m.selectedEntryParent()
	name := filepath.Base(m.explorerYankPath)
	destination := filepath.Join(parent, name)
	if destination == filepath.FromSlash(m.explorerYankPath) {
		destination = filepath.Join(parent, name+"-copy")
	}
	return m.prompt("pasteFile", m.tr("Copy to"), filepath.ToSlash(destination))
}

func (m *Model) pasteEntry(destination string) tea.Cmd {
	root, source := m.explorerYankRoot, m.explorerYankPath
	if root != m.repo.Root || source == "" || m.deletePathDirty(source) {
		m.notice = m.tr("Path state changed; try again")
		return nil
	}
	return m.startOperation("Copy file", func() (string, error) { return "", files.Copy(m.ctx, root, source, destination) })
}
