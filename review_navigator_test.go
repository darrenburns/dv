package main

import (
	"fmt"
	"strings"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
	t "github.com/darrenburns/terma"
	"github.com/stretchr/testify/require"
)

func navigatorFixture(tt *testing.T, count int) *Aside {
	tt.Helper()
	a := appFixture(tt)
	inputs := make([]AnnotationInput, count)
	for i := range inputs {
		inputs[i] = annotation(a.review, fmt.Sprint(i))
		inputs[i].Body = fmt.Sprintf("Check %d: preserve the greeting and cover empty names.", i+1)
	}
	r, err := a.store.Publish(a.review.ID, "review-agent", inputs, true)
	require.NoError(tt, err)
	a.apply(r)
	return a
}

func TestReviewNavigatorFollowsFileAndActiveThread(tt *testing.T) {
	a := navigatorFixture(tt, 3)
	require.Len(tt, a.reviewList.GetItems(), 3)
	a.focusComment(a.review.Threads[2].ID)
	require.Equal(tt, 2, a.reviewList.CursorIndex.Peek())
	a.nextThread(1)
	require.Equal(tt, 0, a.reviewList.CursorIndex.Peek())
	a.toggleResolved()
	require.Equal(tt, "resolved", a.reviewList.GetItems()[0].Status)
	a.toggleAllFiles()
	a.selectFile("README.md")
	require.Empty(tt, a.reviewList.GetItems())
	require.Empty(tt, a.activeThread)
	b := uv.NewBuffer(100, 30)
	renderer := t.NewRenderer(b, 100, 30, t.NewFocusManager(), t.NewAnySignal[t.Focusable](nil), t.NewAnySignal[t.Widget](nil))
	renderer.Render(a)
	require.Nil(tt, renderer.WidgetByID(reviewListID))
	require.Nil(tt, renderer.WidgetByID(reviewSplitID), "files without comments reclaim the whole sidebar")
	a.selectFile("main.go")
	a.focusReviewNavigator()
	require.Equal(tt, a.review.Threads[0].ID, a.activeThread)
	a.filter.SetText("no-such-file")
	a.rebuildFiles()
	require.Empty(tt, a.reviewList.GetItems())
	require.Empty(tt, a.activeThread)
}

func TestReviewNavigatorRetainsIdentityAcrossIncomingUpdates(tt *testing.T) {
	a := navigatorFixture(tt, 3)
	selected := a.review.Threads[1].ID
	a.focusComment(selected)
	r, err := a.store.Update(a.review.ID, nil, func(r *Review) error { r.Threads = r.Threads[1:]; return nil })
	require.NoError(tt, err)
	a.receiveReview(r)
	require.Equal(tt, selected, a.activeThread)
	require.Equal(tt, 0, a.reviewList.CursorIndex.Peek())
	a.beginReply()
	a.editor.SetText("Unsent reply")
	r, err = a.store.Publish(r.ID, "agent", []AnnotationInput{annotation(r, "incoming")}, true)
	require.NoError(tt, err)
	a.receiveReview(r)
	a.selectReviewEntry(r.Threads[1])
	require.Len(tt, a.reviewList.GetItems(), 2, "incoming changes wait until composition finishes")
	require.Equal(tt, selected, a.activeThread)
	require.Equal(tt, "Unsent reply", a.editor.GetText())
	a.cancelComment()
	a.receiveReview(r)
	require.Len(tt, a.reviewList.GetItems(), 3)
	require.Equal(tt, selected, a.activeThread)
}

