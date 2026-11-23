package search

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestListFilePathsReadsLocalIgnoreRules(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root := t.TempDir()
	files := map[string]string{
		".gitignore":        "/root.txt\n*.log\nblocked/\n!blocked/file.go\n!keep.log\nfoo\n!foo/\n\\#hidden\nassets/**/skip.rs\n{literal}\n",
		".ignore":           "extra.go\n",
		".git/config":       "",
		"root.txt":          "",
		"keep.log":          "",
		"drop.log":          "",
		"extra.go":          "",
		"blocked/file.go":   "",
		"foo/file.go":       "",
		"#hidden":           "",
		"{literal}":         "",
		"literal":           "",
		".hidden.go":        "",
		"assets/skip.rs":    "",
		"assets/a/skip.rs":  "",
		"child/root.txt":    "",
		"child/.gitignore":  "!drop.log\n/deep-only\n",
		"child/drop.log":    "",
		"child/deep-only":   "",
		"child/a/deep-only": "",
		"nested/.git":       "gitdir: unavailable",
		"nested/source.go":  "",
	}
	for name, text := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		writeSearchFile(t, path, text)
	}
	want := []string{".gitignore", ".hidden.go", ".ignore", "child/.gitignore", "child/a/deep-only", "child/drop.log", "child/root.txt", "foo/file.go", "keep.log", "literal", "nested/source.go"}
	got, err := ListFilePaths(context.Background(), root, false)
	if err != nil || !slices.Equal(want, got) {
		t.Fatalf("paths = %q, want %q, err = %v", got, want, err)
	}
	all, err := ListFilePaths(context.Background(), root, true)
	if err != nil || len(all) != len(files)-2 || !slices.Contains(all, "blocked/file.go") ||
		!slices.Contains(all, "extra.go") || slices.Contains(all, ".git/config") || slices.Contains(all, "nested/.git") {
		t.Fatalf("including ignored paths returned %q, err = %v", all, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ListFilePaths(ctx, root, false); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled scan returned %v", err)
	}
}

func TestPathIgnoreRulesHandleEscapesAndWildcards(t *testing.T) {
	for _, test := range []struct{ rule, match, miss string }{
		{`trailing\ `, "trailing ", "trailing"},
		{`dir/a**b`, "dir/ab", "dir/a/b"},
		{`dir/**/b`, "dir/a/b", "other/b"},
		{`[[:digit:]].txt`, "3.txt", "a.txt"},
		{`\!literal`, "!literal", "literal"},
	} {
		rule, ok := compilePathIgnoreRule("", test.rule)
		if !ok || !ignoredFilePath([]pathIgnoreRule{rule}, test.match, filepath.Base(test.match), false) ||
			ignoredFilePath([]pathIgnoreRule{rule}, test.miss, filepath.Base(test.miss), false) {
			t.Errorf("rule %q did not match %q and exclude %q", test.rule, test.match, test.miss)
		}
	}
}
