package editor

import (
	"path/filepath"
	"strings"

	"github.com/foursmith/vl/internal/highlight"
)

type commentFormat struct{ prefix, suffix string }

func blockCommentFormats(language string) []commentFormat {
	switch language {
	case "actionscript-3", "bicep", "c", "c3", "chapel", "cpp", "csharp", "css", "d",
		"dart", "gleam", "go", "groovy", "haxe", "hcl", "hlsl", "java", "javascript",
		"json", "jsonc", "jsonnet", "jsx", "kdl", "kotlin", "less", "objective-c",
		"objective-cpp", "odin", "openscad", "php", "plsql", "powerquery", "proto",
		"qml", "rust", "sas", "sass", "scala", "scss", "solidity", "sql", "stylus",
		"swift", "system-verilog", "terraform", "tsx", "typescript", "typst", "v",
		"vala", "verilog", "wgsl":
		return []commentFormat{{"/*", "*/"}}
	case "coq", "fsharp", "ocaml":
		return []commentFormat{{"(*", "*)"}}
	case "elm", "haskell":
		return []commentFormat{{"{-", "-}"}}
	case "lean":
		return []commentFormat{{"/-", "-/"}}
	case "lua":
		return []commentFormat{{"--[[", "]]"}}
	case "julia":
		return []commentFormat{{"#=", "=#"}}
	case "powershell":
		return []commentFormat{{"<#", "#>"}}
	case "angular-html", "html", "html-derivative", "markdown", "svelte", "vue", "xml":
		return []commentFormat{{"<!--", "-->"}}
	case "handlebars":
		return []commentFormat{{"{{!--", "--}}"}}
	case "jinja", "twig":
		return []commentFormat{{"{#", "#}"}}
	case "pascal":
		return []commentFormat{{"{", "}"}, {"(*", "*)"}}
	case "smalltalk":
		return []commentFormat{{"\"", "\""}}
	}
	return nil
}

func lineCommentPrefix(d *Document, language string) string {
	name := filepath.Base(d.Path)
	if name == "go.work" {
		return "//"
	}
	switch language {
	case "actionscript-3", "ballerina", "bicep", "c", "c3", "chapel", "cpp", "csharp",
		"d", "dart", "fsharp", "gleam", "go", "groovy", "haxe", "hlsl", "java", "javascript",
		"json", "jsonc", "jsonnet", "jsx", "kdl", "kotlin", "less", "moonbit", "objective-c", "objective-cpp",
		"odin", "openscad", "pascal", "php", "powerquery", "proto", "qml", "rust", "sass", "scala", "scss",
		"solidity", "stylus", "swift", "system-verilog", "tsx", "typescript", "typst", "v",
		"vala", "verilog", "wgsl", "zig":
		return "//"
	case "awk", "cmake", "coffee", "crystal", "desktop", "docker", "dotenv", "elixir",
		"fish", "gdscript", "gherkin", "graphql", "hcl", "http", "julia", "make", "mojo",
		"nix", "nushell", "perl", "powershell", "puppet",
		"python", "r", "ruby", "shellscript", "terraform", "toml", "yaml":
		return "#"
	case "elm", "haskell", "lean", "lua", "plsql", "sql", "vhdl":
		return "--"
	case "ahk", "clojure", "common-lisp", "fennel", "hy", "ini", "reg", "scheme", "systemd":
		return ";"
	case "asm":
		switch strings.ToLower(filepath.Ext(name)) {
		case ".asm", ".nasm", ".yasm":
			return ";"
		}
	case "bat":
		return "REM"
	case "vb":
		return "'"
	case "erlang", "tex":
		return "%"
	case "gomod":
		if name == "go.mod" {
			return "//"
		}
	case "wasm":
		return ";;"
	case "viml":
		for line := range strings.SplitSeq(d.Text, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "\"") {
				continue
			}
			// Keep a commented Vim9 declaration recognizable for the next toggle.
			line = strings.TrimSpace(strings.TrimPrefix(line, "#"))
			fields := strings.Fields(line)
			if len(fields) > 0 && (fields[0] == "vim9script" || fields[0] == "vim9") {
				return "#"
			}
			break
		}
		return "\""
	}
	return ""
}

func (v *Vim) commentMotion(d *Document, key string, count int, hadCount bool) {
	row, _ := d.Position()
	switch key {
	case "c", "_":
		toggleComments(d, row, row+count-1)
	case "i", "a", "g":
		v.beginPending("gc"+key, count)
	case "u":
		uncommentAdjacent(d)
	default:
		motion := motionKey(key)
		if motion == "G" && !hadCount {
			count = len(d.Lines())
		}
		cursor := d.Cursor
		if v.motion(d, motion, count) {
			last, _ := d.Position()
			if motion == "w" && last > row {
				line := d.Lines()[last]
				body := strings.TrimLeft(line, " \t")
				if d.ByteOffset(d.Cursor) == d.byteOffsets[last]+len(line)-len(body) {
					last--
				}
			}
			d.Cursor = cursor
			toggleComments(d, min(row, last), max(row, last))
		}
	}
}

