package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	gitservice "github.com/foursmith/vl/internal/git"
)

func TestHistoryFileReviewAndReadOnlyActions(t *testing.T) {
	m := explorerWorkspace(t)
	for _, args := range [][]string{{"config", "user.name", "Test"}, {"config", "user.email", "test@example.test"}, {"add", "."}, {"-c", "commit.gpgsign=false", "commit", "-m", "initial"}} {
		if _, err := m.repo.Run(m.ctx, args...); err != nil {
			t.Fatal(err)
		}
	}
	drain(t, m, m.refresh())
	drain(t, m, m.selectSection(history))
	drain(t, m, press(m, "enter"))
	if m.diffOpen || !m.sidebar {
		t.Fatal("expanding a commit opened a diff or moved focus")
	}
	var fileIndex int = -1
	for i, entry := range m.items {
		if entry.historyChange != nil && entry.historyChange.Path == "cmd/main.go" {
			fileIndex = i
		}
	}
	if fileIndex < 0 {
		t.Fatal("missing nested commit file")
	}
	m.selected = fileIndex - 1
	drain(t, m, press(m, "j"))
	if m.diffPath != "cmd/main.go" || !m.sidebar || !m.tabs[m.activeTab].preview {
		t.Fatal("wrong per-file diff")
	}
	if cmd := m.stageDiffLines(false, "Stage Hunk", nil); cmd != nil {
		t.Fatal("history permitted staging")
	}
	if cmd := m.editDiff(); cmd != nil || m.editing() {
		t.Fatal("history opened worktree for editing")
	}
	m.selectParentFolder()
	if m.current().folder != "cmd" {
		t.Fatal("file parent navigation")
	}
	drain(t, m, m.openSelected())
	for _, entry := range m.items {
		if entry.historyChange != nil && entry.historyChange.Path == "cmd/main.go" {
			t.Fatal("collapsed file remains visible")
		}
	}
	m.selectParentFolder()
	if m.current().commit == nil {
		t.Fatal("folder parent navigation")
	}
	drain(t, m, m.openSelected())
	if len(m.items) != 2 || !m.items[0].repoHeader || m.items[1].commit == nil {
		t.Fatal("commit collapse should retain its repository header")
	}
	for _, width := range []int{40, 80, 140} {
		m.width = width
		m.resize()
		for _, line := range strings.Split(m.View().Content, "\n") {
			if ansi.StringWidth(line) != width {
				t.Fatalf("width %d: %d", width, ansi.StringWidth(line))
			}
		}
	}
}

func TestHistoryIgnoresLateFileLists(t *testing.T) {
	m := tabWorkspace(t, 0)
	m.section = history
	m.historySeq = 2
	m.historyStates[historyKey("", "new")] = &historyState{seq: 2, loading: true}
	m.applyHistoryFiles(historyFilesMsg{seq: 1, id: "old", files: []gitservice.Change{{Path: "old"}}})
	if len(m.historyStates[historyKey("", "new")].files) > 0 {
		t.Fatal("stale commit files applied")
	}
	m.selectSection(explorer)
	m.applyHistoryFiles(historyFilesMsg{seq: 2, id: "new"})
	if m.diffOpen || m.historyStates[historyKey("", "new")].loaded {
		t.Fatal("late history load opened diff outside history")
	}
}

