package editor

func (v *Vim) textObject(d *Document, key string, inner bool, count int) (int, int, bool) {
	if key == "w" || key == "W" {
		return v.wordObject(d, key, inner, count)
	}
	open, close, ok := delimiterPair(key)
	if !ok {
		return 0, 0, false
	}
	return delimiterObject(d, open, close, inner, count)
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
	start, end, _ = wordClassRange(d, d.snap(offset, false), floor, ceiling, d.wordClassifier(dictionary, false))
	return start, end
}

func wordClassRange(d *Buffer, cursor, floor, ceiling int, class func(int) int) (start, end, kind int) {
	kind = class(cursor)
	start, end = cursor, d.next(cursor)
	for start > floor && class(d.previous(start)) == kind {
		start = d.previous(start)
	}
	for end < ceiling && class(end) == kind {
		end = d.next(end)
	}
	return start, end, kind
}

// Word objects use the same character classes as word motions and keep
// document graphemes intact.
func (v *Vim) wordObject(d *Document, key string, inner bool, count int) (int, int, bool) {
	row, _ := d.Position()
	floor := d.LineOffset(row)
	ceiling := floor + d.LineRuneCount(row)
	if ceiling == floor {
		return 0, 0, false
	}
	class := d.wordClassifier(v.HanDictionary, key == "W")
	cursor := d.snap(d.Cursor, false)
	if cursor >= ceiling {
		cursor = d.previous(ceiling)
	}
	start, end, kind := wordClassRange(&d.Buffer, cursor, floor, ceiling, class)
	// On whitespace, aw includes the following word; iw selects the whitespace.
	if kind == 0 && !inner && end < ceiling {
		nextClass := class(end)
		for end < ceiling && class(end) == nextClass {
			end = d.next(end)
		}
	}
	for i := 1; i < count; i++ {
		next := end
		for next < d.runeCount && class(next) == 0 {
			next = d.next(next)
		}
		if next >= d.runeCount {
			break
		}
		nextClass := class(next)
		end = d.next(next)
		for end < d.runeCount && class(end) == nextClass {
			end = d.next(end)
		}
	}
	if !inner && kind != 0 {
		tail := end
		// Surrounding whitespace stays on its line, so daw does not join lines.
		for tail < d.runeCount {
			r := d.wordRuneAt(tail)
			if r == '\n' || wordClass(r) != 0 {
				break
			}
			tail = d.next(tail)
		}
		if tail > end {
			end = tail
		} else {
			for start > floor {
				prev := d.previous(start)
				if wordClass(d.wordRuneAt(prev)) != 0 {
					break
				}
				start = prev
			}
		}
	}
	return start, end, start < end
}