func (v *Vim) commentTarget(d *Document, op, key string, count int) {
	row, _ := d.Position()
	if op == "gcg" {
		switch key {
		case "g":
			target := min(count-1, len(d.Lines())-1)
			toggleComments(d, min(row, target), max(row, target))
		case "c":
			uncommentAdjacent(d)
		}
		return
	}
	if key == "p" {
		lines := d.Lines()
		first, last := row, row
		for first > 0 && strings.TrimSpace(lines[first]) != "" && strings.TrimSpace(lines[first-1]) != "" {
			first--
		}
		for range count {
			for last+1 < len(lines) && strings.TrimSpace(lines[last+1]) != "" {
				last++
			}
			for last+1 < len(lines) && strings.TrimSpace(lines[last+1]) == "" {
				last++
			}
			if last+1 == len(lines) {
				break
			}
		}
		toggleComments(d, first, last)
		return
	}
	if start, end, ok := v.textObject(d, key, op == "gci", count); ok && start < end {
		first, _ := d.positionAt(start)
		last, _ := d.positionAt(d.previous(end))
		toggleComments(d, first, last)
	}
}

func uncommentAdjacent(d *Document) {
	language := highlight.Language(d.Path).WithShebang(d.Text).ID
	lines := d.Lines()
	row, _ := d.Position()
	prefix := lineCommentPrefix(d, language)
	formats := blockCommentFormats(language)
	if uncommentBlock(d, row, row, formats) || (prefix == "" && len(formats) == 0) {
		return
	}
	commented := func(i int) bool {
		body := strings.TrimLeft(lines[i], " \t")
		_, ok := stripCommentLine(body, prefix, language, formats)
		return body == "" || ok && !(i == 0 && strings.HasPrefix(lines[i], "#!"))
	}
	if !commented(row) {
		return
	}
	first, last := row, row
	for first > 0 && commented(first-1) {
		first--
	}
	for last+1 < len(lines) && commented(last+1) {
		last++
	}
	uncommentLines(d, first, last, prefix, language, formats)
}

// ToggleComments toggles the comments on the selection or current line.
func (d *Document) ToggleComments() {
	start, end := d.Selection()
	first, _ := d.positionAt(start)
	last := first
	if end > start {
		last, _ = d.positionAt(d.previous(end))
	}
	before := d.Text
	toggleComments(d, first, last)
	if row, _ := d.Position(); d.Text != before && row > last {
		d.SetPosition(last, d.LineRuneCount(last))
	}
}

func toggleComments(d *Document, first, last int) {
	if d.ReadOnly {
		return
	}
	lines := d.Lines()
	last = min(last, len(lines)-1)
	if first == 0 && strings.HasPrefix(lines[0], "#!") {
		first++
	}
	if first > last {
		return
	}
	for first < last && strings.TrimSpace(lines[first]) == "" {
		first++
	}
	for last > first && strings.TrimSpace(lines[last]) == "" {
		last--
	}
	language := highlight.Language(d.Path).WithShebang(d.Text).ID
	formats := blockCommentFormats(language)
	if uncommentBlock(d, first, last, formats) {
		return
	}
	prefix := lineCommentPrefix(d, language)
	if (prefix != "" || len(formats) > 0) && uncommentLines(d, first, last, prefix, language, formats) {
		return
	}
	if len(formats) > 0 && (first < last || prefix == "") {
		addBlockComment(d, first, last, formats[0])
	} else if prefix != "" {
		addLineComments(d, first, last, prefix, formats)
	}
}

func addBlockComment(d *Document, first, last int, format commentFormat) {
	lines := d.Lines()
	if strings.TrimSpace(lines[first]) == "" {
		return
	}
	for _, line := range lines[first : last+1] {
		if strings.Contains(line, format.prefix) || strings.Contains(line, format.suffix) {
			return
		}
	}
	changed := append([]string(nil), lines[first:last+1]...)
	indent, body := splitCommentIndent(changed[0])
	changed[0] = indent + format.prefix + " " + body
	tail := strings.TrimRight(changed[last-first], " \t")
	changed[last-first] = tail + " " + format.suffix + changed[last-first][len(tail):]
	replaceCommentLines(d, first, changed)
}

