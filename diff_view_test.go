package main

import (
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
	t "github.com/darrenburns/terma"
	"github.com/stretchr/testify/require"
)

func TestLineNumberRolesForLine(tt *testing.T) {
	oldRole, newRole := lineNumberRolesForLine(RenderedLineContext)
	require.Equal(tt, TokenRoleOldLineNumber, oldRole)
	require.Equal(tt, TokenRoleNewLineNumber, newRole)

	oldRole, newRole = lineNumberRolesForLine(RenderedLineAdd)
	require.Equal(tt, TokenRoleLineNumberAdd, oldRole)
	require.Equal(tt, TokenRoleLineNumberAdd, newRole)

	oldRole, newRole = lineNumberRolesForLine(RenderedLineRemove)
	require.Equal(tt, TokenRoleLineNumberRemove, oldRole)
	require.Equal(tt, TokenRoleLineNumberRemove, newRole)
}

func TestHorizontalScrollXForLine(tt *testing.T) {
	require.Equal(tt, 0, horizontalScrollXForLine(RenderedLineHunkHeader, 17))
	require.Equal(tt, 17, horizontalScrollXForLine(RenderedLineContext, 17))
	require.Equal(tt, 17, horizontalScrollXForLine(RenderedLineAdd, 17))
}

func TestDisplayLinePrefix(tt *testing.T) {
	add := RenderedDiffLine{Kind: RenderedLineAdd, Prefix: "+"}
	require.Equal(tt, "+", displayLinePrefix(add, false))
	require.Equal(tt, " ", displayLinePrefix(add, true))

	remove := RenderedDiffLine{Kind: RenderedLineRemove, Prefix: "-"}
	require.Equal(tt, "-", displayLinePrefix(remove, false))
	require.Equal(tt, " ", displayLinePrefix(remove, true))

	context := RenderedDiffLine{Kind: RenderedLineContext, Prefix: " "}
	require.Equal(tt, " ", displayLinePrefix(context, false))
	require.Equal(tt, " ", displayLinePrefix(context, true))

	empty := RenderedDiffLine{Kind: RenderedLineMeta, Prefix: ""}
	require.Equal(tt, " ", displayLinePrefix(empty, false))
}

func TestRenderedGutterWidth(tt *testing.T) {
	rendered := &RenderedFile{
		OldNumWidth: 3,
		NewNumWidth: 4,
	}

	require.Equal(tt, 11, renderedGutterWidth(rendered, false))
	require.Equal(tt, 9, renderedGutterWidth(rendered, true))
	require.Equal(tt, 6, renderedGutterWidth(nil, false))
	require.Equal(tt, 4, renderedGutterWidth(nil, true))
}

func TestWrappedLineRowCount(tt *testing.T) {
	line := RenderedDiffLine{
		Segments:     []RenderedSegment{{Text: "abcdefghi", Role: TokenRoleSyntaxPlain}},
		ContentWidth: 9,
	}
	require.Equal(tt, 3, wrappedLineRowCount(line, 4))
	require.Equal(tt, 1, wrappedLineRowCount(line, 20))

	empty := RenderedDiffLine{}
	require.Equal(tt, 1, wrappedLineRowCount(empty, 4))
}

func TestWrappedLineAtRow(tt *testing.T) {
	lines := []RenderedDiffLine{
		{
			Kind:         RenderedLineContext,
			OldLine:      1,
			NewLine:      1,
			Prefix:       " ",
			Segments:     []RenderedSegment{{Text: "abc", Role: TokenRoleSyntaxPlain}},
			ContentWidth: 3,
		},
		{
			Kind:         RenderedLineAdd,
			OldLine:      2,
			NewLine:      2,
			Prefix:       "+",
			Segments:     []RenderedSegment{{Text: "abcdefgh", Role: TokenRoleSyntaxPlain}},
			ContentWidth: 8,
		},
	}

	line, wrapRow, ok := wrappedLineAtRow(lines, 4, 0)
	require.True(tt, ok)
	require.Equal(tt, 1, line.NewLine)
	require.Equal(tt, 0, wrapRow)

	line, wrapRow, ok = wrappedLineAtRow(lines, 4, 1)
	require.True(tt, ok)
	require.Equal(tt, 2, line.NewLine)
	require.Equal(tt, 0, wrapRow)

	line, wrapRow, ok = wrappedLineAtRow(lines, 4, 2)
	require.True(tt, ok)
	require.Equal(tt, 2, line.NewLine)
	require.Equal(tt, 1, wrapRow)

	_, _, ok = wrappedLineAtRow(lines, 4, 3)
	require.False(tt, ok)
}

