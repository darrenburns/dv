package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStdinDiffProvider_LoadDiff(t *testing.T) {
	provider := StdinDiffProvider{
		WorkDir: "/tmp/repo",
		Diff:    diffForPaths("a.txt"),
	}

	unstaged, err := provider.LoadDiff(false)
	require.NoError(t, err)
	require.Equal(t, provider.Diff, unstaged)

	staged, err := provider.LoadDiff(true)
	require.NoError(t, err)
	require.Empty(t, staged)
}

func TestStdinDiffProvider_SectionsAndManualRefresh(t *testing.T) {
	provider := StdinDiffProvider{}
	require.Equal(t, []DiffSection{DiffSectionFiles}, provider.Sections())
	require.False(t, provider.ManualRefreshEnabled())
}
