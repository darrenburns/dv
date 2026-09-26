package main

import (
	"strings"
	"testing"

	t "github.com/darrenburns/terma"
	"github.com/stretchr/testify/require"
)

func appFixture(tt *testing.T) *Aside {
	tt.Helper()
	store, r := newTestReview(tt)
	r.Title = "Greeting review"
	app := NewAside(store, r, DefaultAsideInitialState())
	return app
}
func selectNewLine(app *Aside) {
	r := app.view.Rendered.Peek()
	for i, line := range r.Lines {
		if line.NewLine == 5 {
			app.view.StartSelection(DiffSelectionTrackUnified, DiffSelectionPoint{Row: i, Lane: DiffSelectionLaneUnified})
			app.view.StopSelectionDrag()
			return
		}
	}
}
func TestAsideCommentSubmissionAndReply(tt *testing.T) {
	a := appFixture(tt)
	selectNewLine(a)
	a.beginComment()
	require.True(tt, a.editing.Peek())
	a.editor.SetText("Please handle empty input.")
	a.saveComment()
	require.False(tt, a.editing.Peek())
	require.Len(tt, a.review.Threads, 1)
	require.False(tt, a.review.Threads[0].Submitted)
	var clipboard string
	a.copyFeedback = func(s string) error { clipboard = s; return nil }
	a.exportFeedback()
	require.Contains(tt, clipboard, "Please handle empty input.")
	require.True(tt, a.review.Threads[0].Submitted)
	a.nextThread(1)
	a.beginReply()
	a.editor.SetText("Verified the edge case.")
	a.saveComment()
	require.Len(tt, a.review.Threads[0].Messages, 2)
	a.toggleResolved()
	require.Equal(tt, "resolved", a.review.Threads[0].Status)
	a.toggleResolved()
	require.Equal(tt, "open", a.review.Threads[0].Status)
}
func TestAsideSelectionCoordinates(tt *testing.T) {
	a := appFixture(tt)
	selectNewLine(a)
	anchor, err := a.selectionAnchor()
	require.NoError(tt, err)
	require.Equal(tt, 5, anchor.Start)
	require.Equal(tt, "new", anchor.Side)
	r := a.view.Rendered.Peek()
	oldRow, newRow := 0, 0
	for i, line := range r.Lines {
		if line.OldLine == 4 && line.Kind == RenderedLineRemove {
			oldRow = i
		}
		if line.NewLine == 5 {
			newRow = i
		}
	}
	a.view.StartSelection(DiffSelectionTrackUnified, DiffSelectionPoint{Row: oldRow, Lane: DiffSelectionLaneUnified})
	a.view.UpdateSelection(DiffSelectionPoint{Row: newRow, Grapheme: 5, Lane: DiffSelectionLaneUnified})
	_, err = a.selectionAnchor()
	require.ErrorContains(tt, err, "one diff side")
	a.mode.Set(DiffLayoutSideBySide)
	a.renderActive(false)
	rows := a.view.SideBySide.Peek().Rows
	for i, row := range rows {
		if row.Left != nil && row.Left.LineNumber == 4 {
			a.view.StartSelection(DiffSelectionTrackLeft, DiffSelectionPoint{Row: i, Lane: DiffSelectionLaneLeft})
			anchor, err = a.selectionAnchor()
			require.NoError(tt, err)
			require.Equal(tt, "old", anchor.Side)
			require.Equal(tt, 4, anchor.Start)
			break
		}
	}
}
func TestAsideFilterKeepsNavigationInVisibleFiles(tt *testing.T) {
	a := appFixture(tt)
	a.allFiles.Set(true)
	a.filter.SetText("readme")
	a.rebuildFiles()
	require.Equal(tt, []string{"README.md"}, a.paths)
	require.Equal(tt, "README.md", a.active)
	a.nextFile(1)
	require.Equal(tt, "README.md", a.active)
	a.filter.SetText("missing")
	a.rebuildFiles()
	require.Empty(tt, a.paths)
	require.Empty(tt, a.active)
	a.nextFile(1)
}
func TestAsideNoRepositoryMutationActions(tt *testing.T) {
	a := appFixture(tt)
	for _, key := range a.Keybinds() {
		name := strings.ToLower(key.Name)
		for _, word := range []string{"commit", "push", "stage", "unstage", "pull request"} {
			require.NotContains(tt, name, word)
		}
	}
}
func TestAsideInlineBlocksPreserveCodeCoordinates(tt *testing.T) {
	a := appFixture(tt)
	r, err := a.store.Publish(a.review.ID, "agent", []AnnotationInput{annotation(a.review, "one")}, true)
	require.NoError(tt, err)
	a.apply(r)
	selectNewLine(a)
	anchor, err := a.selectionAnchor()
	require.NoError(tt, err)
	require.Equal(tt, 5, anchor.Start)
	require.Contains(tt, anchor.Quote, "世界")
	var cards []ReviewThread
	for _, section := range a.sections {
		if section.Thread != nil {
			cards = append(cards, *section.Thread)
		}
	}
	require.Len(tt, cards, 1)
	require.Contains(tt, cards[0].Messages[0].Body, "Handle an empty name")
	for _, line := range a.view.Rendered.Peek().Lines {
		require.NotContains(tt, lineText(line), "Handle an empty name")
	}

}
func TestSnapshotAsideUnified(tt *testing.T) {
	t.SetTheme(t.ThemeNameObsidianTide)
	a := appFixture(tt)
	t.AssertSnapshot(tt, a, 110, 26, "Aside file browser and unified diff, with no repository mutation controls")
}
func TestSnapshotAsideSplitFinding(tt *testing.T) {
	t.SetTheme(t.ThemeNameObsidianTide)
	a := appFixture(tt)
	r, err := a.store.Publish(a.review.ID, "agent", []AnnotationInput{annotation(a.review, "one")}, true)
	require.NoError(tt, err)
	a.apply(r)
	a.mode.Set(DiffLayoutSideBySide)
	a.renderActive(false)
	t.AssertSnapshot(tt, a, 120, 30, "Agent finding below changed code in split view")
}
func TestSnapshotAsideComposer(tt *testing.T) {
	t.SetTheme(t.ThemeNameObsidianTide)
	a := appFixture(tt)
	selectNewLine(a)
	a.beginComment()
	a.editor.SetText("Please preserve the Unicode greeting.")
	t.AssertSnapshot(tt, a, 100, 26, "Feedback composer shows the selected file, side, and range")
}
func TestSnapshotAsideOutdatedNarrow(tt *testing.T) {
	t.SetTheme(t.ThemeNameObsidianTide)
	a := appFixture(tt)
	r, err := a.store.Publish(a.review.ID, "agent", []AnnotationInput{annotation(a.review, "one")}, true)
	require.NoError(tt, err)
	r, err = a.store.Open(r.Repo, r.Title, r.Source, ReviewSnapshot{Diff: "", Base: "new", Head: "working-tree"})
	require.NoError(tt, err)
	a.apply(r)
	t.AssertSnapshot(tt, a, 65, 30, "Outdated feedback remains reachable after the file disappears; original quote wraps")
}
func TestSnapshotAsideEmpty(tt *testing.T) {
	t.SetTheme(t.ThemeNameObsidianTide)
	a := appFixture(tt)
	r, err := a.store.Open(a.review.Repo, a.review.Title, a.review.Source, ReviewSnapshot{Diff: "", Base: "base", Head: "working-tree"})
	require.NoError(tt, err)
	a.apply(r)
	t.AssertSnapshot(tt, a, 90, 22, "Empty review has a clear state and remains usable")
}

