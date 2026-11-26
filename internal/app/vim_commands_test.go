package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/foursmith/vl/internal/editor"
)

func typeVimCommand(m *Model, command string) {
	press(m, ":")
	for _, char := range command {
		press(m, string(char))
	}
}

func TestVimToggleComments(t *testing.T) {
	for _, tc := range []struct {
		name, original, keys, commented string
	}{
		{"file.go", "\t你好()\n  \n\tworld()\n", "3gcc", "\t/* 你好()\n  \n\tworld() */\n"},
		{"script", "#!/bin/sh\necho hello\n", "2gcc", "#!/bin/sh\n# echo hello\n"},
		{"site.css", "  color: red;  \n", "gcc", "  /* color: red; */  \n"},
		{"page.vue", "<template>hello</template>\n", "gcc", "<!-- <template>hello</template> -->\n"},
		{"page.jinja", "{{ title }}\n", "gcc", "{# {{ title }} #}\n"},
		{"block.go", "\n  first()\n\tsecond()  \n", "3gcc", "\n  /* first()\n\tsecond() */  \n"},
		{"block.py", "print('hello')\nprint('world')\n", "2gcc", "# print('hello')\n# print('world')\n"},
		{"script.cmd", "REMARK\n", "gcc", "REM REMARK\n"},
		{"script.bat", "echo one & echo two\n", "gcc", "echo one & echo two\n"},
		{"script.vim", "vim9script\necho 'hello'\n", "gcc", "# vim9script\necho 'hello'\n"},
		{"legacy.vim", "echo 'hello'\n", "gcc", "\" echo 'hello'\n"},
		{"shader.wgsl", "var counter: u32;\n", "gcc", "// var counter: u32;\n"},
		{"go.work", "go 1.27.3\n", "gcc", "// go 1.27.3\n"},
		{"go.sum", "example.org/pkg v1.0.0 h1:abc\n", "gcc", "example.org/pkg v1.0.0 h1:abc\n"},
		{"file.json", "{}\n", "gcc", "// {}\n"},
		{"file.jsonc", "{}\n", "gcc", "// {}\n"},
		{"file.txt", "text\n", "gcc", "text\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tc.name)
			if err := os.WriteFile(path, []byte(tc.original), 0600); err != nil {
				t.Fatal(err)
			}
			m, err := NewEditor(context.Background(), path, false)
			if err != nil {
				t.Fatal(err)
			}
			defer m.Close()
			doc := m.document()
			m.vimRegister = editor.VimRegisterValue{Text: "keep clipboard"}
			for _, key := range tc.keys {
				press(m, string(key))
			}
			if doc.Text != tc.commented {
				t.Fatalf("commented text = %q, want %q", doc.Text, tc.commented)
			}
			if tc.commented == tc.original {
				if doc.Dirty() {
					t.Fatal("unsupported file became dirty")
				}
				return
			}
			press(m, "u")
			if doc.Text != tc.original {
				t.Fatalf("undo = %q, want %q", doc.Text, tc.original)
			}
			press(m, ".")
			if doc.Text != tc.commented {
				t.Fatalf("repeat = %q, want %q", doc.Text, tc.commented)
			}
			for _, key := range tc.keys {
				press(m, string(key))
			}
			if doc.Text != tc.original || m.vimRegister.Text != "keep clipboard" {
				t.Fatalf("uncomment = %q, register = %q", doc.Text, m.vimRegister.Text)
			}
		})
	}
}

