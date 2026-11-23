// Package search implements workspace text search using repository-local ignore rules.
package search

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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
	preview    *filePreview
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

// Scan streams results from open buffers and disk. Simple disk matches can
// carry only the matching lines until the document is opened.
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
	openBuffers := make(map[string]string, len(buffers))
	for file, text := range buffers {
		rel, err := filepath.Rel(root, file)
		if err == nil && filepath.IsLocal(rel) && rel != "." && !metadataPath(rel) {
			openBuffers[filepath.ToSlash(rel)] = text
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
	inScope := func(relative string) bool {
		return (len(includes.patterns) == 0 || matchesGlobs(includes, relative)) && !matchesGlobs(excludes, relative)
	}
	emit := func(file, text string) {
		if matches := m.Find(text, ""); len(matches) != 0 {
			found(File{Path: file, Text: text, Matches: matches})
		}
	}
	scanDisk := func(relative string, prefilter bool) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		file := filepath.Join(root, relative)
		data, err := readRegularFile(file, &scratch)
		if err != nil {
			if !os.IsNotExist(err) {
				unreadable++
			}
			return nil
		}
		if data == nil || (prefilter && m.rawLiteral && !m.expression.Match(data)) ||
			!utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
			return nil
		}
		emit(file, normalizeText(data))
		return nil
	}
	var includePath func(string) bool
	if len(includes.patterns) != 0 {
		includePath = func(relative string) bool { return matchesGlobs(includes, relative) }
	}
	ignores := ignoreFiles{root: root, cache: map[string]directoryIgnores{}, include: includePath}
	ignoredPath := func(relative string) (bool, error) {
		if o.IncludeIgnored {
			return false, nil
		}
		return ignores.ignored(relative)
	}
	for relative, text := range openBuffers {
		if !inScope(relative) {
			continue
		}
		ignored, err := ignoredPath(relative)
		if err != nil {
			return unreadable, err
		}
		if ignored {
			continue
		}
		if err := ctx.Err(); err != nil {
			return unreadable, err
		}
		file := filepath.Join(root, relative)
		if o.ChangedOnly && !changed[relative] {
			data, err := readRegularFile(file, &scratch)
			if err == nil && data != nil && normalizeText(data) == text {
				continue
			}
		}
		emit(file, text)
	}
	if o.OnlyOpen {
		return unreadable, ctx.Err()
	}
	if o.ChangedOnly {
		paths := make([]string, 0, len(changed))
		for relative := range changed {
			paths = append(paths, relative)
		}
		slices.Sort(paths)
		for _, relative := range paths {
			if _, open := openBuffers[relative]; open || !inScope(relative) {
				continue
			}
			ignored, err := ignoredPath(relative)
			if err != nil {
				return unreadable, err
			}
			if ignored {
				continue
			}
			if err := scanDisk(relative, true); err != nil {
				return unreadable, err
			}
		}
		return unreadable, ctx.Err()
	}
	var verified map[string]struct{}
	// An untranslated include could have matches the other globs miss.
	if len(includes.ripgrepGlobs) == len(includes.patterns) {
		if executable, err := exec.LookPath("rg"); err == nil {
			var globs []string
			for _, pattern := range includes.ripgrepGlobs {
				globs = append(globs, pattern, pattern+"/**")
			}
			for _, pattern := range excludes.ripgrepGlobs {
				globs = append(globs, "!"+pattern)
			}
			scanCandidate := func(relative string) error {
				return scanDisk(relative, false)
			}
			diskInScope := func(relative string) bool {
				_, open := openBuffers[relative]
				return !open && !metadataPath(relative) && inScope(relative)
			}
			if rg := newRipgrepSearch(ctx, root, executable, m.expression.String()); rg != nil {
				if m.canUseRipgrepPositions() {
					verified, err = rg.scanJSONWorkspace(o.IncludeIgnored, diskInScope, globs, func(relative string, lines []ripgrepLine) error {
						if err := ctx.Err(); err != nil {
							return err
						}
						path := filepath.Join(root, relative)
						info, err := os.Lstat(path)
						if err != nil {
							if !os.IsNotExist(err) {
								unreadable++
							}
							return nil
						}
						if !info.Mode().IsRegular() {
							return nil
						}
						if file, ok := newFilePreview(path, info, lines); ok {
							found(file)
							return nil
						}
						return scanCandidate(relative)
					})
				} else {
					verified, err = rg.scanWorkspace(o.IncludeIgnored, diskInScope, globs, scanCandidate)
				}
				if err == nil || ctx.Err() != nil {
					return unreadable, ctx.Err()
				}
			}
		}
	}
	err = walkFilePaths(ctx, root, o.IncludeIgnored, includePath, func(relative string, regular bool) error {
		if !regular || !inScope(relative) {
			return nil
		}
		if _, open := openBuffers[relative]; open {
			return nil
		}
		if _, scanned := verified[relative]; scanned {
			return nil
		}
		return scanDisk(relative, true)
	})
	if err != nil {
		return unreadable, err
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
			relative, err := filepath.Rel(root, filepath.Join(owner.Root, entry.Path))
			if err == nil && filepath.IsLocal(relative) && relative != "." && !metadataPath(relative) {
				changed[filepath.ToSlash(relative)] = true
			}
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
