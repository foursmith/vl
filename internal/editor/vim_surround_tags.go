package editor

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

type surroundTagToken struct {
	start, end    int
	name          string
	closing, self bool
}

func tagNameRune(r rune, first bool) bool {
	return unicode.IsLetter(r) || r == '_' || r == ':' || !first && (unicode.IsDigit(r) || r == '-' || r == '.')
}
func readSurroundTag(text string, start int) (surroundTagToken, bool) {
	token := surroundTagToken{start: start}
	pos := start + 1
	if pos >= len(text) {
		return token, false
	}
	if text[pos] == '/' {
		token.closing = true
		pos++
	}
	nameStart := pos
	first := true
	for pos < len(text) {
		r, size := utf8.DecodeRuneInString(text[pos:])
		if !tagNameRune(r, first) {
			break
		}
		first = false
		pos += size
	}
	if pos == nameStart {
		return token, false
	}
	token.name = text[nameStart:pos]
	if pos < len(text) && text[pos] != '>' && text[pos] != '/' && !unicode.IsSpace(rune(text[pos])) {
		return token, false
	}
	var quote byte
	for ; pos < len(text); pos++ {
		b := text[pos]
		if quote != 0 {
			if b == quote {
				quote = 0
			}
			continue
		}
		if b == '\'' || b == '"' {
			quote = b
			continue
		}
		if b == '<' {
			return token, false
		}
		if b == '>' {
			tail := strings.TrimSpace(text[nameStart+len(token.name) : pos])
			if token.closing && tail != "" {
				return token, false
			}
			token.self = strings.HasSuffix(tail, "/") || htmlVoidTag(token.name)
			token.end = pos + 1
			return token, true
		}
	}
	return token, false
}
func htmlVoidTag(name string) bool {
	switch strings.ToLower(name) {
	case "area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param", "source", "track", "wbr":
		return true
	}
	return false
}
func surroundTagName(text string) (string, bool) {
	if !strings.HasPrefix(text, "<") {
		return "", false
	}
	token, ok := readSurroundTag(text, 0)
	return token.name, ok && token.end == len(text) && !token.closing && !token.self
}
func tagSurround(d *Document, count int) (surroundBounds, bool) {
	text := d.Text
	cursor := d.ByteOffset(d.Cursor)
	row, _ := d.Position()
	lineEnd := d.ByteOffset(d.LineOffset(row) + d.LineRuneCount(row))
	var stack []surroundTagToken
	var enclosing, following []surroundBounds
	raw := ""
	for pos := 0; pos < len(text); {
		if raw != "" {
			offset := strings.Index(text[pos:], "</"+raw)
			if offset < 0 {
				break
			}
			pos += offset
		} else {
			offset := strings.IndexByte(text[pos:], '<')
			if offset < 0 {
				break
			}
			pos += offset
		}
		if strings.HasPrefix(text[pos:], "<!--") {
			end := strings.Index(text[pos+4:], "-->")
			if end < 0 {
				break
			}
			pos += 4 + end + 3
			continue
		}
		if strings.HasPrefix(text[pos:], "<![CDATA[") {
			end := strings.Index(text[pos+9:], "]]>")
			if end < 0 {
				break
			}
			pos += 9 + end + 3
			continue
		}
		if strings.HasPrefix(text[pos:], "<?") {
			end := strings.Index(text[pos+2:], "?>")
			if end < 0 {
				break
			}
			pos += 2 + end + 2
			continue
		}
		if strings.HasPrefix(text[pos:], "<!") {
			depth := 0
			var quote byte
			end := pos + 2
			for ; end < len(text); end++ {
				c := text[end]
				if quote != 0 {
					if c == quote {
						quote = 0
					}
					continue
				}
				if c == '\'' || c == '"' {
					quote = c
					continue
				}
				if c == '[' {
					depth++
				}
				if c == ']' {
					depth = max(0, depth-1)
				}
				if c == '>' && depth == 0 {
					end++
					break
				}
			}
			pos = end
			continue
		}
		token, ok := readSurroundTag(text, pos)
		if !ok {
			pos++
			continue
		}
		pos = token.end
		if raw != "" && (!token.closing || token.name != raw) {
			continue
		}
		if token.closing {
			if len(stack) == 0 || stack[len(stack)-1].name != token.name {
				continue
			}
			open := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			raw = ""
			bounds := surroundBounds{open.start, open.end, token.start, token.end}
			if open.start <= cursor && cursor < token.end {
				enclosing = append(enclosing, bounds)
			}
			if open.start > cursor && open.start < lineEnd {
				following = append(following, bounds)
			}
		} else if !token.self {
			stack = append(stack, token)
			if token.name == "script" || token.name == "style" {
				raw = token.name
			}
		}
	}
	var selected surroundBounds
	if len(enclosing) >= count {
		selected = enclosing[count-1]
	} else if len(enclosing) > 0 || count != 1 || len(following) == 0 {
		return surroundBounds{}, false
	} else {
		selected = following[0]
		for _, candidate := range following[1:] {
			if candidate.start < selected.start {
				selected = candidate
			}
		}
	}
	selected.start = utf8.RuneCountInString(text[:selected.start])
	selected.insideStart = utf8.RuneCountInString(text[:selected.insideStart])
	selected.insideEnd = utf8.RuneCountInString(text[:selected.insideEnd])
	selected.end = utf8.RuneCountInString(text[:selected.end])
	return selected, true
}
