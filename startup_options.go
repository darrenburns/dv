package main

import (
	"fmt"
	"strings"

	t "github.com/darrenburns/terma"
)

const themeAliasCatpuccin = "catpuccin"

func startupInitialStateFromFlags(viewMode string, sidebarVisible bool, themeName string, intralineStyle string, showSymbols bool) (DvInitialState, error) {
	layoutMode, err := parseDiffLayoutMode(viewMode)
	if err != nil {
		return DvInitialState{}, err
	}

	parsedThemeName, err := parseThemeName(themeName)
	if err != nil {
		return DvInitialState{}, err
	}

	parsedIntralineStyle, err := parseIntralineStyleMode(intralineStyle)
	if err != nil {
		return DvInitialState{}, err
	}

	return DvInitialState{
		LayoutMode:      layoutMode,
		SidebarVisible:  sidebarVisible,
		ThemeName:       parsedThemeName,
		IntralineStyle:  parsedIntralineStyle,
		ShowChangeSigns: showSymbols,
	}, nil
}

func parseDiffLayoutMode(value string) (DiffLayoutMode, error) {
	switch normalizeCLIValue(value) {
	case "unified":
		return DiffLayoutUnified, nil
	case "split", "side-by-side", "sidebyside":
		return DiffLayoutSideBySide, nil
	default:
		return DiffLayoutUnified, fmt.Errorf("invalid --view value %q (expected \"unified\" or \"split\")", value)
	}
}

func parseIntralineStyleMode(value string) (IntralineStyleMode, error) {
	switch normalizeCLIValue(value) {
	case "background", "bg":
		return IntralineStyleModeBackground, nil
	case "underline":
		return IntralineStyleModeUnderline, nil
	default:
		return IntralineStyleModeBackground, fmt.Errorf("invalid --intraline-style value %q (expected \"background\" or \"underline\")", value)
	}
}

func parseThemeName(value string) (string, error) {
	normalized := normalizeCLIValue(value)
	if normalized == themeAliasCatpuccin {
		normalized = t.ThemeNameCatppuccin
	}
	if _, ok := t.GetTheme(normalized); ok {
		return normalized, nil
	}
	return "", fmt.Errorf("invalid --theme value %q (available themes: %s)", value, strings.Join(t.ThemeNames(), ", "))
}

func normalizeCLIValue(value string) string {
	normalized := strings.TrimSpace(strings.ToLower(value))
	normalized = strings.ReplaceAll(normalized, "_", "-")
	normalized = strings.ReplaceAll(normalized, " ", "-")
	return normalized
}
