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
	require.Same(t, nextRendered, state.Rendered.Peek())
	require.Same(t, nextSide, state.SideBySide.Peek())
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
