package main

import (
	"bytes"
	"context"
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

type ContextDiffProvider interface {
	LoadDiffContext(ctx context.Context, staged bool, ignoreWhitespace bool) (string, error)
	RepoRootContext(ctx context.Context) (string, error)
	CurrentBranchContext(ctx context.Context) (string, error)
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

type ContextIndexCapable interface {
	StagePathContext(ctx context.Context, path string) error
	StageAllContext(ctx context.Context) error
	UnstagePathContext(ctx context.Context, path string) error
	UnstageAllContext(ctx context.Context) error
}

type CommitCapable interface {
	CommitMessage(message string) error
}

type ContextCommitCapable interface {
	CommitMessageContext(ctx context.Context, message string) error
}

type PushCapable interface {
	PushCurrentBranch() error
}

type ContextPushCapable interface {
	PushCurrentBranchContext(ctx context.Context) error
}

type gitMutationResult struct {
	Args   []string
	Stdout string
	Stderr string
	Err    error
}

type mutationOutputProvider interface {
	stagePathResult(path string) gitMutationResult
	stageAllResult() gitMutationResult
	unstagePathResult(path string) gitMutationResult
	unstageAllResult() gitMutationResult
	commitMessageResult(message string) gitMutationResult
	pushCurrentBranchResult() gitMutationResult
}

type contextMutationOutputProvider interface {
	stagePathResultContext(ctx context.Context, path string) gitMutationResult
	stageAllResultContext(ctx context.Context) gitMutationResult
	unstagePathResultContext(ctx context.Context, path string) gitMutationResult
	unstageAllResultContext(ctx context.Context) gitMutationResult
	commitMessageResultContext(ctx context.Context, message string) gitMutationResult
	pushCurrentBranchResultContext(ctx context.Context) gitMutationResult
}

// GitDiffProvider loads diff data by shelling out to git.
type GitDiffProvider struct {
	WorkDir string
}

func (p GitDiffProvider) LoadDiff(staged bool, ignoreWhitespace bool) (string, error) {
	return p.LoadDiffContext(context.Background(), staged, ignoreWhitespace)
}

func (p GitDiffProvider) LoadDiffContext(ctx context.Context, staged bool, ignoreWhitespace bool) (string, error) {
	args := buildDiffArgs(staged, ignoreWhitespace)
	stdout, stderr, err := runGit(ctx, p.WorkDir, args)
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr))
	}
	return stdout, nil
}

func (p GitDiffProvider) RepoRoot() (string, error) {
	return p.RepoRootContext(context.Background())
}

func (p GitDiffProvider) RepoRootContext(ctx context.Context) (string, error) {
	stdout, stderr, err := runGit(ctx, p.WorkDir, []string{"rev-parse", "--show-toplevel"})
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel failed: %w: %s", err, strings.TrimSpace(stderr))
	}
	return strings.TrimSpace(stdout), nil
}

func (p GitDiffProvider) CurrentBranch() (string, error) {
	return p.CurrentBranchContext(context.Background())
}

func (p GitDiffProvider) CurrentBranchContext(ctx context.Context) (string, error) {
	stdout, stderr, err := runGit(ctx, p.WorkDir, []string{"branch", "--show-current"})
	if err != nil {
		return "", fmt.Errorf("git branch --show-current failed: %w: %s", err, strings.TrimSpace(stderr))
	}
	return strings.TrimSpace(stdout), nil
}

func (p GitDiffProvider) StagePath(path string) error {
	return p.StagePathContext(context.Background(), path)
}

func (p GitDiffProvider) StagePathContext(ctx context.Context, path string) error {
	return runGitMutation(ctx, p.WorkDir, buildStagePathArgs(path))
}

func (p GitDiffProvider) stagePathResult(path string) gitMutationResult {
	return p.stagePathResultContext(context.Background(), path)
}

func (p GitDiffProvider) stagePathResultContext(ctx context.Context, path string) gitMutationResult {
	return runGitMutationResult(ctx, p.WorkDir, buildStagePathArgs(path), "")
}

func (p GitDiffProvider) StageAll() error {
	return p.StageAllContext(context.Background())
}

func (p GitDiffProvider) StageAllContext(ctx context.Context) error {
	return runGitMutation(ctx, p.WorkDir, buildStageAllArgs())
}

func (p GitDiffProvider) stageAllResult() gitMutationResult {
	return p.stageAllResultContext(context.Background())
}

func (p GitDiffProvider) stageAllResultContext(ctx context.Context) gitMutationResult {
	return runGitMutationResult(ctx, p.WorkDir, buildStageAllArgs(), "")
}

func (p GitDiffProvider) UnstagePath(path string) error {
	return p.UnstagePathContext(context.Background(), path)
}

func (p GitDiffProvider) UnstagePathContext(ctx context.Context, path string) error {
	if gitHeadExists(p.WorkDir) {
		return runGitMutation(ctx, p.WorkDir, buildUnstagePathArgs(path))
	}
	return runGitMutation(ctx, p.WorkDir, buildUnstagePathArgsWithoutHead(path))
}

func (p GitDiffProvider) unstagePathResult(path string) gitMutationResult {
	return p.unstagePathResultContext(context.Background(), path)
}

func (p GitDiffProvider) unstagePathResultContext(ctx context.Context, path string) gitMutationResult {
	args := buildUnstagePathArgsWithoutHead(path)
	if gitHeadExists(p.WorkDir) {
		args = buildUnstagePathArgs(path)
	}
	return runGitMutationResult(ctx, p.WorkDir, args, "")
}

func (p GitDiffProvider) UnstageAll() error {
	return p.UnstageAllContext(context.Background())
}

