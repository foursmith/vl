package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/vl/internal/config"
	"github.com/ourongxing/vl/internal/diff"
	"github.com/ourongxing/vl/internal/theme"
)

func (m *Model) setTestTheme(name, uiTheme string) tea.Cmd {
	selected, err := theme.Load(name, uiTheme)
	if err != nil {
		m.notice = m.tr("Load theme: %s", m.tr(err.Error()))
		return nil
	}
	return m.SetColorTheme(selected)
}

func TestThemeSwitchRejectsOldSyntaxAndPreservesEditing(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openFile("file0.go")
	old := m.scheduleHighlight()().(highlightStartMsg).request
	m.editorLines() // populate colored row cache
	text, cursor := m.document().Text, m.document().Cursor
	drain(t, m, m.setTestTheme("light_modern", ""))
	if old.ctx.Err() == nil || m.startHighlight(old) != nil {
		t.Fatal("old-theme syntax was accepted")
	}
	if m.document().Text != text || m.document().Cursor != cursor {
		t.Fatal("theme switch changed document")
	}
	if !m.colorTheme.IsLight() || m.colors != theme.Builtin("light_modern").Palette() {
		t.Fatal("light palette not applied")
	}
	if len(m.tabs[0].spans) == 0 || m.tabs[0].spans[0][0].Color != "#0000FF" {
		t.Fatal("editor retained dark syntax")
	}
	if !sameColor(m.View().BackgroundColor, lipgloss.Color(m.colors.Panel)) {
		t.Fatal("theme switch did not update terminal padding")
	}
	drain(t, m, m.setTestTheme("dark_modern", ""))
	if m.colors != theme.Builtin("dark_modern").Palette() || m.tabs[0].spans[0][0].Color != "#569CD6" {
		t.Fatal("switching back lost dark theme")
	}
}

func TestLightThemePickerAndDiffSurfaces(t *testing.T) {
	m := tabWorkspace(t, 0)
	m.setTestTheme("light_modern", "")
	m.openPalette()
	m.input.SetValue(">Color Theme")
	rows := m.filteredActions()
	if len(rows) != 1 || rows[0].id != "theme.select" {
		t.Fatal("theme command is not discoverable")
	}
	base := make([]string, m.height)
	for i := range base {
		base[i] = strings.Repeat(" ", m.width)
	}
	layout := m.pickerLayout(rows)
	painted := m.pickerOverlay(base)
	cells := uv.NewStyledString(painted[layout.y+3]).Lines(ansi.GraphemeWidth)[0]
	if !sameColor(cells[layout.x+2].Style.Fg, lipgloss.Color(m.colors.PopupSelectionForeground)) || !sameColor(cells[layout.x+2].Style.Bg, lipgloss.Color(m.colors.PopupSelection)) {
		t.Fatal("picker selection is not light-themed")
	}
	m.diffPath = "file.go"
	m.patch = diff.Parse("--- a/file.go\n+++ b/file.go\n@@ -1 +1 @@\n-var old = 1\n+var value = 2\n")
	m.renderDiff()
	oldSyntax := m.highlightDiff()()
	drain(t, m, m.setTestTheme("dark_modern", ""))
	m.Update(oldSyntax)
	for _, spans := range m.diffSyntax {
		for _, span := range spans {
			if span.Color == "#0000FF" {
				t.Fatal("stale light diff syntax replaced dark syntax")
			}
		}
	}
	drain(t, m, m.setTestTheme("light_modern", ""))
	for i := range m.patch.Lines {
		line := &m.patch.Lines[i]
		if line.Kind != diff.Added {
			continue
		}
		cells := uv.NewStyledString(m.diffRow(line, 50, true, diffRange{})).Lines(ansi.GraphemeWidth)[0]
		if !sameColor(cells[0].Style.Bg, lipgloss.Color("#FFFFFF")) {
			t.Fatal("light diff gutter is not white")
		}
		if !sameColor(cells[49].Style.Bg, lipgloss.Color(m.colors.AddedBackground)) {
			t.Fatal("light diff background missing")
		}
	}
}

