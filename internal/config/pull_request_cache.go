package config

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	gitservice "github.com/foursmith/vl/internal/git"
)

type PullRequestDiffCache struct {
	dir   string
	limit int
}

type cachedPullRequestDiff struct {
	Version string
	Patch   string
}

var pullRequestCacheWrites sync.Mutex

func pullRequestCacheDir(workspace, kind string) string {
	base := os.Getenv("XDG_CACHE_HOME")
	if !filepath.IsAbs(base) {
		var err error
		base, err = os.UserCacheDir()
		if err != nil {
			return ""
		}
	}
	key := sha256.Sum256([]byte(filepath.Clean(workspace)))
	return filepath.Join(base, "vl", kind, fmt.Sprintf("%x", key))
}

func NewPullRequestDiffCache(workspace string, limit int) *PullRequestDiffCache {
	dir := pullRequestCacheDir(workspace, "pr-patches")
	if dir == "" {
		return nil
	}
	return &PullRequestDiffCache{dir: dir, limit: limit}
}

type PullRequestListCache struct {
	path    string
	remotes [32]byte
}

type cachedPullRequestList struct {
	Remotes [32]byte
	List    gitservice.PullRequestList
}

func NewPullRequestListCache(workspace, remotes string, filter gitservice.PullRequestFilter) PullRequestListCache {
	dir := pullRequestCacheDir(workspace, "pr-lists")
	if dir == "" || remotes == "" {
		return PullRequestListCache{}
	}
	return PullRequestListCache{
		path:    filepath.Join(dir, string(filter)+".json"),
		remotes: sha256.Sum256([]byte(remotes)),
	}
}

func (c PullRequestListCache) Load() (gitservice.PullRequestList, bool) {
	var entry cachedPullRequestList
	if c.path == "" || loadJSON(c.path, &entry) != nil || entry.Remotes != c.remotes {
		return gitservice.PullRequestList{}, false
	}
	return entry.List, true
}

func (c PullRequestListCache) Save(list gitservice.PullRequestList) {
	if c.path != "" {
		_ = saveJSON(c.path, cachedPullRequestList{Remotes: c.remotes, List: list})
	}
}

func (c *PullRequestDiffCache) recordPath(target string) string {
	return filepath.Join(c.dir, fmt.Sprintf("%x.json", sha256.Sum256([]byte(target))))
}

func (c *PullRequestDiffCache) Load(target, version string) (string, bool) {
	if c == nil {
		return "", false
	}
	var entry cachedPullRequestDiff
	if err := loadJSON(c.recordPath(target), &entry); err != nil || entry.Version == "" || entry.Version != version {
		return "", false
	}
	c.Touch(target)
	return entry.Patch, true
}

func (c *PullRequestDiffCache) Touch(target string) {
	if c != nil {
		now := time.Now()
		_ = os.Chtimes(c.recordPath(target), now, now)
	}
}

func (c *PullRequestDiffCache) Save(target, version, patch string) {
	if c == nil {
		return
	}
	pullRequestCacheWrites.Lock()
	defer pullRequestCacheWrites.Unlock()
	path := c.recordPath(target)
	if err := saveJSON(path, cachedPullRequestDiff{Version: version, Patch: patch}); err != nil {
		return
	}
	entries, err := os.ReadDir(c.dir)
	if err != nil || len(entries) <= c.limit {
		return
	}
	var diffs []os.FileInfo
	for _, entry := range entries {
		if len(entry.Name()) != 64+len(".json") || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		if info, err := entry.Info(); err == nil && info.Mode().IsRegular() {
			diffs = append(diffs, info)
		}
	}
	sort.SliceStable(diffs, func(i, j int) bool {
		return diffs[i].ModTime().After(diffs[j].ModTime())
	})
	for _, entry := range diffs[min(c.limit, len(diffs)):] {
		stalePath := filepath.Join(c.dir, entry.Name())
		// Another process may have used or replaced it since enumeration.
		if latest, err := os.Stat(stalePath); err == nil && os.SameFile(entry, latest) && latest.ModTime().Equal(entry.ModTime()) {
			_ = os.Remove(stalePath)
		}
	}
}
