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

// Keep the same viewport while moving the cursor through highlighted source.
func BenchmarkEditorCursorFrame(b *testing.B) {
	m, err := NewEditor(context.Background(), "model.go", false)
	if err != nil {
		b.Fatal(err)
	}
	defer m.Close()
	m.width, m.height = 180, 55
	drain(b, m, m.scheduleHighlight())
	d := m.document()
	d.SetPosition(10, 0)
	m.ensureEditor()
	m.View()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.SetPosition(10+i%30, 0)
		m.ensureEditor()
		m.View()
	}
}
