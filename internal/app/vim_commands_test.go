package app

import (
	"github.com/ourongxing/vl/internal/editor"
	"os"
	"testing"
)

func TestVimMappedSaveAndClose(t *testing.T) {
	for _, key := range []string{"w", "z", "q"} {
		t.Run(key, func(t *testing.T) {
			m := tabWorkspace(t, 1)
			defer m.Close()
			drain(t, m, m.openFile("file0.go"))
			m.vimEnabled = true
			d := m.document()
			d.Insert("// changed\n")
			press(m, " ")
			drain(t, m, press(m, key))
			if key == "q" {
				if m.modal != "close" || m.document() != d {
					t.Fatal("close bypassed dirty prompt")
				}
				return
			}
			data, err := os.ReadFile(d.Path)
			if err != nil || string(data) != d.Text || d.Dirty() {
				t.Fatal("mapped save failed", err)
			}
			if key == "z" && m.document() != nil {
				t.Fatal("save-close retained tab")
			}
			if key == "w" && m.document() != d {
				t.Fatal("save closed tab")
			}
		})
	}
}
func TestVimMappedSaveCloseConflict(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	drain(t, m, m.openFile("file0.go"))
	m.vimEnabled = true
	d := m.document()
	d.Insert("local")
	if err := os.WriteFile(d.Path, []byte("external"), 0644); err != nil {
		t.Fatal(err)
	}
	press(m, " ")
	drain(t, m, press(m, "z"))
	if m.document() != d || !d.Dirty() {
		t.Fatal("save conflict closed tab")
	}
	data, _ := os.ReadFile(d.Path)
	if string(data) != "external" {
		t.Fatal("save conflict overwrote disk")
	}
}
func TestVimMappedFocusAndClearSearch(t *testing.T) {
	m := newHoverRenderModel(t)
	d := &editor.Document{Text: "one two", Anchor: -1}
	m.tabs = []tab{{doc: d}}
	m.activeTab = 0
	m.sidebar = false
	m.vimEnabled = true
	m.find = "one"
	d.SelectRange(0, 3)
	press(m, " ")
	press(m, "enter")
	if d.Anchor != -1 || m.find != "one" {
		t.Fatal("clear highlights lost search pattern or selection")
	}
	press(m, "c")
	press(m, "w")
	if !m.sidebar || !m.sidebarVisible || d.Text != "one two" {
		t.Fatal("cw did not focus sidebar")
	}
}
func TestVimClipboardDeliveryGuards(t *testing.T) {
	for _, stale := range []string{"", "text", "cursor", "mode", "modal", "tab", "disabled"} {
		t.Run(stale, func(t *testing.T) {
			m := newHoverRenderModel(t)
			d := &editor.Document{Text: "abc", Anchor: -1}
			m.tabs = []tab{{doc: d}}
			m.activeTab = 0
			m.sidebar = false
			m.vimEnabled = true
			msg := vimClipboardMsg{doc: d, source: d.Text, cursor: d.Cursor, anchor: d.Anchor, text: "X", after: true}
			switch stale {
			case "text":
				d.Insert("new")
			case "cursor":
				d.SetPosition(0, 1)
			case "mode":
				m.tabs[0].vim.InsertMode = true
			case "modal":
				m.modal = "find"
			case "tab":
				m.tabs[0].doc = &editor.Document{Text: "other", Anchor: -1}
			case "disabled":
				m.vimEnabled = false
			}
			before := d.Text
			m.applyVimClipboard(msg)
			if stale == "" {
				if d.Text != "aXbc" {
					t.Fatal(d.Text)
				}
			} else if d.Text != before {
				t.Fatal("stale clipboard result applied")
			}
		})
	}
}
