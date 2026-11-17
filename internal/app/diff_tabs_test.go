package app

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	gitservice "github.com/ourongxing/vl/internal/git"
)

func diffTabText(label string) string {
	return "--- a/file0.go\n+++ b/file0.go\n@@ -1,60 +1,60 @@\n-old\n+" + label + "\n" + strings.Repeat(" same\n", 59)
}

func TestDiffTabsOwnIdentityContentAndViewport(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openFile("file0.go")
	doc := m.document()
	targets := []diffTarget{
		{path: "file0.go"},
		{path: "file0.go", staged: true},
		{path: "file0.go", commit: "111111111111"},
		{path: "file0.go", commit: "222222222222"},
		{path: "file0.go", repository: "child"},
	}
	var states []*diffState
	for i, target := range targets {
		raw := diffTabText(fmt.Sprintf("source %d", i))
		drain(t, m, m.requestDiff(target, func(context.Context) (string, error) { return raw, nil }))
		m.viewport.SetYOffset(i + 1)
		states = append(states, m.diffState)
	}
	if len(m.tabs) != len(targets)+1 || m.tabs[0].doc != doc {
		t.Fatal("diff replaced a document or another review source")
	}
	for i, state := range states {
		m.activateTab(i + 1)
		if m.diffState != state || m.document() != nil || m.viewport.YOffset() != i+1 || !strings.Contains(m.patch.Raw, fmt.Sprintf("source %d", i)) {
			t.Fatalf("tab %d lost its content or viewport", i)
		}
		for _, width := range []int{30, 80, 140} {
			m.Update(tea.WindowSizeMsg{Width: width, Height: 24})
			if m.diffState != state {
				t.Fatal("resize changed the selected review")
			}
		}
	}
	m.activateTab(0)
	if m.document() != doc || m.diffOpen || m.patch.Raw != "" {
		t.Fatal("document inherited a diff surface")
	}
	raw := diffTabText("updated index")
	drain(t, m, m.requestDiff(targets[1], func(context.Context) (string, error) { return raw, nil }))
	if len(m.tabs) != 6 || m.activeTab != 2 || m.diffState != states[1] || !strings.Contains(m.patch.Raw, "updated index") {
		t.Fatal("reopening the same review did not reuse its tab")
	}
}

func TestDiffRepliesStayWithTheirTabAcrossSwitchAndClose(t *testing.T) {
	m := tabWorkspace(t, 1)
	first := m.requestDiff(diffTarget{path: "file0.go"}, func(context.Context) (string, error) { return diffTabText("first"), nil })
	d1 := m.diffState
	second := m.requestDiff(diffTarget{path: "file0.go", staged: true}, func(context.Context) (string, error) { return diffTabText("second"), nil })
	d2 := m.diffState
	m.openFile("file0.go")
	doc := m.document()
	drain(t, m, second)
	drain(t, m, first)
	if m.document() != doc || m.diffOpen || !strings.Contains(d1.patch.Raw, "first") || !strings.Contains(d2.patch.Raw, "second") {
		t.Fatal("out-of-order replies overwrote another tab or stole focus")
	}
	m.activateTab(0)
	stale := m.loadDiff(d1, false)().(diffMsg)
	syntax := m.highlightDiff()().(diffSyntaxMsg)
	m.closeTab()
	if m.diffState != d2 || d1.diffOpen || syntax.ctx.Err() == nil {
		t.Fatal("closing active diff did not cancel its work and select its neighbor")
	}
	drain(t, m, m.requestDiff(diffTarget{path: "file0.go"}, func(context.Context) (string, error) { return diffTabText("reopened"), nil }))
	d3 := m.diffState
	m.Update(stale)
	m.Update(syntax)
	if d3 == d1 || !strings.Contains(m.patch.Raw, "reopened") || m.diffState != d3 {
		t.Fatal("closed-tab replies affected the reopened tab")
	}
	// Closing an inactive diff through its actual hitbox preserves the document.
	m.activateTab(1)
	for _, slot := range m.tabLayout() {
		if slot.control == 0 && slot.index == 0 {
			m.click(tea.MouseClickMsg{X: m.mainX() + slot.x + 2, Y: tabRow, Button: tea.MouseMiddle})
			break
		}
	}
	if m.document() != doc || d2.diffOpen || len(m.tabs) != 2 {
		t.Fatal("inactive diff close changed the active document")
	}
}

