package search

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

func readRegularFile(path string, scratch *bytes.Buffer) ([]byte, error) {
	real, err := filepath.EvalSymlinks(path)
	if err != nil || real != path {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, err
	}
	const retainedSize = 1 << 20
	if info.Size() > retainedSize {
		return os.ReadFile(path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if scratch.Cap() > retainedSize {
		*scratch = bytes.Buffer{}
	} else {
		scratch.Reset()
	}
	scratch.Grow(int(info.Size()) + bytes.MinRead)
	_, err = scratch.ReadFrom(file)
	return scratch.Bytes(), err
}

func normalizeText(data []byte) string {
	return strings.TrimPrefix(strings.ReplaceAll(string(data), "\r\n", "\n"), "\xef\xbb\xbf")
}
