package main

import (
	"strings"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
	t "github.com/darrenburns/terma"
	"github.com/stretchr/testify/require"
)

func TestCommentRangeDecorationsPreserveSourceAndSelection(tt *testing.T) {
	for _, side := range []string{"old", "new"} {
		tt.Run(side, func(tt *testing.T) {
			a := appFixture(tt)
			a.mode.Set(DiffLayoutSideBySide)
			a.renderActive(false)
			theme, _ := t.GetTheme(t.ThemeNameObsidianTide)
			view := DiffView{State: a.view, LayoutMode: DiffLayoutSideBySide, HardWrap: true, Palette: NewThemePalette(theme), commentTheme: theme}
			const width, height = 26, 80 // Forces wrapping on both sides.
			render := func() *uv.Buffer {
				b := uv.NewBuffer(width, height)
				view.Render(t.NewRenderContext(b, width, height, nil, nil, t.BuildContext{}, nil))
				return b
			}
			before := render()
			view.CommentRanges = []DiffCommentRange{{Side: side, Start: 4, End: 4, Focused: true}}
			after := render()
			split := a.view.SideBySide.Peek()
			panes := sideBySidePaneLayout(width, split, false, .5)
			x, n := panes.LeftPaneX, split.LeftNumWidth
			if side == "new" {
				x, n = panes.RightPaneX, split.RightNumWidth
			}
			rails := 0
			for y := 0; y < height; y++ {
				for col := 0; col < width; col++ {
					if col >= x && col <= x+n {
						continue
					}
					require.Equal(tt, before.CellAt(col, y).Content, after.CellAt(col, y).Content, "code or opposite pane changed at %d,%d", col, y)
				}
				mark := after.CellAt(x+n, y).Content
				if mark == "╭" || mark == "│" {
					rails++
				}
			}
			require.Greater(tt, rails, 1, "range rail must continue through wrapped rows")
			for i, row := range split.Rows {
				cell, track, lane := row.Left, DiffSelectionTrackLeft, DiffSelectionLaneLeft
				if side == "new" {
					cell, track, lane = row.Right, DiffSelectionTrackRight, DiffSelectionLaneRight
				}
				if cell != nil && cell.LineNumber == 4 {
					a.view.StartSelection(track, DiffSelectionPoint{Row: i, Lane: lane})
					a.view.StopSelectionDrag()
					anchor, err := a.selectionAnchor()
					require.NoError(tt, err)
					require.Equal(tt, side, anchor.Side)
					require.Equal(tt, 4, anchor.Start)
					require.NotContains(tt, anchor.Quote, "╭")
					break
				}
			}
		})
	}
}

func TestCommentFocusDistinguishesOverlappingRanges(tt *testing.T) {
	a := appFixture(tt)
	first, second := annotation(a.review, "first"), annotation(a.review, "second")
	second.Start, second.End = 5, 6
	r, err := a.store.Publish(a.review.ID, "agent", []AnnotationInput{first, second}, true)
	require.NoError(tt, err)
	a.apply(r)
	a.focusComment(r.Threads[1].ID)
	view := DiffView{CommentRanges: a.commentRanges()}
	for _, tc := range []struct {
		line    int
		focused bool
	}{{4, false}, {5, true}, {6, true}} {
		mark, ok := view.commentRange("new", tc.line)
		require.True(tt, ok)
		require.Equal(tt, tc.focused, mark.Focused)
	}
	_, ok := view.commentRange("old", 5)
	require.False(tt, ok)
	_, ok = view.commentRange("new", 7)
	require.False(tt, ok)
	// Reply editing must not be switched to another conversation by a click.
	a.beginReply()
	a.focusComment(r.Threads[0].ID)
	require.Equal(tt, r.Threads[1].ID, a.activeThread)
}

func TestCommentAnchorsNeverConnectOutdatedOrStackedCards(tt *testing.T) {
	a := appFixture(tt)
	r, err := a.store.Publish(a.review.ID, "agent", []AnnotationInput{annotation(a.review, "first"), annotation(a.review, "same-end")}, true)
	require.NoError(tt, err)
	a.apply(r)
	for _, mode := range []DiffLayoutMode{DiffLayoutUnified, DiffLayoutSideBySide} {
		a.mode.Set(mode)
		a.renderActive(false)
		connected := 0
		for i, section := range a.sections {
			if section.Thread != nil && i > 0 && a.sectionEndsAt(a.sections[i-1], section.Thread.Anchor) {
				connected++
			}
		}
		require.Equal(tt, 1, connected, "only the first card touches the source")
	}
	r, err = a.store.Open(r.Repo, r.Title, r.Source, ReviewSnapshot{Diff: strings.ReplaceAll(reviewFixture, "Hello, 世界", "Greetings"), Base: "changed", Head: "worktree"})
	require.NoError(tt, err)
	a.apply(r)
	require.Empty(tt, a.commentRanges(), "old snapshot must not highlight new code at reused line numbers")
	for _, section := range a.sections {
		require.False(tt, a.sectionEndsAt(section, r.Threads[0].Anchor))
	}
}

