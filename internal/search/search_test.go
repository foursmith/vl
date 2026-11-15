package search

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindOptionsUnicodeAndReplacement(t *testing.T) {
	tests := []struct {
		o                 Options
		text, replacement string
		want              int
		expanded          string
	}{
		{Options{Query: "cat"}, "CAT cat scatter", "$1", 3, "$1"},
		{Options{Query: "cat", Case: true, Word: true}, "CAT cat scatter", "dog", 1, "dog"},
		{Options{Query: "猫", Word: true}, "小猫 猫 猫咪", "犬", 1, "犬"},
		{Options{Query: `(猫)(\d+)`, Regex: true}, "前\n中猫12 後", "$2-$1", 1, "12-猫"},
		{Options{Query: `^`, Regex: true}, "猫", "x", 1, "x"},
		{Options{Query: "a.b"}, "axb a.b", "c", 1, "c"},
	}
	for _, tt := range tests {
		got := findMatches(t, tt.o, tt.text, tt.replacement)
		if len(got) != tt.want {
			t.Fatalf("%+v: %+v", tt.o, got)
		}
		if got[0].Replacement != tt.expanded {
			t.Fatalf("expansion: %+v", got[0])
		}
	}
	got := findMatches(t, Options{Query: `猫\d+`, Regex: true}, "前\n中猫12", "")
	if got[0].Line != 2 || got[0].Start != 7 || got[0].End != 12 {
		t.Fatalf("Unicode match offsets: %+v", got)
	}
	if _, err := (Options{Query: "[", Regex: true}).Compile(); err == nil {
		t.Fatal("invalid regex accepted")
	}
	got = findMatches(t, Options{Query: "cat", Word: true}, strings.Repeat("scatter ", 2501)+"cat", "")
	if len(got) != 1 {
		t.Fatal("valid later word match lost")
	}
}
func findMatches(t *testing.T, options Options, text, replacement string) []Match {
	t.Helper()
	matcher, err := options.Compile()
	if err != nil {
		t.Fatal(err)
	}
	return matcher.Find(text, replacement)
}

