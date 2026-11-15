package imagepreview

import (
	"bytes"
	"fmt"
	"image"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/kitty"
)

func kittyPixelRows(id, columns, rows int, pixels image.Image) ([]string, error) {
	var upload bytes.Buffer
	err := kitty.EncodeGraphics(&upload, pixels, &kitty.Options{
		Action: kitty.TransmitAndPut, ID: id, PlacementID: 1,
		Transmission: kitty.Direct, Format: kitty.PNG, Chunk: true, Quiet: 2,
		VirtualPlacement: true, Columns: columns, Rows: rows, DoNotMoveCursor: true,
	})
	if err != nil {
		return nil, err
	}
	result := make([]string, rows)
	for y := 0; y < rows; y++ {
		var line strings.Builder
		if y == 0 {
			line.WriteString(upload.String())
		}
		// Indexed foreground plus the third diacritic keeps IDs exact even when
		// the workbench renderer uses a 256-color profile.
		fmt.Fprintf(&line, "\x1b[38;5;%dm", id&255)
		for x := 0; x < columns; x++ {
			line.WriteRune(kitty.Placeholder)
			line.WriteRune(kitty.Diacritic(y))
			line.WriteRune(kitty.Diacritic(x))
			line.WriteRune(kitty.Diacritic(id >> 24))
		}
		line.WriteString("\x1b[0m")
		result[y] = line.String()
	}
	return result, nil
}

func DeleteKitty(id int) string {
	return ansi.KittyGraphics(nil, fmt.Sprintf("a=d,d=I,i=%d,q=2", id))
}
