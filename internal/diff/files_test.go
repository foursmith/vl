package diff

import (
	"slices"
	"strings"
	"testing"
)

func TestSplitFilesKeepsExtendedHeadersAndGitPaths(t *testing.T) {
	patches := []string{
		"diff --git a/old name b/new name\nsimilarity index 100%\nrename from old name\nrename to new name\n",
		"diff --git a/dir with spaces/bin b/dir with spaces/bin\nBinary files a/dir with spaces/bin and b/dir with spaces/bin differ\n",
		"diff --git \"a/\\346\\226\\207.txt\" \"b/\\346\\226\\207.txt\"\ndeleted file mode 100644\n--- \"a/\\346\\226\\207.txt\"\n+++ /dev/null\n@@ -1 +0,0 @@\n-diff --git a/ignore b/ignore\n",
	}
	want := []FilePatch{
		{Path: "new name", Raw: patches[0], Status: 'R'},
		{Path: "dir with spaces/bin", Raw: patches[1], Status: 'M'},
		{Path: "文.txt", Raw: patches[2], Status: 'D'},
	}
	if got, err := SplitFiles(strings.Join(patches, "")); err != nil || !slices.Equal(got, want) {
		t.Fatalf("split patches = %+v, error = %v", got, err)
	}
}
