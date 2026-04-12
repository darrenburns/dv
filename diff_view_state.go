package main

import (
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/x/ansi"
	t "github.com/darrenburns/terma"
)

const (
	sideDividerOverlayHoldDuration = 1 * time.Second
	selectionAutoScrollInterval    = 50 * time.Millisecond
)

type DiffSelectionTrack int

const (
	DiffSelectionTrackNone DiffSelectionTrack = iota
	DiffSelectionTrackUnified
	DiffSelectionTrackLeft
	DiffSelectionTrackRight
	DiffSelectionTrackShared
)

type DiffSelectionLane int

const (
	DiffSelectionLaneNone DiffSelectionLane = iota
	DiffSelectionLaneUnified
	DiffSelectionLaneShared
	DiffSelectionLaneLeft
	DiffSelectionLaneRight
)

type DiffSelectionPoint struct {
	Row      int
	Grapheme int
	Lane     DiffSelectionLane
}

// DiffViewState tracks scroll state and rendered diff content for DiffView.
type DiffViewState struct {
	ScrollY    t.Signal[int]
	ScrollX    t.Signal[int]
	Rendered   t.AnySignal[*RenderedFile]
	SideBySide t.AnySignal[*SideBySideRenderedFile]
	SplitRatio t.Signal[float64]

	viewportWidth  int
	viewportHeight int

	selectionTrack    t.Signal[DiffSelectionTrack]
	selectionAnchor   t.AnySignal[*DiffSelectionPoint]
	selectionCursor   t.AnySignal[*DiffSelectionPoint]
	selectionDragging t.Signal[bool]
	selectionPointerX int
	selectionPointerY int

	selectionAutoScrollMu    sync.Mutex
	selectionAutoScrollTimer *time.Timer

	sideDividerDragging     t.Signal[bool]
	sideDividerDragOffset   int
	sideDividerLastResize   t.Signal[int64]
	sideDividerOverlayPing  t.Signal[int]
	sideDividerOverlayMu    sync.Mutex
	sideDividerOverlayTimer *time.Timer
}

func NewDiffViewState(rendered *RenderedFile) *DiffViewState {
	return &DiffViewState{
		ScrollY:                t.NewSignal(0),
		ScrollX:                t.NewSignal(0),
		Rendered:               t.NewAnySignal(rendered),
		SideBySide:             t.NewAnySignal(buildSideBySideFromRendered(rendered)),
		SplitRatio:             t.NewSignal(0.5),
		selectionTrack:         t.NewSignal(DiffSelectionTrackNone),
		selectionAnchor:        t.NewAnySignal((*DiffSelectionPoint)(nil)),
		selectionCursor:        t.NewAnySignal((*DiffSelectionPoint)(nil)),
		selectionDragging:      t.NewSignal(false),
		sideDividerDragging:    t.NewSignal(false),
		sideDividerLastResize:  t.NewSignal(int64(0)),
		sideDividerOverlayPing: t.NewSignal(0),
	}
}

func (s *DiffViewState) SetRendered(rendered *RenderedFile) {
	s.SetRenderedPair(rendered, buildSideBySideFromRendered(rendered))
}

func (s *DiffViewState) SetRenderedPair(rendered *RenderedFile, sideBySide *SideBySideRenderedFile) {
	if s == nil {
		return
	}
	s.Rendered.Set(rendered)
	s.SideBySide.Set(sideBySide)
	s.sideDividerDragging.Set(false)
	s.sideDividerDragOffset = 0
	s.sideDividerLastResize.Set(0)
	s.stopSideDividerOverlayTimer()
	s.ScrollY.Set(0)
	s.ScrollX.Set(0)
	s.ClearSelection()
	s.Clamp(0)
}

func (s *DiffViewState) SideBySideSplitRatio() float64 {
	if s == nil || !s.SplitRatio.IsValid() {
		return 0.5
	}
	return clampSideBySideSplitRatio(s.SplitRatio.Get())
}

func (s *DiffViewState) SetSideBySideSplitRatio(ratio float64) {
	if s == nil || !s.SplitRatio.IsValid() {
		return
	}
	s.SplitRatio.Set(clampSideBySideSplitRatio(ratio))
}

func (s *DiffViewState) StartSideDividerDrag(pointerX int, dividerX int) {
	if s == nil {
		return
	}
	s.sideDividerDragging.Set(true)
	s.sideDividerDragOffset = pointerX - dividerX
}

