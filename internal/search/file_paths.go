package search

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/MoeForge/moego/regexp"
)

type pathIgnoreRule struct {
	base, literal, suffix     string
	pattern                   *regexp.Regexp
	anchored, directory, keep bool
}

// ListFilePaths discovers local files for Go to File. Its ignore scope is the
// workspace directory and the .gitignore and .ignore files below it.
func ListFilePaths(ctx context.Context, root string, includeIgnored bool) ([]string, error) {
	var paths []string
	err := walkFilePaths(ctx, root, includeIgnored, func(path string, _ bool) error {
		paths = append(paths, path)
		return nil
	})
	return paths, err
}

var localIgnoreNames = [...]string{".gitignore", ".ignore"}

// walkFilePaths visits paths in directory order. The second argument reports
// whether the entry is a regular file, so content searches can skip symlinks.
func walkFilePaths(ctx context.Context, root string, includeIgnored bool, found func(string, bool) error) error {
	var ignoreFiles []string
	if !includeIgnored {
		ignoreFiles = localIgnoreNames[:]
	}
	var visit func(string, []pathIgnoreRule) error
	visit = func(dir string, inherited []pathIgnoreRule) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		entries, err := os.ReadDir(filepath.Join(root, dir))
		if err != nil {
			return err
		}
		prefix := dir
		if prefix != "" {
			prefix += "/"
		}
		rules := inherited
		for _, name := range ignoreFiles {
			index, found := slices.BinarySearchFunc(entries, name, func(entry os.DirEntry, name string) int {
				return strings.Compare(entry.Name(), name)
			})
			if !found || !entries[index].Type().IsRegular() {
				continue
			}
			rules, err = appendPathIgnoreRules(filepath.Join(root, dir, name), prefix, rules)
			if err != nil {
				return err
			}
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}
			name := entry.Name()
			path := prefix + name
			if strings.EqualFold(name, ".git") || ignoredFilePath(rules, path, name, entry.IsDir()) {
				continue
			}
			if entry.IsDir() {
				if err := visit(path, rules[:len(rules):len(rules)]); err != nil {
					return err
				}
			} else if regular := entry.Type().IsRegular(); regular || entry.Type()&os.ModeSymlink != 0 {
				if err := found(path, regular); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return visit("", nil)
}

func appendPathIgnoreRules(path, prefix string, rules []pathIgnoreRule) ([]pathIgnoreRule, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return rules, nil
	}
	if err != nil {
		return nil, err
	}
	for raw := range strings.SplitSeq(string(data), "\n") {
		if rule, ok := compilePathIgnoreRule(prefix, raw); ok {
			rules = append(rules, rule)
		}
	}
	return rules, nil
}

func compilePathIgnoreRule(base, raw string) (pathIgnoreRule, bool) {
	line := strings.TrimSuffix(raw, "\r")
	for len(line) > 0 && line[len(line)-1] == ' ' {
		i := len(line) - 1
		for i > 0 && line[i-1] == '\\' {
			i--
		}
		if (len(line)-1-i)%2 != 0 {
			break
		}
		line = line[:len(line)-1]
	}
	if line == "" || line[0] == '#' {
		return pathIgnoreRule{}, false
	}
	line, keep := strings.CutPrefix(line, "!")
	line, directory := strings.CutSuffix(line, "/")
	rule := pathIgnoreRule{base: base, anchored: strings.Contains(line, "/"), directory: directory, keep: keep}
	line = strings.TrimPrefix(line, "/")
	if line == "" {
		return pathIgnoreRule{}, false
	}
	if !strings.ContainsAny(line, "*?[\\") {
		rule.literal = line
		return rule, true
	}
	if !rule.anchored && strings.HasPrefix(line, "*") && !strings.ContainsAny(line[1:], "*?[\\") {
		rule.suffix = line[1:]
		if rule.suffix != "" {
			return rule, true
		}
	}
	body, err := pathGlobExpression(line, true)
	if err != nil {
		return pathIgnoreRule{}, false
	}
	rule.pattern, err = regexp.Compile("(?s)^(?:" + body + ")$")
	return rule, err == nil
}

func ignoredFilePath(rules []pathIgnoreRule, path, name string, directory bool) bool {
	// Last matching rule wins. The walk prunes excluded directories before
	// looking at their children, so a negation cannot reopen an excluded parent.
	for i := len(rules) - 1; i >= 0; i-- {
		rule := rules[i]
		if rule.directory && !directory {
			continue
		}
		value := name
		if rule.anchored {
			value = path[len(rule.base):] // inherited rules are scoped to ancestors
		}
		switch {
		case rule.literal != "":
			if value == rule.literal {
				return !rule.keep
			}
		case rule.suffix != "":
			if strings.HasSuffix(value, rule.suffix) {
				return !rule.keep
			}
		case rule.pattern.MatchString(value):
			return !rule.keep
		}
	}
	return false
}
