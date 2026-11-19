package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestPickerListPagingResizeAndMouseTarget(t *testing.T) {
	m := tabWorkspace(t, 0)
	m.Update(tea.WindowSizeMsg{Width: 48, Height: 12})
	m.workspaces = nil
	for i := 0; i < 20; i++ {
		m.workspaces = append(m.workspaces, fmt.Sprintf("/workspace/目录%d", i))
	}
	m.openRecentWorkspaces()
	press(m, "pgdown")
	if m.pickerIndex != 5 {
		t.Fatalf("page down selected %d", m.pickerIndex)
	}
	press(m, "end")
	if m.pickerIndex != 19 {
		t.Fatal("end did not reach last folder")
	}
	for _, size := range [][2]int{{30, 8}, {120, 36}, {48, 12}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		lines := strings.Split(m.View().Content, "\n")
		if len(lines) != m.height || m.pickerIndex != 19 {
			t.Fatal("resize changed frame size or selected folder")
		}
		for _, line := range lines {
			if ansi.StringWidth(line) != m.width {
				t.Fatalf("frame width: %q", plain(line))
			}
		}
		layout := m.pickerLayout(m.filteredActions())
		if !strings.Contains(plain(lines[layout.y+3+19-layout.start]), "目录19") {
			t.Fatal("selected folder not visible")
		}
	}
	picked := -1
	for i := range m.palette {
		m.palette[i].run = func() tea.Cmd { picked = i; return nil }
	}
	layout := m.pickerLayout(m.filteredActions())
	m.Update(tea.MouseClickMsg{X: layout.x + 2, Y: layout.y + 3, Button: tea.MouseLeft})
	if picked != layout.start || m.modal != "" {
		t.Fatalf("click selected %d, expected %d", picked, layout.start)
	}
}

func TestPickerListSearchTypingAndPaste(t *testing.T) {
	m := tabWorkspace(t, 0)
	m.workspaces = []string{"/work/alpha", "/work/jk", "/work/zulu"}
	m.openRecentWorkspaces()
	press(m, "j")
	press(m, "k")
	if m.input.Value() != "jk" || m.pickerIndex != 0 || len(m.filteredActions()) != 1 {
		t.Fatal("list shortcuts captured search text")
	}
	m.input.SetValue("")
	m.Update(tea.PasteMsg{Content: "zulu"})
	if m.input.Value() != "zulu" || m.pickerIndex != 0 {
		t.Fatal("paste did not filter and reset selection")
	}
	rows := m.filteredActions()
	if len(rows) != 1 || rows[0].id != "/work/zulu" {
		t.Fatal("paste filtered the wrong folder")
	}
	press(m, "esc")
	if m.modal != "" || m.input.Focused() {
		t.Fatal("dismissal left input focused")
	}
}

func TestPickerListModesFilterAndExecuteVisibleAction(t *testing.T) {
	for _, mode := range []string{"quick", "refs", "remotes", "recent"} {
		t.Run(mode, func(t *testing.T) {
			m := tabWorkspace(t, 0)
			m.Update(tea.WindowSizeMsg{Width: 48, Height: 12})
			picked := -1
			for i := range 30 {
				m.palette = append(m.palette, action{id: fmt.Sprintf("item%d", i), label: fmt.Sprintf("item%d", i), run: func() tea.Cmd { picked = i; return nil }})
			}
			query, title := "", "Choose"
			if mode == "quick" {
				query, title = ">", "Commands"
			}
			m.prompt(mode, title, query)
			press(m, "pgdown")
			if m.pickerIndex != 5 {
				t.Fatal("paging did not use the shared list")
			}
			press(m, "end")
			m.View()
			l := m.pickerLayout(m.filteredActions())
			m.Update(tea.MouseClickMsg{X: l.x + 2, Y: l.y + 3, Button: tea.MouseLeft})
			if picked != 25 || m.modal != "" {
				t.Fatal("paged click executed the wrong action")
			}
			m.prompt(mode, title, query)
			m.Update(tea.PasteMsg{Content: "item27"})
			if len(m.filteredActions()) != 1 || m.pickerIndex != 0 {
				t.Fatal("filter did not reset the list")
			}
			for _, line := range strings.Split(m.View().Content, "\n") {
				if ansi.StringWidth(line) != m.width {
					t.Fatal("filtered picker changed frame width")
				}
			}
			press(m, "enter")
			if picked != 27 || m.modal != "" {
				t.Fatal("filtered enter executed the wrong action")
			}
		})
	}
}

