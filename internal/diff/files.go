package diff

import (
	"fmt"
	"strconv"
	"strings"
)

type FilePatch struct {
	Path   string
	Raw    string
	Status byte
}

// SplitFiles retains each file's extended headers for the diff viewer.
func SplitFiles(raw string) ([]FilePatch, error) {
	const header = "diff --git "
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	if !strings.HasPrefix(raw, header) {
		return nil, fmt.Errorf("expected a git diff")
	}
	var files []FilePatch
	indices := make(map[string]int)
	for raw != "" {
		end := strings.Index(raw, "\n"+header) + 1
		if end == 0 {
			end = len(raw)
		}
		patch := raw[:end]
		path, status := patchPath(patch)
		if path == "" {
			return nil, fmt.Errorf("missing path in file diff %d", len(files)+1)
		}
		if index, ok := indices[path]; ok {
			files[index].Raw += patch
			files[index].Status = 'T'
		} else {
			indices[path] = len(files)
			files = append(files, FilePatch{Path: path, Raw: patch, Status: status})
		}
		raw = raw[end:]
	}
	return files, nil
}

func patchPath(patch string) (string, byte) {
	header, rest, _ := strings.Cut(patch, "\n")
	oldPath := ""
	status := byte('M')
	for line := range strings.SplitSeq(rest, "\n") {
		switch {
		case strings.HasPrefix(line, "@@ "):
			return oldPath, status
		case strings.HasPrefix(line, "new file mode "):
			status = 'A'
		case strings.HasPrefix(line, "deleted file mode "):
			status = 'D'
		case strings.HasPrefix(line, "rename to "):
			return gitPath(strings.TrimPrefix(line, "rename to ")), 'R'
		case strings.HasPrefix(line, "copy to "):
			return gitPath(strings.TrimPrefix(line, "copy to ")), 'C'
		case strings.HasPrefix(line, "--- "):
			oldPath = strings.TrimPrefix(gitPath(strings.TrimPrefix(line, "--- ")), "a/")
		case strings.HasPrefix(line, "+++ "):
			path := gitPath(strings.TrimPrefix(line, "+++ "))
			if path == "/dev/null" {
				return oldPath, 'D'
			}
			return strings.TrimPrefix(path, "b/"), status
		}
	}
	// Binary and mode-only changes can have only the diff header. Both paths
	// are identical after removing the a/ and b/ prefixes in these cases.
	paths := strings.TrimPrefix(header, "diff --git ")
	for offset, r := range paths {
		if r != ' ' {
			continue
		}
		left, right := gitPath(paths[:offset]), gitPath(paths[offset+1:])
		if strings.HasPrefix(left, "a/") && strings.HasPrefix(right, "b/") && left[2:] == right[2:] {
			return right[2:], status
		}
	}
	return "", 0
}

func gitPath(path string) string {
	path = strings.TrimSuffix(path, "\t")
	if strings.HasPrefix(path, "\"") {
		if decoded, err := strconv.Unquote(path); err == nil {
			return decoded
		}
	}
	return path
}
