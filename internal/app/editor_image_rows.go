package app

import "sort"

func (m *Model) syncMarkdownImageRows(t *tab) {
	cache := t.markdownImages
	if cache == nil {
		return
	}
	w := min(48, m.mainWidth()-7)
	h := min(12, m.contentHeight()/2)
	if !m.kittySupported || w <= 0 || h <= 0 {
		cache.key, cache.blocks = markdownImageLayoutKey{}, nil
		t.topImageOffset = 0
		return
	}
	key := markdownImageLayoutKey{t.doc.Text, w, h, m.cellWidth, m.cellHeight}
	if cache.key == key {
		return
	}
	t.syncMarkdown()
	clear(cache.blocks)
	cache.key, cache.blocks = key, cache.blocks[:0]
	extra, previous := 0, -1
	for _, ref := range t.markdown.images {
		im := cache.files[ref.path]
		if im == nil || ref.line == previous {
			continue
		}
		base := t.visibleRow(ref.line)
		if t.sourceRow(base) != ref.line {
			continue
		}
		region := im.ViewRegion(w, h, m.cellWidth, m.cellHeight, 1, .5, .5)
		if region.Rows > 0 && region.Columns > 0 {
			cache.blocks = append(cache.blocks, markdownImageBlock{ref.line, base + extra + 1, im, region})
			extra += region.Rows
			previous = ref.line
		}
	}
}

// Display coordinates insert rows after a source line. Vim motions and
// selections continue to operate in source coordinates.
func (t *tab) editorDisplayRow(line int) int {
	base := t.visibleRow(line)
	if c := t.markdownImages; c != nil {
		i := sort.Search(len(c.blocks), func(i int) bool { return c.blocks[i].line >= line })
		if i > 0 {
			prev := c.blocks[i-1]
			base += prev.first + prev.region.Rows - t.visibleRow(prev.line) - 1
		}
	}
	return base
}

func (t *tab) editorDisplayAt(row int) (line int, block *markdownImageBlock, offset int) {
	if c := t.markdownImages; c != nil {
		i := sort.Search(len(c.blocks), func(i int) bool { return c.blocks[i].first+c.blocks[i].region.Rows > row })
		if i < len(c.blocks) && row >= c.blocks[i].first {
			b := &c.blocks[i]
			return b.line, b, row - b.first + 1
		}
		if i > 0 {
			prev := c.blocks[i-1]
			row -= prev.first + prev.region.Rows - t.visibleRow(prev.line) - 1
		}
	}
	return t.sourceRow(row), nil, 0
}

func (t *tab) editorDisplayTop() int {
	top := t.editorDisplayRow(t.top)
	if t.topImageOffset > 0 {
		if line, block, _ := t.editorDisplayAt(top + 1); block != nil && line == t.top {
			top += min(t.topImageOffset, block.region.Rows)
		}
	}
	return top
}

func (t *tab) setEditorDisplayTop(row int) {
	t.top, _, t.topImageOffset = t.editorDisplayAt(max(0, row))
}

func (m *Model) visibleMarkdownImages(visit func(*markdownImageBlock)) {
	if !m.editing() || !m.kittySupported || m.width < 30 || m.height < 8 || m.width < 70 && m.sidebar && m.sidebarVisible {
		return
	}
	t := &m.tabs[m.activeTab]
	if t.markdownImages == nil {
		return
	}
	top := t.editorDisplayTop()
	end := top + m.contentHeight()
	blocks := t.markdownImages.blocks
	first := sort.Search(len(blocks), func(i int) bool { return blocks[i].first+blocks[i].region.Rows > top })
	for i := first; i < len(blocks) && blocks[i].first < end; i++ {
		visit(&blocks[i])
	}
}
