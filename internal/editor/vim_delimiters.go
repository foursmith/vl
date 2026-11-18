package editor

func delimiterPair(key string) (string, string, bool) {
	switch key {
	case "(", ")", "b":
		return "(", ")", true
	case "[", "]":
		return "[", "]", true
	case "{", "}", "B":
		return "{", "}", true
	case "<", ">":
		return "<", ">", true
	case "\"", "'", "`":
		return key, key, true
	default:
		return "", "", false
	}
}

func delimiterObject(d *Document, opening, closing string, inner bool, count int) (int, int, bool) {
	open, close := rune(opening[0]), rune(closing[0])
	runes := []rune(d.Text)
	row, _ := d.Position()
	floor := d.LineOffset(row)
	ceiling := floor + d.LineRuneCount(row)
	if open == close {
		quote := open
		open := -1
		for pos := floor; pos < ceiling; pos++ {
			if runes[pos] == '\\' {
				pos++
				continue
			}
			if runes[pos] != quote {
				continue
			}
			if open < 0 {
				open = pos
				continue
			}
			if pos >= d.Cursor {
				return delimiterRange(d, open, pos, inner)
			}
			open = -1
		}
		return 0, 0, false
	}
	type pair struct{ start, end int }
	var stack []int
	var enclosing []pair
	following := pair{start: -1}
	var quote rune
	escaped := false
	for pos, r := range runes {
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == quote || r == '\n' && quote != '`' {
				quote = 0
			}
			continue
		}
		if r == '"' || r == '\'' || r == '`' {
			quote = r
			continue
		}
		if r == open {
			stack = append(stack, pos)
		}
		if r == close && len(stack) > 0 {
			start := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if start <= d.Cursor && d.Cursor <= pos {
				enclosing = append(enclosing, pair{start, pos})
			}
			if start > d.Cursor && start < ceiling && (following.start < 0 || start < following.start) {
				following = pair{start, pos}
			}
		}
	}
	if len(enclosing) >= count {
		p := enclosing[count-1]
		return delimiterRange(d, p.start, p.end, inner)
	}
	if len(enclosing) > 0 {
		return 0, 0, false
	}
	// A delimiter ahead on the same line is useful for calls such as ci(.
	if following.start >= 0 && count == 1 {
		return delimiterRange(d, following.start, following.end, inner)
	}
	return 0, 0, false
}
func delimiterRange(d *Document, open, close int, inner bool) (int, int, bool) {
	if inner {
		return d.next(open), close, true
	}
	return open, d.next(close), true
}
