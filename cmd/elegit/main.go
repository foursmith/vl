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
	if _, err = tea.NewProgram(app.New(ctx, repo, *icons), tea.WithContext(ctx)).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