func TestWrappedContentHeight(tt *testing.T) {
	lines := []RenderedDiffLine{
		{ContentWidth: 3},
		{ContentWidth: 8},
	}
	require.Equal(tt, 3, wrappedContentHeight(lines, 4))
	require.Equal(tt, 2, wrappedContentHeight(lines, 10))
	require.Equal(tt, 1, wrappedContentHeight(nil, 4))
}

func TestSideBySidePaneLayout(tt *testing.T) {
	side := &SideBySideRenderedFile{
		LeftNumWidth:  3,
		RightNumWidth: 2,
	}

	layout := sideBySidePaneLayout(80, side, false, 0.5)
	require.Equal(tt, 0, layout.LeftPaneX)
	require.Equal(tt, 39, layout.LeftPaneWidth)
	require.Equal(tt, 1, layout.DividerWidth)
	require.Equal(tt, 39, layout.DividerX)
	require.Equal(tt, 40, layout.RightPaneX)
	require.Equal(tt, 40, layout.RightPaneWidth)
	require.Equal(tt, sideLineGutterWidth(3, false), layout.LeftGutterWidth)
	require.Equal(tt, sideLineGutterWidth(2, false), layout.RightGutterWidth)
	require.Equal(tt, layout.LeftPaneWidth-layout.LeftGutterWidth, layout.LeftContentWidth)
	require.Equal(tt, layout.RightPaneWidth-layout.RightGutterWidth, layout.RightContentWidth)
}

func TestSideBySidePaneLayout_ClampsToDynamicMinima(tt *testing.T) {
	side := &SideBySideRenderedFile{
		LeftNumWidth:  3,
		RightNumWidth: 2,
	}
	metrics := sideBySideDividerMetrics(60, side, false)
	require.Less(tt, metrics.MinOffset, metrics.MaxOffset)

	leftClamped := sideBySidePaneLayout(60, side, false, 0.01)
	require.Equal(tt, metrics.MinOffset, leftClamped.DividerX)
	require.Equal(tt, metrics.MinOffset, leftClamped.LeftPaneWidth)

	rightClamped := sideBySidePaneLayout(60, side, false, 0.99)
	require.Equal(tt, metrics.MaxOffset, rightClamped.DividerX)
	require.Equal(tt, metrics.Available-metrics.MaxOffset, rightClamped.RightPaneWidth)
}

func TestSideBySidePaneLayout_NarrowViewLocksDividerToCenter(tt *testing.T) {
	side := &SideBySideRenderedFile{
		LeftNumWidth:  3,
		RightNumWidth: 2,
	}
	metrics := sideBySideDividerMetrics(10, side, false)
	require.Equal(tt, metrics.MinOffset, metrics.MaxOffset)
	require.Equal(tt, 4, metrics.MinOffset)

	layout := sideBySidePaneLayout(10, side, false, 0.95)
	require.Equal(tt, metrics.MinOffset, layout.DividerX)
	require.Equal(tt, 4, layout.LeftPaneWidth)
	require.Equal(tt, 5, layout.RightPaneWidth)
}

