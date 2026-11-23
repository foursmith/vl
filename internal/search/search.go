// Package search implements workspace text search using repository-local ignore rules.
package search

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/MoeForge/moego/regexp"
	gitservice "github.com/foursmith/vl/internal/git"
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
	rawLiteral bool
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
	// Removing a BOM and normalizing CRLF cannot create a new occurrence of
	// a literal without line feeds, so misses can skip text decoding.
	return &Matcher{options: o, expression: expression, rawLiteral: !o.Regex && !strings.Contains(o.Query, "\n")}, nil
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
		if m.options.Word {
			before, _ := utf8.DecodeLastRuneInString(text[:a])
			after, _ := utf8.DecodeRuneInString(text[b:])
			if word(before) || word(after) {
				continue
			}
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

// Run collects all matches in path order.
func Run(ctx context.Context, root string, o Options, buffers map[string]string) (Result, error) {
	var result Result
	matcher, err := o.Compile()
	if err != nil {
		return result, err
	}
	unreadable, err := matcher.Scan(ctx, root, buffers, func(file File) {
		result.Count += len(file.Matches)
		result.Files = append(result.Files, file)
	})
	result.Unreadable = unreadable
	sort.Slice(result.Files, func(i, j int) bool { return result.Files[i].Path < result.Files[j].Path })
	return result, err
}

// Scan reports complete file results as they are found. Open editor snapshots
// take precedence over disk.
func (m *Matcher) Scan(ctx context.Context, root string, buffers map[string]string, found func(File)) (int, error) {
	o := m.options
	unreadable := 0
	includes, err := compileGlobs(o.Include)
	if err != nil {
		return unreadable, fmt.Errorf("include: %w", err)
	}
	excludes, err := compileGlobs(o.Exclude)
	if err != nil {
		return unreadable, fmt.Errorf("exclude: %w", err)
	}
	if o.Query == "" {
		return unreadable, nil
	}
	if err := ctx.Err(); err != nil {
		return unreadable, err
	}
	root, err = filepath.Abs(root)
	if err == nil {
		root, err = filepath.EvalSymlinks(root)
	}
	if err != nil {
		return unreadable, fmt.Errorf("resolve search workspace: %w", err)
	}
	pending := make(map[string]string, len(buffers))
	for file, text := range buffers {
		rel, err := filepath.Rel(root, file)
		if err == nil && filepath.IsLocal(rel) && rel != "." && !metadataPath(rel) {
			pending[filepath.ToSlash(rel)] = text
		}
	}
	var changed map[string]bool
	if o.ChangedOnly {
		changed, err = changedSearchFiles(ctx, root)
		if err != nil {
			return unreadable, err
		}
	}
	var scratch bytes.Buffer
	scan := func(relative, text string, open bool) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if len(includes) > 0 && !matchesGlobs(includes, relative) || matchesGlobs(excludes, relative) {
			return nil
		}
		file := filepath.Join(root, relative)
		if o.ChangedOnly && !changed[file] {
			if !open {
				return nil
			}
			data, err := readRegularFile(file, &scratch)
			if err == nil && data != nil && normalizeText(data) == text {
				return nil
			}
		}
		if !open {
			data, err := readRegularFile(file, &scratch)
			if err != nil {
				if !os.IsNotExist(err) {
					unreadable++
				}
				return nil
			}
			if data == nil || (m.rawLiteral && !m.expression.Match(data)) ||
				!utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
				return nil
			}
			text = normalizeText(data)
		}
		if matches := m.Find(text, ""); len(matches) != 0 {
			found(File{file, text, matches})
		}
		return nil
	}
	if !o.OnlyOpen {
		err = walkFilePaths(ctx, root, o.IncludeIgnored, func(relative string, regular bool) error {
			text, open := pending[relative]
			if open {
				delete(pending, relative)
			}
			if regular || open {
				return scan(relative, text, open)
			}
			return nil
		})
		if err != nil {
			return unreadable, err
		}
	}
	ignores := ignoreFiles{root: root, cache: map[string]directoryIgnores{}}
	for relative, text := range pending {
		if !o.IncludeIgnored {
			ignored, err := ignores.ignored(relative)
			if err != nil {
				return unreadable, err
			}
			if ignored {
				continue
			}
		}
		if err := scan(relative, text, true); err != nil {
			return unreadable, err
		}
	}
	return unreadable, ctx.Err()
}

func changedSearchFiles(ctx context.Context, root string) (map[string]bool, error) {
	repo := &gitservice.Repository{Root: root}
	changed := map[string]bool{}
	repositories, err := repo.WorkspaceRepositories(ctx)
	if err != nil {
		return nil, err
	}
	for _, owner := range repositories {
		raw, err := owner.Run(ctx, "status", "--porcelain=v1", "-z", "--untracked-files=all")
		if err != nil {
			return nil, err
		}
		entries, err := gitservice.ParseStatus(raw)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			changed[filepath.Join(owner.Root, entry.Path)] = true
		}
	}
	return changed, nil
}

func metadataPath(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if strings.EqualFold(part, ".git") {
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
