package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	gitservice "github.com/ourongxing/elegit/internal/git"
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
	if m.historyCommit == "" || historyKey("", m.diffCommit) != m.historyCommit || m.diffPath == "" || m.diffChange != nil {
		t.Fatal("commit did not open a read-only file preview")
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
	m.selected = fileIndex
	drain(t, m, m.preview())
	if m.diffPath != "cmd/main.go" {
		t.Fatal("wrong per-file diff")
	}
	if cmd := m.stageHunk(false); cmd != nil {
		t.Fatal("history permitted staging")
	}
	if cmd := m.editDiff(); cmd != nil || m.showEditor {
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
	m.historyCommit = historyKey("", "new")
	m.historySeq = 2
	m.historyStates[historyKey("", "new")] = &historyState{seq: 2, loading: true}
	if cmd := m.applyHistoryFiles(historyFilesMsg{seq: 1, id: "old", files: []gitservice.Change{{Path: "old"}}}); cmd != nil || len(m.historyStates[historyKey("", "new")].files) > 0 {
		t.Fatal("stale commit files applied")
	}
	m.selectSection(explorer)
	if cmd := m.applyHistoryFiles(historyFilesMsg{seq: 2, id: "new"}); cmd != nil {
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
	m.previewHistory(selectCommit(a.ID), true)
	seqA := m.historyStates[historyKey("", a.ID)].seq
	m.previewHistory(selectCommit(b.ID), true)
	seqB := m.historyStates[historyKey("", b.ID)].seq
	// The earlier request may finish last; both results belong to their own trees.
	m.applyHistoryFiles(historyFilesMsg{seq: seqB, id: b.ID, files: []gitservice.Change{{Path: "src/b.go", Index: 'M', Worktree: ' '}}})
	if cmd := m.applyHistoryFiles(historyFilesMsg{seq: seqA, id: a.ID, files: []gitservice.Change{{Path: "src/a.go", Index: 'M', Worktree: ' '}}}); cmd != nil {
		t.Fatal("background commit result replaced active diff")
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
			m.openHistory(entry)
			break
		}
	}
	if visible(a.ID, "src/a.go") || !visible(b.ID, "src/b.go") {
		t.Fatal("directory folding leaked across commits")
	}
	m.openHistory(selectCommit(a.ID))
	if m.historyExpanded(historyKey("", a.ID)) || !m.historyExpanded(historyKey("", b.ID)) {
		t.Fatal("commit collapse affected its neighbor")
	}
	m.openHistory(selectCommit(a.ID))
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
