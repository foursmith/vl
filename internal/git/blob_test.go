package git

import (
	"context"
	"testing"
)

func TestBlobVersionsMissingAndLimits(t *testing.T) {
	r := repository(t)
	ctx := context.Background()
	for _, path := range []string{"image.png", "space\nname: image.png"} {
		write(t, r, path, "old\x00pixels")
	}
	run(t, r, "add", ".")
	run(t, r, "commit", "-m", "images")
	for _, path := range []string{"image.png", "space\nname: image.png"} {
		write(t, r, path, "new\x00pixels")
	}
	run(t, r, "add", ".")
	for _, tc := range []struct{ spec, want string }{
		{"HEAD:image.png", "old\x00pixels"},
		{":image.png", "new\x00pixels"},
		{"HEAD:space\nname: image.png", "old\x00pixels"},
		{":space\nname: image.png", "new\x00pixels"},
	} {
		data, err := r.Blob(ctx, tc.spec, 20)
		if err != nil || string(data) != tc.want {
			t.Fatalf("%q: %q %v", tc.spec, data, err)
		}
	}
	for _, spec := range []string{":missing.png", "HEAD:missing.png", "HEAD^:image.png"} {
		if data, err := r.Blob(ctx, spec, 20); err != nil || data != nil {
			t.Fatalf("missing %q: %q %v", spec, data, err)
		}
	}
	if _, err := r.Blob(ctx, "HEAD:image.png", 2); err == nil {
		t.Fatal("read oversized blob")
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := r.Blob(ctx, "HEAD:image.png", 20); err == nil {
		t.Fatal("ignored cancellation")
	}
}
