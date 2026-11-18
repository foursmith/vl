package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/foursmith/vl/internal/app"
	"github.com/foursmith/vl/internal/config"
	"github.com/foursmith/vl/internal/editor"
	"github.com/foursmith/vl/internal/git"
	"github.com/foursmith/vl/internal/i18n"
	"github.com/foursmith/vl/internal/theme"
)

var buildVersion = "0.1.0-dev"

const usage = `A VSCode-like terminal workbench with Vim-like keybindings.

Usage:
  vl [options] [folder|file[:line[:column]]]
  vl preview [options] file

Open a folder in the workspace viewer or a file in the editor.
Line and column numbers start at 1. With no path, reopen the most recent workspace.

Options:
  -h, --help        Show this help
  -v, --version     Print the version
  --lang LANGUAGE   Use en or zh-CN (default: saved preference, or en)
  --icons=false     Use plain-text icons instead of Nerd Font glyphs
  --theme ID        Use a bundled or imported theme (e.g. dark_modern)

Run 'vl preview --help' for preview options.
`

func main() {
	if len(os.Args) > 1 && os.Args[1] == "preview" {
		os.Exit(runPreview(os.Args[2:], os.Stdout, os.Stderr))
	}
	flags := flag.NewFlagSet("vl", flag.ContinueOnError)
	displayLanguage := flags.String("lang", "", "")
	icons := flags.Bool("icons", true, "")
	colorTheme := flags.String("theme", "", "")
	var version bool
	flags.BoolVar(&version, "version", false, "")
	flags.BoolVar(&version, "v", false, "")
	flags.Usage = func() {
		fmt.Fprint(flags.Output(), usage)
	}
	if err := parseFlags(flags, os.Args[1:]); err != nil {
		if err == flag.ErrHelp {
			return
		}
		os.Exit(2)
	}
	if version {
		fmt.Println("vl", buildVersion)
		return
	}
	language, validLanguage := i18n.Parse(*displayLanguage)
	if !validLanguage {
		fmt.Fprintln(os.Stderr, "Unsupported display language. Use --lang en or --lang zh-CN.")
		os.Exit(2)
	}
	var selectedTheme *theme.Theme
	if *colorTheme != "" {
		var err error
		selectedTheme, err = theme.Load(*colorTheme)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Load theme:", err)
			os.Exit(2)
		}
	}
	if flags.NArg() > 1 {
		flags.Usage()
		os.Exit(2)
	}
	path, err := startupPath(flags.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer cancel()
	m, repo, editorOnly, err := startupModel(ctx, path, *icons)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	configureMemory(editorOnly)
	layoutPath, layoutErr := config.LayoutPath()
	var layout config.Layout
	if layoutErr == nil {
		layout, layoutErr = config.LoadLayout(layoutPath)
	}
	if layoutErr == nil {
		m.SetLayout(layout)
	}
	if *displayLanguage != "" {
		m.SetDisplayLanguage(string(language))
	}
	if selectedTheme != nil {
		m.SetColorTheme(selectedTheme)
	}

	var layoutStore *config.LayoutStore
	if layoutErr == nil {
		layoutStore = config.NewLayoutStore(layoutPath)
		m.SetLayoutStore(layoutStore)
	}
	positionStore, positionErr := config.NewFilePositionStore()
	if positionErr != nil {
		fmt.Fprintln(os.Stderr, "Load file positions:", positionErr)
	}
	m.SetFilePositionStore(positionStore)
	if editorOnly {
		if _, line, column, locationErr := startupLocation(path); locationErr == nil && line > 0 {
			m.SetEditorPosition(line, column)
		}
		m, err = runProgram(ctx, m, nil)
		if saveErr := m.SaveFilePositions(); saveErr != nil {
			fmt.Fprintln(os.Stderr, "Save file positions:", saveErr)
		}
	} else {
		sessionPath := loadWorkspace(m, repo)
		m, err = runProgram(ctx, m, func(old *app.Model, next *git.Repository) *app.Model {
			saveWorkspace(old, sessionPath)
			updated := app.New(ctx, next, *icons)
			updated.SetFilePositionStore(positionStore)
			updated.SetLayout(old.Layout())
			updated.SetLayoutStore(layoutStore)
			sessionPath = loadWorkspace(updated, next)
			return updated
		})
		saveWorkspace(m, sessionPath)
	}
	m.Close()

	if layoutErr != nil {
		fmt.Fprintln(os.Stderr, "Load layout preferences:", layoutErr)
	} else {
		updated := m.Layout()
		if updated != layout {
			if saveErr := layoutStore.Prepare(updated)(); saveErr != nil {
				fmt.Fprintln(os.Stderr, "Save layout preferences:", saveErr)
			}
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// The caller validates that there is at most one positional argument.
func startupPath(args []string) (string, error) {
	var path string
	if len(args) == 1 {
		path = args[0]
	} else {
		folders, err := config.RecentFolders()
		if err != nil {
			return "", fmt.Errorf("read recent workspaces: %w", err)
		}
		if len(folders) == 0 {
			return "", errors.New("no recent workspace; open a folder with: vl /path/to/folder")
		}
		path = folders[0]
	}
	if path == "" {
		return "", errors.New("workspace path must not be empty")
	}
	return path, nil
}

// Return a writable session path only after loading succeeds, so a failed load
// cannot replace an unreadable session with the new model's empty tabs.
func loadWorkspace(m *app.Model, repo *git.Repository) string {
	path, err := config.SessionPath(repo.Root)
	if err == nil {
		var session config.Session
		session, err = config.LoadSession(path)
		if err == nil {
			m.SetSession(session)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Load workspace tabs:", err)
	}
	if recentErr := config.RememberFolder(repo.Root); recentErr != nil {
		fmt.Fprintln(os.Stderr, "Save recent folders:", recentErr)
	}
	if err != nil {
		return ""
	}
	return path
}

func saveWorkspace(m *app.Model, sessionPath string) {
	if err := m.SaveFilePositions(); err != nil {
		fmt.Fprintln(os.Stderr, "Save file positions:", err)
	}
	if sessionPath != "" {
		if err := config.SaveSession(sessionPath, m.Session()); err != nil {
			fmt.Fprintln(os.Stderr, "Save workspace tabs:", err)
		}
	}
}

func runProgram(ctx context.Context, m *app.Model, switchWorkspace func(*app.Model, *git.Repository) *app.Model) (*app.Model, error) {
	runtime := app.NewRuntime(m, switchWorkspace)
	defer func() {
		current := runtime.Current()
		fmt.Fprint(os.Stdout, current.ImageCleanupSequence()+current.ThemeNotificationRestoreSequence())
	}()
	options := []tea.ProgramOption{tea.WithContext(ctx), tea.WithFilter(runtime.Filter)}
	if os.Getenv("NO_COLOR") != "" {
		options = append(options, tea.WithColorProfile(colorprofile.ASCII))
	}
	_, err := tea.NewProgram(runtime, options...).Run()
	return runtime.Current(), err
}

// A direct file invocation is independent of Git discovery and workspace sessions.
func startupModel(ctx context.Context, path string, icons bool) (*app.Model, *git.Repository, bool, error) {
	path, line, column, err := startupLocation(path)
	if err != nil {
		return nil, nil, false, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, false, err
	}
	if !info.IsDir() {
		m, err := app.NewEditor(ctx, path, icons)
		if err == nil && line > 0 {
			m.SetEditorPosition(line, column)
		}
		return m, nil, true, err
	}
	repo, err := git.Open(ctx, path)
	if err != nil {
		return nil, nil, false, err
	}
	return app.New(ctx, repo, icons), repo, false, nil
}

// Existing paths take precedence over location suffixes, including colon names.
func startupLocation(value string) (path string, line, column int, err error) {
	if _, err := os.Stat(value); err == nil || !os.IsNotExist(err) {
		return value, 0, 0, nil
	}
	for end := len(value); end > 0; {
		colon := strings.LastIndex(value[:end], ":")
		if colon < 0 {
			break
		}
		end = colon
		info, statErr := os.Stat(value[:colon])
		if statErr != nil {
			continue
		}
		if info.IsDir() {
			return "", 0, 0, fmt.Errorf("location requires a file: %s", value)
		}
		line, column, err := editor.ParsePosition(value[colon+1:])
		if err != nil {
			return "", 0, 0, fmt.Errorf("invalid file location %q: %w", value, err)
		}
		return value[:colon], line, column, nil
	}
	return value, 0, 0, nil
}

// File editing favors a smaller heap over the workbench's allocation throughput.
// An explicit GOGC remains authoritative, including GOGC=off.
func configureMemory(editorOnly bool) {
	if _, configured := os.LookupEnv("GOGC"); editorOnly && !configured {
		debug.SetGCPercent(50)
	}
}
