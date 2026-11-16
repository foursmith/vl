package git

import (
	"context"
	"errors"
	"strconv"
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

const CommitPageSize = 100

// CommitPage keeps subsequent pages anchored to the same history root.
func (r *Repository) CommitPage(ctx context.Context, revision string, offset int) ([]Commit, error) {
	if revision != "HEAD" && !validObjectID(revision) || offset < 0 {
		return nil, errors.New("invalid history page")
	}
	raw, err := r.Run(ctx, "log", "--max-count="+strconv.Itoa(CommitPageSize), "--skip="+strconv.Itoa(offset), "-z", "--format=%H%x00%s", revision, "--")
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if revision == "HEAD" && offset == 0 {
			if _, headErr := r.Run(ctx, "rev-parse", "--verify", "HEAD"); headErr != nil {
				return nil, ctx.Err() // Unborn repository, unless the probe was canceled.
			}
		}
		return nil, err
	}
	fields := strings.Split(raw, "\x00")
	var commits []Commit
	for i := 0; i+1 < len(fields); i += 2 {
		commits = append(commits, Commit{ID: fields[i], Subject: fields[i+1]})
	}
	return commits, nil
}
