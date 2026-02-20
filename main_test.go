package main

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStartupDiffProvider_UsesGitProviderWhenStdinNotPiped(t *testing.T) {
	provider, cleanup, err := startupDiffProvider("/tmp/repo", strings.NewReader("ignored"), false, nil, nil)
	require.NoError(t, err)
	defer cleanup()

	gitProvider, ok := provider.(GitDiffProvider)
	require.True(t, ok)
	require.Equal(t, "/tmp/repo", gitProvider.WorkDir)
}

func TestStartupDiffProvider_UsesStdinProviderWhenPiped(t *testing.T) {
	inTTY, err := os.CreateTemp("", "dv-stdin-tty-in-*")
	require.NoError(t, err)
	outTTY, err := os.CreateTemp("", "dv-stdin-tty-out-*")
	require.NoError(t, err)
	defer os.Remove(inTTY.Name())
	defer os.Remove(outTTY.Name())

	var assignedStdin *os.File
	diff := diffForPaths("piped.txt")
	provider, cleanup, err := startupDiffProvider(
		"/tmp/repo",
		strings.NewReader(diff),
		true,
		func() (*os.File, *os.File, error) {
			return inTTY, outTTY, nil
		},
		func(file *os.File) {
			assignedStdin = file
		},
	)
	require.NoError(t, err)
	defer cleanup()

	stdinProvider, ok := provider.(StdinDiffProvider)
	require.True(t, ok)
	require.Equal(t, "/tmp/repo", stdinProvider.WorkDir)
	require.Equal(t, diff, stdinProvider.Diff)
	require.Equal(t, inTTY, assignedStdin)
}

func TestStartupDiffProvider_ReturnsClearErrorWhenTTYRebindFails(t *testing.T) {
	expectedErr := errors.New("tty unavailable")
	setCalled := false
	_, cleanup, err := startupDiffProvider(
		"/tmp/repo",
		strings.NewReader(diffForPaths("a.txt")),
		true,
		func() (*os.File, *os.File, error) {
			return nil, nil, expectedErr
		},
		func(file *os.File) {
			setCalled = true
		},
	)
	cleanup()
	require.Error(t, err)
	require.ErrorIs(t, err, expectedErr)
	require.ErrorContains(t, err, "reopen terminal input after reading piped stdin")
	require.False(t, setCalled)
}
