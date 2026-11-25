package jsonc

import (
	"bytes"
	"errors"

	"github.com/MoeForge/moego/json"
)

func Unmarshal(data []byte, value any) error {
	clean, _, err := normalize(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(clean, value)
}

// Remove JSONC comments and trailing commas without touching string contents.
// Whitespace preserves line/byte positions for JSON error diagnostics.
func normalize(input []byte) ([]byte, []int, error) {
	b := bytes.Clone(input)
	if bytes.HasPrefix(b, []byte{0xef, 0xbb, 0xbf}) {
		copy(b, "   ")
	}
	for i := 0; i < len(b); i++ {
		if b[i] == '"' {
			i = stringEnd(b, i)
			continue
		}
		if b[i] != '/' || i+1 >= len(b) {
			continue
		}
		switch b[i+1] {
		case '/':
			b[i], b[i+1] = ' ', ' '
			i += 2
			for i < len(b) && b[i] != '\n' && b[i] != '\r' {
				b[i] = ' '
				i++
			}
			i--
		case '*':
			b[i], b[i+1] = ' ', ' '
			i += 2
			for i+1 < len(b) && (b[i] != '*' || b[i+1] != '/') {
				if b[i] != '\n' && b[i] != '\r' {
					b[i] = ' '
				}
				i++
			}
			if i+1 >= len(b) {
				return nil, nil, errors.New("unterminated JSONC comment")
			}
			b[i], b[i+1] = ' ', ' '
			i++
		}
	}
	var trailingCommas []int
	for i := 0; i < len(b); i++ {
		switch b[i] {
		case '"':
			i = stringEnd(b, i)
		case ',':
			j := skipSpace(b, i+1)
			if j == len(b) || b[j] != '}' && b[j] != ']' {
				continue
			}
			previous := bytes.TrimRight(b[:i], " \t\r\n")
			if len(previous) > 0 && !bytes.ContainsAny(previous[len(previous)-1:], "{[,") {
				b[i] = ' '
				trailingCommas = append(trailingCommas, i)
			}
		}
	}
	return b, trailingCommas, nil
}

func stringEnd(data []byte, start int) int {
	for i := start + 1; i < len(data); i++ {
		switch data[i] {
		case '\\':
			i++
		case '"':
			return i
		}
	}
	return len(data) - 1 // The JSON decoder reports unterminated strings.
}

func skipSpace(data []byte, offset int) int {
	for offset < len(data) {
		switch data[offset] {
		case ' ', '\n', '\r', '\t':
			offset++
		default:
			return offset
		}
	}
	return offset
}
