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

	"github.com/ourongxing/vl/internal/files"
)

type Repository struct{ Root string }

// HasRepository checks the workspace itself, including initialization performed
// outside vl after an ordinary folder was opened.
func (r *Repository) HasRepository() bool {
	_, err := os.Lstat(filepath.Join(r.Root, ".git"))
	return !os.IsNotExist(err)
}

type Change struct {
	Path, Original  string
	Repository      string // Owning repository path relative to the workspace root.
	Gitlink         bool
	GitlinkDirty    bool
	Index, Worktree byte
}

func (c Change) Untracked() bool { return c.Index == '?' && c.Worktree == '?' }
func (c Change) Conflicted() bool {
	return strings.Contains("DD AU UD UA DU AA UU", string([]byte{c.Index, c.Worktree})) && c.Index != ' ' && c.Worktree != ' '
}
func (c Change) Staged() bool   { return c.Index != ' ' && c.Index != '?' }
func (c Change) Unstaged() bool { return c.Worktree != ' ' }

type Commit struct{ ID, Subject string }
type Snapshot struct {
	MetadataError    string
	NoRepository     bool
	Branch           string
	Changes          []Change
	Commits          []Commit
	Files            []string
	Head             string
	Detached         bool
	Refs             []Ref
	Remotes          []Remote
	Upstream         string
	Ahead, Behind    int
	TrackingKnown    bool
	GitHubURL        string
	Repositories     []string
	GitDirectories   []string
	RepositoryStates map[string]Snapshot
}

// Open resolves repository paths to their root and accepts ordinary folders.
func Open(ctx context.Context, path string) (*Repository, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		abs = filepath.Dir(abs)
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, err
	}
	r := &Repository{Root: abs}
	out, err := r.Run(ctx, "rev-parse", "--show-toplevel")
	if err == nil {
		r.Root = strings.TrimSuffix(strings.TrimSuffix(out, "\n"), "\r")
		return r, nil
	}
	if strings.Contains(err.Error(), "fatal: not a git repository (or any") {
		// A Git marker means discovery failed on a damaged repository.
		// Preserve that error instead of treating it as an ordinary folder.
		for dir := abs; ; dir = filepath.Dir(dir) {
			if _, markerErr := os.Lstat(filepath.Join(dir, ".git")); !os.IsNotExist(markerErr) {
				break
			}
			if filepath.Dir(dir) == dir {
				return r, nil
			}
		}
	}
	return nil, fmt.Errorf("open workspace: %w", err)
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
	// Stash uses Git-generated pathspecs to clean the files it saves.
	if args[0] == "check-ignore" || args[0] == "stash" {
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
	if !r.HasRepository() {
		paths, err := files.List(ctx, r.Root)
		return Snapshot{NoRepository: true, Files: paths}, err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var s Snapshot
	var statusErr, metadataErr, filesErr error
	var tasks sync.WaitGroup
	tasks.Go(func() {
		state, err := r.readChanges(ctx)
		s.Changes, s.Repositories, s.GitDirectories = state.changes, state.repositories, state.metadata
		s.RepositoryStates = state.states
		if err == nil {
			var dirs []string
			dirs, err = r.MetadataDirectories(ctx)
			s.GitDirectories = append(s.GitDirectories, dirs...)
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
		listing, err := r.ListWorkspaceFiles(ctx, false)
		s.Files, filesErr = listing.Paths, err
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
	s.Commits, err = r.CommitPage(ctx, "HEAD", 0)
	if err != nil {
		return err
	}
	if err = r.readReferenceState(ctx, s); err != nil {
		return err
	}
	return nil
}

func (r *Repository) Diff(ctx context.Context, c Change, staged bool) (string, error) {
	r, c, err := r.ForChange(ctx, c)
	if err != nil {
		return "", err
	}

	args := []string{"diff", "--no-ext-diff", "--no-textconv", "--no-color", "--find-renames", "--unified=3", "--submodule=short", "--ignore-submodules=none"}
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
	if c.GitlinkDirty {
		return "", fmt.Errorf("stage internal files in their own repository")
	}
	r, c, err := r.ForChange(ctx, c)
	if err != nil {
		return "", err
	}

	args := []string{"add", "--all", "--", c.Path}
	if c.Original != "" {
		args = append(args, c.Original)
	}
	return r.Run(ctx, args...)
}

func (r *Repository) Unstage(ctx context.Context, c Change) (string, error) {
	r, c, err := r.ForChange(ctx, c)
	if err != nil {
		return "", err
	}

	args := []string{"reset", "-q", "--", c.Path}
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

func (r *Repository) ignored(ctx context.Context, paths []string) (map[string]bool, error) {
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
