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
// workspace directory and the .gitignore, .ignore and .rgignore files below it.
func ListFilePaths(ctx context.Context, root string, includeIgnored bool) ([]string, error) {
	var paths []string
	err := walkFilePaths(ctx, root, includeIgnored, nil, func(path string, _ bool) error {
		paths = append(paths, path)
		return nil
	})
	return paths, err
}

// Like ripgrep, prefer .rgignore over .ignore over .gitignore, even
// when the higher-priority rule is in a parent directory.
var localIgnoreNames = [...]string{".gitignore", ".ignore", ".rgignore"}

type pathIgnoreRules [len(localIgnoreNames)][]pathIgnoreRule

// walkFilePaths visits paths in directory order. The second argument reports
// whether the entry is a regular file, so content searches can skip symlinks.
func walkFilePaths(ctx context.Context, root string, includeIgnored bool, include func(string) bool, found func(string, bool) error) error {
	var ignoreFiles []string
	if !includeIgnored {
		ignoreFiles = localIgnoreNames[:]
	}
	var visit func(string, pathIgnoreRules) error
	visit = func(dir string, rules pathIgnoreRules) error {
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
		for kind, name := range ignoreFiles {
			index, found := slices.BinarySearchFunc(entries, name, func(entry os.DirEntry, name string) int {
				return strings.Compare(entry.Name(), name)
			})
			if !found || !entries[index].Type().IsRegular() {
				continue
			}
			err = rules.load(kind, filepath.Join(root, dir, name), prefix)
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
			if strings.EqualFold(name, ".git") ||
				rules.ignored(path, name, entry.IsDir()) && (include == nil || !include(path)) {
				continue
			}
			if entry.IsDir() {
				if err := visit(path, rules); err != nil {
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
	return visit("", pathIgnoreRules{})
}

func (rules *pathIgnoreRules) load(kind int, path, prefix string) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	inherited := rules[kind]
	group := inherited[:len(inherited):len(inherited)]
	for raw := range strings.SplitSeq(strings.TrimPrefix(string(data), "\uFEFF"), "\n") {
		if rule, ok := compilePathIgnoreRule(prefix, raw); ok {
			group = append(group, rule)
		}
	}
	rules[kind] = group
	return nil
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

func (rules pathIgnoreRules) ignored(path, name string, directory bool) bool {
	for kind := len(rules) - 1; kind >= 0; kind-- {
		if rule := lastMatchingIgnoreRule(rules[kind], path, name, directory); rule != nil {
			return !rule.keep
		}
	}
	return false
}

// Within one ignore file type, closer directories and later lines win.
// The walker prunes an excluded directory before inspecting its children.
func lastMatchingIgnoreRule(rules []pathIgnoreRule, path, name string, directory bool) *pathIgnoreRule {
	for i := len(rules) - 1; i >= 0; i-- {
		rule := &rules[i]
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
				return rule
			}
		case rule.suffix != "":
			if strings.HasSuffix(value, rule.suffix) {
				return rule
			}
		case rule.pattern.MatchString(value):
			return rule
		}
	}
	return nil
}
