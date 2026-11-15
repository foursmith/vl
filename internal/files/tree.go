package files

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

type Entry struct {
	Path               string
	Label              string
	Directory, Ignored bool
	Depth              int
}

func ReadTree(ctx context.Context, root string, expanded map[string]bool) ([]Entry, error) {
	var result []Entry
	readDirectory := func(path string) ([]os.DirEntry, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		entries, err := os.ReadDir(filepath.Join(root, path))
		return slices.DeleteFunc(entries, func(entry os.DirEntry) bool { return entry.Name() == ".git" }), err
	}
	var visit func(string, []os.DirEntry, int) error
	visit = func(relative string, entries []os.DirEntry, depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		sort.Slice(entries, func(i, j int) bool {
			a, b := entries[i], entries[j]
			if a.IsDir() != b.IsDir() {
				return a.IsDir()
			}
			return strings.ToLower(a.Name()) < strings.ToLower(b.Name())
		})
		for _, entry := range entries {
			path := filepath.ToSlash(filepath.Join(relative, entry.Name()))
			var children []os.DirEntry
			var readErr error
			if entry.IsDir() {
				// Inspect only the single-directory chain, even while collapsed.
				for {
					children, readErr = readDirectory(path)
					if readErr != nil || len(children) != 1 || !children[0].IsDir() {
						break
					}
					path += "/" + children[0].Name()
				}
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			result = append(result, Entry{Path: path, Label: strings.TrimPrefix(path, relative+"/"), Directory: entry.IsDir(), Depth: depth})
			if entry.IsDir() && expanded[path] {
				if readErr != nil {
					return readErr
				}
				if err := visit(path, children, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}
	entries, err := readDirectory("")
	if err != nil {
		return nil, err
	}
	err = visit("", entries, 0)
	return result, err
}

func Create(root, path string, directory bool) error {
	clean := filepath.Clean(path)
	if filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("enter a path inside the workspace")
	}
	for _, part := range strings.Split(filepath.ToSlash(clean), "/") {
		if part == ".git" {
			return fmt.Errorf("cannot create entries in Git metadata")
		}
	}
	target := filepath.Join(root, clean)
	parent := filepath.Dir(target)
	realParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return err
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(realRoot, realParent)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("parent folder is outside the workspace")
	}
	if directory {
		return os.Mkdir(target, 0755)
	}
	f, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	return f.Close()
}
