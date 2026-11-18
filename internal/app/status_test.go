package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/vl/internal/config"
	gitservice "github.com/ourongxing/vl/internal/git"
)

func TestStatusBarStylePickerPersistsAndRestores(t *testing.T) {
	m := tabWorkspace(t, 1)
	t.Cleanup(m.Close)
	path := filepath.Join(t.TempDir(), "layout.json")
	m.SetLayoutStore(config.NewLayoutStore(path))

	for _, editorOnly := range []bool{false, true} {
		m.editorOnly = editorOnly
		m.openPalette()
		m.input.SetValue(">Status Bar Style")
		m.syncQuickPicker()
		if rows := m.filteredActions(); len(rows) != 1 || rows[0].id != "statusBar.style" {
			t.Fatalf("status bar style command not discoverable: %#v", rows)
		}
		drain(t, m, m.selectPickerChoice())
		if m.modal != "statusBarStyles" || len(m.palette) != 2 || m.palette[0].key != "Current" {
			t.Fatal("style picker should mark Standard as current")
		}
		m.input.SetValue("Pure")
		drain(t, m, m.selectPickerChoice())
		saved, err := config.LoadLayout(path)
		if err != nil || saved.StatusBarStyle != statusBarPure || m.statusBarStyle != statusBarPure || m.modal != "" {
			t.Fatalf("Pure was not applied and saved: %+v, %v", saved, err)
		}
		m.SetLayout(config.Layout{StatusBarStyle: "unknown"})
		if m.statusBarStyle != statusBarStandard {
			t.Fatal("unknown style should use Standard")
		}
		m.SetLayout(saved)
		m.openStatusBarStylePicker()
		if m.palette[1].key != "Current" {
			t.Fatal("saved Pure was not restored")
		}
		m.input.SetValue("Standard")
		drain(t, m, m.selectPickerChoice())
		saved, err = config.LoadLayout(path)
		if err != nil || saved.StatusBarStyle != statusBarStandard || m.statusBarStyle != statusBarStandard {
			t.Fatalf("Standard was not applied and saved: %+v, %v", saved, err)
		}
	}
}

func branchWorkspace(t *testing.T) *Model {
	t.Helper()
	m := explorerWorkspace(t)
	for _, args := range [][]string{{"config", "user.name", "Test"}, {"config", "user.email", "test@example.test"}, {"config", "commit.gpgsign", "false"}, {"add", "."}, {"commit", "-m", "initial"}, {"switch", "-c", "topic"}} {
		if _, err := m.repo.Run(m.ctx, args...); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(m.repo.Root, "README.md"), []byte("# Topic\n"), 0644)
	for _, args := range [][]string{{"commit", "-am", "topic"}, {"tag", "v1"}, {"switch", "main"}} {
		if _, err := m.repo.Run(m.ctx, args...); err != nil {
			t.Fatal(err)
		}
	}
	drain(t, m, m.refresh())
	return m
}

func TestStatusBranchPickerCheckoutAndCreateFromReference(t *testing.T) {
	m := branchWorkspace(t)
	m.openFile("README.md")
	var branch statusSegment
	for _, s := range m.statusLayout() {
		if s.id == "branch" {
			branch = s
		}
	}
	drain(t, m, m.click(tea.MouseClickMsg{X: branch.x + 1, Y: m.height - 1, Button: tea.MouseLeft}))
	if m.modal != "refs" {
		t.Fatal("branch status did not open picker")
	}
	m.input.SetValue("refs/heads/topic")
	drain(t, m, press(m, "enter"))
	if m.snapshot.Branch != "topic" || m.document().Text != "# Topic\n" {
		t.Fatal("checkout did not update branch and clean editor", m.notice)
	}
	m.openBranchPicker("", "base")
	m.input.SetValue("refs/heads/main")
	drain(t, m, press(m, "enter"))
	if m.modal != "branch" || m.branchStart == "" {
		t.Fatal("base picker did not retain start commit")
	}
	m.input.SetValue("from-main")
	drain(t, m, press(m, "enter"))
	if m.snapshot.Branch != "from-main" || m.document().Text != "# Workspace\n" {
		t.Fatal("new branch did not use chosen start", m.notice)
	}
	m.openBranchPicker("", "")
	m.input.SetValue("refs/tags/v1")
	drain(t, m, press(m, "enter"))
	if !m.snapshot.Detached || !strings.Contains(m.statusLine(), "detached") {
		t.Fatal("tag checkout did not expose detached state")
	}
}

