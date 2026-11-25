package app

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestEditorIndentKeys(t *testing.T) {
	for _, mode := range []string{"normal", "insert", "plain"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "example.yaml")
			const original = "a:\n  b: value\n"
			if err := os.WriteFile(path, []byte(original), 0600); err != nil {
				t.Fatal(err)
			}
			m, err := NewEditor(t.Context(), path, false)
			if err != nil {
				t.Fatal(err)
			}
			defer m.Close()
			switch mode {
			case "insert":
				press(m, "i")
			case "plain":
				m.setVimDisabled(true)
			}

			m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
			if got := m.document().Text; got != "  "+original {
				t.Fatalf("Tab produced %q", got)
			}
			m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
			if got := m.document().Text; got != original {
				t.Fatalf("Shift+Tab produced %q", got)
			}
		})
	}
}
