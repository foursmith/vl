package files

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Copy creates a new entry exclusively. Existing entries are never overwritten.
// Both paths stay inside root; symlinks and special files are rejected.
func Copy(ctx context.Context, root, source, destination string) error {
	src, err := entryPath(source)
	if err != nil {
		return err
	}
	dst, err := entryPath(destination)
	if err != nil {
		return err
	}
	if dst == src || strings.HasPrefix(dst, src+string(filepath.Separator)) {
		return fmt.Errorf("cannot copy an entry into itself")
	}
	dir, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer dir.Close()
	sourceInfo, err := dir.Lstat(src)
	if err != nil {
		return err
	}
	// Reject symlink ancestors so a destination cannot alias the source tree.
	for _, path := range []string{src, filepath.Dir(dst)} {
		for parent := path; parent != "."; parent = filepath.Dir(parent) {
			info, err := dir.Lstat(parent)
			if err != nil {
				return err
			}
			if path == filepath.Dir(dst) && sourceInfo.IsDir() && os.SameFile(sourceInfo, info) {
				return fmt.Errorf("cannot copy an entry into itself")
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("cannot copy through symbolic links: %s", parent)
			}
		}
	}
	created := false
	var copyEntry func(string, string) error
	copyEntry = func(from, to string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		info, err := dir.Lstat(from)
		if err != nil {
			return err
		}
		if info.IsDir() {
			if err = dir.Mkdir(to, info.Mode().Perm()|0700); err != nil {
				return err
			}
			if to == dst {
				created = true
			}
			f, err := dir.Open(from)
			if err != nil {
				return err
			}
			entries, err := f.ReadDir(-1)
			f.Close()
			if err != nil {
				return err
			}
			for _, entry := range entries {
				if entry.Name() == ".git" {
					continue
				}
				if err = copyEntry(filepath.Join(from, entry.Name()), filepath.Join(to, entry.Name())); err != nil {
					return err
				}
			}
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("cannot copy symbolic links or special files: %s", from)
		}
		input, err := dir.Open(from)
		if err != nil {
			return err
		}
		defer input.Close()
		output, err := dir.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
		if err != nil {
			return err
		}
		if to == dst {
			created = true
		}
		// Check cancellation between chunks, including large individual files.
		buffer := make([]byte, 128*1024)
		for {
			if err = ctx.Err(); err != nil {
				break
			}
			n, readErr := input.Read(buffer)
			if n > 0 {
				if _, err = output.Write(buffer[:n]); err != nil {
					break
				}
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				err = readErr
				break
			}
		}
		closeErr := output.Close()
		if err != nil {
			return err
		}
		return closeErr
	}
	err = copyEntry(src, dst)
	if err != nil && created {
		_ = dir.RemoveAll(dst)
	}
	return err
}
