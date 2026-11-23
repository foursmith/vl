package search

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRipgrepWorkspaceSearchParity(t *testing.T) {
	rg, err := exec.LookPath("rg")
	if err != nil {
		t.Skip("ripgrep is unavailable")
	}
	root := t.TempDir()
	bodies := map[string]string{
		".ignore":        "ignored.txt\n",
		".rgignore":      "rg-ignored.txt\n",
		"ignored.txt":    "needle",
		"rg-ignored.txt": "needle",
		".hidden.txt":    "NEEDLE",
		"open.txt":       "needle",
		"normal.txt":     "\xef\xbb\xbfneedle\r\nsecond line\r\n",
		"fold.txt":       "ſcope K",
		"greek.txt":      "Ω",
		"dash.txt":       "--",
		"binary.txt":     "needle\x00",
	}
	if runtime.GOOS != "windows" {
		bodies["space and\nnewline.txt"] = "needle"
	}
	for name, body := range bodies {
		writeSearchFile(t, filepath.Join(root, name), body)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	buffers := map[string]string{
		filepath.Join(root, "open.txt"):    "unsaved scope",
		filepath.Join(root, "deleted.txt"): "needle",
	}
	cases := []struct {
		options Options
		rgMatch string
	}{
		{Options{Query: "needle"}, ".hidden.txt"},
		{Options{Query: "needle", Case: true, IncludeIgnored: true, Exclude: "normal.txt"}, "ignored.txt"},
		{Options{Query: "scope", Word: true}, "fold.txt"},
		{Options{Query: "k"}, "fold.txt"},
		{Options{Query: "ω"}, "greek.txt"},
		{Options{Query: "--"}, "dash.txt"},
		{Options{Query: `^(needle)\nsecond`, Regex: true}, "normal.txt"},
		{Options{Query: `^\W`, Regex: true}, ""},
		{Options{Query: "needle\nsecond"}, "normal.txt"},
		{Options{Query: "needle", OnlyOpen: true}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.options.Query, func(t *testing.T) {
			t.Setenv("PATH", filepath.Dir(rg))
			if tc.rgMatch != "" {
				matcher, err := tc.options.Compile()
				if err != nil {
					t.Fatal(err)
				}
				var candidates []string
				backend := newRipgrepSearch(context.Background(), root, rg, matcher.expression.String())
				if backend == nil {
					t.Fatal("expected ripgrep to handle this query")
				}
				scope := func(path string) bool { return path == tc.rgMatch || path == ".ignore" }
				if _, err := backend.scanWorkspace(tc.options.IncludeIgnored, scope, nil, func(path string) error {
					candidates = append(candidates, path)
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				if len(candidates) != 1 || candidates[0] != tc.rgMatch {
					t.Fatalf("unexpected ripgrep candidates: %v", candidates)
				}
			}
			got, err := collectResults(context.Background(), root, tc.options, buffers)
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", t.TempDir())
			want, err := collectResults(context.Background(), root, tc.options, buffers)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("ripgrep result: %+v; built-in result: %+v, err=%v", got, want, err)
			}
		})
	}
}

func TestRipgrepFailureFallsBack(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires a POSIX shell")
	}
	root := t.TempDir()
	writeSearchFile(t, filepath.Join(root, "match.txt"), "needle")
	writeSearchFile(t, filepath.Join(root, "other.txt"), "needle")
	path := t.TempDir()
	t.Setenv("PATH", path)
	for _, script := range []string{
		"echo 'failed to read' >&2; exit 1",
		"printf '../unknown.txt\\000'",
		"printf 'match.txt\\000'; echo 'failed to read' >&2; exit 2",
	} {
		if err := os.WriteFile(filepath.Join(path, "rg"), []byte("#!/bin/sh\n"+script+"\n"), 0700); err != nil {
			t.Fatal(err)
		}
		result, err := collectResults(context.Background(), root, Options{Query: "needle", Word: true}, nil)
		if err != nil || result.Count != 2 || len(result.Files) != 2 || result.Unreadable != 0 {
			t.Fatalf("fallback result: %+v, err=%v", result, err)
		}
	}
}

func TestRipgrepRecursiveScope(t *testing.T) {
	rg, err := exec.LookPath("rg")
	if err != nil {
		t.Skip("ripgrep is unavailable")
	}
	parent := t.TempDir()
	root := filepath.Join(parent, "workspace")
	writeSearchFile(t, filepath.Join(parent, ".ignore"), "workspace/l/x.txt\n")
	for name, contents := range map[string]string{
		".gitignore":               "ignored/\nroot-hidden.rs\ndisk-hidden.rs\nignored.rs/\n",
		".ignore":                  "root.txt\n",
		".rgignore":                "!root.txt\n\\[literal\\]/\n",
		".git/private.txt":         "needle",
		".git/info/exclude":        "l/x.txt\n",
		"ignored/child.txt":        "needle",
		"[literal]/x.txt":          "needle",
		"l/x.txt":                  "needle",
		".hidden.txt":              "needle",
		"root.txt":                 "needle",
		"nested/.ignore":           "generated/\n",
		"nested/generated/out.txt": "needle",
		"nested/main.txt":          "needle",
		"folder.rs/inside.txt":     "needle",
		"nested/in.rs":             "needle",
		"ignored/deep.rs":          "needle",
		"root-hidden.rs":           "needle",
		"disk-hidden.rs":           "needle",
		"ignored.rs/deep.txt":      "needle",
	} {
		file := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatal(err)
		}
		writeSearchFile(t, file, contents)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	matcher, err := (Options{Query: "needle"}).Compile()
	if err != nil {
		t.Fatal(err)
	}
	backend := newRipgrepSearch(context.Background(), root, rg, matcher.expression.String())
	t.Run("include globs", func(t *testing.T) {
		t.Setenv("PATH", filepath.Dir(rg))
		options := Options{Query: "needle", Include: "**/*.{rs,md}"}
		buffers := map[string]string{filepath.Join(root, "root-hidden.rs"): "needle edited needle"}
		got, err := collectResults(context.Background(), root, options, buffers)
		if err != nil || len(got.Files) != 5 || got.Count != 6 {
			t.Fatalf("rg included the wrong files: %+v, err=%v", got, err)
		}
		t.Setenv("PATH", t.TempDir())
		want, err := collectResults(context.Background(), root, options, buffers)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("rg result %+v differs from built-in result %+v, err=%v", got, want, err)
		}
	})
	t.Setenv("PATH", t.TempDir())
	for _, includeIgnored := range []bool{false, true} {
		got, err := backend.scanWorkspace(includeIgnored, func(path string) bool { return path != ".hidden.txt" }, nil, func(string) error { return nil })
		want := map[string]struct{}{"l/x.txt": {}, "root.txt": {}, "nested/main.txt": {}, "folder.rs/inside.txt": {}, "nested/in.rs": {}}
		if includeIgnored {
			want["ignored/child.txt"] = struct{}{}
			want["ignored/deep.rs"] = struct{}{}
			want["root-hidden.rs"] = struct{}{}
			want["disk-hidden.rs"] = struct{}{}
			want["ignored.rs/deep.txt"] = struct{}{}
			want["[literal]/x.txt"] = struct{}{}
			want["nested/generated/out.txt"] = struct{}{}
		}
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("includeIgnored=%t: recursive paths %v, want %v, err=%v", includeIgnored, got, want, err)
		}
		result, err := collectResults(context.Background(), root, Options{Query: "needle", IncludeIgnored: includeIgnored},
			map[string]string{filepath.Join(root, ".hidden.txt"): "unsaved"})
		if err != nil || len(result.Files) != len(got) {
			t.Fatalf("includeIgnored=%t: fallback result %+v, err=%v", includeIgnored, result, err)
		}
		for _, file := range result.Files {
			relative, err := filepath.Rel(root, file.Path)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := got[filepath.ToSlash(relative)]; !ok {
				t.Fatalf("includeIgnored=%t: unexpected fallback path %q", includeIgnored, relative)
			}
		}
	}
}

