package git

import (
	"context"
	"errors"
	"strings"
)

// Compare with the first parent, including merges; --root handles initial commits.
func (r *Repository) CommitFiles(ctx context.Context, id string) ([]Change, error) {
	if !validObjectID(id) {
		return nil, errors.New("invalid commit object ID")
	}
	raw, err := r.Run(ctx, "diff-tree", "--root", "--diff-merges=first-parent", "--no-commit-id", "--name-status", "-r", "-z", "--find-renames", id, "--")
	if err != nil {
		return nil, err
	}
	fields := strings.Split(strings.TrimSuffix(raw, "\x00"), "\x00")
	var files []Change
	for i := 0; i < len(fields) && fields[i] != ""; {
		status := fields[i]
		i++
		if i >= len(fields) {
			return nil, errors.New("incomplete commit file record")
		}
		c := Change{Path: fields[i], Index: status[0], Worktree: ' '}
		i++
		if c.Index == 'R' || c.Index == 'C' {
			if i >= len(fields) {
				return nil, errors.New("incomplete commit rename record")
			}
			c.Original, c.Path = c.Path, fields[i]
			i++
		}
		files = append(files, c)
	}
	return files, nil
}

func (r *Repository) CommitFileDiff(ctx context.Context, id string, c Change) (string, error) {
	if !validObjectID(id) {
		return "", errors.New("invalid commit object ID")
	}
	r, c, err := r.ForChange(ctx, c)
	if err != nil {
		return "", err
	}
	args := []string{"show", "--format=", "--diff-merges=first-parent", "--root", "--no-ext-diff", "--no-textconv", "--no-color", "--find-renames", "--unified=3", "--submodule=short", id, "--", c.Path}
	if c.Original != "" {
		args = append(args, c.Original)
	}
	return r.Run(ctx, args...)
}

func (r *Repository) readCommits(ctx context.Context) ([]Commit, error) {
	raw, err := r.Run(ctx, "log", "-100", "-z", "--format=%H%x00%h%x00%s")
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if _, headErr := r.Run(ctx, "rev-parse", "--verify", "HEAD"); headErr == nil {
			return nil, err
		}
		return nil, nil // Unborn repository.
	}
	fields := strings.Split(raw, "\x00")
	var commits []Commit
	for i := 0; i+2 < len(fields); i += 3 {
		commits = append(commits, Commit{ID: fields[i], Short: fields[i+1], Subject: fields[i+2]})
	}
	return commits, nil
}
