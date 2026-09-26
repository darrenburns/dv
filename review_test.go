package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

const reviewFixture = `diff --git a/main.go b/main.go
index 0000000..1111111 100644
--- a/main.go
+++ b/main.go
@@ -1,5 +1,6 @@
 package main
` + " \n" + ` func greeting() string {
-    return "Hello"
+    // Welcome everyone.
+    return "Hello, 世界"
 }
`

func newTestReview(t *testing.T) (ReviewStore, *Review) {
	t.Helper()
	store := ReviewStore{Dir: t.TempDir()}
	r, err := store.Open("/fixture", "Greeting review", "worktree", ReviewSnapshot{Diff: reviewFixture, Base: "base", Head: "working-tree", Files: []string{"main.go", "README.md"}})
	require.NoError(t, err)
	return store, r
}
func annotation(r *Review, key string) AnnotationInput {
	return AnnotationInput{Key: key, Snapshot: r.Current, Path: "main.go", Side: "new", Start: 4, End: 5, Kind: "finding", Body: "Handle an empty name before formatting."}
}
func TestReviewRoundTrip(t *testing.T) {
	store, r := newTestReview(t)
	input := annotation(r, "human")
	input.Kind = "feedback"
	r, err := store.Publish(r.ID, "you", []AnnotationInput{input}, false)
	require.NoError(t, err)
	require.NotContains(t, feedbackMarkdown(r, 0), input.Body)
	r, err = store.Submit(r.ID)
	require.NoError(t, err)
	cursor := r.Revision
	exported := feedbackMarkdown(r, 0)
	require.Contains(t, exported, "new lines 4–5")
	require.Contains(t, exported, `return "Hello, 世界"`)
	require.Contains(t, exported, input.Body)
	r, err = store.Reply(r.ID, r.Threads[0].ID, "agent", "Added a regression test.")
	require.NoError(t, err)
	require.Contains(t, feedbackMarkdown(r, cursor), "regression test")
	r, err = store.SetStatus(r.ID, r.Threads[0].ID, "addressed")
	require.NoError(t, err)
	require.Contains(t, feedbackMarkdown(r, 0), "[addressed]")
	r, err = store.SetStatus(r.ID, r.Threads[0].ID, "resolved")
	require.NoError(t, err)
	require.NotContains(t, feedbackMarkdown(r, 0), input.Body)
	loaded, err := (ReviewStore{Dir: store.Dir}).Read(r.ID)
	require.NoError(t, err)
	require.Equal(t, r, loaded)
}
func TestReviewAtomicValidationAndIdempotency(t *testing.T) {
	store, r := newTestReview(t)
	input := annotation(r, "one")
	r, err := store.Publish(r.ID, "agent", []AnnotationInput{input}, true)
	require.NoError(t, err)
	r, err = store.Publish(r.ID, "agent", []AnnotationInput{input}, true)
	require.NoError(t, err)
	require.Len(t, r.Threads, 1)
	input.Body = "changed"
	_, err = store.Publish(r.ID, "agent", []AnnotationInput{input}, true)
	require.ErrorContains(t, err, "different content")
	good := annotation(r, "two")
	bad := annotation(r, "three")
	bad.Start = 200
	bad.End = 200
	_, err = store.Publish(r.ID, "agent", []AnnotationInput{good, bad}, true)
	require.Error(t, err)
	loaded, err := store.Read(r.ID)
	require.NoError(t, err)
	require.Len(t, loaded.Threads, 1)
	for _, change := range []func(*AnnotationInput){func(a *AnnotationInput) { a.Snapshot = "unknown" }, func(a *AnnotationInput) { a.Side = "left" }, func(a *AnnotationInput) { a.Path = "other.go" }, func(a *AnnotationInput) { a.End = 2 }, func(a *AnnotationInput) { a.Body = " " }, func(a *AnnotationInput) { a.Kind = "bug" }} {
		in := annotation(r, "invalid")
		change(&in)
		_, err = store.Publish(r.ID, "agent", []AnnotationInput{in}, true)
		require.Error(t, err)
	}
}
func TestReviewSnapshotsPreserveOldCoordinates(t *testing.T) {
	store, r := newTestReview(t)
	input := annotation(r, "one")
	old := r.Current
	r, err := store.Publish(r.ID, "agent", []AnnotationInput{input}, true)
	require.NoError(t, err)
	r, err = store.Open(r.Repo, r.Title, r.Source, ReviewSnapshot{Diff: strings.ReplaceAll(reviewFixture, "Hello, 世界", "Greetings"), Base: "base", Head: "working-tree"})
	require.NoError(t, err)
	require.NotEqual(t, old, r.Current)
	require.Equal(t, old, r.Threads[0].Anchor.Snapshot)
	require.Len(t, r.Snapshots, 2)
	require.Contains(t, feedbackMarkdown(r, 0), "OUTDATED")
	require.Contains(t, feedbackMarkdown(r, 0), "Hello, 世界")
	// An agent may finish reviewing a captured older snapshot, without mislabelling it current.
	input.Key = "late"
	_, err = store.Publish(r.ID, "agent", []AnnotationInput{input}, true)
	require.NoError(t, err)
}
func TestReviewConcurrentWriters(t *testing.T) {
	store, r := newTestReview(t)
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := (ReviewStore{Dir: store.Dir}).Publish(r.ID, "agent", []AnnotationInput{annotation(r, fmt.Sprint(i))}, true)
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	r, err := store.Read(r.ID)
	require.NoError(t, err)
	require.Len(t, r.Threads, 20)
}
func TestReviewCorruptionAndTraversal(t *testing.T) {
	store, r := newTestReview(t)
	_, err := store.Read("../../escape")
	require.Error(t, err)
	path, _ := store.path(r.ID)
	require.NoError(t, os.WriteFile(path, []byte("broken"), 0600))
	_, err = store.Publish(r.ID, "agent", []AnnotationInput{annotation(r, "one")}, true)
	require.Error(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "broken", string(data))
}
func TestAnchorOldSideAndMissingLines(t *testing.T) {
	_, r := newTestReview(t)
	snap := r.Snapshots[r.Current]
	a, err := anchorFor(snap, "main.go", "old", 4, 4)
	require.NoError(t, err)
	require.Equal(t, `    return "Hello"`, a.Quote)
	_, err = anchorFor(snap, "main.go", "new", 0, 1)
	require.Error(t, err)
	_, err = anchorFor(snap, "main.go", "new", 1, 100)
	require.Error(t, err)
}
func TestMCPReviewExchange(t *testing.T) {
	store, r := newTestReview(t)
	ctx := context.Background()
	server := newMCPServer(store, r.Repo)
	client := mcp.NewClient(&mcp.Implementation{Name: "test-agent", Version: "1"}, nil)
	s, c := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, s, nil)
	require.NoError(t, err)
	defer ss.Close()
	cs, err := client.Connect(ctx, c, nil)
	require.NoError(t, err)
	defer cs.Close()
	listed, err := cs.ListTools(ctx, nil)
	require.NoError(t, err)
	require.Len(t, listed.Tools, 7)
	call := func(name string, input ReviewRequest) *mcp.CallToolResult {
		result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: input})
		require.NoError(t, err)
		return result
	}
	read := call("aside_read", ReviewRequest{ID: r.ID})
	require.False(t, read.IsError)
	require.Contains(t, read.Content[0].(*mcp.TextContent).Text, r.Current)
	result := call("aside_publish", ReviewRequest{ID: r.ID, Author: "test-agent", Annotations: []AnnotationInput{annotation(r, "mcp-one")}})
	require.False(t, result.IsError)
	r, err = store.Read(r.ID)
	require.NoError(t, err)
	require.Len(t, r.Threads, 1)
	denied := call("aside_status", ReviewRequest{ID: r.ID, Thread: r.Threads[0].ID, Status: "resolved"})
	require.True(t, denied.IsError)
	result = call("aside_feedback", ReviewRequest{ID: r.ID})
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].(*mcp.TextContent).Text, "Handle an empty name")
	result = call("aside_read", ReviewRequest{ID: digest("missing")})
	require.True(t, result.IsError)
}
func TestReviewCLI(t *testing.T) {
	repo := fixtureRepo(t)
	store := ReviewStore{Dir: t.TempDir()}
	var out strings.Builder
	require.NoError(t, runReviewCLI(store, []string{"open", "--repo", repo}, strings.NewReader(""), &out, &out))
	var r Review
	require.NoError(t, json.Unmarshal([]byte(out.String()), &r))
	require.NotEmpty(t, r.Current)
	req := ReviewRequest{ID: r.ID, Author: "agent", Annotations: []AnnotationInput{annotation(&r, "cli-one")}}
	data, err := json.Marshal(req)
	require.NoError(t, err)
	out.Reset()
	require.NoError(t, runReviewCLI(store, []string{"publish", "--repo", repo}, strings.NewReader(string(data)), &out, &out))
	loaded, err := store.Read(r.ID)
	require.NoError(t, err)
	require.Len(t, loaded.Threads, 1)
	other := fixtureRepo(t)
	_, err = reviewOperation(store, other, "read", ReviewRequest{ID: r.ID})
	require.ErrorContains(t, err, "different repository")
	require.NoError(t, os.WriteFile(filepath.Join(repo, "check.txt"), []byte("untracked"), 0600))
}

