package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const defaultConfigRelPath = "dv/config.yaml"

const (
	flagNameView           = "view"
	flagNameSidebar        = "sidebar"
	flagNameTheme          = "theme"
	flagNameIntralineStyle = "intraline-style"
	flagNameShowSymbols    = "show-symbols"
)

type startupConfig struct {
	View           *string `yaml:"view"`
	Sidebar        *bool   `yaml:"sidebar"`
	Theme          *string `yaml:"theme"`
	IntralineStyle *string `yaml:"intraline-style"`
	ShowSymbols    *bool   `yaml:"show-symbols"`
}

type startupFlagValues struct {
	ViewMode       string
	SidebarVisible bool
	ThemeName      string
	IntralineStyle string
	ShowSymbols    bool
}

type resolvedConfigPath struct {
	Path     string
	Required bool
	Enabled  bool
}

func resolveStartupConfigPath(configHome string, explicitPath string, noConfig bool) resolvedConfigPath {
	if noConfig {
		return resolvedConfigPath{Enabled: false}
	}
	if explicitPath != "" {
		return resolvedConfigPath{
			Path:     explicitPath,
			Required: true,
			Enabled:  true,
		}
	}
	return resolvedConfigPath{
		Path:     filepath.Join(configHome, defaultConfigRelPath),
		Required: false,
		Enabled:  true,
	}
}

func loadStartupConfig(configHome string, explicitPath string, noConfig bool) (startupConfig, error) {
	path := resolveStartupConfigPath(configHome, explicitPath, noConfig)
	if !path.Enabled {
		return startupConfig{}, nil
	}
	return readStartupConfig(path.Path, path.Required)
}

func readStartupConfig(path string, required bool) (startupConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && !required {
			return startupConfig{}, nil
		}
		return startupConfig{}, fmt.Errorf("read config %q: %w", path, err)
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	var cfg startupConfig
	if err := decoder.Decode(&cfg); err != nil {
		if errors.Is(err, io.EOF) {
			return startupConfig{}, nil
		}
		return startupConfig{}, fmt.Errorf("parse config %q: %w", path, err)
	}

	if err := validateStartupConfig(path, cfg); err != nil {
		return startupConfig{}, err
	}

	return cfg, nil
}

func validateStartupConfig(path string, cfg startupConfig) error {
	if cfg.View != nil {
		if _, err := parseDiffLayoutMode(*cfg.View); err != nil {
			return fmt.Errorf("invalid config value for key %q in %q: %w", flagNameView, path, err)
		}
	}

	if cfg.Theme != nil {
		if _, err := parseThemeName(*cfg.Theme); err != nil {
			return fmt.Errorf("invalid config value for key %q in %q: %w", flagNameTheme, path, err)
		}
	}

	if cfg.IntralineStyle != nil {
		if _, err := parseIntralineStyleMode(*cfg.IntralineStyle); err != nil {
			return fmt.Errorf("invalid config value for key %q in %q: %w", flagNameIntralineStyle, path, err)
		}
	}

	return nil
}

func applyStartupConfig(values startupFlagValues, cfg startupConfig, explicitlySet map[string]bool) startupFlagValues {
	if cfg.View != nil && !explicitlySet[flagNameView] {
		values.ViewMode = *cfg.View
	}
	if cfg.Sidebar != nil && !explicitlySet[flagNameSidebar] {
		values.SidebarVisible = *cfg.Sidebar
	}
	if cfg.Theme != nil && !explicitlySet[flagNameTheme] {
		values.ThemeName = *cfg.Theme
	}
	if cfg.IntralineStyle != nil && !explicitlySet[flagNameIntralineStyle] {
		values.IntralineStyle = *cfg.IntralineStyle
	}
	if cfg.ShowSymbols != nil && !explicitlySet[flagNameShowSymbols] {
		values.ShowSymbols = *cfg.ShowSymbols
	}
	return values
}