func TestVimCommentaryRanges(t *testing.T) {
	for _, tc := range []struct {
		original, keys, want string
		row                  int
	}{
		{"\tfirst()\n\t\tsecond()\nlast()\n", "gcj", "\t/* first()\n\t\tsecond() */\nlast()\n", 0},
		{"first()\n//second()\nlast()\n", "Vkgc", "/* first()\n//second() */\nlast()\n", 1},
		{"first()\nsecond()\n\nlast()\n", "gcap", "/* first()\nsecond() */\n\nlast()\n", 1},
		{"//first()\n// second()\nlast()\n", "gcu", "first()\nsecond()\nlast()\n", 1},
		{"//first()\n// second()\nlast()\n", "gcgc", "first()\nsecond()\nlast()\n", 1},
		{"first()\nsecond()\nlast()\n", "Vjgc", "/* first()\nsecond() */\nlast()\n", 0},
		{"/* first() */\nsecond()\n", "gcc", "first()\nsecond()\n", 0},
		{"/*\nfirst()\n*/\nsecond()\n", "gcc", "\nfirst()\n\nsecond()\n", 1},
		{"/*\nfirst()\n*/\nsecond()\n", "gcc", "\nfirst()\n\nsecond()\n", 2},
		{"/* first() */\n// second()\n", "2gcc", "first()\nsecond()\n", 0},
		{"/* first() */ second()\n", "gcc", "// /* first() */ second()\n", 0},
		{"first()\nsecond()\nlast()\n", "gc1G", "/* first()\nsecond() */\nlast()\n", 1},
		{"first()\nsecond()\nlast()\n", "gcG", "first()\n/* second()\nlast() */\n", 1},
	} {
		t.Run(tc.keys, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "test.go")
			if err := os.WriteFile(path, []byte(tc.original), 0600); err != nil {
				t.Fatal(err)
			}
			m, err := NewEditor(context.Background(), path, false)
			if err != nil {
				t.Fatal(err)
			}
			defer m.Close()
			doc := m.document()
			doc.SetPosition(tc.row, 0)
			for _, key := range tc.keys {
				press(m, string(key))
			}
			if doc.Text != tc.want || m.tabs[m.activeTab].vim.VisualMode {
				t.Fatalf("text = %q, want %q, visual = %t", doc.Text, tc.want, m.tabs[m.activeTab].vim.VisualMode)
			}
			press(m, "u")
			if doc.Text != tc.original {
				t.Fatalf("undo = %q, want %q", doc.Text, tc.original)
			}
		})
	}
}

func TestCommentShortcut(t *testing.T) {
	for _, mode := range []string{"normal", "visual", "insert", "plain"} {
		t.Run(mode, func(t *testing.T) {
			const original = "first()\nsecond()\n"
			path := filepath.Join(t.TempDir(), "test.go")
			if err := os.WriteFile(path, []byte(original), 0600); err != nil {
				t.Fatal(err)
			}
			m, err := NewEditor(context.Background(), path, false)
			if err != nil {
				t.Fatal(err)
			}
			defer m.Close()
			doc := m.document()
			want := "// first()\nsecond()\n"
			switch mode {
			case "visual":
				press(m, "V")
				press(m, "j")
				want = "/* first()\nsecond() */\n"
			case "insert":
				press(m, "i")
			case "plain":
				m.setVimDisabled(true)
				doc.SelectRange(0, doc.LineOffset(1))
			}
			press(m, "ctrl+/")
			vim := &m.tabs[m.activeTab].vim
			if doc.Text != want || vim.VisualMode || vim.InsertMode != (mode == "insert") {
				t.Fatalf("text = %q, want %q, mode = %s", doc.Text, want, vim.Label())
			}
			press(m, "ctrl+_")
			if doc.Text != want {
				t.Fatalf("Ctrl+_ changed text to %q", doc.Text)
			}
			press(m, "ctrl+/")
			if doc.Text != original {
				t.Fatalf("uncomment = %q, want %q", doc.Text, original)
			}
		})
	}
}

