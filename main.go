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
	flag.BoolVar(&staged, "staged", false, "start focused on staged changes")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.Parse()

	if showVersion {
		fmt.Printf("dv %s (%s)\n", version, commit)
		os.Exit(0)
	}

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	provider := GitDiffProvider{WorkDir: cwd}
	app := NewDiffApp(provider, staged)
	if err := t.Run(app); err != nil {
		log.Fatal(err)
	}
}