func TestCommentAnchorLabelMouseFocus(tt *testing.T) {
	a := appFixture(tt)
	r, err := a.store.Publish(a.review.ID, "agent", []AnnotationInput{annotation(a.review, "mouse")}, true)
	require.NoError(tt, err)
	a.apply(r)
	renderer := t.NewRenderer(uv.NewBuffer(120, 40), 120, 40, t.NewFocusManager(), t.NewAnySignal[t.Focusable](nil), t.NewAnySignal[t.Widget](nil))
	renderer.Render(a)
	for y := 0; y < 40; y++ {
		for x := 0; x < 120; x++ {
			entry := renderer.WidgetAt(x, y)
			if entry != nil && entry.ID == "anchor-"+r.Threads[0].ID {
				entry.EventWidget.(t.Clickable).OnClick(t.MouseEvent{Button: uv.MouseLeft})
				require.Equal(tt, r.Threads[0].ID, a.activeThread)
				renderer.Render(a)
				return
			}
		}
	}
	tt.Fatal("anchor label has no mouse target")
}

func TestSnapshotAsideSplitOldAnchor(tt *testing.T) {
	t.SetTheme(t.ThemeNameObsidianTide)
	a := appFixture(tt)
	input := annotation(a.review, "old")
	input.Side = "old"
	input.Start = 3
	input.End = 4
	r, err := a.store.Publish(a.review.ID, "agent", []AnnotationInput{input}, true)
	require.NoError(tt, err)
	a.apply(r)
	a.mode.Set(DiffLayoutSideBySide)
	a.renderActive(false)
	a.focusComment(r.Threads[0].ID)
	t.AssertSnapshot(tt, a, 110, 32, "Old-side range rail connects to the full-width review card")
}
func TestSnapshotAsideSplitAnchorReplyNarrow(tt *testing.T) {
	t.SetTheme(t.ThemeNameObsidianTide)
	a := appFixture(tt)
	r, err := a.store.Publish(a.review.ID, "agent", []AnnotationInput{annotation(a.review, "new")}, true)
	require.NoError(tt, err)
	a.apply(r)
	a.mode.Set(DiffLayoutSideBySide)
	a.hideSigns.Set(true)
	a.diffRatio.Set(.35)
	a.renderActive(false)
	a.focusComment(r.Threads[0].ID)
	a.beginReply()
	a.editor.SetText("Please cover empty names too.")
	t.AssertSnapshot(tt, a, 80, 40, "Wrapped new-side anchor and nested reply follow a resized divider with change signs hidden")
}
func TestSnapshotAsideOverlappingAnchors(tt *testing.T) {
	t.SetTheme(t.ThemeNameObsidianTide)
	a := appFixture(tt)
	one, two := annotation(a.review, "one"), annotation(a.review, "two")
	two.Start = 5
	two.End = 6
	r, err := a.store.Publish(a.review.ID, "agent", []AnnotationInput{one, two}, true)
	require.NoError(tt, err)
	a.apply(r)
	a.focusComment(r.Threads[1].ID)
	t.AssertSnapshot(tt, a, 110, 42, "The selected range remains distinct where conversations overlap")
}

func TestNewCommentHighlightsOnlyItsPendingRange(tt *testing.T) {
	a := appFixture(tt)
	r, err := a.store.Publish(a.review.ID, "agent", []AnnotationInput{annotation(a.review, "existing")}, true)
	require.NoError(tt, err)
	a.apply(r)
	a.focusComment(r.Threads[0].ID)
	selectNewLine(a)
	a.beginComment()
	view := DiffView{CommentRanges: a.commentRanges()}
	mark, ok := view.commentRange("new", 4)
	require.True(tt, ok)
	require.False(tt, mark.Focused)
	mark, ok = view.commentRange("new", 5)
	require.True(tt, ok)
	require.True(tt, mark.Focused)
	a.cancelComment()
	a.active = "README.md"
	require.Empty(tt, a.commentRanges(), "a different file must never inherit range marks")
}
