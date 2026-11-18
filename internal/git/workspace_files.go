package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ourongxing/vl/internal/files"
)

// WorkspaceFiles contains workspace-relative paths and the directory roots
// needed to apply search filters.
type WorkspaceFiles struct {
	Paths           []string
	OrdinaryFolders []string
	Repositories    []*Repository
}

// ListWorkspaceFiles walks the worktree, including directories with their own
// Git metadata. Git supplies the ignore scope for each worktree.
func (r *Repository) ListWorkspaceFiles(ctx context.Context, includeIgnored bool) (WorkspaceFiles, error) {
	var result WorkspaceFiles
	err := r.walkWorkspace(ctx, r, ".", includeIgnored, &result)
	return result, err
}

func (r *Repository) walkWorkspace(ctx context.Context, repo *Repository, prefix string, includeIgnored bool, result *WorkspaceFiles) error {
	if repo == nil || !repo.HasRepository() {
		result.OrdinaryFolders = append(result.OrdinaryFolders, prefix)
		paths, err := files.List(ctx, filepath.Join(r.Root, prefix))
		for _, path := range paths {
			result.Paths = append(result.Paths, filepath.ToSlash(filepath.Join(prefix, path)))
		}
		return err
	}

	result.Repositories = append(result.Repositories, repo)
	var included map[string]bool
	if !includeIgnored {
		var err error
		included, err = repo.includedWorktreePaths(ctx)
		if err != nil {
			return err
		}
	}
	return filepath.WalkDir(repo.Root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == repo.Root {
			return nil
		}
		local, err := filepath.Rel(repo.Root, path)
		if err != nil {
			return err
		}
		listed, allowed := included[local]
		if entry.Name() == ".git" || !includeIgnored && !allowed {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		relative := filepath.Join(prefix, local)
		if !entry.IsDir() {
			result.Paths = append(result.Paths, filepath.ToSlash(relative))
			return nil
		}
		child, err := nestedRepository(ctx, path)
		if os.IsNotExist(err) && !listed {
			return nil
		}
		if err := r.walkWorkspace(ctx, child, relative, includeIgnored, result); err != nil {
			return err
		}
		return filepath.SkipDir
	})
}

// The values distinguish paths Git lists directly from their ancestor
// directories. An entry listed as a file or gitlink may be a directory on disk.
func (r *Repository) includedWorktreePaths(ctx context.Context) (map[string]bool, error) {
	raw, err := r.Run(ctx, "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--deduplicate")
	if err != nil {
		return nil, fmt.Errorf("read ignore scope in %s: %w", r.Root, err)
	}
	included := map[string]bool{}
	for _, name := range strings.Split(raw, "\x00") {
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
