package diff

import (
	"unicode"
	"unicode/utf8"

	"github.com/clipperhouse/uax29/v2/graphemes"
)

// Range holds half-open rune offsets into a line's text.
type Range struct{ Start, End int }

type inlineToken struct {
	text string
	Range
}

// InlineChanges compares words and punctuation, then refines similar words
// down to whole graphemes.
func InlineChanges(oldText, newText string) (oldChanges, newChanges []Range) {
	if oldText == newText {
		return nil, nil
	}
	compareInlineTokens(inlineTokens(oldText, 0, true), inlineTokens(newText, 0, true), func(old, next []inlineToken) {
		if len(old) == 1 && len(next) == 1 {
			if deleted, added, ok := refineInlineToken(old[0], next[0]); ok {
				oldChanges = append(oldChanges, deleted...)
				newChanges = append(newChanges, added...)
				return
			}
		}
		oldChanges = appendInlineRange(oldChanges, old)
		newChanges = appendInlineRange(newChanges, next)
	})
	return oldChanges, newChanges
}

func refineInlineToken(old, next inlineToken) (deleted, added []Range, ok bool) {
	if old.End-old.Start > maxAlignmentRunes || next.End-next.Start > maxAlignmentRunes {
		return nil, nil, false
	}
	before, _ := utf8.DecodeRuneInString(old.text)
	after, _ := utf8.DecodeRuneInString(next.text)
	class := classifyInline(before)
	if class == inlinePunctuation || class != classifyInline(after) {
		return nil, nil, false
	}
	a, b := inlineTokens(old.text, old.Start, false), inlineTokens(next.text, next.Start, false)
	matches := compareInlineTokens(a, b, func(old, next []inlineToken) {
		deleted = appendInlineRange(deleted, old)
		added = appendInlineRange(added, next)
	})
	// Avoid leaving incidental matching letters unmarked in unrelated words.
	minimum := 55
	if class == inlineSpace || unicode.IsDigit(before) && unicode.IsDigit(after) {
		minimum = 50
	}
	return deleted, added, 200*matches >= minimum*(len(a)+len(b))
}

func compareInlineTokens(old, next []inlineToken, change func(old, next []inlineToken)) int {
	matched := 0
	for len(old) > 0 && len(next) > 0 && old[0].text == next[0].text {
		old, next = old[1:], next[1:]
		matched++
	}
	for len(old) > 0 && len(next) > 0 && old[len(old)-1].text == next[len(next)-1].text {
		old, next = old[:len(old)-1], next[:len(next)-1]
		matched++
	}
	if len(old) == 0 || len(next) == 0 || len(old) > (maxAlignmentRunes*maxAlignmentRunes)/len(next) {
		change(old, next)
		return matched
	}

	scores := scoreInlineTokens(old, next)
	i, j, oldStart, newStart := 0, 0, 0, 0
	for i < len(old) && j < len(next) {
		switch {
		case old[i].text == next[j].text:
			change(old[oldStart:i], next[newStart:j])
			i, j = i+1, j+1
			oldStart, newStart = i, j
		case scores.at(i+1, j) >= scores.at(i, j+1):
			i++
		default:
			j++
		}
	}
	change(old[oldStart:], next[newStart:])
	return matched + scores.at(0, 0)
}

type inlineScores struct {
	width     int
	grid      []int
	oldShared []int
	newShared []int
}

func (s inlineScores) at(i, j int) int {
	if s.grid != nil {
		return s.grid[i*s.width+j]
	}
	// Both tails come from the same shared sequence, so the shorter tail
	// is a subsequence of the longer one.
	return min(s.oldShared[i], s.newShared[j])
}

func scoreInlineTokens(old, next []inlineToken) inlineScores {
	if len(old)*len(next) >= 4096 {
		if a, b := sharedInlineSuffixCounts(old, next); a != nil {
			return inlineScores{oldShared: a, newShared: b}
		}
	}
	width := len(next) + 1
	common := make([]int, (len(old)+1)*width)
	for i := len(old) - 1; i >= 0; i-- {
		for j := len(next) - 1; j >= 0; j-- {
			index := i*width + j
			if old[i].text == next[j].text {
				common[index] = 1 + common[index+width+1]
			} else {
				common[index] = max(common[index+width], common[index+1])
			}
		}
	}
	return inlineScores{width: width, grid: common}
}

// Elements present on just one side cannot be part of a match. If the
// remaining sequences agree, use their suffix lengths as exact LCS scores
// while walking the original tokens to keep the same highlight boundaries.
func sharedInlineSuffixCounts(old, next []inlineToken) ([]int, []int) {
	shared := make(map[string]bool)
	for _, token := range old {
		shared[token.text] = false
	}
	for _, token := range next {
		if _, exists := shared[token.text]; exists {
			shared[token.text] = true
		}
	}
	counts := func(tokens []inlineToken) []int {
		result := make([]int, len(tokens)+1)
		for i := len(tokens) - 1; i >= 0; i-- {
			result[i] = result[i+1]
			if shared[tokens[i].text] {
				result[i]++
			}
		}
		return result
	}
	a, b := counts(old), counts(next)
	if a[0] != b[0] || a[0] == len(old) && b[0] == len(next) {
		return nil, nil
	}
	j := len(next) - 1
	for i := len(old) - 1; i >= 0; i-- {
		if a[i] == a[i+1] {
			continue
		}
		for b[j] == b[j+1] {
			j--
		}
		if old[i].text != next[j].text {
			return nil, nil
		}
		j--
	}
	return a, b
}

func appendInlineRange(ranges []Range, tokens []inlineToken) []Range {
	if len(tokens) != 0 {
		ranges = append(ranges, Range{tokens[0].Start, tokens[len(tokens)-1].End})
	}
	return ranges
}

func inlineTokens(text string, runeOffset int, groupWords bool) []inlineToken {
	var tokens []inlineToken
	var byteOffset int
	var previous inlineClass
	iter := graphemes.FromString(text)
	for iter.Next() {
		cluster := iter.Value()
		class := inlinePunctuation
		if groupWords {
			first, _ := utf8.DecodeRuneInString(cluster)
			class = classifyInline(first)
		}
		end := runeOffset + utf8.RuneCountInString(cluster)
		if class == inlinePunctuation || class != previous {
			tokens = append(tokens, inlineToken{cluster, Range{runeOffset, end}})
			byteOffset = iter.Start()
		} else {
			last := &tokens[len(tokens)-1]
			last.text = text[byteOffset:iter.End()]
			last.End = end
		}
		runeOffset, previous = end, class
	}
	return tokens
}

func classifyInline(char rune) inlineClass {
	switch {
	case unicode.IsSpace(char):
		return inlineSpace
	case char == '_' || unicode.IsDigit(char) || unicode.IsLetter(char) && !unicode.Is(unicode.Han, char):
		return inlineWord
	default:
		return inlinePunctuation
	}
}

type inlineClass uint8

const (
	inlinePunctuation inlineClass = iota
	inlineSpace
	inlineWord
)
