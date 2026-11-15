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
	args := []string{"show", "--format=", "--diff-merges=first-parent", "--root", "--no-ext-diff", "--no-textconv", "--no-color", "--find-renames", "--unified=3", id, "--", c.Path}
	if c.Original != "" {
		args = append(args, c.Original)
	}
	return r.Run(ctx, args...)
}
