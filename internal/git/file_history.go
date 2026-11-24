package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type FileRevision struct {
	Commit
	Change Change
	Date   time.Time
}

type FileHistoryCursor struct {
	Revision string
	Path     string
}

// fileHistoryRepository finds the worktree containing the file, including files
// whose parent directories or worktree entries have since been deleted.
func (r *Repository) fileHistoryRepository(ctx context.Context, path string) (*Repository, string, error) {
	for dir := filepath.Dir(path); dir != "."; dir = filepath.Dir(dir) {
		if _, err := os.Lstat(filepath.Join(r.Root, dir, ".git")); err == nil {
			owner := filepath.ToSlash(dir)
			repo, err := r.RepositoryAt(ctx, owner)
			return repo, owner, err
		}
	}
	return r, "", nil
}

func (r *Repository) FileHistory(ctx context.Context, cursor FileHistoryCursor) ([]FileRevision, FileHistoryCursor, error) {
	if !filepath.IsLocal(cursor.Path) || filepath.Clean(cursor.Path) == "." ||
		cursor.Revision != "HEAD" && !validObjectID(cursor.Revision) {
		return nil, FileHistoryCursor{}, errors.New("invalid file history target")
	}
	cursor.Path = filepath.ToSlash(filepath.Clean(cursor.Path))
	repo, owner, err := r.fileHistoryRepository(ctx, cursor.Path)
	if err != nil {
		return nil, FileHistoryCursor{}, err
	}
	if !repo.HasRepository() {
		return nil, FileHistoryCursor{}, nil
	}
	path := strings.TrimPrefix(cursor.Path, owner+"/")
	revision := cursor.Revision
	if revision == "HEAD" {
		var head string
		head, err = repo.Run(ctx, "rev-parse", "--verify", "--quiet", "HEAD")
		if err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) && exit.ExitCode() == 1 {
				return nil, FileHistoryCursor{}, nil // Unborn branch.
			}
			return nil, FileHistoryCursor{}, err
		}
		revision = strings.TrimSpace(head)
	}
	raw, err := repo.Run(ctx, "log", "--follow", "--find-renames", "--first-parent", "--diff-merges=first-parent",
		"--name-status", "--no-ext-diff", "--no-textconv", "--no-show-signature", "--no-notes", "--root", "-z", "--format=%x00%H%x00%P%x00%ct%x00%s",
		"--max-count="+strconv.Itoa(CommitPageSize), revision, "--", path)
	if err != nil {
		return nil, FileHistoryCursor{}, err
	}
	revisions, next, err := parseFileHistory(raw, owner)
	if len(revisions) < CommitPageSize {
		next = FileHistoryCursor{}
	}
	return revisions, next, err
}

func parseFileHistory(raw, owner string) ([]FileRevision, FileHistoryCursor, error) {
	var revisions []FileRevision
	var next FileHistoryCursor
	invalid := errors.New("invalid file history record")
	// A record starts with an empty field followed by the four header
	// fields. Paths are NUL-terminated and may themselves contain newlines.
	fields := strings.Split(strings.TrimSuffix(raw, "\x00"), "\x00")
	for len(fields) > 1 {
		if len(fields) < 7 || fields[0] != "" || !validObjectID(fields[1]) {
			return nil, FileHistoryCursor{}, invalid
		}
		date, err := strconv.ParseInt(fields[3], 10, 64)
		if err != nil {
			return nil, FileHistoryCursor{}, invalid
		}
		status := strings.TrimPrefix(fields[5], "\n")
		if status == "" || fields[6] == "" {
			return nil, FileHistoryCursor{}, invalid
		}
		record := FileRevision{
			Commit: Commit{ID: fields[1], Subject: fields[4]},
			Change: Change{Path: filepath.ToSlash(filepath.Join(owner, fields[6])), Index: status[0], Worktree: ' ', Repository: owner},
			Date:   time.Unix(date, 0),
		}
		parent, _, _ := strings.Cut(fields[2], " ")
		fields = fields[7:]
		previousPath := record.Change.Path
		if status[0] == 'R' || status[0] == 'C' {
			if len(fields) == 0 || fields[0] == "" {
				return nil, FileHistoryCursor{}, invalid
			}
			record.Change.Original, record.Change.Path = previousPath, filepath.ToSlash(filepath.Join(owner, fields[0]))
			fields = fields[1:]
		}
		revisions = append(revisions, record)
		// Resume at this commit's first parent with the path that
		// existed there. Git's --skip does not work with --follow.
		next = FileHistoryCursor{Revision: parent, Path: previousPath}
	}
	return revisions, next, nil
}
