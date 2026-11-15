package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Repository struct{ Root string }

type Change struct {
	Path, Original  string
	Index, Worktree byte
}

func (c Change) Untracked() bool { return c.Index == '?' && c.Worktree == '?' }
func (c Change) Conflicted() bool {
	return strings.Contains("DD AU UD UA DU AA UU", string([]byte{c.Index, c.Worktree})) && c.Index != ' ' && c.Worktree != ' '
}
func (c Change) Staged() bool   { return c.Index != ' ' && c.Index != '?' }
func (c Change) Unstaged() bool { return c.Worktree != ' ' }

type Commit struct{ ID, Short, Subject string }
type Snapshot struct {
	Branch        string
	Changes       []Change
	Commits       []Commit
	Files         []string
	Head          string
	Detached      bool
	Refs          []Ref
	Remotes       []Remote
	Upstream      string
	Ahead, Behind int
	TrackingKnown bool
	GitHubURL     string
}

func Open(ctx context.Context, path string) (*Repository, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if info, err := os.Stat(abs); err != nil {
		return nil, err
	} else if !info.IsDir() {
		abs = filepath.Dir(abs)
	}
	r := &Repository{Root: abs}
	out, err := r.Run(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}
	r.Root = strings.TrimSuffix(strings.TrimSuffix(out, "\n"), "\r")
	return r, nil
}

func (r *Repository) Run(ctx context.Context, args ...string) (string, error) {
	return r.run(ctx, "", args...)
}

func (r *Repository) run(ctx context.Context, input string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	base := []string{"--no-pager", "-c", "color.ui=false", "-c", "core.quotepath=false"}
	cmd := exec.CommandContext(ctx, "git", append(base, args...)...)
	cmd.Dir = r.Root
	literal := "1"
	if args[0] == "check-ignore" {
		literal = "0"
	}
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_LITERAL_PATHSPECS="+literal)
	// Status polling must not write-refresh the index alongside explicit mutations.
	if args[0] == "status" {
		cmd.Env = append(cmd.Env, "GIT_OPTIONAL_LOCKS=0")
	}
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return stdout.String(), ctx.Err()
	}
	if err != nil {
		return stdout.String(), fmt.Errorf("git %s: %s: %w", args[0], strings.TrimSpace(stderr.String()), err)
	}
	return stdout.String(), nil
}

func ParseStatus(raw string) ([]Change, error) {
	parts := strings.Split(raw, "\x00")
	var changes []Change
	for i := 0; i < len(parts); i++ {
		p := parts[i]
		if p == "" {
			continue
		}
		if len(p) < 4 || p[2] != ' ' {
			return nil, fmt.Errorf("invalid status record %q", p)
		}
		c := Change{Index: p[0], Worktree: p[1], Path: p[3:]}
		if c.Index == 'R' || c.Index == 'C' || c.Worktree == 'R' || c.Worktree == 'C' {
			i++
			if i >= len(parts) || parts[i] == "" {
				return nil, errors.New("missing rename source")
			}
			c.Original = parts[i]
		}
		changes = append(changes, c)
	}
	return changes, nil
}

func (r *Repository) Snapshot(ctx context.Context) (Snapshot, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var s Snapshot
	var statusErr, metadataErr, filesErr error
	var tasks sync.WaitGroup
	tasks.Go(func() {
		raw, err := r.Run(ctx, "status", "--porcelain=v1", "-z", "--untracked-files=all")
		if err == nil {
			s.Changes, err = ParseStatus(raw)
		}
		statusErr = err
		if err != nil {
			cancel()
		}
	})
	tasks.Go(func() {
		metadataErr = r.readMetadata(ctx, &s)
		if metadataErr != nil {
			cancel()
		}
	})
	tasks.Go(func() {
		s.Files, filesErr = r.readFiles(ctx)
		if filesErr != nil {
			cancel()
		}
	})
	tasks.Wait()
	return s, errors.Join(statusErr, metadataErr, filesErr)
}

