package search

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"regexp/syntax"
	"strings"
	"unicode"
	"unicode/utf8"
)

type ripgrepSearch struct {
	ctx              context.Context
	root, executable string
	args             []string
}

func newRipgrepSearch(ctx context.Context, root, executable, pattern string) *ripgrepSearch {
	expression, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return nil
	}
	runes, fold := requiredLiteral(expression)
	if len(runes) == 0 {
		return nil
	}
	if len(runes) > 64 {
		runes = runes[:64]
	}
	args := []string{
		"--no-config", "--encoding", "none", "--line-buffered",
		// Limit filesystem contention when searching large trees.
		"--threads", "2",
	}
	if !fold || isASCII(runes) {
		args = append(args, "--fixed-strings")
		if fold {
			args = append(args, "--ignore-case")
		}
		args = append(args, "-e", string(runes))
	} else {
		// Use Go's Unicode fold sets even if ripgrep uses a different
		// Unicode version. Each rune becomes a character class.
		var query strings.Builder
		for _, r := range runes {
			fmt.Fprintf(&query, `[\x{%x}`, r)
			for next := unicode.SimpleFold(r); next != r; next = unicode.SimpleFold(next) {
				fmt.Fprintf(&query, `\x{%x}`, next)
			}
			query.WriteByte(']')
		}
		args = append(args, "-e", query.String())
	}
	return &ripgrepSearch{ctx: ctx, root: root, executable: executable, args: append(args, "--")}
}

func requiredLiteral(expression *syntax.Regexp) ([]rune, bool) {
	switch expression.Op {
	case syntax.OpLiteral:
		// The disk is scanned before CRLF normalization. A literal on
		// either side of a newline is still required in the raw file.
		var longest []rune
		start := 0
		for i, r := range expression.Rune {
			if r == '\n' || r == 0 {
				if i-start > len(longest) {
					longest = expression.Rune[start:i]
				}
				start = i + 1
			}
		}
		if tail := expression.Rune[start:]; len(tail) > len(longest) {
			longest = tail
		}
		return longest, expression.Flags&syntax.FoldCase != 0
	case syntax.OpCapture, syntax.OpPlus:
		return requiredLiteral(expression.Sub[0])
	case syntax.OpRepeat:
		if expression.Min > 0 {
			return requiredLiteral(expression.Sub[0])
		}
	case syntax.OpConcat:
		var longest []rune
		var fold bool
		for _, sub := range expression.Sub {
			if literal, folded := requiredLiteral(sub); len(literal) > len(longest) {
				longest, fold = literal, folded
			}
		}
		return longest, fold
	}
	return nil, false
}

func isASCII(runes []rune) bool {
	for _, r := range runes {
		if r >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// Let ripgrep apply workspace-local ignores during its directory walk.
func (rg *ripgrepSearch) workspaceArgs(includeIgnored bool, globs []string, outputFlags ...string) []string {
	args := []string{"--hidden"}
	if includeIgnored {
		args = append(args, "--no-ignore")
	} else {
		args = append(args, "--no-require-git", "--no-ignore-parent", "--no-ignore-global", "--no-ignore-exclude")
	}
	for _, pattern := range globs {
		args = append(args, "--glob", pattern)
	}
	args = append(args, "--iglob", "!.git")
	args = append(args, outputFlags...)
	return append(append(args, rg.args...), ".")
}

func (rg *ripgrepSearch) scanWorkspace(includeIgnored bool, inScope func(string) bool, globs []string, visit func(string) error) (map[string]struct{}, error) {
	args := rg.workspaceArgs(includeIgnored, globs, "--files-with-matches", "--null")
	scanned := make(map[string]struct{})
	err := rg.streamPaths(args, func(relative string) error {
		if !filepath.IsLocal(relative) || relative == "." || filepath.ToSlash(filepath.Clean(relative)) != relative {
			return fmt.Errorf("unexpected ripgrep workspace path %q", relative)
		}
		if !inScope(relative) {
			return nil
		}
		if _, duplicate := scanned[relative]; duplicate {
			return nil
		}
		if err := visit(relative); err != nil {
			return err
		}
		scanned[relative] = struct{}{}
		return nil
	})
	return scanned, err
}

func (rg *ripgrepSearch) streamPaths(args []string, visit func(string) error) error {
	gotPaths := false
	err := rg.stream(args, func(stdout io.Reader) error {
		return readRipgrepPaths(stdout, func(relative string) error {
			gotPaths = true
			return visit(relative)
		})
	})
	if !gotPaths && err == nil {
		return errors.New("ripgrep reported success without matching paths")
	}
	if !gotPaths && ripgrepNoMatches(err) {
		return nil
	}
	return err
}

func ripgrepNoMatches(err error) bool {
	var exit *exec.ExitError
	return errors.As(err, &exit) && exit.ExitCode() == 1
}

func (rg *ripgrepSearch) stream(args []string, read func(io.Reader) error) error {
	ctx, cancel := context.WithCancel(rg.ctx)
	defer cancel()
	cmd := exec.CommandContext(ctx, rg.executable, args...)
	cmd.Dir = rg.root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	parseErr := read(stdout)
	if parseErr != nil {
		cancel()
	}
	waitErr := cmd.Wait()
	if parseErr != nil {
		return parseErr
	}
	if stderr.Len() != 0 {
		return errors.New(stderr.String())
	}
	return waitErr
}

func readRipgrepPaths(stdout io.Reader, visit func(string) error) error {
	reader := bufio.NewReader(stdout)
	for {
		path, err := reader.ReadString(0)
		if err == io.EOF && path == "" {
			return nil
		}
		if err != nil {
			return err
		}
		relative := strings.TrimPrefix(filepath.ToSlash(path[:len(path)-1]), "./")
		if err := visit(relative); err != nil {
			return err
		}
	}
}
