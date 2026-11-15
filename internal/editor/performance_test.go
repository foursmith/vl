package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func BenchmarkCursorMove(b *testing.B) {
	d := &Document{Text: strings.Repeat("一二三四五六七八九十 abcdefghijklmn\n", 20000), Anchor: -1}
	d.SetPosition(10000, 5)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Move("right", false)
		d.Move("left", false)
	}
}

func BenchmarkInsertUndo(b *testing.B) {
	d := &Document{Text: strings.Repeat("一二三四五六七八九十 abcdefghijklmn\n", 20000), Anchor: -1}
	d.SetPosition(10000, 5)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Insert("x")
		d.Undo()
	}
}

func BenchmarkSave(b *testing.B) {
	path := filepath.Join(b.TempDir(), "save.txt")
	if err := os.WriteFile(path, []byte(strings.Repeat("line\n", 200000)), 0644); err != nil {
		b.Fatal(err)
	}
	d, err := Open(path)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Insert("x")
		if err := d.Save(); err != nil {
			b.Fatal(err)
		}
	}
}
