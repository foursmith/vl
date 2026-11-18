package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/foursmith/vl/internal/theme"
)

func BenchmarkPreview(b *testing.B) {
	path := filepath.Join(b.TempDir(), "source.go")
	source := "var value = \"sample 中文\" // comment\n"
	if err := os.WriteFile(path, []byte(strings.Repeat(source, 5000)), 0600); err != nil {
		b.Fatal(err)
	}
	var unique strings.Builder
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&unique, "var value%d = \"sample 中文\" // comment\n", i)
	}
	uniquePath := filepath.Join(filepath.Dir(path), "unique.go")
	if err := os.WriteFile(uniquePath, []byte(unique.String()), 0600); err != nil {
		b.Fatal(err)
	}
	for _, name := range []string{"plain", "repeated", "unique"} {
		b.Run(name, func(b *testing.B) {
			opt := previewOptions{tabSize: 4}
			file := path
			if name == "unique" {
				file = uniquePath
			}
			if name != "plain" {
				opt.lines, opt.width, opt.lineNumbers = 100, 100, true
				opt.colors = theme.Builtin("dark_modern")
			}
			if err := writePreview(io.Discard, file, opt); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := writePreview(io.Discard, file, opt); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
