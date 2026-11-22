package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestInsertRelativePathCompletion(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "src")
	folder := filepath.Join(root, "assets")
	for _, path := range []string{dir, folder} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	source := filepath.Join(dir, "source.txt")
	for _, path := range []string{source, filepath.Join(folder, "alpha.txt"), filepath.Join(folder, "文件 名.txt")} {
		if err := os.WriteFile(path, nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	m, err := NewEditor(context.Background(), source, false)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	m.Update(tea.WindowSizeMsg{Width: 30, Height: 8})
	sendKey := func(code rune) {
		t.Helper()
		_, cmd := m.Update(tea.KeyPressMsg{Code: code})
		drain(t, m, cmd)
	}
	press(m, "i")
	for _, r := range `"../a` {
		drain(t, m, press(m, string(r)))
	}
	if !strings.Contains(plain(m.View().Content), "assets/") {
		t.Fatal("parent directory candidate was not visible")
	}
	sendKey(tea.KeyTab)
	if m.document().Text != `"../assets/` || len(m.pathCompletion.matches) != 2 {
		t.Fatal("directory completion did not continue to the next level")
	}
	sendKey(tea.KeyPgDown)
	if m.pathCompletion == nil || m.pathCompletion.selected != 1 {
		t.Fatal("PageDown did not move within the suggestions")
	}
	sendKey(tea.KeyPgUp)
	if m.pathCompletion == nil || m.pathCompletion.selected != 0 {
		t.Fatal("PageUp did not move within the suggestions")
	}
	press(m, "ctrl+j")
	press(m, "ctrl+k")
	if m.pathCompletion == nil || m.pathCompletion.selected != 0 {
		t.Fatal("Ctrl+K did not move to the previous suggestion")
	}
	press(m, "ctrl+j")
	sendKey(tea.KeyTab)
	press(m, `"`)
	if want := `"../assets/文件 名.txt"`; m.document().Text != want {
		t.Fatalf("completed text = %q, want %q", m.document().Text, want)
	}
	if m.pathCompletion != nil {
		t.Fatal("file completion left a popup open")
	}
	press(m, "esc")
	press(m, "u")
	if m.document().Text != "" {
		t.Fatalf("undo left %q", m.document().Text)
	}

	press(m, "i")
	press(m, ".")
	pending := press(m, "/")
	press(m, "esc")
	drain(t, m, pending)
	if m.pathCompletion != nil || m.tabs[m.activeTab].vim.InsertMode {
		t.Fatal("a stale directory result reopened completion after Esc")
	}
}

func TestRelativePathPrefix(t *testing.T) {
	for _, tc := range []struct {
		before, after, path string
		quote               rune
	}{
		{`import "./src/`, `"`, "./src/", '"'},
		{`import './my folder/文`, `'`, "./my folder/文", '\''},
		{"[link](../", ")", "../", 0},
		{"x = ./", "", "./", 0},
		{"https://example.com/../", "", "", 0},
		{`import "./src/`, `main.go"`, "", 0},
		{`import "./my folder/`, ` my file"`, "", 0},
	} {
		if path, quote := relativePathPrefix(tc.before, tc.after); path != tc.path || quote != tc.quote {
			t.Errorf("relativePathPrefix(%q, %q) = %q, %q", tc.before, tc.after, path, quote)
		}
	}
}
