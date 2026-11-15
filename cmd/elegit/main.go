package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "charm.land/bubbletea/v2"
	"github.com/ourongxing/elegit/internal/app"
	"github.com/ourongxing/elegit/internal/config"
	"github.com/ourongxing/elegit/internal/git"
)

func main() {
	icons := flag.Bool("icons", true, "use Nerd Font glyphs adapted from Material Icon Theme; set -icons=false for plain text")
	version := flag.Bool("version", false, "print version")
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "Usage: elegit [options] [repository]\n\nA Git workbench with diff review and basic text editing.")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *version {
		fmt.Println("elegit 0.1.0-dev")
		return
	}
	path := "."
	if flag.NArg() > 1 {
		flag.Usage()
		os.Exit(2)
	}
	if flag.NArg() == 1 {
		path = flag.Arg(0)
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
		m.SetSidebarLayout(layout.SidebarWidth, layout.SidebarRight)
	}
	for {
		if recentErr := config.RememberFolder(repo.Root); recentErr != nil {
			fmt.Fprintln(os.Stderr, "Save recent folders:", recentErr)
		}
		_, err = tea.NewProgram(m, tea.WithContext(ctx)).Run()
		m.Close()
		next := m.NextRepository()
		if err != nil || next == nil {
			break
		}
		width, right := m.SidebarLayout()
		repo = next
		m = app.New(ctx, repo, *icons)
		m.SetSidebarLayout(width, right)
	}
	if layoutErr != nil {
		fmt.Fprintln(os.Stderr, "Load layout preferences:", layoutErr)
	} else {
		width, right := m.SidebarLayout()
		updated := config.Layout{SidebarWidth: width, SidebarRight: right}
		if updated != layout {
			if saveErr := config.SaveLayout(layoutPath, updated); saveErr != nil {
				fmt.Fprintln(os.Stderr, "Save layout preferences:", saveErr)
			}
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
