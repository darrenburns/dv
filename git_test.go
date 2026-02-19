package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildDiffArgsUnstaged(t *testing.T) {
	args := buildDiffArgs(false)
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
	args := buildDiffArgs(true)
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
