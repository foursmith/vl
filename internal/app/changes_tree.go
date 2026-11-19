package app

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	gitservice "github.com/foursmith/vl/internal/git"
)

type changeDirectory struct {
	path        string
	directories map[string]*changeDirectory
	files       []*gitservice.Change
	status      byte
}

func changePrefix(repo string, staged bool) string {
	return fmt.Sprintf("%schanges:%t:", repositoryPrefix(repo), staged)
}

func changeKey(repo string, staged bool, kind, path string) string {
	return changePrefix(repo, staged) + kind + ":" + path
}

func (m *Model) changesSelectionAfterRefresh(previous item) int {
	repository := -1
	for i, row := range m.items {
		if row.repository != previous.repository {
			continue
		}
		switch {
		case row.repoHeader:
			repository = i
		case previous.change != nil && row.change != nil && row.change.Path == previous.change.Path:
			return i
		case previous.folder != "" && row.folder == previous.folder:
			return i
		case previous.header && !previous.repoHeader && row.header:
			return i
		}
	}
	return repository
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
	case 'M', 'D', 'R', 'C', 'T':
		return 3
	case 'U':
		return 2
	case 'A':
		return 1
	}
	return 0
}

func (m *Model) repositoryChangesTree(repo string, staged bool) ([]item, int) {
	if m.sidebarCollapsed[changeKey(repo, staged, "group", "")] {
		count := 0
		for _, c := range m.snapshot.Changes {
			if c.Repository == repo && (staged && c.Staged() || !staged && c.Unstaged()) {
				count++
			}
		}
		return nil, count
	}
	return changeTree(m.snapshot.Changes, repo, staged, m.sidebarCollapsed, changePrefix(repo, staged))
}

func changeTree(changes []gitservice.Change, repo string, staged bool, collapsed map[string]bool, prefix string) ([]item, int) {
	directories := 0
	root := &changeDirectory{directories: map[string]*changeDirectory{}}
	count := 0
	for i := range changes {
		change := &changes[i]
		if change.Repository != repo {
			continue
		}
		if staged && !change.Staged() || !staged && !change.Unstaged() {
			continue
		}
		count++
		parts := strings.Split(repositoryFile(change.Path, repo), "/")
		parent := root
		status := changeStatus(change, staged)
		for i, name := range parts[:len(parts)-1] {
			node := parent.directories[name]
			if node == nil {
				node = &changeDirectory{path: strings.Join(parts[:i+1], "/"), directories: map[string]*changeDirectory{}}
				parent.directories[name] = node
				directories++
			}
			if statusPriority(status) > statusPriority(node.status) {
				node.status = status
			}
			parent = node
		}
		parent.files = append(parent.files, change)
	}
	rows := make([]item, 0, count+directories)
	var visit func(*changeDirectory, int)
	visit = func(parent *changeDirectory, depth int) {
		var names []string
		for name := range parent.directories {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			node := parent.directories[name]
			for len(node.files) == 0 && len(node.directories) == 1 {
				for childName, child := range node.directories {
					name += "/" + childName
					node = child
				}
			}
			key := prefix + "directory:" + node.path
			rows = append(rows, item{label: name, key: key, folder: filepath.ToSlash(filepath.Join(repo, node.path)), repository: repo, depth: depth, staged: staged, status: node.status})
			if !collapsed[key] {
				visit(node, depth+1)
			}
		}
		sort.Slice(parent.files, func(i, j int) bool { return filepath.Base(parent.files[i].Path) < filepath.Base(parent.files[j].Path) })
		for _, change := range parent.files {
			rows = append(rows, item{label: filepath.Base(change.Path), key: prefix + "file:" + change.Path, change: change, repository: repo, staged: staged, depth: depth, status: changeStatus(change, staged)})
		}
	}
	visit(root, 0)
	return rows, count
}
