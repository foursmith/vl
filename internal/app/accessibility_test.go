package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

func TestMonochromeSelectionRemainsVisibleWithoutColor(t *testing.T) {
	m := tabWorkspace(t, 2)
	defer m.Close()
	m.Update(tea.ColorProfileMsg{Profile: colorprofile.ASCII})
	m.openPalette()
	before := plain(m.View().Content)
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	after := plain(m.View().Content)
	if before == after || !strings.Contains(after, "> ") {
		t.Fatal("command selection is invisible without colors")
	}
	// The marker occupies existing padding; result hit testing stays aligned.
	l := m.pickerLayout(m.filteredActions())
	if !strings.HasPrefix(strings.Split(after, "\n")[l.y+4][l.x:], ">") {
		t.Fatal("marker did not follow the selected command")
	}
	press(m, "esc")
	side := explorerWorkspace(t)
	defer side.Close()
	side.Update(tea.ColorProfileMsg{Profile: colorprofile.ASCII})
	side.selected = 0
	before = plain(side.View().Content)
	side.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if before == plain(side.View().Content) {
		t.Fatal("sidebar selection is invisible without colors")
	}
	m.Update(tea.ColorProfileMsg{Profile: colorprofile.TrueColor})
	if m.monochrome {
		t.Fatal("color capability change did not restore normal presentation")
	}
}

func TestConfirmationLongUnicodeTargetCanBeReadBeforeConfirming(t *testing.T) {
	m := tabWorkspace(t, 0)
	defer m.Close()
	title := "删除未跟踪文件 " + strings.Repeat("目录👨‍👩‍👧‍👦/", 30) + "重要文件.txt？"
	for _, size := range [][2]int{{120, 36}, {60, 20}, {30, 8}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		confirmed := false
		m.ask(title, func() tea.Cmd { confirmed = true; return nil })
		m.confirmationDetail = "Git 无法恢复此文件。"
		var seen strings.Builder
		readLines := 0
		for {
			l := m.dialogLayout()
			frame := strings.Split(m.View().Content, "\n")
			if len(frame) != size[1] {
				t.Fatal("dialog escaped terminal height")
			}
			for _, line := range frame {
				if ansi.StringWidth(line) != size[0] {
					t.Fatal("wrapped Unicode dialog escaped terminal width")
				}
			}
			for i, line := range l.content[:min(l.page, len(l.content))] {
				if l.top+i >= readLines {
					seen.WriteString(strings.TrimSpace(plain(line)))
					readLines++
				}
			}
			if !strings.Contains(plain(strings.Join(l.content, "\n")), "Esc") {
				t.Fatal("scrolling hid the cancellation hint")
			}
			if l.top == l.maxTop {
				break
			}
			m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
		}
		if !strings.Contains(strings.ReplaceAll(seen.String(), " ", ""), strings.ReplaceAll(title, " ", "")) || !strings.Contains(seen.String(), "无法恢复") {
			t.Fatalf("scrolling at %v did not expose the complete target and warning: %q", size, seen.String())
		}
		if confirmed {
			t.Fatal("reading confirmation performed the destructive action")
		}
		press(m, "esc")
		if confirmed || m.modal != "" {
			t.Fatal("confirmation could not be canceled after scrolling")
		}
	}
}

func TestWrappedUnsavedDialogButtonsMatchTheirVisiblePosition(t *testing.T) {
	m := tabWorkspace(t, 1)
	defer m.Close()
	m.openFile("file0.go")
	press(m, "x")
	m.Update(tea.WindowSizeMsg{Width: 30, Height: 8})
	m.closeTab()
	m.modalTitle = "Save changes to a/very/long/目录/important-file.go?"
	l := m.dialogLayout()
	if !strings.Contains(plain(l.content[len(l.content)-1]), "Cancel") {
		t.Fatal("narrow dialog clipped an action")
	}
	m.clickModal(tea.MouseClickMsg{X: l.x + 1 + 17, Y: l.y + len(l.content), Button: tea.MouseLeft})
	if m.modal != "" || m.document() == nil || !m.document().Dirty() {
		t.Fatal("visible cancel button discarded or closed the document")
	}
	m.closeTab()
	m.modalTitle = "Save changes to a/very/long/目录/important-file.go?"
	l = m.dialogLayout()
	m.clickModal(tea.MouseClickMsg{X: l.x + 1 + 8, Y: l.y + len(l.content), Button: tea.MouseLeft})
	if m.modal != "" || len(m.tabs) != 0 {
		t.Fatal("visible discard button did not close the dirty document")
	}
}
