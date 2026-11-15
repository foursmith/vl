package app

import (
	"sort"

	searchservice "github.com/ourongxing/elegit/internal/search"
)

// Cache match positions independently of syntax and viewport rendering.
type editorSearchCache struct {
	text    string
	options searchservice.Options
	matcher *searchservice.Matcher
	matches []searchservice.Match
}

func (m *Model) editorSearchOptions() searchservice.Options {
	if m.section == search && m.search.err == "" {
		return m.search.options
	}
	return searchservice.Options{}
}
func (c *editorSearchCache) ranges(text string, options searchservice.Options) []searchservice.Match {
	changed := c.options != options
	if changed {
		c.options = options
		c.matcher, _ = options.Compile()
	}
	if changed || c.text != text {
		c.text = text
		c.matches = nil
		if options.Query != "" && c.matcher != nil {
			c.matches = c.matcher.Find(text, "")
		}
	}
	return c.matches
}
func editorSearchStart(matches []searchservice.Match, offset int) int {
	return sort.Search(len(matches), func(i int) bool { return matches[i].End > offset })
}
