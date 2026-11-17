package editor

import "unicode"

// Word objects use document grapheme boundaries, while character classes only
// distinguish whitespace, identifier characters and punctuation.
func wordObject(d *Document, key string, inner bool, count int) (int, int, bool) {
	row, _ := d.Position()
	floor := d.LineOffset(row)
	ceiling := floor + d.LineRuneCount(row)
	if ceiling == floor {
		return 0, 0, false
	}
	runes := []rune(d.Text)
	class := func(pos int) int {
		c := wordClass(runes[pos])
		if key == "W" && c != 0 {
			return 1
		}
		return c
	}
	cursor := d.snap(d.Cursor, false)
	if cursor >= ceiling {
		cursor = d.previous(ceiling)
	}
	kind := class(cursor)
	start, end := cursor, d.next(cursor)
	for start > floor && class(d.previous(start)) == kind {
		start = d.previous(start)
	}
	for end < ceiling && class(end) == kind {
		end = d.next(end)
	}
	// On whitespace, aw includes the following word; iw selects the whitespace.
	if kind == 0 && !inner && end < ceiling {
		nextClass := class(end)
		for end < ceiling && class(end) == nextClass {
			end = d.next(end)
		}
	}
	for i := 1; i < count; i++ {
		next := end
		for next < len(runes) && class(next) == 0 {
			next = d.next(next)
		}
		if next >= len(runes) {
			break
		}
		nextClass := class(next)
		end = d.next(next)
		for end < len(runes) && class(end) == nextClass {
			end = d.next(end)
		}
	}
	if !inner && kind != 0 {
		tail := end
		// Surrounding whitespace stays on its line, so daw does not join lines.
		for tail < len(runes) && runes[tail] != '\n' && unicode.IsSpace(runes[tail]) {
			tail = d.next(tail)
		}
		if tail > end {
			end = tail
		} else {
			for start > floor {
				prev := d.previous(start)
				if !unicode.IsSpace(runes[prev]) {
					break
				}
				start = prev
			}
		}
	}
	return start, end, start < end
}
