package git

import (
	"context"
	"fmt"
	"strings"
)

const githubHost = "github.com"

func ValidGitHubRepositoryName(name string) bool {
	return validGitHubPathPart(name) && !strings.HasSuffix(strings.ToLower(name), ".git")
}

func (r *Repository) GitHubPublishOwner(ctx context.Context) (string, error) {
	out, err := r.runGH(ctx, "api", "--hostname", githubHost, "user", "--jq", ".login")
	if err != nil {
		return "", err
	}
	owner := strings.TrimSpace(string(out))
	if !validGitHubPathPart(owner) {
		return "", fmt.Errorf("gh did not return a GitHub account name")
	}
	return owner, nil
}

func (r *Repository) PublishToGitHub(ctx context.Context, name, branch string, private bool) (string, error) {
	owner, repo, _ := strings.Cut(name, "/")
	if !validGitHubPathPart(owner) || !ValidGitHubRepositoryName(repo) {
		return "", fmt.Errorf("invalid GitHub repository name")
	}
	if err := r.checkPublishBranch(ctx, branch); err != nil {
		return "", err
	}
	if _, err := r.Run(ctx, "rev-parse", "--verify", "HEAD^{commit}"); err != nil {
		return "", err
	}
	remotes, err := r.Run(ctx, "remote")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(remotes) != "" {
		return "", fmt.Errorf("remotes changed; reopen the publish picker")
	}

	protocol, err := r.runGH(ctx, "config", "get", "git_protocol", "--host", githubHost)
	if err != nil {
		return "", err
	}
	visibility := "--private"
	if !private {
		visibility = "--public"
	}
	// Leave local configuration and pushing to Git. Worktrees and submodules
	// may store the Git directory outside the checked-out repository.
	out, err := r.runGH(ctx, "repo", "create", visibility, "--", githubHost+"/"+name)
	if err != nil {
		return "", err
	}
	url := strings.TrimSpace(string(out))
	if !strings.EqualFold(url, "https://"+githubHost+"/"+name) {
		return "", fmt.Errorf("gh returned an unexpected repository URL: %s", url)
	}
	remoteURL := url + ".git"
	if strings.TrimSpace(string(protocol)) == "ssh" {
		remoteURL = "git@" + githubHost + ":" + name + ".git"
	}
	if _, err := r.Run(ctx, "remote", "add", "origin", remoteURL); err != nil {
		return "", fmt.Errorf("created %s, but could not add origin: %w", url, err)
	}
	output, err := r.Publish(ctx, "origin", branch)
	if err != nil {
		return output, fmt.Errorf("created %s; publish the branch to origin to retry: %w", url, err)
	}
	return output, nil
}
