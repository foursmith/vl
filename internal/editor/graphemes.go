package editor

import (
	"sort"
	"unicode/utf8"

	"github.com/clipperhouse/uax29/v2/graphemes"
)

// Rune offsets remain the document/highlighter contract. Only editing positions
// are snapped to extended grapheme boundaries; no Unicode normalization occurs.
func (d *Document) boundaries(line int) []int {
	text := d.Lines()[line]
	if len(text) == d.LineRuneCount(line) {
		return nil
	} // ASCII, including tabs.
	if b, ok := d.graphemes[line]; ok {
		return b
	}
	b := []int{0}
	offset := 0
	iter := graphemes.FromString(text)
	for iter.Next() {
		offset += utf8.RuneCountInString(iter.Value())
		b = append(b, offset)
	}
	if d.graphemes == nil || len(d.graphemes) >= 128 {
		d.graphemes = make(map[int][]int)
	}
	d.graphemes[line] = b
	return b
}
func (d *Document) snap(cursor int, forward bool) int {
	d.Lines()
	cursor = min(max(0, cursor), d.runeCount)
	line := sort.Search(len(d.lineOffsets), func(i int) bool { return d.lineOffsets[i] > cursor }) - 1
	b := d.boundaries(line)
	if b == nil {
		return cursor
	}
	start := d.lineOffsets[line]
	col := cursor - start
	i := sort.SearchInts(b, col)
	if i < len(b) && b[i] == col {
		return cursor
	}
	if forward {
		return start + b[i]
	}
	return start + b[i-1]
}
func (d *Document) previous(cursor int) int {
	if cursor <= 0 {
		return 0
	}
	return d.snap(cursor-1, false)
}
func (d *Document) next(cursor int) int {
	d.Lines()
	if cursor >= d.runeCount {
		return d.runeCount
	}
	return d.snap(cursor+1, true)
}

// ASCII resets the segmentation context. A local cluster check avoids rebuilding
// the document index for ordinary typing, including ASCII before Unicode text.
func knownBoundary(text string, offset int) bool {
	if offset == 0 || offset == len(text) {
		return true
	}
	if text[offset-1] < utf8.RuneSelf {
		iter := graphemes.FromString(text[offset-1:])
		iter.Next()
		return iter.End() == 1
	}
	return false
}

// SelectRange expands a rune range so selection never cuts a grapheme in half.
func (d *Document) SelectRange(start, end int) {
	d.selectionKind = 0
	if start == end {
		d.Cursor = d.snap(start, false)
		d.Anchor = -1
		return
	}
	d.Anchor = d.snap(start, start > end)
	d.Cursor = d.snap(end, start <= end)
}
