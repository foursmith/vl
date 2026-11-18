package editor

import (
	"strings"
	"unicode/utf8"

	"github.com/clipperhouse/uax29/v2/graphemes"
)

type vimFlashEdit struct {
	operator, query string
	forward         bool
	count           int
}

func (v *Vim) FlashOperator() (string, bool) {
	if v.InsertMode {
		return "", false
	}
	if v.surround != nil {
		if v.surround.kind == "y" && v.surround.phase == surroundRange {
			return "ys", true
		}
		return "", false
	}
	switch v.pending {
	case "", "d", "c", "y":
		return v.pending, true
	}
	return "", false
}
func (v *Vim) ApplyFlash(d *Document, target int, query, operator string) string {
	if operator == "" {
		v.JumpTo(d, target)
		return ""
	}
	edit := &vimFlashEdit{operator: operator, query: query, forward: target >= d.Cursor}
	for _, position := range flashTextPositions(d, query) {
		if position >= min(d.Cursor, target) && position <= max(d.Cursor, target) {
			edit.count++
		}
	}
	edit.count = max(1, edit.count)
	v.CancelPending()
	v.record = &vimRecording{before: d.Text, flash: edit}
	copied := v.applyFlashRange(d, target, operator)
	if v.surround != nil {
		return copied
	}
	if v.InsertMode {
		v.record.beginInsert(d)
	} else {
		v.finishRecording(d)
	}
	return copied
}
func (v *Vim) applyFlashRange(d *Document, target int, operator string) string {
	target = d.snap(target, false)
	low, high := min(d.Cursor, target), d.next(max(d.Cursor, target))
	if operator == "ys" {
		v.beginSurround(d, "y", 1)
		v.surround.phase = surroundTarget
		v.surround.bounds = surroundBounds{low, low, high, high}
		return ""
	}
	copied := v.operate(d, operator, low, high, false)
	v.syncCursor(d)
	return copied
}
func flashTextPositions(d *Document, query string) []int {
	if query == "" {
		return nil
	}
	iter := graphemes.FromString(d.Text)
	offset := 0
	var positions []int
	for iter.Next() {
		tail := d.Text[iter.Start():]
		if strings.HasPrefix(tail, query) {
			end := graphemes.FromString(tail)
			for end.Next() {
				if end.End() >= len(query) {
					if end.End() == len(query) {
						positions = append(positions, offset)
					}
					break
				}
			}
		}
		offset += utf8.RuneCountInString(iter.Value())
	}
	return positions
}
func flashEditTarget(d *Document, edit *vimFlashEdit) (int, bool) {
	positions := flashTextPositions(d, edit.query)
	remaining := edit.count
	if edit.forward {
		for _, position := range positions {
			if position >= d.Cursor {
				remaining--
				if remaining == 0 {
					return position, true
				}
			}
		}
	} else {
		for i := len(positions) - 1; i >= 0; i-- {
			if positions[i] <= d.Cursor {
				remaining--
				if remaining == 0 {
					return positions[i], true
				}
			}
		}
	}
	return 0, false
}