func (r *Repository) readMetadata(ctx context.Context, s *Snapshot) error {
	branch, err := r.Run(ctx, "symbolic-ref", "--short", "-q", "HEAD")
	if err != nil {
		s.Detached = true
		branch, _ = r.Run(ctx, "rev-parse", "--short", "HEAD")
	}
	s.Branch = strings.TrimSpace(branch)
	if s.Branch == "" {
		s.Branch = "unborn"
	}
	if raw, err := r.Run(ctx, "log", "-100", "-z", "--format=%H%x00%h%x00%s"); err == nil {
		fields := strings.Split(raw, "\x00")
		for i := 0; i+2 < len(fields); i += 3 {
			s.Commits = append(s.Commits, Commit{ID: fields[i], Short: fields[i+1], Subject: fields[i+2]})
		}
	} else if _, headErr := r.Run(ctx, "rev-parse", "--verify", "HEAD"); headErr == nil {
		return err
	}
	if err = r.readReferenceState(ctx, s); err != nil {
		return err
	}
	return nil
}

func (r *Repository) readFiles(ctx context.Context) ([]string, error) {
	raw, err := r.Run(ctx, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	var files []string
	seen := map[string]bool{}
	for _, path := range strings.Split(raw, "\x00") {
		if path != "" && !seen[path] {
			files = append(files, path)
			seen[path] = true
		}
	}
	return files, nil
}

func (r *Repository) Diff(ctx context.Context, c Change, staged bool) (string, error) {
	args := []string{"diff", "--no-ext-diff", "--no-textconv", "--no-color", "--find-renames", "--unified=3"}
	if c.Untracked() {
		out, err := r.Run(ctx, append(args, "--no-index", "--", os.DevNull, filepath.Join(r.Root, c.Path))...)
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			err = nil
		}
		return out, err
	}
	if staged {
		args = append(args, "--cached")
	}
	args = append(args, "--", c.Path)
	if c.Original != "" {
		args = append(args, c.Original)
	}
	return r.Run(ctx, args...)
}

func (r *Repository) CommitDiff(ctx context.Context, id string) (string, error) {
	if !validObjectID(id) {
		return "", errors.New("invalid commit object ID")
	}
	return r.Run(ctx, "show", "--format=fuller", "--no-ext-diff", "--no-textconv", "--no-color", "--find-renames", id, "--")
}

func validObjectID(id string) bool {
	if len(id) != 40 && len(id) != 64 {
		return false
	}
	for _, c := range id {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func (r *Repository) Stage(ctx context.Context, c Change) (string, error) {
	args := []string{"add", "--all", "--", c.Path}
	if c.Original != "" {
		args = append(args, c.Original)
	}
	return r.Run(ctx, args...)
}

func (r *Repository) Unstage(ctx context.Context, c Change) (string, error) {
	args := []string{"reset", "-q", "HEAD", "--", c.Path}
	if _, err := r.Run(ctx, "rev-parse", "--verify", "HEAD"); err != nil {
		args = []string{"rm", "--cached", "--", c.Path}
	}
	if c.Original != "" {
		args = append(args, c.Original)
	}
	return r.Run(ctx, args...)
}

func (r *Repository) ApplyHunk(ctx context.Context, patch string, reverse bool) (string, error) {
	args := []string{"apply", "--cached", "--whitespace=nowarn"}
	if reverse {
		args = append(args, "--reverse")
	}
	return r.run(ctx, patch, args...)
}

func (r *Repository) Ignored(ctx context.Context, paths []string) (map[string]bool, error) {
	result := map[string]bool{}
	if len(paths) == 0 {
		return result, nil
	}
	out, err := r.run(ctx, strings.Join(paths, "\x00")+"\x00", "check-ignore", "--stdin", "-z")
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		err = nil
	}
	if err != nil {
		return nil, err
	}
	for _, path := range strings.Split(out, "\x00") {
		if path != "" {
			result[path] = true
		}
	}
	return result, nil
}