func TestHistoryKeepsMultipleCommitsExpandedWithIndependentLoads(t *testing.T) {
	m := tabWorkspace(t, 0)
	m.section = history
	m.snapshot.Commits = []gitservice.Commit{
		{ID: strings.Repeat("a", 40), Subject: "first"},
		{ID: strings.Repeat("b", 40), Subject: "second"},
	}
	a, b := &m.snapshot.Commits[0], &m.snapshot.Commits[1]
	selectCommit := func(id string) item {
		t.Helper()
		for i, entry := range m.items {
			if entry.commit != nil && entry.commit.ID == id {
				m.selected = i
				return entry
			}
		}
		t.Fatal("missing commit")
		return item{}
	}
	m.rebuildItems()
	m.loadHistoryCommit(selectCommit(a.ID))
	seqA := m.historyStates[historyKey("", a.ID)].seq
	m.loadHistoryCommit(selectCommit(b.ID))
	seqB := m.historyStates[historyKey("", b.ID)].seq
	// The earlier request may finish last; both results belong to their own trees.
	m.applyHistoryFiles(historyFilesMsg{seq: seqB, id: b.ID, files: []gitservice.Change{{Path: "src/b.go", Index: 'M', Worktree: ' '}}})
	m.applyHistoryFiles(historyFilesMsg{seq: seqA, id: a.ID, files: []gitservice.Change{{Path: "src/a.go", Index: 'M', Worktree: ' '}}})
	if m.diffOpen {
		t.Fatal("loading commit trees opened a diff")
	}
	visible := func(id, path string) bool {
		for _, entry := range m.items {
			if entry.historyID == historyKey("", id) && entry.historyChange != nil && entry.historyChange.Path == path {
				return true
			}
		}
		return false
	}
	if !visible(a.ID, "src/a.go") || !visible(b.ID, "src/b.go") {
		t.Fatal("opening one commit closed the other")
	}
	for i, entry := range m.items {
		if entry.historyID == historyKey("", a.ID) && entry.folder == "src" {
			m.selected = i
			m.activateSidebarEntry(entry, false)
			break
		}
	}
	if visible(a.ID, "src/a.go") || !visible(b.ID, "src/b.go") {
		t.Fatal("directory folding leaked across commits")
	}
	m.activateSidebarEntry(selectCommit(a.ID), false)
	if m.historyExpanded(historyKey("", a.ID)) || !m.historyExpanded(historyKey("", b.ID)) {
		t.Fatal("commit collapse affected its neighbor")
	}
	m.activateSidebarEntry(selectCommit(a.ID), false)
	if !m.historyExpanded(historyKey("", a.ID)) || m.historyStates[historyKey("", a.ID)].seq != seqA || visible(a.ID, "src/a.go") || !visible(b.ID, "src/b.go") {
		t.Fatal("reopening lost cached files or independent directory state")
	}
	drain(t, m, m.selectSection(explorer))
	m.section = history
	m.rebuildItems()
	if !m.historyExpanded(historyKey("", a.ID)) || !m.historyExpanded(historyKey("", b.ID)) {
		t.Fatal("navigation discarded expanded commits")
	}
}

func TestHistoryTagBadgesPrecedeLongSubjects(t *testing.T) {
	m := explorerWorkspace(t)
	id := strings.Repeat("a", 40)
	m.snapshot.Refs = []gitservice.Ref{
		{Name: "main", Kind: gitservice.LocalBranch, OID: id, TargetOID: id},
		{Name: "v0.2.4", Kind: gitservice.Tag, OID: strings.Repeat("b", 40), TargetOID: id},
		{Name: "other-tag", Kind: gitservice.Tag, TargetOID: strings.Repeat("c", 40)},
	}
	entry := item{commit: &gitservice.Commit{ID: id, Subject: "chore: release with a long subject"}}
	for _, width := range []int{28, 44, 80} {
		text, _ := m.historyRow(entry, width, m.colors.Panel)
		label := plain(text)
		if !strings.Contains(label, "v0.2.4") || strings.Index(label, "v0.2.4") > strings.Index(label, "chore") || strings.Contains(label, "other-tag") {
			t.Fatalf("tag missing or misplaced at width %d: %q", width, label)
		}
		if ansi.StringWidth(text) > width {
			t.Fatalf("row exceeds width %d", width)
		}
	}
}

func TestHistoryKeyboardOpensCommitAsFolder(t *testing.T) {
	for _, key := range []string{"enter", "l"} {
		t.Run(key, func(t *testing.T) {
			m := tabWorkspace(t, 0)
			t.Cleanup(m.Close)
			m.section = history
			commit := gitservice.Commit{ID: strings.Repeat("a", 40), Subject: "initial"}
			other := gitservice.Commit{ID: strings.Repeat("b", 40), Subject: "previous"}
			m.snapshot.Commits = []gitservice.Commit{commit, other}
			m.rebuildItems()
			for i, entry := range m.items {
				if entry.commit != nil {
					m.selected = i
					break
				}
			}
			assertSelected := func(want string) {
				t.Helper()
				entry := m.current()
				if entry == nil || entry.commit == nil || entry.commit.ID != want || len(m.historyStates) != 0 {
					t.Fatal("moving the selection did not focus the commit without opening it")
				}
			}
			press(m, "j")
			assertSelected(other.ID)
			press(m, "k")
			assertSelected(commit.ID)
			m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
			assertSelected(other.ID)
			m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
			assertSelected(commit.ID)
			if press(m, key) == nil {
				t.Fatal("opening an unloaded commit did not request changed files")
			}
			state := m.historyStates[historyKey("", commit.ID)]
			m.Update(historyFilesMsg{seq: state.seq, id: commit.ID, files: []gitservice.Change{{Path: "file.go", Index: 'M', Worktree: ' '}}})
			if len(m.tabs) != 0 || !m.sidebar || m.current().commit == nil || m.current().commit.ID != commit.ID {
				t.Fatal("expanding the commit opened a diff or moved focus")
			}
			if press(m, "j") == nil || len(m.tabs) != 1 || !m.tabs[0].preview || !m.sidebar || m.diffPath != "file.go" {
				t.Fatal("moving onto the file did not preview it with sidebar focus")
			}
		})
	}
}
