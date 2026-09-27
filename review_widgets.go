package main

import (
	"fmt"
	"strings"

	t "github.com/darrenburns/terma"
)

// Review sections interleave code renderers and real, independently laid out
// widgets. Review text never enters the source-line or selection model.
type reviewSection struct {
	Code     *DiffViewState
	Thread   *ReviewThread
	Composer bool
}

func reviewSections(unified *RenderedFile, split *SideBySideRenderedFile, review *Review, path string, mode DiffLayoutMode, pending *ReviewAnchor, activeThread string) []reviewSection {
	count := len(unified.Lines)
	if mode == DiffLayoutSideBySide {
		count = len(split.Rows)
	}
	positionFor := func(anchor ReviewAnchor) int {
		if anchor.Snapshot == review.Current {
			for row := 0; row < count; row++ {
				number := 0
				if mode == DiffLayoutUnified {
					number = unified.Lines[row].NewLine
					if anchor.Side == "old" {
						number = unified.Lines[row].OldLine
					}
				} else {
					cell := split.Rows[row].Right
					if anchor.Side == "old" {
						cell = split.Rows[row].Left
					}
					if cell != nil {
						number = cell.LineNumber
					}
				}
				if number == anchor.End {
					return row + 1
				}
			}
		}
		return count
	}
	after := map[int][]reviewSection{}
	// A new comment editor sits immediately after its selected code, before
	// any existing threads at the same anchor.
	if pending != nil && pending.Path == path {
		after[positionFor(*pending)] = []reviewSection{{Composer: true}}
	}
	for i := range review.Threads {
		thread := &review.Threads[i]
		if thread.Anchor.Path != path {
			continue
		}
		position := positionFor(thread.Anchor)
		after[position] = append(after[position], reviewSection{Thread: thread})
	}
	// Promote within the same source position, without changing store order.
	// The new-comment editor keeps precedence while it owns focus.
	for position, group := range after {
		first := 0
		if len(group) > 0 && group[0].Composer {
			first = 1
		}
		for i := first; i < len(group); i++ {
			if group[i].Thread != nil && group[i].Thread.ID == activeThread {
				selected := group[i]
				copy(group[first+1:i+1], group[first:i])
				group[first] = selected
				break
			}
		}
		after[position] = group
	}
	sections := []reviewSection{}
	start := 0
	for end := 1; end <= count; end++ {
		if len(after[end]) == 0 && end != count {
			continue
		}
		u, s := *unified, *split
		if mode == DiffLayoutUnified {
			u.Lines = unified.Lines[start:end]
			s = *buildSideBySideFromRendered(&u)
		} else {
			s.Rows = split.Rows[start:end]
			u.Lines = nil
		}
		state := NewDiffViewState(&u)
		state.SetRenderedPair(&u, &s)
		sections = append(sections, reviewSection{Code: state})
		sections = append(sections, after[end]...)
		start = end
	}
	return sections
}

type reviewContent struct {
	t.Column
	app *Aside
}

func (c reviewContent) Build(t.BuildContext) t.Widget { return c }
func (c reviewContent) ChildWidgets() []t.Widget      { return c.Children }
func (c reviewContent) OnLayout(_ t.BuildContext, metrics t.LayoutMetrics) {
	a := c.app
	a.threadBounds = map[string]t.Rect{}
	a.codeBounds = map[*DiffViewState]t.Rect{}
	a.reviewContentHeight = metrics.Box().ContentHeight()
	for i, section := range a.sections {
		bounds, ok := metrics.ChildBounds(i)
		if !ok {
			continue
		}
		if section.Composer {
			a.threadBounds[editorID] = bounds
		}
		if section.Thread != nil {
			a.threadBounds[section.Thread.ID] = bounds
		}
		if section.Code != nil {
			a.codeBounds[section.Code] = bounds
		}
	}
	a.revealPendingComment()
}
func (a *Aside) buildReviewContent() t.Widget {
	ranges := a.commentRanges()
	children := make([]t.Widget, 0, len(a.sections))
	for i, section := range a.sections {
		if section.Composer {
			children = append(children, inlineComposer{app: a, standalone: true})
			continue
		}
		if section.Thread != nil {
			children = append(children, reviewCard{
				app: a, thread: *section.Thread,
				connected: i > 0 && a.sectionEndsAt(a.sections[i-1], section.Thread.Anchor),
				stacked:   i > 0 && (a.sections[i-1].Thread != nil || a.sections[i-1].Composer),
			})
			continue
		}
		state := section.Code
		children = append(children, DiffView{ID: fmt.Sprintf("%s-%d", viewerID, i), State: state, CommentRanges: ranges, Embedded: true, OnSelectionStart: func() { a.selectCode(state) }, LayoutMode: a.mode.Get(), HardWrap: a.hardWrap.Get(), HideChangeSigns: a.hideSigns.Get(), IntralineStyle: a.intra, Width: t.Flex(1), Height: t.Auto})
	}
	return reviewContent{Column: t.Column{Style: t.Style{Width: t.Flex(1), Height: t.Auto}, Children: children}, app: a}
}
func (a *Aside) selectCode(state *DiffViewState) {
	for _, section := range a.sections {
		if section.Code != nil && section.Code != state {
			section.Code.ClearSelection()
		}
	}
	a.view = state
}
func (a *Aside) clearSelection() {
	for _, section := range a.sections {
		if section.Code != nil {
			section.Code.ClearSelection()
		}
	}
}

