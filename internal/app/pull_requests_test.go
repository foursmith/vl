package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/foursmith/vl/internal/config"
	gitservice "github.com/foursmith/vl/internal/git"
)

func TestPullRequestsInitializeAndPublishEmptyState(t *testing.T) {
	m := folderWorkspace(t)
	drain(t, m, press(m, "ctrl+5"))
	sidebar := func() string { return plain(strings.Join(m.sidebarLines(), "\n")) }
	if !strings.Contains(sidebar(), "Initialize Repository ↵") {
		t.Fatal("PR tab did not offer initialization")
	}
	drain(t, m, press(m, "enter"))
	if m.snapshot.NoRepository || m.canRefreshPullRequestList() ||
		!strings.Contains(sidebar(), "Make an initial commit") ||
		!strings.Contains(sidebar(), "Publish to GitHub ↵") {
		t.Fatal("PR tab did not offer publishing after initialization")
	}
	drain(t, m, press(m, "enter"))
	if m.modal != "" || m.notice != "Make an initial commit before publishing." {
		t.Fatalf("publishing before the first commit did not explain the requirement: %q", m.notice)
	}
	if _, err := m.repo.Run(m.ctx, "-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgSign=false",
		"commit", "--allow-empty", "-m", "Initial commit"); err != nil {
		t.Fatal(err)
	}
	branch := m.snapshot.Branch
	if _, err := m.repo.Run(m.ctx, "switch", "--detach"); err != nil {
		t.Fatal(err)
	}
	drain(t, m, m.refresh())
	drain(t, m, press(m, "enter"))
	if m.modal != "" || m.notice != "Switch to a branch before publishing." ||
		!strings.Contains(sidebar(), "Switch to a branch") {
		t.Fatalf("publishing with a detached HEAD did not explain the requirement: %q", m.notice)
	}
	if _, err := m.repo.Run(m.ctx, "switch", branch); err != nil {
		t.Fatal(err)
	}
	drain(t, m, m.refresh())
	state := m.repositoryEmptyState()
	button := m.repositoryEmptyLayout(state, m.sidebarContentWidth()).button
	x := m.sideX() + button.x
	m.hoverX, m.hoverY = x, button.y
	if m.hoverHint() != "Publish to GitHub · Enter" {
		t.Fatal("missing publish hover hint")
	}
	if cmd := m.clickSidebar(tea.MouseClickMsg{X: x, Y: button.y, Button: tea.MouseLeft}); cmd == nil ||
		m.modal != "githubPublish" || m.githubPublish.target != "" || m.githubPublish.branch != m.snapshot.Branch {
		t.Fatal("clicking Publish did not open the workspace's GitHub publish picker")
	}
}

