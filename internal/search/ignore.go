package search

import (
	"os"
	"path"
	"path/filepath"
)

type directoryIgnores struct {
	rules   pathIgnoreRules
	ignored bool
}

// Cache the same inherited rules as the directory walk for open buffers that
// were not visited on disk, including buffers whose files were deleted.
type ignoreFiles struct {
	root    string
	cache   map[string]directoryIgnores
	include func(string) bool
}

func (f *ignoreFiles) rules(dir string) (directoryIgnores, error) {
	if scope, ok := f.cache[dir]; ok {
		return scope, nil
	}
	var scope directoryIgnores
	prefix := ""
	if dir != "." {
		parent, err := f.rules(path.Dir(dir))
		if err != nil {
			return directoryIgnores{}, err
		}
		if parent.ignored || parent.rules.ignored(dir, path.Base(dir), true) && !f.explicitlyIncluded(dir) {
			scope.ignored = true
			f.cache[dir] = scope
			return scope, nil
		}
		scope.rules = parent.rules
		prefix = dir + "/"
	}
	for kind, name := range localIgnoreNames {
		filename := filepath.Join(f.root, dir, name)
		info, err := os.Lstat(filename)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return directoryIgnores{}, err
		}
		if info.Mode().IsRegular() {
			if err := scope.rules.load(kind, filename, prefix); err != nil {
				return directoryIgnores{}, err
			}
		}
	}
	f.cache[dir] = scope
	return scope, nil
}

func (f *ignoreFiles) ignored(relative string) (bool, error) {
	scope, err := f.rules(path.Dir(relative))
	if err != nil {
		return false, err
	}
	return scope.ignored || scope.rules.ignored(relative, path.Base(relative), false) && !f.explicitlyIncluded(relative), nil
}

func (f *ignoreFiles) explicitlyIncluded(relative string) bool {
	return f.include != nil && f.include(relative)
}
