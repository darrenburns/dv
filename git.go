package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// DiffProvider abstracts where git diff content comes from.
type DiffProvider interface {
	LoadDiff(staged bool) (string, error)
	RepoRoot() (string, error)
	CurrentBranch() (string, error)
}

// GitDiffProvider loads diff data by shelling out to git.
type GitDiffProvider struct {
	WorkDir string
}

func (p GitDiffProvider) LoadDiff(staged bool) (string, error) {
	args := buildDiffArgs(staged)
	stdout, stderr, err := runGit(p.WorkDir, args)
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr))
	}
	return stdout, nil
}

func (p GitDiffProvider) RepoRoot() (string, error) {
	stdout, stderr, err := runGit(p.WorkDir, []string{"rev-parse", "--show-toplevel"})
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel failed: %w: %s", err, strings.TrimSpace(stderr))
	}
	return strings.TrimSpace(stdout), nil
}

func (p GitDiffProvider) CurrentBranch() (string, error) {
	stdout, stderr, err := runGit(p.WorkDir, []string{"branch", "--show-current"})
	if err != nil {
		return "", fmt.Errorf("git branch --show-current failed: %w: %s", err, strings.TrimSpace(stderr))
	}
	return strings.TrimSpace(stdout), nil
}

func runGit(workDir string, args []string) (stdout string, stderr string, err error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = workDir

	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

func buildDiffArgs(staged bool) []string {
	args := []string{
		"-c", "color.ui=never",
		"diff",
		"--no-color",
		"--no-ext-diff",
		"--patch",
		"--find-renames",
	}
	if staged {
		args = append(args, "--staged")
	}
	return args
}
