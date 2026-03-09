package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// DiffProvider abstracts where git diff content comes from.
type DiffProvider interface {
	LoadDiff(staged bool, ignoreWhitespace bool) (string, error)
	RepoRoot() (string, error)
	CurrentBranch() (string, error)
}

// DiffSectionsProvider optionally customizes which sections dv should render.
type DiffSectionsProvider interface {
	Sections() []DiffSection
}

// ManualRefreshCapable optionally controls whether manual refresh is enabled.
type ManualRefreshCapable interface {
	ManualRefreshEnabled() bool
}

// IndexCapable optionally supports staging and unstaging through the provider.
type IndexCapable interface {
	StagePath(path string) error
	StageAll() error
	UnstagePath(path string) error
	UnstageAll() error
}

// GitDiffProvider loads diff data by shelling out to git.
type GitDiffProvider struct {
	WorkDir string
}

func (p GitDiffProvider) LoadDiff(staged bool, ignoreWhitespace bool) (string, error) {
	args := buildDiffArgs(staged, ignoreWhitespace)
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

func (p GitDiffProvider) StagePath(path string) error {
	return runGitMutation(p.WorkDir, buildStagePathArgs(path))
}

func (p GitDiffProvider) StageAll() error {
	args := buildStageAllArgs()
	return runGitMutation(p.WorkDir, args)
}

func (p GitDiffProvider) UnstagePath(path string) error {
	if gitHeadExists(p.WorkDir) {
		return runGitMutation(p.WorkDir, buildUnstagePathArgs(path))
	}
	return runGitMutation(p.WorkDir, buildUnstagePathArgsWithoutHead(path))
}

func (p GitDiffProvider) UnstageAll() error {
	if gitHeadExists(p.WorkDir) {
		return runGitMutation(p.WorkDir, buildUnstageAllArgs())
	}
	return runGitMutation(p.WorkDir, buildUnstageAllArgsWithoutHead())
}

func runGitMutation(workDir string, args []string) error {
	_, stderr, err := runGit(workDir, args)
	if err != nil {
		return fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr))
	}
	return nil
}

func gitHeadExists(workDir string) bool {
	_, _, err := runGit(workDir, []string{"rev-parse", "--verify", "HEAD"})
	return err == nil
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

func buildDiffArgs(staged bool, ignoreWhitespace bool) []string {
	args := []string{
		"-c", "color.ui=never",
		"diff",
		"--no-color",
		"--no-ext-diff",
		"--patch",
		"--find-renames",
	}
	if ignoreWhitespace {
		args = append(args, "--ignore-all-space")
	}
	if staged {
		args = append(args, "--staged")
	}
	return args
}

func buildStagePathArgs(path string) []string {
	return []string{"add", "--", path}
}

func buildStageAllArgs() []string {
	return []string{"add", "--all"}
}

func buildUnstagePathArgs(path string) []string {
	return []string{"restore", "--staged", "--", path}
}

func buildUnstageAllArgs() []string {
	return []string{"restore", "--staged", "--", ":/"}
}

func buildUnstagePathArgsWithoutHead(path string) []string {
	return []string{"rm", "--cached", "--", path}
}

func buildUnstageAllArgsWithoutHead() []string {
	return []string{"rm", "--cached", "-r", "--", ":/"}
}