func (p GitDiffProvider) UnstageAllContext(ctx context.Context) error {
	if gitHeadExists(p.WorkDir) {
		return runGitMutation(ctx, p.WorkDir, buildUnstageAllArgs())
	}
	return runGitMutation(ctx, p.WorkDir, buildUnstageAllArgsWithoutHead())
}

func (p GitDiffProvider) unstageAllResult() gitMutationResult {
	return p.unstageAllResultContext(context.Background())
}

func (p GitDiffProvider) unstageAllResultContext(ctx context.Context) gitMutationResult {
	args := buildUnstageAllArgsWithoutHead()
	if gitHeadExists(p.WorkDir) {
		args = buildUnstageAllArgs()
	}
	return runGitMutationResult(ctx, p.WorkDir, args, "")
}

func (p GitDiffProvider) CommitMessage(message string) error {
	return p.CommitMessageContext(context.Background(), message)
}

func (p GitDiffProvider) CommitMessageContext(ctx context.Context, message string) error {
	return runGitMutationWithInput(ctx, p.WorkDir, buildCommitMessageArgs(), message)
}

func (p GitDiffProvider) commitMessageResult(message string) gitMutationResult {
	return p.commitMessageResultContext(context.Background(), message)
}

func (p GitDiffProvider) commitMessageResultContext(ctx context.Context, message string) gitMutationResult {
	return runGitMutationResult(ctx, p.WorkDir, buildCommitMessageArgs(), message)
}

func (p GitDiffProvider) PushCurrentBranch() error {
	return p.PushCurrentBranchContext(context.Background())
}

func (p GitDiffProvider) PushCurrentBranchContext(ctx context.Context) error {
	result := p.pushCurrentBranchResultContext(ctx)
	return result.Err
}

func (p GitDiffProvider) pushCurrentBranchResult() gitMutationResult {
	return p.pushCurrentBranchResultContext(context.Background())
}

func (p GitDiffProvider) pushCurrentBranchResultContext(ctx context.Context) gitMutationResult {
	args, err := p.resolvePushArgs(ctx)
	if err != nil {
		return gitMutationResult{
			Args: buildPushArgs("", false),
			Err:  err,
		}
	}
	return runGitMutationResult(ctx, p.WorkDir, args, "")
}

func (p GitDiffProvider) resolvePushArgs(ctx context.Context) ([]string, error) {
	hasUpstream, err := gitBranchHasUpstream(ctx, p.WorkDir)
	if err != nil {
		return nil, err
	}
	if hasUpstream {
		return buildPushArgs("", false), nil
	}

	remote, err := gitDefaultPushRemote(ctx, p.WorkDir)
	if err != nil {
		return nil, err
	}
	return buildPushArgs(remote, true), nil
}

func runGitMutation(ctx context.Context, workDir string, args []string) error {
	return runGitMutationResult(ctx, workDir, args, "").Err
}

func runGitMutationWithInput(ctx context.Context, workDir string, args []string, input string) error {
	return runGitMutationResult(ctx, workDir, args, input).Err
}

func runGitMutationResult(ctx context.Context, workDir string, args []string, input string) gitMutationResult {
	stdout, stderr, err := runGitWithInput(ctx, workDir, args, input)
	if err != nil {
		err = fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr))
	}
	return gitMutationResult{
		Args:   append([]string(nil), args...),
		Stdout: stdout,
		Stderr: stderr,
		Err:    err,
	}
}

func gitHeadExists(workDir string) bool {
	_, _, err := runGit(context.Background(), workDir, []string{"rev-parse", "--verify", "HEAD"})
	return err == nil
}

func runGit(ctx context.Context, workDir string, args []string) (stdout string, stderr string, err error) {
	return runGitWithInput(ctx, workDir, args, "")
}

func runGitWithInput(ctx context.Context, workDir string, args []string, input string) (stdout string, stderr string, err error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = workDir

	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}

	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

func gitBranchHasUpstream(ctx context.Context, workDir string) (bool, error) {
	_, stderr, err := runGit(ctx, workDir, []string{"rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"})
	if err == nil {
		return true, nil
	}
	if strings.Contains(strings.ToLower(stderr), "no upstream configured") || strings.Contains(strings.ToLower(stderr), "no upstream branch") {
		return false, nil
	}
	return false, fmt.Errorf("git rev-parse --abbrev-ref --symbolic-full-name @{upstream} failed: %w: %s", err, strings.TrimSpace(stderr))
}

func gitDefaultPushRemote(ctx context.Context, workDir string) (string, error) {
	stdout, stderr, err := runGit(ctx, workDir, []string{"remote"})
	if err != nil {
		return "", fmt.Errorf("git remote failed: %w: %s", err, strings.TrimSpace(stderr))
	}

	remotes := parseGitRemotes(stdout)
	return selectDefaultPushRemote(remotes)
}

func parseGitRemotes(stdout string) []string {
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	remotes := make([]string, 0, len(lines))
	for _, line := range lines {
		remote := strings.TrimSpace(line)
		if remote == "" {
			continue
		}
		remotes = append(remotes, remote)
	}
	return remotes
}

func selectDefaultPushRemote(remotes []string) (string, error) {
	if len(remotes) == 0 {
		return "", fmt.Errorf("no git remotes configured")
	}
	if len(remotes) == 1 && remotes[0] == "origin" {
		return "origin", nil
	}
	return "", fmt.Errorf("no upstream branch configured and refusing to guess a push remote")
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

func buildCommitMessageArgs() []string {
	return []string{"commit", "--file", "-"}
}

func buildPushArgs(remote string, setUpstream bool) []string {
	args := []string{"push"}
	if setUpstream {
		args = append(args, "--set-upstream", remote, "HEAD")
	}
	return args
}
