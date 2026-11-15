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
	patch, err := p.HunkPatch(1)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(patch, "a/a") || !strings.Contains(patch, "No newline") {
		t.Fatalf("hunk crossed file boundary: %s", patch)
	}
	pairs := p.Pairs()
	for _, pair := range pairs {
		if pair.Left != nil && pair.Left.Kind == Deleted && pair.Right != nil && pair.Right.Kind != Added {
			t.Fatal("invalid replacement pair")
		}
	}
}

func TestAddedFilesRequireWholeFileStaging(t *testing.T) {
	p := Parse("diff --git a/a b/a\nnew file mode 100644\n--- /dev/null\n+++ b/a\n@@ -0,0 +1 @@\n+hello\n")
	if _, err := p.HunkPatch(0); err == nil {
		t.Fatal("accepted partial new-file patch")
	}
	if _, err := p.HunkPatch(-1); err == nil {
		t.Fatal("accepted invalid hunk")
	}
	if len(Parse("").Lines) != 0 {
		t.Fatal("empty patch has content")
	}
}
