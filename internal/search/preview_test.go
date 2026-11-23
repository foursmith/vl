package search

import (
	"context"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRipgrepPreviewsResolveNormalizedPositions(t *testing.T) {
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("ripgrep is unavailable")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "中文.txt")
	longLine := strings.Repeat("界", 7000) + " needle " + strings.Repeat("x", 15000) + " needle"
	raw := "\uFEFF猫 needle needle\r\nno hit\r\n" + longLine + "\r\nsecond needle\r\n"
	writeSearchFile(t, path, raw)
	matcher, err := (Options{Query: "needle"}).Compile()
	if err != nil {
		t.Fatal(err)
	}
	var found []File
	count, err := matcher.Scan(context.Background(), root, nil, func(file File) {
		found = append(found, file)
	})
	if err != nil || count != 0 || len(found) != 1 || found[0].preview == nil || found[0].Text != "" {
		t.Fatalf("expected one preview-only file, got %+v, unreadable=%d, err=%v", found, count, err)
	}
	file := found[0]
	if line, offset := file.PreviewLine(0); line != "猫 needle needle" || offset != 0 || file.Matches[0].Start != 4 {
		t.Fatalf("invalid preview %q with offset %d, matches %+v", line, offset, file.Matches)
	}
	line, start := file.PreviewLine(2)
	hit := file.Matches[2]
	if start == 0 || len(line) > 12_000 || line[hit.Start-start:hit.End-start] != "needle" {
		t.Fatal("long preview did not keep the first match")
	}
	text := normalizeText([]byte(raw))
	got, ok := file.ResolveMatches(text)
	if !ok || !reflect.DeepEqual(got, matcher.Find(text, "")) {
		t.Fatalf("resolved matches %+v, current=%t", got, ok)
	}
	if _, ok := file.ResolveMatches("猫 different\nno hit\nsecond needle\n"); ok {
		t.Fatal("changed match line was accepted")
	}
	writeSearchFile(t, path, raw+"new content")
	if _, ok := file.ResolveMatches(text); ok {
		t.Fatal("changed disk file was accepted")
	}
}