type reviewCard struct {
	app       *Aside
	thread    ReviewThread
	connected bool
	stacked   bool
}

func (c reviewCard) WidgetID() string { return "thread-" + c.thread.ID }
func (c reviewCard) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	thread, a := c.thread, c.thread.Anchor
	status := strings.ToUpper(thread.Status)
	color := theme.Info
	if thread.Status == "resolved" {
		color = theme.Success
	}
	if !thread.Submitted {
		status = "DRAFT"
		color = theme.Warning
	}
	focused := c.app.activeThread == thread.ID
	border := commentAnchorColor(theme, focused)
	kind := "Feedback"
	if thread.Kind == "finding" {
		kind = "Review finding"
	}
	if thread.Kind == "explanation" {
		kind = "Explanation"
	}
	rangeLabel := fmt.Sprintf("· %s %d", strings.ToUpper(a.Side), a.Start)
	if a.End != a.Start {
		rangeLabel += fmt.Sprintf("–%d", a.End)
	}
	children := []t.Widget{
		t.Row{Spacing: 1, Style: t.Style{Width: t.Flex(1)}, Children: []t.Widget{
			t.Text{Content: kind, Style: t.Style{Bold: true, ForegroundColor: theme.Text}},
			t.Text{ID: "anchor-" + thread.ID, Content: rangeLabel, Wrap: t.WrapSoft, Click: func(t.MouseEvent) { c.app.focusComment(thread.ID) }, Style: t.Style{Width: t.Flex(1), Bold: true, ForegroundColor: border}},
			t.Text{Content: " " + status + " ", Style: t.Style{Bold: true, ForegroundColor: color, BackgroundColor: theme.Background}},
		}},
	}
	if a.Snapshot != c.app.review.Current {
		children = append(children,
			t.Text{Content: "OUTDATED · original snapshot " + a.Snapshot[:8], Wrap: t.WrapSoft, Style: t.Style{Width: t.Flex(1), ForegroundColor: theme.Warning}},
			t.Text{Content: safeText(a.Quote), Wrap: t.WrapSoft, Style: t.Style{Width: t.Flex(1), Padding: t.EdgeInsetsXY(1, 0), BackgroundColor: theme.Background, ForegroundColor: theme.TextMuted}},
		)
	}
	for _, message := range thread.Messages {
		children = append(children, t.Spacer{Height: t.Cells(1)})
		children = append(children,
			t.Text{Content: safeText(message.Author), Style: t.Style{Width: t.Flex(1), Bold: true, ForegroundColor: theme.Secondary}},
			t.Text{Content: safeText(message.Body), Wrap: t.WrapSoft, Style: t.Style{Width: t.Flex(1), ForegroundColor: theme.Text}},
		)
	}
	reply := func() {
		if c.app.editing.Peek() {
			return
		}
		c.app.activeThread = thread.ID
		c.app.beginReply()
	}
	resolve := func() {
		if c.app.editing.Peek() {
			return
		}
		c.app.activeThread = thread.ID
		c.app.toggleResolved()
	}
	resolveLabel := "Resolve"
	if thread.Status == "resolved" {
		resolveLabel = "Reopen"
	}
	actions := []t.Widget{
		shortcutButton{Button: t.Button{ID: "reply-" + thread.ID, Label: "Reply", OnPress: reply, Click: func(t.MouseEvent) { reply() }, Style: t.Style{ForegroundColor: theme.Primary, BackgroundColor: theme.Surface}}, Shortcut: "e", Key: "e"},
		shortcutButton{Button: t.Button{ID: "resolve-" + thread.ID, Label: resolveLabel, OnPress: resolve, Click: func(t.MouseEvent) { resolve() }, Style: t.Style{ForegroundColor: theme.TextMuted, BackgroundColor: theme.Surface}}, Shortcut: "x", Key: "x"},
	}

	if c.app.editing.Get() && c.app.replying && c.app.activeThread == thread.ID {
		children = append(children, t.Spacer{Height: t.Cells(1)}, inlineComposer{app: c.app})
	} else {
		children = append(children, t.Spacer{Height: t.Cells(1)}, t.Row{Spacing: 2, Style: t.Style{Width: t.Flex(1)}, Children: actions})
		if !thread.Submitted {
			children = append(children, shortcutButton{Button: t.Button{ID: "submit-" + thread.ID, Label: "Submit feedback", OnPress: c.app.exportFeedback, Click: func(t.MouseEvent) { c.app.exportFeedback() }, Style: t.Style{ForegroundColor: theme.Primary, BackgroundColor: theme.Surface}}, Shortcut: "f", Key: "f"})
		}
	}
	card := t.Column{Style: t.Style{Width: t.Flex(1), Height: t.Auto, Padding: t.EdgeInsetsXY(1, 0), Border: t.RoundedBorder(border), BackgroundColor: theme.Surface}, Children: children}
	if c.stacked {
		// Adjacent cards need no empty connector row between their borders.
		return hatchedFrame(theme, card, t.Spacer{Height: t.Cells(0)})
	}
	return c.app.anchorFrame(theme, card, a, focused, c.connected)
}