func TestVimSaveAndQuitCommands(t *testing.T) {
	for _, tc := range []struct {
		command string
		saves   bool
		closes  bool
		warns   bool
	}{
		{command: "w", saves: true},
		{command: "w!", saves: true},
		{command: "q", warns: true},
		{command: "q!", closes: true},
		{command: "wq", saves: true, closes: true},
		{command: "wq!", saves: true, closes: true},
	} {
		t.Run(tc.command, func(t *testing.T) {
			m := tabWorkspace(t, 2)
			defer m.Close()
			m.openFile("file0.go")
			other := m.document()
			other.Insert("// keep unsaved\n")
			m.openFile("file1.go")
			doc := m.document()
			original := doc.Text
			doc.Insert("// changed\n")

			typeVimCommand(m, tc.command)
			rows := m.filteredActions()
			if len(rows) != 1 || rows[0].label == "" || !strings.Contains(plain(m.pickerRow(rows[0], 0, 70, -1)), rows[0].label) {
				t.Fatal("picker did not display the command's action label")
			}
			drain(t, m, press(m, "enter"))
			if m.modal != "" || (m.document() == other) != tc.closes || !other.Dirty() {
				t.Fatal("command changed the wrong editor or left a dialog open")
			}
			if strings.Contains(m.notice, ":q!") != tc.warns {
				t.Fatalf("unexpected notice %q", m.notice)
			}
			want := original
			if tc.saves {
				want = doc.Text
			}
			data, err := os.ReadFile(doc.Path)
			if err != nil || string(data) != want {
				t.Fatalf("disk = %q, want %q, error = %v", data, want, err)
			}
		})
	}
}

func TestVimQuitCommandInStandaloneEditor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := NewEditor(context.Background(), path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	typeVimCommand(m, "q")
	cmd := press(m, "enter")
	if cmd == nil || m.document() != nil {
		t.Fatal("quit did not close the standalone editor")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("quit did not exit vl")
	}
}

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
				if !hasSaveConfirmation(m) || m.document() != d {
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
	typeVimCommand(m, "wq!")
	drain(t, m, press(m, "enter"))
	if m.document() != d || !d.Dirty() {
		t.Fatal(":wq! closed tab after a save conflict")
	}
	data, _ := os.ReadFile(d.Path)
	if string(data) != "external" {
		t.Fatal("save conflict overwrote disk")
	}
}
func TestVimMappedFocusAndClearSearch(t *testing.T) {
	m := newHoverRenderModel(t)
	d := &editor.Document{Buffer: editor.Buffer{Text: "one two", Anchor: -1}}
	m.tabs = []tab{{doc: d}}
	m.activeTab = 0
	m.sidebar = false

	setTestFind(m, "one")
	d.SelectRange(0, 3)
	press(m, "ctrl+f")
	press(m, "esc")
	if !m.findHidden || m.findWidget.query.Value() != "one" {
		t.Fatal("closing Find did not hide highlights and preserve the query")
	}
	d.SetPosition(0, 0)
	press(m, "c")
	press(m, "w")
	if m.sidebar || !m.tabs[0].vim.InsertMode || d.Text != " two" {
		t.Fatal("cw did not change the word in the editor")
	}
	press(m, "esc")
	m.Update(tea.KeyPressMsg{Code: tea.KeyF2})
	if !m.sidebar || !m.sidebarVisible {
		t.Fatal("F2 did not focus sidebar")
	}
}
func TestVimUnnamedRegister(t *testing.T) {
	m := flashModel(t, "one\ntwo")
	source := m.document()
	for _, key := range []string{"d", "d", "p"} {
		handled, cmd := m.vimKey(key)
		if !handled || cmd != nil {
			t.Fatalf("%s did not stay in the Vim editor", key)
		}
	}
	if source.Text != "two\none" || m.vimRegister.Text != "one\n" {
		t.Fatalf("ddp = %q, register = %q", source.Text, m.vimRegister.Text)
	}
	target := &editor.Document{Buffer: editor.Buffer{Text: "three", Anchor: -1}}
	m.tabs = append(m.tabs, tab{doc: target})
	m.activeTab = 1
	if handled, cmd := m.vimKey("P"); !handled || cmd != nil || target.Text != "one\nthree" {
		t.Fatalf("put across tabs: %q", target.Text)
	}
}

func TestVimReplaceWithSpace(t *testing.T) {
	m := flashModel(t, "abc")
	press(m, "r")
	press(m, " ")
	if m.document().Text != " bc" || m.leaderActive() {
		t.Fatalf("text %q, leader active %t", m.document().Text, m.leaderActive())
	}
}

