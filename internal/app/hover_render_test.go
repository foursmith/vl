package app

import (
	"context"
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	gitservice "github.com/foursmith/vl/internal/git"
	"github.com/foursmith/vl/internal/theme"
)

func newHoverRenderModel(t testing.TB) *Model {
	t.Helper()
	m := New(context.Background(), &gitservice.Repository{Root: t.TempDir()}, false)
	t.Cleanup(m.cancel)
	m.Update(tea.WindowSizeMsg{Width: 180, Height: 55})
	for i := 0; i < 100; i++ {
		name := fmt.Sprintf("file%03d.go", i)
		m.items = append(m.items, item{file: name, label: name, key: name})
	}
	m.patch = benchmarkPatch()
	m.diffPath = "file.go"
	m.renderDiff()
	return m
}

func assertFreshHoverFrame(t *testing.T, m *Model) {
	t.Helper()
	got := m.View()
	fresh := *m
	fresh.hoverOnly = false
	if want := fresh.View(); got.Content != want.Content {
		t.Fatal("cached hover frame differs from a full render")
	}
}

func TestAllSidebarTreesShareHoverRendering(t *testing.T) {
	for _, section := range []section{explorer, changes, history} {
		t.Run(fmt.Sprint(section), func(t *testing.T) {
			m := newHoverRenderModel(t)
			m.section = section
			change := gitservice.Change{Path: "src/main.go", Index: 'M', Worktree: 'M'}
			m.snapshot = gitservice.Snapshot{Changes: []gitservice.Change{change}}
			m.items = []item{
				{folder: "src", label: "src", historyID: "commit"},
				{file: change.Path, label: "main.go", change: &change},
			}
			if section == history {
				m.items[1].change = nil
				m.items[1].historyChange = &change
			}
			m.View()
			for i := range m.items {
				y := headerHeight + i
				m.Update(tea.MouseMotionMsg{X: 8, Y: y})
				assertFreshHoverFrame(t, m)
				cells := uv.NewStyledString(m.sidebarLines()[y]).Lines(ansi.GraphemeWidth)[0]
				for _, cell := range cells {
					if !sameColor(cell.Style.Bg, lipgloss.Color(theme.Builtin("dark_modern").Palette().Hover)) {
						t.Fatal("tree row did not use the shared hover background")
					}
				}
				if m.selected != -1 {
					t.Fatal("hover changed the selected row")
				}
			}
			m.Update(tea.MouseMotionMsg{X: m.mainX() + 8, Y: headerHeight})
			assertFreshHoverFrame(t, m)
			if m.hoveredSidebarRow() != -1 {
				t.Fatal("tree retained hover after leaving sidebar")
			}
		})
	}
}

func TestHoverFramesMatchFullRender(t *testing.T) {
	for _, width := range []int{60, 180} {
		for _, right := range []bool{false, true} {
			t.Run(fmt.Sprintf("width%d/right%v", width, right), func(t *testing.T) {
				m := newHoverRenderModel(t)
				m.SetSidebarLayout(38, right)
				m.Update(tea.WindowSizeMsg{Width: width, Height: 55})
				m.View()
				for _, msg := range []tea.Msg{
					tea.MouseMotionMsg{X: m.sideX() + 8, Y: 3},
					tea.MouseMotionMsg{X: m.sideX() + 9, Y: 3},
					tea.MouseMotionMsg{X: m.sideX() + 9, Y: 4},
					tea.MouseMotionMsg{X: m.dividerX(), Y: 4},
					tea.MouseMotionMsg{X: m.mainX() + 8, Y: 4},
					tea.MouseWheelMsg{X: m.sideX() + 8, Y: 4, Button: tea.MouseWheelDown},
					tea.MouseMotionMsg{X: m.sideX() + 8, Y: 4},
					tea.MouseWheelMsg{X: m.mainX() + 8, Y: 4, Button: tea.MouseWheelDown},
					tea.MouseMotionMsg{X: m.sideX() + 8, Y: 4},
					tea.KeyPressMsg{Code: tea.KeyF6},
					tea.MouseMotionMsg{X: m.sideX() + 8, Y: 5},
					tea.BlurMsg{},
					tea.WindowSizeMsg{Width: 120, Height: 30},
					tea.MouseMotionMsg{X: 8, Y: 4},
				} {
					m.Update(msg)
					assertFreshHoverFrame(t, m)
				}
			})
		}
	}
}

