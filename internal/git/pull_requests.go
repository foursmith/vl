package git

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/MoeForge/moego/json"
)

type PullRequest struct {
	Number     int
	Title      string
	State      string
	IsDraft    bool
	URL        string
	UpdatedAt  time.Time
	BaseRefOID string
	HeadRefOID string
}

func (pr PullRequest) DiffVersion() string {
	if pr.BaseRefOID != "" && pr.HeadRefOID != "" {
		return pr.BaseRefOID + ":" + pr.HeadRefOID
	}
	if pr.UpdatedAt.IsZero() {
		return ""
	}
	return pr.UpdatedAt.UTC().Format(time.RFC3339Nano)
}

type PullRequestList struct {
	Requests   []PullRequest
	OpenCount  int
	NextCursor string
}

type PullRequestFilter string

const (
	PullRequestOpen   PullRequestFilter = "OPEN"
	PullRequestMerged PullRequestFilter = "MERGED"
	PullRequestClosed PullRequestFilter = "CLOSED"
	PullRequestAll    PullRequestFilter = "ALL"
)

const pullRequestQuery = `
query($owner: String!, $name: String!, $states: [PullRequestState!], $after: String) {
  repository(owner: $owner, name: $name) {
    recent: pullRequests(first: 50, states: $states, after: $after, orderBy: { field: CREATED_AT, direction: DESC }) {
      nodes { number title state isDraft url updatedAt baseRefOid headRefOid }
      pageInfo { hasNextPage endCursor }
    }
    opened: pullRequests(states: [OPEN], first: 1) { totalCount }
  }
}`

func (r *Repository) PullRequests(ctx context.Context, filter PullRequestFilter, after string) (PullRequestList, error) {
	host, nameWithOwner, err := r.pullRequestRepository(ctx)
	if err != nil {
		return PullRequestList{}, err
	}
	owner, name, _ := strings.Cut(nameWithOwner, "/")
	args := []string{"api", "graphql", "--hostname", host, "-f", "owner=" + owner, "-f", "name=" + name, "-f", "query=" + pullRequestQuery}
	if filter != PullRequestAll {
		args = append(args, "-F", "states[]="+string(filter))
	}
	if after != "" {
		args = append(args, "-f", "after="+after)
	}
	out, err := r.runGH(ctx, args...)
	if err != nil {
		return PullRequestList{}, err
	}
	var response struct {
		Data struct {
			Repository struct {
				Recent struct {
					Nodes    []PullRequest
					PageInfo struct {
						HasNextPage bool
						EndCursor   string
					}
				}
				Opened struct{ TotalCount *int }
			}
		}
	}
	if err := json.Unmarshal(out, &response); err != nil {
		return PullRequestList{}, fmt.Errorf("decode gh response: %w", err)
	}
	repo := response.Data.Repository
	if repo.Recent.Nodes == nil || repo.Opened.TotalCount == nil {
		return PullRequestList{}, fmt.Errorf("gh response is missing pull request data")
	}
	list := PullRequestList{Requests: repo.Recent.Nodes, OpenCount: *repo.Opened.TotalCount}
	if info := repo.Recent.PageInfo; info.HasNextPage {
		if info.EndCursor == "" || info.EndCursor == after {
			return PullRequestList{}, fmt.Errorf("gh response is missing a new pull request cursor")
		}
		list.NextCursor = info.EndCursor
	}
	return list, nil
}

func (r *Repository) pullRequestRepository(ctx context.Context) (host, nameWithOwner string, err error) {
	remotes, err := r.readRemotes(ctx)
	if err != nil {
		return "", "", err
	}
	if len(remotes) == 1 {
		remote := remotes[0]
		if target, err := GitHubRepositoryURL(remote.URL); err == nil {
			resolved, _ := r.Run(ctx, "config", "--get", "remote."+remote.Name+".gh-resolved")
			if resolved = strings.TrimSpace(resolved); resolved == "" || resolved == "base" {
				return "github.com", strings.TrimPrefix(target, "https://github.com/"), nil
			}
		}
	}
	// Let gh honor the configured default when a workspace has several remotes
	// or uses a custom GitHub host.
	out, err := r.runGH(ctx, "repo", "view", "--json", "nameWithOwner,url")
	if err != nil {
		return "", "", err
	}
	var repo struct{ NameWithOwner, URL string }
	if err := json.Unmarshal(out, &repo); err != nil {
		return "", "", fmt.Errorf("decode gh repository: %w", err)
	}
	u, err := url.Parse(repo.URL)
	owner, name, _ := strings.Cut(repo.NameWithOwner, "/")
	if err != nil || u.Hostname() == "" || owner == "" || name == "" {
		return "", "", fmt.Errorf("invalid GitHub repository in gh response")
	}
	return u.Hostname(), repo.NameWithOwner, nil
}

func (r *Repository) PullRequestDiff(ctx context.Context, url string) (string, error) {
	out, err := r.runGH(ctx, "pr", "diff", "--color", "never", "--", url)
	return string(out), err
}

func (r *Repository) PullRequestDiffVersion(ctx context.Context, target string) (string, error) {
	out, err := r.runGH(ctx, "pr", "view", "--json", "baseRefOid,headRefOid,updatedAt", "--", target)
	if err != nil {
		return "", err
	}
	var pr PullRequest
	if err := json.Unmarshal(out, &pr); err != nil {
		return "", fmt.Errorf("decode gh pull request version: %w", err)
	}
	if version := pr.DiffVersion(); version != "" {
		return version, nil
	}
	return "", fmt.Errorf("gh response is missing the pull request version")
}

func (r *Repository) runGH(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Dir = r.Root
	// Let gh resolve the workspace's GitHub remote, even when launched from a
	// shell with a repository override for another project.
	cmd.Env = slices.DeleteFunc(cmd.Environ(), func(value string) bool {
		return strings.HasPrefix(value, "GH_REPO=")
	})
	cmd.Env = append(cmd.Env, "GH_PROMPT_DISABLED=1")
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			if stderr := strings.TrimSpace(string(exit.Stderr)); stderr != "" {
				return nil, fmt.Errorf("gh: %s", stderr)
			}
		}
		return nil, err
	}
	return out, nil
}
