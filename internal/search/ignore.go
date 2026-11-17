package search

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type ignoreRule struct {
	base              string
	pattern           *regexp.Regexp
	negate, directory bool
}
type ignoreFiles struct {
	root     string
	ordinary bool
	cache    map[string][]ignoreRule
}

func (f *ignoreFiles) rules(dir string) ([]ignoreRule, error) {
	if rules, ok := f.cache[dir]; ok {
		return rules, nil
	}
	var rules []ignoreRule
	if dir != "." {
		parent, err := f.rules(filepath.Dir(dir))
		if err != nil {
			return nil, err
		}
		rules = append(rules, parent...)
	}
	names := []string{".ignore"}
	if f.ordinary {
		names = []string{".gitignore", ".ignore"}
	}
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(f.root, dir, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
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
				return nil, fmt.Errorf("%s: %w", filepath.Join(dir, name), err)
			}
			prefix := "(?:^|.*/)"
			if anchored {
				prefix = "^"
			}
			re, err := regexp.Compile(prefix + body + "(?:/.*)?$")
			if err != nil {
				return nil, err
			}
			base := filepath.ToSlash(dir)
			if base == "." {
				base = ""
			}
			rules = append(rules, ignoreRule{base: base, pattern: re, negate: negate, directory: directory})
		}
	}
	f.cache[dir] = rules
	return rules, nil
}
func (f *ignoreFiles) ignored(relative string) (bool, error) {
	rules, err := f.rules(filepath.Dir(relative))
	if err != nil {
		return false, err
	}
	parts := strings.Split(filepath.ToSlash(relative), "/")
	// A child cannot be re-included while its parent directory remains ignored.
	for i := range parts {
		path := strings.Join(parts[:i+1], "/")
		ignored := false
		for _, rule := range rules {
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
