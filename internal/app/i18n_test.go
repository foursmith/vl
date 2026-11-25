package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/foursmith/vl/internal/config"
	"github.com/foursmith/vl/internal/editor"
	gitservice "github.com/foursmith/vl/internal/git"
	"github.com/foursmith/vl/internal/i18n"
)

func TestDisplayLanguagePickerPersistsAndRestores(t *testing.T) {
	m := tabWorkspace(t, 0)
	defer m.Close()
	path := filepath.Join(t.TempDir(), "layout.json")
	m.SetLayoutStore(config.NewLayoutStore(path))
	m.openPalette()
	m.input.SetValue(">language")
	m.syncQuickPicker()
	rows := m.filteredActions()
	if len(rows) != 1 || rows[0].id != "language.select" {
		t.Fatalf("language command not discoverable: %#v", rows)
	}
	m.selectPickerChoice()
	if m.modal != "languages" || !m.isPicker() {
		t.Fatal("language picker not opened")
	}
	m.input.SetValue("简体中文")
	m.syncPicker()
	drain(t, m, m.selectPickerChoice())
	if m.displayLanguage != i18n.Chinese || m.modal != "" {
		t.Fatal("language not applied immediately")
	}
	saved, err := config.LoadLayout(path)
	if err != nil || saved.Language != "zh-CN" {
		t.Fatalf("not persisted: %#v, %v", saved, err)
	}
	restored := New(context.Background(), &gitservice.Repository{Root: t.TempDir()}, false)
	defer restored.Close()
	restored.SetLayout(saved)
	if restored.tr("Changes") != "更改" {
		t.Fatal("saved language not restored in another workspace")
	}
	restored.SetLayout(config.Layout{Language: "invalid"})
	if restored.tr("Changes") != "Changes" {
		t.Fatal("invalid saved language did not fall back")
	}
	// Both language choices stay recognizable regardless of the active language.
	m.openLanguagePicker()
	if m.palette[0].label != "English" || m.palette[1].label != "简体中文" {
		t.Fatal("language names were localized")
	}
	m.input.SetValue("English")
	drain(t, m, m.selectPickerChoice())
	if m.displayLanguage != i18n.English {
		t.Fatal("cannot switch back to English")
	}
}

func TestDisplayLanguagePreservesEditingAndFitsBothLanguages(t *testing.T) {
	m := tabWorkspace(t, 0)
	defer m.Close()
	doc := &editor.Document{Buffer: editor.Buffer{Text: "Changes\n更改\n", Anchor: -1}, Path: filepath.Join(m.repo.Root, "Changes.go")}
	doc.Insert("unsaved")
	m.tabs = []tab{{doc: doc}}
	m.activeTab = 0
	text, cursor := doc.Text, doc.Cursor
	for _, language := range []string{"zh-CN", "en"} {
		m.SetDisplayLanguage(language)
		if doc.Text != text || doc.Cursor != cursor || !doc.Dirty() || m.document() != doc {
			t.Fatal("switching language changed editor state")
		}
		for _, size := range [][2]int{{120, 36}, {60, 20}, {30, 8}} {
			m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			m.closeTab()
			if !strings.Contains(m.modalTitle, "Changes.go") {
				t.Fatal("filename was translated")
			}
			for _, line := range strings.Split(m.View().Content, "\n") {
				if ansi.StringWidth(line) != size[0] {
					t.Fatalf("%s dialog escaped %v", language, size)
				}
			}
			if m.confirmationActions[2].label != m.tr("Cancel") {
				t.Fatal("dialog language did not change")
			}
			press(m, "esc")
		}
	}
}

func TestChineseCommandsKeepStableOperationIDs(t *testing.T) {
	m := tabWorkspace(t, 0)
	defer m.Close()
	english := m.actions()
	m.SetDisplayLanguage("zh-CN")
	chinese := m.actions()
	if len(chinese) != len(english) {
		t.Fatal("translation changed command availability")
	}
	for i, action := range chinese {
		if action.id != english[i].id {
			t.Fatal("translation changed command identity")
		}
		if action.label == english[i].label {
			t.Errorf("command not translated: %s: %s", action.id, action.label)
		}
	}
	m.section = changes
	m.snapshot.NoRepository = true
	if !strings.Contains(plain(strings.Join(m.sidebarLines(), "\n")), "初始化仓库") {
		t.Fatal("folder placeholder not translated")
	}
}

func TestLanguageSwitchDuringScopedGitOperation(t *testing.T) {
	m := explorerWorkspace(t)
	defer m.Close()
	m.SetDisplayLanguage("zh-CN")
	drain(t, m, m.selectSection(changes))
	cmd := clickScopeAction(t, m, "cmd", false, false, "stage")
	if cmd == nil {
		t.Fatal("stage command unavailable")
	}
	result := make(chan tea.Msg, 1)
	go func() { result <- cmd() }()
	m.SetDisplayLanguage("en")
	msg := <-result
	operation, ok := msg.(operationMsg)
	if !ok || operation.title != "Stage" {
		t.Fatalf("localized operation identity: %#v", msg)
	}
	_, next := m.Update(msg)
	drain(t, m, next)
	found := false
	for _, change := range m.snapshot.Changes {
		if change.Path == "cmd/main.go" {
			found = change.Staged()
		}
	}
	if !found {
		t.Fatal("language switch interrupted staging")
	}
}
