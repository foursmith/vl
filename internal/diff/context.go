package diff

import "strings"

// ContextBase identifies the old blob of a single-file patch with text hunks.
func ContextBase(raw string) string {
	header, _, ok := strings.Cut(raw, "\n@@ ")
	if !ok || !strings.HasPrefix(header, "diff --git ") || strings.Contains(raw, "\ndiff --git ") {
		return ""
	}
	for line := range strings.SplitSeq(header, "\n") {
		value, ok := strings.CutPrefix(line, "index ")
		if !ok {
			continue
		}
		id, _, ok := strings.Cut(value, "..")
		if !ok || len(id) < 4 || len(id) > 64 || strings.Trim(id, "0") == "" {
			break
		}
		if strings.Trim(id, "0123456789abcdef") == "" {
			return id
		}
		break
	}
	return ""
}

type ContextGap struct {
	Old, New, Count int
}

// ContextGaps returns the omitted ranges before each hunk and after the last
// hunk. Line numbers on both sides are one-based.
func (p Patch) ContextGaps(oldLength int) []ContextGap {
	if len(p.Hunks) == 0 || p.NewFile || p.DeletedFile {
		return nil
	}
	gaps := make([]ContextGap, len(p.Hunks)+1)
	oldEnd, newEnd := 0, 0
	for i, h := range p.Hunks {
		oldBefore := hunkBefore(h.oldStart, h.oldCount)
		newBefore := hunkBefore(h.newStart, h.newCount)
		count := oldBefore - oldEnd
		if count < 0 || count != newBefore-newEnd || oldBefore > oldLength || h.oldCount > oldLength-oldBefore {
			return nil
		}
		gaps[i] = ContextGap{oldEnd + 1, newEnd + 1, count}
		oldEnd, newEnd = oldBefore+h.oldCount, newBefore+h.newCount
	}
	gaps[len(p.Hunks)] = ContextGap{oldEnd + 1, newEnd + 1, oldLength - oldEnd}
	return gaps
}

func hunkBefore(start, count int) int {
	if count == 0 {
		return start
	}
	return start - 1
}
