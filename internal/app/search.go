package app

import (
	"strings"
	"unicode/utf8"
)

// searchRanges returns non-overlapping literal matches in rune coordinates.
// Only rows being painted are scanned; cached rows retain their highlighting.
func searchRanges(text, query string) [][2]int {
	if query == "" {
		return nil
	}
	var ranges [][2]int
	byteOffset, runeOffset := 0, 0
	width := utf8.RuneCountInString(query)
	for byteOffset < len(text) {
		index := strings.Index(text[byteOffset:], query)
		if index < 0 {
			break
		}
		runeOffset += utf8.RuneCountInString(text[byteOffset : byteOffset+index])
		ranges = append(ranges, [2]int{runeOffset, runeOffset + width})
		byteOffset += index + len(query)
		runeOffset += width
	}
	return ranges
}
