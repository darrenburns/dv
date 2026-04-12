package main

import (
	"math"
	"strconv"
	"strings"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	t "github.com/darrenburns/terma"
	"github.com/darrenburns/terma/layout"
)

// DiffView is a purpose-built diff renderer with fixed gutter and scroll support.
type DiffView struct {
	ID              string
	DisableFocus    bool
	State           *DiffViewState
	VerticalScroll  *t.ScrollState
	LayoutMode      DiffLayoutMode
	HardWrap        bool
	HideChangeSigns bool
	IntralineStyle  IntralineStyleMode
	Palette         ThemePalette
	Width           t.Dimension
	Height          t.Dimension
	Style           t.Style
}

const selectionAutoScrollInset = 1

func (d DiffView) Build(ctx t.BuildContext) t.Widget {
	d.Palette = NewThemePalette(ctx.Theme())
	return d
}

func (d DiffView) WidgetID() string {
	return d.ID
}

func (d DiffView) IsFocusable() bool {
	return !d.DisableFocus
}

func (d DiffView) GetContentDimensions() (width, height t.Dimension) {
	dims := d.Style.GetDimensions()
	width, height = dims.Width, dims.Height
	if width.IsUnset() {
		width = d.Width
	}
	if height.IsUnset() {
		height = d.Height
	}
	return width, height
}

func (d DiffView) GetStyle() t.Style {
	return d.Style
}

func (d DiffView) OnMouseDown(event t.MouseEvent) {
	if d.State == nil || event.Button != uv.MouseLeft {
		return
	}
	event = d.viewportSelectionEvent(event)

	if d.LayoutMode == DiffLayoutSideBySide && d.startSideDividerDrag(event) {
		return
	}

	track, point, ok := d.selectionStartPoint(event)
	if !ok {
		d.State.ClearSelection()
		return
	}
	d.State.StartSelection(track, point)
	d.State.SetSelectionPointer(event.LocalX, event.LocalY)
}

func (d DiffView) OnMouseMove(event t.MouseEvent) {
	if d.State == nil {
		return
	}
	event = d.viewportSelectionEvent(event)
	if d.State.SideDividerDragging() {
		d.dragSideDivider(event)
		return
	}
	if !d.State.SelectionDragging() {
		return
	}
	d.State.SetSelectionPointer(event.LocalX, event.LocalY)
	d.runSelectionAutoscrollTick()
}

func (d DiffView) OnMouseUp(event t.MouseEvent) {
	if d.State == nil {
		return
	}
	wasDragging := d.State.SideDividerDragging()
	d.State.StopSideDividerDrag()
	if wasDragging {
		d.State.MarkSideDividerResized()
	}
	d.State.StopSelectionDrag()
}

func (d DiffView) startSideDividerDrag(event t.MouseEvent) bool {
	if d.State == nil || d.LayoutMode != DiffLayoutSideBySide {
		return false
	}
	sideBySide := d.currentSideBySide()
	if sideBySide == nil {
		return false
	}

	viewportWidth := d.State.ViewportWidth()
	if viewportWidth <= 0 {
		return false
	}

	panes := sideBySidePaneLayout(viewportWidth, sideBySide, d.HideChangeSigns, d.sideBySideSplitRatio())
	if panes.DividerWidth <= 0 {
		return false
	}
	if event.LocalX < panes.DividerX || event.LocalX >= panes.DividerX+panes.DividerWidth {
		return false
	}

	d.State.StartSideDividerDrag(event.LocalX, panes.DividerX)
	return true
}

func (d DiffView) dragSideDivider(event t.MouseEvent) {
	if d.State == nil {
		return
	}
	if d.LayoutMode != DiffLayoutSideBySide {
		d.State.StopSideDividerDrag()
		return
	}

	sideBySide := d.currentSideBySide()
	if sideBySide == nil {
		return
	}

	viewportWidth := d.State.ViewportWidth()
	metrics := sideBySideDividerMetrics(viewportWidth, sideBySide, d.HideChangeSigns)
	newOffset := event.LocalX - d.State.SideDividerDragOffset()
	newOffset = clampInt(newOffset, metrics.MinOffset, metrics.MaxOffset)

	ratio := 0.5
	if metrics.Available > 0 {
		ratio = float64(newOffset) / float64(metrics.Available)
	}
	d.State.SetSideBySideSplitRatio(ratio)
	d.State.MarkSideDividerResized()
	d.clampSideBySideHorizontalScroll(viewportWidth, sideBySide)
}

func (d DiffView) selectionStartPoint(event t.MouseEvent) (DiffSelectionTrack, DiffSelectionPoint, bool) {
	if d.State == nil {
		return DiffSelectionTrackNone, DiffSelectionPoint{}, false
	}
	if d.LayoutMode == DiffLayoutSideBySide {
		return d.sideBySideSelectionStartPoint(event)
	}
	return d.unifiedSelectionPoint(event, true)
}

func (d DiffView) selectionDragPoint(event t.MouseEvent) (DiffSelectionPoint, bool) {
	if d.State == nil {
		return DiffSelectionPoint{}, false
	}
	if d.LayoutMode == DiffLayoutSideBySide {
		return d.sideBySideSelectionDragPoint(event)
	}
	_, point, ok := d.unifiedSelectionPoint(event, false)
	return point, ok
}

func (d DiffView) unifiedSelectionPoint(event t.MouseEvent, strict bool) (DiffSelectionTrack, DiffSelectionPoint, bool) {
	rendered := d.currentRendered()
	if rendered == nil {
		return DiffSelectionTrackNone, DiffSelectionPoint{}, false
	}

	gutterWidth := renderedGutterWidth(rendered, d.HideChangeSigns)
	viewportWidth := max(0, d.State.ViewportWidth())
	if strict && event.LocalX < gutterWidth {
		return DiffSelectionTrackNone, DiffSelectionPoint{}, false
	}
	contentX := event.LocalX - gutterWidth
	if contentX < 0 {
		contentX = 0
	}

	rowIdx, line, wrapRow, ok := d.unifiedLineAtMouseRow(event.LocalY)
	if !ok {
		return DiffSelectionTrackNone, DiffSelectionPoint{}, false
	}

	visibleWidth := max(1, viewportWidth-gutterWidth)
	text := lineText(line)
	contentOffset := d.currentHorizontalScroll()
	if d.HardWrap {
		contentOffset = wrapRow * visibleWidth
	} else {
		contentOffset = horizontalScrollXForLine(line.Kind, contentOffset)
	}
	grapheme := graphemeIndexForDisplayColumn(text, contentOffset+contentX)
	return DiffSelectionTrackUnified, DiffSelectionPoint{
		Row:      rowIdx,
		Grapheme: grapheme,
		Lane:     DiffSelectionLaneUnified,
	}, true
}

func (d DiffView) sideBySideSelectionStartPoint(event t.MouseEvent) (DiffSelectionTrack, DiffSelectionPoint, bool) {
	sideBySide := d.currentSideBySide()
	if sideBySide == nil {
		return DiffSelectionTrackNone, DiffSelectionPoint{}, false
	}

	panes := sideBySidePaneLayout(d.State.ViewportWidth(), sideBySide, d.HideChangeSigns, d.sideBySideSplitRatio())
	rowIdx, row, wrapRow, ok := d.sideBySideRowAtMouseRow(event.LocalY, panes)
	if !ok {
		return DiffSelectionTrackNone, DiffSelectionPoint{}, false
	}
	if row.Shared != nil {
		point := d.sharedSelectionPoint(rowIdx, *row.Shared, wrapRow, event.LocalX)
		return DiffSelectionTrackShared, point, true
	}

	if event.LocalX >= panes.LeftPaneX+panes.LeftGutterWidth && event.LocalX < panes.LeftPaneX+panes.LeftPaneWidth && row.Left != nil {
		point := d.sideCellSelectionPoint(rowIdx, row.Left, wrapRow, event.LocalX, panes.LeftPaneX+panes.LeftGutterWidth, max(1, panes.LeftPaneWidth-panes.LeftGutterWidth), DiffSelectionLaneLeft)
		return DiffSelectionTrackLeft, point, true
	}
	if event.LocalX >= panes.RightPaneX+panes.RightGutterWidth && event.LocalX < panes.RightPaneX+panes.RightPaneWidth && row.Right != nil {
		point := d.sideCellSelectionPoint(rowIdx, row.Right, wrapRow, event.LocalX, panes.RightPaneX+panes.RightGutterWidth, max(1, panes.RightPaneWidth-panes.RightGutterWidth), DiffSelectionLaneRight)
		return DiffSelectionTrackRight, point, true
	}
	return DiffSelectionTrackNone, DiffSelectionPoint{}, false
}

