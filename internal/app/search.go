package app

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/ourongxing/vl/internal/editor"
	searchservice "github.com/ourongxing/vl/internal/search"
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

func (m *Model) fileSearchMatcher() *searchservice.Matcher {
	o := m.findOptions
	if o.Query == "" || o.Case && !o.Word && !o.Regex {
		return nil
	}
	if m.findMatcherOptions != o {
		m.findMatcherOptions = o
		m.findMatcher, _ = o.Compile()
	}
	return m.findMatcher
}
func matchedSearchRanges(text, query string, matcher *searchservice.Matcher) [][2]int {
	if query == "" {
		return nil
	}
	if matcher == nil {
		return searchRanges(text, query)
	}
	var ranges [][2]int
	byteOffset, runeOffset := 0, 0
	for _, hit := range matcher.Find(text, "") {
		runeOffset += utf8.RuneCountInString(text[byteOffset:hit.Start])
		end := runeOffset + utf8.RuneCountInString(text[hit.Start:hit.End])
		ranges = append(ranges, [2]int{runeOffset, end})
		byteOffset, runeOffset = hit.End, end
	}
	return ranges
}

type findRangeCache struct {
	text    string
	options searchservice.Options
	ranges  [][2]int
	err     string
}

// Ordinary literal searches scan only visible rows. Regex searches cache whole
// document ranges so matches spanning lines use the same results as navigation.
func (m *Model) fileSearchRowRanges(d *editor.Document, line int) [][2]int {
	if m.findHidden || m.findOptions.Query == "" {
		return nil
	}
	options := m.findOptions
	if !options.Regex {
		return matchedSearchRanges(d.Lines()[line], m.findOptions.Query, m.fileSearchMatcher())
	}
	cache := m.fileFindCache()
	start := d.LineOffset(line)
	end := start + d.LineRuneCount(line)
	index := sort.Search(len(cache.ranges), func(i int) bool { return cache.ranges[i][1] > start })
	var result [][2]int
	for ; index < len(cache.ranges) && cache.ranges[index][0] < end; index++ {
		hit := cache.ranges[index]
		if hit[0] == hit[1] {
			continue
		}
		result = append(result, [2]int{max(start, hit[0]) - start, min(end, hit[1]) - start})
	}
	return result
}
