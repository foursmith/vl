package app

import (
	"context"
	"errors"
	"slices"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/config"
	"github.com/foursmith/vl/internal/imagepreview"
	"github.com/foursmith/vl/internal/theme"
)

type markdownRemoteImage struct {
	cancel context.CancelFunc
	image  *imageState
	err    error
}

const markdownImageCacheKind = "markdown-images"

type markdownRemoteImageMsg struct {
	cache   *markdownImageCache
	address string
	state   *markdownRemoteImage
	image   *imagepreview.Image
	err     error
}

type markdownRemoteCacheScan struct {
	paths  []string
	cancel context.CancelFunc
}

type markdownRemoteCacheMsg struct {
	cache  *markdownImageCache
	scan   *markdownRemoteCacheScan
	images map[string]*imagepreview.Image
}

type markdownRemoteBudget struct {
	slots, pixels int
}

func (cache *markdownImageCache) remoteBudget() markdownRemoteBudget {
	slots := maxMarkdownImageFiles
	for _, state := range cache.remote {
		if state.cancel != nil || state.image != nil {
			slots--
		}
	}
	return markdownRemoteBudget{slots, cache.remainingPixels()}
}

func (b markdownRemoteBudget) full() bool { return b.slots <= 0 || b.pixels <= 0 }

func (b *markdownRemoteBudget) take(im *imagepreview.Image) bool {
	if im == nil || b.full() {
		return false
	}
	pixels := markdownImagePixels(im)
	if pixels > b.pixels {
		return false
	}
	b.slots--
	b.pixels -= pixels
	return true
}

func (m *Model) scheduleMarkdownRemoteCache(cache *markdownImageCache, paths []string) tea.Cmd {
	paths = slices.DeleteFunc(paths, func(path string) bool { return cache.remote[path] != nil })
	if scan := cache.remoteScan; scan != nil {
		if slices.Equal(scan.paths, paths) {
			return nil
		}
		scan.cancel()
		cache.remoteScan = nil
	}
	if len(paths) == 0 {
		return nil
	}
	limit := cache.remoteBudget()
	if limit.full() {
		return nil
	}
	ctx, cancel := context.WithCancel(m.ctx)
	scan := &markdownRemoteCacheScan{paths: paths, cancel: cancel}
	cache.remoteScan = scan
	return func() tea.Msg {
		dir := config.CacheDir(markdownImageCacheKind)
		budget := limit
		images := make(map[string]*imagepreview.Image)
		for _, path := range scan.paths {
			if ctx.Err() != nil {
				return nil
			}
			if budget.full() {
				break
			}
			if im, _ := imagepreview.OpenRemote(ctx, path, dir, false); budget.take(im) {
				images[path] = im
			}
		}
		return markdownRemoteCacheMsg{cache, scan, images}
	}
}

func (m *Model) finishMarkdownRemoteCache(msg markdownRemoteCacheMsg) {
	for i := range m.tabs {
		cache := m.tabs[i].markdownImages
		if cache != msg.cache || cache.remoteScan != msg.scan {
			continue
		}
		msg.scan.cancel()
		cache.remoteScan = nil
		budget := cache.remoteBudget()
		for _, path := range msg.scan.paths {
			if cache.remote[path] != nil {
				continue
			}
			state := &markdownRemoteImage{}
			cache.remote[path] = state
			if im := msg.images[path]; budget.take(im) {
				state.image = newImageState(im, m.allocateImageID())
			}
		}
		m.updateMarkdownImageRows(i)
		return
	}
}

func (m *Model) loadMarkdownRemoteImage(address string) tea.Cmd {
	cache := m.tabs[m.activeTab].markdownImages
	if state := cache.remote[address]; state != nil && (state.cancel != nil || state.image != nil) {
		return nil
	}
	if cache.remoteBudget().full() {
		m.notice = m.tr("Markdown image preview limit reached")
		return nil
	}

	ctx, cancel := context.WithCancel(m.ctx)
	state := &markdownRemoteImage{cancel: cancel}
	cache.remote[address] = state
	return func() tea.Msg {
		im, err := imagepreview.OpenRemote(ctx, address, config.CacheDir(markdownImageCacheKind), true)
		return markdownRemoteImageMsg{cache, address, state, im, err}
	}
}

func (m *Model) finishMarkdownRemoteImage(msg markdownRemoteImageMsg) {
	for i := range m.tabs {
		cache := m.tabs[i].markdownImages
		if cache != msg.cache || cache.remote[msg.address] != msg.state {
			continue
		}
		state := msg.state
		state.cancel()
		state.cancel, state.err = nil, msg.err
		if msg.image == nil {
			return
		}
		budget := cache.remoteBudget()
		if budget.take(msg.image) {
			state.image = newImageState(msg.image, m.allocateImageID())
			m.updateMarkdownImageRows(i)
		} else {
			state.err = errors.New("preview limit reached")
			if i == m.activeTab {
				m.notice = m.tr("Markdown image preview limit reached")
			}
		}
		return
	}
}

func (m *Model) markdownRemoteImageLabel(cache *markdownImageCache, address string) string {
	if state := cache.remote[address]; state != nil {
		switch {
		case state.cancel != nil:
			return m.tr("Loading image…")
		case errors.Is(state.err, imagepreview.ErrCurlUnavailable):
			return m.tr("Retry image (curl required)")
		case state.err != nil:
			return m.tr("Retry image (load failed)")
		}
	}
	return m.tr("Load image") + "  ↵"
}

func (m *Model) markdownRemoteImageRow(cache *markdownImageCache, block *markdownImageBlock, offset int) string {
	w, h := block.region.Columns, block.region.Rows
	bg := m.colors.Background
	border := theme.Style(m.colors.Border, bg)
	if offset == 1 {
		return inputBorderRule(border, w, "╭", "╮")
	}
	if offset == h {
		return inputBorderRule(border, w, "╰", "╯")
	}
	inner := max(0, w-2)
	label := ""
	if offset == (h+1)/2 {
		label = ansi.Truncate(m.markdownRemoteImageLabel(cache, block.address), inner, "…")
		label = strings.Repeat(" ", (inner-ansi.StringWidth(label))/2) + label
	}
	side := border.Render("│")
	return side + row(label, inner, m.colors.Accent, bg) + side
}

func (m *Model) clickMarkdownRemoteImage(x, y int) (tea.Cmd, bool) {
	x -= editorGutterWidth
	if !m.editing() || !m.kittySupported || x < 0 {
		return nil, false
	}
	t := &m.tabs[m.activeTab]
	_, block, _ := t.editorDisplayAt(t.editorDisplayTop() + y)
	if block == nil || block.image != nil || x >= block.region.Columns || y < len(m.editorStickyRows()) {
		return nil, false
	}
	return m.loadMarkdownRemoteImage(block.address), true
}

func (m *Model) enterMarkdownRemoteImage() (tea.Cmd, bool) {
	if !m.editing() || !m.kittySupported {
		return nil, false
	}
	t := &m.tabs[m.activeTab]
	if t.markdownImages == nil || m.textInserting() || t.vim.VisualMode || t.vim.CommandPending() {
		return nil, false
	}
	line, _ := t.doc.Position()
	blocks := t.markdownImages.blocks
	first := sort.Search(len(blocks), func(i int) bool { return blocks[i].line >= line })
	for i := first; i < len(blocks) && blocks[i].line == line; i++ {
		block := &blocks[i]
		state := t.markdownImages.remote[block.address]
		if block.image == nil && (state == nil || state.cancel == nil) {
			return m.loadMarkdownRemoteImage(block.address), true
		}
	}
	return nil, false
}
