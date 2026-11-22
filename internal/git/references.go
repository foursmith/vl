package git

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

type RefKind string

const (
	LocalBranch  RefKind = "branch"
	RemoteBranch RefKind = "remote"
	Tag          RefKind = "tag"
)

type Ref struct {
	Name, FullName, OID, TargetOID string
	Kind                           RefKind
	Current                        bool
}
type Remote struct{ Name, URL string }

func (r *Repository) readReferenceState(ctx context.Context, s *Snapshot) error {
	head, err := r.Run(ctx, "rev-parse", "--verify", "HEAD")
	if err == nil {
		s.Head = strings.TrimSpace(head)
	}
	upstreamRemote, err := r.readRefs(ctx, s)
	if err != nil {
		return err
	}
	if s.Upstream != "" {
		counts, err := r.Run(ctx, "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
		if err == nil {
			fields := strings.Fields(counts)
			if len(fields) == 2 {
				a, e1 := strconv.Atoi(fields[0])
				b, e2 := strconv.Atoi(fields[1])
				if e1 == nil && e2 == nil {
					s.Ahead, s.Behind, s.TrackingKnown = a, b, true
				}
			}
		}
	}
	s.Remotes, err = r.readRemotes(ctx)
	if err != nil {
		return err
	}
	preferences := []string{"origin", ""}
	if upstreamRemote != "" {
		preferences = append([]string{upstreamRemote}, preferences...)
	}
	for _, preferred := range preferences {
		for _, remote := range s.Remotes {
			if preferred != "" && remote.Name != preferred {
				continue
			}
			if parsed, err := GitHubRepositoryURL(remote.URL); err == nil {
				s.GitHubURL = parsed
				return nil
			}
		}
	}
	return nil
}

func (r *Repository) readRefs(ctx context.Context, s *Snapshot) (string, error) {
	raw, err := r.Run(ctx, "for-each-ref", "--sort=-committerdate", "--format=%(refname)%00%(objectname)%00%(HEAD)%00%(symref)%00%(upstream:short)%00%(upstream:remotename)%00%(*objectname)", "refs/heads/", "refs/remotes/", "refs/tags/")
	if err != nil {
		return "", err
	}
	upstreamRemote := ""
	for _, line := range strings.Split(strings.TrimSuffix(raw, "\n"), "\n") {
		fields := strings.Split(line, "\x00")
		if len(fields) != 7 || fields[3] != "" {
			continue
		}
		ref := Ref{FullName: fields[0], OID: fields[1], TargetOID: fields[1], Current: fields[2] == "*"}
		switch {
		case strings.HasPrefix(ref.FullName, "refs/heads/"):
			ref.Kind = LocalBranch
			ref.Name = strings.TrimPrefix(ref.FullName, "refs/heads/")
		case strings.HasPrefix(ref.FullName, "refs/remotes/"):
			ref.Kind = RemoteBranch
			ref.Name = strings.TrimPrefix(ref.FullName, "refs/remotes/")
		case strings.HasPrefix(ref.FullName, "refs/tags/"):
			ref.Kind = Tag
			ref.Name = strings.TrimPrefix(ref.FullName, "refs/tags/")
			// Annotated refs point at a tag object; history matches its peeled target.
			if fields[6] != "" {
				ref.TargetOID = fields[6]
			}
		default:
			continue
		}
		if ref.Current {
			s.Upstream = fields[4]
			upstreamRemote = fields[5]
		}
		s.Refs = append(s.Refs, ref)
	}
	return upstreamRemote, nil
}

// Read each URL through Git (including insteadOf rules), with at most four
// subprocesses at once and deterministic remote ordering.
func (r *Repository) readRemotes(ctx context.Context) ([]Remote, error) {
	raw, err := r.Run(ctx, "remote")
	if err != nil {
		return nil, err
	}
	raw = strings.TrimSuffix(raw, "\n")
	if raw == "" {
		return nil, nil
	}
	names := strings.Split(raw, "\n")
	remotes := make([]Remote, len(names))
	failures := make([]error, len(names))
	for start := 0; start < len(names); start += 4 {
		var tasks sync.WaitGroup
		end := min(start+4, len(names))
		for i := start; i < end; i++ {
			tasks.Go(func() {
				value, err := r.Run(ctx, "remote", "get-url", "--", names[i])
				remotes[i] = Remote{Name: names[i], URL: strings.TrimSpace(value)}
				failures[i] = err
			})
		}
		tasks.Wait()
		for i := start; i < end; i++ {
			if failures[i] != nil {
				return nil, failures[i]
			}
		}
	}
	return remotes, nil
}

func GitHubRepositoryURL(raw string) (string, error) {
	if !strings.Contains(raw, "://") {
		at := strings.LastIndex(raw, "@")
		hostPath := raw[at+1:]
		colon := strings.Index(hostPath, ":")
		if colon < 0 {
			return "", fmt.Errorf("remote is not a GitHub URL")
		}
		raw = "ssh://" + hostPath[:colon] + "/" + hostPath[colon+1:]
	}
	u, err := url.Parse(raw)
	if err != nil || !strings.EqualFold(u.Hostname(), "github.com") {
		return "", fmt.Errorf("remote is not hosted on github.com")
	}
	if u.Scheme != "https" && u.Scheme != "http" && u.Scheme != "ssh" && u.Scheme != "git" {
		return "", fmt.Errorf("unsupported remote URL")
	}
	path := strings.TrimSuffix(strings.Trim(u.Path, "/"), ".git")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid GitHub repository path")
	}
	for _, part := range parts {
		if !validGitHubPathPart(part) {
			return "", fmt.Errorf("invalid GitHub repository path")
		}
	}
	return "https://github.com/" + path, nil
}

