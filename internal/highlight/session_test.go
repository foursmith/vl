package highlight

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/ourongxing/vl/internal/theme"
)

func TestSessionMatchesFreshHighlightAcrossEdits(t *testing.T) {
	ctx := context.Background()
	var session Session
	colors := theme.Builtin("dark_modern")
	for _, text := range []string{
		"package main\n/* 中文\ncomment\n*/\nvar n = 1\n",
		"package main\n// 中文\ncomment\n*/\nvar n = 1\n",
		"package main\n/* 中文\ncomment\n*/\nvar n = 1\n",
		"package main\n/* 中文\nextra\ncomment\n*/\nvar n = 1\n",
		"package main\nvar n = 1\n",
		"",
		"var 中文 = `multi\nline`\n",
	} {
		got := session.Lines(ctx, "file.go", text, colors)
		want := referenceLines(ctx, "file.go", text, colors)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("incremental mismatch for %q\ngot: %#v\nwant: %#v", text, got, want)
		}
	}
}

func TestSessionSharesOnlyMatchingLexicalStates(t *testing.T) {
	ctx := context.Background()
	colors := theme.Builtin("dark_modern")
	var session Session
	first := session.Lines(ctx, "file.go", "var n = 1\nvar other = 2", colors)
	saved := append([]Span(nil), first[0]...)
	second := session.Lines(ctx, "file.go", "var n = 1\nvar other = 3", colors)
	if len(first[0]) == 0 || &first[0][0] != &second[0][0] {
		t.Fatal("unchanged context did not share spans")
	}
	text := "/*\nvar n = 1\n*/"
	got := session.Lines(ctx, "file.go", text, colors)
	if !reflect.DeepEqual(got, referenceLines(ctx, "file.go", text, colors)) {
		t.Fatal("shared tokens ignored the line's lexical state")
	}
	if !reflect.DeepEqual(first[0], saved) {
		t.Fatal("later snapshots mutated published spans")
	}
}

func TestSessionThemeGrammarAndCancellation(t *testing.T) {
	ctx := context.Background()
	var session Session
	text := "var n = 1\n"
	for _, tc := range []struct{ path, theme string }{
		{"file.go", "dark_modern"}, {"file.go", "light_plus"},
		{"file.json", "light_plus"}, {"file.unknown-vl", "light_plus"},
		{"file.go", "dark_modern"},
	} {
		colors := theme.Builtin(tc.theme)
		if got, want := session.Lines(ctx, tc.path, text, colors), referenceLines(ctx, tc.path, text, colors); !reflect.DeepEqual(got, want) {
			t.Fatalf("stale theme or grammar for %+v", tc)
		}
	}
	old := session.doc
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if got := session.Lines(cancelled, "file.go", "/*", theme.Builtin("dark_modern")); got != nil || session.doc != old {
		t.Fatal("cancelled request changed session")
	}
	colors := theme.Builtin("dark_modern")
	if got := session.Lines(ctx, "file.go", text, colors); !reflect.DeepEqual(got, referenceLines(ctx, "file.go", text, colors)) {
		t.Fatal("cancelled request corrupted later work")
	}
}

func TestSessionConcurrentSnapshots(t *testing.T) {
	var session Session
	colors := theme.Builtin("dark_modern")
	var workers sync.WaitGroup
	for i := 0; i < 8; i++ {
		workers.Go(func() {
			for j := 0; j < 8; j++ {
				text := fmt.Sprintf("/* worker %d */\nvar n = %d\n", i, j)
				got := session.Lines(context.Background(), "file.go", text, colors)
				want := referenceLines(context.Background(), "file.go", text, colors)
				if !reflect.DeepEqual(got, want) {
					t.Error("concurrent request received another snapshot's spans")
				}
			}
		})
	}
	workers.Wait()
}

func TestSessionLineLimitAndRecovery(t *testing.T) {
	var session Session
	ctx := context.Background()
	colors := theme.Builtin("dark_modern")
	text := "package main\n" + strings.Repeat("x", 20_001) + "\nvar n int\n"
	got := session.Lines(ctx, "file.go", text, colors)
	if !reflect.DeepEqual(got, referenceLines(ctx, "file.go", text, colors)) {
		t.Fatal("line limit propagated incomplete lexical state")
	}
	text = "package main\n/* repaired */\nvar n int\n"
	if got := session.Lines(ctx, "file.go", text, colors); !reflect.DeepEqual(got, referenceLines(ctx, "file.go", text, colors)) {
		t.Fatal("editing a stopped line did not repair following lines")
	}
	if got := session.Lines(ctx, "file.go", strings.Repeat("x", MaxBytes+1), colors); got != nil {
		t.Fatal("oversized snapshot was tokenized")
	}
}

func BenchmarkSessionEdit(b *testing.B) {
	ctx := context.Background()
	colors := theme.Builtin("dark_modern")
	var session Session
	text := strings.Repeat("var value = \"sample\" // 中文\n", 5000)
	session.Lines(ctx, "file.go", text, colors)
	cut := strings.Index(text, "\n") + 1
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		session.Lines(ctx, "file.go", fmt.Sprintf("var changed = %d\n", i)+text[cut:], colors)
	}
}

func BenchmarkDiffHighlightShared(b *testing.B) {
	ctx := context.Background()
	colors := theme.Builtin("dark_modern")
	var source strings.Builder
	for i := 0; i < 2000; i++ {
		fmt.Fprintf(&source, "var value%d = %d\n", i, i)
	}
	old := source.String()
	added := strings.Replace(old, "var value1000 = 1000", "var value1000 = 1001", 1)
	for _, shared := range []bool{false, true} {
		b.Run(fmt.Sprintf("shared=%v", shared), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if shared {
					var session Session
					session.Lines(ctx, "file.go", old, colors)
					session.Lines(ctx, "file.go", added, colors)
				} else {
					referenceLines(ctx, "file.go", old, colors)
					referenceLines(ctx, "file.go", added, colors)
				}
			}
		})
	}
}
