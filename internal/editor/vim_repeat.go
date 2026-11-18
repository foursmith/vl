package editor

import (
	"strings"
	"unicode/utf8"

	"github.com/clipperhouse/uax29/v2/graphemes"
)

type vimSelection struct {
	linewise bool
	size     int
}
type vimInsertEdit struct {
	before, remove int
	text           string
}
type vimChange struct {
	keys      []string
	selection *vimSelection
	insert    *vimInsertEdit
	putText   string
	after     bool
	flash     *vimFlashEdit
}
type vimRecording struct {
	before            string
	keys              []string
	selection         *vimSelection
	insertText        string
	insertAnchor      int
	inserting, unsafe bool
	flash             *vimFlashEdit
}

func (r *vimRecording) beginInsert(d *Document) {
	r.inserting = true
	r.insertText = d.Text
	r.insertAnchor = d.Cursor
}

func repeatableInsertKey(key string) bool {
	switch key {
	case "left", "right", "up", "down", "home", "end", "pgup", "pgdown", "ctrl+z", "ctrl+y", "ctrl+shift+z":
		return false
	}
	return !strings.HasPrefix(key, "alt+") && (!strings.HasPrefix(key, "ctrl+") || key == "ctrl+s")
}
func (v *Vim) finishRecording(d *Document) {
	recording := v.record
	v.record = nil
	if recording == nil || recording.unsafe || recording.before == d.Text {
		return
	}
	change := &vimChange{
		keys:      recording.keys,
		selection: recording.selection,
		flash:     recording.flash,
	}
	if recording.inserting {
		change.insert = insertDifference(recording.insertText, d.Text, recording.insertAnchor)
	}
	v.lastChange = change
}
func splitClusters(text string) []string {
	var clusters []string
	iter := graphemes.FromString(text)
	for iter.Next() {
		clusters = append(clusters, iter.Value())
	}
	return clusters
}
func insertDifference(before, after string, anchor int) *vimInsertEdit {
	old, current := splitClusters(before), splitClusters(after)
	anchorIndex, offset := 0, 0
	for anchorIndex < len(old) && offset < anchor {
		offset += utf8.RuneCountInString(old[anchorIndex])
		anchorIndex++
	}
	prefix := 0
	for prefix < anchorIndex && prefix < len(current) && old[prefix] == current[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(old)-prefix && suffix < len(current)-prefix && old[len(old)-suffix-1] == current[len(current)-suffix-1] {
		suffix++
	}
	return &vimInsertEdit{before: anchorIndex - prefix, remove: len(old) - prefix - suffix, text: strings.Join(current[prefix:len(current)-suffix], "")}
}
func (v *Vim) repeatSelection(d *Document) *vimSelection {
	if !v.VisualMode {
		return nil
	}
	start, end := d.Selection()
	shape := &vimSelection{linewise: v.VisualLine}
	if shape.linewise {
		first, _ := d.positionAt(start)
		last, _ := d.positionAt(d.previous(end))
		shape.size = last - first + 1
	} else {
		for pos := start; pos < end; pos = d.next(pos) {
			shape.size++
		}
	}
	return shape
}
func (v *Vim) selectForRepeat(d *Document, shape *vimSelection) {
	if shape == nil {
		return
	}
	v.startVisual(d, shape.linewise)
	if shape.linewise {
		row, col := d.Position()
		d.SetPosition(row+shape.size-1, col)
	} else {
		for i := 1; i < shape.size; i++ {
			d.Cursor = d.next(d.Cursor)
		}
	}
	v.SyncVisual(d)
}
func (v *Vim) repeatChange(d *Document, count int) string {
	change := v.lastChange
	if change == nil {
		return ""
	}
	var register string
	for range count {
		if len(d.Text) >= MaxFileSize {
			break
		}
		if change.insert != nil && len(d.Text)+len(change.insert.text) > MaxFileSize {
			break
		}
		v.selectForRepeat(d, change.selection)
		if change.putText != "" {
			v.put(d, change.putText, change.after)
		} else {
			if change.flash != nil {
				edit := change.flash
				if target, ok := flashEditTarget(d, edit); ok {
					if text := v.applyFlashRange(d, target, edit.operator); text != "" {
						register = text
					}
					if edit.operator == "ys" {
						for _, key := range change.keys {
							v.handleKey(d, key)
						}
					}
				}
			} else {
				for _, key := range change.keys {
					if _, text := v.handleKey(d, key); text != "" {
						register = text
					}
				}
			}
			if v.InsertMode {
				edit := change.insert
				if edit != nil {
					start := d.Cursor
					for range edit.before {
						start = d.previous(start)
					}
					end := start
					for range edit.remove {
						end = d.next(end)
					}
					d.SelectRange(start, end)
					d.Insert(edit.text)
				}
				v.handleKey(d, "esc")
			}
		}
		if v.VisualMode {
			v.Reset(d)
		}
	}
	return register
}
func (v *Vim) putAndRecord(d *Document, text string, after bool) {
	if text == "" {
		return
	}
	before := d.Text
	shape := v.repeatSelection(d)
	v.put(d, text, after)
	if before != d.Text {
		v.lastChange = &vimChange{putText: text, after: after, selection: shape}
	}
}
