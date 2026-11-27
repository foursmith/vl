package diff

import (
	"strings"
	"unicode"
)

const (
	maxAlignmentPairs = 4096
	maxAlignmentRunes = 256
	alignmentBudget   = 4_000_000
	minLineSimilarity = 60
)

type alignmentStep uint8

type alignmentCell struct {
	score int
	step  alignmentStep
}

const (
	takeDeletion alignmentStep = iota
	takeInsertion
	takePair
)

func alignChanges(out []Pair, left, right []*Line, hunk int, budget *int) []Pair {
	width := len(right) + 1
	var cells []alignmentCell
	if len(left) > 0 && len(right) > 0 && len(left) <= maxAlignmentPairs/len(right) && *budget > 0 {
		old, oldSize := alignmentText(left)
		newText, newSize := alignmentText(right)
		// Bound the work per patch, including the cost of the line comparisons.
		if cost := oldSize * newSize; cost <= *budget {
			*budget -= cost
			cells = make([]alignmentCell, (len(left)+1)*width)
			for i := len(left) - 1; i >= 0; i-- {
				for j := len(right) - 1; j >= 0; j-- {
					index := i*width + j
					cell := &cells[index]
					cell.score = cells[index+width].score
					if score := cells[index+1].score; score > cell.score {
						cell.score, cell.step = score, takeInsertion
					}
					similarity := lineSimilarity(old[i], newText[j])
					// Give weak matches less weight when the alignments compete.
					if score := cells[index+width+1].score + similarity - 50; similarity >= minLineSimilarity && score >= cell.score {
						cell.score, cell.step = score, takePair
					}
				}
			}
		}
	}

	for i, j := 0, 0; i < len(left) || j < len(right); {
		step := takeDeletion
		switch {
		case i == len(left):
			step = takeInsertion
		case j < len(right) && cells != nil:
			step = cells[i*width+j].step
		}
		pair := Pair{Hunk: hunk}
		if step != takeInsertion {
			pair.Left = left[i]
			i++
		}
		if step != takeDeletion {
			pair.Right = right[j]
			j++
		}
		out = append(out, pair)
	}
	return out
}

func alignmentText(lines []*Line) ([][]rune, int) {
	text := make([][]rune, len(lines))
	size := len(lines)
	for i, line := range lines {
		code := strings.TrimSpace(line.Text[1:])
		if len(code) <= 4*maxAlignmentRunes {
			if chars := []rune(code); len(chars) <= maxAlignmentRunes {
				for j, char := range chars {
					chars[j] = unicode.ToLower(char)
				}
				text[i] = chars
				size += len(chars)
			}
		}
	}
	return text, size
}

func lineSimilarity(a, b []rune) int {
	if a == nil || b == nil {
		return 0
	}
	total := len(a) + len(b)
	if total == 0 {
		return 100
	}
	if 200*min(len(a), len(b)) < minLineSimilarity*total {
		return 0
	}
	if len(b) > len(a) {
		a, b = b, a
	}
	// One row is enough to find the length of the common subsequence.
	var row [maxAlignmentRunes + 1]int
	for _, char := range a {
		diagonal := 0
		for j, other := range b {
			above := row[j+1]
			if char == other {
				row[j+1] = diagonal + 1
			} else {
				row[j+1] = max(above, row[j])
			}
			diagonal = above
		}
	}
	return 200 * row[len(b)] / total
}