func TestPullRequestsTabUsesWorkspaceAndPreservesSelection(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell executable as gh")
	}
	m := tabWorkspace(t, 1)
	t.Cleanup(m.Close)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	m.openFile("file0.go")
	if _, err := m.repo.Run(m.ctx, "remote", "add", "origin", "https://github.com/example/repo.git"); err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(m.repo.Root, ".git", "gh-fixture.json")
	if err := os.WriteFile(fixture, []byte(`{"data":{"repository":{"recent":{"nodes":[
		{"number":42,"title":"Fix toolbar","state":"OPEN","isDraft":true,"url":"https://github.com/example/repo/pull/42"},
		{"number":41,"title":"Add History","state":"OPEN","url":"https://github.com/example/repo/pull/41"}
	]},"opened":{"totalCount":2}}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	path := t.TempDir()
	gh := `#!/bin/sh
set -eu
[ -z "${GH_REPO+x}" ]
[ "$GH_PROMPT_DISABLED" = 1 ]
case "$*" in
  'api graphql --hostname github.com -f owner=example -f name=repo -f query='*|\
  'api graphql --hostname git.example.org -f owner=upstream -f name=another -f query='*)
    case "${10}" in
      *'recent: pullRequests(first: 50, states: $states'*'opened: pullRequests(states: [OPEN], first: 1) { totalCount }'*)
        if [ "$VL_PR_FILTER_ARG" = "ALL" ]; then
          case "${12-}" in ''|after=page-2) ;; *) exit 1 ;; esac
        else
          [ "${12-}" = "$VL_PR_FILTER_ARG" ]
        fi
        case "$*" in
          *' -f after=page-2') cat .git/gh-page-2.json ;;
          *) cat .git/gh-fixture.json ;;
        esac ;;
      *) exit 1 ;;
    esac ;;
  'repo view --json nameWithOwner,url')
    printf '%s\n' '{"nameWithOwner":"upstream/another","url":"https://git.example.org/upstream/another"}' ;;
  'pr view --json baseRefOid,headRefOid,updatedAt -- '*)
    printf '%s\n' '{"baseRefOid":"base","headRefOid":"head"}' ;;
  'pr diff --color never -- https://github.com/example/repo/pull/42')
    printf '%s\n' 'diff --git a/file0.go b/file0.go' '--- a/file0.go' '+++ b/file0.go' '@@ -1 +1 @@' '-package main' '+package draft' ;;
  'pr diff --color never -- https://github.com/example/repo/pull/41')
    if [ "${VL_PR_PATCH_ERROR:-}" = 1 ]; then
      printf '%s\n' 'diff unavailable' >&2
      exit 1
    fi
    printf '%s\n' 'diff --git a/src/pkg/file0.go b/src/pkg/file0.go' '--- a/src/pkg/file0.go' '+++ b/src/pkg/file0.go' '@@ -1 +1 @@' '-package main' '+package review' \
      'diff --git a/src/pkg/other file.txt b/src/pkg/other file.txt' 'new file mode 100644' '--- /dev/null' '+++ b/src/pkg/other file.txt' '@@ -0,0 +1 @@' "+$VL_PR_PATCH_LINE" ;;
  *) exit 1 ;;