func TestPickerCannotDiscardUnsavedDocument(t *testing.T) {
	m := branchWorkspace(t)
	m.openFile("README.md")
	press(m, "x")
	m.openBranchPicker("", "")
	m.input.SetValue("refs/heads/topic")
	drain(t, m, press(m, "enter"))
	if m.modal != "refs" || m.busy || m.snapshot.Branch != "main" || !m.document().Dirty() {
		t.Fatal("picker bypassed unsaved editor guard")
	}
}

func TestGitHubStatusUsesNormalizedRemoteAndReportsLaunchError(t *testing.T) {
	m := branchWorkspace(t)
	if _, err := m.repo.Run(m.ctx, "remote", "add", "origin", "git@github.com:owner/repo.git"); err != nil {
		t.Fatal(err)
	}
	drain(t, m, m.refresh())
	called := ""
	m.openURL = func(_ context.Context, target string) error { called = target; return nil }
	for _, s := range m.statusLayout() {
		if s.id == "github" {
			drain(t, m, m.click(tea.MouseClickMsg{X: s.x + 1, Y: m.height - 1, Button: tea.MouseLeft}))
			break
		}
	}
	if called != "https://github.com/owner/repo" {
		t.Fatalf("opened wrong URL: %q", called)
	}
	m.openURL = func(context.Context, string) error { return errors.New("no browser") }
	drain(t, m, m.openGitHub())
	if !strings.Contains(m.notice, "no browser") {
		t.Fatal("browser failure was hidden")
	}
	m.snapshot.GitHubURL = ""
	called = ""
	if cmd := m.openGitHub(); cmd != nil || called != "" {
		t.Fatal("launched browser without GitHub remote")
	}
}

func TestStatusGeometryAndEditorMetadata(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openFile("file0.go")
	m.snapshot = gitservice.Snapshot{Branch: "feature/很长的分支", Upstream: "origin/main", Ahead: 3, Behind: 2, TrackingKnown: true, GitHubURL: "https://github.com/owner/repo", Changes: []gitservice.Change{{Path: "a", Index: 'M', Worktree: 'M'}}}
	for _, width := range []int{30, 48, 80, 120, 200} {
		m.Update(tea.WindowSizeMsg{Width: width, Height: 24})
		end := 0
		branch, github := false, false
		for _, s := range m.statusLayout() {
			if s.x < end || s.x+s.width > width {
				t.Fatalf("status segments overlap at %d: %+v", width, s)
			}
			end = s.x + s.width
			branch = branch || s.id == "branch"
			github = github || s.id == "github"
		}
		if !branch || (width >= 48 && !github) || ansi.StringWidth(m.statusLine()) != width {
			t.Fatalf("missing controls or incorrect width %d", width)
		}
	}
	text := plain(m.statusLine())
	for _, value := range []string{"2" + m.symbol(symbolArrowDown) + " 3" + m.symbol(symbolUp), "UTF-8", "LF", "Go", "Ln 1, Col 1"} {
		if !strings.Contains(text, value) {
			t.Fatalf("missing %q: %s", value, text)
		}
	}
}

