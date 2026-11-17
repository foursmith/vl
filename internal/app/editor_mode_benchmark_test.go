package app

import (
	"context"
	"testing"
)

// A full edit/frame/highlight and undo/frame/highlight cycle on the app model source.
func BenchmarkEditorEditHighlight(b *testing.B) {
	m, err := NewEditor(context.Background(), "model.go", false)
	if err != nil {
		b.Fatal(err)
	}
	defer m.Close()

	drain(b, m, m.scheduleHighlight())
	m.View()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d := m.document()
		d.Insert("// memory check\n")
		m.ensureEditor()
		m.View()
		drain(b, m, m.scheduleHighlight())
		d.Undo()
		m.ensureEditor()
		m.View()
		drain(b, m, m.scheduleHighlight())
	}
}
