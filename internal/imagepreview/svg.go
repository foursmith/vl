package imagepreview

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

func renderSVG(ctx context.Context, source []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// A fixed bounding box preserves the aspect ratio and bounds raster allocation.
	// stdin also handles Git blobs and avoids resolving paths relative to the SVG file.
	cmd := exec.CommandContext(ctx, "resvg", "--width", "2048", "--height", "2048", "-", "-c")
	cmd.WaitDelay = 200 * time.Millisecond
	cmd.Stdin = bytes.NewReader(source)
	var output svgOutput
	cmd.Stdout = &output
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		return nil, fmt.Errorf("SVG preview: %w", err)
	}
	return output.buf.Bytes(), nil
}

type svgOutput struct{ buf bytes.Buffer }

func (out *svgOutput) Write(p []byte) (int, error) {
	if len(p) > MaxBytes-out.buf.Len() {
		return 0, fmt.Errorf("SVG preview exceeds 32 MiB output limit")
	}
	return out.buf.Write(p)
}