func (d DiffView) sideBySideSelectionDragPoint(event t.MouseEvent) (DiffSelectionPoint, bool) {
	sideBySide := d.currentSideBySide()
	if sideBySide == nil {
		return DiffSelectionPoint{}, false
	}

	track := d.State.SelectionTrack()
	panes := sideBySidePaneLayout(d.State.ViewportWidth(), sideBySide, d.HideChangeSigns, d.sideBySideSplitRatio())
	rowIdx, row, wrapRow, ok := d.sideBySideRowAtMouseRow(event.LocalY, panes)
	if !ok {
		return DiffSelectionPoint{}, false
	}
	if row.Shared != nil {
		return d.sharedSelectionPoint(rowIdx, *row.Shared, wrapRow, event.LocalX), true
	}

	switch track {
	case DiffSelectionTrackLeft:
		return d.sideCellSelectionPoint(rowIdx, row.Left, wrapRow, event.LocalX, panes.LeftPaneX+panes.LeftGutterWidth, max(1, panes.LeftPaneWidth-panes.LeftGutterWidth), DiffSelectionLaneLeft), true
	case DiffSelectionTrackRight:
		return d.sideCellSelectionPoint(rowIdx, row.Right, wrapRow, event.LocalX, panes.RightPaneX+panes.RightGutterWidth, max(1, panes.RightPaneWidth-panes.RightGutterWidth), DiffSelectionLaneRight), true
	case DiffSelectionTrackShared:
		return DiffSelectionPoint{}, false
	default:
		return DiffSelectionPoint{}, false
	}
}

func (d DiffView) unifiedLineAtMouseRow(localY int) (rowIdx int, line RenderedDiffLine, wrapRow int, ok bool) {
	rendered := d.currentRendered()
	if rendered == nil {
		return 0, RenderedDiffLine{}, 0, false
	}
	contentRow := d.mouseContentRow(localY)
	if d.HardWrap {
		return wrappedLineAtRowWithIndex(rendered.Lines, max(1, d.State.ViewportWidth()-renderedGutterWidth(rendered, d.HideChangeSigns)), contentRow)
	}
	if contentRow < 0 || contentRow >= len(rendered.Lines) {
		return 0, RenderedDiffLine{}, 0, false
	}
	return contentRow, rendered.Lines[contentRow], 0, true
}

func (d DiffView) sideBySideRowAtMouseRow(localY int, panes sidePaneLayout) (rowIdx int, row SideBySideRenderedRow, wrapRow int, ok bool) {
	sideBySide := d.currentSideBySide()
	if sideBySide == nil {
		return 0, SideBySideRenderedRow{}, 0, false
	}
	contentRow := d.mouseContentRow(localY)
	if d.HardWrap {
		return wrappedSideRowAtRowWithIndex(sideBySide.Rows, panes, d.State.ViewportWidth(), contentRow)
	}
	if contentRow < 0 || contentRow >= len(sideBySide.Rows) {
		return 0, SideBySideRenderedRow{}, 0, false
	}
	return contentRow, sideBySide.Rows[contentRow], 0, true
}

func (d DiffView) sharedSelectionPoint(rowIdx int, line RenderedDiffLine, wrapRow int, localX int) DiffSelectionPoint {
	visibleWidth := max(1, d.State.ViewportWidth())
	contentX := localX
	if contentX < 0 {
		contentX = 0
	}
	contentOffset := d.currentHorizontalScroll()
	if d.HardWrap {
		contentOffset = wrapRow * visibleWidth
	} else {
		contentOffset = horizontalScrollXForLine(line.Kind, contentOffset)
	}
	return DiffSelectionPoint{
		Row:      rowIdx,
		Grapheme: graphemeIndexForDisplayColumn(lineText(line), contentOffset+contentX),
		Lane:     DiffSelectionLaneShared,
	}
}

func (d DiffView) sideCellSelectionPoint(rowIdx int, cell *RenderedSideCell, wrapRow int, localX int, contentStartX int, visibleWidth int, lane DiffSelectionLane) DiffSelectionPoint {
	contentX := localX - contentStartX
	if contentX < 0 {
		contentX = 0
	}
	contentOffset := d.currentHorizontalScroll()
	if d.HardWrap {
		contentOffset = wrapRow * max(1, visibleWidth)
	}
	return DiffSelectionPoint{
		Row:      rowIdx,
		Grapheme: graphemeIndexForDisplayColumn(diffCellText(cell), contentOffset+contentX),
		Lane:     lane,
	}
}

func (d DiffView) mouseContentRow(localY int) int {
	return d.currentVerticalScroll() + localY
}

func (d DiffView) currentVerticalScroll() int {
	if d.VerticalScroll != nil {
		return d.VerticalScroll.Offset.Peek()
	}
	if d.State == nil {
		return 0
	}
	return d.State.ScrollY.Peek()
}

func (d DiffView) currentHorizontalScroll() int {
	if d.State == nil {
		return 0
	}
	return d.State.ScrollX.Peek()
}

func (d DiffView) viewportSelectionEvent(event t.MouseEvent) t.MouseEvent {
	if d.VerticalScroll == nil {
		return event
	}
	event.LocalY -= d.currentVerticalScroll()
	return event
}

func (d DiffView) clampSelectionViewportEvent(event t.MouseEvent) t.MouseEvent {
	if d.State == nil {
		return event
	}
	width := d.State.ViewportWidth()
	height := d.State.ViewportHeight()
	if width > 0 {
		event.LocalX = clampInt(event.LocalX, 0, width-1)
	}
	if height > 0 {
		event.LocalY = clampInt(event.LocalY, 0, height-1)
	}
	return event
}

func (d DiffView) runSelectionAutoscrollTick() {
	if d.State == nil || !d.State.SelectionDragging() {
		return
	}
	localX, localY := d.State.SelectionPointer()
	event := t.MouseEvent{LocalX: localX, LocalY: localY}
	deltaX, deltaY := d.selectionAutoscrollDelta(event)
	if deltaY != 0 {
		d.scrollSelectionY(deltaY)
	}
	if deltaX != 0 {
		d.scrollSelectionX(deltaX)
	}
	clamped := d.clampSelectionViewportEvent(event)
	if point, ok := d.selectionDragPoint(clamped); ok {
		d.State.UpdateSelection(point)
	}
	if deltaX == 0 && deltaY == 0 {
		d.State.StopSelectionAutoscrollTimer()
		return
	}
	d.State.ScheduleSelectionAutoscrollTick(func() {
		t.Dispatch(func() {
			d.runSelectionAutoscrollTick()
		})
	})
}

func (d DiffView) selectionAutoscrollDelta(event t.MouseEvent) (deltaX int, deltaY int) {
	if d.State == nil {
		return 0, 0
	}
	deltaY = selectionAutoscrollAxisDelta(event.LocalY, d.State.ViewportHeight())
	if d.HardWrap {
		return 0, deltaY
	}
	startX, endX, ok := d.selectionHorizontalAutoscrollBounds()
	if !ok {
		return 0, deltaY
	}
	deltaX = selectionAutoscrollRangeDelta(event.LocalX, startX, endX)
	return deltaX, deltaY
}

func selectionAutoscrollAxisDelta(position int, size int) int {
	if size <= 0 {
		return 0
	}
	return selectionAutoscrollRangeDelta(position, 0, size-1)
}

func selectionAutoscrollRangeDelta(position int, start int, end int) int {
	if end < start {
		return 0
	}
	size := end - start + 1
	inset := selectionAutoScrollInset
	if inset <= 0 {
		inset = 1
	}
	if size <= inset*2 {
		inset = max(1, size/2)
	}
	leftEdge := start + inset
	rightEdge := end - inset
	if position < leftEdge {
		return position - leftEdge
	}
	if position > rightEdge {
		return position - rightEdge
	}
	return 0
}

