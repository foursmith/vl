package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// nestedRepository checks the marker before discovery: an empty, uninitialized
// submodule directory would otherwise resolve back to its parent repository.
func nestedRepository(ctx context.Context, root string) (*Repository, error) {
	if _, err := os.Lstat(filepath.Join(root, ".git")); err != nil {
		return nil, err
	}
	repo, err := Open(ctx, root)
	if err != nil {
		return nil, err
	}
	real, err := filepath.EvalSymlinks(root)
	if err != nil || repo.Root != real {
		return nil, fmt.Errorf("repository root does not match directory %q", root)
	}
	return repo, nil
}

// Ignore rules belong to the repository containing a file. Sending a child's
// paths to its superproject's check-ignore fails with "path is in submodule".
func (r *Repository) Ignored(ctx context.Context, paths []string) (map[string]bool, error) {
	if !r.HasRepository() {
		return map[string]bool{}, ctx.Err()
	}
	type group struct {
		repo  *Repository
		paths []string
	}
	groups := map[string]*group{r.Root: {repo: r}}
	owners := map[string]*Repository{".": r}
	var owner func(string) *Repository
	owner = func(dir string) *Repository {
		if repo, ok := owners[dir]; ok {
			return repo
		}
		repo := owner(filepath.Dir(dir))
		target := filepath.Join(r.Root, dir)
		if _, err := os.Lstat(filepath.Join(target, ".git")); err == nil {
			// A broken/uninitialized child must not prevent browsing its files.
			if child, err := nestedRepository(ctx, target); err == nil {
				repo = child
			}
		}
		owners[dir] = repo
		return repo
	}
	for _, path := range paths {
		if !filepath.IsLocal(path) {
			return nil, fmt.Errorf("path is outside the workspace")
		}
		repo := owner(filepath.Dir(filepath.Clean(path)))
		if groups[repo.Root] == nil {
			groups[repo.Root] = &group{repo: repo}
		}
		local, err := filepath.Rel(repo.Root, filepath.Join(r.Root, path))
		if err != nil {
			return nil, err
		}
		groups[repo.Root].paths = append(groups[repo.Root].paths, filepath.ToSlash(local))
	}
	result := map[string]bool{}
	for _, group := range groups {
		if len(group.paths) == 0 {
			continue
		}
		ignored, err := group.repo.ignored(ctx, group.paths)
		if err != nil {
			if strings.Contains(err.Error(), "is in submodule") {
				continue
			}
			return nil, err
		}
		for path := range ignored {
			relative, err := filepath.Rel(r.Root, filepath.Join(group.repo.Root, path))
			if err != nil {
				return nil, err
			}
			result[filepath.ToSlash(relative)] = true
		}
	}
	return result, ctx.Err()
}

// RepositoryAt resolves a nested repository without falling back to its parent.
func (r *Repository) RepositoryAt(ctx context.Context, path string) (*Repository, error) {
	if path == "" {
		return r, nil
	}
	if !filepath.IsLocal(path) || filepath.Clean(path) == "." {
		return nil, fmt.Errorf("invalid repository path")
	}
	target := filepath.Join(r.Root, path)
	real, err := filepath.EvalSymlinks(target)
	if err != nil {
		return nil, err
	}
	relative, err := filepath.Rel(r.Root, real)
	if err != nil || !filepath.IsLocal(relative) {
		return nil, fmt.Errorf("repository is outside the workspace")
	}
	return nestedRepository(ctx, target)
}

// ForChange resolves the owner and converts workspace paths to local paths.
func (r *Repository) ForChange(ctx context.Context, c Change) (*Repository, Change, error) {
	repo, err := r.RepositoryAt(ctx, c.Repository)
	if err != nil {
		return nil, c, err
	}
	if c.Repository == "" {
		return repo, c, nil
	}
	prefix := filepath.ToSlash(filepath.Clean(c.Repository)) + "/"
	for _, path := range []string{c.Path, c.Original} {
		if path != "" && (!strings.HasPrefix(path, prefix) || !filepath.IsLocal(strings.TrimPrefix(path, prefix))) {
			return nil, c, fmt.Errorf("file does not belong to its repository")
		}
	}
	c.Path = strings.TrimPrefix(c.Path, prefix)
	c.Original = strings.TrimPrefix(c.Original, prefix)
	c.Repository = ""
	return repo, c, nil
}

type workspaceChanges struct {
	changes                []Change
	repositories, metadata []string
	states                 map[string]Snapshot
}

func (r *Repository) readChanges(ctx context.Context) (workspaceChanges, error) {
	state := workspaceChanges{states: map[string]Snapshot{}}
	raw, err := r.Run(ctx, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignore-submodules=none")
	if err != nil {
		return state, err
	}
	state.changes, err = ParseStatus(raw)
	if err != nil {
		return state, err
	}
	raw, err = r.Run(ctx, "ls-files", "--stage", "-z")
	if err != nil {
		return state, err
	}
	links := map[string]string{}
	for _, record := range strings.Split(raw, "\x00") {
		meta, path, ok := strings.Cut(record, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 || fields[0] != "160000" {
			continue
		}
		if _, seen := links[path]; !seen || fields[2] == "0" {
			links[path] = fields[1]
		}
	}
	var children []Change
	for path, oid := range links {
		for i := range state.changes {
			if state.changes[i].Path == path {
				state.changes[i].Gitlink = true
			}
		}
		child, err := r.RepositoryAt(ctx, path)
		if err != nil {
			continue
		} // Uninitialized children remain ordinary empty folders.
		nested, err := child.readChanges(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return state, ctx.Err()
			}
			continue
		}
		var metadata Snapshot
		historyErr := child.readMetadata(ctx, &metadata)
		if historyErr != nil {
			metadata.MetadataError = historyErr.Error()
		}
		state.states[path] = metadata
		for descendant, data := range nested.states {
			state.states[filepath.ToSlash(filepath.Join(path, descendant))] = data
		}
		state.repositories = append(state.repositories, path)
		for _, descendant := range nested.repositories {
			state.repositories = append(state.repositories, filepath.ToSlash(filepath.Join(path, descendant)))
		}
		dirs, err := child.MetadataDirectories(ctx)
		if err == nil {
			state.metadata = append(state.metadata, dirs...)
		}
		state.metadata = append(state.metadata, nested.metadata...)
		for _, c := range nested.changes {
			c.Path = filepath.ToSlash(filepath.Join(path, c.Path))
			if c.Original != "" {
				c.Original = filepath.ToSlash(filepath.Join(path, c.Original))
			}
			c.Repository = filepath.ToSlash(filepath.Join(path, c.Repository))
			children = append(children, c)
		}
		// Keep the parent submodule summary separate from the child file list.
		if metadata.Head != "" && metadata.Head == oid {
			for i := range state.changes {
				c := &state.changes[i]
				if c.Path == path && !c.Conflicted() && c.Worktree == 'M' {
					c.GitlinkDirty = true
				}
			}
		}
	}
	state.changes = append(state.changes, children...)
	sort.Slice(state.changes, func(i, j int) bool { return state.changes[i].Path < state.changes[j].Path })
	sort.Strings(state.repositories)
	sort.Strings(state.metadata)
	return state, ctx.Err()
}
