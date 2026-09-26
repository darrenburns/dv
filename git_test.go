package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func testGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	out, err := runGit(context.Background(), repo, args...)
	require.NoError(t, err)
	return strings.TrimSpace(out)
}
func fixtureRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	repo, _ = filepath.EvalSymlinks(repo)
	testGit(t, repo, "init", "-q")
	testGit(t, repo, "config", "user.name", "Aside tests")
	testGit(t, repo, "config", "user.email", "aside@example.test")
	testGit(t, repo, "config", "commit.gpgsign", "false")
	require.NoError(t, os.WriteFile(filepath.Join(repo, "main.go"), []byte("package main\n\nfunc greeting() string {\n    return \"Hello\"\n}\n"), 0600))
	testGit(t, repo, "add", "main.go")
	testGit(t, repo, "commit", "-qm", "fixture")
	require.NoError(t, os.WriteFile(filepath.Join(repo, "main.go"), []byte("package main\n\nfunc greeting() string {\n    // Welcome everyone.\n    return \"Hello, 世界\"\n}\n"), 0600))
	return repo
}
func TestCaptureWorkingTreeReadOnlyAndUntracked(t *testing.T) {
	repo := fixtureRepo(t)
	for path, body := range map[string]string{"space name.txt": "hello\n", "empty.txt": "", "binary.bin": "\x00\xff\x00"} {
		require.NoError(t, os.WriteFile(filepath.Join(repo, path), []byte(body), 0600))
	}
	testGit(t, repo, "add", "main.go")
	before := testGit(t, repo, "status", "--porcelain=v1")
	indexBefore := testGit(t, repo, "diff", "--cached")
	head := testGit(t, repo, "rev-parse", "HEAD")
	snap, err := captureWorkingTree(repo)
	require.NoError(t, err)
	doc, err := snapshotDocument(snap)
	require.NoError(t, err)
	byPath := map[string]*DiffFile{}
	for _, file := range doc.Files {
		byPath[file.DisplayPath] = file
	}
	require.Contains(t, byPath, "main.go")
	require.Contains(t, byPath, "space name.txt")
	require.Contains(t, byPath, "empty.txt")
	require.Contains(t, byPath, "binary.bin")
	require.True(t, byPath["binary.bin"].IsBinary)
	require.Equal(t, before, testGit(t, repo, "status", "--porcelain=v1"))
	require.Equal(t, indexBefore, testGit(t, repo, "diff", "--cached"))
	require.Equal(t, head, testGit(t, repo, "rev-parse", "HEAD"))
}
func TestCaptureUnbornAndDeleted(t *testing.T) {
	repo := t.TempDir()
	testGit(t, repo, "init", "-q")
	require.NoError(t, os.WriteFile(filepath.Join(repo, "new.txt"), []byte("new\n"), 0600))
	snap, err := captureWorkingTree(repo)
	require.NoError(t, err)
	require.Contains(t, snap.Diff, "+new")
	repo = fixtureRepo(t)
	require.NoError(t, os.Remove(filepath.Join(repo, "main.go")))
	snap, err = captureWorkingTree(repo)
	require.NoError(t, err)
	snap.ID = "test"
	anchor, err := anchorFor(snap, "main.go", "old", 4, 4)
	require.NoError(t, err)
	require.Contains(t, anchor.Quote, "Hello")
	_, err = anchorFor(snap, "main.go", "new", 4, 4)
	require.Error(t, err)
}
func TestCaptureRangeIsPinnedWithoutCheckout(t *testing.T) {
	repo := fixtureRepo(t)
	base := testGit(t, repo, "rev-parse", "HEAD")
	testGit(t, repo, "add", ".")
	testGit(t, repo, "commit", "-qm", "change")
	head := testGit(t, repo, "rev-parse", "HEAD")
	before := testGit(t, repo, "status", "--porcelain=v1")
	snap, err := captureRange(repo, base, head)
	require.NoError(t, err)
	require.Equal(t, base, snap.Base)
	require.Equal(t, head, snap.Head)
	require.Contains(t, snap.Diff, "Hello, 世界")
	require.Equal(t, before, testGit(t, repo, "status", "--porcelain=v1"))
	require.Equal(t, head, testGit(t, repo, "rev-parse", "HEAD"))
}

func TestCaptureUnusualPaths(t *testing.T) {
	repo := fixtureRepo(t)
	names := []string{"empty space.txt", "binary space.bin", "tab\tname.txt", "quote\"name.txt", "世界.txt", "line\nname.txt"}
	for _, name := range names {
		body := "hello\n"
		if strings.HasPrefix(name, "empty") {
			body = ""
		}
		if strings.HasPrefix(name, "binary") {
			body = "\x00\xff"
		}
		require.NoError(t, os.WriteFile(filepath.Join(repo, name), []byte(body), 0600))
	}
	snap, err := captureWorkingTree(repo)
	require.NoError(t, err)
	doc, err := snapshotDocument(snap)
	require.NoError(t, err)
	var paths []string
	for _, f := range doc.Files {
		paths = append(paths, f.DisplayPath)
	}
	for _, name := range names {
		require.Contains(t, paths, name)
	}
}

func TestCapturePRConsistency(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX gh fixture")
	}
	for _, moving := range []bool{false, true} {
		t.Run(fmt.Sprint(moving), func(t *testing.T) {
			dir := t.TempDir()
			script := `#!/bin/sh
if [ "$2" = view ]; then
  if [ -f "$ASIDE_GH_COUNTER" ] && [ "$ASIDE_GH_MOVING" = true ]; then head=changed; else head=head; fi
  touch "$ASIDE_GH_COUNTER"
  printf '{"baseRefOid":"base","headRefOid":"%s","title":"Fixture PR","url":"https://example.test/pr/1"}\n' "$head"
else
  cat "$ASIDE_GH_DIFF"
fi
`
			require.NoError(t, os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0700))
			diffPath := filepath.Join(dir, "diff")
			require.NoError(t, os.WriteFile(diffPath, []byte(reviewFixture), 0600))
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("ASIDE_GH_COUNTER", filepath.Join(dir, "counter"))
			t.Setenv("ASIDE_GH_MOVING", fmt.Sprint(moving))
			t.Setenv("ASIDE_GH_DIFF", diffPath)
			snap, title, source, err := capturePR(dir, "123")
			if moving {
				require.ErrorContains(t, err, "changed while reading")
				return
			}
			require.NoError(t, err)
			require.Equal(t, "head", snap.Head)
			require.Equal(t, reviewFixture, snap.Diff)
			require.Equal(t, "Fixture PR", title)
			require.Equal(t, "https://example.test/pr/1", source)
		})
	}
}
func TestCaptureRenameAndSymlink(t *testing.T) {
	repo := fixtureRepo(t)
	testGit(t, repo, "add", ".")
	testGit(t, repo, "commit", "-qm", "baseline")
	testGit(t, repo, "mv", "main.go", "new name.go")
	external := filepath.Join(t.TempDir(), "private-target.txt")
	require.NoError(t, os.WriteFile(external, []byte("target content must stay outside review"), 0600))
	require.NoError(t, os.Symlink(external, filepath.Join(repo, "link")))
	snap, err := captureWorkingTree(repo)
	require.NoError(t, err)
	require.NotContains(t, snap.Diff, "target content must stay outside review")
	doc, err := snapshotDocument(snap)
	require.NoError(t, err)
	require.Equal(t, "new name.go", doc.Files[0].DisplayPath)
	require.Contains(t, snap.Files, "link")
}
