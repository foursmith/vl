package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/config"
	"github.com/foursmith/vl/internal/diff"
	gitservice "github.com/foursmith/vl/internal/git"
)

func TestStatusBarKeepsLayoutWhileTextDiffLoads(t *testing.T) {
	change := gitservice.Change{Path: "file0.go", Index: ' ', Worktree: 'M'}
	for _, tc := range []struct {
		name    string
		section section
		target  diffTarget
	}{
		{"Changes", changes, diffTarget{path: change.Path, change: &change}},
		{"History", history, diffTarget{path: change.Path, commit: "test"}},
		{"PullRequests", pullRequests, diffTarget{path: change.Path, pr: gitservice.PullRequest{Number: 1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := tabWorkspace(t, 1)
			t.Cleanup(m.Close)
			m.section, m.sidebar, m.selected = tc.section, true, 0
			m.items = []item{{change: tc.target.change}}
			m.SetTerminalSize(120, 24)
			cmd := m.requestDiff(tc.target, func(context.Context) (string, error) {
				return diffTabText("new"), nil
			})
			status := func() string {
				lines := strings.Split(m.View().Content, "\n")
				return lines[len(lines)-1]
			}
			loading := status()
			if m.editorDocument() != nil || !strings.Contains(plain(loading), "NORMAL") {
				t.Fatal("loading text diff did not render its initial status")
			}
			drain(t, m, cmd)
			if got := status(); got != loading {
				t.Fatalf("status changed on diff load:\nloading: %q\n loaded: %q", plain(loading), plain(got))
			}
		})
	}
}

func TestRevisionHoverAndFocusUseStatusNoticeArea(t *testing.T) {
	const title = "长标题 · Display a full revision title that cannot fit in the sidebar row"
	for _, tt := range []struct {
		section section
		entry   item
		want    string
	}{
		{history, item{commit: &gitservice.Commit{ID: "test", Subject: title}}, title},
		{pullRequests, item{pullRequest: &gitservice.PullRequest{Number: 42}, label: "#42 " + title}, "#42 " + title},
	} {
		m := tabWorkspace(t, 1)
		t.Cleanup(m.Close)
		m.openFile("file0.go")
		m.section = tt.section
		m.items = []item{tt.entry, {label: "file0.go", pullRequest: tt.entry.pullRequest, pullRequestFile: &diff.FilePatch{Path: "file0.go"}}}
		m.SetSidebarLayout(28, true)
		m.Update(tea.WindowSizeMsg{Width: 150, Height: 24})
		m.View()
		branch, encoding := statusPart(t, m, "branch"), statusPart(t, m, "encoding")

		m.Update(tea.MouseMotionMsg{X: m.sideX() + 3, Y: headerHeight})
		lines := strings.Split(m.View().Content, "\n")
		if got := plain(lines[len(lines)-1]); !strings.Contains(got, tt.want) {
			t.Fatalf("status bar = %q, want title %q", got, tt.want)
		}
		if statusPart(t, m, "notice").text != tt.want || statusPart(t, m, "branch") != branch || statusPart(t, m, "encoding") != encoding {
			t.Fatal("hover title displaced an existing status segment")
		}
		for _, pos := range [][2]int{{m.sideX() + 3, headerHeight + 1}, {m.mainX() + 3, headerHeight}} {
			m.Update(tea.MouseMotionMsg{X: pos[0], Y: pos[1]})
			lines = strings.Split(m.View().Content, "\n")
			if got := plain(lines[len(lines)-1]); strings.Contains(got, title) || !strings.Contains(got, "UTF-8") {
				t.Fatalf("ordinary status bar was not restored: %q", got)
			}
		}
		m.selected = 0
		m.Update(tea.KeyPressMsg{Code: tea.KeyF2})
		if !m.sidebar || statusPart(t, m, "notice").text != tt.want {
			t.Fatal("focusing the revision did not show its title")
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyF2})
		if m.sidebar || strings.Contains(plain(m.statusLine()), title) {
			t.Fatal("revision title remained after focus moved to the editor")
		}
	}
}

