package diff

import (
	"fmt"
	"strings"
)

func (p Patch) PartialStagingError() error {
	if len(p.Hunks) == 0 {
		return fmt.Errorf("No text changes to stage partially")
	}
	header := p.Hunks[0].header
	if strings.Contains(header, "\nrename from ") || strings.Contains(header, "\ncopy from ") ||
		strings.Contains(header, " 160000\n") || strings.Contains(header, " 120000\n") {
		return fmt.Errorf("Stage this file as a whole")
	}
	for _, h := range p.Hunks[1:] {
		if h.header != header {
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
	pairs := p.Pairs()
	for index, h := range p.Hunks {
		for len(pairs) > 0 && pairs[0].Hunk < index {
			pairs = pairs[1:]
		}
		end := 0
		for end < len(pairs) && pairs[end].Hunk == index {
			end++
		}
		lines, oldCount, newCount, edits := stagingHunk(pairs[:end], reverse, selected)
		pairs = pairs[end:]
		if edits == 0 {
			continue
		}
		sourceStart, sourceCount := h.oldStart, h.oldCount
		if reverse {
			sourceStart, sourceCount = h.newStart, h.newCount
		}
		start := hunkBefore(sourceStart, sourceCount)
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
		for _, line := range strings.Split(p.Hunks[0].header, "\n") {
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

type stagingEdit struct {
	marker byte
	line   *Line
}

func stagingHunk(pairs []Pair, reverse bool, selected func(*Line) bool) (string, int, int, int) {
	var result, removals, additions []stagingEdit
	oldCount, newCount, edits := 0, 0, 0
	appendEdit := func(out *[]stagingEdit, marker byte, line *Line) {
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
	source, target := Deleted, Added
	if reverse {
		source, target = target, source
	}
	// A one-sided insertion belongs between the same source lines as in the
	// displayed alignment, even when the neighboring edits are skipped.
	for _, pair := range pairs {
		before, after := pair.Left, pair.Right
		if reverse {
			before, after = after, before
		}
		if before != nil {
			switch {
			case before.Kind == source && selected(before):
				appendEdit(&removals, '-', before)
			case before.Kind == source || before.Kind == Context:
				flushEdits()
				appendEdit(&result, ' ', before)
			}
		}
		if after != nil && after.Kind == target && selected(after) {
			appendEdit(&additions, '+', after)
		}
	}
	flushEdits()
	return renderStagingEdits(result), oldCount, newCount, edits
}

func renderStagingEdits(edits []stagingEdit) string {
	lastTarget := len(edits) - 1
	for lastTarget >= 0 && edits[lastTarget].marker == '-' {
		lastTarget--
	}
	var result strings.Builder
	write := func(edit stagingEdit, noNewline bool) {
		result.WriteByte(edit.marker)
		result.WriteString(edit.line.Text[1:] + "\n")
		if noNewline {
			result.WriteString("\\ No newline at end of file\n")
		}
	}
	for i, edit := range edits {
		noNewline := edit.line.noNewline
		// A line without a terminator can only be the last output line.
		// Keep the source bytes when context becomes an interior line.
		if noNewline && i < lastTarget && edit.marker != '-' {
			if edit.marker == ' ' {
				edit.marker = '-'
				write(edit, true)
				edit.marker = '+'
			}
			noNewline = false
		}
		write(edit, noNewline)
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
