package diff

import (
	"strconv"
	"strings"

	"github.com/MoeForge/moego/regexp"
)

type Kind uint8

const (
	Context Kind = iota
	Added
	Deleted
	Header
	HunkHeader
	Fold
)

type Line struct {
	Text     string
	Kind     Kind
	Old, New int
	Hunk     int
}
type Hunk struct {
	header             string
	start, end         int
	oldStart, newStart int
	oldCount, newCount int
}
type Patch struct {
	Raw                  string
	Lines                []Line
	Hunks                []Hunk
	Added, Deleted       int
	NewFile, DeletedFile bool
}

var hunkPattern = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

// HunkStartLine returns the one-based start of the hunk on the new side.
func (p Patch) HunkStartLine(index int) int {
	if index < 0 || index >= len(p.Hunks) {
		return 0
	}
	return max(1, p.Hunks[index].newStart)
}

// MapOldLine maps a one-based source line through the patch. A removed line
// lands at the deletion boundary on the new side.
func (p Patch) MapOldLine(line int) int {
	offset := 0
	for _, pair := range p.Pairs() {
		old, newLine := 0, 0
		if pair.Left != nil {
			old = pair.Left.Old
		}
		if pair.Right != nil {
			newLine = pair.Right.New
		}
		switch {
		case old == line && newLine > 0:
			return newLine
		case old >= line:
			return line + offset
		case old > 0 && newLine > 0:
			offset = newLine - old
		case old > 0:
			offset--
		case newLine > 0 && newLine-offset <= line:
			offset++
		}
	}
	return line + offset
}

func Parse(raw string) Patch {
	p := Patch{Raw: raw}
	old, newLine, hunk := 0, 0, -1
	header := ""
	for _, text := range strings.Split(strings.TrimSuffix(raw, "\n"), "\n") {
		if raw == "" {
			break
		}
		line := Line{Text: text, Kind: Header, Hunk: -1}
		if strings.HasPrefix(text, "diff --git ") {
			header = ""
			hunk = -1
		}
		if match := hunkPattern.FindStringSubmatch(text); match != nil {
			old, _ = strconv.Atoi(match[1])
			newLine, _ = strconv.Atoi(match[3])
			hunk = len(p.Hunks)
			p.Hunks = append(p.Hunks, Hunk{
				header: header, start: len(p.Lines), end: len(p.Lines) + 1,
				oldStart: old, newStart: newLine,
				oldCount: hunkCount(match[2]), newCount: hunkCount(match[4]),
			})
			line.Kind, line.Hunk = HunkHeader, hunk
		} else if hunk >= 0 && len(text) > 0 && strings.ContainsRune(" +-\\", rune(text[0])) {
			line.Hunk = hunk
			switch text[0] {
			case '+':
				line.Kind, line.New = Added, newLine
				newLine++
				p.Added++
			case '-':
				line.Kind, line.Old = Deleted, old
				old++
				p.Deleted++
			case ' ':
				line.Kind, line.Old, line.New = Context, old, newLine
				old++
				newLine++
			}
			p.Hunks[hunk].end++
		} else {
			hunk = -1
			header += text + "\n"
			p.NewFile = p.NewFile || strings.HasPrefix(text, "new file mode ")
			p.DeletedFile = p.DeletedFile || strings.HasPrefix(text, "deleted file mode ")
		}
		p.Lines = append(p.Lines, line)
	}
	return p
}

func hunkCount(count string) int {
	if count == "" {
		return 1
	}
	n, _ := strconv.Atoi(count)
	return n
}

type Pair struct {
	Left, Right *Line
	Hunk        int
}

func (p Patch) Pairs() []Pair {
	var pairs []Pair
	for i := 0; i < len(p.Lines); {
		line := &p.Lines[i]
		if line.Kind != Added && line.Kind != Deleted {
			pairs = append(pairs, Pair{line, line, line.Hunk})
			i++
			continue
		}
		var left, right, notes []*Line
	changes:
		for ; i < len(p.Lines); i++ {
			switch {
			case p.Lines[i].Kind == Deleted:
				left = append(left, &p.Lines[i])
			case p.Lines[i].Kind == Added:
				right = append(right, &p.Lines[i])
			case p.Lines[i].Hunk == line.Hunk && strings.HasPrefix(p.Lines[i].Text, "\\"):
				notes = append(notes, &p.Lines[i])
			default:
				break changes
			}
		}
		for j := 0; j < max(len(left), len(right)); j++ {
			pair := Pair{Hunk: line.Hunk}
			if j < len(left) {
				pair.Left = left[j]
			}
			if j < len(right) {
				pair.Right = right[j]
			}
			pairs = append(pairs, pair)
		}
		for _, note := range notes {
			pairs = append(pairs, Pair{note, note, note.Hunk})
		}
	}
	return pairs
}
