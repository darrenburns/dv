package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	t "github.com/darrenburns/terma"
)

// Set at build time by GoReleaser via ldflags.
var (
	version = "dev"
	commit  = "none"
)

func main() {
	var staged bool
	var showVersion bool
	var viewMode string
	var sidebarVisible bool
	var themeName string
	var intralineStyle string
	var showSymbols bool

	flag.BoolVar(&staged, "staged", false, "start focused on staged changes")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.StringVar(&viewMode, "view", "unified", "default view mode: unified or split")
	flag.BoolVar(&sidebarVisible, "sidebar", true, "show sidebar on startup")
	flag.StringVar(&themeName, "theme", t.ThemeNameCatppuccin, "default theme")
	flag.StringVar(&intralineStyle, "intraline-style", "background", "default intraline style: background or underline")
	flag.BoolVar(&showSymbols, "show-symbols", false, "show +/- symbols by default")
	flag.Parse()

	if showVersion {
		fmt.Printf("dv %s (%s)\n", version, commit)
		os.Exit(0)
	}

	initialState, err := startupInitialStateFromFlags(viewMode, sidebarVisible, themeName, intralineStyle, showSymbols)
	if err != nil {
		log.Fatal(err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	provider := GitDiffProvider{WorkDir: cwd}
	app := NewDv(provider, staged, initialState)
	if err := t.Run(app); err != nil {
		log.Fatal(err)
	}
}
