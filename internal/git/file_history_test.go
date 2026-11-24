package git

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestFileHistoryPageContinuesAcrossRename(t *testing.T) {
	r := repository(t)
	ctx := context.Background()
	oldPath, path := "old\tname.go", "新\n文件.go"
	write(t, r, oldPath, "original\n")
	run(t, r, "add", ".")
	run(t, r, "commit", "-m", "root")
	run(t, r, "mv", oldPath, path)
	run(t, r, "commit", "-m", "rename")

	// Create enough commits to end the first page on the rename.
	var history strings.Builder
	for i := 1; i < CommitPageSize; i++ {
		subject := fmt.Sprintf("change %d", i)
		content := subject + "\n"
		fmt.Fprintf(&history, "commit refs/heads/main\ncommitter Test <test@example.com> 1795438899 +0000\ndata %d\n%s\n", len(subject), subject)
		if i == 1 {
			fmt.Fprintln(&history, "from refs/heads/main^0")
		}
		fmt.Fprintf(&history, "M 100644 inline %q\ndata %d\n%s\n", path, len(content), content)
	}
	if _, err := r.run(ctx, history.String(), "fast-import", "--quiet"); err != nil {
		t.Fatal(err)
	}

	page, next, err := r.FileHistory(ctx, FileHistoryCursor{Revision: "HEAD", Path: path})
	if err != nil || len(page) != CommitPageSize {
		t.Fatalf("file history length = %d, err = %v", len(page), err)
	}
	rename := page[len(page)-1]
	if rename.Subject != "rename" || rename.Change.Path != path || rename.Change.Original != oldPath || next.Path != oldPath {
		t.Fatalf("rename = %+v, cursor = %+v", rename, next)
	}
	page, next, err = r.FileHistory(ctx, next)
	if err != nil || len(page) != 1 || page[0].Subject != "root" || page[0].Change.Path != oldPath || next.Revision != "" {
		t.Fatalf("history before rename = %+v, cursor = %+v, err = %v", page, next, err)
	}
	raw, err := r.CommitFileDiff(ctx, page[0].ID, page[0].Change)
	if err != nil || !strings.Contains(raw, "+original") {
		t.Fatalf("root diff = %q, err = %v", raw, err)
	}
}