func TestPickerContinuousScrolling(t *testing.T) {
	m := tabWorkspace(t, 0)
	m.Update(tea.WindowSizeMsg{Width: 48, Height: 12})
	for i := range 23 {
		m.workspaces = append(m.workspaces, fmt.Sprintf("/workspace/folder%02d", i))
	}
	m.openRecentWorkspaces()
	check := func(selected, top int) {
		t.Helper()
		layout := m.pickerLayout(m.filteredActions())
		if m.pickerIndex != selected || layout.start != top || layout.count != 5 {
			t.Fatalf("selected=%d viewport=%+v; want selected=%d top=%d count=5", m.pickerIndex, layout, selected, top)
		}
		lines := strings.Split(m.View().Content, "\n")
		for i := range layout.count {
			want := fmt.Sprintf("folder%02d", top+i)
			if !strings.Contains(plain(lines[layout.y+3+i]), want) {
				t.Fatalf("visible row %d missing %s", i, want)
			}
		}
	}
	for range 4 {
		press(m, "down")
	}
	check(4, 0)
	press(m, "down")
	check(5, 1)
	press(m, "up")
	check(4, 1)
	press(m, "ctrl+d")
	check(6, 3)
	press(m, "ctrl+u")
	check(4, 1)
	press(m, "end")
	check(22, 18)
	press(m, "pgup")
	check(17, 17)
	press(m, "pgdown")
	check(22, 18)
	press(m, "home")
	check(0, 0)
	press(m, "up")
	check(0, 0)
	press(m, "end")
	picked := -1
	for i := range m.palette {
		m.palette[i].run = func() tea.Cmd { picked = i; return nil }
	}
	layout := m.pickerLayout(m.filteredActions())
	m.Update(tea.MouseClickMsg{X: layout.x + 2, Y: layout.y + 3, Button: tea.MouseLeft})
	if picked != 18 {
		t.Fatalf("bottom viewport click selected %d, want 18", picked)
	}
}

func TestPickerWheelScrollsOnlyList(t *testing.T) {
	for _, mode := range []string{"quick", "refs", "remotes", "recent"} {
		t.Run(mode, func(t *testing.T) {
			m := tabWorkspace(t, 0)
			defer m.Close()
			m.Update(tea.WindowSizeMsg{Width: 48, Height: 12})
			picked := -1
			for i := range 23 {
				m.palette = append(m.palette, action{id: fmt.Sprint(i), label: fmt.Sprintf("item%02d", i), run: func() tea.Cmd { picked = i; return nil }})
			}
			query, title := "", "Choose"
			if mode == "quick" {
				query, title = ">", "Commands"
			}
			m.prompt(mode, title, query)
			l := m.pickerLayout(m.filteredActions())
			before := strings.Split(m.View().Content, "\n")
			wheel := func(x, y int, button tea.MouseButton) {
				m.Update(tea.MouseWheelMsg{X: x, Y: y, Button: button})
			}
			for _, point := range [][2]int{{l.x, l.y}, {l.x + 2, l.y + 1}, {l.x - 1, l.y + 3}, {l.x + l.width, l.y + 3}, {l.x, l.y + 3 + l.count}} {
				wheel(point[0], point[1], tea.MouseWheelDown)
			}
			if m.pickerTop != 0 || m.pickerIndex != 0 {
				t.Fatal("wheel outside list moved viewport or selection")
			}
			for range 7 {
				wheel(l.x+2, l.y+3, tea.MouseWheelDown)
			}
			after := strings.Split(m.View().Content, "\n")
			if m.pickerTop != 7 || m.pickerIndex != 0 || m.input.Value() != query || !strings.Contains(plain(after[l.y+3]), "item07") {
				t.Fatal("wheel did not scroll viewport independently of selection")
			}
			for i := range before {
				if (i < l.y+3 || i >= l.y+3+l.count) && before[i] != after[i] {
					t.Fatalf("wheel changed frame outside list at row %d", i)
				}
			}
			wheel(l.x+2, l.y+3, tea.MouseWheelLeft)
			if m.pickerTop != 7 {
				t.Fatal("horizontal wheel changed viewport")
			}
			for range 30 {
				wheel(l.x+2, l.y+3, tea.MouseWheelDown)
			}
			if m.pickerTop != 18 {
				t.Fatal("wheel did not clamp at bottom")
			}
			wheel(l.x+2, l.y+3, tea.MouseWheelUp)
			if m.pickerTop != 17 {
				t.Fatal("wheel up did not scroll list")
			}
			press(m, "down")
			if m.pickerIndex != 1 || m.pickerTop != 1 {
				t.Fatal("keyboard did not reveal selection after wheel scrolling")
			}
			wheel(l.x+2, l.y+3, tea.MouseWheelDown)
			m.Update(tea.MouseClickMsg{X: l.x + 2, Y: l.y + 3, Button: tea.MouseLeft})
			if picked != 2 || m.modal != "" {
				t.Fatalf("click after wheel selected %d, want visible item 2", picked)
			}
		})
	}
}