esac
`
	if err := os.WriteFile(filepath.Join(path, "gh"), []byte(gh), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", path+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GH_REPO", "example/other")
	t.Setenv("VL_PR_PATCH_LINE", "some text")
	t.Setenv("VL_PR_FILTER_ARG", "states[]=OPEN")

	cmd := press(m, "ctrl+5")
	if got := plain(m.sidebarLines()[headerHeight+1]); !strings.Contains(got, "Loading pull requests") {
		t.Fatalf("loading row = %q", got)
	}
	drain(t, m, cmd)
	if m.section != pullRequests || len(m.items) != 3 || !m.items[0].repoHeader {
		t.Fatalf("section = %d, rows = %d, gh error = %v", m.section, len(m.items), m.pullRequestsError)
	}
	if m.openPullRequestCount != 2 {
		t.Fatalf("open PR count = %d, want 2 including the draft", m.openPullRequestCount)
	}
	if got := plain(m.sidebarLines()[headerHeight+1]); !strings.Contains(got, "* #42 Fix toolbar") || !strings.Contains(got, "[Draft]") {
		t.Fatalf("draft row = %q", got)
	}

	t.Run("show cached rows before refreshing", func(t *testing.T) {
		reopened := New(context.Background(), m.repo, false)
		defer reopened.Close()
		cmd := press(reopened, "ctrl+5")
		msg, ok := cmd().(cachedPullRequestsMsg)
		if !ok {
			t.Fatal("opening the PR tab waited for a fresh list")
		}
		_, refresh := reopened.Update(msg)
		if row := plain(reopened.sidebarLines()[headerHeight+1]); !strings.Contains(row, "#42 Fix toolbar") ||
			reopened.openPullRequestCount != 2 || reopened.pullRequestsCancel == nil {
			t.Fatalf("cached row during refresh = %q, count = %d", row, reopened.openPullRequestCount)
		}
		// An unavailable gh must leave the cached list visible.
		savedFixture, err := os.ReadFile(fixture)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(fixture); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := os.WriteFile(fixture, savedFixture, 0600); err != nil {
				t.Error(err)
			}
		})
		drain(t, reopened, refresh)
		if reopened.pullRequestsError == nil || len(reopened.pullRequests) != 2 {
			t.Fatal("failed refresh discarded the cached PRs")
		}
		freshFixture := strings.Replace(string(savedFixture), "Fix toolbar", "Tune toolbar", 1)
		if err := os.WriteFile(fixture, []byte(freshFixture), 0600); err != nil {
			t.Fatal(err)
		}
		_, refresh = reopened.Update(tea.KeyPressMsg{Code: tea.KeyF5})
		drain(t, reopened, refresh)
		if row := plain(reopened.sidebarLines()[headerHeight+1]); !strings.Contains(row, "#42 Tune toolbar") ||
			reopened.pullRequestsError != nil {
			t.Fatalf("row after successful refresh = %q, error = %v", row, reopened.pullRequestsError)
		}
	})

	listFetchedAt := m.pullRequestsFetchedAt
	drain(t, m, press(m, "ctrl+3"))
	drain(t, m, press(m, "ctrl+5"))
	if m.pullRequestsFetchedAt != listFetchedAt {
		t.Fatal("reopening PR tab fetched a fresh list")
	}
	if doc := m.document(); doc == nil || filepath.Base(doc.Path) != "file0.go" {
		t.Fatal("PR lookup replaced the active editor")
	}
	m.focusEditor()
	listSeq, repoSeq := m.pullRequestsSeq, m.refreshSeq
	drain(t, m, m.key(tea.KeyPressMsg{Code: tea.KeyF5}))
	if m.pullRequestsSeq != listSeq || m.refreshSeq != repoSeq+1 {
		t.Fatal("F5 in the text editor did not refresh the workspace independently of the visible PR list")
	}
	m.sidebar = true
	drain(t, m, press(m, "j"))
	if !m.sidebar || len(m.pullRequestReviews) != 0 || m.diffOpen {
		t.Fatal("moving the selection opened or loaded a PR")
	}
	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyF5})
	drain(t, m, cmd)
	if entry := m.current(); entry == nil || entry.pullRequest.Number != 41 {
		t.Fatalf("refresh lost selection: %+v", entry)
	}
	var opened string
	m.openURL = func(_ context.Context, target string) error {
		opened = target
		return nil
	}
	drain(t, m, press(m, "o"))
	if opened != "https://github.com/example/repo/pull/41" || !m.sidebar || m.diffOpen {
		t.Fatalf("o did not open the selected PR with sidebar focus: %q", opened)
	}
	drain(t, m, press(m, "l"))
	if !m.sidebar || m.diffOpen || !m.current().isPullRequest() || !m.pullRequestExpanded(*m.current()) {
		t.Fatal("expanding a PR opened a diff or moved focus")
	}
	drain(t, m, press(m, "j"))
	if folder := m.current(); folder == nil || folder.folder != "src/pkg" || !m.folderExpanded(*folder) {
		t.Fatalf("expected compact folder row, got %+v", folder)
	}
	drain(t, m, press(m, "enter"))
	if len(m.items) != 4 {
		t.Fatal("collapsing the folder left file rows visible")
	}
	drain(t, m, press(m, "enter"))
	drain(t, m, press(m, "j"))
	if !m.sidebar || !m.tabs[m.activeTab].preview || m.diffPR.Number != 41 || m.diffPath != "src/pkg/file0.go" || !strings.Contains(m.patch.Raw, "+package review") {
		t.Fatalf("moving to the first file did not preview its diff: %q", m.patch.Raw)
	}
	if m.diffChange != nil || m.review == nil || !m.review.ReadOnly {
		t.Fatal("PR diff is not read-only")
	}
	drain(t, m, press(m, "j"))
	if !m.sidebar || !m.tabs[m.activeTab].preview || m.diffPath != "src/pkg/other file.txt" || !strings.Contains(m.patch.Raw, "+some text") {
		t.Fatalf("second file diff = %q", m.patch.Raw)
	}
	opened = ""
	drain(t, m, press(m, "o"))
	if opened != "https://github.com/example/repo/pull/41" || !m.sidebar || m.diffPath != "src/pkg/other file.txt" {
		t.Fatalf("o did not open the PR for the selected file: %q", opened)
	}
	t.Setenv("VL_PR_PATCH_LINE", "updated text")
	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyF5})
	drain(t, m, cmd)
	if m.diffPath != "src/pkg/other file.txt" || !strings.Contains(m.patch.Raw, "+updated text") {
		t.Fatalf("refreshed file diff = %q", m.patch.Raw)
	}
	cached := m.pullRequestReview
	m.selectSidebarRow(m.sidebarRowWithKey("pr:42"))
	drain(t, m, press(m, "enter"))
	t.Setenv("VL_PR_PATCH_LINE", "newer text")
	m.selectSidebarRow(m.sidebarRowWithKey("pr:41:file:src/pkg/other file.txt"))
	drain(t, m, press(m, "enter"))
	if m.pullRequestReview != cached || !strings.Contains(m.patch.Raw, "+updated text") {
		t.Fatalf("reopening PR missed the cached patch: %q", m.patch.Raw)
	}

	m.pullRequestsFetchedAt = m.pullRequestsFetchedAt.Add(-pullRequestRefreshInterval)
	if delay := m.pullRequestTickDelay(pullRequestRefreshInterval); delay != 0 {
		t.Fatalf("expired cache tick delay = %s", delay)
	}
	drain(t, m, m.refreshPullRequests(false))
	if !strings.Contains(m.patch.Raw, "+updated text") || m.pullRequestTickDelay(pullRequestRefreshInterval) <= 0 {
		t.Fatalf("automatic list refresh replaced the cached diff or failed to update its timer: %q", m.patch.Raw)
	}
	drain(t, m, m.selectSection(explorer))
	m.focusEditor()
	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyF5})
	drain(t, m, cmd)
	if !strings.Contains(m.patch.Raw, "+newer text") {
		t.Fatalf("manual refresh did not update the cached diff: %q", m.patch.Raw)
	}
	t.Setenv("VL_PR_PATCH_ERROR", "1")
	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyF5})
	drain(t, m, cmd)
	if !strings.Contains(m.notice, "diff unavailable") || !strings.Contains(m.patch.Raw, "+newer text") {
		t.Fatalf("failed refresh did not report the error and preserve the displayed diff: %q, %q", m.notice, m.patch.Raw)
	}
	t.Setenv("VL_PR_PATCH_ERROR", "")
	saved := m.Session()
	if len(saved.Tabs) != 2 || saved.Tabs[0].Kind != "file" || saved.Active != 1 ||
		saved.Tabs[1].Kind != "pr-diff" || saved.Tabs[1].PullRequest.Number != 41 {
		t.Fatalf("saved session = %+v", saved)
	}
	drain(t, m, press(m, "ctrl+5"))
	m.snapshot.Branch = "main"
	m.rebuildItems()
	layout := m.layoutRepositoryRow(m.items[0], m.sidebarContentWidth())
	actions := layout.actions
	clickAction := func(action rowAction) tea.Cmd {
		_, cmd := m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: m.sideX() + action.x + 1, Y: headerHeight})
		return cmd
	}
	for _, action := range actions {
		if !strings.Contains(plain(m.sidebarLines()[headerHeight]), action.glyph) ||
			layout.branchAt(action.x+1) {
			t.Fatal("PR header actions are missing or overlap the branch")
		}
	}
	drain(t, m, clickAction(actions[0]))
	if m.modal != "pullRequestFilter" || m.palette[0].key != "Current" {
		t.Fatal("filter button did not open the state picker with Open selected")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	seq := m.pullRequestsSeq
	cmd = press(m, "enter")
	if m.pullRequestFilter != gitservice.PullRequestAll || len(m.pullRequests) != 0 {
		t.Fatal("changing filters did not reset the displayed results")
	}
	m.Update(pullRequestsMsg{seq: seq, list: gitservice.PullRequestList{OpenCount: 99}})
	if m.openPullRequestCount != 2 || m.pullRequestsCancel == nil {
		t.Fatal("previous filter result replaced the pending list")
	}

	recent := make([]gitservice.PullRequest, 50)
	for i := range recent {
		recent[i] = gitservice.PullRequest{Number: i + 101, State: "CLOSED"}
	}
	data, err := json.Marshal(recent)
	if err != nil {
		t.Fatal(err)
	}
	response := fmt.Sprintf(`{"data":{"repository":{"recent":{"nodes":%s,"pageInfo":{"hasNextPage":true,"endCursor":"page-2"}},"opened":{"totalCount":126}}}}`, data)
	if err := os.WriteFile(fixture, []byte(response), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := m.repo.Run(m.ctx, "remote", "add", "upstream", "https://git.example.org/upstream/another.git"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VL_PR_FILTER_ARG", "ALL")
	drain(t, m, cmd)
	if m.openPullRequestCount != 126 {
		t.Fatalf("badge count with a full page of closed PRs = %d, want 126", m.openPullRequestCount)
	}
	if len(m.pullRequests) != 50 || m.pullRequests[0].State != "CLOSED" {
		t.Fatal("All filter did not display the new results")
	}
	seq = m.pullRequestsSeq
	drain(t, m, clickAction(actions[1]))
	if m.pullRequestsSeq != seq+1 || m.pullRequestsCancel != nil {
		t.Fatal("refresh button did not force a request")
	}

	m.Update(tea.WindowSizeMsg{Width: 120, Height: 18})
	m.sideTop = len(m.items) - m.treeList().height - 3
	_, cmd = m.Update(tea.MouseWheelMsg{X: m.sideX() + 2, Y: headerHeight + 1, Button: tea.MouseWheelDown})
	drain(t, m, cmd)
	if m.pullRequestsPageError == nil || m.pullRequestsCancel != nil {
		t.Fatal("scrolling to the bottom did not attempt the next page")
	}
	if label := m.items[len(m.items)-1].label; !strings.Contains(label, "press Enter to retry") {
		t.Fatalf("missing next-page retry row: %q", label)
	}
	pageTwo := `{"data":{"repository":{"recent":{"nodes":[
		{"number":150,"state":"CLOSED"},{"number":151,"state":"CLOSED"},{"number":152,"state":"CLOSED"}
	]},"opened":{"totalCount":126}}}}`
	if err := os.WriteFile(filepath.Join(m.repo.Root, ".git", "gh-page-2.json"), []byte(pageTwo), 0600); err != nil {
		t.Fatal(err)
	}
	top := m.sideTop
	m.selectSidebarRow(len(m.items) - 1)
	drain(t, m, press(m, "enter"))
	if len(m.pullRequests) != 52 || m.pullRequestsNextCursor != "" ||
		m.current().key != "pr:151" || m.sideTop != top {
		t.Fatalf("next page did not append unique PRs in place: count=%d, top=%d, row=%+v", len(m.pullRequests), m.sideTop, m.current())
	}
	drain(t, m, m.refreshPullRequests(true))
	if len(m.pullRequests) != 52 || m.pullRequestsNextCursor != "" || m.sideTop != top {
		t.Fatal("refreshing the loaded range discarded older PRs or moved the viewport")
	}

	pagePath := filepath.Join(m.repo.Root, ".git", "gh-page-2.json")
	if err := os.Remove(pagePath); err != nil {
		t.Fatal(err)
	}
	reopened := New(context.Background(), m.repo, false)
	defer reopened.Close()
	reopened.pullRequestFilter = gitservice.PullRequestAll
	drain(t, reopened, press(reopened, "ctrl+5"))
	if reopened.pullRequestsError == nil || reopened.openPullRequestCount != 126 || len(reopened.pullRequests) != 52 {
		t.Fatal("a failed second-page refresh did not preserve the complete cached list")
	}
	newPage := strings.ReplaceAll(strings.Replace(pageTwo, "152", "153", 1), "126", "125")
	if err := os.WriteFile(pagePath, []byte(newPage), 0600); err != nil {
		t.Fatal(err)
	}
	pending := reopened.refreshPullRequests(true)
	seq = reopened.pullRequestsSeq
	cmd = press(reopened, "r")
	drain(t, reopened, pending)
	if reopened.pullRequestsSeq != seq+1 || reopened.pullRequestsCancel == nil {
		t.Fatal("r did not supersede the active list refresh")
	}
	drain(t, reopened, cmd)
	if reopened.openPullRequestCount != 125 || len(reopened.pullRequests) != 52 ||
		reopened.sidebarRowWithKey("pr:152") >= 0 || reopened.sidebarRowWithKey("pr:153") < 0 {
		t.Fatal("refresh did not replace the older page and count together")
	}
}

func TestPullRequestDiffCacheAcrossWorkspaces(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell executable as gh")
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	path := t.TempDir()
	gh := `#!/bin/sh