func (d DiffView) selectionHorizontalAutoscrollBounds() (startX int, endX int, ok bool) {
	if d.State == nil {
		return 0, 0, false
	}
	viewportWidth := d.State.ViewportWidth()
	if viewportWidth <= 0 {
		return 0, 0, false
	}

	switch d.State.SelectionTrack() {
	case DiffSelectionTrackUnified:
		startX = renderedGutterWidth(d.currentRendered(), d.HideChangeSigns)
		endX = viewportWidth - 1
	case DiffSelectionTrackLeft:
		sideBySide := d.currentSideBySide()
		if sideBySide == nil {
			return 0, 0, false
		}
		panes := sideBySidePaneLayout(viewportWidth, sideBySide, d.HideChangeSigns, d.sideBySideSplitRatio())
		startX = panes.LeftPaneX + panes.LeftGutterWidth
		endX = panes.LeftPaneX + panes.LeftPaneWidth - 1
	case DiffSelectionTrackRight:
		sideBySide := d.currentSideBySide()
		if sideBySide == nil {
			return 0, 0, false
		}
		panes := sideBySidePaneLayout(viewportWidth, sideBySide, d.HideChangeSigns, d.sideBySideSplitRatio())
		startX = panes.RightPaneX + panes.RightGutterWidth
		endX = panes.RightPaneX + panes.RightPaneWidth - 1
	case DiffSelectionTrackShared:
		startX = 0
		endX = viewportWidth - 1
	default:
		return 0, 0, false
	}
	if startX < 0 {
		startX = 0
	}
	if endX >= viewportWidth {
		endX = viewportWidth - 1
	}
	if endX < startX {
		return 0, 0, false
	}
	return startX, endX, true
}

func (d DiffView) scrollSelectionY(delta int) {
	if d.State == nil || delta == 0 {
		return
	}
	current := d.currentVerticalScroll()
	next := clampInt(current+delta, 0, d.State.MaxScrollY())
	if next == current {
		return
	}
	if d.VerticalScroll != nil {
		d.VerticalScroll.Offset.Set(next)
	}
	d.State.ScrollY.Set(next)
}

func (d DiffView) scrollSelectionX(delta int) {
	if d.State == nil || delta == 0 {
		return
	}
	current := d.currentHorizontalScroll()
	next := clampInt(current+delta, 0, d.State.MaxScrollX(d.currentSelectionGutterWidth()))
	if next == current {
		return
	}
	d.State.ScrollX.Set(next)
}

func (d DiffView) currentSelectionGutterWidth() int {
	if d.State == nil {
		return 0
	}
	rendered := d.currentRendered()
	if d.LayoutMode == DiffLayoutSideBySide {
		return sideBySideStateGutterWidth(
			rendered,
			d.currentSideBySide(),
			d.HideChangeSigns,
			d.State.ViewportWidth(),
			d.sideBySideSplitRatio(),
		)
	}
	return renderedGutterWidth(rendered, d.HideChangeSigns)
}

func (d DiffView) BuildLayoutNode(ctx t.BuildContext) layout.LayoutNode {
	style := d.Style
	padding := toLayoutInsets(style.Padding)
	border := layout.EdgeInsetsAll(style.Border.Width())
	dims := d.Style.GetDimensions()
	if dims.Width.IsUnset() {
		dims.Width = d.Width
	}
	if dims.Height.IsUnset() {
		dims.Height = d.Height
	}

	minWidth, maxWidth, minHeight, maxHeight := dimensionSetToMinMax(dims, padding, border)
	expandWidth := dims.Width.IsFlex() || dims.Width.IsPercent()
	expandHeight := dims.Height.IsFlex() || dims.Height.IsPercent()

	return &layout.BoxNode{
		MinWidth:     minWidth,
		MaxWidth:     maxWidth,
		MinHeight:    minHeight,
		MaxHeight:    maxHeight,
		Padding:      padding,
		Border:       border,
		Margin:       toLayoutInsets(style.Margin),
		ExpandWidth:  expandWidth,
		ExpandHeight: expandHeight,
		MeasureFunc: func(constraints layout.Constraints) (int, int) {
			size := d.Layout(ctx, t.Constraints{
				MinWidth:  constraints.MinWidth,
				MaxWidth:  constraints.MaxWidth,
				MinHeight: constraints.MinHeight,
				MaxHeight: constraints.MaxHeight,
			})
			return size.Width, size.Height
		},
	}
}

func (d DiffView) Layout(ctx t.BuildContext, constraints t.Constraints) t.Size {
	rendered := d.currentRendered()
	sideBySide := d.currentSideBySide()

	dims := d.Style.GetDimensions()
	widthDim := dims.Width
	heightDim := dims.Height
	if widthDim.IsUnset() {
		widthDim = d.Width
	}
	if heightDim.IsUnset() {
		heightDim = d.Height
	}

	if d.LayoutMode == DiffLayoutSideBySide {
		return d.layoutSideBySide(constraints, widthDim, heightDim, sideBySide)
	}

	gutterWidth := renderedGutterWidth(rendered, d.HideChangeSigns)
	contentWidth := 1
	contentHeight := 1
	if rendered != nil {
		contentWidth = max(1, gutterWidth+rendered.MaxContentWidth)
		contentHeight = max(1, len(rendered.Lines))
	}

	width := contentWidth
	switch {
	case widthDim.IsCells():
		width = widthDim.CellsValue()
	case widthDim.IsFlex(), widthDim.IsPercent():
		width = constraints.MaxWidth
	}

	width = clampInt(width, constraints.MinWidth, constraints.MaxWidth)

	if d.HardWrap && rendered != nil {
		wrapWidth := max(1, width-gutterWidth)
		contentHeight = wrappedContentHeight(rendered.Lines, wrapWidth)
	}

	height := contentHeight
	switch {
	case heightDim.IsCells():
		height = heightDim.CellsValue()
	case heightDim.IsFlex(), heightDim.IsPercent():
		height = constraints.MaxHeight
	}

	height = clampInt(height, constraints.MinHeight, constraints.MaxHeight)

	return t.Size{Width: width, Height: height}
}

type sidePaneLayout struct {
	LeftPaneX         int
	LeftPaneWidth     int
	LeftGutterWidth   int
	LeftContentWidth  int
	DividerX          int
	DividerWidth      int
	RightPaneX        int
	RightPaneWidth    int
	RightGutterWidth  int
	RightContentWidth int
}

type sideDividerMetrics struct {
	Available int
	MinOffset int
	MaxOffset int
}

const sideEmptyHatchRune = "╱"
const sideDividerRune = "▏"
const emptyLineSelectionRune = "▌"

func (d DiffView) layoutSideBySide(constraints t.Constraints, widthDim t.Dimension, heightDim t.Dimension, sideBySide *SideBySideRenderedFile) t.Size {
	contentWidth := sideBySideNaturalWidth(sideBySide, d.HideChangeSigns)
	contentHeight := 1
	if sideBySide != nil {
		contentHeight = max(1, len(sideBySide.Rows))
	}

	width := contentWidth
	switch {
	case widthDim.IsCells():
		width = widthDim.CellsValue()
	case widthDim.IsFlex(), widthDim.IsPercent():
		width = constraints.MaxWidth
	}
	width = clampInt(width, constraints.MinWidth, constraints.MaxWidth)

	if d.HardWrap && sideBySide != nil {
		panes := sideBySidePaneLayout(width, sideBySide, d.HideChangeSigns, d.sideBySideSplitRatio())
		contentHeight = wrappedSideContentHeight(sideBySide.Rows, panes, width)
	}

	height := contentHeight
	switch {
	case heightDim.IsCells():
		height = heightDim.CellsValue()
	case heightDim.IsFlex(), heightDim.IsPercent():
		height = constraints.MaxHeight
	}
	height = clampInt(height, constraints.MinHeight, constraints.MaxHeight)

	return t.Size{Width: width, Height: height}
}

func sideBySideNaturalWidth(sideBySide *SideBySideRenderedFile, hideSigns bool) int {
	if sideBySide == nil {
		return 1
	}
	leftGutter := sideLineGutterWidth(sideBySide.LeftNumWidth, hideSigns)
	rightGutter := sideLineGutterWidth(sideBySide.RightNumWidth, hideSigns)
	dividerWidth := 1

	width := leftGutter + max(1, sideBySide.LeftMaxContentWidth) + dividerWidth + rightGutter + max(1, sideBySide.RightMaxContentWidth)
	shared := max(sideBySide.LeftMaxContentWidth, sideBySide.RightMaxContentWidth)
	if shared > width {
		width = shared
	}
	if width <= 0 {
		return 1
	}
	return width
}

