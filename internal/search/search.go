// Package search implements workspace text search using repository-local ignore rules.
package search

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	gitservice "github.com/ourongxing/vl/internal/git"
)

type Options struct {
	Query                 string
	Case, Word, Regex     bool
	IncludeIgnored        bool
	PreserveCase          bool
	Include, Exclude      string
	OnlyOpen, ChangedOnly bool
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
			next = string(m.expression.ExpandString(nil, normalizeReplacement(replacement, m.expression.NumSubexp()), text, loc))
		}
		if m.options.PreserveCase {
			next = preserveCase(text[a:b], next)
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
	includes, err := compileGlobs(o.Include)
	if err != nil {
		return result, fmt.Errorf("include: %w", err)
	}
	excludes, err := compileGlobs(o.Exclude)
	if err != nil {
		return result, fmt.Errorf("exclude: %w", err)
	}
	if o.Query == "" {
		return result, nil
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	root, err = filepath.Abs(root)
	if err == nil {
		root, err = filepath.EvalSymlinks(root)
	}
	if err != nil {
		return result, fmt.Errorf("resolve search workspace: %w", err)
	}
	paths := map[string]bool{}
	repo := &gitservice.Repository{Root: root}
	fileIgnores := ignoreFiles{
		root:    root,
		folders: map[string]bool{".": !repo.HasRepository()},
		cache:   map[string]directoryIgnores{},
	}
	var changed map[string]bool
	if o.ChangedOnly {
		changed = map[string]bool{}
	}
	if !o.OnlyOpen || o.ChangedOnly {
		listing, err := repo.ListWorkspaceFiles(ctx, o.IncludeIgnored)
		if err != nil {
			return result, err
		}
		for _, dir := range listing.OrdinaryFolders {
			fileIgnores.folders[dir] = true
		}
		if !o.OnlyOpen {
			for _, path := range listing.Paths {
				paths[filepath.Join(root, path)] = true
			}
		}
		for _, owner := range listing.Repositories {
			if !o.ChangedOnly {
				break
			}
			raw, err := owner.Run(ctx, "status", "--porcelain=v1", "-z", "--untracked-files=all")
			if err != nil {
				return result, err
			}
			entries, err := gitservice.ParseStatus(raw)
			if err != nil {
				return result, err
			}
			for _, entry := range entries {
				changed[filepath.Join(owner.Root, entry.Path)] = true
			}
		}
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
		ignored, err = repo.Ignored(ctx, relative)
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
		relative, _ := filepath.Rel(root, path)
		relative = filepath.ToSlash(relative)
		if !o.IncludeIgnored {
			ignored, err := fileIgnores.ignored(relative)
			if err != nil {
				return result, err
			}
			if ignored {
				continue
			}
		}
		if len(includes) > 0 && !matchesGlobs(includes, relative) || matchesGlobs(excludes, relative) {
			continue
		}
		if o.ChangedOnly && !changed[path] {
			text, open := buffers[path]
			if !open {
				continue
			}
			data, e := os.ReadFile(path)
			normalized := strings.TrimPrefix(strings.ReplaceAll(string(data), "\r\n", "\n"), "\xef\xbb\xbf")
			if e == nil && normalized == text {
				continue
			}
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

func metadataPath(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == ".git" {
			return true
		}
	}
	return false
}

// ReplaceText applies an ordered, non-overlapping match snapshot without writing to disk.
func ReplaceText(text string, matches []Match) string {
	var b strings.Builder
	pos := 0
	for _, hit := range matches {
		b.WriteString(text[pos:hit.Start])
		b.WriteString(hit.Replacement)
		pos = hit.End
	}
	b.WriteString(text[pos:])
	return b.String()
}