// inlineComposer is shared by new code comments and replies within a thread.
// It owns focus and actions, while its parent supplies the surrounding context.
type inlineComposer struct {
	app        *Aside
	standalone bool
}

func (c inlineComposer) WidgetID() string { return "aside-inline-composer" }
func (c inlineComposer) Build(ctx t.BuildContext) t.Widget {
	a, theme := c.app, ctx.Theme()
	label, saveLabel := "Reply", "Send reply"
	if c.standalone {
		label = fmt.Sprintf("New comment · %s %d–%d", a.pending.Side, a.pending.Start, a.pending.End)
		saveLabel = "Save draft"
	}
	content := t.Column{Style: t.Style{Width: t.Flex(1), Height: t.Auto}, Children: []t.Widget{
		t.Text{Content: label, Wrap: t.WrapSoft, Style: t.Style{Width: t.Flex(1), Bold: true, ForegroundColor: theme.Primary}},
		t.TextArea{ID: editorID, State: a.editor, Style: t.Style{Width: t.Flex(1), Height: t.Cells(4), Padding: t.EdgeInsetsXY(1, 0), BackgroundColor: theme.Background, Border: t.RoundedBorder(theme.Border)}, Placeholder: "Write feedback…", ExtraKeybinds: []t.Keybind{
			{Key: "ctrl+s", Name: saveLabel, Hidden: true, Action: a.saveComment},
			{Key: "escape", Name: "Cancel", Hidden: true, Action: a.cancelComment},
		}},
		t.Row{Spacing: 2, Style: t.Style{Width: t.Flex(1)}, Children: []t.Widget{
			shortcutButton{Button: t.Button{ID: "aside-save-comment", Label: saveLabel, OnPress: a.saveComment, Click: func(t.MouseEvent) { a.saveComment() }, Variant: t.ButtonPrimary}, Shortcut: "Ctrl+S", Key: "ctrl+s"},
			shortcutButton{Button: t.Button{ID: "aside-cancel-comment", Label: "Cancel", OnPress: a.cancelComment, Click: func(t.MouseEvent) { a.cancelComment() }}, Shortcut: "Esc", Key: "escape"},
		}},
	}}
	var child t.Widget = content
	if c.standalone {
		child = a.anchorFrame(theme, t.Column{Style: t.Style{Width: t.Flex(1), Height: t.Auto, Padding: t.EdgeInsetsXY(1, 0), BackgroundColor: theme.Surface, Border: t.RoundedBorder(theme.Primary)}, Children: []t.Widget{content}}, a.pending, true, true)
	}
	return t.FocusTrap{ID: "aside-compose-trap", Active: true, Child: child}
}

func (c inlineComposer) Keybinds() []t.Keybind {
	return []t.Keybind{
		{Key: "ctrl+s", Hidden: true, Action: c.app.saveComment},
		{Key: "escape", Hidden: true, Action: c.app.cancelComment},
	}
}

// When focus is anywhere in a thread's action area, shortcuts belong to that
// thread. During composition, plain letters must remain text input.
func (c reviewCard) Keybinds() []t.Keybind {
	if c.app.editing.Get() {
		return nil
	}
	activate := func(action func()) func() { return func() { c.app.activeThread = c.thread.ID; action() } }
	return []t.Keybind{
		{Key: "e", Hidden: true, Action: activate(c.app.beginReply)},
		{Key: "x", Hidden: true, Action: activate(c.app.toggleResolved)},
	}
}