func sideLineGutterWidth(numWidth int, hideSigns bool) int {
	if numWidth <= 0 {
		numWidth = 1
	}
	width := numWidth + 1
	if !hideSigns {
		width += 2
	}
	return width
}

func sideBySideDividerMetrics(totalWidth int, sideBySide *SideBySideRenderedFile, hideSigns bool) sideDividerMetrics {
	metrics := sideDividerMetrics{}
	if totalWidth <= 0 {
		return metrics
	}

	leftNumWidth := 1
	rightNumWidth := 1
	if sideBySide != nil {
		if sideBySide.LeftNumWidth > 0 {
			leftNumWidth = sideBySide.LeftNumWidth
		}
		if sideBySide.RightNumWidth > 0 {
			rightNumWidth = sideBySide.RightNumWidth
		}
	}

	available := totalWidth - 1
	if available < 0 {
		available = 0
	}
	metrics.Available = available

	leftMinPane := sideLineGutterWidth(leftNumWidth, hideSigns) + 1
	rightMinPane := sideLineGutterWidth(rightNumWidth, hideSigns) + 1
	if leftMinPane+rightMinPane > available {
		center := available / 2
		metrics.MinOffset = center
		metrics.MaxOffset = center
		return metrics
	}

	metrics.MinOffset = leftMinPane
	metrics.MaxOffset = available - rightMinPane
	return metrics
}

func sideBySidePaneLayout(totalWidth int, sideBySide *SideBySideRenderedFile, hideSigns bool, splitRatio float64) sidePaneLayout {
	layout := sidePaneLayout{}
	if totalWidth <= 0 {
		return layout
	}

	leftNumWidth := 1
	rightNumWidth := 1
	if sideBySide != nil {
		if sideBySide.LeftNumWidth > 0 {
			leftNumWidth = sideBySide.LeftNumWidth
		}
		if sideBySide.RightNumWidth > 0 {
			rightNumWidth = sideBySide.RightNumWidth
		}
	}

	layout.LeftPaneX = 0
	layout.LeftGutterWidth = sideLineGutterWidth(leftNumWidth, hideSigns)
	layout.RightGutterWidth = sideLineGutterWidth(rightNumWidth, hideSigns)

	dividerWidth := 1
	layout.DividerWidth = dividerWidth

	metrics := sideBySideDividerMetrics(totalWidth, sideBySide, hideSigns)
	available := metrics.Available
	dividerOffset := available / 2
	if available > 0 {
		scaled := float64(available) * clampSideBySideSplitRatio(splitRatio)
		dividerOffset = int(math.Floor(scaled + 1e-9))
	}
	dividerOffset = clampInt(dividerOffset, metrics.MinOffset, metrics.MaxOffset)

	layout.LeftPaneWidth = dividerOffset
	layout.RightPaneWidth = available - layout.LeftPaneWidth
	layout.DividerX = dividerOffset
	layout.RightPaneX = layout.DividerX + dividerWidth

	layout.LeftContentWidth = layout.LeftPaneWidth - layout.LeftGutterWidth
	layout.RightContentWidth = layout.RightPaneWidth - layout.RightGutterWidth
	if layout.LeftPaneWidth <= 0 {
		layout.LeftContentWidth = 0
	} else if layout.LeftContentWidth <= 0 {
		layout.LeftContentWidth = 1
	}
	if layout.RightPaneWidth <= 0 {
		layout.RightContentWidth = 0
	} else if layout.RightContentWidth <= 0 {
		layout.RightContentWidth = 1
	}
	return layout
}

func sideBySideMaxScrollX(sideBySide *SideBySideRenderedFile, hideSigns bool, viewportWidth int, splitRatio float64) int {
	if sideBySide == nil || viewportWidth <= 0 {
		return 0
	}
	panes := sideBySidePaneLayout(viewportWidth, sideBySide, hideSigns, splitRatio)

	leftVisible := panes.LeftContentWidth
	rightVisible := panes.RightContentWidth
	leftMax := max(1, sideBySide.LeftMaxContentWidth)
	rightMax := max(1, sideBySide.RightMaxContentWidth)

	leftScroll := leftMax - leftVisible
	if leftScroll < 0 {
		leftScroll = 0
	}
	rightScroll := rightMax - rightVisible
	if rightScroll < 0 {
		rightScroll = 0
	}
	return max(leftScroll, rightScroll)
}

func sideBySideStateGutterWidth(rendered *RenderedFile, sideBySide *SideBySideRenderedFile, hideSigns bool, viewportWidth int, splitRatio float64) int {
	if viewportWidth <= 0 {
		return 0
	}

	maxContent := renderedMaxContentWidth(rendered, sideBySide)
	maxScrollX := sideBySideMaxScrollX(sideBySide, hideSigns, viewportWidth, splitRatio)
	visibleContent := maxContent - maxScrollX
	if visibleContent < 0 {
		visibleContent = 0
	}
	if visibleContent > viewportWidth {
		visibleContent = viewportWidth
	}
	gutterWidth := viewportWidth - visibleContent
	if gutterWidth < 0 {
		return 0
	}
	return gutterWidth
}

func wrappedSideContentHeight(rows []SideBySideRenderedRow, panes sidePaneLayout, fullWidth int) int {
	if len(rows) == 0 {
		return 1
	}
	total := 0
	for _, row := range rows {
		total += wrappedSideRowCount(row, panes, fullWidth)
	}
	if total <= 0 {
		return 1
	}
	return total
}

func wrappedSideRowAtRow(rows []SideBySideRenderedRow, panes sidePaneLayout, fullWidth int, rowIdx int) (SideBySideRenderedRow, int, bool) {
	_, row, wrapRow, ok := wrappedSideRowAtRowWithIndex(rows, panes, fullWidth, rowIdx)
	return row, wrapRow, ok
}

func wrappedSideRowAtRowWithIndex(rows []SideBySideRenderedRow, panes sidePaneLayout, fullWidth int, rowIdx int) (itemIdx int, row SideBySideRenderedRow, wrapRow int, ok bool) {
	if rowIdx < 0 {
		return 0, SideBySideRenderedRow{}, 0, false
	}
	remaining := rowIdx
	for idx, row := range rows {
		rowsForItem := wrappedSideRowCount(row, panes, fullWidth)
		if remaining < rowsForItem {
			return idx, row, remaining, true
		}
		remaining -= rowsForItem
	}
	return 0, SideBySideRenderedRow{}, 0, false
}

func wrappedSideRowCount(row SideBySideRenderedRow, panes sidePaneLayout, fullWidth int) int {
	if row.Shared != nil {
		return wrappedLineRowCount(*row.Shared, max(1, fullWidth))
	}
	leftRows := wrappedSideCellRowCount(row.Left, max(1, panes.LeftContentWidth))
	rightRows := wrappedSideCellRowCount(row.Right, max(1, panes.RightContentWidth))
	return max(leftRows, rightRows)
}

func wrappedSideCellRowCount(cell *RenderedSideCell, wrapWidth int) int {
	if cell == nil || wrapWidth <= 0 || cell.ContentWidth <= 0 {
		return 1
	}
	rows := (cell.ContentWidth + wrapWidth - 1) / wrapWidth
	if rows <= 0 {
		return 1
	}
	return rows
}

