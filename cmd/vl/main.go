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
	"github.com/ourongxing/vl/internal/app"
	"github.com/ourongxing/vl/internal/config"
	"github.com/ourongxing/vl/internal/editor"
	"github.com/ourongxing/vl/internal/git"
	"github.com/ourongxing/vl/internal/i18n"
	"github.com/ourongxing/vl/internal/theme"
)

func main() {
	displayLanguage := flag.String("lang", "", "display language: en or zh-CN (default: saved preference, or en)")
	icons := flag.Bool("icons", true, "use Nerd Font glyphs adapted from Material Icon Theme; set -icons=false for plain text")
	colorTheme := flag.String("theme", "", "color theme: builtin ID (dark_modern, dark_plus, dark_vs, light_modern, light_plus, light_vs), local:filename from ~/.config/vl/themes, or a theme file")
	themeType := flag.String("theme-type", "", "local theme uiTheme from its extension: vs, vs-dark, hc-light, hc-black (default: theme type, or vs-dark)")
	version := flag.Bool("version", false, "print version")
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "Usage: vl [options] [folder|file[:line[:column]]]\n\nA VSCode-like Git diff viewer with basic text editing.\nA file opens in editor-only mode. Line and column numbers start at 1.\nWithout a path, opens the most recent workspace.")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *version {
		fmt.Println("vl 0.1.0-dev")
		return
	}
	language, validLanguage := i18n.Parse(*displayLanguage)
	if !validLanguage {
		fmt.Fprintln(os.Stderr, "Unsupported display language; use -lang en or -lang zh-CN")
		os.Exit(2)
	}
	var selectedTheme *theme.Theme
	if *themeType != "" && *colorTheme == "" {
		fmt.Fprintln(os.Stderr, "-theme-type requires a theme file")
		os.Exit(2)
	}
	if *colorTheme != "" {
		var err error
		selectedTheme, err = theme.Load(*colorTheme, *themeType)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Load theme:", err)
			os.Exit(2)
		}
	}
	if flag.NArg() > 1 {
		flag.Usage()
		os.Exit(2)
	}
	path, err := startupPath(flag.Args())
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
		err = runProgram(ctx, m)
		if saveErr := m.SaveFilePositions(); saveErr != nil {
			fmt.Fprintln(os.Stderr, "Save file positions:", saveErr)
		}
		m.Close()
	} else {
		for {
			sessionPath, sessionErr := config.SessionPath(repo.Root)
			if sessionErr == nil {
				var session config.Session
				session, sessionErr = config.LoadSession(sessionPath)
				if sessionErr == nil {
					m.SetSession(session)
				}
			}
			if sessionErr != nil {
				fmt.Fprintln(os.Stderr, "Load workspace tabs:", sessionErr)
			}
			if recentErr := config.RememberFolder(repo.Root); recentErr != nil {
				fmt.Fprintln(os.Stderr, "Save recent folders:", recentErr)
			}
			err = runProgram(ctx, m)
			if saveErr := m.SaveFilePositions(); saveErr != nil {
				fmt.Fprintln(os.Stderr, "Save file positions:", saveErr)
			}
			if sessionErr == nil {
				if saveErr := config.SaveSession(sessionPath, m.Session()); saveErr != nil {
					fmt.Fprintln(os.Stderr, "Save workspace tabs:", saveErr)
				}
			}
			m.Close()
			next := m.NextRepository()
			if err != nil || next == nil {
				break
			}
			preferences := m.Layout()
			repo = next
			m = app.New(ctx, repo, *icons)
			m.SetFilePositionStore(positionStore)
			m.SetLayout(preferences)
			m.SetLayoutStore(layoutStore)
		}
	}
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

func runProgram(ctx context.Context, m *app.Model) error {
	defer func() { fmt.Fprint(os.Stdout, m.ImageCleanupSequence()+m.ThemeNotificationRestoreSequence()) }()
	options := []tea.ProgramOption{tea.WithContext(ctx)}
	if os.Getenv("NO_COLOR") != "" {
		options = append(options, tea.WithColorProfile(colorprofile.ASCII))
	}
	_, err := tea.NewProgram(m, options...).Run()
	return err
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
