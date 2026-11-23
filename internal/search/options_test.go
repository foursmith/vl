package search

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSearchGlobPatterns(t *testing.T) {
	for _, tc := range []struct {
		patterns, path string
		want           bool
	}{
		{"*.go", "main.go", true}, {"*.go", "src/main.go", true}, {"*.go", "src/main.ts", false},
		{"src", "nested/src/a.go", true}, {"./src", "nested/src/a.go", false}, {"./src", "src/a.go", true},
		{"src/**", "src/deep/a.go", true}, {"**/test?.{go,ts}", "test1.ts", true},
		{"*.{go,ts},README.md", "nested/a.ts", true}, {"*.{go,ts},README.md", "README.md", true},
		{"file[0-9].txt", "file2.txt", true}, {"file[!0-9].txt", "filea.txt", true}, {"file[!0-9].txt", "file2.txt", false},
		{"例子/**", "src/例子/中文.go", true}, {"[[]test].go", "[test].go", true},
		{"a?c", "a/c", false}, {"*.go", "x.go/a.txt", true},
	} {
		patterns, err := compileGlobs(tc.patterns)
		if err != nil {
			t.Fatal(tc.patterns, err)
		}
		if got := matchesGlobs(patterns, tc.path); got != tc.want {
			t.Errorf("%q / %q = %v", tc.patterns, tc.path, got)
		}
	}
	for _, pattern := range []string{"[abc", "{go,ts", "abc}", "[z-a]"} {
		if _, err := compileGlobs(pattern); err == nil {
			t.Errorf("accepted invalid pattern %q", pattern)
		}
	}
}
func TestPreserveReplacementCase(t *testing.T) {
	for _, tc := range []struct{ old, next, want string }{
		{"WORD", "hello", "HELLO"}, {"word", "Hello", "hello"}, {"Word", "hello", "Hello"},
		{"wOrd", "Hello", "hello"}, {"Foo-BAR", "new-name", "New-NAME"},
		{"FOO_bar", "new_name", "NEW_name"}, {"ÉCOLE", "été", "ÉTÉ"}, {"École", "été", "Été"},
		{"中文", "New", "New"}, {"", "New", "New"}, {"WORD", "", ""},
	} {
		if got := preserveCase(tc.old, tc.next); got != tc.want {
			t.Errorf("%q -> %q = %q, want %q", tc.old, tc.next, got, tc.want)
		}
	}
	matches := findMatches(t, Options{Query: "(foo)", Regex: true, PreserveCase: true}, "FOO Foo foo", "$1Name")
	for i, want := range []string{"FOONAME", "FooName", "fooname"} {
		if matches[i].Replacement != want {
			t.Fatalf("capture preserve: %+v", matches)
		}
	}
}
func TestSearchIncludeExcludeOpenAndChanged(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	initSearchRepository(t, root)
	for _, name := range []string{"src/a.go", "src/b.ts", "test/c.go", "clean.go"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0755); err != nil {
			t.Fatal(err)
		}
		writeSearchFile(t, filepath.Join(root, name), "needle")
	}
	searchGit(t, root, "add", ".")
	searchGit(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.test", "-c", "commit.gpgsign=false", "commit", "-m", "fixture")
	writeSearchFile(t, filepath.Join(root, "src/a.go"), "needle updated")
	writeSearchFile(t, filepath.Join(root, "untracked.go"), "needle")
	buffers := map[string]string{filepath.Join(root, "src/a.go"): "needle updated", filepath.Join(root, "clean.go"): "needle unsaved"}
	for _, tc := range []struct {
		o    Options
		want int
	}{
		{Options{Query: "needle", Include: "*.go", Exclude: "test"}, 3},
		{Options{Query: "needle", OnlyOpen: true}, 2},
		{Options{Query: "needle", ChangedOnly: true}, 3},
		{Options{Query: "needle", OnlyOpen: true, ChangedOnly: true, Exclude: "clean.go"}, 1},
		{Options{Query: "needle", OnlyOpen: true, Include: "*.ts"}, 0},
	} {
		got, err := Run(context.Background(), root, tc.o, buffers)
		if err != nil || got.Count != tc.want {
			t.Fatalf("%+v: count=%d want=%d err=%v", tc.o, got.Count, tc.want, err)
		}
	}
	if _, err := Run(context.Background(), root, Options{Query: "needle", Include: "[bad"}, nil); err == nil {
		t.Fatal("invalid include was accepted")
	}
}

func TestSearchIgnoreFilesWithoutVSCodeSettings(t *testing.T) {
	for _, git := range []bool{false, true} {
		root, _ := filepath.EvalSymlinks(t.TempDir())
		if git {
			initSearchRepository(t, root)
		}
		if err := os.MkdirAll(filepath.Join(root, ".vscode"), 0755); err != nil {
			t.Fatal(err)
		}
		writeSearchFile(t, filepath.Join(root, ".gitignore"), "git.txt\n")
		writeSearchFile(t, filepath.Join(root, ".ignore"), "ignore.txt\n*.tmp\n!keep.tmp\n")
		writeSearchFile(t, filepath.Join(root, ".vscode/settings.json"), `{
   "files.exclude": {"settings.txt":true,"keep.tmp":true},
   "search.exclude": {"keep.tmp":false,"**/*.js":{"when":"$(basename).ts"}},
  }`)
		for _, name := range []string{"git.txt", "ignore.txt", "settings.txt", "keep.tmp", "drop.tmp", "normal.txt", "app.js", "app.ts", "other.js"} {
			writeSearchFile(t, filepath.Join(root, name), "needle")
		}
		if git {
			searchGit(t, root, "add", "-f", "git.txt", "ignore.txt")
		}
		for _, o := range []Options{{Query: "needle"}, {Query: "needle", OnlyOpen: true}, {Query: "needle", IncludeIgnored: true, Exclude: "normal.txt"}} {
			buffers := map[string]string{}
			if o.OnlyOpen {
				for _, name := range []string{"ignore.txt", "settings.txt", "normal.txt", "git.txt"} {
					buffers[filepath.Join(root, name)] = "needle"
				}
			}
			got, err := Run(context.Background(), root, o, buffers)
			want := 6
			if o.OnlyOpen {
				want = 2
			}
			if o.IncludeIgnored {
				want = 8
			}
			if err != nil || got.Count != want {
				t.Fatalf("git=%v %+v: count=%d want=%d err=%v files=%+v", git, o, got.Count, want, err, got.Files)
			}
		}
	}
}
