package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/adrg/xdg"
	uv "github.com/charmbracelet/ultraviolet"
	t "github.com/darrenburns/terma"
)

// Set at build time by GoReleaser via ldflags.
var (
	version = "dev"
	commit  = "none"
)

type ttyOpener func() (inTTY *os.File, outTTY *os.File, err error)

type stdinSetter func(file *os.File)

func main() {
	var staged bool
	var showVersion bool
	var viewMode string
	var sidebarVisible bool
	var themeName string
	var intralineStyle string
	var showSymbols bool
	var configPath string
	var noConfig bool

	flag.BoolVar(&staged, "staged", false, "start focused on staged changes")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.StringVar(&viewMode, "view", "unified", "default view mode: unified or split")
	flag.BoolVar(&sidebarVisible, "sidebar", true, "show sidebar on startup")
	flag.StringVar(&themeName, "theme", t.ThemeNameCatppuccin, "default theme")
	flag.StringVar(&intralineStyle, "intraline-style", "background", "default intraline style: background or underline")
	flag.BoolVar(&showSymbols, "show-symbols", false, "show +/- symbols by default")
	flag.StringVar(&configPath, "config", "", "path to YAML config file")
	flag.BoolVar(&noConfig, "no-config", false, "disable config file loading")
	flag.Parse()

	explicitlySetFlags := map[string]bool{}
	flag.Visit(func(f *flag.Flag) {
		explicitlySetFlags[f.Name] = true
	})

	if showVersion {
		fmt.Printf("dv %s (%s)\n", version, commit)
		os.Exit(0)
	}

	cfg, err := loadStartupConfig(xdg.ConfigHome, configPath, noConfig)
	if err != nil {
		log.Fatal(err)
	}

	flagValues := startupFlagValues{
		ViewMode:       viewMode,
		SidebarVisible: sidebarVisible,
		ThemeName:      themeName,
		IntralineStyle: intralineStyle,
		ShowSymbols:    showSymbols,
	}
	flagValues = applyStartupConfig(flagValues, cfg, explicitlySetFlags)

	initialState, err := startupInitialStateFromFlags(flagValues.ViewMode, flagValues.SidebarVisible, flagValues.ThemeName, flagValues.IntralineStyle, flagValues.ShowSymbols)
	if err != nil {
		log.Fatal(err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	stdinPiped, err := stdinIsPiped(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}
	provider, closeTTY, err := startupDiffProvider(
		cwd,
		os.Stdin,
		stdinPiped,
		uv.OpenTTY,
		func(file *os.File) { os.Stdin = file },
	)
	if err != nil {
		log.Fatal(err)
	}
	defer closeTTY()

	app := NewDv(provider, staged, initialState)
	if err := t.Run(app); err != nil {
		log.Fatal(err)
	}
}

func stdinIsPiped(stdin *os.File) (bool, error) {
	if stdin == nil {
		return false, fmt.Errorf("stdin is unavailable")
	}
	info, err := stdin.Stat()
	if err != nil {
		return false, fmt.Errorf("stat stdin: %w", err)
	}
	return info.Mode()&os.ModeCharDevice == 0, nil
}

func startupDiffProvider(workDir string, stdin io.Reader, piped bool, openTTY ttyOpener, setStdin stdinSetter) (DiffProvider, func(), error) {
	if openTTY == nil {
		openTTY = uv.OpenTTY
	}
	if setStdin == nil {
		setStdin = func(file *os.File) {
			os.Stdin = file
		}
	}

	if !piped {
		return GitDiffProvider{WorkDir: workDir}, func() {}, nil
	}

	rawDiff, err := io.ReadAll(stdin)
	if err != nil {
		return nil, func() {}, fmt.Errorf("read piped diff from stdin: %w", err)
	}

	inTTY, outTTY, err := openTTY()
	if err != nil {
		return nil, func() {}, fmt.Errorf("reopen terminal input after reading piped stdin: %w", err)
	}
	setStdin(inTTY)

	return StdinDiffProvider{
		WorkDir: workDir,
		Diff:    string(rawDiff),
	}, func() { closeTTYPair(inTTY, outTTY) }, nil
}

func closeTTYPair(inTTY *os.File, outTTY *os.File) {
	if inTTY != nil {
		_ = inTTY.Close()
	}
	if outTTY != nil && outTTY != inTTY {
		_ = outTTY.Close()
	}
}
