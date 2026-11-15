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
	gitservice "github.com/ourongxing/elegit/internal/git"
)

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
	if !m.snapshot.Detached || !strings.Contains(m.statusLine(), "分离") {
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
		if !branch || !github || ansi.StringWidth(m.statusLine()) != width {
			t.Fatalf("missing controls or incorrect width %d", width)
		}
	}
	text := plain(m.statusLine())
	for _, value := range []string{"2" + m.symbol(symbolArrowDown) + " 3" + m.symbol(symbolUp), "UTF-8", "LF", "Go", "Tab: 4", "行 1, 列 1"} {
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
