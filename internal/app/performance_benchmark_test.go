package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ourongxing/elegit/internal/diff"
	gitservice "github.com/ourongxing/elegit/internal/git"
)

func benchmarkPatch() diff.Patch {
	var raw strings.Builder
	raw.WriteString("diff --git a/file.go b/file.go\n--- a/file.go\n+++ b/file.go\n")
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&raw, "@@ -%d,20 +%d,20 @@\n", i*30+1, i*30+1)
		for j := 0; j < 20; j++ {
			fmt.Fprintf(&raw, "-old value %d\n+new value %d\n", j, j)
		}
	}
	return diff.Parse(raw.String())
}

func BenchmarkDiffRender(b *testing.B) {
	m := New(context.Background(), &gitservice.Repository{Root: b.TempDir()}, true)
	m.width, m.height = 180, 55
	m.patch = benchmarkPatch()
	m.diffPath = "file.go"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.diffRows = nil
		m.renderDiff()
	}
}

func BenchmarkHunkNavigation(b *testing.B) {
	m := New(context.Background(), &gitservice.Repository{Root: b.TempDir()}, true)
	m.width, m.height = 180, 55
	m.patch = benchmarkPatch()
	m.diffPath = "file.go"
	m.renderDiff()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d := 1
		if i%2 != 0 {
			d = -1
		}
		m.nextHunk(d)
		m.View()
	}
}

func BenchmarkEditorTyping(b *testing.B) {
	root := b.TempDir()
	if err := os.WriteFile(filepath.Join(root, "source.go"), []byte(strings.Repeat("var value = \"sample\" // 中文\n", 5000)), 0644); err != nil {
		b.Fatal(err)
	}
	m := New(context.Background(), &gitservice.Repository{Root: root}, true)
	m.openFile("source.go")
	d := m.document()
	d.Move("ctrl+end", false)
	m.ensureEditor()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Insert("x")
		m.ensureEditor()
		m.View()
		d.Undo()
		m.ensureEditor()
	}
}

func BenchmarkQuickOpenFilter(b *testing.B) {
	m := New(context.Background(), &gitservice.Repository{Root: b.TempDir()}, false)
	for i := 0; i < 10000; i++ {
		path := fmt.Sprintf("src/package%03d/file%05d.go", i%100, i)
		m.palette = append(m.palette, action{id: path, label: path})
	}
	m.input.SetValue("file099")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.filteredActions()
	}
}

func BenchmarkChangeTree(b *testing.B) {
	m := New(context.Background(), &gitservice.Repository{Root: b.TempDir()}, false)
	for i := 0; i < 10000; i++ {
		m.snapshot.Changes = append(m.snapshot.Changes, gitservice.Change{Path: fmt.Sprintf("src/package%03d/file%05d.go", i%100, i), Index: ' ', Worktree: 'M'})
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.repositoryChangesTree("", false)
	}
}

func BenchmarkLongLineFrame(b *testing.B) {
	root := b.TempDir()
	if err := os.WriteFile(filepath.Join(root, "long.txt"), []byte(strings.Repeat("x", 1<<20)), 0644); err != nil {
		b.Fatal(err)
	}
	m := New(context.Background(), &gitservice.Repository{Root: root}, false)
	m.openFile("long.txt")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.document().Move("right", false)
		m.ensureEditor()
		m.View()
	}
}

func BenchmarkOutputPanelFrame(b *testing.B) {
	m := New(context.Background(), &gitservice.Repository{Root: b.TempDir()}, false)
	m.outputVisible = true
	m.resize()
	m.Update(operationMsg{title: "Bench", output: strings.Repeat("progress line\n", 50000)})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.View()
	}
}
