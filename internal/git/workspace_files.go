package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/foursmith/vl/internal/files"
)

func (r *Repository) listWorkspaceFiles(ctx context.Context) ([]string, error) {
	var paths []string
	err := r.walkWorkspace(ctx, r, ".", workspaceWalk{file: func(path string) {
		paths = append(paths, path)
	}})
	return paths, err
}

// WorkspaceRepositories discovers nested worktrees for Git-scoped operations.
func (r *Repository) WorkspaceRepositories(ctx context.Context) ([]*Repository, error) {
	var repositories []*Repository
	err := r.walkWorkspace(ctx, r, ".", workspaceWalk{repository: func(repo *Repository) {
		repositories = append(repositories, repo)
	}})
	return repositories, err
}

type workspaceWalk struct {
	file       func(string)
	repository func(*Repository)
}

func (r *Repository) walkWorkspace(ctx context.Context, repo *Repository, prefix string, walk workspaceWalk) error {
	if repo == nil || !repo.HasRepository() {
		if walk.file == nil {
			return ctx.Err()
		}
		paths, err := files.List(ctx, filepath.Join(r.Root, prefix))
		for _, path := range paths {
			walk.file(filepath.ToSlash(filepath.Join(prefix, path)))
		}
		return err
	}

	var included map[string]bool
	if walk.file != nil {
		var err error
		included, err = repo.includedWorktreePaths(ctx)
		if err != nil {
			return err
		}
	} else {
		walk.repository(repo)
	}
	return r.walkRepositoryFiles(ctx, repo, prefix, "", included, walk)
}

func (r *Repository) walkRepositoryFiles(ctx context.Context, repo *Repository, prefix, local string, included map[string]bool, walk workspaceWalk) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path := filepath.Join(repo.Root, local)
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	if local != "" {
		// Directory entries tell us whether discovery is needed. Ordinary
		// directories need no extra .git lookup.
		hasMarker := false
		for _, entry := range entries {
			if strings.EqualFold(entry.Name(), ".git") {
				hasMarker = true
				break
			}
		}
		if hasMarker || included[local] {
			child, err := nestedRepository(ctx, path)
			if !os.IsNotExist(err) || included[local] {
				relative := filepath.Join(prefix, local)
				return r.walkWorkspace(ctx, child, relative, walk)
			}
		}
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.Name() == ".git" {
			continue
		}
		child := filepath.Join(local, entry.Name())
		if _, allowed := included[child]; included != nil && !allowed {
			continue
		}
		if entry.IsDir() {
			if err := r.walkRepositoryFiles(ctx, repo, prefix, child, included, walk); err != nil {
				return err
			}
		} else if walk.file != nil {
			relative := child
			if prefix != "." {
				relative = filepath.Join(prefix, child)
			}
			walk.file(filepath.ToSlash(relative))
		}
	}
	return nil
}

// The values distinguish paths Git lists directly from their ancestor
// directories. An entry listed as a file or gitlink may be a directory on disk.
func (r *Repository) includedWorktreePaths(ctx context.Context) (map[string]bool, error) {
	raw, err := r.Run(ctx, "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--deduplicate")
	if err != nil {
		return nil, fmt.Errorf("read ignore scope in %s: %w", r.Root, err)
	}
	included := map[string]bool{}
	for name := range strings.SplitSeq(raw, "\x00") {
		if name == "" || !filepath.IsLocal(name) {
			continue
		}
		path := filepath.Clean(name)
		included[path] = true
		for dir := filepath.Dir(path); dir != "."; dir = filepath.Dir(dir) {
			if _, ok := included[dir]; ok {
				break
			}
			included[dir] = false
		}
	}
	return included, nil
}
