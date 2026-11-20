package search

import (
	"fmt"
	"strings"

	"github.com/MoeForge/moego/regexp"
)

// Search fields accept comma lists, except commas inside braces or classes.
// Unanchored patterns match any workspace directory; ./ anchors at its root.
func compileGlobs(value string) ([]*regexp.Regexp, error) {
	var patterns []*regexp.Regexp
	depth, class, start := 0, false, 0
	add := func(raw string) error {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return nil
		}
		anchored := strings.HasPrefix(raw, "./") || strings.HasPrefix(raw, "/")
		raw = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(raw, "./"), "/"), "/")
		if raw == "" {
			return fmt.Errorf("empty file pattern")
		}
		body, err := globExpression(raw)
		if err != nil {
			return err
		}
		prefix := "(?:^|.*/)"
		if anchored {
			prefix = "^"
		}
		re, err := regexp.Compile(prefix + body + "(?:/.*)?$")
		if err != nil {
			return fmt.Errorf("invalid file pattern %q: %w", raw, err)
		}
		patterns = append(patterns, re)
		return nil
	}
	for i, r := range value {
		switch {
		case r == '[' && !class:
			class = true
		case r == ']' && class:
			class = false
		case !class && r == '{':
			depth++
		case !class && r == '}':
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("unmatched } in file pattern")
			}
		case !class && depth == 0 && r == ',':
			if err := add(value[start:i]); err != nil {
				return nil, err
			}
			start = i + 1
		}
	}
	if depth != 0 || class {
		return nil, fmt.Errorf("unclosed brace or class in file pattern")
	}
	if err := add(value[start:]); err != nil {
		return nil, err
	}
	return patterns, nil
}
func globExpression(pattern string) (string, error) {
	var b strings.Builder
	depth := 0
	for i := 0; i < len(pattern); i++ {
		switch c := pattern[i]; c {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				i++
				if i+1 < len(pattern) && pattern[i+1] == '/' {
					b.WriteString("(?:.*/)?")
					i++
				} else {
					b.WriteString(".*")
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		case '{':
			depth++
			b.WriteString("(?:")
		case '}':
			depth--
			b.WriteByte(')')
		case ',':
			if depth > 0 {
				b.WriteByte('|')
			} else {
				b.WriteByte(',')
			}
		case '[':
			end := i + 1
			if end < len(pattern) && (pattern[end] == '!' || pattern[end] == '^') {
				end++
			}
			// A leading ] is a literal member, as in []a].
			if end < len(pattern) && pattern[end] == ']' {
				end++
			}
			for end < len(pattern) && pattern[end] != ']' {
				end++
			}
			if end == len(pattern) {
				return "", fmt.Errorf("unclosed class in file pattern %q", pattern)
			}
			content := pattern[i+1 : end]
			b.WriteByte('[')
			if strings.HasPrefix(content, "!") {
				b.WriteString("^/")
				content = content[1:]
			}
			b.WriteString(strings.ReplaceAll(content, "\\", "\\\\"))
			b.WriteByte(']')
			i = end
		default:
			if c >= 128 {
				b.WriteByte(c)
			} else {
				b.WriteString(regexp.QuoteMeta(string(c)))
			}
		}
	}
	return b.String(), nil
}
func matchesGlobs(patterns []*regexp.Regexp, path string) bool {
	for _, pattern := range patterns {
		if pattern.MatchString(path) {
			return true
		}
	}
	return false
}