func (d DiffView) Render(ctx *t.RenderContext) {
	if ctx.Width <= 0 || ctx.Height <= 0 || d.State == nil {
		return
	}

	rendered := d.State.Rendered.Get()
	if rendered == nil {
		rendered = buildMetaRenderedFile("Diff", []string{"No diff content to display."})
	}
	sideBySide := d.State.SideBySide.Get()
	if sideBySide == nil {
		sideBySide = buildSideBySideFromRendered(rendered)
	}

	clip := ctx.ClipBounds()
	visibleStart := 0
	if clip.Y > ctx.Y {
		visibleStart = clip.Y - ctx.Y
	}
	if visibleStart < 0 {
		visibleStart = 0
	}
	visibleEnd := ctx.Height
	clipEnd := clip.Y + clip.Height - ctx.Y
	if clipEnd < visibleEnd {
		visibleEnd = clipEnd
	}
	if visibleEnd > ctx.Height {
		visibleEnd = ctx.Height
	}
	if visibleEnd <= visibleStart {
		return
	}

	blankLine := strings.Repeat(" ", ctx.Width)
	for row := visibleStart; row < visibleEnd; row++ {
		ctx.DrawText(0, row, blankLine)
	}

	gutterWidth := renderedGutterWidth(rendered, d.HideChangeSigns)
	if d.LayoutMode == DiffLayoutSideBySide {
		gutterWidth = sideBySideStateGutterWidth(
			rendered,
			sideBySide,
			d.HideChangeSigns,
			ctx.Width,
			d.sideBySideSplitRatio(),
		)
	}
	d.State.SetViewport(ctx.Width, visibleEnd-visibleStart, gutterWidth)

	scrollY := d.State.ScrollY.Get()
	if d.VerticalScroll != nil {
		scrollY = d.VerticalScroll.Offset.Get()
		d.State.ScrollY.Set(scrollY)
	}
	scrollX := d.State.ScrollX.Get()
	if d.HardWrap {
		scrollX = 0
		if d.State.ScrollX.Peek() != 0 {
			d.State.ScrollX.Set(0)
		}
	}
	if scrollY < 0 {
		scrollY = 0
	}
	if scrollX < 0 {
		scrollX = 0
	}

	if d.LayoutMode == DiffLayoutSideBySide {
		d.renderSideBySide(ctx, sideBySide, visibleStart, visibleEnd, scrollY, scrollX)
		return
	}

	wrapWidth := max(1, ctx.Width-gutterWidth)
	for row := visibleStart; row < visibleEnd; row++ {
		contentRow := row
		if d.VerticalScroll == nil {
			contentRow = scrollY + row
		}

		var line RenderedDiffLine
		lineIdx := contentRow
		contentScrollX := scrollX
		continuation := false
		if d.HardWrap {
			var wrapRow int
			var ok bool
			lineIdx, line, wrapRow, ok = wrappedLineAtRowWithIndex(rendered.Lines, wrapWidth, contentRow)
			if !ok {
				continue
			}
			contentScrollX = wrapRow * wrapWidth
			continuation = wrapRow > 0
		} else {
			if contentRow < 0 || contentRow >= len(rendered.Lines) {
				continue
			}
			line = rendered.Lines[contentRow]
			contentScrollX = horizontalScrollXForLine(line.Kind, contentScrollX)
		}

		if lineStyle, ok := d.Palette.LineStyleForKind(line.Kind); ok && lineStyle.BackgroundColor != nil && lineStyle.BackgroundColor.IsSet() {
			bg := lineStyle.BackgroundColor.ColorAt(ctx.Width, 1, 0, 0)
			ctx.FillRect(0, row, ctx.Width, 1, bg)
		}
		if gutterStyle, ok := d.Palette.GutterStyleForKind(line.Kind); ok && gutterStyle.BackgroundColor != nil && gutterStyle.BackgroundColor.IsSet() {
			gutterBg := gutterStyle.BackgroundColor.ColorAt(gutterWidth, 1, 0, 0)
			gutterCols := gutterWidth
			if gutterCols > ctx.Width {
				gutterCols = ctx.Width
			}
			if gutterCols > 0 {
				ctx.FillRect(0, row, gutterCols, 1, gutterBg)
			}
		}
		gutterLine := line
		if continuation {
			gutterLine.OldLine = 0
			gutterLine.NewLine = 0
			gutterLine.Prefix = " "
		}
		d.renderGutterLine(ctx, rendered, row, gutterLine)
		d.renderContentLine(ctx, row, lineIdx, gutterWidth, line, contentScrollX)
	}
}

func (d DiffView) renderSideBySide(ctx *t.RenderContext, sideBySide *SideBySideRenderedFile, visibleStart int, visibleEnd int, scrollY int, scrollX int) {
	if sideBySide == nil {
		return
	}

	panes := sideBySidePaneLayout(ctx.Width, sideBySide, d.HideChangeSigns, d.sideBySideSplitRatio())
	for row := visibleStart; row < visibleEnd; row++ {
		contentRow := row
		if d.VerticalScroll == nil {
			contentRow = scrollY + row
		}

		var line SideBySideRenderedRow
		wrapRow := 0
		rowIdx := contentRow
		ok := false
		if d.HardWrap {
			rowIdx, line, wrapRow, ok = wrappedSideRowAtRowWithIndex(sideBySide.Rows, panes, ctx.Width, contentRow)
		} else if contentRow >= 0 && contentRow < len(sideBySide.Rows) {
			line = sideBySide.Rows[contentRow]
			ok = true
		}
		if !ok {
			continue
		}

		if line.Shared != nil {
			d.renderSideSharedRow(ctx, row, rowIdx, *line.Shared, wrapRow, scrollX)
			continue
		}

		d.renderSidePairedRow(ctx, row, rowIdx, panes, sideBySide, line, wrapRow, scrollX)
	}

	if d.State != nil && d.State.SideDividerOverlayVisible() {
		d.renderSideDividerSizeOverlay(ctx, panes, d.sideDividerOverlayRow(visibleStart, visibleEnd))
	}
}

func (d DiffView) sideDividerOverlayRow(visibleStart int, visibleEnd int) int {
	if visibleEnd <= visibleStart {
		return visibleStart
	}
	return visibleStart + (visibleEnd-visibleStart)/3
}

func (d DiffView) renderSideDividerSizeOverlay(ctx *t.RenderContext, panes sidePaneLayout, row int) {
	if panes.DividerWidth <= 0 || row < 0 || row >= ctx.Height {
		return
	}

	leftText, leftX, rightText, rightX := sideDividerSizeOverlayLayout(panes, ctx.Width)
	overlayStyle := d.sideDividerSizeOverlayStyle()

	if leftText != "" {
		ctx.DrawStyledText(leftX, row, leftText, overlayStyle)
	}
	if panes.DividerX >= 0 && panes.DividerX < ctx.Width {
		ctx.DrawStyledText(panes.DividerX, row, sideDividerRune, overlayStyle)
	}
	if rightText != "" {
		ctx.DrawStyledText(rightX, row, rightText, overlayStyle)
	}
}

func (d DiffView) sideDividerSizeOverlayStyle() t.Style {
	overlayFg := t.BrightWhite
	overlayBg := t.Black.WithAlpha(0.7)
	if theme, ok := t.GetTheme(t.CurrentThemeName()); ok {
		overlayFg = theme.SecondaryText
		overlayBg = theme.SecondaryBg
	}
	return t.Style{
		ForegroundColor: overlayFg,
		BackgroundColor: overlayBg,
		Bold:            true,
	}
}

func sideDividerSizeOverlayLayout(panes sidePaneLayout, viewportWidth int) (leftText string, leftX int, rightText string, rightX int) {
	if viewportWidth <= 0 {
		return "", 0, "", 0
	}

	leftNumber := strconv.Itoa(max(0, panes.LeftPaneWidth))
	rightText = strconv.Itoa(max(0, panes.RightPaneWidth))

	availableLeft := panes.DividerX
	if availableLeft <= 0 {
		leftText = ""
		leftX = panes.DividerX
	} else {
		// Keep the rightmost digits visible when space is constrained.
		digitSlots := availableLeft
		usePadding := availableLeft >= 2
		if usePadding {
			digitSlots = availableLeft - 1
		}
		if digitSlots > len(leftNumber) {
			digitSlots = len(leftNumber)
		}
		if digitSlots <= 0 {
			leftText = ""
		} else {
			leftText = leftNumber[len(leftNumber)-digitSlots:]
		}
		if leftText != "" {
			withArrow := "← " + leftText
			required := ansi.StringWidth(withArrow)
			if usePadding {
				required++
			}
			if required <= availableLeft {
				leftText = withArrow
			}
		}
		if usePadding && leftText != "" {
			leftText += " "
		}
		leftX = panes.DividerX - ansi.StringWidth(leftText)
	}

	rightX = panes.DividerX + panes.DividerWidth
	if rightX >= viewportWidth {
		rightText = ""
		return leftText, leftX, rightText, rightX
	}
	maxRightChars := viewportWidth - rightX
	if maxRightChars <= 0 {
		rightText = ""
	} else {
		withArrow := rightText + " →"
		if ansi.StringWidth(withArrow) <= maxRightChars {
			rightText = withArrow
		} else if len(rightText) > maxRightChars {
			rightText = rightText[:maxRightChars]
		}
	}

	return leftText, leftX, rightText, rightX
}

