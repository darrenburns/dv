package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDiffViewState_ClampScrollXAndY(t *testing.T) {
	rendered := buildTestRenderedFile(40, 80)
	state := NewDiffViewState(rendered)
	gutterWidth := renderedGutterWidth(rendered, false)

	state.SetViewport(30, 10, gutterWidth)
	state.ScrollY.Set(999)
	state.ScrollX.Set(999)

	state.Clamp(gutterWidth)

	require.Equal(t, 30, state.MaxScrollY())
	require.Equal(t, 58, state.MaxScrollX(gutterWidth))
	require.Equal(t, 30, state.ScrollY.Peek())
	require.Equal(t, 58, state.ScrollX.Peek())
}

func TestDiffViewState_PageAndHalfPageSteps(t *testing.T) {
	rendered := buildTestRenderedFile(100, 20)
	state := NewDiffViewState(rendered)
	gutterWidth := renderedGutterWidth(rendered, false)
	state.SetViewport(40, 12, gutterWidth)

	state.PageDown(gutterWidth)
	require.Equal(t, 11, state.ScrollY.Peek())

	state.HalfPageDown(gutterWidth)
	require.Equal(t, 17, state.ScrollY.Peek())

	state.PageUp(gutterWidth)
	require.Equal(t, 6, state.ScrollY.Peek())

	state.HalfPageUp(gutterWidth)
	require.Equal(t, 0, state.ScrollY.Peek())
}

func TestDiffViewState_GoTopAndGoBottom(t *testing.T) {
	rendered := buildTestRenderedFile(25, 10)
	state := NewDiffViewState(rendered)
	gutterWidth := renderedGutterWidth(rendered, false)
	state.SetViewport(20, 5, gutterWidth)

	state.GoBottom(gutterWidth)
	require.Equal(t, 20, state.ScrollY.Peek())

	state.GoTop(gutterWidth)
	require.Equal(t, 0, state.ScrollY.Peek())
}

func TestDiffViewState_SetRenderedPairResetsScrollAndStoresBothModels(t *testing.T) {
	initial := buildTestRenderedFile(10, 40)
	state := NewDiffViewState(initial)
	state.ScrollY.Set(4)
	state.ScrollX.Set(7)
	state.SetSideBySideSplitRatio(0.73)
	state.StartSelection(DiffSelectionTrackUnified, DiffSelectionPoint{Row: 0, Grapheme: 1, Lane: DiffSelectionLaneUnified})
	state.UpdateSelection(DiffSelectionPoint{Row: 0, Grapheme: 3, Lane: DiffSelectionLaneUnified})

	nextRendered := buildTestRenderedFile(20, 90)
	nextSide := &SideBySideRenderedFile{
		Title:                "test",
		Rows:                 []SideBySideRenderedRow{{Shared: &RenderedDiffLine{Kind: RenderedLineMeta, Segments: []RenderedSegment{{Text: "line", Role: TokenRoleDiffMeta}}, ContentWidth: 4}}},
		LeftNumWidth:         1,
		RightNumWidth:        1,
		LeftMaxContentWidth:  4,
		RightMaxContentWidth: 4,
	}

	state.SetRenderedPair(nextRendered, nextSide)

	require.Equal(t, 0, state.ScrollY.Peek())
	require.Equal(t, 0, state.ScrollX.Peek())
	require.Equal(t, 0.73, state.SideBySideSplitRatio())
	require.False(t, state.HasSelection())
	require.Same(t, nextRendered, state.Rendered.Peek())
	require.Same(t, nextSide, state.SideBySide.Peek())
}

func TestDiffViewState_SelectedTextUnified(t *testing.T) {
	rendered := &RenderedFile{
		Title: "test",
		Lines: []RenderedDiffLine{
			newRenderedLine(RenderedLineContext, 1, 1, " ", []RenderedSegment{{Text: "hello world", Role: TokenRoleSyntaxPlain}}),
			newRenderedLine(RenderedLineContext, 2, 2, " ", []RenderedSegment{{Text: "goodbye", Role: TokenRoleSyntaxPlain}}),
		},
		OldNumWidth:     1,
		NewNumWidth:     1,
		MaxContentWidth: 11,
	}
	state := NewDiffViewState(rendered)
	state.StartSelection(DiffSelectionTrackUnified, DiffSelectionPoint{Row: 0, Grapheme: 6, Lane: DiffSelectionLaneUnified})
	state.UpdateSelection(DiffSelectionPoint{Row: 1, Grapheme: 4, Lane: DiffSelectionLaneUnified})

	require.Equal(t, "world\ngood", state.SelectedText())

	start, end, ok := state.SelectionRangeForUnifiedLine(0)
	require.True(t, ok)
	require.Equal(t, 6, start)
	require.Equal(t, -1, end)

	start, end, ok = state.SelectionRangeForUnifiedLine(1)
	require.True(t, ok)
	require.Equal(t, 0, start)
	require.Equal(t, 4, end)
}

