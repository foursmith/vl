package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTreeVimNavigationAndFileOperators(t *testing.T) {
	m := explorerWorkspace(t)
	defer m.Close()
	for _, k := range []string{"g", "g", "2", "j"} {
		drain(t, m, press(m, k))
	}
	if m.selected != 2 {
		t.Fatalf("2j selected %d", m.selected)
	}
	press(m, "G")
	if m.selected != len(m.items)-1 {
		t.Fatal("G")
	}
	press(m, "3")
	press(m, "G")
	if m.selected != 2 {
		t.Fatal("3G")
	}
	drain(t, m, press(m, "l"))
	if !m.expanded["cmd"] {
		t.Fatal("l did not expand folder")
	}
	drain(t, m, press(m, "h"))
	if !m.current().header || !m.expanded["cmd"] {
		t.Fatal("h should select the workspace root and leave cmd expanded")
	}
	drain(t, m, press(m, "2"))
	drain(t, m, press(m, "j"))
	drain(t, m, press(m, "l"))
	if m.expanded["cmd"] {
		t.Fatal("l did not collapse the expanded folder")
	}
	for i, e := range m.items {
		if e.file == "README.md" {
			m.selected = i
		}
	}
	press(m, "y")
	press(m, "y")
	press(m, "p")
	if m.modal != "pasteFile" {
		t.Fatal("yy p did not prompt")
	}
	m.input.SetValue("copied.md")
	drain(t, m, press(m, "enter"))
	data, err := os.ReadFile(filepath.Join(m.repo.Root, "copied.md"))
	if err != nil || string(data) != "# Workspace\n" {
		t.Fatalf("copy: %q %v", data, err)
	}
	for i, e := range m.items {
		if e.file == "copied.md" {
			m.selected = i
		}
	}
	press(m, "d")
	press(m, "d")
	if m.modal != "confirm" {
		t.Fatal("dd bypassed confirmation")
	}
	press(m, "esc")
	if _, err = os.Stat(filepath.Join(m.repo.Root, "copied.md")); err != nil {
		t.Fatal("cancel deleted file")
	}
	// Unfinished operators cannot survive a focus switch.
	press(m, "d")
	press(m, "f6")
	press(m, "f6")
	press(m, "d")
	if m.modal != "" {
		t.Fatal("unfinished dd crossed focus switch")
	}
}

func TestDiffVimNavigationAndShortcutPriority(t *testing.T) {
	m := reviewWorkspace(t, false)
	press(m, "3")
	press(m, "j")
	if row, _ := m.review.Position(); row != 3 {
		t.Fatal("3j did not move the read-only cursor")
	}
	press(m, "G")
	if row, _ := m.review.Position(); row != len(m.review.Lines())-1 || m.reviewCursor() == nil {
		t.Fatal("G did not reveal the last source line")
	}
	press(m, "g")
	press(m, "g")
	if row, _ := m.review.Position(); row != 0 {
		t.Fatal("gg")
	}
	split := m.split
	press(m, "ctrl+d")
	if m.split == split || !m.review.ReadOnly {
		t.Fatal("Ctrl+D must change layout without changing read-only capability")
	}
	press(m, "ctrl+f")
	if !m.fileFindFocused() || m.modal != "" {
		t.Fatal("Ctrl+F must open the shared find widget")
	}
}

func TestVimFileOperatorsProtectDirtyFilesAndOtherSections(t *testing.T) {
	m := explorerWorkspace(t)
	defer m.Close()
	m.openFile("README.md")
	m.sidebar = true
	for i, e := range m.items {
		if e.file == "README.md" {
			m.selected = i
		}
	}
	m.document().Insert("unsaved")
	press(m, "y")
	press(m, "y")
	if m.explorerYankPath != "" {
		t.Fatal("copied dirty file")
	}
	press(m, "d")
	press(m, "d")
	if m.modal != "" {
		t.Fatal("allowed dirty file deletion")
	}
	m.document().Undo()
	drain(t, m, m.selectSection(changes))
	selectChanged(t, m, "README.md", false)
	press(m, "d")
	press(m, "d")
	if m.modal != "confirm" || m.confirmationKey != changesDiscardConfirmKey || m.busy {
		t.Fatal("dd should only open the Git discard confirmation in Changes")
	}
}

func TestMenuVimCountsAndEdges(t *testing.T) {
	m := explorerWorkspace(t)
	defer m.Close()
	m.contextMenu = &contextMenu{actions: []action{{label: "a"}, {label: "b"}, {label: "c"}, {label: "d"}}}
	press(m, "2")
	press(m, "j")
	if m.contextMenu.selected != 2 {
		t.Fatal("menu 2j")
	}
	press(m, "g")
	press(m, "g")
	if m.contextMenu.selected != 0 {
		t.Fatal("menu gg")
	}
	press(m, "G")
	if m.contextMenu.selected != 3 {
		t.Fatal("menu G")
	}
	press(m, "h")
	if m.contextMenu != nil {
		t.Fatal("menu h")
	}
}
