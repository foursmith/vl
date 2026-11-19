package imagepreview

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"
)

const remoteCacheBytes = 128 << 20

var remoteCacheWrites sync.Mutex

var ErrCurlUnavailable = errors.New("curl is unavailable")

// OpenRemote checks the on-disk cache first. It invokes curl on a miss only
// when allowDownload is true.
func OpenRemote(ctx context.Context, address, cacheDir string, allowDownload bool) (*Image, error) {
	u, err := url.Parse(address)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return nil, fmt.Errorf("invalid remote image URL")
	}

	cachePath := remoteCachePath(cacheDir, address)
	if im := readRemoteCache(ctx, cachePath, u.Path); im != nil {
		return im, nil
	}
	if !allowDownload {
		return nil, nil
	}

	curl, err := exec.LookPath("curl")
	if err != nil {
		return nil, ErrCurlUnavailable
	}
	fetchCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(fetchCtx, curl, "--disable", "--silent", "--fail", "--location",
		"--max-redirs", "5", "--connect-timeout", "5", "--globoff",
		"--proto", "=http,https", "--proto-redir", "=http,https",
		"--max-filesize", strconv.Itoa(MaxBytes), "--url", address)
	cmd.WaitDelay = 200 * time.Millisecond
	var output previewOutput
	cmd.Stdout = &output
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	data := output.buf.Bytes()
	im, err := Decode(ctx, u.Path, data)
	if err == nil && ctx.Err() == nil && cachePath != "" {
		saveRemoteCache(cachePath, data)
	}
	return im, err
}

func remoteCachePath(dir, address string) string {
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, fmt.Sprintf("%x.img", sha256.Sum256([]byte(address))))
}

func readRemoteCache(ctx context.Context, path, imagePath string) *Image {
	if path == "" || ctx.Err() != nil {
		return nil
	}
	if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
		return nil
	}
	data, err := readImageFile(path)
	if err != nil {
		return nil
	}
	im, err := Decode(ctx, imagePath, data)
	if err != nil {
		return nil
	}
	now := time.Now()
	_ = os.Chtimes(path, now, now)
	return im
}

func saveRemoteCache(path string, data []byte) {
	remoteCacheWrites.Lock()
	defer remoteCacheWrites.Unlock()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return
	}
	file, err := os.CreateTemp(dir, ".image-*")
	if err != nil {
		return
	}
	defer os.Remove(file.Name())
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil || os.Rename(file.Name(), path) != nil {
		return
	}

	entries, _ := os.ReadDir(dir)
	var images []os.FileInfo
	for _, entry := range entries {
		if len(entry.Name()) == 64+len(".img") && filepath.Ext(entry.Name()) == ".img" {
			if info, err := entry.Info(); err == nil && info.Mode().IsRegular() {
				images = append(images, info)
			}
		}
	}
	sort.SliceStable(images, func(i, j int) bool { return images[i].ModTime().After(images[j].ModTime()) })
	var bytes int64
	for _, entry := range images {
		bytes += entry.Size()
		if bytes > remoteCacheBytes {
			stale := filepath.Join(dir, entry.Name())
			if latest, err := os.Lstat(stale); err == nil && os.SameFile(entry, latest) && latest.ModTime().Equal(entry.ModTime()) {
				_ = os.Remove(stale)
			}
		}
	}
}
