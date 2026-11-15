package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/elegit/internal/diff"
	"github.com/ourongxing/elegit/internal/theme"
)

func TestThemeSwitchRejectsOldSyntaxAndPreservesEditing(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openFile("file0.go")
	old := m.scheduleHighlight()().(highlightStartMsg).request
	m.editorLines() // populate colored row cache
	text, cursor := m.document().Text, m.document().Cursor
	drain(t, m, m.SetThemeMode("light"))
	if old.ctx.Err() == nil || m.startHighlight(old) != nil {
		t.Fatal("old-theme syntax was accepted")
	}
	if m.document().Text != text || m.document().Cursor != cursor {
		t.Fatal("theme switch changed document")
	}
	if !m.light || m.colors != theme.Light() {
		t.Fatal("light palette not applied")
	}
	if len(m.tabs[0].spans) == 0 || m.tabs[0].spans[0][0].Color != "#0000FF" {
		t.Fatal("editor retained dark syntax")
	}
	if !sameColor(m.View().BackgroundColor, lipgloss.Color(m.colors.Panel)) {
		t.Fatal("terminal background retained dark theme")
	}
	drain(t, m, m.SetThemeMode("dark"))
	if m.colors != theme.Dark() || m.tabs[0].spans[0][0].Color != "#569CD6" {
		t.Fatal("switching back lost dark theme")
	}
}

func TestLightThemePickerAndDiffSurfaces(t *testing.T) {
	m := tabWorkspace(t, 0)
	m.SetThemeMode("light")
	m.openPalette()
	m.input.SetValue("Color Theme")
	rows := m.filteredActions()
	if len(rows) != 3 {
		t.Fatal("theme choices are not discoverable")
	}
	base := make([]string, m.height)
	for i := range base {
		base[i] = strings.Repeat(" ", m.width)
	}
	layout := m.pickerLayout(rows)
	painted := m.pickerOverlay(base)
	cells := uv.NewStyledString(painted[layout.y+3]).Lines(ansi.GraphemeWidth)[0]
	if !sameColor(cells[layout.x+2].Style.Fg, lipgloss.Color(m.colors.SelectionForeground)) || !sameColor(cells[layout.x+2].Style.Bg, lipgloss.Color(m.colors.Selection)) {
		t.Fatal("picker selection is not light-themed")
	}
	m.diffPath = "file.go"
	m.patch = diff.Parse("--- a/file.go\n+++ b/file.go\n@@ -1 +1 @@\n-var old = 1\n+var value = 2\n")
	m.renderDiff()
	oldSyntax := m.highlightDiff()()
	drain(t, m, m.SetThemeMode("dark"))
	m.Update(oldSyntax)
	for _, spans := range m.diffSyntax {
		for _, span := range spans {
			if span.Color == "#0000FF" {
				t.Fatal("stale light diff syntax replaced dark syntax")
			}
		}
	}
	drain(t, m, m.SetThemeMode("light"))
	for i := range m.patch.Lines {
		line := &m.patch.Lines[i]
		if line.Kind != diff.Added {
			continue
		}
		cells := uv.NewStyledString(m.diffRow(line, 50, true)).Lines(ansi.GraphemeWidth)[0]
		if !sameColor(cells[0].Style.Bg, lipgloss.Color("#FFFFFF")) {
			t.Fatal("light diff gutter is not white")
		}
		if !sameColor(cells[49].Style.Bg, lipgloss.Color(m.colors.AddedBackground)) {
			t.Fatal("light diff background missing")
		}
	}
}

func TestAutomaticThemeTracksTerminalAndHonorsOverride(t *testing.T) {
	m := tabWorkspace(t, 0)
	if m.ThemeMode() != "auto" {
		t.Fatal("default theme is not automatic")
	}
	if m.View().BackgroundColor != nil {
		t.Fatal("auto mode overrides the terminal background")
	}
	m.Update(uv.LightColorSchemeEvent{})
	if !m.light || m.ThemeMode() != "auto" {
		t.Fatal("light terminal did not select light palette automatically")
	}
	m.Update(uv.DarkColorSchemeEvent{})
	if m.light {
		t.Fatal("terminal change did not select dark palette")
	}
	m.SetThemeMode("light")
	m.Update(uv.DarkColorSchemeEvent{})
	if !m.light || m.ThemeMode() != "light" {
		t.Fatal("automatic report replaced explicit preference")
	}
	m.SetThemeMode("auto")
	m.Update(uv.DarkColorSchemeEvent{})
	if m.light || m.ThemeMode() != "auto" {
		t.Fatal("returning to auto did not restore tracking")
	}
	// Remember notifications even while an explicit palette is selected.
	m.SetThemeMode("dark")
	m.Update(uv.LightColorSchemeEvent{})
	if m.light {
		t.Fatal("notification replaced explicit dark palette")
	}
	m.SetThemeMode("auto")
	if !m.light {
		t.Fatal("auto did not apply the latest terminal notification")
	}
	m.width, m.height = 20, 5
	if m.View().BackgroundColor != nil {
		t.Fatal("small auto view overrides terminal background")
	}
}

func TestThemeNotificationSubscriptionAndRestore(t *testing.T) {
	for _, check := range []struct {
		name            string
		initial         ansi.ModeSetting
		output, restore string
	}{
		{"disabled", ansi.ModeReset, ansi.SetModeLightDark + ansi.RequestLightDarkReport, ansi.ResetModeLightDark},
		{"enabled", ansi.ModeSet, ansi.RequestLightDarkReport, ""},
		{"always enabled", ansi.ModePermanentlySet, ansi.RequestLightDarkReport, ""},
		{"unsupported", ansi.ModeNotRecognized, "", ""},
		{"always disabled", ansi.ModePermanentlyReset, "", ""},
	} {
		t.Run(check.name, func(t *testing.T) {
			m := tabWorkspace(t, 0)
			_, cmd := m.Update(tea.ModeReportMsg{Mode: ansi.ModeLightDark, Value: check.initial})
			if check.output == "" {
				if cmd != nil {
					t.Fatal("unsupported terminal received a subscription")
				}
			} else if cmd == nil || cmd().(tea.RawMsg).Msg != check.output {
				t.Fatal("wrong subscription/query sequence")
			}
			if m.ThemeNotificationRestoreSequence() != check.restore {
				t.Fatal("cleanup did not preserve original notification mode")
			}
			_, duplicate := m.Update(tea.ModeReportMsg{Mode: ansi.ModeLightDark, Value: check.initial})
			if duplicate != nil {
				t.Fatal("duplicate mode report resubscribed")
			}
		})
	}
	m := tabWorkspace(t, 0)
	_, cmd := m.Update(tea.ModeReportMsg{Mode: ansi.ModeFocusEvent, Value: ansi.ModeReset})
	if cmd != nil || m.ThemeNotificationRestoreSequence() != "" {
		t.Fatal("unrelated mode enabled theme notifications")
	}
}
