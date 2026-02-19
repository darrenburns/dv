package main

import (
	"sync"
	"time"

	t "github.com/darrenburns/terma"
)

const sideDividerOverlayHoldDuration = 1 * time.Second

// DiffViewState tracks scroll state and rendered diff content for DiffView.
type DiffViewState struct {
	ScrollY    t.Signal[int]
	ScrollX    t.Signal[int]
	Rendered   t.AnySignal[*RenderedFile]
	SideBySide t.AnySignal[*SideBySideRenderedFile]
	SplitRatio t.Signal[float64]

	viewportWidth  int
	viewportHeight int

	sideDividerDragging     bool
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
	s.sideDividerDragging = false
	s.sideDividerDragOffset = 0
	s.sideDividerLastResize.Set(0)
	s.stopSideDividerOverlayTimer()
	s.ScrollY.Set(0)
	s.ScrollX.Set(0)
	s.Clamp(0)
}

func (s *DiffViewState) SideBySideSplitRatio() float64 {
	if s == nil || !s.SplitRatio.IsValid() {
		return 0.5
	}
	return clampSideBySideSplitRatio(s.SplitRatio.Peek())
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
	s.sideDividerDragging = true
	s.sideDividerDragOffset = pointerX - dividerX
}

func (s *DiffViewState) StopSideDividerDrag() {
	if s == nil {
		return
	}
	s.sideDividerDragging = false
	s.sideDividerDragOffset = 0
}

func (s *DiffViewState) SideDividerDragging() bool {
	return s != nil && s.sideDividerDragging
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
	if s.sideDividerDragging {
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
