package search

import (
	"os"
	"path"
	"path/filepath"
)

type directoryIgnores struct {
	rules   []pathIgnoreRule
	ignored bool
}

// Cache the same inherited rules as the directory walk for open buffers that
// were not visited on disk, including buffers whose files were deleted.
type ignoreFiles struct {
	root  string
	cache map[string]directoryIgnores
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
		if parent.ignored || ignoredFilePath(parent.rules, dir, path.Base(dir), true) {
			scope.ignored = true
			f.cache[dir] = scope
			return scope, nil
		}
		scope.rules = parent.rules[:len(parent.rules):len(parent.rules)]
		prefix = dir + "/"
	}
	for _, name := range localIgnoreNames {
		filename := filepath.Join(f.root, dir, name)
		info, err := os.Lstat(filename)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return directoryIgnores{}, err
		}
		if info.Mode().IsRegular() {
			scope.rules, err = appendPathIgnoreRules(filename, prefix, scope.rules)
			if err != nil {
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
	return scope.ignored || ignoredFilePath(scope.rules, relative, path.Base(relative), false), nil
}
