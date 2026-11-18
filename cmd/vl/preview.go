package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/vl/internal/config"
	"github.com/ourongxing/vl/internal/highlight"
	"github.com/ourongxing/vl/internal/theme"
)

type previewOptions struct {
	skip, lines, width, tabSize int
	lineNumbers, wrap           bool
	colors                      *theme.Theme
}

// Keep reads bounded even for minified or generated files with very long lines.
const previewLineBytes = 32 << 10
const previewContextLines = 64

func runPreview(args []string, stdout, stderr io.Writer) int {
	var opt previewOptions
	flags := flag.NewFlagSet("preview", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.IntVar(&opt.skip, "skip", 0, "skip this many source lines")
	flags.IntVar(&opt.lines, "lines", 0, "maximum source lines to print (0 prints through EOF)")
	flags.IntVar(&opt.width, "width", 0, "maximum columns per row, including line numbers (0 leaves rows unclipped)")
	flags.IntVar(&opt.tabSize, "tab-size", 4, "spaces per tab (1–32)")
	flags.BoolVar(&opt.lineNumbers, "line-numbers", false, "show source line numbers")
	flags.BoolVar(&opt.wrap, "wrap", false, "wrap long lines at --width instead of clipping")
	color := flags.String("color", "auto", "syntax colors: auto, always, or never")
	themeName := flags.String("theme", "", "syntax theme ID or theme file (default: VL's saved theme)")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: vl preview [options] file")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
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
		return theme.Load(override, "")
	}
	path, err := config.LayoutPath()
	if err == nil {
		layout, err := config.LoadLayout(path)
		if err == nil && layout.Theme != "" {
			if selected, err := theme.Load(layout.Theme, layout.ThemeType); err == nil {
				return selected, nil
			}
		}
	}
	return theme.Builtin("dark_modern"), nil
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
	for index := 0; opt.lines == 0 || index-opt.skip < opt.lines; index++ {
		line, err := reader.ReadSlice('\n')
		if len(line) == 0 && err == io.EOF {
			break
		}
		shortened := err == bufio.ErrBufferFull
		var source string
		if index >= contextStart {
			line = bytes.TrimSuffix(bytes.TrimSuffix(line, []byte("\n")), []byte("\r"))
			shortened = shortened || len(line) > previewLineBytes
			source = string(line[:min(len(line), previewLineBytes)])
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
		if index == 0 {
			source = strings.TrimPrefix(source, "\ufeff")
		}
		source = strings.ToValidUTF8(source, "\ufffd")
		visible := index >= opt.skip
		var spans []highlight.Span
		if index == contextStart && opt.colors != nil {
			syntax = highlight.NewPreview(path, opt.colors)
		}
		if syntax != nil && !shortened {
			spans = syntax.Line(source, visible)
		}
		if !visible {
			continue
		}
		lineText := previewLine(source, tabs, spans)
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

	if opt.width > 0 {
		width := opt.width - len(gutter)
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

func previewLine(source, tabs string, spans []highlight.Span) string {
	if len(spans) == 0 {
		return previewText(source, tabs)
	}
	var line strings.Builder
	runes := []rune(source)
	start := 0
	for _, span := range spans {
		line.WriteString(previewText(string(runes[start:span.Start]), tabs))
		style := span.Font.Apply(lipgloss.NewStyle())
		if span.Color != "" {
			style = style.Foreground(lipgloss.Color(span.Color))
		}
		if span.Background != "" {
			style = style.Background(lipgloss.Color(span.Background))
		}
		line.WriteString(style.Render(previewText(string(runes[span.Start:span.End]), tabs)))
		start = span.End
	}
	line.WriteString(previewText(string(runes[start:]), tabs))
	return line.String()
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