func (s *DiffViewState) StopSideDividerDrag() {
	if s == nil {
		return
	}
	s.sideDividerDragging.Set(false)
	s.sideDividerDragOffset = 0
}

func (s *DiffViewState) SideDividerDragging() bool {
	return s != nil && s.sideDividerDragging.Peek()
}

func (s *DiffViewState) SideDividerDragOffset() int {
	if s == nil {
		return 0
	}
	return s.sideDividerDragOffset
}

func (s *DiffViewState) MarkSideDividerResized() {
	if s == nil {
		return
	}
	s.sideDividerLastResize.Set(time.Now().UnixNano())
	s.scheduleSideDividerOverlayRefresh()
}

func (s *DiffViewState) SideDividerOverlayVisible() bool {
	return s.sideDividerOverlayVisibleAt(time.Now())
}

func (s *DiffViewState) sideDividerOverlayVisibleAt(now time.Time) bool {
	if s == nil {
		return false
	}
	if s.sideDividerDragging.Get() {
		return true
	}
	_ = s.sideDividerOverlayPing.Get()
	lastResizeAt := s.sideDividerLastResize.Get()
	if lastResizeAt <= 0 {
		return false
	}
	return now.Sub(time.Unix(0, lastResizeAt)) < sideDividerOverlayHoldDuration
}

func (s *DiffViewState) scheduleSideDividerOverlayRefresh() {
	if s == nil {
		return
	}
	s.sideDividerOverlayMu.Lock()
	defer s.sideDividerOverlayMu.Unlock()
	if s.sideDividerOverlayTimer != nil {
		s.sideDividerOverlayTimer.Stop()
	}
	s.sideDividerOverlayTimer = time.AfterFunc(sideDividerOverlayHoldDuration, func() {
		s.sideDividerOverlayPing.Update(func(v int) int { return v + 1 })
	})
}

func (s *DiffViewState) stopSideDividerOverlayTimer() {
	if s == nil {
		return
	}
	s.sideDividerOverlayMu.Lock()
	defer s.sideDividerOverlayMu.Unlock()
	if s.sideDividerOverlayTimer != nil {
		s.sideDividerOverlayTimer.Stop()
		s.sideDividerOverlayTimer = nil
	}
}

func clampSideBySideSplitRatio(ratio float64) float64 {
	if ratio < 0 {
		return 0
	}
	if ratio > 1 {
		return 1
	}
	return ratio
}

func (s *DiffViewState) SetViewport(width int, height int, gutterWidth int) {
	if s == nil {
		return
	}
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	s.viewportWidth = width
	s.viewportHeight = height
	s.Clamp(gutterWidth)
}

func (s *DiffViewState) Clamp(gutterWidth int) {
	if s == nil {
		return
	}
	maxY := s.MaxScrollY()
	maxX := s.MaxScrollX(gutterWidth)

	nextY := s.ScrollY.Peek()
	if nextY < 0 {
		nextY = 0
	} else if nextY > maxY {
		nextY = maxY
	}
	if nextY != s.ScrollY.Peek() {
		s.ScrollY.Set(nextY)
	}

	nextX := s.ScrollX.Peek()
	if nextX < 0 {
		nextX = 0
	} else if nextX > maxX {
		nextX = maxX
	}
	if nextX != s.ScrollX.Peek() {
		s.ScrollX.Set(nextX)
	}
}

func (s *DiffViewState) MoveY(delta int, gutterWidth int) {
	if s == nil {
		return
	}
	next := s.ScrollY.Peek() + delta
	if next < 0 {
		next = 0
	}
	maxY := s.MaxScrollY()
	if next > maxY {
		next = maxY
	}
	if next != s.ScrollY.Peek() {
		s.ScrollY.Set(next)
	}
	s.Clamp(gutterWidth)
}

func (s *DiffViewState) MoveX(delta int, gutterWidth int) {
	if s == nil {
		return
	}
	next := s.ScrollX.Peek() + delta
	if next < 0 {
		next = 0
	}
	maxX := s.MaxScrollX(gutterWidth)
	if next > maxX {
		next = maxX
	}
	if next != s.ScrollX.Peek() {
		s.ScrollX.Set(next)
	}
	s.Clamp(gutterWidth)
}

func (s *DiffViewState) PageUp(gutterWidth int) {
	if s == nil {
		return
	}
	s.MoveY(-s.pageStep(), gutterWidth)
}

func (s *DiffViewState) PageDown(gutterWidth int) {
	if s == nil {
		return
	}
	s.MoveY(s.pageStep(), gutterWidth)
}