func TestRipgrepSearchesOpenBuffersBeforeStartingDiskSearch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires a POSIX shell")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"open.go", "disk.go", "skip.txt"} {
		writeSearchFile(t, filepath.Join(root, name), "needle")
	}
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	calls := filepath.Join(bin, "calls")
	t.Setenv("VL_TEST_RG_CALLS", calls)
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" >> \"$VL_TEST_RG_CALLS\"\nprintf 'disk.go\\000open.go\\000skip.txt\\000'\n"
	if err := os.WriteFile(filepath.Join(bin, "rg"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	matcher, err := (Options{Query: "needle", Word: true, Exclude: "vendor, *.txt"}).Compile()
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	_, err = matcher.Scan(context.Background(), root, map[string]string{filepath.Join(root, "open.go"): "unsaved needle"}, func(file File) {
		order = append(order, filepath.Base(file.Path))
		if len(order) == 1 {
			if _, err := os.Stat(calls); !os.IsNotExist(err) {
				t.Fatalf("disk search had already started, stat: %v", err)
			}
		}
	})
	if err != nil || !reflect.DeepEqual(order, []string{"open.go", "disk.go"}) {
		t.Fatalf("results %v, err=%v", order, err)
	}
	data, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	args := string(data)
	if !strings.Contains(args, "--glob\n!**/vendor\n") || strings.Count(args, "--\n.\n") != 1 {
		t.Fatalf("expected one recursive call with the exclude glob, got %q", args)
	}
	if err := os.Remove(calls); err != nil {
		t.Fatal(err)
	}
	result, err := collectResults(context.Background(), root, Options{Query: "needle", Word: true, Include: "*.go"}, nil)
	if err != nil || result.Count != 2 {
		t.Fatalf("include-filtered result %+v, err=%v", result, err)
	}
	data, err = os.ReadFile(calls)
	if err != nil || !strings.Contains(string(data), "--glob\n**/*.go\n--glob\n**/*.go/**\n") {
		t.Fatalf("include filter was not passed to ripgrep: args=%q, err=%v", data, err)
	}
	if err := os.Remove(calls); err != nil {
		t.Fatal(err)
	}
	result, err = collectResults(context.Background(), root, Options{Query: "needle", Include: "*.go, [s]kip.txt"}, nil)
	if err != nil || result.Count != 3 {
		t.Fatalf("mixed include filters: result %+v, err=%v", result, err)
	}
	if _, err := os.Stat(calls); !os.IsNotExist(err) {
		t.Fatalf("untranslatable include started an incomplete ripgrep search: %v", err)
	}
}

func TestRipgrepCancellationStopsStreamingWithoutFallback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires a POSIX shell")
	}
	root := t.TempDir()
	writeSearchFile(t, filepath.Join(root, "first.txt"), "needle")
	writeSearchFile(t, filepath.Join(root, "second.txt"), "needle")
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	script := "#!/bin/sh\nprintf 'first.txt\\000'\nexec /bin/sleep 30\n"
	if err := os.WriteFile(filepath.Join(bin, "rg"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	matcher, err := (Options{Query: "needle", Word: true}).Compile()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var got []string
	_, err = matcher.Scan(ctx, root, nil, func(file File) {
		got = append(got, filepath.Base(file.Path))
		cancel()
	})
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(got, []string{"first.txt"}) {
		t.Fatalf("canceled search returned %q, err=%v", got, err)
	}
}
