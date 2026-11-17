package editor

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/clipperhouse/uax29/v2/graphemes"
)

type surroundPhase uint8

const (
	surroundSource surroundPhase = iota
	surroundRange
	surroundObject
	surroundFirstLine
	surroundTarget
	surroundTag
)

type surroundBounds struct{ start, insideStart, insideEnd, end int }
type vimSurround struct {
	kind                   string
	phase                  surroundPhase
	count, digits          int
	inner, linewise, block bool
	bounds                 surroundBounds
	text                   string
	cursor, anchor         int
	tag                    string
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
	request.block = v.VisualLine
	request.bounds = surroundBounds{start, start, end, end}
}
func (v *Vim) handleSurround(d *Document, key string) bool {
	request := v.surround
	if request.text != d.Text || request.cursor != d.Cursor || request.anchor != d.Anchor {
		if v.record != nil {
			v.record.unsafe = true
		}
		v.clear()
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
		v.clear()
		return false
	}
	switch request.phase {
	case surroundSource:
		bounds, ok := sourceSurround(d, key, request.count)
		if !ok {
			v.clear()
			return true
		}
		request.bounds = bounds
		if request.kind == "d" {
			v.completeSurround(d, "", "", false)
		} else {
			request.phase = surroundTarget
		}
	case surroundRange:
		if len(key) == 1 && key[0] >= '0' && key[0] <= '9' && (key != "0" || request.digits > 0) {
			request.digits = min(9999, request.digits*10+int(key[0]-'0'))
			return true
		}
		count := min(9999, request.count*max(1, request.digits))
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
			request.block = last > row
		default:
			if !v.surroundMotion(d, request, key, count) {
				v.clear()
			}
		}
	case surroundObject:
		start, end, ok := textObject(d, key, request.inner, request.count)
		if !ok {
			v.clear()
			return true
		}
		request.bounds = surroundBounds{start, start, end, end}
		request.phase = surroundTarget
	case surroundFirstLine:
		if key != "g" {
			v.clear()
			return true
		}
		row, _ := d.Position()
		target := min(request.count-1, len(d.Lines())-1)
		first, last := min(row, target), max(row, target)
		start, end := d.LineOffset(first), d.LineOffset(last)+d.LineRuneCount(last)
		request.bounds = surroundBounds{start, start, end, end}
		request.phase = surroundTarget
		request.block = last > first
	case surroundTarget:
		if key == "<" || key == "t" {
			request.phase = surroundTag
			request.tag = "<"
			return true
		}
		open, close, ok := delimiterPair(key)
		if !ok {
			v.clear()
			return true
		}
		v.completeSurround(d, open, close, false)
	case surroundTag:
		switch key {
		case "backspace":
			iter := graphemes.FromString(request.tag)
			end := 0
			for iter.Next() {
				end = iter.Start()
			}
			request.tag = request.tag[:end]
			if request.tag == "" {
				request.phase = surroundTarget
			}
			return true
		case "enter":
			value := request.tag
			if !strings.HasSuffix(value, ">") {
				value += ">"
			}
			if name, ok := surroundTagName(value); ok {
				v.completeSurround(d, value, "</"+name+">", true)
			}
			return true
		case "space":
			key = " "
		}
		if key == "tab" || key == "delete" || key == "left" || key == "right" || key == "up" || key == "down" || key == "home" || key == "end" || key == "pgup" || key == "pgdown" || key == "shift+tab" || strings.IndexFunc(key, unicode.IsControl) >= 0 || len(request.tag)+len(key) > 1024 {
			return true
		}
		request.tag += key
		if strings.HasSuffix(request.tag, ">") {
			if name, ok := surroundTagName(request.tag); ok {
				v.completeSurround(d, request.tag, "</"+name+">", true)
			}
		}
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
		request.block = last > first
		return true
	}
	if !v.motion(d, key, count, true) {
		return false
	}
	end := d.Cursor
	if key == "e" {
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
	if key == "t" {
		return tagSurround(d, count)
	}
	if _, _, ok := delimiterPair(key); !ok {
		return surroundBounds{}, false
	}
	start, end, ok := textObject(d, key, false, count)
	if !ok {
		return surroundBounds{}, false
	}
	return surroundBounds{start, d.next(start), d.previous(end), end}, true
}
func (v *Vim) completeSurround(d *Document, open, close string, tag bool) {
	request := v.surround
	bounds := request.bounds
	if request.linewise && bounds.insideEnd > bounds.insideStart && d.Text[d.ByteOffset(bounds.insideEnd-1):d.ByteOffset(bounds.insideEnd)] == "\n" {
		bounds.insideEnd--
		close += "\n"
	}
	if tag && request.block {
		open += "\n"
		close = "\n" + close
	}
	v.clear()
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
func (v *Vim) PasteSurround(d *Document, text string) bool {
	if v.surround == nil || v.surround.phase != surroundTarget && v.surround.phase != surroundTag {
		return false
	}
	if len(text) > 1024 || strings.IndexFunc(text, unicode.IsControl) >= 0 {
		return true
	}
	iter := graphemes.FromString(text)
	for iter.Next() {
		if v.surround == nil {
			break
		}
		v.Handle(d, iter.Value())
	}
	return true
}
