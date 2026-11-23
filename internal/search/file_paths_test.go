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

func TestWorkspaceLocalIgnorePrecedence(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		".gitignore":        "plain.txt\nnested/allow.txt\n",
		".ignore":           "nested/lower.txt\n!nested/allow.txt\n",
		".rgignore":         "\uFEFFnested/locked*.txt\r\nnested/top.txt\n!nested/lower.txt\n",
		"nested/.ignore":    "!locked*.txt\nlower.txt\n",
		"nested/.rgignore":  "!top.txt\n",
		"plain.txt":         "needle",
		"nested/allow.txt":  "needle",
		"nested/locked.txt": "needle",
		"nested/lower.txt":  "needle",
		"nested/top.txt":    "needle",
	}
	for name, text := range files {
		file := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatal(err)
		}
		writeSearchFile(t, file, text)
	}
	deleted := filepath.Join(root, "nested", "locked-deleted.txt")
	buffers := map[string]string{deleted: "unsaved needle"}
	for _, includeIgnored := range []bool{false, true} {
		searchPaths := []string{"nested/allow.txt", "nested/lower.txt", "nested/top.txt"}
		if includeIgnored {
			searchPaths = append(searchPaths, "nested/locked.txt", "plain.txt")
			slices.Sort(searchPaths)
		}
		paths, err := ListFilePaths(context.Background(), root, includeIgnored)
		if err != nil {
			t.Fatal(err)
		}
		paths = slices.DeleteFunc(paths, func(path string) bool { return filepath.Ext(path) != ".txt" })
		if !slices.Equal(paths, searchPaths) {
			t.Fatalf("includeIgnored=%t: paths %q, want %q", includeIgnored, paths, searchPaths)
		}
		if includeIgnored {
			searchPaths = append(searchPaths, "nested/locked-deleted.txt")
			slices.Sort(searchPaths)
		}
		result, err := collectResults(context.Background(), root, Options{Query: "needle", IncludeIgnored: includeIgnored}, buffers)
		if err != nil {
			t.Fatal(err)
		}
		var matches []string
		for _, file := range result.Files {
			relative, err := filepath.Rel(root, file.Path)
			if err != nil {
				t.Fatal(err)
			}
			matches = append(matches, filepath.ToSlash(relative))
		}
		if !slices.Equal(matches, searchPaths) {
			t.Fatalf("includeIgnored=%t: matches %q, want %q", includeIgnored, matches, searchPaths)
		}
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
		rules := pathIgnoreRules{{rule}}
		if !ok || !rules.ignored(test.match, filepath.Base(test.match), false) ||
			rules.ignored(test.miss, filepath.Base(test.miss), false) {
			t.Errorf("rule %q did not match %q and exclude %q", test.rule, test.match, test.miss)
		}
	}
}
