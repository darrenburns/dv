package main

import (
	"testing"

	terma "github.com/darrenburns/terma"
	"github.com/stretchr/testify/require"
)

func TestStartupInitialStateFromFlags_ParsesValues(t *testing.T) {
	initialState, err := startupInitialStateFromFlags("split", false, "catpuccin", "underline", true)
	require.NoError(t, err)
	require.Equal(t, DiffLayoutSideBySide, initialState.LayoutMode)
	require.False(t, initialState.SidebarVisible)
	require.Equal(t, terma.ThemeNameCatppuccin, initialState.ThemeName)
	require.Equal(t, IntralineStyleModeUnderline, initialState.IntralineStyle)
	require.True(t, initialState.ShowChangeSigns)
}

func TestParseDiffLayoutMode(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    DiffLayoutMode
		wantErr bool
	}{
		{name: "unified", value: "unified", want: DiffLayoutUnified},
		{name: "split", value: "split", want: DiffLayoutSideBySide},
		{name: "sideBySideAlias", value: "side-by-side", want: DiffLayoutSideBySide},
		{name: "invalid", value: "stacked", wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseDiffLayoutMode(tc.value)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestParseIntralineStyleMode(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    IntralineStyleMode
		wantErr bool
	}{
		{name: "background", value: "background", want: IntralineStyleModeBackground},
		{name: "bgAlias", value: "bg", want: IntralineStyleModeBackground},
		{name: "underline", value: "underline", want: IntralineStyleModeUnderline},
		{name: "invalid", value: "outline", wantErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseIntralineStyleMode(tc.value)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestParseThemeName(t *testing.T) {
	themeName, err := parseThemeName("catpuccin")
	require.NoError(t, err)
	require.Equal(t, terma.ThemeNameCatppuccin, themeName)

	_, err = parseThemeName("missing-theme")
	require.Error(t, err)
	require.ErrorContains(t, err, "--theme")
}