func TestVimLeaderClipboardOperators(t *testing.T) {
	for _, tt := range []struct {
		keys, text, copied string
		insert, readOnly   bool
	}{
		{" dd", "two", "one\n", false, false},
		{" cc", "\ntwo", "one\n", true, false},
		{" yy", "one\ntwo", "one\n", false, true},
		{" yiw", "one\ntwo", "one", false, false},
		{`"+yiw`, "one\ntwo", "one", false, false},
		{`"+yiw`, "one\ntwo", "one", false, true},
		{" yw", "one\ntwo", "one", false, true},
		{" diw", "\ntwo", "one", false, false},
		{" ciw", "\ntwo", "one", true, false},
		{"v d", "ne\ntwo", "o", false, false},
		{"V c", "\ntwo", "one\n", true, false},
		{"V y", "one\ntwo", "one\n", false, true},
		{"v x", "ne\ntwo", "o", false, false},
		{" dd", "one\ntwo", "", false, true},
		{" dP", "one\ntwo", "", false, false},
	} {
		t.Run(tt.keys, func(t *testing.T) {
			m := flashModel(t, "one\ntwo")
			d := m.document()
			d.ReadOnly = tt.readOnly
			var cmd tea.Cmd
			for _, key := range tt.keys {
				if cmd != nil {
					t.Fatal("clipboard command started before the sequence completed")
				}
				cmd = press(m, string(key))
			}
			v := &m.tabs[0].vim
			if d.Text != tt.text || m.vimRegister.Text != tt.copied || (cmd != nil) != (tt.copied != "") ||
				v.InsertMode != tt.insert || v.VisualMode || v.RegisterPending() || m.leaderActive() {
				t.Fatalf("text = %q, register = %q, clipboard command = %t, mode = %s, leader = %v",
					d.Text, m.vimRegister.Text, cmd != nil, v.Label(), m.leaderActive())
			}
		})
	}
}

func TestVimClipboardRegisterEndsWithCommand(t *testing.T) {
	m := flashModel(t, "one two")
	for _, keys := range []string{`"+h`, `"+yi`, " yi"} {
		for _, key := range keys {
			press(m, string(key))
		}
		press(m, "esc")
		press(m, "y")
		press(m, "i")
		if cmd := press(m, "w"); cmd != nil || m.vimRegister.Text != "one" {
			t.Fatalf("%q left the clipboard selected for the next command", keys)
		}
	}
}

func TestVimClipboardOperatorCancelsOnBlur(t *testing.T) {
	m := flashModel(t, "one two")
	press(m, " ")
	press(m, "y")
	m.Update(tea.BlurMsg{})
	if m.leaderActive() || m.tabs[0].vim.CommandPending() {
		t.Fatal("clipboard operator survived window blur")
	}
}

func TestVimClipboardOperatorFollowsFlash(t *testing.T) {
	m := flashModel(t, "one two")
	press(m, " ")
	press(m, "y")
	flashQuery(m, "two")
	if m.flash == nil || len(m.flash.targets) != 1 {
		t.Fatal("expected one Flash target")
	}
	if cmd := press(m, m.flash.targets[0].label); cmd == nil || m.vimRegister.Text != "one t" || m.leaderActive() {
		t.Fatalf("clipboard command = %t, register = %q, leader = %t", cmd != nil, m.vimRegister.Text, m.leaderActive())
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
	for _, step := range []struct {
		key  rune
		want int
	}{
		{tea.KeyLeft, 2},
		{tea.KeyRight, 0},
		{tea.KeyRight, 1},
		{tea.KeyRight, 2},
		{tea.KeyRight, 0},
		{tea.KeyLeft, 2},
		{tea.KeyLeft, 1},
	} {
		press(m, " ")
		m.Update(tea.KeyPressMsg{Code: step.key})
		if m.activeTab != step.want || m.diffOpen != (step.want == 2) {
			t.Fatalf("Space %s: active tab %d, want %d", (tea.KeyPressMsg{Code: step.key}).String(), m.activeTab, step.want)
		}
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
	m.Update(tea.KeyPressMsg{Code: tea.KeyF2})
	m.Update(tea.KeyPressMsg{Code: tea.KeyF2})
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
	if !hasSaveConfirmation(m) || m.document() != document {
		t.Fatal("editor close bypassed unsaved prompt")
	}
}
