package theme

import (
	"testing"
)

func TestRegisteredDefaultsResolveAgainstThemeOverrides(t *testing.T) {
	dir := t.TempDir()
	filename := writeTheme(t, dir, "references.json", `{"colors":{"foreground":"#102030","tab.activeForeground":"#203040"}}`)
	th := mustLoad(t, filename)
	for id, want := range map[string]string{
		"foreground": "#102030", "editorWidget.foreground": "#102030", "quickInput.foreground": "#102030", "descriptionForeground": "#102030B3", "input.placeholderForeground": "#10203080", "tab.inactiveForeground": "#20304080", "list.inactiveSelectionForeground": "", "list.hoverForeground": "", "widget.border": "",
	} {
		if got := th.Color(id); got != want {
			t.Errorf("%s: %s, want %s", id, got, want)
		}
	}
	if th.Color("unknown.id") != "" {
		t.Fatal("unknown colors invented a fallback")
	}
}

func TestRegisteredVariantsAndAllReferences(t *testing.T) {
	for _, kind := range []string{"vs-dark", "vs", "hc-black", "hc-light"} {
		th := &Theme{Type: kind, colors: map[string]string{}}
		for id := range registeredColors {
			th.Color(id)
		}
		if kind == "hc-black" && (th.Color("editor.background") != "#000000" || th.Color("diffEditor.insertedLineBackground") != "" || th.Color("list.activeSelectionForeground") != "") {
			t.Fatal("high contrast nulls were replaced with dark colors")
		}
	}
}

func TestRGBAIsRetainedUntilActualSurface(t *testing.T) {
	dir := t.TempDir()
	filename := writeTheme(t, dir, "alpha.json", `{"colors":{"foreground":"#11223380"},"tokenColors":[{"scope":"string","settings":{"foreground":"#FFFFFF80","background":"#FF000080"}}]}`)
	th := mustLoad(t, filename)
	if th.Color("input.placeholderForeground") != "#11223340" {
		t.Fatal("computed opacity discarded the original alpha")
	}
	token := th.Resolve([]string{"source.go", "string.quoted"})
	if token.Color != "#FFFFFF80" || token.Background != "#FF000080" {
		t.Fatal("token colors were precomposed before the editor/diff surface was known")
	}
	if Composite(token.Color, "#000000") != "#808080" || Composite(token.Color, "#FFFFFF") != "#FFFFFF" {
		t.Fatal("foreground alpha did not follow the actual surface")
	}
}

func TestIncludeDoesNotSetThemeType(t *testing.T) {
	other := t.TempDir()
	writeTheme(t, other, "base.json", `{"type":"light","colors":{}}`)
	root := writeTheme(t, other, "root.json", `{"include":"base.json"}`)
	if mustLoad(t, root).IsLight() {
		t.Fatal("included JSON type overrode the root theme's default category")
	}
}

func TestEditorDecorationPalette(t *testing.T) {
	for _, id := range []string{"dark_plus", "light_plus"} {
		th := Builtin(id)
		p := th.Palette()
		if p.CurrentLine == p.Background && p.CurrentLineBorder == "" {
			t.Fatalf("%s current line is invisible", id)
		}
	}
}

func TestReadableContrastUsesTheFinalSurface(t *testing.T) {
	for _, pair := range [][2]string{{"#F8F8F8", "#F3F3F3"}, {"#FFFFFF66", "#007ACC"}, {"#0F4A85", "#0F4A85"}, {"#000000", "#FFFFFF"}} {
		fg, bg := pair[0], pair[1]
		got := Readable(fg, bg)
		if contrast(got, bg) < 4.5 {
			t.Errorf("%s on %s resolves below the contrast floor: %s", fg, bg, got)
		}
		if original := Composite(fg, bg); contrast(original, bg) >= 4.5 && original != got {
			t.Errorf("already readable color changed from %s to %s", original, got)
		}
	}
}
