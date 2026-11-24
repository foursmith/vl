package app

import (
	"os"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func sidebarSearchWorkspace(t *testing.T, count int) *Model {
	t.Helper()
	m := tabWorkspace(t, count)
	t.Cleanup(m.Close)
	m.openSearch()
	m.search.query.SetValue("package")
	drain(t, m, m.runSearch())
	m.focusSearch(2)
	return m
}

func TestSearchFileOperatorsDoNotMutateDisk(t *testing.T) {
	for _, key := range []string{"dd", "Delete"} {
		t.Run(key, func(t *testing.T) {
			m := sidebarSearchWorkspace(t, 2)
			path := m.search.result.Files[0].Path
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			m.explorerYankRoot, m.explorerYankPath = m.repo.Root, "file0.go"
			for _, key := range []string{"y", "y", "p"} {
				press(m, key)
			}
			if m.modal != "" || m.busy {
				t.Fatal("search exposed disk operations")
			}
			count := len(m.search.result.Files)
			if key == "dd" {
				press(m, "d")
				if len(m.search.result.Files) != count {
					t.Fatal("single d dismissed a result")
				}
				press(m, "d")
			} else {
				m.Update(tea.KeyPressMsg{Code: tea.KeyDelete})
			}
			if len(m.search.result.Files) != count-1 || m.modal != "" || m.busy {
				t.Fatalf("%s did not dismiss the file group", key)
			}
			after, err := os.ReadFile(path)
			if err != nil || string(after) != string(before) {
				t.Fatal("dismissing a result modified the file")
			}
		})
	}
}

func TestSearchContextMenuResolvesResultAndRejectsChangedTarget(t *testing.T) {
	m := sidebarSearchWorkspace(t, 3)
	index := len(m.searchRows()) - 1
	m.search.top = 1
	y := m.searchResultY() + index - m.search.top
	m.Update(tea.MouseClickMsg{X: m.sideX() + 5, Y: y, Button: tea.MouseRight})
	if m.contextMenu == nil || m.search.selected != index {
		t.Fatal("context menu did not resolve search coordinates")
	}
	for _, a := range m.contextMenu.actions {
		if a.label == "Copy" || a.label == "Paste" || a.label == "Delete…" {
			t.Fatal("search menu contains a disk operation")
		}
	}
	path := m.sidebarEntry().file
	count := m.search.result.Count
	press(m, "d")
	if m.contextMenu == nil || m.search.result.Count != count {
		t.Fatal("first d acted before the menu shortcut was complete")
	}
	press(m, "d")
	if m.search.result.Count >= count {
		t.Fatal("menu did not dismiss the selected match")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("dismissal deleted the file")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyF7})
	if m.contextMenu == nil {
		t.Fatal("keyboard menu unavailable in results")
	}
	m.search.result.Files = nil
	m.Update(tea.KeyPressMsg{Code: tea.KeyDelete})
	if m.modal != "" || m.notice != "Target changed; reopen the menu" {
		t.Fatal("stale menu acted on a replacement result")
	}
}

func TestSearchUsesSharedPreviewAndKeepBehavior(t *testing.T) {
	for _, double := range []bool{false, true} {
		m := sidebarSearchWorkspace(t, 1)
		m.clickSearch(7, m.searchResultY())
		if len(m.tabs) != 1 || !m.tabs[0].preview {
			t.Fatal("single click did not preview search file")
		}
		if double {
			m.clickSearch(7, m.searchResultY())
		} else {
			m.sidebar = true
			m.focusSearch(2)
			press(m, "enter")
		}
		if len(m.tabs) != 1 || m.tabs[0].preview || m.sidebar {
			t.Fatal("activation did not keep and focus search file")
		}
	}
}

func TestSearchNavigationReusesPreviewAndPreservesListFocus(t *testing.T) {
	m := sidebarSearchWorkspace(t, 2)
	for _, key := range []string{"j", "j", "j"} {
		press(m, key)
	}
	if len(m.tabs) != 1 || !m.tabs[0].preview || !m.sidebar || m.search.focus != 2 {
		t.Fatal("navigation did not reuse preview with list focus")
	}
	if m.document().Path != m.sidebarEntry().file {
		t.Fatal("preview does not follow selection")
	}
	press(m, "enter")
	if m.tabs[0].preview || m.sidebar {
		t.Fatal("Enter did not keep the preview")
	}
}

func TestSearchLeftSelectsParentWithoutCollapsing(t *testing.T) {
	for _, key := range []string{"h", "left"} {
		m := sidebarSearchWorkspace(t, 2)
		m.search.selected = 3
		m.preview()
		doc := m.document()
		selectedFile := m.searchRows()[m.search.selected].file
		path := m.search.result.Files[selectedFile].Path
		rows := len(m.searchRows())
		m.Update(tea.KeyPressMsg{Code: map[string]rune{"h": 'h', "left": tea.KeyLeft}[key]})
		selected := m.searchRows()[m.search.selected]
		if selected.file != selectedFile || selected.match != -1 {
			t.Fatal("left did not select the enclosing file group")
		}
		if m.search.folded[path] || len(m.searchRows()) != rows {
			t.Fatal("left collapsed the file group")
		}
		if m.document() != doc || !m.sidebar {
			t.Fatal("left changed preview or list focus")
		}
		m.Update(tea.KeyPressMsg{Code: map[string]rune{"h": 'h', "left": tea.KeyLeft}[key]})
		if m.search.folded[path] || len(m.searchRows()) != rows || m.searchRows()[m.search.selected] != selected {
			t.Fatal("left on a group changed its state")
		}
	}
}