func (d DiffView) renderSideSharedRow(ctx *t.RenderContext, row int, rowIdx int, line RenderedDiffLine, wrapRow int, scrollX int) {
	if lineStyle, ok := d.Palette.LineStyleForKind(line.Kind); ok && lineStyle.BackgroundColor != nil && lineStyle.BackgroundColor.IsSet() {
		bg := lineStyle.BackgroundColor.ColorAt(ctx.Width, 1, 0, 0)
		ctx.FillRect(0, row, ctx.Width, 1, bg)
	}

	contentScrollX := scrollX
	if d.HardWrap {
		contentScrollX = wrapRow * max(1, ctx.Width)
	} else {
		contentScrollX = horizontalScrollXForLine(line.Kind, contentScrollX)
	}
	selectionStart, selectionEnd, hasSelection := d.State.SelectionRangeForSideRow(rowIdx, DiffSelectionLaneShared)
	d.renderSegments(ctx, row, 0, ctx.Width, line.Segments, contentScrollX, selectionStart, selectionEnd, hasSelection)
}

func (d DiffView) renderSidePairedRow(ctx *t.RenderContext, row int, rowIdx int, panes sidePaneLayout, sideBySide *SideBySideRenderedFile, line SideBySideRenderedRow, wrapRow int, scrollX int) {
	d.renderSideCell(
		ctx,
		row,
		rowIdx,
		panes.LeftPaneX,
		panes.LeftPaneWidth,
		panes.LeftGutterWidth,
		max(1, sideNumWidthForPane(true, sideBySide)),
		line.Left,
		true,
		wrapRow,
		scrollX,
	)
	d.renderSideCell(
		ctx,
		row,
		rowIdx,
		panes.RightPaneX,
		panes.RightPaneWidth,
		panes.RightGutterWidth,
		max(1, sideNumWidthForPane(false, sideBySide)),
		line.Right,
		false,
		wrapRow,
		scrollX,
	)
	d.renderSideDivider(ctx, row, panes, line)
}

func (d DiffView) renderSideDivider(ctx *t.RenderContext, row int, panes sidePaneLayout, line SideBySideRenderedRow) {
	if !shouldRenderSideDivider(line) {
		return
	}
	if panes.DividerWidth <= 0 {
		return
	}
	x := panes.DividerX
	if x < 0 || x >= ctx.Width {
		return
	}

	runeText := sideDividerRune
	if line.Right == nil {
		runeText = sideEmptyHatchRune
	}

	style, ok := d.sideDividerStyle(line)
	if !ok {
		ctx.DrawText(x, row, runeText)
		return
	}
	ctx.DrawStyledText(x, row, runeText, style)
}

func shouldRenderSideDivider(line SideBySideRenderedRow) bool {
	return line.Right != nil || line.Left != nil
}

func (d DiffView) sideDividerStyle(line SideBySideRenderedRow) (t.Style, bool) {
	if line.Right == nil {
		style := d.styleForRole(TokenRoleDiffHatch)
		if d.State != nil && d.State.SideDividerDragging() && style.ForegroundColor != nil && style.ForegroundColor.IsSet() {
			boosted := style.ForegroundColor.ColorAt(1, 1, 0, 0).WithAlpha(0.95)
			style.ForegroundColor = boosted
		}
		return style, true
	}

	role, kind, ok := sideDividerLineNumberRole(line)
	if !ok {
		return t.Style{}, false
	}
	span, ok := d.Palette.StyleForRole(role)
	if !ok || !span.Foreground.IsSet() {
		return t.Style{}, false
	}

	dragging := d.State != nil && d.State.SideDividerDragging()
	fg := span.Foreground
	alphaFactor := 0.24
	if dragging {
		alphaFactor = 0.95
	}
	fg = fg.WithAlpha(fg.Alpha() * alphaFactor)
	style := t.Style{ForegroundColor: fg}

	if gutterStyle, ok := d.Palette.GutterStyleForKind(kind); ok && gutterStyle.BackgroundColor != nil && gutterStyle.BackgroundColor.IsSet() {
		style.BackgroundColor = gutterStyle.BackgroundColor
	}
	return style, true
}

func sideDividerLineNumberRole(line SideBySideRenderedRow) (TokenRole, RenderedLineKind, bool) {
	if line.Right != nil {
		return sideLineNumberRole(line.Right.Kind, false), line.Right.Kind, true
	}
	if line.Left != nil {
		return sideLineNumberRole(line.Left.Kind, true), line.Left.Kind, true
	}
	return TokenRoleOldLineNumber, RenderedLineContext, false
}

func (d DiffView) renderSideCell(ctx *t.RenderContext, row int, rowIdx int, paneX int, paneWidth int, gutterWidth int, numWidth int, cell *RenderedSideCell, isLeft bool, wrapRow int, scrollX int) {
	if paneWidth <= 0 {
		return
	}

	if cell != nil {
		if lineStyle, ok := d.Palette.LineStyleForKind(cell.Kind); ok && lineStyle.BackgroundColor != nil && lineStyle.BackgroundColor.IsSet() {
			bg := lineStyle.BackgroundColor.ColorAt(paneWidth, 1, 0, 0)
			ctx.FillRect(paneX, row, paneWidth, 1, bg)
		}
	}

	gutterCols := gutterWidth
	if gutterCols > paneWidth {
		gutterCols = paneWidth
	}
	if gutterCols > 0 && cell != nil {
		if gutterStyle, ok := d.Palette.GutterStyleForKind(cell.Kind); ok && gutterStyle.BackgroundColor != nil && gutterStyle.BackgroundColor.IsSet() {
			gutterBg := gutterStyle.BackgroundColor.ColorAt(gutterCols, 1, 0, 0)
			ctx.FillRect(paneX, row, gutterCols, 1, gutterBg)
		}
	}

	if cell == nil {
		d.renderSideEmptyCellHatch(ctx, row, paneX, paneWidth)
		return
	}

	visibleWidth := paneWidth - gutterWidth
	if visibleWidth <= 0 {
		visibleWidth = 1
	}
	cellRows := 1
	if d.HardWrap {
		cellRows = wrappedSideCellRowCount(cell, visibleWidth)
	}
	if wrapRow >= cellRows {
		return
	}

	continuation := wrapRow > 0
	number := cell.LineNumber
	prefix := cell.Prefix
	if continuation {
		number = 0
		prefix = " "
	}

	x := paneX
	if x < paneX+paneWidth {
		d.drawText(ctx, x, row, lineNumberText(number, numWidth), sideLineNumberRole(cell.Kind, isLeft))
	}
	x += numWidth
	if x < paneX+paneWidth {
		ctx.DrawText(x, row, " ")
	}
	x++
	if !d.HideChangeSigns {
		if x < paneX+paneWidth {
			role := TokenRoleDiffPrefixContext
			if prefixRole, ok := prefixRoleForLine(cell.Kind); ok {
				role = prefixRole
			}
			d.drawText(ctx, x, row, displayLinePrefix(RenderedDiffLine{Kind: cell.Kind, Prefix: prefix}, d.HideChangeSigns), role)
		}
		x++
		if x < paneX+paneWidth {
			ctx.DrawText(x, row, " ")
		}
	}

	contentScrollX := scrollX
	if d.HardWrap {
		contentScrollX = wrapRow * visibleWidth
	}
	selectionStart, selectionEnd, hasSelection := d.State.SelectionRangeForSideRow(rowIdx, laneForSideCell(isLeft))
	d.renderSegments(ctx, row, paneX+gutterWidth, paneWidth-gutterWidth, cell.Segments, contentScrollX, selectionStart, selectionEnd, hasSelection)
}

func (d DiffView) renderSideEmptyCellHatch(ctx *t.RenderContext, row int, startX int, width int) {
	if width <= 0 {
		return
	}
	style := d.styleForRole(TokenRoleDiffHatch)
	for col := 0; col < width; col++ {
		x := startX + col
		if x < 0 || x >= ctx.Width {
			continue
		}
		ctx.DrawStyledText(x, row, sideEmptyHatchRune, style)
	}
}

