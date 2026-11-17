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

	setTestFind(m, "one")
	d.SelectRange(0, 3)
	press(m, " ")
	press(m, "enter")
	if d.Anchor != -1 || m.findOptions.Query != "one" {
		t.Fatal("clear highlights lost search pattern or selection")
	}
	press(m, "c")
	press(m, "w")
	if !m.sidebar || !m.sidebarVisible || d.Text != "one two" {
		t.Fatal("cw did not focus sidebar")
	}
}
func TestVimClipboardDeliveryGuards(t *testing.T) {
	for _, stale := range []string{"", "text", "cursor", "mode", "modal", "tab"} {
		t.Run(stale, func(t *testing.T) {
			m := newHoverRenderModel(t)
			d := &editor.Document{Text: "abc", Anchor: -1}
			m.tabs = []tab{{doc: d}}
			m.activeTab = 0
			m.sidebar = false

			msg := vimClipboardMsg{doc: d, source: d.Text, cursor: d.Cursor, anchor: d.Anchor, text: "X", after: true}
			switch stale {
			case "text":
				d.Insert("new")
			case "cursor":
				d.SetPosition(0, 1)
			case "mode":
				m.tabs[0].vim.InsertMode = true
			case "modal":
				m.modal = "confirm"
			case "tab":
				m.tabs[0].doc = &editor.Document{Text: "other", Anchor: -1}
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

func TestVimLeaderSelectsTabByNumber(t *testing.T) {
	m := tabWorkspace(t, 3)
	defer m.Close()
	for _, path := range []string{"file0.go", "file1.go", "file2.go"} {
		drain(t, m, m.openFile(path))
	}

	original := m.document()
	original.Insert("unsaved")
	for _, tt := range []struct {
		key  string
		want int
	}{{"1", 0}, {"2", 1}, {"3", 2}, {"9", 2}} {
		press(m, " ")
		press(m, tt.key)
		if m.activeTab != tt.want || m.sidebar {
			t.Fatalf("Space %s: active tab %d", tt.key, m.activeTab)
		}
	}
	if !original.Dirty() || len(m.tabs) != 3 {
		t.Fatal("switching discarded edits or closed a tab")
	}
	press(m, " ")
	press(m, "esc")
	press(m, "1")
	if m.activeTab != 2 {
		t.Fatal("Escape did not cancel leader")
	}
	press(m, "i")
	press(m, " ")
	press(m, "1")
	if m.activeTab != 2 || !m.tabs[2].vim.InsertMode {
		t.Fatal("Insert input activated tab shortcut")
	}
}

func TestDiffLeaderSelectsTabsWithoutSpacePaging(t *testing.T) {
	m := tabWorkspace(t, 2)
	defer m.Close()
	m.openFile("file0.go")
	m.openFile("file1.go")
	showDiffFixture(m)

	press(m, " ")
	press(m, "1")
	if m.activeTab != 0 {
		t.Fatal("Diff Space 1 did not select first tab")
	}
	press(m, " ")
	press(m, "3")
	if m.activeTab != 2 || !m.diffOpen {
		t.Fatal("editor could not return to Diff")
	}
	press(m, " ")
	press(m, "2")
	if m.activeTab != 1 {
		t.Fatal("Diff Space 2")
	}
	m.activateTab(2)
	press(m, " ")
	press(m, "9")
	if m.activeTab != 2 {
		t.Fatal("invalid tab changed selection")
	}
	press(m, " ")
	press(m, "esc")
	press(m, "1")
	if m.activeTab != 2 {
		t.Fatal("Escape did not cancel Diff leader")
	}
	press(m, " ")
	press(m, "f6")
	press(m, "f6")
	press(m, "1")
	if m.activeTab != 2 {
		t.Fatal("Diff leader crossed focus switch")
	}
	m.viewport.SetHeight(3)
	m.viewport.SetContent("0\n1\n2\n3\n4\n5\n6\n7\n8")
	press(m, " ")
	press(m, " ")
	if m.viewport.YOffset() != 0 {
		t.Fatal("Space must not page in Diff")
	}
	press(m, "f")
	if m.flash == nil || m.viewport.YOffset() != 0 {
		t.Fatal("f must use shared Flash without paging")
	}
	press(m, "esc")
}

func TestDiffLeaderClosesOnlyCurrentTab(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	m.openFile("file0.go")
	document := m.document()
	document.Insert("unsaved")
	showDiffFixture(m)

	press(m, " ")
	press(m, "esc")
	press(m, "q")
	if len(m.tabs) != 2 {
		t.Fatal("canceled leader closed tab")
	}
	press(m, " ")
	drain(t, m, press(m, "q"))
	if len(m.tabs) != 1 || m.document() != document || !document.Dirty() || m.modal != "" {
		t.Fatal("Diff close changed the other dirty document")
	}
	press(m, " ")
	drain(t, m, press(m, "q"))
	if m.modal != "close" || m.document() != document {
		t.Fatal("editor close bypassed unsaved prompt")
	}
}
