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

func TestBuildStagePathArgs(t *testing.T) {
	args := buildStagePathArgs("dir/file.txt")
	require.Equal(t, []string{"add", "--", "dir/file.txt"}, args)
}

func TestBuildStageAllArgs(t *testing.T) {
	args := buildStageAllArgs()
	require.Equal(t, []string{"add", "--all"}, args)
}

func TestBuildUnstagePathArgs(t *testing.T) {
	args := buildUnstagePathArgs("dir/file.txt")
	require.Equal(t, []string{"restore", "--staged", "--", "dir/file.txt"}, args)
}

func TestBuildUnstageAllArgs(t *testing.T) {
	args := buildUnstageAllArgs()
	require.Equal(t, []string{"restore", "--staged", "--", ":/"}, args)
}

func TestBuildUnstagePathArgsWithoutHead(t *testing.T) {
	args := buildUnstagePathArgsWithoutHead("dir/file.txt")
	require.Equal(t, []string{"rm", "--cached", "--", "dir/file.txt"}, args)
}

func TestBuildUnstageAllArgsWithoutHead(t *testing.T) {
	args := buildUnstageAllArgsWithoutHead()
	require.Equal(t, []string{"rm", "--cached", "-r", "--", ":/"}, args)
}