func TestStatusHintsFollowPreviewFocusAndFitNarrowWidth(t *testing.T) {
	m := tabWorkspace(t, 1)
	t.Cleanup(m.Close)
	m.items = []item{{file: "file0.go", label: "file0.go"}}
	m.selected, m.sidebar = 0, true
	m.openDocument("file0.go", true)
	m.items[0].file = m.document().Path
	m.Update(tea.WindowSizeMsg{Width: 180, Height: 24})
	if got := statusPart(t, m, "notice").text; got != "Enter Keep Preview · F2 Focus Editor" {
		t.Fatalf("preview hints = %q", got)
	}
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 18})
	if got := statusPart(t, m, "notice").text; got != "Enter Keep Preview" {
		t.Fatalf("narrow preview hint = %q", got)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyF2})
	if m.tabs[m.activeTab].preview || strings.Contains(plain(m.statusLine()), "Keep Preview") {
		t.Fatal("preview hint remained after the editor took focus")
	}
}

func TestStatusHintsFollowGitScopeAndSelection(t *testing.T) {
	m := tabWorkspace(t, 0)
	t.Cleanup(m.Close)
	m.Update(tea.WindowSizeMsg{Width: 180, Height: 24})
	change := gitservice.Change{Path: "file.go", Worktree: 'M', Index: 'M'}
	m.section, m.sidebar, m.selected = changes, true, 0
	m.snapshot.Changes = []gitservice.Change{change}
	for _, tt := range []struct {
		entry item
		want  string
	}{
		{item{change: &change}, "s Stage File · S Stage Repository · d Discard File · D Discard Repository Changes"},
		{item{folder: "dir"}, "s Stage Folder · S Stage Repository · d Discard Folder · D Discard Repository Changes"},
		{item{change: &change, staged: true}, "s Unstage File · S Unstage Repository"},
		{item{repoHeader: true}, "s/S Stage Repository · D Discard Repository Changes"},
	} {
		m.items = []item{tt.entry}
		if got := statusPart(t, m, "notice").text; got != tt.want {
			t.Errorf("changes hint = %q, want %q", got, tt.want)
		}
	}

	m.diffPath, m.diffChange = change.Path, &change
	m.patch = diff.Parse("@@ -1 +1 @@\n-old\n+new\n")
	showDiffFixture(m)
	if got := statusPart(t, m, "notice").text; got != "s Stage Hunk · S Stage File · d Discard Hunk · D Discard File" {
		t.Fatalf("diff hint = %q", got)
	}
	m.diffSelection = diffSelection{active: true, right: m.diffLayout.split, anchor: textPoint{0, 0}, cursor: textPoint{0, 3}}
	if got := statusPart(t, m, "notice").text; !strings.HasPrefix(got, "s Stage Selected Ranges · S Stage File") {
		t.Fatalf("selection hint = %q", got)
	}
	m.diffStaged = true
	if got := statusPart(t, m, "notice").text; strings.Contains(got, "Discard") || !strings.Contains(got, "S Unstage File") {
		t.Fatalf("index diff hint = %q", got)
	}
	m.notice = "Git result"
	if got := statusPart(t, m, "notice").text; got != m.notice {
		t.Fatalf("operation notice = %q", got)
	}
}

func TestGitCompletionNoticeExpiryRestoresHints(t *testing.T) {
	m := tabWorkspace(t, 0)
	t.Cleanup(m.Close)
	m.Update(tea.WindowSizeMsg{Width: 180, Height: 24})
	change := gitservice.Change{Path: "file.go", Worktree: 'M'}
	m.section, m.sidebar, m.selected = changes, true, 0
	m.items = []item{{change: &change}}

	m.Update(operationMsg{title: "Unstage Changes"})
	first := m.statusFeedback
	if got := statusPart(t, m, "notice").text; got != "Unstage Changes completed" {
		t.Fatalf("first notice = %q", got)
	}
	m.Update(operationMsg{title: "Unstage Changes"})
	second := m.statusFeedback
	m.Update(statusFeedbackExpiredMsg{feedback: first})
	if got := m.statusNotice(); got != "Unstage Changes completed" {
		t.Fatalf("old timer cleared the newer notice: %q", got)
	}
	m.Update(statusFeedbackExpiredMsg{feedback: second})
	if got := statusPart(t, m, "notice").text; !strings.HasPrefix(got, "s Stage File") {
		t.Fatalf("expiry did not restore the Changes shortcut hints: %q", got)
	}

	m.Update(operationMsg{title: "Stage"})
	third := m.statusFeedback
	m.Update(operationMsg{title: "Stage", err: errors.New("Git failed")})
	m.Update(statusFeedbackExpiredMsg{feedback: third})
	if got := m.statusNotice(); got != "Git failed" {
		t.Fatalf("expiry altered the failure notice: %q", got)
	}
}