func sideLineNumberRole(kind RenderedLineKind, isLeft bool) TokenRole {
	switch kind {
	case RenderedLineAdd:
		return TokenRoleLineNumberAdd
	case RenderedLineRemove:
		return TokenRoleLineNumberRemove
	default:
		if isLeft {
			return TokenRoleOldLineNumber
		}
		return TokenRoleNewLineNumber
	}
}

func sideNumWidthForPane(left bool, sideBySide *SideBySideRenderedFile) int {
	if sideBySide == nil {
		return 1
	}
	if left {
		return sideBySide.LeftNumWidth
	}
	return sideBySide.RightNumWidth
}

func (d DiffView) renderGutterLine(ctx *t.RenderContext, rendered *RenderedFile, row int, line RenderedDiffLine) {
	oldNum := lineNumberText(line.OldLine, rendered.OldNumWidth)
	newNum := lineNumberText(line.NewLine, rendered.NewNumWidth)
	oldNumRole, newNumRole := lineNumberRolesForLine(line.Kind)

	x := 0
	if x < ctx.Width {
		d.drawText(ctx, x, row, oldNum, oldNumRole)
	}
	x += rendered.OldNumWidth
	if x < ctx.Width {
		ctx.DrawText(x, row, " ")
	}
	x++
	if x < ctx.Width {
		d.drawText(ctx, x, row, newNum, newNumRole)
	}
	x += rendered.NewNumWidth
	if x < ctx.Width {
		ctx.DrawText(x, row, " ")
	}
	x++

	if !d.HideChangeSigns {
		prefixRole := TokenRoleDiffPrefixContext
		if role, ok := prefixRoleForLine(line.Kind); ok {
			prefixRole = role
		}
		if x < ctx.Width {
			prefix := displayLinePrefix(line, d.HideChangeSigns)
			d.drawText(ctx, x, row, prefix, prefixRole)
		}
		x++
		if x < ctx.Width {
			ctx.DrawText(x, row, " ")
		}
	}
}

func lineNumberRolesForLine(kind RenderedLineKind) (oldRole TokenRole, newRole TokenRole) {
	oldRole = TokenRoleOldLineNumber
	newRole = TokenRoleNewLineNumber
	switch kind {
	case RenderedLineAdd:
		return TokenRoleLineNumberAdd, TokenRoleLineNumberAdd
	case RenderedLineRemove:
		return TokenRoleLineNumberRemove, TokenRoleLineNumberRemove
	default:
		return oldRole, newRole
	}
}

func horizontalScrollXForLine(kind RenderedLineKind, scrollX int) int {
	if kind == RenderedLineHunkHeader {
		return 0
	}
	return scrollX
}

func displayLinePrefix(line RenderedDiffLine, hideChangeSigns bool) string {
	if hideChangeSigns {
		switch line.Kind {
		case RenderedLineAdd, RenderedLineRemove:
			return " "
		}
	}
	prefix := line.Prefix
	if prefix == "" {
		return " "
	}
	return prefix
}

func (d DiffView) renderContentLine(ctx *t.RenderContext, row int, lineIdx int, gutterWidth int, line RenderedDiffLine, scrollX int) {
	if gutterWidth >= ctx.Width {
		return
	}

	visibleWidth := ctx.Width - gutterWidth
	if visibleWidth <= 0 {
		return
	}

	selectionStart, selectionEnd, hasSelection := d.State.SelectionRangeForUnifiedLine(lineIdx)
	d.renderSegments(ctx, row, gutterWidth, visibleWidth, line.Segments, scrollX, selectionStart, selectionEnd, hasSelection)
}

func (d DiffView) renderSegments(ctx *t.RenderContext, row int, startX int, visibleWidth int, segments []RenderedSegment, scrollX int, selectionStart int, selectionEnd int, hasSelection bool) {
	if row < 0 || row >= ctx.Height || visibleWidth <= 0 {
		return
	}

	contentCol := 0
	graphemeIdx := 0
	hasContentGrapheme := false
	for _, segment := range segments {
		if segment.Text == "" {
			continue
		}
		style := d.styleForSegment(segment)
		remaining := segment.Text
		for len(remaining) > 0 {
			grapheme, width := ansi.FirstGraphemeCluster(remaining, ansi.GraphemeWidth)
			if grapheme == "" {
				break
			}
			if width <= 0 {
				width = ansi.StringWidth(grapheme)
			}
			if width <= 0 {
				width = 1
			}
			hasContentGrapheme = true

			nextCol := contentCol + width
			if nextCol <= scrollX {
				contentCol = nextCol
				remaining = remaining[len(grapheme):]
				graphemeIdx++
				continue
			}
			if contentCol >= scrollX+visibleWidth {
				return
			}

			drawX := startX + (contentCol - scrollX)
			if drawX >= startX && drawX < startX+visibleWidth {
				drawStyle := style
				if hasSelection && selectionContainsGrapheme(graphemeIdx, selectionStart, selectionEnd) {
					drawStyle.BackgroundColor = d.Palette.SelectionBackground()
				}
				if width == 1 || (drawX+width) <= startX+visibleWidth {
					ctx.DrawStyledText(drawX, row, grapheme, drawStyle)
				}
			}

			contentCol = nextCol
			remaining = remaining[len(grapheme):]
			graphemeIdx++
		}
	}

	if hasContentGrapheme || !selectionCrossesEmptyLine(selectionStart, selectionEnd) {
		return
	}

	indicatorStyle := d.styleForRole(TokenRoleSyntaxPlain)
	indicatorStyle.ForegroundColor = d.Palette.SelectionBackground()
	indicatorStyle.BackgroundColor = nil
	ctx.DrawStyledText(startX, row, emptyLineSelectionRune, indicatorStyle)
}

func (d DiffView) styleForSegment(segment RenderedSegment) t.Style {
	style := d.styleForRole(segment.Role)
	overlay, ok := d.Palette.IntralineOverlayStyle(segment.Intraline, d.IntralineStyle)
	if !ok {
		return style
	}
	return applyIntralineOverlay(style, overlay)
}

func (d DiffView) drawText(ctx *t.RenderContext, x int, y int, value string, role TokenRole) {
	if x >= ctx.Width || y < 0 || y >= ctx.Height || value == "" {
		return
	}
	ctx.DrawStyledText(x, y, value, d.styleForRole(role))
}

func (d DiffView) styleForRole(role TokenRole) t.Style {
	span, ok := d.Palette.StyleForRole(role)
	if !ok {
		return t.Style{}
	}
	return styleFromSpanStyle(span)
}

func applyIntralineOverlay(base t.Style, overlay t.SpanStyle) t.Style {
	if overlay.Foreground.IsSet() {
		base.ForegroundColor = overlay.Foreground
	}
	if overlay.Background.IsSet() {
		base.BackgroundColor = overlay.Background
	}
	if overlay.Underline != t.UnderlineNone {
		base.Underline = overlay.Underline
	}
	if overlay.UnderlineColor.IsSet() {
		base.UnderlineColor = overlay.UnderlineColor
	}
	if overlay.Bold {
		base.Bold = true
	}
	if overlay.Faint {
		base.Faint = true
	}
	if overlay.Italic {
		base.Italic = true
	}
	if overlay.Blink {
		base.Blink = true
	}
	if overlay.Reverse {
		base.Reverse = true
	}
	if overlay.Conceal {
		base.Conceal = true
	}
	if overlay.Strikethrough {
		base.Strikethrough = true
	}
	base = applyIntralineReadabilityFilter(base, overlay)
	return base
}

const (
	intralineForegroundReadabilityFloor      = 3.0
	intralineForegroundReadabilityIterations = 10
)

func applyIntralineReadabilityFilter(style t.Style, overlay t.SpanStyle) t.Style {
	if !overlay.Background.IsSet() {
		return style
	}
	if style.ForegroundColor == nil || !style.ForegroundColor.IsSet() {
		return style
	}
	if style.BackgroundColor == nil || !style.BackgroundColor.IsSet() {
		return style
	}

	fg := style.ForegroundColor.ColorAt(1, 1, 0, 0)
	bg := style.BackgroundColor.ColorAt(1, 1, 0, 0)
	style.ForegroundColor = blendForegroundTowardReadability(fg, bg, intralineForegroundReadabilityFloor)
	return style
}

