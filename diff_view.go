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
	Palette         ThemePalette
	Width           t.Dimension
	Height          t.Dimension
	Style           t.Style
}

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
	if d.State == nil || d.LayoutMode != DiffLayoutSideBySide || event.Button != uv.MouseLeft {
		return
	}

	sideBySide := d.currentSideBySide()
	if sideBySide == nil {
		return
	}

	viewportWidth := d.State.ViewportWidth()
	if viewportWidth <= 0 {
		return
	}

	panes := sideBySidePaneLayout(viewportWidth, sideBySide, d.HideChangeSigns, d.sideBySideSplitRatio())
	if panes.DividerWidth <= 0 {
		return
	}
	if event.LocalX < panes.DividerX || event.LocalX >= panes.DividerX+panes.DividerWidth {
		return
	}

	d.State.StartSideDividerDrag(event.LocalX, panes.DividerX)
}

func (d DiffView) OnMouseMove(event t.MouseEvent) {
	if d.State == nil || !d.State.SideDividerDragging() {
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

func (d DiffView) OnMouseUp(event t.MouseEvent) {
	if d.State == nil {
		return
	}
	wasDragging := d.State.SideDividerDragging()
	d.State.StopSideDividerDrag()
	if wasDragging {
		d.State.MarkSideDividerResized()
	}
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
	if rowIdx < 0 {
		return SideBySideRenderedRow{}, 0, false
	}
	remaining := rowIdx
	for _, row := range rows {
		rowsForItem := wrappedSideRowCount(row, panes, fullWidth)
		if remaining < rowsForItem {
			return row, remaining, true
		}
		remaining -= rowsForItem
	}
	return SideBySideRenderedRow{}, 0, false
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
		contentScrollX := scrollX
		continuation := false
		if d.HardWrap {
			var wrapRow int
			var ok bool
			line, wrapRow, ok = wrappedLineAtRow(rendered.Lines, wrapWidth, contentRow)
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
		d.renderContentLine(ctx, row, gutterWidth, line, contentScrollX)
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
		ok := false
		if d.HardWrap {
			line, wrapRow, ok = wrappedSideRowAtRow(sideBySide.Rows, panes, ctx.Width, contentRow)
		} else if contentRow >= 0 && contentRow < len(sideBySide.Rows) {
			line = sideBySide.Rows[contentRow]
			ok = true
		}
		if !ok {
			continue
		}

		if line.Shared != nil {
			d.renderSideSharedRow(ctx, row, *line.Shared, wrapRow, scrollX)
			continue
		}

		d.renderSidePairedRow(ctx, row, panes, sideBySide, line, wrapRow, scrollX)
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

func (d DiffView) renderSideSharedRow(ctx *t.RenderContext, row int, line RenderedDiffLine, wrapRow int, scrollX int) {
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
	d.renderSegments(ctx, row, 0, ctx.Width, line.Segments, contentScrollX)
}

func (d DiffView) renderSidePairedRow(ctx *t.RenderContext, row int, panes sidePaneLayout, sideBySide *SideBySideRenderedFile, line SideBySideRenderedRow, wrapRow int, scrollX int) {
	d.renderSideCell(
		ctx,
		row,
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

func (d DiffView) renderSideCell(ctx *t.RenderContext, row int, paneX int, paneWidth int, gutterWidth int, numWidth int, cell *RenderedSideCell, isLeft bool, wrapRow int, scrollX int) {
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
	d.renderSegments(ctx, row, paneX+gutterWidth, paneWidth-gutterWidth, cell.Segments, contentScrollX)
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

func (d DiffView) renderContentLine(ctx *t.RenderContext, row int, gutterWidth int, line RenderedDiffLine, scrollX int) {
	if gutterWidth >= ctx.Width {
		return
	}

	visibleWidth := ctx.Width - gutterWidth
	if visibleWidth <= 0 {
		return
	}

	d.renderSegments(ctx, row, gutterWidth, visibleWidth, line.Segments, scrollX)
}

func (d DiffView) renderSegments(ctx *t.RenderContext, row int, startX int, visibleWidth int, segments []RenderedSegment, scrollX int) {
	if row < 0 || row >= ctx.Height || visibleWidth <= 0 {
		return
	}

	contentCol := 0
	for _, segment := range segments {
		if segment.Text == "" {
			continue
		}
		style := d.styleForRole(segment.Role)
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

			nextCol := contentCol + width
			if nextCol <= scrollX {
				contentCol = nextCol
				remaining = remaining[len(grapheme):]
				continue
			}
			if contentCol >= scrollX+visibleWidth {
				return
			}

			drawX := startX + (contentCol - scrollX)
			if drawX >= startX && drawX < startX+visibleWidth {
				if width == 1 || (drawX+width) <= startX+visibleWidth {
					ctx.DrawStyledText(drawX, row, grapheme, style)
				}
			}

			contentCol = nextCol
			remaining = remaining[len(grapheme):]
		}
	}
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
		d.State.Rendered.Peek(),
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
	return d.State.Rendered.Peek()
}

func (d DiffView) currentSideBySide() *SideBySideRenderedFile {
	if d.State == nil {
		return nil
	}
	return d.State.SideBySide.Peek()
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
	if rowIdx < 0 {
		return RenderedDiffLine{}, 0, false
	}
	remaining := rowIdx
	for _, line := range lines {
		rows := wrappedLineRowCount(line, wrapWidth)
		if remaining < rows {
			return line, remaining, true
		}
		remaining -= rows
	}
	return RenderedDiffLine{}, 0, false
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

func clampInt(value int, minValue int, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