func TestAgentInterfacesExcludeDrafts(t *testing.T) {
	store, r := newTestReview(t)
	r, err := store.Publish(r.ID, "you", []AnnotationInput{annotation(r, "private-draft")}, false)
	require.NoError(t, err)
	for _, op := range []string{"read", "list", "feedback"} {
		result, err := reviewOperation(store, r.Repo, op, ReviewRequest{ID: r.ID})
		require.NoError(t, err)
		data, err := json.Marshal(result)
		require.NoError(t, err)
		require.NotContains(t, string(data), "Handle an empty name", op)
	}
}

func TestAgentCannotModifyDraft(t *testing.T) {
	store, r := newTestReview(t)
	r, err := store.Publish(r.ID, "you", []AnnotationInput{annotation(r, "private")}, false)
	require.NoError(t, err)
	for _, op := range []string{"reply", "status"} {
		_, err = reviewOperation(store, r.Repo, op, ReviewRequest{ID: r.ID, Thread: r.Threads[0].ID, Body: "response", Status: "addressed"})
		require.ErrorContains(t, err, "submitted")
	}
}
func TestReviewRejectsMalformedThread(t *testing.T) {
	store, r := newTestReview(t)
	r.Threads = []ReviewThread{{ID: "x", Anchor: ReviewAnchor{Snapshot: r.Current}}}
	data, err := json.Marshal(r)
	require.NoError(t, err)
	path, _ := store.path(r.ID)
	require.NoError(t, os.WriteFile(path, data, 0600))
	_, err = store.Read(r.ID)
	require.Error(t, err)
}
