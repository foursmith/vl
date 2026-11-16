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
	m.openRecentFolders()
	press(m, "pgdown")
	if m.pickerList.Index() != 5 {
		t.Fatalf("page down selected %d", m.pickerList.Index())
	}
	press(m, "end")
	if m.pickerList.Index() != 19 {
		t.Fatal("end did not reach last folder")
	}
	for _, size := range [][2]int{{30, 8}, {120, 36}, {48, 12}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		lines := strings.Split(m.View().Content, "\n")
		if len(lines) != m.height || m.pickerList.Index() != 19 {
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

func TestPickerListSearchTypingPasteAndWheel(t *testing.T) {
	m := tabWorkspace(t, 0)
	m.workspaces = []string{"/work/alpha", "/work/jk", "/work/zulu"}
	m.openRecentFolders()
	m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	if m.pickerList.Index() != 1 {
		t.Fatal("wheel did not select next folder")
	}
	press(m, "j")
	press(m, "k")
	if m.input.Value() != "jk" || m.pickerList.Index() != 0 || len(m.filteredActions()) != 1 {
		t.Fatal("list shortcuts captured search text")
	}
	m.input.SetValue("")
	m.Update(tea.PasteMsg{Content: "zulu"})
	if m.input.Value() != "zulu" || m.pickerList.Index() != 0 {
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
			if m.pickerList.Index() != 5 {
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
			if len(m.pickerList.Items()) != 1 || m.pickerList.Index() != 0 {
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
