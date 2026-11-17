package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSessionStorage(t *testing.T) {
	first, err := SessionPath("/work/first")
	if err != nil {
		t.Fatal(err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := filepath.Dir(first), filepath.Join(home, ".config", "vl", "workspaces"); got != want {
		t.Fatalf("workspace directory = %q, want %q", got, want)
	}
	second, err := SessionPath("/work/second")
	if err != nil || first == second {
		t.Fatal("workspaces share session path")
	}
	path := filepath.Join(t.TempDir(), "session.json")
	if _, err := LoadSession(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSession(path); err == nil {
		t.Fatal("invalid session accepted")
	}
}