func TestSideBySidePaneLayout_RatioRoundTripPreservesOffsets(tt *testing.T) {
	side := &SideBySideRenderedFile{
		LeftNumWidth:  3,
		RightNumWidth: 2,
	}
	metrics := sideBySideDividerMetrics(80, side, false)
	for offset := metrics.MinOffset; offset <= metrics.MaxOffset; offset++ {
		ratio := 0.5
		if metrics.Available > 0 {
			ratio = float64(offset) / float64(metrics.Available)
		}
		panes := sideBySidePaneLayout(80, side, false, ratio)
		require.Equal(tt, offset, panes.DividerX, "offset %d should map back exactly", offset)
	}
}

func TestSideDividerLineNumberRole(tt *testing.T) {
	role, kind, ok := sideDividerLineNumberRole(SideBySideRenderedRow{
		Left: &RenderedSideCell{Kind: RenderedLineRemove},
		Right: &RenderedSideCell{
			Kind: RenderedLineAdd,
		},
	})
	require.True(tt, ok)
	require.Equal(tt, TokenRoleLineNumberAdd, role)
	require.Equal(tt, RenderedLineAdd, kind)

	role, kind, ok = sideDividerLineNumberRole(SideBySideRenderedRow{
		Right: &RenderedSideCell{Kind: RenderedLineAdd},
	})
	require.True(tt, ok)
	require.Equal(tt, TokenRoleLineNumberAdd, role)
	require.Equal(tt, RenderedLineAdd, kind)

	_, _, ok = sideDividerLineNumberRole(SideBySideRenderedRow{})
	require.False(tt, ok)
}

func TestShouldRenderSideDivider(tt *testing.T) {
	require.True(tt, shouldRenderSideDivider(SideBySideRenderedRow{
		Right: &RenderedSideCell{Kind: RenderedLineContext},
	}))
	require.True(tt, shouldRenderSideDivider(SideBySideRenderedRow{
		Left: &RenderedSideCell{Kind: RenderedLineRemove},
	}))
	require.False(tt, shouldRenderSideDivider(SideBySideRenderedRow{}))
}

func TestSideDividerStyle_UsesHatchStyleWhenRightIsEmpty(tt *testing.T) {
	theme, ok := t.GetTheme(t.CurrentThemeName())
	require.True(tt, ok)

	view := DiffView{
		Palette: NewThemePalette(theme),
	}
	line := SideBySideRenderedRow{
		Left: &RenderedSideCell{Kind: RenderedLineRemove},
	}

	style, ok := view.sideDividerStyle(line)
	require.True(tt, ok)

	expectedHatch := view.styleForRole(TokenRoleDiffHatch)
	if expectedHatch.ForegroundColor == nil || !expectedHatch.ForegroundColor.IsSet() {
		require.Nil(tt, style.ForegroundColor)
		return
	}
	require.NotNil(tt, style.ForegroundColor)
	expectedFg := expectedHatch.ForegroundColor.ColorAt(1, 1, 0, 0)
	actualFg := style.ForegroundColor.ColorAt(1, 1, 0, 0)
	require.Equal(tt, expectedFg, actualFg)
}

func TestSideDividerStyle_DraggingEmphasizesDividerColumn(tt *testing.T) {
	theme, ok := t.GetTheme(t.CurrentThemeName())
	require.True(tt, ok)

	state := NewDiffViewState(buildTestRenderedFile(4, 10))
	state.StartSideDividerDrag(5, 5)

	view := DiffView{
		State:   state,
		Palette: NewThemePalette(theme),
	}
	line := SideBySideRenderedRow{
		Right: &RenderedSideCell{Kind: RenderedLineAdd},
	}

	dragStyle, ok := view.sideDividerStyle(line)
	require.True(tt, ok)
	require.NotNil(tt, dragStyle.ForegroundColor)
	require.True(tt, dragStyle.ForegroundColor.IsSet())

	state.StopSideDividerDrag()
	normalStyle, ok := view.sideDividerStyle(line)
	require.True(tt, ok)
	require.NotNil(tt, normalStyle.ForegroundColor)
	require.True(tt, normalStyle.ForegroundColor.IsSet())

	dragFg := dragStyle.ForegroundColor.ColorAt(1, 1, 0, 0)
	normalFg := normalStyle.ForegroundColor.ColorAt(1, 1, 0, 0)
	require.Greater(tt, dragFg.Alpha(), normalFg.Alpha())
}