func (s *DiffViewState) HalfPageUp(gutterWidth int) {
	if s == nil {
		return
	}
	s.MoveY(-s.halfPageStep(), gutterWidth)
}

func (s *DiffViewState) HalfPageDown(gutterWidth int) {
	if s == nil {
		return
	}
	s.MoveY(s.halfPageStep(), gutterWidth)
}

func (s *DiffViewState) GoTop(gutterWidth int) {
	if s == nil {
		return
	}
	if s.ScrollY.Peek() != 0 {
		s.ScrollY.Set(0)
	}
	s.Clamp(gutterWidth)
}

func (s *DiffViewState) GoBottom(gutterWidth int) {
	if s == nil {
		return
	}
	maxY := s.MaxScrollY()
	if s.ScrollY.Peek() != maxY {
		s.ScrollY.Set(maxY)
	}
	s.Clamp(gutterWidth)
}

func (s *DiffViewState) MaxScrollY() int {
	if s == nil || s.viewportHeight <= 0 {
		return 0
	}
	rendered := s.Rendered.Peek()
	if rendered == nil {
		return 0
	}
	maxScroll := len(rendered.Lines) - s.viewportHeight
	if maxScroll < 0 {
		return 0
	}
	return maxScroll
}

func (s *DiffViewState) MaxScrollX(gutterWidth int) int {
	if s == nil || s.viewportWidth <= 0 {
		return 0
	}
	maxContent := renderedMaxContentWidth(s.Rendered.Peek(), s.SideBySide.Peek())
	if maxContent <= 0 {
		return 0
	}
	codeWidth := s.viewportWidth - gutterWidth
	if codeWidth < 0 {
		codeWidth = 0
	}
	maxScroll := maxContent - codeWidth
	if maxScroll < 0 {
		return 0
	}
	return maxScroll
}

func (s *DiffViewState) ViewportWidth() int {
	if s == nil {
		return 0
	}
	return s.viewportWidth
}

func (s *DiffViewState) ViewportHeight() int {
	if s == nil {
		return 0
	}
	return s.viewportHeight
}

func (s *DiffViewState) StartSelection(track DiffSelectionTrack, point DiffSelectionPoint) {
	if s == nil {
		return
	}
	if !selectionPointCompatible(track, point) {
		s.ClearSelection()
		return
	}
	anchor := point
	cursor := point
	s.selectionTrack.Set(track)
	s.selectionAnchor.Set(&anchor)
	s.selectionCursor.Set(&cursor)
	s.selectionDragging.Set(true)
	s.selectionPointerX = 0
	s.selectionPointerY = 0
}

func (s *DiffViewState) UpdateSelection(point DiffSelectionPoint) bool {
	if s == nil {
		return false
	}
	track := s.selectionTrack.Peek()
	if !selectionPointCompatible(track, point) {
		return false
	}
	if s.selectionAnchor.Peek() == nil {
		return false
	}
	cursor := point
	s.selectionCursor.Set(&cursor)
	return true
}

func (s *DiffViewState) StopSelectionDrag() {
	if s == nil {
		return
	}
	s.selectionDragging.Set(false)
	s.stopSelectionAutoscrollTimer()
}

func (s *DiffViewState) SelectionDragging() bool {
	return s != nil && s.selectionDragging.Peek()
}

func (s *DiffViewState) SetSelectionPointer(localX int, localY int) {
	if s == nil {
		return
	}
	s.selectionAutoScrollMu.Lock()
	s.selectionPointerX = localX
	s.selectionPointerY = localY
	s.selectionAutoScrollMu.Unlock()
}

func (s *DiffViewState) SelectionPointer() (localX int, localY int) {
	if s == nil {
		return 0, 0
	}
	s.selectionAutoScrollMu.Lock()
	localX = s.selectionPointerX
	localY = s.selectionPointerY
	s.selectionAutoScrollMu.Unlock()
	return localX, localY
}

func (s *DiffViewState) ScheduleSelectionAutoscrollTick(fn func()) {
	if s == nil || fn == nil {
		return
	}
	s.selectionAutoScrollMu.Lock()
	defer s.selectionAutoScrollMu.Unlock()
	if s.selectionAutoScrollTimer != nil {
		s.selectionAutoScrollTimer.Stop()
	}
	s.selectionAutoScrollTimer = time.AfterFunc(selectionAutoScrollInterval, fn)
}

