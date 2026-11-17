package files

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyDirectoryAndRefuseUnsafeTargets(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	os.Mkdir(filepath.Join(root, "src"), 0755)
	os.WriteFile(filepath.Join(root, "src", "run"), []byte("hello"), 0755)
	if err := Copy(ctx, root, "src", "dst"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "dst", "run"))
	if err != nil || string(data) != "hello" {
		t.Fatal("directory contents")
	}
	info, _ := os.Stat(filepath.Join(root, "dst", "run"))
	if info.Mode().Perm() != 0755 {
		t.Fatal("lost executable mode")
	}
	for _, dst := range []string{"dst", "src/child", "../escape", ".git/copy", "."} {
		if err := Copy(ctx, root, "src", dst); err == nil {
			t.Fatalf("allowed %s", dst)
		}
	}
	data, _ = os.ReadFile(filepath.Join(root, "dst", "run"))
	if string(data) != "hello" {
		t.Fatal("overwrote existing copy")
	}
	os.Symlink("src", filepath.Join(root, "alias"))
	if err := Copy(ctx, root, "src", "alias/child"); err == nil {
		t.Fatal("copied through alias into self")
	}
	os.Symlink("run", filepath.Join(root, "src", "link"))
	if err := Copy(ctx, root, "src", "failed"); err == nil {
		t.Fatal("copied symlink")
	}
	if _, err := os.Stat(filepath.Join(root, "failed")); !os.IsNotExist(err) {
		t.Fatal("partial copy remains")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := Copy(canceled, root, "dst", "canceled"); err == nil {
		t.Fatal("ignored cancellation")
	}
}