func TestThemeFollowsOnlyWhenPreviouslyAligned(t *testing.T) {
	m := tabWorkspace(t, 0)
	m.Update(uv.LightColorSchemeEvent{})
	if m.colorTheme.IsLight() {
		t.Fatal("first report must only establish the appearance baseline")
	}
	m.setTestTheme("light_plus", "")
	m.Update(uv.DarkColorSchemeEvent{})
	if m.committedTheme().ID != "dark_modern" {
		t.Fatal("aligned theme did not follow dark appearance")
	}
	m.setTestTheme("dark_vs", "")
	m.Update(uv.LightColorSchemeEvent{})
	if m.committedTheme().ID != "light_plus" {
		t.Fatal("remembered light theme was lost")
	}
	m.Update(uv.DarkColorSchemeEvent{})
	if m.committedTheme().ID != "dark_vs" {
		t.Fatal("remembered dark theme was lost")
	}
	m.setTestTheme("light_vs", "") // deliberately differs from dark terminal
	m.Update(uv.DarkColorSchemeEvent{})
	m.Update(uv.LightColorSchemeEvent{})
	if m.committedTheme().ID != "light_vs" {
		t.Fatal("mismatched manual choice was replaced")
	}
	m.Update(uv.DarkColorSchemeEvent{}) // now aligned again, so follow
	if m.committedTheme().ID != "dark_vs" {
		t.Fatal("alignment did not resume implicit following")
	}
}

func TestThemePairAndAlignmentSurviveRestart(t *testing.T) {
	m := tabWorkspace(t, 0)
	m.Update(uv.DarkColorSchemeEvent{})
	m.setTestTheme("light_plus", "")
	m.setTestTheme("dark_plus", "")
	next := tabWorkspace(t, 0)
	next.SetLayout(m.Layout())
	next.Update(uv.LightColorSchemeEvent{})
	if next.committedTheme().ID != "dark_plus" {
		t.Fatal("first report replaced saved theme")
	}
	next.Update(uv.DarkColorSchemeEvent{})
	if next.committedTheme().ID != "dark_plus" {
		t.Fatal("mismatched saved theme was replaced")
	}
	next.Update(uv.LightColorSchemeEvent{})
	if next.committedTheme().ID != "light_plus" {
		t.Fatal("remembered light theme did not follow subsequent change")
	}
	next.Update(uv.DarkColorSchemeEvent{})
	if next.committedTheme().ID != "dark_plus" {
		t.Fatal("remembered dark theme was lost")
	}
}

func TestFirstTerminalReportDoesNotTreatSystemHandoffAsAppearanceChange(t *testing.T) {
	m := tabWorkspace(t, 0)
	m.setTestTheme("dark_plus", "")
	m.Update(systemAppearanceMsg{light: false, known: true})
	m.Update(uv.LightColorSchemeEvent{})
	if m.committedTheme().ID != "dark_plus" {
		t.Fatal("terminal's first report overrode saved theme after system fallback")
	}
	m.Update(uv.DarkColorSchemeEvent{})
	m.Update(uv.LightColorSchemeEvent{})
	if m.committedTheme().ID != "light_modern" {
		t.Fatal("subsequent aligned terminal changes did not follow")
	}
}

