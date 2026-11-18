package diff

import (
	"fmt"
	"strconv"
	"strings"
)

func (p Patch) PartialStagingError() error {
	if len(p.Hunks) == 0 {
		return fmt.Errorf("No text changes to stage partially")
	}
	header := p.Hunks[0].Header
	if strings.Contains(header, "\nrename from ") || strings.Contains(header, "\ncopy from ") ||
		strings.Contains(header, " 160000\n") || strings.Contains(header, " 120000\n") {
		return fmt.Errorf("Stage this file as a whole")
	}
	for _, h := range p.Hunks[1:] {
		if h.Header != header {
			return fmt.Errorf("Stage this file as a whole")
		}
	}
	return nil
}

// StagingPatch transforms the displayed diff into a text-only patch. In
// either direction, skipped source lines remain as context and skipped target
// lines are omitted. Recompute offsets using only the emitted changes.
func (p Patch) StagingPatch(path string, reverse bool, selected func(*Line) bool) (string, error) {
	if err := p.PartialStagingError(); err != nil {
		return "", err
	}
	var body strings.Builder
	delta, applied := 0, 0
	for _, h := range p.Hunks {
		match := hunkPattern.FindStringSubmatch(h.Lines[0])
		source := 1
		if reverse {
			source = 3
		}
		start, _ := strconv.Atoi(match[source])
		if match[source+1] != "0" {
			start--
		}

		lines, oldCount, newCount, edits := p.stagingHunk(h, reverse, selected)
		if edits == 0 {
			continue
		}
		fmt.Fprintf(&body, "@@ -%s +%s @@\n", stagingRange(start, oldCount), stagingRange(start+delta, newCount))
		body.WriteString(lines)
		delta += newCount - oldCount
		applied += edits
	}
	if applied == 0 {
		return "", fmt.Errorf("Select changed lines first")
	}
	a, b := quotePatchPath("a/"+path), quotePatchPath("b/"+path)
	oldPath, newPath := a, b
	metadata := ""
	sourceMissing := p.NewFile && !reverse || p.DeletedFile && reverse
	targetMissing := (p.DeletedFile && !reverse || p.NewFile && reverse) && applied == p.Added+p.Deleted
	if sourceMissing || targetMissing {
		modePrefix := "new file mode "
		if p.DeletedFile {
			modePrefix = "deleted file mode "
		}
		for _, line := range strings.Split(p.Hunks[0].Header, "\n") {
			if mode, ok := strings.CutPrefix(line, modePrefix); ok {
				if sourceMissing {
					metadata, oldPath = "new file mode "+mode+"\n", "/dev/null"
				} else {
					metadata, newPath = "deleted file mode "+mode+"\n", "/dev/null"
				}
				break
			}
		}
	}
	return fmt.Sprintf("diff --git %s %s\n%s--- %s\n+++ %s\n%s", a, b, metadata, oldPath, newPath, body.String()), nil
}

type stagingLine struct {
	line      *Line
	noNewline bool
}

type stagingEdit struct {
	marker byte
	stagingLine
}

func (p Patch) stagingHunk(h Hunk, reverse bool, selected func(*Line) bool) (string, int, int, int) {
	var result, removals, additions []stagingEdit
	oldCount, newCount, edits := 0, 0, 0
	appendEdit := func(out *[]stagingEdit, marker byte, line stagingLine) {
		*out = append(*out, stagingEdit{marker, line})
		if marker != '+' {
			oldCount++
		}
		if marker != '-' {
			newCount++
		}
		if marker != ' ' {
			edits++
		}
	}
	flushEdits := func() {
		result = append(result, removals...)
		result = append(result, additions...)
		removals, additions = removals[:0], additions[:0]
	}
	var before, after []stagingLine
	flushGroup := func() {
		for i := range max(len(before), len(after)) {
			if i < len(before) {
				if selected(before[i].line) {
					appendEdit(&removals, '-', before[i])
				} else {
					flushEdits()
					appendEdit(&result, ' ', before[i])
				}
			}
			if i < len(after) && selected(after[i].line) {
				appendEdit(&additions, '+', after[i])
			}
		}
		flushEdits()
		before, after = before[:0], after[:0]
	}
	lines := p.Lines[h.start+1 : h.start+len(h.Lines)]
	source, target := Deleted, Added
	if reverse {
		source, target = target, source
	}
	for i := range lines {
		line := stagingLine{line: &lines[i], noNewline: i+1 < len(lines) && strings.HasPrefix(lines[i+1].Text, "\\")}
		switch line.line.Kind {
		case Context:
			flushGroup()
			appendEdit(&result, ' ', line)
		case source:
			before = append(before, line)
		case target:
			after = append(after, line)
		}
	}
	flushGroup()
	return renderStagingEdits(result), oldCount, newCount, edits
}

func renderStagingEdits(edits []stagingEdit) string {
	lastTarget := len(edits) - 1
	for lastTarget >= 0 && edits[lastTarget].marker == '-' {
		lastTarget--
	}
	var result strings.Builder
	write := func(edit stagingEdit) {
		result.WriteByte(edit.marker)
		result.WriteString(edit.line.Text[1:] + "\n")
		if edit.noNewline {
			result.WriteString("\\ No newline at end of file\n")
		}
	}
	for i, edit := range edits {
		// A line without a terminator can only be the last output line.
		// Keep the source bytes when context becomes an interior line.
		if edit.noNewline && i < lastTarget && edit.marker != '-' {
			if edit.marker == ' ' {
				edit.marker = '-'
				write(edit)
				edit.marker = '+'
			}
			edit.noNewline = false
		}
		write(edit)
	}
	return result.String()
}

func stagingRange(start, count int) string {
	if count > 0 {
		start++
	}
	return fmt.Sprintf("%d,%d", start, count)
}

func quotePatchPath(path string) string {
	var result strings.Builder
	result.WriteByte('"')
	for i := 0; i < len(path); i++ {
		b := path[i]
		switch {
		case b == '"' || b == '\\':
			result.WriteByte('\\')
			result.WriteByte(b)
		case b < ' ' || b > '~':
			fmt.Fprintf(&result, "\\%03o", b)
		default:
			result.WriteByte(b)
		}
	}
	result.WriteByte('"')
	return result.String()
}
