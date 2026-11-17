package app

import (
	"path/filepath"
	"testing"
)

func TestExplorerKeyboardNavigationPreviewsWithoutTakingFocus(t *testing.T) {
	for _, keys := range [][2]string{{"j", "k"}, {"down", "up"}} {
		t.Run(keys[0], func(t *testing.T) {
			m := explorerWorkspace(t)
			defer m.Close()
			for i, entry := range m.items {
				if entry.file == ".gitignore" {
					m.selected = i
					break
				}
			}
			drain(t, m, press(m, keys[0]))
			if m.current().file != "README.md" || m.document() == nil || filepath.Base(m.document().Path) != "README.md" || !m.sidebar {
				t.Fatal("Explorer did not preview the next file while keeping tree focus")
			}
			if len(m.tabs) != 1 || !m.tabs[0].preview {
				t.Fatal("navigation must use a reusable preview tab")
			}
			drain(t, m, press(m, keys[1]))
			if m.current().file != ".gitignore" || filepath.Base(m.document().Path) != ".gitignore" || !m.sidebar || len(m.tabs) != 1 {
				t.Fatal("reverse navigation did not replace the preview or keep focus")
			}
			drain(t, m, press(m, keys[0]))
			drain(t, m, press(m, "enter"))
			if m.sidebar || m.tabs[m.activeTab].preview || filepath.Base(m.document().Path) != "README.md" {
				t.Fatal("Enter did not keep and focus the selected preview")
			}
		})
	}
}

func TestExplorerKeyboardPreviewRetainsDirtyDocument(t *testing.T) {
	m := explorerWorkspace(t)
	defer m.Close()
	for i, entry := range m.items {
		if entry.file == ".gitignore" {
			m.selected = i
			break
		}
	}
	drain(t, m, m.preview())
	original := m.document()
	original.Insert("unsaved")
	drain(t, m, press(m, "j"))
	if !m.sidebar || len(m.tabs) != 2 || m.document() == original || !original.Dirty() {
		t.Fatal("tree preview replaced dirty content or moved focus")
	}
	for _, tab := range m.tabs {
		if tab.doc == original && tab.preview {
			t.Fatal("dirty preview was not kept")
		}
	}
}

func TestTreesShareParentToggleAndFileActivation(t *testing.T) {
	for _, section := range []section{explorer, changes} {
		m := explorerWorkspace(t)
		defer m.Close()
		drain(t, m, m.selectSection(section))
		selectFolder := func() {
			t.Helper()
			for i, row := range m.items {
				if row.folder == "cmd" && !row.staged {
					m.selected = i
					return
				}
			}
			t.Fatal("missing cmd folder")
		}
		selectFolder()
		folder := *m.current()
		expanded := m.folderExpanded(folder)
		drain(t, m, press(m, "l"))
		if m.folderExpanded(folder) == expanded || !m.sidebar || m.current().key != folder.key {
			t.Fatal("l must toggle folder without changing focus or selection")
		}
		drain(t, m, press(m, "enter"))
		if m.folderExpanded(folder) != expanded || !m.sidebar {
			t.Fatal("Enter must toggle the same folder state as l")
		}
		drain(t, m, press(m, "h"))
		if m.folderExpanded(folder) != expanded || !m.sidebar {
			t.Fatal("h changed folder state or moved focus")
		}
		if section == changes && (m.current() == nil || !m.current().header || m.current().repoHeader) {
			t.Fatal("h did not select the Changes folder's group parent")
		}
		selectFolder()
		if !m.folderExpanded(folder) {
			drain(t, m, press(m, "l"))
		}
		for _, key := range []string{"l", "enter"} {
			found := false
			for i, row := range m.items {
				if row.file == "cmd/main.go" || row.change != nil && row.change.Path == "cmd/main.go" {
					m.selected = i
					found = true
					break
				}
			}
			if !found {
				t.Fatal("missing child file")
			}
			m.sidebar = true
			drain(t, m, press(m, key))
			if m.sidebar || m.activeTab < 0 || m.tabs[m.activeTab].preview || filepath.Base(m.tabs[m.activeTab].path()) != "main.go" {
				t.Fatalf("section %d: %s must keep and focus the selected file", section, key)
			}
		}
	}
}
