package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveStartupConfigPath(t *testing.T) {
	configHome := "/tmp/xdg-config-home"

	t.Run("defaultPath", func(t *testing.T) {
		path := resolveStartupConfigPath(configHome, "", false)
		require.True(t, path.Enabled)
		require.False(t, path.Required)
		require.Equal(t, filepath.Join(configHome, defaultConfigRelPath), path.Path)
	})

	t.Run("explicitPath", func(t *testing.T) {
		path := resolveStartupConfigPath(configHome, "/tmp/custom.yaml", false)
		require.True(t, path.Enabled)
		require.True(t, path.Required)
		require.Equal(t, "/tmp/custom.yaml", path.Path)
	})

	t.Run("noConfigWins", func(t *testing.T) {
		path := resolveStartupConfigPath(configHome, "/tmp/custom.yaml", true)
		require.False(t, path.Enabled)
		require.False(t, path.Required)
		require.Empty(t, path.Path)
	})
}

func TestLoadStartupConfig_DefaultMissingFileIsOptional(t *testing.T) {
	configHome := t.TempDir()
	cfg, err := loadStartupConfig(configHome, "", false)
	require.NoError(t, err)
	require.Equal(t, startupConfig{}, cfg)
}

func TestLoadStartupConfig_ExplicitMissingFileErrors(t *testing.T) {
	configHome := t.TempDir()
	_, err := loadStartupConfig(configHome, filepath.Join(configHome, "missing.yaml"), false)
	require.Error(t, err)
	require.ErrorContains(t, err, "read config")
	require.ErrorContains(t, err, "missing.yaml")
}

func TestLoadStartupConfig_NoConfigSkipsLoading(t *testing.T) {
	configHome := t.TempDir()
	configPath := filepath.Join(configHome, "custom.yaml")
	writeTestConfig(t, configPath, "view: [")

	cfg, err := loadStartupConfig(configHome, configPath, true)
	require.NoError(t, err)
	require.Equal(t, startupConfig{}, cfg)
}

func TestLoadStartupConfig_ParsesValidYAML(t *testing.T) {
	configHome := t.TempDir()
	configPath := filepath.Join(configHome, "custom.yaml")
	writeTestConfig(t, configPath, `
view: split
theme: catppuccin
intraline-style: underline
`)

	cfg, err := loadStartupConfig(configHome, configPath, false)
	require.NoError(t, err)
	require.NotNil(t, cfg.View)
	require.NotNil(t, cfg.Theme)
	require.NotNil(t, cfg.IntralineStyle)
	require.Equal(t, "split", *cfg.View)
	require.Equal(t, "catppuccin", *cfg.Theme)
	require.Equal(t, "underline", *cfg.IntralineStyle)
}

func TestLoadStartupConfig_ParsesIntralineStyleOff(t *testing.T) {
	configHome := t.TempDir()
	configPath := filepath.Join(configHome, "custom.yaml")
	writeTestConfig(t, configPath, `
intraline-style: off
`)

	cfg, err := loadStartupConfig(configHome, configPath, false)
	require.NoError(t, err)
	require.NotNil(t, cfg.IntralineStyle)
	require.Equal(t, "off", *cfg.IntralineStyle)
}

func TestLoadStartupConfig_UnknownKeyErrors(t *testing.T) {
	configHome := t.TempDir()
	configPath := filepath.Join(configHome, "custom.yaml")
	writeTestConfig(t, configPath, `
view: split
unknown: true
`)

	_, err := loadStartupConfig(configHome, configPath, false)
	require.Error(t, err)
	require.ErrorContains(t, err, "parse config")
	require.ErrorContains(t, err, "unknown")
}

func TestLoadStartupConfig_InvalidYAMLErrors(t *testing.T) {
	configHome := t.TempDir()
	configPath := filepath.Join(configHome, "custom.yaml")
	writeTestConfig(t, configPath, "view: [")

	_, err := loadStartupConfig(configHome, configPath, false)
	require.Error(t, err)
	require.ErrorContains(t, err, "parse config")
}

func TestLoadStartupConfig_InvalidValuesIncludeKeyContext(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		wantKeyName string
	}{
		{
			name:        "view",
			yaml:        "view: stacked\n",
			wantKeyName: flagNameView,
		},
		{
			name:        "theme",
			yaml:        "theme: missing-theme\n",
			wantKeyName: flagNameTheme,
		},
		{
			name:        "intralineStyle",
			yaml:        "intraline-style: outline\n",
			wantKeyName: flagNameIntralineStyle,
		},
		{
			name:        "stagedScopeIsRejected",
			yaml:        "staged: true\n",
			wantKeyName: "staged",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			configHome := t.TempDir()
			configPath := filepath.Join(configHome, "custom.yaml")
			writeTestConfig(t, configPath, tc.yaml)

			_, err := loadStartupConfig(configHome, configPath, false)
			require.Error(t, err)
			require.ErrorContains(t, err, tc.wantKeyName)
		})
	}
}

func TestApplyStartupConfig_AppliesWhenFlagNotSet(t *testing.T) {
	view := "split"
	theme := "dracula"
	intralineStyle := "underline"

	got := applyStartupConfig(
		startupFlagValues{
			ViewMode:       "unified",
			ThemeName:      "catppuccin",
			IntralineStyle: "background",
		},
		startupConfig{
			View:           &view,
			Theme:          &theme,
			IntralineStyle: &intralineStyle,
		},
		map[string]bool{},
	)

	require.Equal(t, "split", got.ViewMode)
	require.Equal(t, "dracula", got.ThemeName)
	require.Equal(t, "underline", got.IntralineStyle)
}

func TestApplyStartupConfig_FlagsOverrideConfig(t *testing.T) {
	view := "split"
	theme := "dracula"
	intralineStyle := "underline"

	got := applyStartupConfig(
		startupFlagValues{
			ViewMode:       "unified",
			ThemeName:      "catppuccin",
			IntralineStyle: "background",
		},
		startupConfig{
			View:           &view,
			Theme:          &theme,
			IntralineStyle: &intralineStyle,
		},
		map[string]bool{
			flagNameView:           true,
			flagNameTheme:          true,
			flagNameIntralineStyle: true,
		},
	)

	require.Equal(t, "unified", got.ViewMode)
	require.Equal(t, "catppuccin", got.ThemeName)
	require.Equal(t, "background", got.IntralineStyle)
}

func writeTestConfig(t *testing.T, path string, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
