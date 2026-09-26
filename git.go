package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Repository access is read-only. Review writes go exclusively to ReviewStore.
func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "GIT_PAGER=cat")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(stderr.String()), err)
	}
	return string(data), nil
}
func repoRoot(dir string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	value, err := runGit(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(strings.TrimSpace(value))
}
func captureWorkingTree(repo string) (ReviewSnapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	head, err := runGit(ctx, repo, "rev-parse", "--verify", "HEAD")
	if err != nil {
		// Empty repositories have no HEAD; compare against Git's empty tree.
		head = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	}
	head = strings.TrimSpace(head)
	diff, err := runGit(ctx, repo, "-c", "core.quotePath=false", "diff", "--no-ext-diff", "--no-textconv", "--no-color", "--find-renames", head, "--")
	if err != nil {
		return ReviewSnapshot{}, err
	}
	untracked, err := runGit(ctx, repo, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return ReviewSnapshot{}, err
	}
	for _, path := range strings.Split(untracked, "\x00") {
		if path == "" {
			continue
		}
		info, err := os.Lstat(filepath.Join(repo, path))
		if err != nil {
			return ReviewSnapshot{}, err
		}
		if info.IsDir() {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		} // Never follow untracked symlinks.
		if info.Size() > 2*1024*1024 {
			diff += fmt.Sprintf("diff --git %s %s\nBinary files /dev/null and %s differ\n", strconv.Quote("a/"+path), strconv.Quote("b/"+path), strconv.Quote("b/"+path))
			continue
		}
		cmd := exec.CommandContext(ctx, "git", "-c", "core.quotePath=false", "diff", "--no-index", "--no-ext-diff", "--no-textconv", "--no-color", "--", "/dev/null", path)
		cmd.Dir = repo
		out, err := cmd.Output()
		var exit *exec.ExitError
		if err != nil && !(errors.As(err, &exit) && exit.ExitCode() == 1) {
			return ReviewSnapshot{}, err
		}
		if len(out) == 0 {
			out = []byte(fmt.Sprintf("diff --git %s %s\nnew file mode 100644\n", strconv.Quote("a/"+path), strconv.Quote("b/"+path)))
		}
		diff += string(out)
	}
	files, err := runGit(ctx, repo, "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	if err != nil {
		return ReviewSnapshot{}, err
	}
	unique := map[string]bool{}
	var paths []string
	for _, path := range strings.Split(files, "\x00") {
		if path != "" && !unique[path] {
			paths = append(paths, path)
			unique[path] = true
		}
	}
	sort.Strings(paths)
	return ReviewSnapshot{Diff: diff, Base: head, Head: "working-tree", Files: paths}, nil
}
func captureRange(repo, base, head string) (ReviewSnapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	resolve := func(ref string) (string, error) {
		v, e := runGit(ctx, repo, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
		return strings.TrimSpace(v), e
	}
	b, err := resolve(base)
	if err != nil {
		return ReviewSnapshot{}, err
	}
	h, err := resolve(head)
	if err != nil {
		return ReviewSnapshot{}, err
	}
	b, err = runGit(ctx, repo, "merge-base", b, h)
	if err != nil {
		return ReviewSnapshot{}, err
	}
	b = strings.TrimSpace(b)
	diff, err := runGit(ctx, repo, "-c", "core.quotePath=false", "diff", "--no-ext-diff", "--no-textconv", "--no-color", "--find-renames", b, h, "--")
	return ReviewSnapshot{Diff: diff, Base: b, Head: h}, err
}
func openReview(store ReviewStore, dir, base, head, pr string) (*Review, error) {
	if pr != "" && base != "" {
		return nil, errors.New("choose either PR or base, not both")
	}
	if base == "" && pr == "" && head != "" && head != "HEAD" {
		return nil, errors.New("head requires a base ref")
	}
	repo, err := repoRoot(dir)
	if err != nil {
		return nil, err
	}
	if pr != "" {
		snapshot, title, source, err := capturePR(repo, pr)
		if err != nil {
			return nil, err
		}
		return store.Open(repo, title, source, snapshot)
	}
	if base != "" {
		if head == "" {
			head = "HEAD"
		}
		snapshot, err := captureRange(repo, base, head)
		if err != nil {
			return nil, err
		}
		return store.Open(repo, base+" → "+head, "range:"+base+"..."+head, snapshot)
	}
	snapshot, err := captureWorkingTree(repo)
	if err != nil {
		return nil, err
	}
	return store.Open(repo, filepath.Base(repo)+" · Working changes", "worktree", snapshot)
}
func capturePR(repo, pr string) (ReviewSnapshot, string, string, error) {
	if strings.HasPrefix(pr, "-") {
		return ReviewSnapshot{}, "", "", errors.New("invalid PR number or URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	run := func(args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, "gh", args...)
		cmd.Dir = repo
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		data, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("gh: %s: %w", strings.TrimSpace(stderr.String()), err)
		}
		return data, nil
	}
	type metadata struct{ BaseRefOid, HeadRefOid, Title, URL string }
	read := func() (metadata, error) {
		var m metadata
		data, err := run("pr", "view", pr, "--json", "baseRefOid,headRefOid,title,url")
		if err == nil {
			err = json.Unmarshal(data, &m)
		}
		return m, err
	}
	before, err := read()
	if err != nil {
		return ReviewSnapshot{}, "", "", err
	}
	if before.BaseRefOid == "" || before.HeadRefOid == "" || before.URL == "" {
		return ReviewSnapshot{}, "", "", errors.New("PR metadata is incomplete")
	}
	diff, err := run("pr", "diff", pr, "--color", "never")
	if err != nil {
		return ReviewSnapshot{}, "", "", err
	}
	after, err := read()
	if err != nil {
		return ReviewSnapshot{}, "", "", err
	}
	if before.BaseRefOid != after.BaseRefOid || before.HeadRefOid != after.HeadRefOid {
		return ReviewSnapshot{}, "", "", errors.New("PR changed while reading; retry to capture a consistent review")
	}
	return ReviewSnapshot{Diff: string(diff), Base: before.BaseRefOid, Head: before.HeadRefOid}, before.Title, before.URL, nil
}
