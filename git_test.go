package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildDiffArgsUnstaged(t *testing.T) {
	args := buildDiffArgs(false, false)
	require.Equal(t, []string{
		"-c", "color.ui=never",
		"diff",
		"--no-color",
		"--no-ext-diff",
		"--patch",
		"--find-renames",
	}, args)
}

func TestBuildDiffArgsStaged(t *testing.T) {
	args := buildDiffArgs(true, false)
	require.Equal(t, []string{
		"-c", "color.ui=never",
		"diff",
		"--no-color",
		"--no-ext-diff",
		"--patch",
		"--find-renames",
		"--staged",
	}, args)
}

func TestBuildDiffArgsUnstagedIgnoreWhitespace(t *testing.T) {
	args := buildDiffArgs(false, true)
	require.Equal(t, []string{
		"-c", "color.ui=never",
		"diff",
		"--no-color",
		"--no-ext-diff",
		"--patch",
		"--find-renames",
		"--ignore-all-space",
	}, args)
}

func TestBuildDiffArgsStagedIgnoreWhitespace(t *testing.T) {
	args := buildDiffArgs(true, true)
	require.Equal(t, []string{
		"-c", "color.ui=never",
		"diff",
		"--no-color",
		"--no-ext-diff",
		"--patch",
		"--find-renames",
		"--ignore-all-space",
		"--staged",
	}, args)
}