func TestWrappedSideContentHeight_UsesMaxWrappedRowsPerPair(tt *testing.T) {
	rows := []SideBySideRenderedRow{
		{
			Left: &RenderedSideCell{
				Kind:         RenderedLineContext,
				LineNumber:   1,
				Prefix:       " ",
				Segments:     []RenderedSegment{{Text: "abcdefgh", Role: TokenRoleSyntaxPlain}},
				ContentWidth: 8,
			},
			Right: &RenderedSideCell{
				Kind:         RenderedLineContext,
				LineNumber:   1,
				Prefix:       " ",
				Segments:     []RenderedSegment{{Text: "abc", Role: TokenRoleSyntaxPlain}},
				ContentWidth: 3,
			},
		},
		{
			Shared: &RenderedDiffLine{
				Kind:         RenderedLineMeta,
				Segments:     []RenderedSegment{{Text: "meta", Role: TokenRoleDiffMeta}},
				ContentWidth: 4,
			},
		},
	}
	panes := sidePaneLayout{
		LeftContentWidth:  4,
		RightContentWidth: 4,
	}
	require.Equal(tt, 3, wrappedSideContentHeight(rows, panes, 4))
}

func TestSideBySideMaxScrollX(tt *testing.T) {
	side := &SideBySideRenderedFile{
		LeftNumWidth:         2,
		RightNumWidth:        2,
		LeftMaxContentWidth:  80,
		RightMaxContentWidth: 50,
	}

	maxScroll := sideBySideMaxScrollX(side, false, 60, 0.5)
	panes := sideBySidePaneLayout(60, side, false, 0.5)
	expected := max(80-panes.LeftContentWidth, 50-panes.RightContentWidth)
	if expected < 0 {
		expected = 0
	}
	require.Equal(tt, expected, maxScroll)
}

func TestDiffView_OnMouseDownStartsDragOnlyOnDivider(tt *testing.T) {
	view, state, _, sideBySide := newSideBySideDragTestView(80)
	panes := sideBySidePaneLayout(80, sideBySide, view.HideChangeSigns, state.SideBySideSplitRatio())

	view.OnMouseDown(t.MouseEvent{LocalX: panes.DividerX - 1, Button: uv.MouseLeft})
	require.False(tt, state.SideDividerDragging())

	view.OnMouseDown(t.MouseEvent{LocalX: panes.DividerX, Button: uv.MouseLeft})
	require.True(tt, state.SideDividerDragging())
	require.Equal(tt, 0, state.SideDividerDragOffset())
}

func TestDiffView_OnMouseMoveUpdatesSplitAndClampsScroll(tt *testing.T) {
	view, state, rendered, sideBySide := newSideBySideDragTestView(80)
	panes := sideBySidePaneLayout(80, sideBySide, view.HideChangeSigns, state.SideBySideSplitRatio())
	metrics := sideBySideDividerMetrics(80, sideBySide, view.HideChangeSigns)

	view.OnMouseDown(t.MouseEvent{LocalX: panes.DividerX, Button: uv.MouseLeft})
	require.True(tt, state.SideDividerDragging())

	state.ScrollX.Set(999)
	targetX := panes.DividerX + 8
	view.OnMouseMove(t.MouseEvent{LocalX: targetX, Button: uv.MouseLeft})

	expectedOffset := clampInt(targetX, metrics.MinOffset, metrics.MaxOffset)
	expectedSplit := float64(expectedOffset) / float64(metrics.Available)
	require.InDelta(tt, expectedSplit, state.SideBySideSplitRatio(), 0.0001)

	gutterWidth := sideBySideStateGutterWidth(rendered, sideBySide, view.HideChangeSigns, 80, state.SideBySideSplitRatio())
	require.Equal(tt, state.MaxScrollX(gutterWidth), state.ScrollX.Peek())
}

