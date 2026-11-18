package diff

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type Kind uint8

const (
	Context Kind = iota
	Added
	Deleted
	Header
	HunkHeader
)

type Line struct {
	Text     string
	Kind     Kind
	Old, New int
	Hunk     int
}
type Hunk struct {
	Header string
	Lines  []string
	start  int
}
type Patch struct {
	Raw                  string
	Lines                []Line
	Hunks                []Hunk
	Added, Deleted       int
	NewFile, DeletedFile bool
}

var hunkPattern = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

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
			p.Hunks = append(p.Hunks, Hunk{Header: header, Lines: []string{text}, start: len(p.Lines)})
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
			p.Hunks[hunk].Lines = append(p.Hunks[hunk].Lines, text)
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

func (p Patch) HunkPatch(index int) (string, error) {
	if index < 0 || index >= len(p.Hunks) {
		return "", fmt.Errorf("select a diff hunk first")
	}
	h := p.Hunks[index]
	if strings.Contains(h.Header, " 160000\n") {
		return "", fmt.Errorf("stage submodule pointers as a whole file")
	}
	if strings.Contains(h.Header, "rename from ") || strings.Contains(h.Header, "copy from ") || strings.Contains(h.Header, "new file mode ") || strings.Contains(h.Header, "deleted file mode ") {
		return "", fmt.Errorf("stage added, deleted, copied or renamed files as a whole")
	}
	return h.Header + strings.Join(h.Lines, "\n") + "\n", nil
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