func TestWorkspaceIgnoresBinarySymlinksAndUsesBuffers(t *testing.T) {
	root := t.TempDir()
	root, _ = filepath.EvalSymlinks(root)
	if out, err := exec.Command("git", "init", root).CombinedOutput(); err != nil {
		t.Fatalf("%s %v", out, err)
	}
	for name, body := range map[string]string{".gitignore": "ignored\n", "ignored": "needle", "binary": "needle\x00", "text": "disk", "many": "猫 needle\nneedle"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	outside := filepath.Join(t.TempDir(), "outside")
	os.WriteFile(outside, []byte("needle"), 0644)
	os.Symlink(outside, filepath.Join(root, "link"))
	result, err := Run(context.Background(), root, Options{Query: "needle"}, map[string]string{filepath.Join(root, "text"): "buffer needle", filepath.Join(root, "ignored"): "open needle"})
	if err != nil || result.Count != 3 || len(result.Files) != 2 {
		t.Fatalf("%+v %v", result, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Run(ctx, root, Options{Query: "needle"}, nil); err == nil {
		t.Fatal("cancel ignored")
	}
	os.WriteFile(filepath.Join(root, "many"), []byte(strings.Repeat("needle\n", 2501)), 0644)
	result, err = Run(context.Background(), root, Options{Query: "needle"}, nil)
	if err != nil || result.Unreadable > 0 || result.Count != 2501 {
		t.Fatalf("full results: %+v %v", result, err)
	}
}

func initSearchRepository(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", root).CombinedOutput(); err != nil {
		t.Fatalf("%s: %v", out, err)
	}
}
func searchGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s: %v", args, out, err)
	}
	return strings.TrimSpace(string(out))
}
func writeSearchFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}
func TestWorkspaceSearchesNestedRepositoriesAndTheirIgnoreRules(t *testing.T) {
	root := t.TempDir()
	root, _ = filepath.EvalSymlinks(root)
	initSearchRepository(t, root)
	child := filepath.Join(root, "module")
	initSearchRepository(t, child)
	deep := filepath.Join(child, "deep")
	initSearchRepository(t, deep)
	embedded := filepath.Join(root, "embedded")
	initSearchRepository(t, embedded)
	for _, dir := range []string{root, child, deep, embedded} {
		writeSearchFile(t, filepath.Join(dir, "tracked.txt"), "needle")
		searchGit(t, dir, "add", "tracked.txt")
		writeSearchFile(t, filepath.Join(dir, "untracked.txt"), "needle")
		writeSearchFile(t, filepath.Join(dir, ".gitignore"), "ignored.txt\n")
		writeSearchFile(t, filepath.Join(dir, "ignored.txt"), "needle")
	}
	// Real gitlink entries exercise the exact top-level listing that previously skipped submodules.
	for _, pair := range [][2]string{{child, "deep"}, {root, "module"}} {
		nested := filepath.Join(pair[0], pair[1])
		searchGit(t, nested, "-c", "user.name=Test", "-c", "user.email=test@example.test", "-c", "commit.gpgsign=false", "commit", "-m", "fixture")
		sha := searchGit(t, nested, "rev-parse", "HEAD")
		searchGit(t, pair[0], "update-index", "--add", "--cacheinfo", "160000,"+sha+","+pair[1])
	}
	searchGit(t, child, "init", "--separate-git-dir="+filepath.Join(t.TempDir(), "module.git"))
	for _, include := range []bool{false, true} {
		result, err := Run(context.Background(), root, Options{Query: "needle", IncludeIgnored: include}, nil)
		expected := 8
		if include {
			expected = 12
		}
		if err != nil || result.Count != expected || result.Unreadable > 0 {
			t.Fatalf("ignored=%v: got %d matches in %d files, want %d: %v", include, result.Count, len(result.Files), expected, err)
		}
		for _, file := range result.Files {
			if strings.Contains(file.Path, string(filepath.Separator)+".git"+string(filepath.Separator)) {
				t.Fatal("searched Git metadata")
			}
		}
	}
}
func TestWorkspaceDoesNotStopAtFormerByteOrFileSizeLimits(t *testing.T) {
	root := t.TempDir()
	root, _ = filepath.EvalSymlinks(root)
	initSearchRepository(t, root)
	// Files before the final match cumulatively exceed the former 64 MiB scan budget.
	padding := strings.Repeat("x", 4<<20)
	for i := 0; i < 17; i++ {
		writeSearchFile(t, filepath.Join(root, fmt.Sprintf("a%02d.txt", i)), padding)
	}
	// Larger than the editor limit, with mixed line endings; still valid searchable text.
	writeSearchFile(t, filepath.Join(root, "z.txt"), padding+"\r\nneedle\nneedle")
	result, err := Run(context.Background(), root, Options{Query: "needle"}, nil)
	if err != nil || result.Count != 2 || result.Unreadable > 0 {
		t.Fatalf("late matches lost: count=%d unreadable=%d err=%v", result.Count, result.Unreadable, err)
	}
	if hit := result.Files[0].Matches[0]; hit.Line != 2 || hit.Start != len(padding)+1 {
		t.Fatalf("normalized position: %+v", hit)
	}
}
func TestWorkspaceIncludesDeletedOpenBuffersWithoutSearchingMetadata(t *testing.T) {
	root := t.TempDir()
	root, _ = filepath.EvalSymlinks(root)
	initSearchRepository(t, root)
	result, err := Run(context.Background(), root, Options{Query: "needle"}, map[string]string{filepath.Join(root, "deleted.txt"): "unsaved needle", filepath.Join(root, ".git", "config"): "needle"})
	if err != nil || result.Count != 1 || filepath.Base(result.Files[0].Path) != "deleted.txt" {
		t.Fatalf("open buffer search: %+v %v", result, err)
	}
}
func TestFindAllMatchesAndMultilineAnchors(t *testing.T) {
	matches := findMatches(t, Options{Query: "needle"}, strings.Repeat("needle ", 3000), "new")
	if len(matches) != 3000 || matches[2999].Start != 2999*7 {
		t.Fatalf("all matches: %d", len(matches))
	}
	matches = findMatches(t, Options{Query: "^needle$", Regex: true}, "before\nneedle\nafter\nneedle", "")
	if len(matches) != 2 || matches[1].Line != 4 {
		t.Fatalf("line anchors: %+v", matches)
	}
}
