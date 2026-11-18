package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	patchdiff "github.com/foursmith/vl/internal/diff"
)

func (r *Repository) Discard(ctx context.Context, change Change) (string, error) {
	r, change, err := r.ForChange(ctx, change)
	if err != nil {
		return "", err
	}
	if change.Conflicted() {
		return "", fmt.Errorf("resolve conflicts before discarding")
	}
	if err := r.checkDiscardStatus(ctx, change); err != nil {
		return "", err
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

func (r *Repository) checkDiscardStatus(ctx context.Context, change Change) error {
	raw, err := r.Run(ctx, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--", change.Path)
	if err != nil {
		return err
	}
	current, err := ParseStatus(raw)
	if err != nil {
		return err
	}
	for _, candidate := range current {
		if candidate.Path == change.Path && !candidate.Conflicted() && candidate.Untracked() == change.Untracked() {
			return nil
		}
	}
	return fmt.Errorf("file status changed; refresh before discarding")
}

func (r *Repository) DiscardLines(ctx context.Context, change Change, source patchdiff.Patch, selected func(*patchdiff.Line) bool) (string, error) {
	r, change, err := r.ForChange(ctx, change)
	if err != nil {
		return "", err
	}
	if err := r.checkDiscardStatus(ctx, change); err != nil {
		return "", err
	}
	current, err := r.Diff(ctx, change, false)
	if err != nil {
		return "", err
	}
	if current != source.Raw {
		return "", fmt.Errorf("Diff changed; refresh before discarding")
	}
	patch, err := source.StagingPatch(change.Path, true, selected)
	if err != nil {
		return "", err
	}
	return r.run(ctx, patch, "apply", "--whitespace=nowarn")
}