func validGitHubPathPart(part string) bool {
	if part == "" || part == "." || part == ".." {
		return false
	}
	for _, r := range part {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' || r == '.') {
			return false
		}
	}
	return true
}

func (r *Repository) Switch(ctx context.Context, ref Ref) (string, error) {
	if !validObjectID(ref.OID) {
		return "", fmt.Errorf("invalid reference object ID")
	}
	current, err := r.Run(ctx, "rev-parse", "--verify", ref.FullName)
	if err != nil || strings.TrimSpace(current) != ref.OID {
		return "", fmt.Errorf("reference changed; reopen the branch picker")
	}
	switch ref.Kind {
	case LocalBranch:
		return r.Run(ctx, "switch", "--", ref.Name)
	case RemoteBranch:
		name := strings.TrimPrefix(ref.FullName, "refs/remotes/")
		slash := strings.Index(name, "/")
		if slash < 0 {
			return "", fmt.Errorf("invalid remote branch")
		}
		local := name[slash+1:]
		if _, err := r.Run(ctx, "show-ref", "--verify", "--quiet", "refs/heads/"+local); err == nil {
			return "", fmt.Errorf("local branch %s already exists; select it or create a branch from this remote", local)
		}
		return r.Run(ctx, "switch", "--track", "-c", local, ref.FullName)
	case Tag:
		return r.Run(ctx, "switch", "--detach", ref.OID)
	}
	return "", fmt.Errorf("unsupported reference type")
}

func (r *Repository) CreateBranch(ctx context.Context, name, start string) (string, error) {
	if _, err := r.Run(ctx, "check-ref-format", "--branch", name); err != nil || strings.HasPrefix(name, "-") {
		return "", fmt.Errorf("invalid branch name")
	}
	args := []string{"switch", "-c", name}
	if start != "" {
		if !validObjectID(start) {
			return "", fmt.Errorf("invalid start commit")
		}
		args = append(args, start)
	}
	return r.Run(ctx, args...)
}

func (r *Repository) Sync(ctx context.Context) (string, error) {
	head, err := r.Run(ctx, "symbolic-ref", "-q", "HEAD")
	if err != nil {
		return "", fmt.Errorf("sync requires a branch")
	}
	head = strings.TrimSpace(head)
	tracking, err := r.Run(ctx, "for-each-ref", "--format=%(upstream:remotename)%00%(upstream:remoteref)", head)
	if err != nil {
		return "", err
	}
	parts := strings.Split(strings.TrimSuffix(tracking, "\n"), "\x00")
	if len(parts) != 2 || parts[0] == "" || !strings.HasPrefix(parts[1], "refs/heads/") {
		return "", fmt.Errorf("branch has no upstream; publish it first")
	}
	pull, err := r.Run(ctx, "pull", "--ff-only")
	if err != nil {
		return pull, err
	}
	current, err := r.Run(ctx, "symbolic-ref", "-q", "HEAD")
	if err != nil || strings.TrimSpace(current) != head {
		return pull, fmt.Errorf("active branch changed during pull; push was skipped")
	}
	// An explicit refspec keeps push.default=matching from publishing other branches.
	push, err := r.Run(ctx, "push", "--", parts[0], head+":"+parts[1])
	return pull + push, err
}

func (r *Repository) Publish(ctx context.Context, remote, branch string) (string, error) {
	if err := r.checkPublishBranch(ctx, branch); err != nil {
		return "", err
	}
	return r.Run(ctx, "push", "--set-upstream", "--", remote, "refs/heads/"+branch+":refs/heads/"+branch)
}

func (r *Repository) checkPublishBranch(ctx context.Context, branch string) error {
	current, err := r.Run(ctx, "symbolic-ref", "--short", "-q", "HEAD")
	if err != nil || branch == "" || strings.TrimSpace(current) != branch {
		return fmt.Errorf("active branch changed; reopen the publish picker")
	}
	return nil
}