func (s *DiffViewState) StopSelectionAutoscrollTimer() {
	if s == nil {
		return
	}
	s.stopSelectionAutoscrollTimer()
}

func (s *DiffViewState) stopSelectionAutoscrollTimer() {
	if s == nil {
		return
	}
	s.selectionAutoScrollMu.Lock()
	defer s.selectionAutoScrollMu.Unlock()
	if s.selectionAutoScrollTimer != nil {
		s.selectionAutoScrollTimer.Stop()
		s.selectionAutoScrollTimer = nil
	}
}

func (s *DiffViewState) ClearSelection() {
	if s == nil {
		return
	}
	s.selectionTrack.Set(DiffSelectionTrackNone)
	s.selectionAnchor.Set(nil)
	s.selectionCursor.Set(nil)
	s.selectionDragging.Set(false)
	s.stopSelectionAutoscrollTimer()
}

func (s *DiffViewState) SelectionTrack() DiffSelectionTrack {
	if s == nil || !s.selectionTrack.IsValid() {
		return DiffSelectionTrackNone
	}
	return s.selectionTrack.Peek()
}

func (s *DiffViewState) HasSelection() bool {
	_, start, end, ok := s.normalizedSelection()
	return ok && compareDiffSelectionPoints(start, end) != 0
}

func (s *DiffViewState) SelectedText() string {
	track, start, end, ok := s.normalizedSelection()
	if !ok || compareDiffSelectionPoints(start, end) == 0 {
		return ""
	}

	switch track {
	case DiffSelectionTrackUnified:
		return s.selectedUnifiedText(start, end)
	case DiffSelectionTrackLeft, DiffSelectionTrackRight, DiffSelectionTrackShared:
		return s.selectedSideBySideText(track, start, end)
	default:
		return ""
	}
}

func (s *DiffViewState) SelectionRangeForUnifiedLine(lineIdx int) (start, end int, ok bool) {
	track, selectionStart, selectionEnd, ok := s.normalizedSelection()
	if !ok || track != DiffSelectionTrackUnified {
		return 0, 0, false
	}
	start, end = lineSelectionRange(lineIdx, selectionStart, selectionEnd)
	return start, end, true
}

func (s *DiffViewState) SelectionRangeForSideRow(rowIdx int, lane DiffSelectionLane) (start, end int, ok bool) {
	track, selectionStart, selectionEnd, ok := s.normalizedSelection()
	if !ok || track == DiffSelectionTrackUnified || track == DiffSelectionTrackNone {
		return 0, 0, false
	}
	switch track {
	case DiffSelectionTrackLeft:
		if lane != DiffSelectionLaneLeft && lane != DiffSelectionLaneShared {
			return 0, 0, false
		}
	case DiffSelectionTrackRight:
		if lane != DiffSelectionLaneRight && lane != DiffSelectionLaneShared {
			return 0, 0, false
		}
	case DiffSelectionTrackShared:
		if lane != DiffSelectionLaneShared {
			return 0, 0, false
		}
	}
	start, end = lineSelectionRange(rowIdx, selectionStart, selectionEnd)
	return start, end, true
}

func (s *DiffViewState) pageStep() int {
	if s == nil || s.viewportHeight <= 1 {
		return 1
	}
	return s.viewportHeight - 1
}

func (s *DiffViewState) halfPageStep() int {
	if s == nil || s.viewportHeight <= 1 {
		return 1
	}
	half := s.viewportHeight / 2
	if half <= 0 {
		return 1
	}
	return half
}

func (s *DiffViewState) normalizedSelection() (track DiffSelectionTrack, start DiffSelectionPoint, end DiffSelectionPoint, ok bool) {
	if s == nil {
		return DiffSelectionTrackNone, DiffSelectionPoint{}, DiffSelectionPoint{}, false
	}
	track = s.selectionTrack.Get()
	anchor := s.selectionAnchor.Get()
	cursor := s.selectionCursor.Get()
	if track == DiffSelectionTrackNone || anchor == nil || cursor == nil {
		return DiffSelectionTrackNone, DiffSelectionPoint{}, DiffSelectionPoint{}, false
	}
	start = *anchor
	end = *cursor
	if compareDiffSelectionPoints(start, end) > 0 {
		start, end = end, start
	}
	return track, start, end, true
}

