package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestStartupLocations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	colonPath := path + ":2"
	for _, name := range []string{path, colonPath} {
		if err := os.WriteFile(name, []byte("first\n中文value\nlast"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tt := range []struct {
		value, path string
		line, col   int
	}{
		{path, path, 0, 0},
		{path + ":3", path, 3, 1},
		{path + ":3:2", path, 3, 2},
		{colonPath, colonPath, 0, 0},
		{colonPath + ":3:2", colonPath, 3, 2},
	} {
		got, line, col, err := startupLocation(tt.value)
		if err != nil || got != tt.path || line != tt.line || col != tt.col {
			t.Fatalf("startupLocation(%q) = %q, %d, %d, %v", tt.value, got, line, col, err)
		}
	}
	for _, suffix := range []string{":", ":0", ":-1", ":x", ":3:0", ":3:x", ":3:2:1", ":999999999999999999999"} {
		if _, _, _, err := startupLocation(path + suffix); err == nil {
			t.Fatalf("accepted invalid location %q", suffix)
		}
	}
	if _, _, _, err := startupModel(context.Background(), dir+":3", false); err == nil {
		t.Fatal("accepted directory location")
	}
	if err := os.Remove(colonPath); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	for _, tt := range []struct{ suffix, status string }{
		{":2:3", "Ln 2, Col 3"},
		{":3", "Ln 3, Col 1"},
		{":999:999", "Ln 3, Col 5"},
	} {
		m, repo, only, err := startupModel(context.Background(), path+tt.suffix, false)
		if err != nil {
			t.Fatal(err)
		}
		m.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
		content := m.View().Content
		m.Close()
		if !only || repo != nil || !strings.Contains(content, tt.status) {
			t.Fatalf("location %s failed: editor=%v, view=%s", tt.suffix, only, content)
		}
	}
}
