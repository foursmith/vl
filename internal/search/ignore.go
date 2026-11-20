package search

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MoeForge/moego/regexp"
)

type ignoreRule struct {
	base              string
	pattern           *regexp.Regexp
	negate, directory bool
}
type ignoreFiles struct {
	root    string
	folders map[string]bool
	cache   map[string]directoryIgnores
}

type directoryIgnores struct {
	rules    []ignoreRule
	ordinary bool
}

func (f *ignoreFiles) rules(dir string) (directoryIgnores, error) {
	if scope, ok := f.cache[dir]; ok {
		return scope, nil
	}
	scope := directoryIgnores{ordinary: f.folders[dir]}
	if dir != "." {
		parent, err := f.rules(filepath.Dir(dir))
		if err != nil {
			return directoryIgnores{}, err
		}
		scope.rules = append(scope.rules, parent.rules...)
		scope.ordinary = scope.ordinary || parent.ordinary
	}
	names := []string{".ignore"}
	if scope.ordinary {
		names = []string{".gitignore", ".ignore"}
	}
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(f.root, dir, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return directoryIgnores{}, err
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimRight(line, "\r ")
			if line == "" || line[0] == '#' {
				continue
			}
			negate := strings.HasPrefix(line, "!")
			if negate {
				line = line[1:]
			}
			if strings.HasPrefix(line, "\\#") || strings.HasPrefix(line, "\\!") {
				line = line[1:]
			}
			directory := strings.HasSuffix(line, "/")
			line = strings.TrimSuffix(line, "/")
			anchored := strings.HasPrefix(line, "/") || strings.Contains(line, "/")
			line = strings.TrimPrefix(line, "/")
			if line == "" {
				continue
			}
			body, err := globExpression(line)
			if err != nil {
				return directoryIgnores{}, fmt.Errorf("%s: %w", filepath.Join(dir, name), err)
			}
			prefix := "(?:^|.*/)"
			if anchored {
				prefix = "^"
			}
			re, err := regexp.Compile(prefix + body + "(?:/.*)?$")
			if err != nil {
				return directoryIgnores{}, err
			}
			base := filepath.ToSlash(dir)
			if base == "." {
				base = ""
			}
			scope.rules = append(scope.rules, ignoreRule{base: base, pattern: re, negate: negate, directory: directory})
		}
	}
	f.cache[dir] = scope
	return scope, nil
}
func (f *ignoreFiles) ignored(relative string) (bool, error) {
	scope, err := f.rules(filepath.Dir(relative))
	if err != nil {
		return false, err
	}
	parts := strings.Split(filepath.ToSlash(relative), "/")
	// A child cannot be re-included while its parent directory remains ignored.
	for i := range parts {
		path := strings.Join(parts[:i+1], "/")
		ignored := false
		for _, rule := range scope.rules {
			local := path
			if rule.base != "" {
				prefix := rule.base + "/"
				if !strings.HasPrefix(path, prefix) {
					continue
				}
				local = strings.TrimPrefix(path, prefix)
			}
			if rule.directory && i == len(parts)-1 {
				continue
			}
			if rule.pattern.MatchString(local) {
				ignored = !rule.negate
			}
		}
		if ignored {
			return true, nil
		}
	}
	return false, nil
}
