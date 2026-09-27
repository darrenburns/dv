package main

import t "github.com/darrenburns/terma"

// Capture the actual usable viewport before laying out the review content.
// Scrolling uses measured card sizes, including wrapping and an open editor.
type reviewViewport struct {
	t.Column
	app *Aside
}

func (v reviewViewport) Build(t.BuildContext) t.Widget { return v }
func (v reviewViewport) ChildWidgets() []t.Widget      { return v.Children }
func (v reviewViewport) OnLayout(_ t.BuildContext, metrics t.LayoutMetrics) {
	child, ok := metrics.ChildLayout(0)
	if !ok {
		return
	}
	box := child.Box.UsableContentBox()
	width, height := box.Width, box.Height
	a := v.app
	if (width != a.reviewViewportWidth || height != a.reviewViewportHeight) && a.pendingScroll == "" {
		a.pendingScroll = a.activeThread
		if a.editing.Peek() && !a.replying {
			a.pendingScroll = editorID
		}
	}
	a.reviewViewportWidth, a.reviewViewportHeight = width, height
}

func (a *Aside) revealPendingComment() {
	id := a.pendingScroll
	card, ok := a.threadBounds[id]
	height := a.reviewViewportHeight
	if id == "" || !ok || height <= 0 {
		return
	}
	anchor := a.pending
	if id != editorID {
		for _, thread := range a.review.Threads {
			if thread.ID == id {
				anchor = thread.Anchor
				break
			}
		}
	}
	source, found := a.sourceRangeBounds(anchor)
	top := card.Y
	if found {
		top = source.Y
	}
	bottom := card.Y + card.Height
	offset := a.scroll.Offset.Peek()
	if bottom-top <= height {
		// Keep a little preceding context when there is room; otherwise fit exactly.
		minOffset := max(0, bottom-height)
		maxOffset := max(minOffset, top-1)
		offset = clampInt(offset, minOffset, maxOffset)
	} else if a.editing.Peek() {
		// A conversation taller than the window must prioritise its reply editor.
		offset = max(0, bottom-height)
	} else if found {
		// Keep the end of the code range above the start of an oversized comment.
		offset = max(top, card.Y-min(6, max(1, height/3)))
	} else {
		offset = card.Y
	}
	a.pendingScroll = ""
	a.scroll.Offset.Set(clampInt(offset, 0, max(0, a.reviewContentHeight-height)))
}

func (a *Aside) sourceRangeBounds(anchor ReviewAnchor) (t.Rect, bool) {
	if anchor.Snapshot != a.review.Current || anchor.Path != a.active {
		return t.Rect{}, false
	}
	first, last, found := 0, 0, false
	for _, section := range a.sections {
		if section.Code == nil {
			continue
		}
		bounds, ok := a.codeBounds[section.Code]
		if !ok {
			continue
		}
		visit := func(number, y, rows int) {
			if number < anchor.Start || number > anchor.End || number <= 0 {
				return
			}
			start, end := bounds.Y+y, bounds.Y+y+rows
			if !found || start < first {
				first = start
			}
			if !found || end > last {
				last = end
			}
			found = true
		}
		y := 0
		if a.mode.Peek() == DiffLayoutSideBySide {
			split := section.Code.SideBySide.Peek()
			panes := sideBySidePaneLayout(bounds.Width, split, a.hideSigns.Peek(), a.diffRatio.Peek())
			for _, row := range split.Rows {
				rows := 1
				if a.hardWrap.Peek() {
					rows = wrappedSideRowCount(row, panes, bounds.Width)
				}
				cell := row.Right
				if anchor.Side == "old" {
					cell = row.Left
				}
				if cell != nil {
					visit(cell.LineNumber, y, rows)
				}
				y += rows
			}
		} else {
			rendered := section.Code.Rendered.Peek()
			width := max(1, bounds.Width-renderedGutterWidth(rendered, a.hideSigns.Peek()))
			for _, line := range rendered.Lines {
				rows := 1
				if a.hardWrap.Peek() {
					rows = wrappedLineRowCount(line, width)
				}
				number := line.NewLine
				if anchor.Side == "old" {
					number = line.OldLine
				}
				visit(number, y, rows)
				y += rows
			}
		}
	}
	return t.Rect{Y: first, Height: last - first}, found
}