func TestReviewNavigatorUsesListKeysAndIndependentScroll(tt *testing.T) {
	a := navigatorFixture(tt, 20)
	a.focusReviewNavigator()
	b := uv.NewBuffer(110, 30)
	renderer := t.NewRenderer(b, 110, 30, t.NewFocusManager(), t.NewAnySignal[t.Focusable](nil), t.NewAnySignal[t.Widget](nil))
	renderer.Render(a)
	entry := renderer.WidgetByID(reviewListID)
	require.NotNil(tt, entry)
	list, ok := entry.EventWidget.(t.List[ReviewThread])
	require.True(tt, ok, "navigation is supplied by Terma's List")
	for _, key := range list.Keybinds() {
		if key.Key == "end" {
			key.Action()
			break
		}
	}
	renderer.Render(a)
	require.Equal(tt, a.review.Threads[19].ID, a.activeThread)
	require.Positive(tt, a.reviewListScroll.Offset.Peek())
	require.Zero(tt, a.treeScroll.Offset.Peek(), "review scrolling must not move the file tree")
	require.Contains(tt, reviewBufferText(b, 110, 30), "Check 20:")
	for _, key := range list.Keybinds() {
		if key.Key == "up" {
			key.Action()
			break
		}
	}
	require.Equal(tt, a.review.Threads[18].ID, a.activeThread)
}

func TestReviewNavigatorMouseSelectsThread(tt *testing.T) {
	a := navigatorFixture(tt, 2)
	b := uv.NewBuffer(120, 40)
	renderer := t.NewRenderer(b, 120, 40, t.NewFocusManager(), t.NewAnySignal[t.Focusable](nil), t.NewAnySignal[t.Widget](nil))
	renderer.Render(a)
	row := renderer.WidgetByID("review-entry-" + a.review.Threads[1].ID)
	require.NotNil(tt, row)
	for y := row.Bounds.Y; y < row.Bounds.Y+row.Bounds.Height; y++ {
		for x := row.Bounds.X; x < row.Bounds.X+row.Bounds.Width; x++ {
			a.focusComment(a.review.Threads[0].ID)
			renderer.Render(a)
			entry := renderer.WidgetAt(x, y)
			require.NotNil(tt, entry)
			clickable, ok := entry.EventWidget.(t.Clickable)
			require.True(tt, ok, "row cell (%d, %d) must be clickable", x, y)
			clickable.OnClick(t.MouseEvent{Button: uv.MouseLeft, X: x, Y: y})
			require.Equal(tt, a.review.Threads[1].ID, a.activeThread, "click at (%d, %d) hit %T", x, y, entry.EventWidget)
			require.Equal(tt, 1, a.reviewList.CursorIndex.Peek())
		}
	}
}

func TestReviewNavigatorSummaryHandlesBlankLinesAndControls(tt *testing.T) {
	thread := ReviewThread{Messages: []ReviewMessage{{Body: "\n\t\nPreserve\t greeting 世界\x1b.\nLong explanation follows."}}}
	require.Equal(tt, "Preserve greeting 世界�.", reviewEntrySummary(thread))
	thread.Messages[0].Body = strings.Repeat("Long summary ", 100)
	require.LessOrEqual(tt, len([]rune(reviewEntrySummary(thread))), 160)
	require.Equal(tt, "No comment text", reviewEntrySummary(ReviewThread{}))
}

func TestSnapshotAsideReviewNavigator(tt *testing.T) {
	t.SetTheme(t.ThemeNameObsidianTide)
	a := navigatorFixture(tt, 3)
	r, err := a.store.Update(a.review.ID, nil, func(r *Review) error {
		r.Threads[0].Messages[0].Body = "Handle empty input before formatting."
		r.Threads[1].Kind = "explanation"
		r.Threads[1].Messages[0].Body = "Preserve the greeting prefix."
		r.Threads[1].Status = "resolved"
		r.Threads[2].Submitted = false
		return nil
	})
	require.NoError(tt, err)
	a.apply(r)
	a.focusComment(r.Threads[1].ID)
	a.reviewSplit.SetPosition(.35)
	t.AssertSnapshot(tt, a, 120, 40, "Review navigator shows current-file summaries, selected range, resolved state and local draft")
}

func TestSnapshotAsideReviewNavigatorNarrow(tt *testing.T) {
	t.SetTheme(t.ThemeNameObsidianTide)
	a := navigatorFixture(tt, 2)
	a.focusComment(a.review.Threads[1].ID)
	a.reviewSplit.SetPosition(.4)
	t.AssertSnapshot(tt, a, 65, 26, "Narrow navigator wraps metadata, bounds summary height and keeps its active entry visible")
}
