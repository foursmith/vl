package search

import (
	"crypto/sha256"
	"os"
	"strings"
	"unicode/utf8"
)

type previewLine struct {
	text       string
	start      int
	fullDigest *[sha256.Size]byte
}

type filePreview struct {
	info  os.FileInfo
	lines map[int]previewLine
}

// Preview hits carry line-relative byte ranges, just as ripgrep reports them.
func newFilePreview(path string, info os.FileInfo, records []ripgrepLine) (File, bool) {
	preview := &filePreview{info: info, lines: make(map[int]previewLine, len(records))}
	file := File{Path: path, preview: preview}
	lastLine := 0
	for _, record := range records {
		if record.Number <= lastLine || len(record.Spans) == 0 {
			return File{}, false
		}
		lastLine = record.Number
		line := record.Text
		if withoutLF, ok := strings.CutSuffix(line, "\n"); ok {
			line = strings.TrimSuffix(withoutLF, "\r")
		}
		bom := 0
		if record.Number == 1 && strings.HasPrefix(line, "\uFEFF") {
			bom = len("\uFEFF")
			line = line[bom:]
		}
		if !utf8.ValidString(line) || strings.IndexByte(line, 0) >= 0 {
			return File{}, false
		}
		endOfPrevious := 0
		for _, span := range record.Spans {
			start, end := span.Start-bom, span.End-bom
			if start < endOfPrevious || start >= end || end > len(line) ||
				!utf8.RuneStart(line[start]) || end < len(line) && !utf8.RuneStart(line[end]) {
				return File{}, false
			}
			file.Matches = append(file.Matches, Match{Start: start, End: end, Line: record.Number})
			endOfPrevious = end
		}
		const previewBytes = 10_000
		if len(line) <= previewBytes {
			preview.lines[record.Number] = previewLine{text: line}
			continue
		}
		first := record.Spans[0].Start - bom
		start, end := max(0, first-previewBytes/5), min(len(line), first+previewBytes)
		for !utf8.RuneStart(line[start]) {
			start++
		}
		for end < len(line) && !utf8.RuneStart(line[end]) {
			end--
		}
		digest := sha256.Sum256([]byte(line))
		preview.lines[record.Number] = previewLine{text: strings.Clone(line[start:end]), start: start, fullDigest: &digest}
	}
	return file, true
}

// ResolveMatches verifies previews against the opened document and maps them
// to its normalized text before jumping to or replacing a result.
func (f File) ResolveMatches(text string) ([]Match, bool) {
	if f.preview == nil {
		return f.Matches, f.Text == text
	}
	current, err := os.Lstat(f.Path)
	if err != nil || !current.Mode().IsRegular() || !os.SameFile(f.preview.info, current) ||
		f.preview.info.Size() != current.Size() || !f.preview.info.ModTime().Equal(current.ModTime()) {
		return nil, false
	}
	resolved := make([]Match, 0, len(f.Matches))
	offset, lineNumber, verifiedLine := 0, 1, 0
	for _, hit := range f.Matches {
		for lineNumber < hit.Line {
			next := strings.IndexByte(text[offset:], '\n')
			if next == -1 {
				return nil, false
			}
			offset += next + 1
			lineNumber++
		}
		if verifiedLine != lineNumber {
			line, _, _ := strings.Cut(text[offset:], "\n")
			preview := f.preview.lines[lineNumber]
			if preview.fullDigest != nil && sha256.Sum256([]byte(line)) != *preview.fullDigest ||
				preview.fullDigest == nil && line != preview.text {
				return nil, false
			}
			verifiedLine = lineNumber
		}
		hit.Start += offset
		hit.End += offset
		resolved = append(resolved, hit)
	}
	return resolved, true
}

// PreviewLine returns the displayed line and its offset in the match's range.
func (f File) PreviewLine(index int) (string, int) {
	hit := f.Matches[index]
	if f.preview != nil {
		line := f.preview.lines[hit.Line]
		return line.text, line.start
	}
	start := strings.LastIndexByte(f.Text[:hit.Start], '\n') + 1
	line, _, _ := strings.Cut(f.Text[start:], "\n")
	return line, start
}