func TestFirstAppearancePreservesExplicitSelection(t *testing.T) {
	m := tabWorkspace(t, 0)
	m.setTestTheme("dark_plus", "")
	m.Update(uv.LightColorSchemeEvent{})
	if m.committedTheme().ID != "dark_plus" {
		t.Fatal("first report replaced manual theme")
	}
	m.Update(systemAppearanceMsg{light: false, known: true})
	if !m.appearance.light {
		t.Fatal("system fallback overrode terminal report")
	}
	m.width, m.height = 20, 5
	if !sameColor(m.View().BackgroundColor, lipgloss.Color(m.colors.Panel)) {
		t.Fatal("small view did not update terminal padding")
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

func TestSameBrightnessSwitchInvalidatesSyntax(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.openFile("file0.go")
	old := m.scheduleHighlight()().(highlightStartMsg).request
	text, cursor := m.document().Text, m.document().Cursor
	drain(t, m, m.setTestTheme("dark_vs", ""))
	if m.colorTheme.IsLight() || m.colors.Background != "#1E1E1E" || m.committedTheme().ID != "dark_vs" {
		t.Fatal("same-brightness theme was not installed")
	}
	if old.ctx.Err() == nil || m.startHighlight(old) != nil {
		t.Fatal("old syntax survived same-brightness switch")
	}
	if m.document().Text != text || m.document().Cursor != cursor {
		t.Fatal("theme changed editor state")
	}
	drain(t, m, m.setTestTheme("dark_plus", ""))
	if m.colorTheme != theme.Builtin("dark_plus") {
		t.Fatal("Dark+ not selected")
	}
	m.Update(uv.LightColorSchemeEvent{})
	if m.colorTheme.IsLight() {
		t.Fatal("terminal notification replaced explicit theme")
	}
}

func TestLocalThemeRestoresTypeAndPalette(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "day.json")
	if err := os.WriteFile(path, []byte(`{"colors":{"editor.background":"#F0E0D0","editor.foreground":"#123456","editor.selectionBackground":"#12345680","input.foreground":"#654321"},"tokenColors":[{"scope":"keyword", "settings":{"foreground":"#AABBCC","fontStyle":"bold"}}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	selected, err := theme.Load(path, "vs")
	if err != nil {
		t.Fatal(err)
	}
	m := tabWorkspace(t, 1)
	m.openFile("file0.go")
	drain(t, m, m.SetColorTheme(selected))
	if !m.colorTheme.IsLight() || m.colors.Background != "#F0E0D0" || m.colors.InputForeground != "#654321" {
		t.Fatal("local workbench colors not applied")
	}
	if len(m.tabs[0].spans) == 0 || m.tabs[0].spans[0][0].Color != "#AABBCC" || m.tabs[0].spans[0][0].Font != theme.Bold {
		t.Fatal("local syntax not applied")
	}
	next := tabWorkspace(t, 0)
	next.SetLayout(m.Layout())
	if !next.colorTheme.IsLight() || next.committedTheme().ID != path || next.colors != m.colors {
		t.Fatal("local theme or extension type lost on restore")
	}
	before := m.colorTheme
	m.setTestTheme(filepath.Join(dir, "missing.json"), "")
	if m.colorTheme != before || m.committedTheme().ID != path || !strings.Contains(m.notice, "Load theme") {
		t.Fatal("invalid local theme replaced working theme")
	}
}

func TestLocalThemeCatalogAppearsAndSelects(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "personal.json"), []byte(`{"name":"Personal","type":"light","colors":{"editor.background":"#FFEEDD"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	local, err := theme.Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := tabWorkspace(t, 0)
	m.openThemePicker()
	m.input.SetValue("Personal")
	m.applyLocalThemes(localThemesMsg{seq: m.localThemesSeq, themes: local})
	rows := m.filteredActions()
	if len(rows) != 1 || !strings.Contains(rows[0].label, "Light · Personal · Local") {
		t.Fatalf("local menu: %+v", rows)
	}
	drain(t, m, m.selectPickerChoice())
	if m.committedTheme().ID != "local:personal.json" || !m.colorTheme.IsLight() || m.colors.Background != "#FFEEDD" || m.Layout().Theme != "local:personal.json" {
		t.Fatal("local selection or stable preference lost")
	}
	m.applyLocalThemes(localThemesMsg{seq: m.localThemesSeq - 1})
	if len(m.localThemes) != 1 {
		t.Fatal("stale scan replaced current theme catalog")
	}
}

func TestColorThemeHasOneCommandAndSeparatePicker(t *testing.T) {
	m := tabWorkspace(t, 0)
	m.setTestTheme("dark_plus", "")
	original := m.colorTheme
	m.openPalette()
	m.input.SetValue(">Color Theme")
	rows := m.filteredActions()
	if len(rows) != 1 || rows[0].id != "theme.select" {
		t.Fatalf("theme entries leaked into commands: %+v", rows)
	}
	m.selectPickerChoice()
	if m.modal != "themes" || m.input.Value() != "" || m.lastCommand != "theme.select" {
		t.Fatal("theme command did not open its own searchable picker")
	}
	rows = m.filteredActions()
	if len(rows) != 6 || rows[m.pickerIndex].id != "dark_plus" || rows[m.pickerIndex].key != "Current" {
		t.Fatal("theme picker did not select the current theme")
	}
	m.input.SetValue("Light Modern")
	m.syncPicker()
	if len(m.filteredActions()) != 1 {
		t.Fatal("theme search did not filter choices")
	}
	if m.colorTheme != original {
		t.Fatal("search changed the theme before confirmation")
	}
	m.updateModal(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.modal != "" || m.colorTheme != original {
		t.Fatal("cancel changed the theme")
	}
	m.openThemePicker()
	m.input.SetValue("Light Modern")
	m.syncPicker()
	drain(t, m, m.selectPickerChoice())
	if m.modal != "" || !m.colorTheme.IsLight() || m.colorTheme.ID != "light_modern" || m.lastCommand != "theme.select" {
		t.Fatal("confirmed theme was not applied independently of command history")
	}
}

func TestThemeArrowPreviewCancelsAndConfirms(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.setTestTheme("dark_plus", "")
	m.openFile("file0.go")
	old := m.scheduleHighlight()().(highlightStartMsg).request
	original, preference := m.colorTheme, m.Layout()
	text, cursor := m.document().Text, m.document().Cursor
	m.openThemePicker()
	drain(t, m, m.updateModal(tea.KeyPressMsg{Code: tea.KeyDown}))
	if !m.colorTheme.IsLight() || m.colorTheme == original || m.modal != "themes" {
		t.Fatal("arrow did not preview the highlighted theme")
	}
	if m.Layout() != preference || m.committedTheme().ID != "dark_plus" {
		t.Fatal("preview changed the saved preference")
	}
	if old.ctx.Err() == nil || m.startHighlight(old) != nil {
		t.Fatal("preview accepted stale original syntax")
	}
	if m.document().Text != text || m.document().Cursor != cursor {
		t.Fatal("preview changed document state")
	}
	preview := m.colorTheme
	drain(t, m, m.updateModal(tea.KeyPressMsg{Code: tea.KeyEscape}))
	if m.colorTheme != original || m.Layout() != preference || m.themeBeforePreview != nil {
		t.Fatal("Escape did not restore the original theme")
	}
	m.openThemePicker()
	drain(t, m, m.updateModal(tea.KeyPressMsg{Code: tea.KeyDown}))
	drain(t, m, m.updateModal(tea.KeyPressMsg{Code: tea.KeyEnter}))
	if m.themeBeforePreview != nil || m.modal != "" || m.colorTheme != preview || m.committedTheme().ID != preview.ID || m.Layout().Theme != preview.ID {
		t.Fatal("Enter did not commit the preview")
	}
}

func TestThemePreviewKeepsSlotsAndTracksCommittedAppearance(t *testing.T) {
	m := tabWorkspace(t, 0)
	m.Update(uv.DarkColorSchemeEvent{})
	m.setTestTheme("light_plus", "")
	m.setTestTheme("dark_plus", "")
	m.openThemePicker()
	drain(t, m, m.updateModal(tea.KeyPressMsg{Code: tea.KeyHome}))
	preview := m.colorTheme
	m.Update(uv.LightColorSchemeEvent{})
	if m.colorTheme != preview || m.committedTheme().ID != "light_plus" || m.Layout().LightTheme != "light_plus" {
		t.Fatal("appearance change replaced preview or corrupted committed preference")
	}
	drain(t, m, m.openPalette())
	if m.colorTheme.ID != "light_plus" || m.themeBeforePreview != nil || !m.commandPicker() {
		t.Fatal("leaving picker did not restore latest committed appearance")
	}
	m.openThemePicker()
	drain(t, m, m.updateModal(tea.KeyPressMsg{Code: tea.KeyHome}))
	drain(t, m, m.updateModal(tea.KeyPressMsg{Code: tea.KeyEnter}))
	if m.Layout().DarkTheme != m.committedTheme().ID || m.Layout().LightTheme != "light_plus" {
		t.Fatal("Enter did not remember only the chosen appearance")
	}
}

func TestPickerHasContinuousFrame(t *testing.T) {
	m := tabWorkspace(t, 1)
	m.snapshot.Files = []string{"file0.go"}
	m.openQuickFiles()
	layout := m.pickerLayout(m.filteredActions())
	lines := strings.Split(m.View().Content, "\n")
	// Every result keeps its side edges, including the selected row, so the
	// terminal does not rely on a background shadow to distinguish the popup.
	for i := 0; i < layout.count; i++ {
		cells := uv.NewStyledString(lines[layout.y+3+i]).Lines(ansi.GraphemeWidth)[0]
		for _, x := range []int{layout.x, layout.x + layout.width - 1} {
			if cells[x].Content != "│" || !sameColor(cells[x].Style.Fg, lipgloss.Color(m.colors.PopupBorder)) || !sameColor(cells[x].Style.Bg, lipgloss.Color(m.colors.Popup)) {
				t.Fatal("picker edge vanished on result row")
			}
		}
	}
	bottom := uv.NewStyledString(lines[layout.y+3+max(1, layout.count)]).Lines(ansi.GraphemeWidth)[0]
	if bottom[layout.x].Content != "╰" || bottom[layout.x+layout.width-1].Content != "╯" {
		t.Fatal("picker has no bottom frame")
	}
}

func TestSystemAppearanceFallbackAndTerminalPriority(t *testing.T) {
	m := tabWorkspace(t, 0)
	m.Update(systemAppearanceMsg{light: true, known: true})
	if m.colorTheme.IsLight() || !m.appearance.light {
		t.Fatal("first system report must only establish the appearance baseline")
	}
	m.setTestTheme("light_vs", "")
	m.Update(systemAppearanceMsg{light: false, known: true})
	if m.colorTheme.IsLight() {
		t.Fatal("aligned theme did not follow system change")
	}
	m.Update(systemAppearanceMsg{light: true, known: false})
	if m.colorTheme.IsLight() || m.appearance.light {
		t.Fatal("unknown system appearance changed theme")
	}
	m.Update(uv.LightColorSchemeEvent{})
	if m.committedTheme().ID != "dark_modern" {
		t.Fatal("first terminal report replaced saved selection")
	}
	m.Update(uv.DarkColorSchemeEvent{})
	m.Update(uv.LightColorSchemeEvent{})
	if m.committedTheme().ID != "light_vs" {
		t.Fatal("terminal appearance did not use saved counterpart")
	}
	m.Update(systemAppearanceMsg{light: false, known: true})
	if !m.colorTheme.IsLight() || !m.appearance.light {
		t.Fatal("system overrode terminal appearance")
	}
	if m.pollSystemAppearance(true) != nil {
		t.Fatal("system polling continued after terminal report")
	}
}

func TestRememberedLocalThemeOverrideAndMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "day.json")
	if err := os.WriteFile(path, []byte(`{"colors":{"editor.background":"#F0E0D0"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	selected, err := theme.Load(path, "vs")
	if err != nil {
		t.Fatal(err)
	}
	m := tabWorkspace(t, 0)
	m.Update(uv.DarkColorSchemeEvent{})
	m.SetColorTheme(selected)
	m.setTestTheme("dark_plus", "")
	next := tabWorkspace(t, 0)
	next.SetLayout(m.Layout())
	next.Update(uv.DarkColorSchemeEvent{})
	next.Update(uv.LightColorSchemeEvent{})
	if !next.colorTheme.IsLight() || next.colorTheme.UIThemeOverride != "vs" || next.colors.Background != "#F0E0D0" {
		t.Fatal("remembered local override not restored")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	missing := tabWorkspace(t, 0)
	missing.SetLayout(m.Layout())
	missing.Update(uv.DarkColorSchemeEvent{})
	missing.Update(uv.LightColorSchemeEvent{})
	if missing.committedTheme().ID != "light_modern" || !strings.Contains(missing.notice, "Load remembered theme") {
		t.Fatal("missing counterpart did not fall back with notice")
	}
}

func TestUnsupportedTerminalSavesConfirmedThemeBeforeExit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "layout.json")
	m := tabWorkspace(t, 0)
	m.SetLayoutStore(config.NewLayoutStore(path))
	m.Update(tea.ModeReportMsg{Mode: ansi.ModeLightDark, Value: ansi.ModeNotRecognized})
	m.openThemePicker()
	m.input.SetValue("Light+")
	m.syncPicker()
	drain(t, m, m.previewSelectedTheme())
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("preview wrote preferences")
	}
	drain(t, m, m.selectPickerChoice())
	saved, err := config.LoadLayout(path)
	if err != nil || saved.Theme != "light_plus" || saved.LightTheme != "light_plus" {
		t.Fatalf("confirmation not saved before exit: %+v, %v", saved, err)
	}
	// No quit, terminal appearance event, or exit-time save is involved.
	next := tabWorkspace(t, 0)
	next.SetLayout(saved)
	next.Update(tea.ModeReportMsg{Mode: ansi.ModeLightDark, Value: ansi.ModePermanentlyReset})
	if next.committedTheme().ID != "light_plus" {
		t.Fatal("unsupported terminal lost saved choice")
	}
	next.SetLayoutStore(config.NewLayoutStore(path))
	drain(t, next, next.setTestTheme("dark_plus", ""))
	saved, err = config.LoadLayout(path)
	if err != nil || saved.DarkTheme != "dark_plus" || saved.LightTheme != "light_plus" {
		t.Fatalf("remembered pair not saved: %+v %v", saved, err)
	}
}

func TestThemeSaveFailureRemainsVisible(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(parent, []byte("occupied"), 0600); err != nil {
		t.Fatal(err)
	}
	m := tabWorkspace(t, 0)
	m.SetLayoutStore(config.NewLayoutStore(filepath.Join(parent, "layout.json")))
	drain(t, m, m.setTestTheme("light_plus", ""))
	if m.committedTheme().ID != "light_plus" || !strings.Contains(m.notice, "Save preferences:") {
		t.Fatal("save failure was silent or discarded selection")
	}
}
