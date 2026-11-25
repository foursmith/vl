package editor

import "unicode"

type wordBoundaryMode uint8

const (
	vimWord wordBoundaryMode = iota
	vimWORD
	whitespaceWord
)

// wordClassifier identifies the word at a rune offset. Zero denotes whitespace,
// and each segmented Han word gets its own class within the buffer.
func (d *Buffer) wordClassifier(dictionary func() *HanDictionary, mode wordBoundaryMode) func(int) int {
	return func(pos int) int {
		r := d.wordRuneAt(pos)
		if mode == vimWORD {
			if wordClass(r) == 0 {
				return 0
			}
			return 1
		}
		if dictionary != nil && unicode.Is(unicode.Han, r) {
			if dict := dictionary(); dict != nil {
				return d.hanWordRun.class(dict, d.Text, pos, d.wordByteOffset(pos))
			}
		}
		if mode == whitespaceWord {
			if unicode.IsSpace(r) {
				return 0
			}
			return 1
		}
		return wordClass(r)
	}
}

func (d *Buffer) adjacentWordStart(forward bool, dictionary func() *HanDictionary) int {
	classify := d.wordClassifier(dictionary, whitespaceWord)
	pos := d.Cursor
	if forward {
		if class := classify(pos); class != 0 && pos < d.runeCount {
			pos++
			for pos < d.runeCount && classify(pos) == class {
				pos++
			}
		}
		for pos < d.runeCount && unicode.IsSpace(d.wordRuneAt(pos)) {
			pos++
		}
		return pos
	}
	for pos > 0 && unicode.IsSpace(d.wordRuneAt(pos-1)) {
		pos--
	}
	if pos == 0 {
		return 0
	}
	pos--
	class := classify(pos)
	for pos > 0 && classify(pos-1) == class {
		pos--
	}
	return pos
}

// InnerWordRange returns the Vim word under a rune offset without moving the
// cursor. An offset at the end of a line has no word under it.
func (d *Buffer) InnerWordRange(offset int, dictionary func() *HanDictionary) (start, end int) {
	row, _ := d.positionAt(offset)
	floor := d.LineOffset(row)
	ceiling := floor + d.LineRuneCount(row)
	if offset >= ceiling {
		return ceiling, ceiling
	}
	start, end, _ = wordClassRange(d, d.snap(offset, false), floor, ceiling, d.wordClassifier(dictionary, vimWord))
	return start, end
}

func wordClassRange(d *Buffer, cursor, floor, ceiling int, class func(int) int) (start, end, kind int) {
	kind = class(cursor)
	start, end = cursor, d.next(cursor)
	for start > floor {
		prev := d.previous(start)
		if class(prev) != kind {
			break
		}
		start = prev
	}
	for end < ceiling && class(end) == kind {
		end = d.next(end)
	}
	return start, end, kind
}
