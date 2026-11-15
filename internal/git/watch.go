package git

import (
	"context"
	"path/filepath"
	"strings"
)

// MetadataDirectories resolves actual Git locations, including linked worktrees.
func (r *Repository) MetadataDirectories(ctx context.Context) ([]string, error) {
	var dirs []string
	for _, option := range []string{"--git-dir", "--git-common-dir"} {
		out, err := r.Run(ctx, "rev-parse", "--path-format=absolute", option)
		if err != nil {
			return nil, err
		}
		path := strings.TrimSuffix(strings.TrimSuffix(out, "\n"), "\r")
		if !filepath.IsAbs(path) {
			path = filepath.Join(r.Root, path)
		}
		if real, err := filepath.EvalSymlinks(path); err == nil {
			path = real
		}
		if len(dirs) == 0 || dirs[0] != path {
			dirs = append(dirs, path)
		}
	}
	return dirs, nil
}