func blendForegroundTowardReadability(fg t.Color, bg t.Color, minContrast float64) t.Color {
	if !fg.IsSet() || !bg.IsSet() || minContrast <= 0 {
		return fg
	}

	currentContrast := fg.ContrastRatio(bg)
	if currentContrast >= minContrast {
		return fg
	}

	readableTarget := bg.AutoText()
	if !readableTarget.IsSet() {
		return fg
	}
	targetContrast := readableTarget.ContrastRatio(bg)
	if targetContrast <= currentContrast {
		return fg
	}
	if targetContrast < minContrast {
		return readableTarget
	}

	low := 0.0
	high := 1.0
	best := readableTarget
	for range intralineForegroundReadabilityIterations {
		mid := (low + high) / 2
		candidate := fg.Blend(readableTarget, mid)
		if candidate.ContrastRatio(bg) >= minContrast {
			best = candidate
			high = mid
		} else {
			low = mid
		}
	}

	return best
}

func styleFromSpanStyle(span t.SpanStyle) t.Style {
	style := t.Style{
		Bold:           span.Bold,
		Faint:          span.Faint,
		Italic:         span.Italic,
		Underline:      span.Underline,
		UnderlineColor: span.UnderlineColor,
		Blink:          span.Blink,
		Reverse:        span.Reverse,
		Conceal:        span.Conceal,
		Strikethrough:  span.Strikethrough,
	}
	if span.Foreground.IsSet() {
		style.ForegroundColor = span.Foreground
	}
	if span.Background.IsSet() {
		style.BackgroundColor = span.Background
	}
	return style
}

func (d DiffView) sideBySideSplitRatio() float64 {
	if d.State == nil {
		return 0.5
	}
	return d.State.SideBySideSplitRatio()
}

func (d DiffView) clampSideBySideHorizontalScroll(viewportWidth int, sideBySide *SideBySideRenderedFile) {
	if d.State == nil {
		return
	}
	gutterWidth := sideBySideStateGutterWidth(
		d.State.Rendered.Get(),
		sideBySide,
		d.HideChangeSigns,
		viewportWidth,
		d.sideBySideSplitRatio(),
	)
	d.State.Clamp(gutterWidth)
}

func (d DiffView) currentRendered() *RenderedFile {
	if d.State == nil {
		return nil
	}
	return d.State.Rendered.Get()
}

func (d DiffView) currentSideBySide() *SideBySideRenderedFile {
	if d.State == nil {
		return nil
	}
	return d.State.SideBySide.Get()
}

func renderedGutterWidth(rendered *RenderedFile, hideChangeSigns bool) int {
	if rendered == nil {
		if hideChangeSigns {
			return 4
		}
		return 6
	}
	oldWidth := rendered.OldNumWidth
	if oldWidth <= 0 {
		oldWidth = 1
	}
	newWidth := rendered.NewNumWidth
	if newWidth <= 0 {
		newWidth = 1
	}
	width := oldWidth + 1 + newWidth + 1
	if !hideChangeSigns {
		width += 2
	}
	return width
}

func wrappedContentHeight(lines []RenderedDiffLine, wrapWidth int) int {
	if len(lines) == 0 {
		return 1
	}
	total := 0
	for _, line := range lines {
		total += wrappedLineRowCount(line, wrapWidth)
	}
	if total <= 0 {
		return 1
	}
	return total
}

func wrappedLineAtRow(lines []RenderedDiffLine, wrapWidth int, rowIdx int) (RenderedDiffLine, int, bool) {
	_, line, wrapRow, ok := wrappedLineAtRowWithIndex(lines, wrapWidth, rowIdx)
	return line, wrapRow, ok
}

func wrappedLineAtRowWithIndex(lines []RenderedDiffLine, wrapWidth int, rowIdx int) (lineIdx int, line RenderedDiffLine, wrapRow int, ok bool) {
	if rowIdx < 0 {
		return 0, RenderedDiffLine{}, 0, false
	}
	remaining := rowIdx
	for idx, line := range lines {
		rows := wrappedLineRowCount(line, wrapWidth)
		if remaining < rows {
			return idx, line, remaining, true
		}
		remaining -= rows
	}
	return 0, RenderedDiffLine{}, 0, false
}

func wrappedLineRowCount(line RenderedDiffLine, wrapWidth int) int {
	if wrapWidth <= 0 {
		return 1
	}
	if line.ContentWidth <= 0 {
		return 1
	}
	rows := (line.ContentWidth + wrapWidth - 1) / wrapWidth
	if rows <= 0 {
		return 1
	}
	return rows
}

func toLayoutInsets(in t.EdgeInsets) layout.EdgeInsets {
	return layout.EdgeInsets{
		Top:    in.Top,
		Right:  in.Right,
		Bottom: in.Bottom,
		Left:   in.Left,
	}
}

func dimensionSetToMinMax(ds t.DimensionSet, padding layout.EdgeInsets, border layout.EdgeInsets) (minW int, maxW int, minH int, maxH int) {
	explicitMinW := dimensionToCells(ds.MinWidth)
	explicitMaxW := dimensionToCells(ds.MaxWidth)
	explicitMinH := dimensionToCells(ds.MinHeight)
	explicitMaxH := dimensionToCells(ds.MaxHeight)

	if ds.Width.IsCells() {
		width := clampFixedDimension(ds.Width.CellsValue(), explicitMinW, explicitMaxW)
		minW, maxW = width, width
	} else {
		minW, maxW = explicitMinW, explicitMaxW
	}
	if ds.Height.IsCells() {
		height := clampFixedDimension(ds.Height.CellsValue(), explicitMinH, explicitMaxH)
		minH, maxH = height, height
	} else {
		minH, maxH = explicitMinH, explicitMaxH
	}

	hInset := padding.Horizontal() + border.Horizontal()
	vInset := padding.Vertical() + border.Vertical()

	if minW > 0 {
		minW += hInset
	}
	if maxW > 0 {
		maxW += hInset
	}
	if minH > 0 {
		minH += vInset
	}
	if maxH > 0 {
		maxH += vInset
	}
	return minW, maxW, minH, maxH
}

func dimensionToCells(dim t.Dimension) int {
	if dim.IsCells() {
		return dim.CellsValue()
	}
	return 0
}

func clampFixedDimension(value int, minValue int, maxValue int) int {
	if minValue > 0 && maxValue > 0 && maxValue < minValue {
		return minValue
	}
	if minValue > 0 && value < minValue {
		value = minValue
	}
	if maxValue > 0 && value > maxValue {
		value = maxValue
	}
	return value
}

func lineText(line RenderedDiffLine) string {
	if len(line.Segments) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, segment := range line.Segments {
		builder.WriteString(segment.Text)
	}
	return builder.String()
}

func diffCellText(cell *RenderedSideCell) string {
	if cell == nil || len(cell.Segments) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, segment := range cell.Segments {
		builder.WriteString(segment.Text)
	}
	return builder.String()
}

func graphemeIndexForDisplayColumn(text string, displayCol int) int {
	if displayCol <= 0 {
		return 0
	}

	graphemeIdx := 0
	col := 0
	for remaining := text; len(remaining) > 0; graphemeIdx++ {
		grapheme, width := ansi.FirstGraphemeCluster(remaining, ansi.GraphemeWidth)
		if grapheme == "" {
			break
		}
		if width <= 0 {
			width = ansi.StringWidth(grapheme)
		}
		if width <= 0 {
			width = 1
		}
		nextCol := col + width
		if displayCol < nextCol {
			if displayCol-col >= width/2 {
				return graphemeIdx + 1
			}
			return graphemeIdx
		}
		if displayCol == nextCol {
			return graphemeIdx + 1
		}
		col = nextCol
		remaining = remaining[len(grapheme):]
	}
	return graphemeIdx
}

func selectionContainsGrapheme(graphemeIdx int, selectionStart int, selectionEnd int) bool {
	if selectionEnd >= 0 {
		return graphemeIdx >= selectionStart && graphemeIdx < selectionEnd
	}
	return graphemeIdx >= selectionStart
}

func selectionCrossesEmptyLine(selectionStart int, selectionEnd int) bool {
	if selectionStart != 0 {
		return false
	}
	return selectionEnd != 0
}

func laneForSideCell(isLeft bool) DiffSelectionLane {
	if isLeft {
		return DiffSelectionLaneLeft
	}
	return DiffSelectionLaneRight
}

func clampInt(value int, minValue int, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
