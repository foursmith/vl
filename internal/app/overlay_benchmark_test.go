package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/kitty"
)

// Image rows carry both virtual placeholders and a potentially large pixel upload.
func BenchmarkImageContextMenuOverlay(b *testing.B) {
	m := &Model{width: 160, height: 50}
	base := make([]string, m.height)
	for y := range base {
		base[y] = strings.Repeat(string(kitty.Placeholder)+string(kitty.Diacritic(y))+string(kitty.Diacritic(0)), m.width)
	}
	base[0] = ansi.KittyGraphics([]byte(strings.Repeat("A", 512<<10)), "a=T,i=42,f=100") + base[0]
	popup := []string{"┌──────────────────────┐", "│ Fit to Window        │", "│ Copy Path            │", "└──────────────────────┘"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.composeOverlay(base, popup, 30, 10)
	}
}
