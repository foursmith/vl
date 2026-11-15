package platform

import (
	"context"
	"testing"
)

func TestOpenURLRejectsNonHTTPSWithoutLaunchingAnything(t *testing.T) {
	for _, target := range []string{"javascript:alert(1)", "file:///tmp/file", "http://github.com/owner/repo", "https://"} {
		if err := OpenURL(context.Background(), target); err == nil {
			t.Fatalf("accepted %q", target)
		}
	}
}