func TestAsideIncomingReviewPreservesSelectionAndDraft(tt *testing.T) {
	a := appFixture(tt)
	selectNewLine(a)
	selected, err := a.selectionAnchor()
	require.NoError(tt, err)
	newer, err := a.store.Publish(a.review.ID, "agent", []AnnotationInput{annotation(a.review, "incoming")}, true)
	require.NoError(tt, err)
	a.receiveReview(newer)
	require.Empty(tt, a.review.Threads)
	after, err := a.selectionAnchor()
	require.NoError(tt, err)
	require.Equal(tt, selected, after)
	a.beginComment()
	a.editor.SetText("Still composing")
	a.receiveReview(newer)
	require.Empty(tt, a.review.Threads)
	a.saveComment()
	require.Len(tt, a.review.Threads, 2)
}
func TestAsideCommentNavigationRevealsFilteredFile(tt *testing.T) {
	a := appFixture(tt)
	r, err := a.store.Publish(a.review.ID, "agent", []AnnotationInput{annotation(a.review, "filtered")}, true)
	require.NoError(tt, err)
	a.apply(r)
	a.filter.SetText("missing")
	a.rebuildFiles()
	a.nextThread(1)
	require.Equal(tt, "main.go", a.active)
	require.Contains(tt, a.paths, a.active)
	require.Empty(tt, a.filter.GetText())
}