func TestStatusNoticeTakesPriorityOverHistoryTitleAndHover(t *testing.T) {
	m := tabWorkspace(t, 0)
	t.Cleanup(m.Close)
	m.Update(tea.WindowSizeMsg{Width: 180, Height: 24})
	m.section, m.sidebar, m.selected = history, true, 0
	m.items = []item{{commit: &gitservice.Commit{ID: "test", Subject: "Revision title"}}}
	m.Update(tea.MouseMotionMsg{X: m.sideX() + 3, Y: headerHeight})
	m.Update(operationMsg{title: "Fetch", err: errors.New("Git failed")})
	if got := statusPart(t, m, "notice").text; got != "Git failed" {
		t.Fatalf("history or hover hid error: %q", got)
	}

	m.showStatusFeedback("Saved file")
	feedback := m.statusFeedback
	if got := statusPart(t, m, "notice").text; got != "Saved file" {
		t.Fatalf("history or hover hid feedback: %q", got)
	}
	m.Update(statusFeedbackExpiredMsg{feedback: feedback})
	if got := statusPart(t, m, "notice").text; got != "Revision title" {
		t.Fatalf("expiry did not restore history title: %q", got)
	}
}

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
		if err != nil || saved.Appearance.StatusBarStyle != statusBarPure || m.statusBarStyle != statusBarPure || m.modal != "" {
			t.Fatalf("Pure was not applied and saved: %+v, %v", saved, err)
		}
		m.SetLayout(config.Layout{Appearance: config.Appearance{StatusBarStyle: "unknown"}})
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
		if err != nil || saved.Appearance.StatusBarStyle != "" || m.statusBarStyle != statusBarStandard {
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
	m.pullRequestsFetchedAt = time.Now() // Keep the fake remote off the network.
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

func TestRemoteProgressKeepsCountsAndActionPositions(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.snapshot = gitservice.Snapshot{Branch: "main", Upstream: "origin/main", TrackingKnown: true, Behind: 12, Ahead: 3}
	m.selectSection(history)
	width := m.sidebarContentWidth()
	header := m.items[0]
	before := statusPart(t, m, "sync")
	original := m.layoutRepositoryRow(header, width)
	m.showRemoteProgress(m.startOperation("Pull", func() (string, error) { return "", nil }), "", "pull")
	progress := m.remoteProgress
	for _, frame := range remoteProgressFrames {
		m.Update(remoteProgressFrameMsg{progress: progress})
		got := statusPart(t, m, "sync")
		if got.x != before.x || got.width != before.width || got.pullOffset != before.pullOffset ||
			got.pushOffset != before.pushOffset || !strings.Contains(got.text, "12"+frame+" 3"+m.symbol(symbolUp)) {
			t.Fatalf("status controls moved or lost their counts: %+v", got)
		}
		during := m.layoutRepositoryRow(header, width)
		if during.branchStart != original.branchStart || len(during.actions) != 3 {
			t.Fatalf("History controls shifted: %+v", during)
		}
		for i, action := range during.actions {
			if action.x != original.actions[i].x || action.width != original.actions[i].width {
				t.Fatalf("History action moved: %+v", action)
			}
		}
		if during.actions[1].glyph != "12"+frame || !during.actions[1].running {
			t.Fatalf("History did not animate the Pull icon: %+v", during.actions)
		}
	}
	m.Update(operationMsg{title: "Pull"})
	m.Update(remoteProgressFrameMsg{progress: progress})
	if m.remoteProgress != nil || statusPart(t, m, "sync") != before {
		t.Fatal("completed Pull left its progress indicator visible")
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

func TestStatusRemoteActionsTargetChildRepository(t *testing.T) {
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
	publish := m.clickStatus(statusPart(t, m, "sync").x + 1)
	if publish == nil || m.modal != "" {
		t.Fatal("child publish did not use its sole remote directly", m.notice)
	}
	// Changing files after starting the operation cannot redirect it.
	m.openFile("README.md")
	drain(t, m, publish)
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
	runRemoteAction := func(cmd tea.Cmd, title string) {
		t.Helper()
		if cmd == nil {
			t.Fatalf("%s button did not start an operation", title)
		}
		drain(t, m, cmd)
		if got := m.statusNotice(); got != title+" completed" {
			t.Fatalf("%s button result: %s", title, got)
		}
	}
	segment := statusPart(t, m, "sync")
	before, _, found := strings.Cut(segment.text, m.symbol(symbolUp))
	if !found {
		t.Fatalf("missing Push button in %q", segment.text)
	}
	runRemoteAction(m.click(tea.MouseClickMsg{X: segment.x + 1 + ansi.StringWidth(before), Y: m.height - 1, Button: tea.MouseLeft}), "Push")
	state = m.snapshot.RepositoryStates["child space"]
	if !state.TrackingKnown || state.Ahead != 0 || state.Behind != 0 || m.snapshot.Head != parentHead {
		t.Fatal("push failed or modified parent", m.notice)
	}
	m.openFile("README.md")
	m.SetSidebarLayout(38, false)
	for _, tt := range []struct {
		section section
		key     string
	}{
		{changes, repositoryKey("child space")},
		{history, historyRepositoryKey("child space")},
	} {
		if _, err := child.Run(m.ctx, "reset", "--hard", "HEAD^"); err != nil {
			t.Fatal(err)
		}
		drain(t, m, m.refresh())
		if state = m.snapshot.RepositoryStates["child space"]; state.Behind != 1 {
			t.Fatalf("expected child to be one behind: %+v", state)
		}
		drain(t, m, m.selectSection(tt.section))
		m.selected = m.sidebarRowWithKey(tt.key)
		if m.selected < 0 {
			t.Fatalf("missing repository row %q", tt.key)
		}
		m.ensureSidebar()
		m.Update(tea.MouseClickMsg{X: m.sideX() + 3, Y: m.sidebarSelectionY(), Button: tea.MouseRight})
		runRemoteAction(chooseContextAction(t, m, "Pull"), "Pull")
		if state = m.snapshot.RepositoryStates["child space"]; state.Behind != 0 || m.snapshot.Head != parentHead {
			t.Fatalf("pull from %q failed or modified parent: %s", tt.key, m.notice)
		}
	}
	clickHistoryAction := func(id, title string) {
		t.Helper()
		for y := headerHeight; y < m.height-1; y++ {
			index := m.sidebarRowAt(y)
			if index < 0 || !m.items[index].repoHeader || m.items[index].repository != "child space" {
				continue
			}
			layout := m.layoutRepositoryRow(m.items[index], m.sidebarContentWidth())
			for _, action := range layout.actions {
				if action.id == id {
					line := plain(m.sidebarLines()[y])
					if !strings.Contains(line, action.glyph) || !strings.Contains(line, "child-topic") {
						t.Fatalf("History title or %s button missing from %q", title, line)
					}
					runRemoteAction(m.click(tea.MouseClickMsg{X: m.sideX() + action.x + 1, Y: y, Button: tea.MouseLeft}), title)
					return
				}
			}
		}
		t.Fatalf("missing History %s action", title)
	}
	if _, err := child.Run(m.ctx, "commit", "--allow-empty", "-m", "history push"); err != nil {
		t.Fatal(err)
	}
	drain(t, m, m.refresh())
	clickHistoryAction("push", "Push")
	if state = m.snapshot.RepositoryStates["child space"]; state.Ahead != 0 || m.snapshot.Head != parentHead {
		t.Fatal("History push failed or modified parent", m.notice)
	}
	if _, err := child.Run(m.ctx, "commit", "--allow-empty", "-m", "sync"); err != nil {
		t.Fatal(err)
	}
	drain(t, m, m.refresh())
	m.openFile("child space/README.md")
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
