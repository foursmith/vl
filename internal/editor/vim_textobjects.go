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

// Word objects use the same character classes as word motions and keep
// document graphemes intact.
func (v *Vim) wordObject(d *Document, key string, inner bool, count int) (int, int, bool) {
	row, _ := d.Position()
	floor := d.LineOffset(row)
	ceiling := floor + d.LineRuneCount(row)
	if ceiling == floor {
		return 0, 0, false
	}
	mode := vimWord
	if key == "W" {
		mode = vimWORD
	}
	class := d.wordClassifier(v.HanDictionary, mode)
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