func TestReferencePickerMouseSelectionAndFrameWidth(t *testing.T) {
	m := branchWorkspace(t)
	m.openBranchPicker("", "")
	m.input.SetValue("refs/heads/topic")
	layout := m.pickerLayout(m.filteredActions())
	drain(t, m, m.clickModal(tea.MouseClickMsg{X: layout.x + 2, Y: layout.y + 3, Button: tea.MouseLeft}))
	if m.snapshot.Branch != "topic" {
		t.Fatal("picker row click missed", m.notice)
	}
	for _, width := range []int{30, 48, 80, 120} {
		m.Update(tea.WindowSizeMsg{Width: width, Height: 24})
		m.openBranchPicker("", "")
		for _, line := range strings.Split(m.View().Content, "\n") {
			if ansi.StringWidth(line) != width {
				t.Fatalf("picker frame width %d: %q", width, plain(line))
			}
		}
	}
}

func TestBranchPickerRejectsStaleCheckout(t *testing.T) {
	m := branchWorkspace(t)
	m.openFile("README.md")
	m.openBranchPicker("", "")
	m.input.SetValue("refs/heads/topic")
	if _, err := m.repo.Run(m.ctx, "branch", "-f", "topic", "main"); err != nil {
		t.Fatal(err)
	}
	drain(t, m, m.selectPickerChoice())
	if m.snapshot.Branch != "main" || !strings.Contains(m.notice, "reference changed") {
		t.Fatal("picker bypassed stale reference guard", m.notice)
	}
	m.document().Insert("// unsaved\n")
	m.openBranchPicker("", "")
	m.input.SetValue("refs/heads/topic")
	if cmd := m.selectPickerChoice(); cmd != nil || m.busy {
		t.Fatal("picker checkout ignored unsaved document")
	}
	m.document().Undo()
	m.openBranchPicker("", "")
	m.input.SetValue("refs/heads/topic")
	drain(t, m, m.selectPickerChoice())
	if m.snapshot.Branch != "topic" {
		t.Fatal("refreshed picker checkout failed", m.notice)
	}
}

func statusPart(t *testing.T, m *Model, id string) statusSegment {
	t.Helper()
	for _, s := range m.statusLayout() {
		if s.id == id {
			return s
		}
	}
	t.Fatalf("missing status segment %q", id)
	return statusSegment{}
}

func TestStatusFollowsActiveFileDiffAndLastClosedFile(t *testing.T) {
	m, child := repositoryBranchWorkspace(t)
	if _, err := child.Run(m.ctx, "switch", "child-topic"); err != nil {
		t.Fatal(err)
	}
	if _, err := child.Run(m.ctx, "remote", "set-url", "origin", "git@github.com:child/project.git"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(child.Root, "README.md"), []byte("child edit\n"), 0644); err != nil {
		t.Fatal(err)
	}
	drain(t, m, m.refresh())
	m.openFile("child space/README.md")
	if got := statusPart(t, m, "branch").text; !strings.Contains(got, "child space · child-topic*") {
		t.Fatalf("wrong child branch: %q", got)
	}
	if got := statusPart(t, m, "changes").text; !strings.HasSuffix(got, " 1") {
		t.Fatalf("child count includes parent gitlink: %q", got)
	}
	called := ""
	m.openURL = func(_ context.Context, target string) error { called = target; return nil }
	drain(t, m, m.clickStatus(statusPart(t, m, "github").x+1))
	if called != "https://github.com/child/project" {
		t.Fatalf("wrong child remote: %q", called)
	}
	m.clickStatus(statusPart(t, m, "branch").x + 1)
	if m.branchRepository != "child space" {
		t.Fatal("child branch control targets parent")
	}
	press(m, "esc")
	m.openFile("README.md")
	if m.statusRepository() != "" || !strings.Contains(statusPart(t, m, "branch").text, filepath.Base(m.repo.Root)+" · main") {
		t.Fatal("parent file retained child status")
	}
	m.activateTab(0)
	m.clickStatus(statusPart(t, m, "changes").x + 1)
	if m.current() == nil || !m.current().repoHeader || m.current().repository != "child space" || m.statusRepository() != "child space" {
		t.Fatal("changes control lost active repository")
	}
	// A parent gitlink diff belongs to its parent even though its path names a child.
	selectChanged(t, m, "child space", false)
	drain(t, m, m.preview())
	m.keepTab()
	if m.statusRepository() != "" {
		t.Fatal("gitlink diff targets child instead of parent")
	}
	selectChanged(t, m, "child space/README.md", false)
	drain(t, m, m.preview())
	if m.statusRepository() != "child space" {
		t.Fatal("child diff targets parent")
	}
	m.closeTab() // Child diff.
	m.closeTab() // Parent gitlink diff, now the adjacent tab.
	m.activateTab(1)
	m.closeTab() // Parent document.
	m.closeTab() // Child document.
	if len(m.tabs) != 0 || m.statusRepository() != "child space" {
		t.Fatal("closing last file lost most recent repository")
	}
	m.snapshot.Repositories = nil
	if m.statusRepository() != "" {
		t.Fatal("removed child repository remained active")
	}
}

