package diff

import (
	"reflect"
	"strings"
	"testing"
)

func TestPairsAlignReplacementsAroundInsertionsAndDeletions(t *testing.T) {
	p := Parse("@@ -4,3 +4,3 @@\n-    load(\"alpha\", keepMiddle(), 10)\n-    return nil\n-    closeStream(\"default\")\n+    audit(\"new event\")\n+    load(\"beta\", keepMiddle(), 20)\n+    closeStream(\"fast\")\n")
	want := []Pair{
		{nil, &p.Lines[4], 0},
		{&p.Lines[1], &p.Lines[5], 0},
		{&p.Lines[2], nil, 0},
		{&p.Lines[3], &p.Lines[6], 0},
	}
	if got := p.Pairs()[1:]; !reflect.DeepEqual(got, want) {
		t.Fatalf("pairs = %v, want %v", got, want)
	}
	if got := p.MapOldLine(4); got != 4 {
		t.Fatalf("visual alignment changed source mapping to line %d", got)
	}
}

func TestInlineChangesKeepUnchangedWordsAndWholeGraphemes(t *testing.T) {
	fragments := func(line string, ranges []Range) []string {
		text := []rune(line)
		var result []string
		for _, r := range ranges {
			result = append(result, string(text[r.Start:r.End]))
		}
		return result
	}
	for _, tc := range []struct {
		old, next      string
		deleted, added []string
	}{
		{"log(\"e\u0301\", 保留(), 👩‍💻, 10)", "log(\"ê\", 保留(), 👩‍🔬, 20)", []string{"e\u0301", "👩‍💻", "1"}, []string{"ê", "👩‍🔬", "2"}},
		{"getUser(100)", "getUsers(120)", []string{"0"}, []string{"s", "2"}},
		{"getUserByID", "getAccountByID", []string{"User"}, []string{"Account"}},
		{"getUserByID", "setUserByIP", []string{"g", "D"}, []string{"s", "P"}},
		{"render(cafe\u0301)", "render(café)", []string{"e\u0301"}, []string{"é"}},
		{"alpha", "beta", []string{"alpha"}, []string{"beta"}},
		{"getUsers()", "getUser()", []string{"s"}, nil},
	} {
		t.Run(tc.old+" → "+tc.next, func(t *testing.T) {
			old, next := InlineChanges(tc.old, tc.next)
			if got := fragments(tc.old, old); !reflect.DeepEqual(got, tc.deleted) {
				t.Fatalf("old ranges = %q, want %q", got, tc.deleted)
			}
			if got := fragments(tc.next, next); !reflect.DeepEqual(got, tc.added) {
				t.Fatalf("new ranges = %q, want %q", got, tc.added)
			}
		})
	}
}

func TestInlineChangesLongSharedSequenceKeepsOriginalOffsets(t *testing.T) {
	common := strings.Repeat("ab", 90)
	old, next := InlineChanges("X"+common+"Y", "P"+common+"Q")
	want := []Range{{0, 1}, {181, 182}}
	if !reflect.DeepEqual(old, want) || !reflect.DeepEqual(next, want) {
		t.Fatalf("old ranges %v, new ranges %v, want %v on each side", old, next, want)
	}
}

func TestMultipleFilesAndHunks(t *testing.T) {
	raw := "diff --git a/a b/a\nindex 111..222 100644\n--- a/a\n+++ b/a\n@@ -1,2 +1,3 @@\n same\n-old\n+new\n+extra\ndiff --git a/b b/b\n--- a/b\n+++ b/b\n@@ -9 +9 @@\n-x\n+y\n\\ No newline at end of file\n"
	p := Parse(raw)
	if len(p.Hunks) != 2 || p.Added != 3 || p.Deleted != 2 {
		t.Fatalf("parse: %+v", p)
	}
	second := p.Hunks[1]
	last := p.Lines[len(p.Lines)-2]
	if strings.Contains(second.header, "a/a") || last.Hunk != 1 || !last.noNewline {
		t.Fatalf("wrong hunk or newline metadata: %+v, %+v", second, last)
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
