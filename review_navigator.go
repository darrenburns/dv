package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
	t "github.com/darrenburns/terma"
)

const (
	reviewListID       = "aside-review-list"
	reviewListScrollID = "aside-review-list-scroll"
	reviewSplitID      = "aside-review-split"
)

// Refresh from the current file in handlers, preserving identity rather than
// row position when another client adds, removes, or replaces comments.
func (a *Aside) syncReviewNavigator() {
	previous, _ := a.reviewList.SelectedItem()
	fileChanged := a.reviewListFile != a.active
	items := []ReviewThread{}
	for _, thread := range a.review.Threads {
		if thread.Anchor.Path == a.active {
			items = append(items, thread)
		}
	}
	a.reviewList.SetItems(items)
	a.reviewListFile = a.active
	if fileChanged {
		a.reviewListScroll.Offset.Set(0)
	}
	index, activeFound := 0, false
	for i, thread := range items {
		if !fileChanged && thread.ID == previous.ID {
			index = i
		}
		if thread.ID == a.activeThread {
			index, activeFound = i, true
			break
		}
	}
	if !activeFound {
		a.activeThread = ""
	}
	if len(items) > 0 {
		a.reviewList.SelectIndex(index)
	}
}

func (a *Aside) selectReviewEntry(thread ReviewThread) {
	if a.editing.Peek() || thread.Anchor.Path != a.active {
		return
	}
	a.focusComment(thread.ID)
}

func (a *Aside) focusReviewNavigator() {
	if a.editing.Peek() {
		return
	}
	if thread, ok := a.reviewList.SelectedItem(); ok {
		a.sidebarVisible.Set(true)
		a.selectReviewEntry(thread)
		t.RequestFocus(reviewListID)
	} else {
		a.status.Set("No comments in this file")
	}
}

func (a *Aside) resizeReviewNavigator() {
	if a.editing.Peek() || a.reviewList.ItemCount() == 0 {
		return
	}
	a.sidebarVisible.Set(true)
	a.reviewDividerActive.Set(true)
	a.palette.SetNextFocusIDOnClose(reviewSplitID)
	t.RequestFocus(reviewSplitID)
}

func reviewEntrySummary(thread ReviewThread) string {
	if len(thread.Messages) > 0 {
		for _, line := range strings.Split(thread.Messages[0].Body, "\n") {
			if line = strings.Join(strings.Fields(safeText(line)), " "); line != "" {
				return ansi.Truncate(line, 160, "…")
			}
		}
	}
	return "No comment text"
}

func reviewEntryRange(anchor ReviewAnchor) string {
	label := fmt.Sprintf("%s %d", strings.ToUpper(anchor.Side), anchor.Start)
	if anchor.End != anchor.Start {
		label += fmt.Sprintf("–%d", anchor.End)
	}
	return label
}