func TestFileRepositoryUsesDeepestDirectoryBoundary(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.snapshot.Repositories = []string{"child", "child/nested"}
	for path, want := range map[string]string{
		"README.md": "", "child": "", "childish/file.go": "",
		"child/file.go": "child", "child/nested": "child",
		"child/nested/file.go": "child/nested",
	} {
		if got := m.fileRepository(filepath.Join(m.repo.Root, path)); got != want {
			t.Errorf("owner of %q = %q, want %q", path, got, want)
		}
	}
}

func TestStatusSyncAndPublishTargetChildRepository(t *testing.T) {
	m, child := repositoryBranchWorkspace(t)
	remote := filepath.Join(t.TempDir(), "remote.git")
	for _, args := range [][]string{
		{"init", "--bare", remote},
		{"remote", "set-url", "origin", remote},
		{"push", "-u", "origin", "main"},
		{"switch", "child-topic"},
	} {
		if _, err := child.Run(m.ctx, args...); err != nil {
			t.Fatal(err)
		}
	}
	drain(t, m, m.refresh())
	m.openFile("child space/README.md")
	m.clickStatus(statusPart(t, m, "sync").x + 1)
	if m.modal != "remotes" || len(m.palette) != 1 || m.palette[0].id != "origin" {
		t.Fatal("child publish did not use child remotes", m.notice)
	}
	publish := m.palette[0].run
	press(m, "esc")
	// Changing files after opening the picker cannot redirect its operation.
	m.openFile("README.md")
	drain(t, m, publish())
	state := m.snapshot.RepositoryStates["child space"]
	if state.Upstream != "origin/child-topic" || !state.TrackingKnown || state.Ahead != 0 || state.Behind != 0 {
		t.Fatalf("child publish did not refresh tracking: %+v", state)
	}
	if _, err := child.Run(m.ctx, "commit", "--allow-empty", "-m", "ahead"); err != nil {
		t.Fatal(err)
	}
	drain(t, m, m.refresh())
	m.openFile("child space/README.md")
	if got := statusPart(t, m, "sync").text; !strings.Contains(got, "1"+m.symbol(symbolUp)) {
		t.Fatalf("missing child ahead count: %q", got)
	}
	parentHead := m.snapshot.Head
	cmd := m.clickStatus(statusPart(t, m, "sync").x + 1)
	m.openFile("README.md")
	drain(t, m, cmd)
	state = m.snapshot.RepositoryStates["child space"]
	if !state.TrackingKnown || state.Ahead != 0 || state.Behind != 0 || m.snapshot.Head != parentHead {
		t.Fatal("sync failed or modified parent", m.notice)
	}
	if _, err := child.Run(m.ctx, "switch", "--detach"); err != nil {
		t.Fatal(err)
	}
	drain(t, m, m.refresh())
	m.openFile("child space/README.md")
	if !strings.Contains(statusPart(t, m, "branch").text, "detached") {
		t.Fatal("child detached HEAD is hidden")
	}
	if cmd := m.syncRepository(); cmd != nil || !strings.Contains(m.notice, "detached") {
		t.Fatal("detached child sync was not blocked")
	}
}