func TestHoverActionFramesMatchFullRender(t *testing.T) {
	m := explorerWorkspace(t)
	drain(t, m, m.selectSection(changes))
	m.View()
	for i, entry := range m.items {
		y := headerHeight + i - m.sideTop
		for _, action := range m.changeRowActions(entry, m.sideWidth()) {
			m.Update(tea.MouseMotionMsg{X: m.sideX() + action.x + 1, Y: y})
			assertFreshHoverFrame(t, m)
		}
	}
	// A non-mouse result must discard the entire cache before the next hover.
	m.Update(operationMsg{title: "Test"})
	if m.hoverRender.main != nil {
		t.Fatal("operation result retained hover cache")
	}
	assertFreshHoverFrame(t, m)
}

func TestHoverAfterEditorUpdatesMatchesFullRender(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openFile("file0.go")
	m.sidebar = false
	m.View()
	for _, msg := range []tea.Msg{
		tea.MouseMotionMsg{X: 8, Y: 3},
		tea.KeyPressMsg{Code: 'x', Text: "x"},
		tea.MouseMotionMsg{X: 8, Y: 3},
		tea.MouseWheelMsg{X: m.mainX() + 8, Y: 4, Button: tea.MouseWheelDown},
		tea.MouseMotionMsg{X: 8, Y: 4},
	} {
		m.Update(msg)
		assertFreshHoverFrame(t, m)
	}
}

func BenchmarkHoverFrame(b *testing.B) {
	for _, cached := range []bool{false, true} {
		for _, sameRow := range []bool{false, true} {
			b.Run(fmt.Sprintf("cached%v/sameRow%v", cached, sameRow), func(b *testing.B) {
				m := newHoverRenderModel(b)
				m.View()
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					y := 3
					if !sameRow {
						y += i % 45
					}
					m.Update(tea.MouseMotionMsg{X: 8 + i%2, Y: y})
					if !cached {
						m.hoverOnly = false
					}
					m.View()
				}
			})
		}
	}
}

func TestPickerHoverUsesVisibleRowsWithoutChangingSelection(t *testing.T) {
	for _, mode := range []string{"dark_modern", "light_modern"} {
		t.Run(mode, func(t *testing.T) {
			m := newHoverRenderModel(t)
			m.setTestTheme(mode)
			m.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
			m.workspaces = nil
			for i := range 23 {
				m.workspaces = append(m.workspaces, fmt.Sprintf("/workspace/folder%02d", i))
			}
			m.openRecentWorkspaces()
			press(m, "end")
			layout := m.pickerLayout(m.filteredActions())
			m.View()
			for i := range layout.count {
				y := layout.y + 3 + i
				m.Update(tea.MouseMotionMsg{X: layout.x + 2, Y: y})
				assertFreshHoverFrame(t, m)
				if m.hoveredPickerIndex() != layout.start+i || m.pickerIndex != 22 {
					t.Fatal("hover did not track the visible row or changed selection")
				}
				bg := m.colors.Hover
				if layout.start+i == 22 {
					bg = m.colors.PopupSelection
				}
				cells := uv.NewStyledString(m.View().Content).Lines(ansi.GraphemeWidth)[y]
				for _, cell := range cells[layout.x+1 : layout.x+layout.width-1] {
					if !sameColor(cell.Style.Bg, lipgloss.Color(bg)) {
						t.Fatal("picker row has incorrect hover or selection background")
					}
				}
			}
			for _, point := range [][2]int{{layout.x - 1, 4}, {layout.x, 3}, {layout.x, 9}} {
				m.Update(tea.MouseMotionMsg{X: point[0], Y: point[1]})
				assertFreshHoverFrame(t, m)
				if m.hoveredPickerIndex() != -1 {
					t.Fatal("hover persisted outside list rows")
				}
			}
		})
	}
}
