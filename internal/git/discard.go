package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

func (r *Repository) Discard(ctx context.Context, change Change) (string, error) {
	if change.Conflicted() {
		return "", fmt.Errorf("resolve conflicts before discarding")
	}
	raw, err := r.Run(ctx, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--", change.Path)
	if err != nil {
		return "", err
	}
	current, err := ParseStatus(raw)
	if err != nil {
		return "", err
	}
	var selected *Change
	for i := range current {
		if current[i].Path == change.Path {
			selected = &current[i]
			break
		}
	}
	if selected == nil || selected.Conflicted() || selected.Untracked() != change.Untracked() {
		return "", fmt.Errorf("file status changed; refresh before discarding")
	}
	if !change.Untracked() {
		if info, err := os.Lstat(filepath.Join(r.Root, change.Path)); err == nil && info.IsDir() {
			return "", fmt.Errorf("only individual files can be discarded")
		}
		return r.Run(ctx, "restore", "--worktree", "--", change.Path)
	}
	clean := filepath.Clean(change.Path)
	if filepath.IsAbs(clean) || !filepath.IsLocal(clean) {
		return "", fmt.Errorf("file is outside the repository")
	}
	path := filepath.Join(r.Root, clean)
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	root, err := filepath.EvalSymlinks(r.Root)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, parent)
	if err != nil || rel != "." && !filepath.IsLocal(rel) {
		return "", fmt.Errorf("parent is outside the repository")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
		return "", fmt.Errorf("only individual files can be discarded")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return "", os.Remove(path)
}