func TestDiffRefreshUsesTabSourceAndPreservesActiveDocument(t *testing.T) {
	m := tabWorkspace(t, 2)
	for i := 0; i < 2; i++ {
		raw := diffTabText(fmt.Sprintf("review %d", i))
		drain(t, m, m.requestDiff(diffTarget{path: fmt.Sprintf("file%d.go", i)}, func(context.Context) (string, error) { return raw, nil }))
		m.viewport.SetYOffset(5 + i)
	}
	m.openFile("file0.go")
	doc := m.document()
	// The selected sidebar file is unrelated to either review.
	m.items = []item{{file: "file1.go", key: "file1.go"}}
	m.selected = 0
	_, cmd := m.Update(snapshotMsg{seq: m.refreshSeq, snapshot: m.snapshot})
	drain(t, m, cmd)
	if m.document() != doc || m.activeTab != 2 || len(m.tabs) != 3 {
		t.Fatal("snapshot refresh changed active tab or opened the sidebar selection")
	}
	for i := 0; i < 2; i++ {
		if d := m.tabs[i].diff; !strings.Contains(d.patch.Raw, fmt.Sprintf("review %d", i)) || d.viewport.YOffset() != 5+i {
			t.Fatalf("refresh lost tab %d identity or scroll", i)
		}
	}
	m.closeOtherTabs()
	if len(m.tabs) != 1 || m.document() != doc {
		t.Fatal("close others did not close all inactive diff tabs")
	}
}

func TestMixedTabsRetainDirtyCloseGuard(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openFile("file0.go")
	press(m, "x")
	doc := m.document()
	drain(t, m, m.requestDiff(diffTarget{path: "file0.go"}, func(context.Context) (string, error) { return diffTabText("review"), nil }))
	m.closeOtherTabs()
	if m.modal != "close" || len(m.tabs) != 2 {
		t.Fatal("close others bypassed a dirty document")
	}
	m.chooseClose("cancel")
	if len(m.tabs) != 2 || m.tabs[0].doc != doc || !m.diffOpen {
		t.Fatal("cancel changed tab selection or removed the document")
	}
	m.closeAllTabs()
	if m.modal != "close" || len(m.tabs) != 1 || m.document() != doc {
		t.Fatal("close all did not preserve the dirty document prompt")
	}
	m.chooseClose("discard")
	if len(m.tabs) != 0 || m.activeTab != -1 || !m.welcomeVisible() {
		t.Fatal("closing all mixed tabs did not return to the empty state")
	}
}

func TestFilesAndDiffsSharePreviewAndKeepMechanism(t *testing.T) {
	m := tabWorkspace(t, 3)
	openPreview := func(path string, staged bool) {
		t.Helper()
		drain(t, m, m.requestDiff(diffTarget{path: path, staged: staged, preview: true}, func(context.Context) (string, error) { return diffTabText(path), nil }))
	}
	m.openPreviewFile("file0.go")
	openPreview("file1.go", false)
	first := m.diffState
	if len(m.tabs) != 1 || !m.tabs[0].preview || m.document() != nil {
		t.Fatal("diff did not reuse file preview")
	}
	stale := m.loadDiff(first, false)().(diffMsg)
	m.openPreviewFile("file2.go")
	doc := m.document()
	m.Update(stale)
	if len(m.tabs) != 1 || first.diffOpen || m.document() != doc {
		t.Fatal("file preview did not close and replace diff safely")
	}
	press(m, "x")
	openPreview("file0.go", false)
	if len(m.tabs) != 2 || !doc.Dirty() || m.tabs[0].preview {
		t.Fatal("diff replaced an edited file")
	}
	m.keepTab()
	fixed := m.diffState
	openPreview("file1.go", true)
	if len(m.tabs) != 3 || m.tabs[1].preview || !m.tabs[2].preview {
		t.Fatal("keeping diff did not protect it from later previews")
	}
	openPreview("file0.go", false)
	if len(m.tabs) != 3 || m.diffState != fixed || m.tabs[1].preview {
		t.Fatal("revisiting kept diff changed its state")
	}
	// The tab bar uses the same promotion gesture for either kind.
	m.activateTab(2)
	slot := m.tabLayout()[2]
	for i := 0; i < 2; i++ {
		m.click(tea.MouseClickMsg{X: m.mainX() + slot.x + 4, Y: tabRow, Button: tea.MouseLeft})
	}
	if m.tabs[2].preview {
		t.Fatal("double click on diff tab did not keep it")
	}
	for _, a := range m.actions() {
		if a.id == "file.keepOpen" && a.disabled != "" {
			t.Fatal("keep preview command is disabled for diff")
		}
	}
}

