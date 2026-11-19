package app

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/foursmith/vl/internal/imagepreview"
	"github.com/yuin/goldmark/util"
)

// Keep Markdown image decoding bounded per tab. The same local file can be
// placed repeatedly using one decoded image and one Kitty upload.
const maxMarkdownImageFiles = 8

type markdownImageRef struct {
	line   int
	path   string
	remote bool
}

func markdownImagePath(document string, destination []byte) (path string, remote bool) {
	raw := string(util.URLEscape(destination, true))
	if strings.HasPrefix(raw, "//") {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	if (u.Scheme == "http" || u.Scheme == "https") && u.Hostname() != "" && u.User == nil {
		u.Fragment, u.RawFragment = "", ""
		return u.String(), true
	}
	if u.Scheme != "" || u.Host != "" || u.Path == "" {
		return "", false
	}
	path = filepath.FromSlash(u.Path)
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(document), path)
	}
	if !imagepreview.Supported(path) {
		return "", false
	}
	return filepath.Clean(path), false
}

type markdownImageCache struct {
	source     string
	paths      []string
	files      map[string]*imageState
	remote     map[string]*markdownRemoteImage
	remoteScan *markdownRemoteCacheScan
	cancel     context.CancelFunc
	job        uint64
	key        markdownImageLayoutKey
	blocks     []markdownImageBlock
}

type markdownImageLayoutKey struct {
	source                               string
	width, height, cellWidth, cellHeight int
}

type markdownImageBlock struct {
	line, first int // Source line and first inserted screen row.
	address     string
	image       *imageState
	region      imagepreview.Region
}

type markdownImageLoadedMsg struct {
	cache *markdownImageCache
	job   uint64
	path  string
	image *imagepreview.Image
}

func (m *Model) scheduleMarkdownImages() tea.Cmd {
	if !m.editing() || !m.kittySupported {
		return nil
	}
	t := &m.tabs[m.activeTab]
	t.syncMarkdown()
	refs := t.markdown.images
	if len(refs) == 0 && t.markdownImages == nil {
		return nil
	}
	if t.markdownImages == nil {
		t.markdownImages = &markdownImageCache{
			files:  make(map[string]*imageState),
			remote: make(map[string]*markdownRemoteImage),
		}
	}
	cache := t.markdownImages
	if cache.source == t.doc.Text {
		return nil
	}
	cache.source = t.doc.Text
	var paths, remotePaths []string
	remoteRefs := make(map[string]bool)
	for _, ref := range refs {
		if ref.remote {
			if !remoteRefs[ref.path] {
				remoteRefs[ref.path] = true
				remotePaths = append(remotePaths, ref.path)
			}
		} else if len(paths) < maxMarkdownImageFiles && !slices.Contains(paths, ref.path) {
			paths = append(paths, ref.path)
		}
	}
	for address, state := range cache.remote {
		if !remoteRefs[address] {
			if state.cancel != nil {
				state.cancel()
			}
			if state.image != nil {
				m.retireImageID(state.image.id)
			}
			delete(cache.remote, address)
		}
	}
	return tea.Batch(m.scheduleLocalMarkdownImages(cache, paths), m.scheduleMarkdownRemoteCache(cache, remotePaths))
}

func (m *Model) scheduleLocalMarkdownImages(cache *markdownImageCache, paths []string) tea.Cmd {
	if slices.Equal(paths, cache.paths) {
		return nil
	}
	if cache.cancel != nil {
		cache.cancel()
		cache.cancel = nil
	}
	cache.job++
	cache.paths = paths
	for path, im := range cache.files {
		if !slices.Contains(paths, path) {
			if im != nil {
				m.retireImageID(im.id)
			}
			delete(cache.files, path)
		}
	}

	if len(cache.files) == len(paths) {
		return nil
	}
	ctx, cancel := context.WithCancel(m.ctx)
	cache.cancel = cancel
	job := cache.job
	slots := make(chan struct{}, 1)
	var commands []tea.Cmd
	for _, path := range paths {
		if _, loaded := cache.files[path]; loaded {
			continue
		}
		commands = append(commands, func() tea.Msg {
			select {
			case slots <- struct{}{}:
			case <-ctx.Done():
				return nil
			}
			defer func() { <-slots }()
			if ctx.Err() != nil {
				return nil
			}
			im := readMarkdownImage(ctx, path)
			if ctx.Err() != nil {
				return nil
			}
			return markdownImageLoadedMsg{cache, job, path, im}
		})
	}
	return tea.Batch(commands...)
}

func readMarkdownImage(ctx context.Context, path string) *imagepreview.Image {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}
	im, _ := imagepreview.Open(ctx, path)
	return im
}

func markdownImagePixels(im *imagepreview.Image) int {
	bounds := im.Bounds()
	return bounds.Dx() * bounds.Dy()
}

func (cache *markdownImageCache) remainingPixels() int {
	remaining := imagepreview.MaxPixels
	for _, im := range cache.files {
		if im != nil {
			remaining -= markdownImagePixels(im.Image)
		}
	}
	for _, remote := range cache.remote {
		if remote.image != nil {
			remaining -= markdownImagePixels(remote.image.Image)
		}
	}
	return remaining
}

func (m *Model) finishMarkdownImage(msg markdownImageLoadedMsg) {
	for i := range m.tabs {
		t := &m.tabs[i]
		cache := t.markdownImages
		if cache == nil || cache != msg.cache || cache.job != msg.job {
			continue
		}
		im := msg.image
		added := im != nil && markdownImagePixels(im) <= cache.remainingPixels()
		cache.files[msg.path] = nil
		if added {
			cache.files[msg.path] = newImageState(im, m.allocateImageID())
		}
		if cache.cancel != nil && len(cache.files) == len(cache.paths) {
			cache.cancel()
			cache.cancel = nil
		}
		if !added {
			return
		}
		m.updateMarkdownImageRows(i)
		return
	}
}

func (m *Model) updateMarkdownImageRows(index int) {
	t := &m.tabs[index]
	t.markdownImages.key = markdownImageLayoutKey{}
	if index == m.activeTab && m.editing() {
		line, _ := t.doc.Position()
		y := t.editorDisplayRow(line) - t.editorDisplayTop()
		m.syncMarkdownImageRows(t)
		m.cancelFlash()
		if y >= 0 && y < m.contentHeight() {
			m.ensureEditor()
		} else {
			m.clampEditorScroll(t)
		}
	}
}
