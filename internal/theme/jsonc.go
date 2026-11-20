package theme

import (
	"bytes"
	"fmt"

	"github.com/MoeForge/moego/json"
)

func readJSONC(name string, value any) error {
	data, err := readThemeFile(name)
	if err != nil {
		return err
	}
	return decodeJSONC(data, value)
}

func decodeJSONC(data []byte, value any) error {
	clean, err := jsonc(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(clean, value)
}

// Remove JSONC comments and trailing commas without touching string contents.
// Whitespace preserves line/byte positions for JSON error diagnostics.
func jsonc(input []byte) ([]byte, error) {
	b := bytes.TrimPrefix(input, []byte{0xef, 0xbb, 0xbf})
	b = bytes.Clone(b)
	quoted, escape := false, false
	for i := 0; i < len(b); i++ {
		c := b[i]
		if quoted {
			if escape {
				escape = false
			} else if c == '\\' {
				escape = true
			} else if c == '"' {
				quoted = false
			}
			continue
		}
		if c == '"' {
			quoted = true
			continue
		}
		if c != '/' || i+1 >= len(b) {
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
				return nil, fmt.Errorf("unterminated JSONC comment")
			}
			b[i], b[i+1] = ' ', ' '
			i++
		}
	}
	quoted, escape = false, false
	for i, c := range b {
		if quoted {
			if escape {
				escape = false
			} else if c == '\\' {
				escape = true
			} else if c == '"' {
				quoted = false
			}
			continue
		}
		if c == '"' {
			quoted = true
			continue
		}
		if c == ',' {
			j := i + 1
			for j < len(b) && (b[j] == ' ' || b[j] == '\n' || b[j] == '\r' || b[j] == '\t') {
				j++
			}
			if j < len(b) && (b[j] == '}' || b[j] == ']') {
				b[i] = ' '
			}
		}
	}
	return b, nil
}
