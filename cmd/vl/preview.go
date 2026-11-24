package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/config"
	"github.com/foursmith/vl/internal/highlight"
	"github.com/foursmith/vl/internal/platform"
	"github.com/foursmith/vl/internal/theme"
)

type previewOptions struct {
	skip, lines, width, tabSize int
	lineNumbers, wrap           bool
	colors                      *theme.Theme
}

// Keep reads bounded even for minified or generated files with very long lines.
const previewLineBytes = 32 << 10
const previewContextLines = 64

const previewUsage = `Usage:
  vl preview [options] file

Options:
  -h, --help       Show this help
  --skip N         Skip N source lines
  --lines N        Print up to N source lines (0 prints through EOF)
  --width N        Limit each row to N columns, including line numbers (0 is unlimited)
  --tab-size N     Use N spaces per tab (1–32, default: 4)
  --line-numbers   Show source line numbers
  --wrap           Wrap long lines at --width
  --color MODE     Use auto, always, or never (default: auto)
  --theme ID       Use a syntax theme (default: saved theme)
`

func runPreview(args []string, stdout, stderr io.Writer) int {
	var opt previewOptions
	flags := flag.NewFlagSet("preview", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.IntVar(&opt.skip, "skip", 0, "")
	flags.IntVar(&opt.lines, "lines", 0, "")
	flags.IntVar(&opt.width, "width", 0, "")
	flags.IntVar(&opt.tabSize, "tab-size", 4, "")
	flags.BoolVar(&opt.lineNumbers, "line-numbers", false, "")
	flags.BoolVar(&opt.wrap, "wrap", false, "")
	color := flags.String("color", "auto", "")
	themeName := flags.String("theme", "", "")
	flags.Usage = func() {
		fmt.Fprint(stderr, previewUsage)
	}
	if err := parseFlags(flags, args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() != 1 || opt.skip < 0 || opt.lines < 0 || opt.width < 0 || opt.tabSize < 1 || opt.tabSize > 32 {
		flags.Usage()
		return 2
	}
	if opt.wrap && opt.width < 2 {
		fmt.Fprintln(stderr, "vl preview: --wrap requires --width of at least 2")
		return 2
	}

	var profile colorprofile.Profile
	switch *color {
	case "auto":
		if os.Getenv("NO_COLOR") == "" {
			profile = colorprofile.Detect(stdout, os.Environ())
		}
	case "always":
		profile = colorprofile.TrueColor
	case "never":
	default:
		fmt.Fprintln(stderr, "vl preview: --color must be auto, always, or never")
		return 2
	}
	if profile >= colorprofile.ANSI {
		var err error
		opt.colors, err = loadPreviewTheme(*themeName)
		if err != nil {
			fmt.Fprintln(stderr, "vl preview:", err)
			return 2
		}
	}

	path := flags.Arg(0)
	out := bufio.NewWriterSize(stdout, 16<<10)
	var output io.Writer = out
	if profile >= colorprofile.ANSI && profile < colorprofile.TrueColor {
		output = &colorprofile.Writer{Forward: out, Profile: profile}
	}
	if err := writePreview(output, path, opt); err != nil {
		fmt.Fprintln(stderr, "vl preview:", err)
		return 1
	}
	if err := out.Flush(); err != nil {
		fmt.Fprintln(stderr, "vl preview:", err)
		return 1
	}
	return 0
}

func loadPreviewTheme(override string) (*theme.Theme, error) {
	if override != "" {
		return theme.Load(override)
	}
	light, _ := platform.SystemAppearance(context.Background())
	fallback := theme.DefaultDarkID
	if light {
		fallback = theme.DefaultLightID
	}
	if path, err := config.LayoutPath(); err == nil {
		if layout, err := config.LoadLayout(path); err == nil {
			id := layout.Appearance.Theme.Dark
			if light {
				id = layout.Appearance.Theme.Light
			}
			if id != "" {
				if selected, err := theme.Load(id); err == nil {
					return selected, nil
				}
			}
		}
	}
	return theme.Builtin(fallback), nil
}

func previewLanguage(path string, firstLine []byte) highlight.LanguageInfo {
	if record, err := config.FileSyntaxPath(path); err == nil {
		if id, err := config.LoadFileSyntax(record); err == nil && id != "" {
			if language := highlight.LanguageByID(id); language.ID != "" {
				return language
			}
		}
	}
	language := highlight.Language(path)
	firstLine = bytes.TrimPrefix(firstLine, []byte("\xef\xbb\xbf"))
	if bytes.HasPrefix(firstLine, []byte("#!")) {
		language = language.WithShebang(string(firstLine))
	}
	return language
}

func writePreview(out io.Writer, path string, opt previewOptions) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%q is not a regular file", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := bufio.NewReaderSize(file, previewLineBytes+2) // room for CRLF
	tabs := strings.Repeat(" ", opt.tabSize)
	digits := 0
	if opt.lineNumbers {
		lastLine := uint64(opt.skip) + uint64(opt.lines)
		digits = max(3, len(strconv.FormatUint(lastLine, 10)))
	}
	contextStart := opt.skip
	if opt.colors != nil {
		contextStart = max(0, opt.skip-previewContextLines)
	}
	var syntax *highlight.Preview
	var language highlight.LanguageInfo
	styles := make(map[previewStyle]string)
	plain := opt.colors == nil && opt.width == 0 && !opt.lineNumbers
	for index := 0; opt.lines == 0 || index-opt.skip < opt.lines; index++ {
		line, err := reader.ReadSlice('\n')
		if len(line) == 0 && err == io.EOF {
			break
		}
		if index == 0 && opt.colors != nil {
			language = previewLanguage(path, line)
		}
		shortened := err == bufio.ErrBufferFull
		if index >= contextStart {
			line = bytes.TrimSuffix(bytes.TrimSuffix(line, []byte("\n")), []byte("\r"))
			shortened = shortened || len(line) > previewLineBytes
			line = line[:min(len(line), previewLineBytes)]
			if index == 0 {
				line = bytes.TrimPrefix(line, []byte("\xef\xbb\xbf"))
			}
		}
		// ReadSlice's buffer is reused when draining an oversized line.
		var source string
		if err == bufio.ErrBufferFull && index >= contextStart {
			source = string(line)
		}
		for err == bufio.ErrBufferFull {
			_, err = reader.ReadSlice('\n')
		}
		if err != nil && err != io.EOF {
			return err
		}
		if index < contextStart {
			continue
		}
		if plain && !shortened && cleanPreviewBytes(line) {
			if _, err := out.Write(line); err != nil {
				return err
			}
			if _, err := io.WriteString(out, "\n"); err != nil {
				return err
			}
			continue
		}
		if source == "" {
			source = string(line)
		}
		source = strings.ToValidUTF8(source, "\ufffd")
		visible := index >= opt.skip
		var spans []highlight.Span
		if index == contextStart && opt.colors != nil {
			syntax = highlight.NewPreview(language, opt.colors)
		}
		if syntax != nil && !shortened {
			spans = syntax.Line(source, visible)
		}
		if !visible {
			continue
		}
		lineText := previewLine(source, tabs, spans, styles)
		if shortened {
			lineText += "…"
		}
		if err := opt.writeLine(out, lineText, index+1, digits); err != nil {
			return err
		}
	}
	return nil
}

func (opt previewOptions) writeLine(out io.Writer, line string, number, digits int) error {
	gutter := ""
	if digits > 0 {
		label := strconv.Itoa(number)
		labelWidth := max(digits, len(label))
		if opt.width > 0 {
			contentWidth := 1
			if opt.wrap {
				contentWidth = 2
			}
			labelWidth = min(labelWidth, opt.width-2-contentWidth)
		}
		if labelWidth >= len(label) {
			gutter = strings.Repeat(" ", labelWidth-len(label)) + label + "  "
		}
	}

	if width := opt.width - len(gutter); opt.width > 0 && len(line) > width {
		if opt.wrap {
			line = ansi.Hardwrap(line, width, true)
			if strings.Contains(line, "\n") {
				if opt.colors != nil {
					var styled strings.Builder
					writer := lipgloss.NewWrapWriter(&styled)
					_, _ = io.WriteString(writer, line)
					_ = writer.Close()
					line = styled.String()
				}
				line = strings.ReplaceAll(line, "\n", "\n"+strings.Repeat(" ", len(gutter)))
			}
		} else {
			line = ansi.Truncate(line, width, "")
		}
	}
	if opt.colors != nil && gutter != "" {
		gutter = lipgloss.NewStyle().Foreground(lipgloss.Color(opt.colors.Palette().LineNumber)).Render(gutter)
	}
	_, err := io.WriteString(out, gutter+line+"\n")
	return err
}

type previewStyle struct {
	fg, bg string
	font   theme.FontStyle
}

func (s previewStyle) lipglossStyle() lipgloss.Style {
	style := s.font.Apply(lipgloss.NewStyle())
	if s.fg != "" {
		style = style.Foreground(lipgloss.Color(s.fg))
	}
	if s.bg != "" {
		style = style.Background(lipgloss.Color(s.bg))
	}
	return style
}

func previewLine(source, tabs string, spans []highlight.Span, styles map[previewStyle]string) string {
	if len(spans) == 0 {
		return previewText(source, tabs)
	}
	var line strings.Builder
	line.Grow(len(source))
	column, offset := 0, 0
	take := func(end int) string {
		start := offset
		for column < end {
			_, size := utf8.DecodeRuneInString(source[offset:])
			offset += size
			column++
		}
		return previewText(source[start:offset], tabs)
	}
	for _, span := range spans {
		line.WriteString(take(span.Start))
		style := previewStyle{span.Color, span.Background, span.Font}
		text := take(span.End)
		if span.Font&(theme.Underline|theme.Strikethrough) != 0 {
			// Lipgloss applies these decorations per grapheme.
			line.WriteString(style.lipglossStyle().Render(text))
			continue
		}
		prefix, ok := styles[style]
		if !ok {
			// For ordinary single-line runs only the style determines the SGR prefix.
			prefix, ok = strings.CutSuffix(style.lipglossStyle().Render("x"), "x"+ansi.ResetStyle)
			if !ok {
				prefix = ""
			}
			styles[style] = prefix
		}
		line.WriteString(prefix)
		line.WriteString(text)
		if prefix != "" {
			line.WriteString(ansi.ResetStyle)
		}
	}
	line.WriteString(previewText(source[offset:], tabs))
	return line.String()
}

func cleanPreviewBytes(line []byte) bool {
	for i := 0; i < len(line); {
		b := line[i]
		if b < ' ' || b == 0x7f {
			return false
		}
		if b < utf8.RuneSelf {
			i++
			continue
		}
		r, size := utf8.DecodeRune(line[i:])
		// Non-ASCII controls are C1. RuneError can represent an invalid byte.
		if r <= 0x9f || r == utf8.RuneError {
			return false
		}
		i += size
	}
	return true
}

func previewText(text, tabs string) string {
	if strings.IndexFunc(text, unicode.IsControl) < 0 {
		return text
	}
	var clean strings.Builder
	for _, r := range text {
		if r == '\t' {
			clean.WriteString(tabs)
		} else if !unicode.IsControl(r) {
			clean.WriteRune(r)
		}
	}
	return clean.String()
}
