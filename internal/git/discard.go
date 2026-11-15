package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (r *Repository) Discard(ctx context.Context, change Change) (string, error) {
	r, change, err := r.ForChange(ctx, change)
	if err != nil {
		return "", err
	}
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
		index, err := r.Run(ctx, "ls-files", "--stage", "-z", "--", change.Path)
		if err != nil {
			return "", err
		}
		for _, record := range strings.Split(index, "\x00") {
			if strings.HasPrefix(record, "160000 ") {
				return "", fmt.Errorf("a submodule directory cannot be discarded as a file")
			}
		}
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
