package editor

import "strings"

const (
	defaultIndentWidth     = 4
	maxDetectedIndentWidth = 32
)

// indentUnit remembers the detected style for this open document, so removing
// its last indented line does not change what the next Tab inserts.
func (d *Document) indentUnit() string {
	if d.indentation != "" {
		return d.indentation
	}
	var tabs, spaces, previous int
	var steps [maxDetectedIndentWidth + 1]int
	for _, line := range d.Lines() {
		body := strings.TrimLeft(line, " \t")
		if body == "" {
			continue
		}
		width := len(line) - len(body)
		if strings.ContainsRune(line[:width], '\t') {
			tabs++
			previous = 0
			continue
		}
		if width > 0 {
			spaces++
		}
		if step := max(width-previous, previous-width); step > 0 && step < len(steps) {
			steps[step]++
		}
		previous = width
	}
	if tabs == 0 && spaces == 0 {
		return "\t"
	}
	d.indentation = "\t"
	if spaces > tabs {
		width, best := defaultIndentWidth, 0
		for step, count := range steps {
			if count > best {
				width, best = step, count
			}
		}
		d.indentation = strings.Repeat(" ", width)
	}
	return d.indentation
}

// InsertIndent inserts one indentation level at the cursor.
func (d *Document) InsertIndent() {
	if !d.ReadOnly {
		d.Insert(d.indentUnit())
	}
}

// Outdent removes one indentation level from the cursor line.
func (d *Document) Outdent() {
	if d.ReadOnly {
		return
	}
	row, col := d.Position()
	removed := outdentWidth(d.Lines()[row], d.indentUnit())
	if removed > 0 {
		start := d.byteOffsets[row]
		d.Edit(d.Text[:start] + d.Text[start+removed:])
		d.SetPosition(row, col-removed)
	}
}

func outdentWidth(line, unit string) int {
	if strings.HasPrefix(line, "\t") {
		return 1
	}
	width := len(unit)
	if unit == "\t" {
		width = defaultIndentWidth
	}
	removed := 0
	for removed < len(line) && removed < width && line[removed] == ' ' {
		removed++
	}
	return removed
}
