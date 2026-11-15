package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkSnapshot(b *testing.B) {
	ctx := context.Background()
	r := &Repository{Root: b.TempDir()}
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"config", "user.name", "Bench"}, {"config", "user.email", "bench@example.test"}, {"config", "commit.gpgsign", "false"}, {"config", "core.hooksPath", filepath.Join(r.Root, ".no-hooks")}} {
		if _, err := r.Run(ctx, args...); err != nil {
			b.Fatal(err)
		}
	}
	for i := 0; i < 1000; i++ {
		if err := os.WriteFile(filepath.Join(r.Root, fmt.Sprintf("file%04d.go", i)), []byte("package main\n"), 0644); err != nil {
			b.Fatal(err)
		}
	}
	for _, args := range [][]string{{"add", "."}, {"commit", "-qm", "initial"}} {
		if _, err := r.Run(ctx, args...); err != nil {
			b.Fatal(err)
		}
	}
	for i := 0; i < 5; i++ {
		if _, err := r.Run(ctx, "remote", "add", fmt.Sprintf("remote%d", i), "https://github.com/example/repository.git"); err != nil {
			b.Fatal(err)
		}
	}
	for i := 0; i < 100; i++ {
		if err := os.WriteFile(filepath.Join(r.Root, fmt.Sprintf("file%04d.go", i)), []byte("package changed\n"), 0644); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := r.Snapshot(ctx); err != nil {
			b.Fatal(err)
		}
	}
}
