// Package search implements workspace text search using repository-local ignore rules.
package search

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	gitservice "github.com/ourongxing/elegit/internal/git"
)

type Options struct {
	Query             string
	Case, Word, Regex bool
	IncludeIgnored    bool
}
type Match struct {
	Start, End, Line int
	Replacement      string
}
type File struct {
	Path, Text string
	Matches    []Match
}
type Result struct {
	Files      []File
	Count      int
	Unreadable int
}

type Matcher struct {
	options    Options
	expression *regexp.Regexp
}

func (o Options) Compile() (*Matcher, error) {
	pattern := o.Query
	if !o.Regex {
		pattern = regexp.QuoteMeta(pattern)
	} else {
		pattern = "(?m)" + pattern
	}
	if !o.Case {
		pattern = "(?i)" + pattern
	}
	expression, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return &Matcher{options: o, expression: expression}, nil
}
func word(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }
func (m *Matcher) Find(text, replacement string) []Match {
	if m.options.Query == "" {
		return nil
	}
	var result []Match
	// Include submatches for $1 / ${name} expansion.
	line, scanned := 1, 0
	for _, loc := range m.expression.FindAllStringSubmatchIndex(text, -1) {
		a, b := loc[0], loc[1]
		before, _ := utf8.DecodeLastRuneInString(text[:a])
		after, _ := utf8.DecodeRuneInString(text[b:])
		if m.options.Word && ((a > 0 && word(before)) || (b < len(text) && word(after))) {
			continue
		}
		segment := text[scanned:a]
		line += strings.Count(segment, "\n")
		scanned = a
		next := replacement
		if m.options.Regex {
			next = string(m.expression.ExpandString(nil, replacement, text, loc))
		}
		result = append(result, Match{Start: a, End: b, Line: line, Replacement: next})
	}
	return result
}

// Run reads only regular workspace files. Open editor snapshots take precedence over disk.
func Run(ctx context.Context, root string, o Options, buffers map[string]string) (Result, error) {
	var result Result
	matcher, err := o.Compile()
	if err != nil {
		return result, err
	}
	if o.Query == "" {
		return result, nil
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return result, fmt.Errorf("resolve search workspace: %w", err)
	}
	paths := map[string]bool{}
	if err := listFiles(ctx, root, o.IncludeIgnored, paths, map[string]bool{}); err != nil {
		return result, err
	}
	// Buffers override disk contents without bypassing the selected ignore scope.
	extra := map[string]string{}
	for p := range buffers {
		rel, e := filepath.Rel(root, p)
		if e == nil && filepath.IsLocal(rel) && !metadataPath(rel) && !paths[p] {
			extra[filepath.ToSlash(rel)] = p
		}
	}
	ignored := map[string]bool{}
	if !o.IncludeIgnored && len(extra) > 0 {
		relative := make([]string, 0, len(extra))
		for rel := range extra {
			relative = append(relative, rel)
		}
		var err error
		ignored, err = (&gitservice.Repository{Root: root}).Ignored(ctx, relative)
		if err != nil {
			return result, fmt.Errorf("check search buffer ignore rules: %w", err)
		}
	}
	for rel, p := range extra {
		if !ignored[rel] {
			paths[p] = true
		}
	}
	ordered := make([]string, 0, len(paths))
	for p := range paths {
		ordered = append(ordered, p)
	}
	sort.Strings(ordered)
	for _, path := range ordered {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		text, ok := buffers[path]
		if !ok {
			real, e := filepath.EvalSymlinks(path)
			if os.IsNotExist(e) {
				continue
			}
			if e != nil {
				result.Unreadable++
				continue
			}
			if real != filepath.Clean(path) {
				continue
			}
			info, e := os.Lstat(path)
			if e != nil {
				result.Unreadable++
				continue
			}
			if !info.Mode().IsRegular() {
				continue
			}
			data, e := os.ReadFile(path)
			if e != nil {
				result.Unreadable++
				continue
			}
			if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
				continue
			}
			text = strings.TrimPrefix(strings.ReplaceAll(string(data), "\r\n", "\n"), "\xef\xbb\xbf")
		}
		matches := matcher.Find(text, "")
		if len(matches) == 0 {
			continue
		}
		result.Files = append(result.Files, File{path, text, matches})
		result.Count += len(matches)
	}
	return result, nil
}

// Git reports initialized submodules and embedded repositories as directories.
// List each repository separately so its own ignore rules and untracked files apply.
func listFiles(ctx context.Context, root string, includeIgnored bool, paths, seen map[string]bool) error {
	if seen[root] {
		return nil
	}
	seen[root] = true
	args := []string{"-C", root, "ls-files", "-z", "--cached", "--others"}
	if !includeIgnored {
		args = append(args, "--exclude-standard")
	}
	data, err := exec.CommandContext(ctx, "git", args...).Output()
	if err != nil {
		return fmt.Errorf("list search files in %s: %w", root, err)
	}
	for _, p := range strings.Split(string(data), "\x00") {
		if p == "" || !filepath.IsLocal(p) || metadataPath(p) {
			continue
		}
		path := filepath.Clean(filepath.Join(root, p))
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if info.IsDir() {
			real, err := filepath.EvalSymlinks(path)
			if err != nil {
				return err
			}
			if real != path {
				continue
			}
			if _, err := os.Lstat(filepath.Join(path, ".git")); err == nil {
				if err := listFiles(ctx, path, includeIgnored, paths, seen); err != nil {
					return err
				}
			}
		} else {
			paths[path] = true
		}
	}
	return ctx.Err()
}
func metadataPath(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == ".git" {
			return true
		}
	}
	return false
}
