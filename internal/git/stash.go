package git

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

type Stash struct {
	Index       int
	ID, Subject string
}

func (s Stash) Ref() string { return "stash@{" + strconv.Itoa(s.Index) + "}" }

func (r *Repository) Stashes(ctx context.Context) ([]Stash, error) {
	out, err := r.Run(ctx, "stash", "list", "-z", "--no-show-signature", "--format=%H%x00%gs")
	if err != nil {
		return nil, err
	}
	fields := strings.Split(strings.TrimSuffix(out, "\x00"), "\x00")
	var stashes []Stash
	for i := 0; i+1 < len(fields); i += 2 {
		stashes = append(stashes, Stash{Index: len(stashes), ID: fields[i], Subject: fields[i+1]})
	}
	return stashes, nil
}

type StashFile struct {
	Change
	Commit    string
	Untracked bool
}

func (r *Repository) StashFiles(ctx context.Context, id string) ([]StashFile, error) {
	tracked, err := r.CommitFiles(ctx, id)
	if err != nil {
		return nil, err
	}
	files := make([]StashFile, 0, len(tracked))
	for _, file := range tracked {
		files = append(files, StashFile{Change: file, Commit: id})
	}
	parents, err := r.Run(ctx, "show", "-s", "--no-show-signature", "--format=%P", id, "--")
	if err != nil {
		return nil, err
	}
	if fields := strings.Fields(parents); len(fields) >= 3 {
		untracked, err := r.CommitFiles(ctx, fields[2])
		if err != nil {
			return nil, err
		}
		for _, file := range untracked {
			files = append(files, StashFile{Change: file, Commit: fields[2], Untracked: true})
		}
	}
	return files, nil
}

func (r *Repository) UseStash(ctx context.Context, stash Stash, action string, index bool) (string, error) {
	if action != "apply" && action != "pop" && action != "drop" || !validObjectID(stash.ID) || stash.Index < 0 {
		return "", errors.New("invalid stash operation")
	}
	args := []string{"stash", action}
	if index && action != "drop" {
		args = append(args, "--index")
	}
	if action == "apply" {
		return r.Run(ctx, append(args, stash.ID)...)
	}
	// Resolve the displayed slot again since its number can change outside vl.
	ref := stash.Ref()
	id, err := r.Run(ctx, "rev-parse", "--verify", ref)
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil || strings.TrimSpace(id) != stash.ID {
		return "", errors.New("Stash list changed; reopen it and select the stash again")
	}
	return r.Run(ctx, append(args, ref)...)
}
