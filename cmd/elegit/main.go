package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/ourongxing/elegit/internal/app"
	"github.com/ourongxing/elegit/internal/config"
	"github.com/ourongxing/elegit/internal/git"
	"github.com/ourongxing/elegit/internal/theme"
)

func main() {
	icons := flag.Bool("icons", true, "use Nerd Font glyphs adapted from Material Icon Theme; set -icons=false for plain text")
	colorTheme := flag.String("theme", "", "color theme: builtin ID (dark_modern, dark_plus, dark_vs, light_modern, light_plus, light_vs), local:filename from ~/.config/elegit/themes, or a theme file")
	themeType := flag.String("theme-type", "", "local theme uiTheme from its extension: vs, vs-dark, hc-light, hc-black (default: theme type, or vs-dark)")
	version := flag.Bool("version", false, "print version")
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "Usage: elegit [options] [folder]\n\nA Git workbench with diff review and basic text editing.\nWithout a folder, opens the most recent workspace.")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *version {
		fmt.Println("elegit 0.1.0-dev")
		return
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
	repo, err := git.Open(ctx, path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	m := app.New(ctx, repo, *icons)
	layoutPath, layoutErr := config.LayoutPath()
	var layout config.Layout
	if layoutErr == nil {
		layout, layoutErr = config.LoadLayout(layoutPath)
	}
	if layoutErr == nil {
		m.SetLayout(layout)
	}
	if selectedTheme != nil {
		m.SetColorTheme(selectedTheme)
	}

	var layoutStore *config.LayoutStore
	if layoutErr == nil {
		layoutStore = config.NewLayoutStore(layoutPath)
		m.SetLayoutStore(layoutStore)
	}
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
		_, err = runWorkbench(ctx, m)
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
		m.SetLayout(preferences)
		m.SetLayoutStore(layoutStore)
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
			return "", errors.New("no recent workspace; open a folder with: elegit /path/to/folder")
		}
		path = folders[0]
	}
	if path == "" {
		return "", errors.New("workspace path must not be empty")
	}
	return path, nil
}

func runWorkbench(ctx context.Context, m *app.Model) (tea.Model, error) {
	defer func() { fmt.Fprint(os.Stdout, m.ImageCleanupSequence()+m.ThemeNotificationRestoreSequence()) }()
	options := []tea.ProgramOption{tea.WithContext(ctx)}
	if os.Getenv("NO_COLOR") != "" {
		options = append(options, tea.WithColorProfile(colorprofile.ASCII))
	}
	return tea.NewProgram(m, options...).Run()
}