func TestDiffView_OnMouseUpStopsDrag(tt *testing.T) {
	view, state, _, sideBySide := newSideBySideDragTestView(80)
	panes := sideBySidePaneLayout(80, sideBySide, view.HideChangeSigns, state.SideBySideSplitRatio())
	view.OnMouseDown(t.MouseEvent{LocalX: panes.DividerX, Button: uv.MouseLeft})
	require.True(tt, state.SideDividerDragging())

	view.OnMouseUp(t.MouseEvent{Button: uv.MouseLeft})
	require.False(tt, state.SideDividerDragging())
	require.True(tt, state.SideDividerOverlayVisible())
}

func TestDiffView_DraggingIsNoopOutsideSideBySideMode(tt *testing.T) {
	view, state, _, sideBySide := newSideBySideDragTestView(80)
	panes := sideBySidePaneLayout(80, sideBySide, view.HideChangeSigns, state.SideBySideSplitRatio())
	initialSplit := state.SideBySideSplitRatio()

	view.LayoutMode = DiffLayoutUnified
	view.OnMouseDown(t.MouseEvent{LocalX: panes.DividerX, Button: uv.MouseLeft})
	require.False(tt, state.SideDividerDragging())

	state.StartSideDividerDrag(panes.DividerX, panes.DividerX)
	view.OnMouseMove(t.MouseEvent{LocalX: panes.DividerX + 12, Button: uv.MouseLeft})
	require.False(tt, state.SideDividerDragging())
	require.Equal(tt, initialSplit, state.SideBySideSplitRatio())
}

func newSideBySideDragTestView(width int) (DiffView, *DiffViewState, *RenderedFile, *SideBySideRenderedFile) {
	rendered := buildTestRenderedFile(20, 120)
	sideBySide := &SideBySideRenderedFile{
		Title: "drag-test",
		Rows: []SideBySideRenderedRow{
			{
				Left:  &RenderedSideCell{Kind: RenderedLineContext, LineNumber: 1, Prefix: " ", ContentWidth: 120},
				Right: &RenderedSideCell{Kind: RenderedLineContext, LineNumber: 1, Prefix: " ", ContentWidth: 96},
			},
		},
		LeftNumWidth:         3,
		RightNumWidth:        2,
		LeftMaxContentWidth:  120,
		RightMaxContentWidth: 96,
	}
	state := NewDiffViewState(rendered)
	state.SetRenderedPair(rendered, sideBySide)
	gutterWidth := sideBySideStateGutterWidth(rendered, sideBySide, false, width, state.SideBySideSplitRatio())
	state.SetViewport(width, 10, gutterWidth)

	view := DiffView{
		State:           state,
		LayoutMode:      DiffLayoutSideBySide,
		HideChangeSigns: false,
	}
	return view, state, rendered, sideBySide
}

func TestSideDividerSizeOverlayLayout(t *testing.T) {
	panes := sidePaneLayout{
		LeftPaneWidth:  39,
		DividerX:       39,
		DividerWidth:   1,
		RightPaneWidth: 40,
	}

	leftText, leftX, rightText, rightX := sideDividerSizeOverlayLayout(panes, 80)
	require.Equal(t, "← 39 ", leftText)
	require.Equal(t, 34, leftX)
	require.Equal(t, "40 →", rightText)
	require.Equal(t, 40, rightX)
}

func TestSideDividerSizeOverlayLayout_HandlesNarrowViewport(tt *testing.T) {
	panes := sidePaneLayout{
		LeftPaneWidth:  123,
		DividerX:       1,
		DividerWidth:   1,
		RightPaneWidth: 45,
	}

	leftText, leftX, rightText, rightX := sideDividerSizeOverlayLayout(panes, 4)
	require.Equal(tt, "3", leftText)
	require.Equal(tt, 0, leftX)
	require.Equal(tt, "45", rightText)
	require.Equal(tt, 2, rightX)
}