[ -z "${VL_PR_RACE_FILE-}" ] || {
  head=$(cat "$VL_PR_RACE_FILE")
  if [ "$1 $2" = "pr view" ]; then
    printf '{"baseRefOid":"base","headRefOid":"%s"}\n' "$head"
    exit 0
  fi
  if [ "$head" = "before" ]; then printf 'after' > "$VL_PR_RACE_FILE"; fi
  VL_PR_PATCH_LINE="$head"
}
[ "$1 $2" != "pr view" ] || {
  printf '{"baseRefOid":"base","headRefOid":"%s"}\n' "$VL_PR_HEAD"
  exit 0
}
[ "$1 $2" = "pr diff" ] || exit 1
printf '%s\n' 'diff --git a/file.go b/file.go' '--- a/file.go' '+++ b/file.go' '@@ -0,0 +1 @@' "+$VL_PR_PATCH_LINE"
`
	if err := os.WriteFile(filepath.Join(path, "gh"), []byte(gh), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", path+string(os.PathListSeparator)+os.Getenv("PATH"))
	newWorkspace := func() *gitservice.Repository {
		t.Helper()
		repo := &gitservice.Repository{Root: t.TempDir()}
		if _, err := repo.Run(context.Background(), "init", "-b", "main"); err != nil {
			t.Fatal(err)
		}
		return repo
	}
	workspace := newWorkspace()
	pr := gitservice.PullRequest{
		Number: 42, URL: "https://github.com/example/repo/pull/42", BaseRefOID: "base", HeadRefOID: "head",
		UpdatedAt: time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC),
	}
	open := func(repo *gitservice.Repository, want string) {
		t.Helper()
		m := New(context.Background(), repo, false)
		defer m.Close()
		m.section, m.pullRequests = pullRequests, []gitservice.PullRequest{pr}
		m.rebuildItems()
		drain(t, m, press(m, "enter"))
		drain(t, m, press(m, "j"))
		if !strings.Contains(m.patch.Raw, want) {
			t.Fatalf("preview = %q, want %q", m.patch.Raw, want)
		}
	}

	t.Setenv("VL_PR_PATCH_LINE", "first version")
	t.Setenv("VL_PR_HEAD", "head")
	open(workspace, "+first version")
	t.Setenv("VL_PR_PATCH_LINE", "second version")
	pr.UpdatedAt = pr.UpdatedAt.Add(time.Minute)
	open(workspace, "+first version")
	open(newWorkspace(), "+second version")
	pr.HeadRefOID = "new-head"
	t.Setenv("VL_PR_HEAD", "new-head")
	open(workspace, "+second version")
	t.Setenv("VL_PR_PATCH_LINE", "third version")
	open(workspace, "+second version")

	raceFile := filepath.Join(path, "head")
	if err := os.WriteFile(raceFile, []byte("before"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VL_PR_RACE_FILE", raceFile)
	files, version, err := readPullRequestFiles(context.Background(), workspace, pr.URL, "")
	if err != nil || version != "base:after" || len(files) != 1 || !strings.Contains(files[0].Raw, "+after") {
		t.Fatalf("diff after concurrent push: version %q, files %+v, error %v", version, files, err)
	}
	cache := config.NewPullRequestDiffCache(workspace.Root, pullRequestCacheSize)
	if _, ok := cache.Load(pr.URL, "base:before"); ok {
		t.Fatal("cached a diff under the revision that changed during the fetch")
	}
	if raw, ok := cache.Load(pr.URL, "base:after"); !ok || !strings.Contains(raw, "+after") {
		t.Fatal("verified diff was not cached under its revision")
	}
}
