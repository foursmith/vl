package diff

import (
	"strings"
	"testing"
)

func TestMultipleFilesAndHunks(t *testing.T) {
	raw := "diff --git a/a b/a\nindex 111..222 100644\n--- a/a\n+++ b/a\n@@ -1,2 +1,3 @@\n same\n-old\n+new\n+extra\ndiff --git a/b b/b\n--- a/b\n+++ b/b\n@@ -9 +9 @@\n-x\n+y\n\\ No newline at end of file\n"
	p := Parse(raw)
	if len(p.Hunks) != 2 || p.Added != 3 || p.Deleted != 2 {
		t.Fatalf("parse: %+v", p)
	}
	first, second := p.Hunks[0], p.Hunks[1]
	if first.end >= second.start || strings.Contains(second.header, "a/a") || p.Lines[second.end-1].Text != "\\ No newline at end of file" {
		t.Fatalf("hunk crossed file boundary: %+v, %+v", first, second)
	}
	pairs := p.Pairs()
	for _, pair := range pairs {
		if pair.Left != nil && pair.Left.Kind == Deleted && pair.Right != nil && pair.Right.Kind != Added {
			t.Fatal("invalid replacement pair")
		}
	}
}

func TestEmptyPatch(t *testing.T) {
	if len(Parse("").Lines) != 0 {
		t.Fatal("empty patch has content")
	}
}

func TestFileCreationAndDeletionMetadata(t *testing.T) {
	for _, tc := range []struct {
		name, raw      string
		added, deleted bool
	}{
		{"new", "diff --git a/a b/a\nnew file mode 100644\n", true, false},
		{"deleted", "diff --git a/a b/a\ndeleted file mode 100644\n", false, true},
		{"content", "--- a/a\n+++ b/a\n@@ -1,2 +1,2 @@\n-new file mode 100644\n-deleted file mode 100644\n+new file mode 100755\n+deleted file mode 100755\n", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := Parse(tc.raw)
			if p.NewFile != tc.added || p.DeletedFile != tc.deleted {
				t.Fatalf("new=%v deleted=%v", p.NewFile, p.DeletedFile)
			}
		})
	}
}