func TestDiffView_SideDividerOverlayRowIsOneThirdFromTop(tt *testing.T) {
	view := DiffView{}
	require.Equal(tt, 6, view.sideDividerOverlayRow(0, 20))
	require.Equal(tt, 6, view.sideDividerOverlayRow(5, 10))
	require.Equal(tt, 5, view.sideDividerOverlayRow(5, 6))
	require.Equal(tt, 5, view.sideDividerOverlayRow(5, 5))
}

func TestDiffView_SideDividerSizeOverlayStyle_UsesSecondaryLabelColors(tt *testing.T) {
	theme, ok := t.GetTheme(t.CurrentThemeName())
	require.True(tt, ok)

	view := DiffView{}
	style := view.sideDividerSizeOverlayStyle()
	require.NotNil(tt, style.ForegroundColor)
	require.NotNil(tt, style.BackgroundColor)
	require.Equal(tt, theme.SecondaryText, style.ForegroundColor.ColorAt(1, 1, 0, 0))
	require.Equal(tt, theme.SecondaryBg, style.BackgroundColor.ColorAt(1, 1, 0, 0))
}

func TestStyleForSegment_AppliesBackgroundIntralineOverlay(tt *testing.T) {
	theme, ok := t.GetTheme(t.CurrentThemeName())
	require.True(tt, ok)

	view := DiffView{
		Palette:        NewThemePalette(theme),
		IntralineStyle: IntralineStyleModeBackground,
	}
	segment := RenderedSegment{Text: "x", Role: TokenRoleSyntaxString, Intraline: IntralineMarkAdd}

	base := view.styleForRole(segment.Role)
	style := view.styleForSegment(segment)
	overlay, ok := view.Palette.IntralineOverlayStyle(IntralineMarkAdd, IntralineStyleModeBackground)
	require.True(tt, ok)
	require.True(tt, overlay.Background.IsSet())

	require.NotNil(tt, style.BackgroundColor)
	require.Equal(tt, overlay.Background, style.BackgroundColor.ColorAt(1, 1, 0, 0))

	require.NotNil(tt, base.ForegroundColor)
	require.NotNil(tt, style.ForegroundColor)
	require.Equal(
		tt,
		base.ForegroundColor.ColorAt(1, 1, 0, 0),
		style.ForegroundColor.ColorAt(1, 1, 0, 0),
	)
}

func TestStyleForSegment_AppliesUnderlineIntralineOverlayWithoutChangingForeground(tt *testing.T) {
	theme, ok := t.GetTheme(t.CurrentThemeName())
	require.True(tt, ok)

	view := DiffView{
		Palette:        NewThemePalette(theme),
		IntralineStyle: IntralineStyleModeUnderline,
	}
	segment := RenderedSegment{Text: "x", Role: TokenRoleSyntaxKeyword, Intraline: IntralineMarkRemove}

	base := view.styleForRole(segment.Role)
	style := view.styleForSegment(segment)

	require.Equal(tt, t.UnderlineSingle, style.Underline)
	require.Equal(tt, theme.Error, style.UnderlineColor)
	require.NotNil(tt, base.ForegroundColor)
	require.NotNil(tt, style.ForegroundColor)
	require.Equal(
		tt,
		base.ForegroundColor.ColorAt(1, 1, 0, 0),
		style.ForegroundColor.ColorAt(1, 1, 0, 0),
	)
}

func TestStyleForSegment_LeavesBaseStyleWhenNoIntralineMark(tt *testing.T) {
	theme, ok := t.GetTheme(t.CurrentThemeName())
	require.True(tt, ok)

	view := DiffView{
		Palette:        NewThemePalette(theme),
		IntralineStyle: IntralineStyleModeUnderline,
	}
	segment := RenderedSegment{Text: "x", Role: TokenRoleSyntaxPlain, Intraline: IntralineMarkNone}

	base := view.styleForRole(segment.Role)
	style := view.styleForSegment(segment)

	require.Equal(tt, base, style)
}
