package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/foursmith/vl/internal/files"
	gitservice "github.com/foursmith/vl/internal/git"
)

func BenchmarkScrollFrame(b *testing.B) {
	for _, surface := range []string{"editor", "sidebar"} {
		b.Run(surface, func(b *testing.B) {
			root := b.TempDir()
			source := strings.Repeat("func example(value string) string { return value + \"sample\" }\n", 10000)
			if err := os.WriteFile(filepath.Join(root, "large.go"), []byte(source), 0644); err != nil {
				b.Fatal(err)
			}
			m := New(context.Background(), &gitservice.Repository{Root: root}, true)
			m.Update(tea.WindowSizeMsg{Width: 180, Height: 55})
			m.openFile("large.go")
			m.tabs[0].top = 1000
			for i := 0; i < 10000; i++ {
				name := fmt.Sprintf("file%05d.go", i)
				m.items = append(m.items, item{file: name, label: name, key: name})
			}
			x := m.mainX() + 10
			if surface == "sidebar" {
				x = 10
				m.sideTop = 1000
				m.selected = 1000
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				direction := tea.MouseWheelDown
				if i%2 != 0 {
					direction = tea.MouseWheelUp
				}
				m.Update(tea.MouseWheelMsg{X: x, Y: 10, Button: direction})
				m.View()
			}
		})
	}
}

func BenchmarkExplorerDecorations(b *testing.B) {
	m := New(context.Background(), &gitservice.Repository{Root: b.TempDir()}, true)
	for i := 0; i < 5000; i++ {
		path := fmt.Sprintf("src/package%03d/file%05d.go", i%100, i)
		m.treeEntries = append(m.treeEntries, files.Entry{Path: path, Label: filepath.Base(path), Depth: 2})
		m.snapshot.Changes = append(m.snapshot.Changes, gitservice.Change{Path: path, Index: ' ', Worktree: 'M'})
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.explorerItems()
	}
}
