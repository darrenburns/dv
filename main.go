package main

import (
	"flag"
	"log"
	"os"

	t "github.com/darrenburns/terma"
)

func main() {
	var staged bool
	flag.BoolVar(&staged, "staged", false, "start focused on staged changes")
	flag.Parse()

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
