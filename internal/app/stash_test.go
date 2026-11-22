package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStashPickerReviewsNestedRepositoryFiles(t *testing.T) {
	m, child := childWorkspace(t)
	for name, data := range map[string]string{"README.md": "child edit\n", "new.txt": "new file\n"} {
		if err := os.WriteFile(filepath.Join(child.Root, name), []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := child.Run(m.ctx, "stash", "push", "-u", "-m", "child stash"); err != nil {
		t.Fatal(err)
	}
	drain(t, m, m.selectSection(changes))
	for i, entry := range m.items {
		if entry.repoHeader && entry.repository == "child space" {
			m.selected = i
			break
		}
	}
	found := false
	for _, a := range m.actions() {
		if a.id == "git.stashList" {
			drain(t, m, a.run())
			found = true
			break
		}
	}
	if !found || m.modal != "stashPicker" || len(m.palette) != 2 || !strings.Contains(m.palette[0].label, "child stash") {
		t.Fatalf("expected child repository stash list, got %s %+v", m.modal, m.palette)
	}
	drain(t, m, press(m, "enter"))
	if len(m.palette) != 4 || m.palette[1].label != "new.txt" || m.palette[1].key != "Untracked" {
		t.Fatalf("expected both child stash files, got %+v", m.palette)
	}
	m.selectPickerIndex(1)
	drain(t, m, press(m, "enter"))
	if m.diffPath != "child space/new.txt" || m.diffRepository != "child space" || m.diffChange != nil ||
		!strings.Contains(m.patch.Raw, "+new file") {
		t.Fatalf("expected a read-only nested stash diff, got %s\n%s", m.diffPath, m.patch.Raw)
	}
}