func TestDiffSidebarSingleDoubleClickAndEnter(t *testing.T) {
	for _, section := range []section{changes, history} {
		t.Run(fmt.Sprint(section), func(t *testing.T) {
			m := tabWorkspace(t, 2)
			m.section = section
			for i := 0; i < 2; i++ {
				c := &gitservice.Change{Path: fmt.Sprintf("file%d.go", i), Index: ' ', Worktree: 'M'}
				entry := item{key: fmt.Sprintf("review:%d", i), change: c}
				if section == history {
					entry.change = nil
					entry.historyChange = c
					entry.historyID = "111111111111"
				}
				m.items = append(m.items, entry)
			}
			click := func(index int) {
				m.click(tea.MouseClickMsg{X: 5, Y: headerHeight + index, Button: tea.MouseLeft})
			}
			click(0)
			first := m.diffState
			click(1)
			if len(m.tabs) != 1 || !m.tabs[0].preview || first.diffOpen || !m.sidebar {
				t.Fatal("single click did not reuse diff preview and preserve sidebar focus")
			}
			click(1)
			if m.tabs[0].preview || m.sidebar {
				t.Fatal("double click did not keep diff and focus its content")
			}
			click(0)
			if len(m.tabs) != 2 || !m.tabs[1].preview {
				t.Fatal("next preview replaced kept diff")
			}
			press(m, "enter")
			if m.tabs[1].preview || m.sidebar {
				t.Fatal("Enter in sidebar did not keep diff and focus it")
			}
			// A later single click on a kept source must not turn it back into preview.
			click(1)
			if len(m.tabs) != 2 || m.tabs[0].preview {
				t.Fatal("single click downgraded a kept diff")
			}
		})
	}
}

func TestRemovedHistoryRepositoryClosesItsTabsAndKeepsOtherReview(t *testing.T) {
	m := tabWorkspace(t, 0)
	m.snapshot.Repositories = []string{"child"}
	id := "111111111111"
	childKey := historyKey("child", id)
	m.historyStates[childKey] = &historyState{repository: "child"}
	m.historyCommit, m.pendingHistory = childKey, childKey
	var removed []*diffState
	for _, path := range []string{"child/a.go", "child/b.go"} {
		drain(t, m, m.requestDiff(diffTarget{path: path, repository: "child", commit: id}, func(context.Context) (string, error) { return diffTabText("child"), nil }))
		removed = append(removed, m.diffState)
	}
	drain(t, m, m.requestDiff(diffTarget{path: "root.go", commit: id}, func(context.Context) (string, error) { return diffTabText("root"), nil }))
	root := m.diffState
	m.snapshot.Repositories = nil
	m.reconcileHistoryRepositories()
	if len(m.tabs) != 1 || m.diffState != root || !strings.Contains(m.patch.Raw, "root") || m.historyStates[childKey] != nil || m.historyCommit != "" || m.pendingHistory != "" {
		t.Fatal("repository cleanup affected another review or left orphaned history")
	}
	for _, d := range removed {
		if d.diffOpen {
			t.Fatal("orphaned history tab stayed open")
		}
	}
}

func TestEmptyDiffClosesOnlyItsOwnTab(t *testing.T) {
	for _, active := range []bool{false, true} {
		t.Run(fmt.Sprint(active), func(t *testing.T) {
			m := tabWorkspace(t, 1)
			defer m.Close()
			m.openFile("file0.go")
			doc := m.document()
			drain(t, m, m.requestDiff(diffTarget{path: "file0.go"}, func(context.Context) (string, error) { return diffTabText("review"), nil }))
			d := m.diffState
			if !active {
				m.activateTab(0)
			}
			d.read = func(context.Context) (string, error) { return "", nil }
			drain(t, m, m.refreshDiffs())
			if len(m.tabs) != 1 || m.document() != doc || m.diffOpen || d.diffOpen {
				t.Fatal("empty diff did not close safely or left the wrong tab bound")
			}
			// A late reply cannot reopen the removed review.
			m.finishDiff(diffMsg{target: d, seq: d.diffSeq, raw: diffTabText("late")})
			if len(m.tabs) != 1 || m.document() != doc {
				t.Fatal("closed diff reply changed the active document")
			}
		})
	}
}

func TestEmptyInitialDiffClosesButErrorsAndMetadataStayOpen(t *testing.T) {
	m := tabWorkspace(t, 0)
	defer m.Close()
	drain(t, m, m.requestDiff(diffTarget{path: "empty"}, func(context.Context) (string, error) { return "", nil }))
	if len(m.tabs) != 0 || m.activeTab != -1 || m.diffOpen || !m.welcomeVisible() {
		t.Fatal("initial empty diff left a blank tab")
	}
	drain(t, m, m.requestDiff(diffTarget{path: "error"}, func(context.Context) (string, error) { return "", fmt.Errorf("read failed") }))
	if len(m.tabs) != 1 {
		t.Fatal("read failure closed the tab")
	}
	for i, raw := range []string{
		"diff --git a/file b/file\nold mode 100644\nnew mode 100755\n",
		"diff --git a/file b/file\nBinary files a/file and b/file differ\n",
	} {
		drain(t, m, m.requestDiff(diffTarget{path: fmt.Sprint(i)}, func(context.Context) (string, error) { return raw, nil }))
	}
	if len(m.tabs) != 3 {
		t.Fatal("non-text changes were treated as empty")
	}
}
