package app

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	gitservice "github.com/ourongxing/elegit/internal/git"
)

type changeDirectory struct {
	path        string
	directories map[string]*changeDirectory
	files       []*gitservice.Change
	status      byte
}

func changeKey(staged bool, kind, path string) string {
	return fmt.Sprintf("changes:%t:%s:%s", staged, kind, path)
}

func changeStatus(c *gitservice.Change, staged bool) byte {
	if c.Conflicted() {
		return '!'
	}
	if c.Untracked() {
		return 'U'
	}
	if staged {
		return c.Index
	}
	return c.Worktree
}

func statusPriority(status byte) int {
	switch status {
	case '!':
		return 4
	case 'M', 'D', 'R', 'C':
		return 3
	case 'U':
		return 2
	case 'A':
		return 1
	}
	return 0
}

func (m *Model) changesTree(staged bool) ([]item, int) {
	root := &changeDirectory{directories: map[string]*changeDirectory{}}
	count := 0
	for i := range m.snapshot.Changes {
		change := &m.snapshot.Changes[i]
		if staged && !change.Staged() || !staged && !change.Unstaged() {
			continue
		}
		count++
		parts := strings.Split(change.Path, "/")
		parent := root
		status := changeStatus(change, staged)
		for i, name := range parts[:len(parts)-1] {
			node := parent.directories[name]
			if node == nil {
				node = &changeDirectory{path: strings.Join(parts[:i+1], "/"), directories: map[string]*changeDirectory{}}
				parent.directories[name] = node
			}
			if statusPriority(status) > statusPriority(node.status) {
				node.status = status
			}
			parent = node
		}
		parent.files = append(parent.files, change)
	}
	var rows []item
	var visit func(*changeDirectory, int)
	visit = func(parent *changeDirectory, depth int) {
		var names []string
		for name := range parent.directories {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			node := parent.directories[name]
			key := changeKey(staged, "directory", node.path)
			rows = append(rows, item{label: name, key: key, folder: node.path, depth: depth, staged: staged, status: node.status})
			if !m.changeCollapsed[key] {
				visit(node, depth+1)
			}
		}
		sort.Slice(parent.files, func(i, j int) bool { return filepath.Base(parent.files[i].Path) < filepath.Base(parent.files[j].Path) })
		for _, change := range parent.files {
			rows = append(rows, item{label: filepath.Base(change.Path), key: changeKey(staged, "file", change.Path), change: change, staged: staged, depth: depth, status: changeStatus(change, staged)})
		}
	}
	visit(root, 0)
	return rows, count
}

func (m *Model) folderExpanded(entry item) bool {
	if m.section == changes {
		return !m.changeCollapsed[entry.key]
	}
	return m.expanded[entry.folder]
}

func (m *Model) selectParentFolder() {
	entry := m.current()
	if entry == nil {
		return
	}
	path := entry.folder
	if entry.change != nil {
		path = entry.change.Path
	}
	if entry.file != "" {
		path = entry.file
	}
	parent := filepath.ToSlash(filepath.Dir(path))
	if m.section == changes && parent == "." && !entry.header {
		for i, row := range m.items {
			if row.header && row.staged == entry.staged {
				m.selected = i
				m.ensureSidebar()
				return
			}
		}
	}
	for i, row := range m.items {
		if row.folder == parent && (m.section != changes || row.staged == entry.staged) {
			m.selected = i
			m.ensureSidebar()
			return
		}
	}
}
