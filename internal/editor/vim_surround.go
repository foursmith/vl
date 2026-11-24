package editor

import (
	"strings"
	"unicode/utf8"
)

type surroundPhase uint8

const (
	surroundSource surroundPhase = iota
	surroundRange
	surroundObject
	surroundFirstLine
	surroundTarget
)

type surroundBounds struct{ start, insideStart, insideEnd, end int }
type vimSurround struct {
	kind            string
	phase           surroundPhase
	count, digits   int
	inner, linewise bool
	bounds          surroundBounds
	text            string
	cursor, anchor  int
}

func (v *Vim) beginSurround(d *Document, kind string, count int) {
	phase := surroundSource
	if kind == "y" {
		phase = surroundRange
	}
	v.surround = &vimSurround{kind: kind, phase: phase, count: count, text: d.Text, cursor: d.Cursor, anchor: d.Anchor}
}
func (v *Vim) beginVisualSurround(d *Document) {
	start, end := d.Selection()
	v.beginSurround(d, "y", 1)
	request := v.surround
	request.phase = surroundTarget
	request.linewise = v.VisualLine
	request.bounds = surroundBounds{start, start, end, end}
}
func (v *Vim) handleSurround(d *Document, key string) bool {
	request := v.surround
	if request.text != d.Text || request.cursor != d.Cursor || request.anchor != d.Anchor {
		if v.record != nil {
			v.record.unsafe = true
		}
		v.clearPending()
		return true
	}
	functionKey := len(key) > 1 && key[0] == 'f'
	if functionKey {
		for _, r := range key[1:] {
			if r < '0' || r > '9' {
				functionKey = false
				break
			}
		}
	}
	if functionKey || strings.HasPrefix(key, "ctrl+") || strings.HasPrefix(key, "alt+") {
		v.clearPending()
		return false
	}
	switch request.phase {
	case surroundSource:
		bounds, ok := sourceSurround(d, key, request.count)
		if !ok {
			v.clearPending()
			return true
		}
		request.bounds = bounds
		if request.kind == "d" {
			v.completeSurround(d, "", "")
		} else {
			request.phase = surroundTarget
		}
	case surroundRange:
		if appendVimCount(&request.digits, key) {
			return true
		}
		count := min(maxVimCount, request.count*max(1, request.digits))
		request.count = count
		request.digits = 0
		switch key {
		case "i", "a":
			request.inner = key == "i"
			request.phase = surroundObject
		case "g":
			request.phase = surroundFirstLine
		case "s":
			row, _ := d.Position()
			last := min(row+count-1, len(d.Lines())-1)
			start, end := d.LineOffset(row), d.LineOffset(last)+d.LineRuneCount(last)
			request.bounds = surroundBounds{start, start, end, end}
			request.phase = surroundTarget
		default:
			if !v.surroundMotion(d, request, key, count) {
				v.clearPending()
			}
		}
	case surroundObject:
		start, end, ok := v.textObject(d, key, request.inner, request.count)
		if !ok {
			v.clearPending()
			return true
		}
		request.bounds = surroundBounds{start, start, end, end}
		request.phase = surroundTarget
	case surroundFirstLine:
		if key != "g" {
			v.clearPending()
			return true
		}
		row, _ := d.Position()
		target := min(request.count-1, len(d.Lines())-1)
		first, last := min(row, target), max(row, target)
		start, end := d.LineOffset(first), d.LineOffset(last)+d.LineRuneCount(last)
		request.bounds = surroundBounds{start, start, end, end}
		request.phase = surroundTarget
	case surroundTarget:
		open, close, ok := delimiterPair(key)
		if !ok {
			v.clearPending()
			return true
		}
		v.completeSurround(d, open, close)
	}
	return true
}
func (v *Vim) surroundMotion(d *Document, request *vimSurround, key string, count int) bool {
	cursor, anchor := d.Cursor, d.Anchor
	row, _ := d.Position()
	if key == "G" || key == "j" || key == "k" {
		last := row
		switch key {
		case "G":
			last = len(d.Lines()) - 1
			if count > 1 {
				last = min(count-1, last)
			}
		case "j":
			last = min(row+count, len(d.Lines())-1)
		case "k":
			last = max(0, row-count)
		}
		first := min(row, last)
		last = max(row, last)
		start, end := d.LineOffset(first), d.LineOffset(last)+d.LineRuneCount(last)
		request.bounds = surroundBounds{start, start, end, end}
		request.phase = surroundTarget
		return true
	}
	if !v.motion(d, key, count) {
		return false
	}
	end := d.Cursor
	if key == "e" && end < wordMotionEnd(d) {
		end = d.next(end)
	}
	start := min(cursor, end)
	end = max(cursor, end)
	d.Cursor, d.Anchor = cursor, anchor
	if start == end {
		return false
	}
	request.bounds = surroundBounds{start, start, end, end}
	request.phase = surroundTarget
	return true
}
func sourceSurround(d *Document, key string, count int) (surroundBounds, bool) {
	open, close, ok := delimiterPair(key)
	if !ok {
		return surroundBounds{}, false
	}
	start, end, ok := delimiterObject(d, open, close, false, count)
	if !ok {
		return surroundBounds{}, false
	}
	return surroundBounds{start, d.next(start), d.previous(end), end}, true
}
func (v *Vim) completeSurround(d *Document, open, close string) {
	request := v.surround
	bounds := request.bounds
	if request.linewise && bounds.insideEnd > bounds.insideStart && d.Text[d.ByteOffset(bounds.insideEnd-1):d.ByteOffset(bounds.insideEnd)] == "\n" {
		bounds.insideEnd--
		close += "\n"
	}
	v.clearPending()
	v.VisualMode, v.VisualLine = false, false
	v.replaceSurround(d, bounds, open, close)
}
func (v *Vim) replaceSurround(d *Document, bounds surroundBounds, open, close string) {
	if bounds.insideStart > bounds.insideEnd {
		return
	}
	left, right := d.ByteOffset(bounds.start), d.ByteOffset(bounds.end)
	content := d.Text[d.ByteOffset(bounds.insideStart):d.ByteOffset(bounds.insideEnd)]
	text := d.Text[:left] + open + content + close + d.Text[right:]
	if text == d.Text || len(text) > MaxFileSize {
		d.ClearSelection()
		return
	}
	cursor := d.Cursor
	if cursor >= bounds.end {
		cursor += utf8.RuneCountInString(open+close) - (bounds.insideStart - bounds.start) - (bounds.end - bounds.insideEnd)
	} else if cursor >= bounds.insideStart {
		cursor = bounds.start + utf8.RuneCountInString(open) + min(cursor, bounds.insideEnd) - bounds.insideStart
	} else if cursor >= bounds.start {
		cursor = bounds.start
	}
	d.ClearSelection()
	d.BeginEditGroup()
	d.Edit(text)
	d.Cursor = d.snap(cursor, false)
	d.EndEditGroup()
	v.clamp(d)
}