func TestDiffViewState_SelectedTextSideBySideLeftUsesSharedRows(t *testing.T) {
	rendered := buildTestRenderedFile(2, 12)
	side := &SideBySideRenderedFile{
		Title: "side",
		Rows: []SideBySideRenderedRow{
			{Shared: &RenderedDiffLine{Kind: RenderedLineHunkHeader, Segments: []RenderedSegment{{Text: "@@ -1 +1 @@", Role: TokenRoleDiffHunkHeader}}, ContentWidth: 10}},
			{
				Left:  &RenderedSideCell{Kind: RenderedLineContext, LineNumber: 1, Prefix: " ", Segments: []RenderedSegment{{Text: "left-one", Role: TokenRoleSyntaxPlain}}, ContentWidth: 8},
				Right: &RenderedSideCell{Kind: RenderedLineContext, LineNumber: 1, Prefix: " ", Segments: []RenderedSegment{{Text: "right-one", Role: TokenRoleSyntaxPlain}}, ContentWidth: 9},
			},
			{
				Left:  nil,
				Right: &RenderedSideCell{Kind: RenderedLineAdd, LineNumber: 2, Prefix: "+", Segments: []RenderedSegment{{Text: "only-right", Role: TokenRoleSyntaxPlain}}, ContentWidth: 10},
			},
		},
		LeftNumWidth:         1,
		RightNumWidth:        1,
		LeftMaxContentWidth:  8,
		RightMaxContentWidth: 10,
	}
	state := NewDiffViewState(rendered)
	state.SetRenderedPair(rendered, side)
	state.StartSelection(DiffSelectionTrackLeft, DiffSelectionPoint{Row: 0, Grapheme: 3, Lane: DiffSelectionLaneShared})
	state.UpdateSelection(DiffSelectionPoint{Row: 2, Grapheme: 0, Lane: DiffSelectionLaneLeft})

	require.Equal(t, "-1 +1 @@\nleft-one\n", state.SelectedText())
}

func TestDiffViewState_SideBySideSplitRatioClampsToRange(t *testing.T) {
	state := NewDiffViewState(buildTestRenderedFile(4, 10))
	require.Equal(t, 0.5, state.SideBySideSplitRatio())

	state.SetSideBySideSplitRatio(-1)
	require.Equal(t, 0.0, state.SideBySideSplitRatio())

	state.SetSideBySideSplitRatio(2)
	require.Equal(t, 1.0, state.SideBySideSplitRatio())
}

func TestDiffViewState_SideDividerOverlayVisibleForOneSecondAfterResize(t *testing.T) {
	state := NewDiffViewState(buildTestRenderedFile(4, 10))
	base := time.Unix(10, 0)
	state.sideDividerLastResize.Set(base.UnixNano())

	require.True(t, state.sideDividerOverlayVisibleAt(base.Add(999*time.Millisecond)))
	require.False(t, state.sideDividerOverlayVisibleAt(base.Add(1*time.Second)))
}

func TestDiffViewState_SideDividerOverlayVisibleWhileDragging(t *testing.T) {
	state := NewDiffViewState(buildTestRenderedFile(4, 10))
	state.StartSideDividerDrag(4, 4)

	require.True(t, state.sideDividerOverlayVisibleAt(time.Unix(0, 0).Add(24*time.Hour)))

	state.StopSideDividerDrag()
	require.False(t, state.sideDividerOverlayVisibleAt(time.Unix(0, 0).Add(24*time.Hour)))
}

func buildTestRenderedFile(lineCount int, contentWidth int) *RenderedFile {
	lines := make([]RenderedDiffLine, 0, lineCount)
	for i := 0; i < lineCount; i++ {
		lines = append(lines, RenderedDiffLine{
			Kind:         RenderedLineContext,
			OldLine:      i + 1,
			NewLine:      i + 1,
			Prefix:       " ",
			Segments:     []RenderedSegment{{Text: "x", Role: TokenRoleSyntaxPlain}},
			ContentWidth: contentWidth,
		})
	}
	return &RenderedFile{
		Title:           "test",
		Lines:           lines,
		OldNumWidth:     2,
		NewNumWidth:     2,
		MaxContentWidth: contentWidth,
	}
}
