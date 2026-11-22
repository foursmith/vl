package app

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGitHubPublishPickerCreatesRemoteAndPublishesSelectedRepository(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell executable as gh")
	}
	m, child := childWorkspace(t)
	target := "child space"
	remote := filepath.Join(t.TempDir(), "remote.git")
	url := "https://github.com/example/renamed"
	for _, args := range [][]string{
		{"init", "--bare", remote},
		{"remote", "remove", "origin"},
		{"config", "url." + remote + ".insteadOf", url + ".git"},
	} {
		if _, err := child.Run(m.ctx, args...); err != nil {
			t.Fatal(err)
		}
	}
	drain(t, m, m.refresh())
	bin := t.TempDir()
	script := `#!/bin/sh
set -eu
[ -z "${GH_REPO+x}" ]
[ "$GH_PROMPT_DISABLED" = 1 ]
case "$*" in
  'api --hostname github.com user --jq .login') printf '%s\n' example ;;
  'config get git_protocol --host github.com') printf '%s\n' https ;;
  'repo create --private -- github.com/example/renamed')
    [ "$PWD" = "$VL_PUBLISH_CHILD" ]
    printf '%s\n' 'https://github.com/example/renamed' ;;
  *) printf '%s\n' "Unexpected gh call: $*" >&2; exit 1 ;;
esac
`
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GH_REPO", "example/other")
	t.Setenv("VL_PUBLISH_CHILD", child.Root)

	drain(t, m, m.publishBranchInRepository(target))
	if m.modal != "githubPublish" || m.input.Value() != "child-space" {
		t.Fatal("publish picker did not prefill the selected repository name")
	}
	m.input.SetValue("renamed")
	rows := m.filteredActions()
	if len(rows) != 2 || rows[0].label != "Publish private · example/renamed" || rows[1].label != "Publish public · example/renamed" {
		t.Fatalf("unexpected visibility choices: %+v", rows)
	}
	drain(t, m, press(m, "enter"))
	if state := m.repositoryState(target); state.Upstream != "origin/main" || m.statusNotice() != "Publish completed" {
		t.Fatalf("publish did not set the child's upstream: %q, %s", state.Upstream, m.statusNotice())
	}
	if got, err := child.Run(m.ctx, "config", "--get", "remote.origin.url"); err != nil || strings.TrimSpace(got) != url+".git" {
		t.Fatalf("child origin = %q, %v", got, err)
	}
	if got, err := m.repo.Run(m.ctx, "remote"); err != nil || got != "" {
		t.Fatalf("parent remotes changed: %q, %v", got, err)
	}
}