func uncommentBlock(d *Document, first, last int, formats []commentFormat) bool {
	lines := d.Lines()
	for _, format := range formats {
		start := -1
		nested := false
		for row, line := range lines {
			if start < 0 {
				if row > first {
					break
				}
				body := strings.TrimLeft(line, " \t")
				rest, ok := strings.CutPrefix(body, format.prefix)
				if !ok {
					continue
				}
				start, line = row, rest
			}
			before, after, closed := strings.Cut(line, format.suffix)
			nested = nested || strings.Contains(before, format.prefix)
			if !closed {
				continue
			}
			if row > start && row >= last && strings.TrimSpace(after) == "" {
				if nested {
					return true
				}
				changed := append([]string(nil), lines[start:row+1]...)
				indent, opening := splitCommentIndent(changed[0])
				changed[0] = indent + strings.TrimPrefix(opening[len(format.prefix):], " ")
				changed[row-start] = strings.TrimSuffix(before, " ") + after
				replaceCommentLines(d, start, changed)
				return true
			}
			start, nested = -1, false
		}
	}
	return false
}

func uncommentLines(d *Document, first, last int, prefix, language string, formats []commentFormat) bool {
	changed := append([]string(nil), d.Lines()[first:last+1]...)
	for i, line := range changed {
		indent, body := splitCommentIndent(line)
		if body == "" {
			continue
		}
		inner, ok := stripCommentLine(body, prefix, language, formats)
		if !ok {
			return false
		}
		changed[i] = indent + inner
	}
	replaceCommentLines(d, first, changed)
	return true
}

func addLineComments(d *Document, first, last int, prefix string, formats []commentFormat) {
	lines := d.Lines()[first : last+1]
	firstIndent, _ := splitCommentIndent(lines[0])
	changed := append([]string(nil), lines...)
	for i, line := range lines {
		indent, body := splitCommentIndent(line)
		if body == "" {
			continue
		}
		if strings.HasPrefix(indent, firstIndent) {
			indent = firstIndent
		}
		if startsOverlappingBlockComment(body, prefix, formats) {
			return
		}
		body = line[len(indent):]
		// CMD can still interpret shell syntax and expansions on a REM line.
		if prefix == "REM" && strings.ContainsAny(body, "&|<>()^%!") {
			return
		}
		changed[i] = indent + prefix + " " + body
	}
	replaceCommentLines(d, first, changed)
}

func splitCommentIndent(line string) (indent, body string) {
	body = strings.TrimLeft(line, " \t")
	return line[:len(line)-len(body)], body
}

func replaceCommentLines(d *Document, first int, changed []string) {
	row, col := d.Position()
	last := first + len(changed) - 1
	start, end := d.byteOffsets[first], d.byteOffsets[last]+len(d.Lines()[last])
	text := d.Text[:start] + strings.Join(changed, "\n") + d.Text[end:]
	if d.EncodedSize(text) > MaxFileSize {
		return
	}
	d.Edit(text)
	d.SetPosition(row, col)
}

func stripLineComment(body, prefix, language string) (string, bool) {
	if (language == "hcl" || language == "terraform") && strings.HasPrefix(body, "//") {
		prefix = "//"
	} else if language == "php" && strings.HasPrefix(body, "#") {
		prefix = "#"
	}
	inner, ok := strings.CutPrefix(body, prefix)
	if prefix == "REM" && len(body) >= len(prefix) && strings.EqualFold(body[:len(prefix)], prefix) {
		inner = body[len(prefix):]
		ok = inner == "" || inner[0] == ' ' || inner[0] == '\t'
	}
	if !ok {
		return "", false
	}
	return strings.TrimPrefix(inner, " "), true
}

func stripCommentLine(body, prefix, language string, formats []commentFormat) (string, bool) {
	if prefix != "" && !startsOverlappingBlockComment(body, prefix, formats) {
		if inner, ok := stripLineComment(body, prefix, language); ok {
			return inner, true
		}
	}
	for _, format := range formats {
		inner, ok := strings.CutPrefix(body, format.prefix)
		if !ok {
			continue
		}
		trimmed := strings.TrimRight(inner, " \t")
		content, ok := strings.CutSuffix(trimmed, format.suffix)
		if ok && !strings.Contains(content, format.suffix) && !strings.Contains(content, format.prefix) {
			return strings.TrimPrefix(strings.TrimSuffix(content, " "), " ") + inner[len(trimmed):], true
		}
	}
	return "", false
}

func startsOverlappingBlockComment(body, prefix string, formats []commentFormat) bool {
	for _, format := range formats {
		if strings.HasPrefix(format.prefix, prefix) && strings.HasPrefix(body, format.prefix) {
			return true
		}
	}
	return false
}
