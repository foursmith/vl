package files

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Entry struct {
	Path               string
	Directory, Ignored bool
	Depth              int
}

func ReadTree(ctx context.Context, root string, expanded map[string]bool) ([]Entry, error) {
	var result []Entry
	var visit func(string, int) error
	visit = func(relative string, depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		entries, err := os.ReadDir(filepath.Join(root, relative))
		if err != nil {
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
			if entry.Name() == ".git" {
				continue
			}
			path := filepath.ToSlash(filepath.Join(relative, entry.Name()))
			result = append(result, Entry{Path: path, Directory: entry.IsDir(), Depth: depth})
			if entry.IsDir() && expanded[path] {
				if err := visit(path, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}
	err := visit("", 0)
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