func (s *DiffViewState) selectedUnifiedText(start DiffSelectionPoint, end DiffSelectionPoint) string {
	rendered := s.Rendered.Peek()
	if rendered == nil || len(rendered.Lines) == 0 {
		return ""
	}

	var parts []string
	lastIdx := min(end.Row, len(rendered.Lines)-1)
	for idx := max(0, start.Row); idx <= lastIdx; idx++ {
		text := lineText(rendered.Lines[idx])
		partStart := 0
		partEnd := graphemeCount(text)
		if idx == start.Row {
			partStart = clampInt(start.Grapheme, 0, partEnd)
		}
		if idx == end.Row {
			partEnd = clampInt(end.Grapheme, 0, partEnd)
		}
		if partEnd < partStart {
			partEnd = partStart
		}
		parts = append(parts, sliceTextByGraphemeRange(text, partStart, partEnd))
	}
	return strings.Join(parts, "\n")
}

func (s *DiffViewState) selectedSideBySideText(track DiffSelectionTrack, start DiffSelectionPoint, end DiffSelectionPoint) string {
	sideBySide := s.SideBySide.Peek()
	if sideBySide == nil || len(sideBySide.Rows) == 0 {
		return ""
	}

	var parts []string
	lastIdx := min(end.Row, len(sideBySide.Rows)-1)
	for idx := max(0, start.Row); idx <= lastIdx; idx++ {
		text := selectedSideRowText(sideBySide.Rows[idx], track)
		partStart := 0
		partEnd := graphemeCount(text)
		if idx == start.Row {
			partStart = clampInt(start.Grapheme, 0, partEnd)
		}
		if idx == end.Row {
			partEnd = clampInt(end.Grapheme, 0, partEnd)
		}
		if partEnd < partStart {
			partEnd = partStart
		}
		parts = append(parts, sliceTextByGraphemeRange(text, partStart, partEnd))
	}
	return strings.Join(parts, "\n")
}

func selectedSideRowText(row SideBySideRenderedRow, track DiffSelectionTrack) string {
	if row.Shared != nil {
		return lineText(*row.Shared)
	}
	switch track {
	case DiffSelectionTrackLeft:
		return diffCellText(row.Left)
	case DiffSelectionTrackRight:
		return diffCellText(row.Right)
	default:
		return ""
	}
}

func lineSelectionRange(rowIdx int, start DiffSelectionPoint, end DiffSelectionPoint) (rangeStart, rangeEnd int) {
	if rowIdx < start.Row || rowIdx > end.Row {
		return 0, 0
	}
	rangeStart = 0
	rangeEnd = -1
	if rowIdx == start.Row {
		rangeStart = start.Grapheme
	}
	if rowIdx == end.Row {
		rangeEnd = end.Grapheme
	}
	return rangeStart, rangeEnd
}

func selectionPointCompatible(track DiffSelectionTrack, point DiffSelectionPoint) bool {
	if point.Row < 0 || point.Grapheme < 0 {
		return false
	}
	switch track {
	case DiffSelectionTrackUnified:
		return point.Lane == DiffSelectionLaneUnified
	case DiffSelectionTrackLeft:
		return point.Lane == DiffSelectionLaneLeft || point.Lane == DiffSelectionLaneShared
	case DiffSelectionTrackRight:
		return point.Lane == DiffSelectionLaneRight || point.Lane == DiffSelectionLaneShared
	case DiffSelectionTrackShared:
		return point.Lane == DiffSelectionLaneShared
	default:
		return false
	}
}

func compareDiffSelectionPoints(a DiffSelectionPoint, b DiffSelectionPoint) int {
	if a.Row < b.Row {
		return -1
	}
	if a.Row > b.Row {
		return 1
	}
	if a.Grapheme < b.Grapheme {
		return -1
	}
	if a.Grapheme > b.Grapheme {
		return 1
	}
	return 0
}

func graphemeCount(text string) int {
	count := 0
	for remaining := text; len(remaining) > 0; count++ {
		grapheme, _ := ansi.FirstGraphemeCluster(remaining, ansi.GraphemeWidth)
		if grapheme == "" {
			break
		}
		remaining = remaining[len(grapheme):]
	}
	return count
}

func sliceTextByGraphemeRange(text string, start, end int) string {
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}

	var builder strings.Builder
	index := 0
	for remaining := text; len(remaining) > 0; index++ {
		grapheme, _ := ansi.FirstGraphemeCluster(remaining, ansi.GraphemeWidth)
		if grapheme == "" {
			break
		}
		if index >= end {
			break
		}
		if index >= start {
			builder.WriteString(grapheme)
		}
		remaining = remaining[len(grapheme):]
	}
	return builder.String()
}
