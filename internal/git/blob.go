package git

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// Blob reads an immutable object after checking its size. A missing path or
// revision (including HEAD in an unborn repository) is an absent diff side.
func (r *Repository) Blob(ctx context.Context, spec string, maxBytes int) ([]byte, error) {
	meta, err := r.run(ctx, spec+"\x00", "cat-file", "--batch-check", "-Z")
	if err != nil {
		return nil, err
	}
	meta = strings.TrimSuffix(meta, "\x00")
	if meta == spec+" missing" {
		return nil, nil
	}
	fields := strings.Fields(meta)
	if len(fields) != 3 || !validObjectID(fields[0]) || fields[1] != "blob" {
		return nil, fmt.Errorf("invalid image blob metadata")
	}
	size, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil || size < 0 || size > int64(maxBytes) {
		return nil, fmt.Errorf("image exceeds %d MiB preview limit", maxBytes>>20)
	}
	raw, err := r.Run(ctx, "cat-file", "blob", fields[0])
	if err != nil {
		return nil, err
	}
	return []byte(raw), nil
}