func (a *Aside) buildReviewSidebar(ctx t.BuildContext, theme t.ThemeData, files t.Widget) t.Widget {
	items := a.reviewList.Items.Get()
	if len(items) == 0 {
		return files
	}
	open := 0
	for _, thread := range items {
		if thread.Status != "resolved" {
			open++
		}
	}
	list := t.List[ReviewThread]{
		ID: reviewListID, State: a.reviewList, ScrollState: a.reviewListScroll,
		DisableFocus:   a.editing.Get(),
		OnCursorChange: a.selectReviewEntry,
		OnSelect: func(thread ReviewThread) {
			a.selectReviewEntry(thread)
			if !a.editing.Peek() {
				t.RequestFocus(viewerScrollID)
			}
		},
		Style: t.Style{Width: t.Flex(1)},
	}
	focused := ctx.IsFocused(list)
	list.RenderItem = func(thread ReviewThread, cursor, _ bool) t.Widget {
		active := thread.ID == a.activeThread
		bg, fg, muted := theme.Background, theme.Text, theme.TextMuted
		if active || (cursor && focused) {
			bg = theme.ActiveCursor.WithAlpha(.18)
			if focused && cursor {
				bg, fg, muted = theme.ActiveCursor, theme.SelectionText, theme.SelectionText
			}
		}
		kind := "Feedback"
		switch thread.Kind {
		case "finding":
			kind = "Finding"
		case "explanation":
			kind = "Explanation"
		}
		marker, markerColor := "○", theme.TextMuted
		if thread.Status == "resolved" {
			marker, markerColor = "✓", theme.Success
		} else if active {
			marker, markerColor = "▸", theme.Primary
		}
		if focused && cursor {
			markerColor = theme.SelectionText
		}
		selectEntry := func(t.MouseEvent) {
			a.selectReviewEntry(thread)
			if !a.editing.Peek() {
				t.RequestFocus(reviewListID)
			}
		}
		children := []t.Widget{
			t.Text{Content: reviewEntryRange(thread.Anchor) + " · " + kind, Wrap: t.WrapSoft, Click: selectEntry, Style: t.Style{Width: t.Flex(1), Bold: true, ForegroundColor: fg}},
			t.Text{Content: reviewEntrySummary(thread), Wrap: t.WrapSoft, Click: selectEntry, Style: t.Style{Width: t.Flex(1), MaxHeight: t.Cells(2), ForegroundColor: muted}},
		}
		labels := []string{}
		if !thread.Submitted {
			labels = append(labels, "Draft")
		}
		if thread.Anchor.Snapshot != a.review.Current {
			labels = append(labels, "Outdated")
		}
		if thread.Status == "resolved" {
			labels = append(labels, "Resolved")
		} else if thread.Status == "addressed" {
			labels = append(labels, "Addressed")
		}
		if len(labels) > 0 {
			children = append(children, t.Text{Content: strings.Join(labels, " · "), Wrap: t.WrapSoft, Click: selectEntry, Style: t.Style{Width: t.Flex(1), ForegroundColor: muted}})
		}
		return t.Row{
			ID: "review-entry-" + thread.ID, Spacing: 1,
			Style: t.Style{Width: t.Flex(1), Padding: t.EdgeInsets{Left: 1, Right: 1, Bottom: 1}, BackgroundColor: bg},
			Click: selectEntry,
			Children: []t.Widget{
				t.Text{Content: marker, Click: selectEntry, Style: t.Style{ForegroundColor: markerColor, Bold: true}},
				t.Column{Click: selectEntry, Style: t.Style{Width: t.Flex(1)}, Children: children},
			},
		}
	}
	header := t.Row{Style: t.Style{Width: t.Flex(1), Padding: t.EdgeInsetsXY(1, 0), BackgroundColor: theme.Surface}, Children: []t.Widget{
		t.Text{Content: "REVIEW", Style: t.Style{Width: t.Flex(1), Bold: true, ForegroundColor: theme.Text}},
		t.Text{Content: fmt.Sprintf("%d open", open), Style: t.Style{ForegroundColor: theme.TextMuted}},
	}}
	navigator := t.Dock{Top: []t.Widget{header}, Body: t.Scrollable{ID: reviewListScrollID, State: a.reviewListScroll, Style: t.Style{Width: t.Flex(1), Height: t.Flex(1)}, Child: list}}
	dividerFocused := false
	if id, ok := ctx.Focused().(t.Identifiable); ok {
		dividerFocused = id.WidgetID() == reviewSplitID
	}
	return FocusAwareSplitPane{SplitPane: t.SplitPane{
		ID: reviewSplitID, State: a.reviewSplit, Orientation: t.SplitVertical, MinPaneSize: 5, DividerSize: 1,
		DividerForeground: theme.Border, DividerFocusForeground: theme.Primary, DividerBackground: theme.Background,
		First: files, Second: navigator, Style: t.Style{Width: t.Flex(1), Height: t.Flex(1)},
		OnExitFocus: func() { a.reviewDividerActive.Set(false); t.RequestFocus(reviewListID) },
	}, AllowFocus: a.reviewDividerActive.Get(), EnableKeybinds: dividerFocused}
}
