package app

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/ourongxing/vl/internal/imagepreview"
	"github.com/yuin/goldmark/util"
)

// Keep Markdown image decoding bounded per tab. The same local file can be
// placed repeatedly using one decoded image and one Kitty upload.
const maxMarkdownImageFiles = 8

type markdownImageRef struct {
	line int
	path string
}

func markdownImagePath(document string, destination []byte) string {
	raw := string(util.URLEscape(destination, true))
	if strings.HasPrefix(raw, "//") {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if u.Scheme == "http" || u.Scheme == "https" {
		if u.Hostname() == "" || u.User != nil {
			return ""
		}
		u.Fragment = ""
		return u.String()
	}
	if u.Scheme != "" || u.Host != "" || u.Path == "" {
		return ""
	}
	path := filepath.FromSlash(u.Path)
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(document), path)
	}
	if !imagepreview.Supported(path) {
		return ""
	}
	return filepath.Clean(path)
}

type markdownImageCache struct {
	source string
	paths  []string
	files  map[string]*imageState
	cancel context.CancelFunc
	job    uint64
	key    markdownImageLayoutKey
	blocks []markdownImageBlock
}

type markdownImageLayoutKey struct {
	source                               string
	width, height, cellWidth, cellHeight int
}

type markdownImageBlock struct {
	line, first int // Source line and first inserted screen row.
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
		t.markdownImages = &markdownImageCache{files: make(map[string]*imageState)}
	}
	cache := t.markdownImages
	if cache.source == t.doc.Text {
		return nil
	}
	cache.source = t.doc.Text
	var paths []string
	for _, ref := range refs {
		if !slices.Contains(paths, ref.path) {
			paths = append(paths, ref.path)
			if len(paths) == maxMarkdownImageFiles {
				break
			}
		}
	}
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
				m.imageRetired = append(m.imageRetired, im.id)
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
	// Reserve a local slot so slow HTTP requests cannot delay local images.
	localSlots, remoteSlots := make(chan struct{}, 1), make(chan struct{}, 2)
	var commands []tea.Cmd
	for _, path := range paths {
		if _, loaded := cache.files[path]; loaded {
			continue
		}
		slots := localSlots
		remote := strings.HasPrefix(path, "https://") || strings.HasPrefix(path, "http://")
		if remote {
			slots = remoteSlots
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
			im := readMarkdownImage(ctx, path, remote)
			if ctx.Err() != nil {
				return nil
			}
			return markdownImageLoadedMsg{cache, job, path, im}
		})
	}
	return tea.Batch(commands...)
}

func readMarkdownImage(ctx context.Context, path string, remote bool) *imagepreview.Image {
	if remote {
		im, _ := imagepreview.OpenURL(ctx, path)
		return im
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}
	im, _ := imagepreview.Open(path)
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
		cache.key = markdownImageLayoutKey{}
		if i == m.activeTab && m.editing() {
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
		return
	}
}
