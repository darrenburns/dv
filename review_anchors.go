package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
	t "github.com/darrenburns/terma"
)

// DiffCommentRange decorates source gutters without changing source rows,
// code selection, horizontal scrolling, or the width available to the code.
type DiffCommentRange struct {
	Side       string
	Start, End int
	Focused    bool
}

func sideName(left bool) string {
	if left {
		return "old"
	}
	return "new"
}
func commentAnchorColor(theme t.ThemeData, focused bool) t.Color {
	if focused {
		return theme.Primary
	}
	return theme.TextMuted.Blend(theme.Primary, .45)
}
func (a *Aside) commentRanges() []DiffCommentRange {
	var ranges []DiffCommentRange
	for _, thread := range a.review.Threads {
		anchor := thread.Anchor
		if anchor.Path == a.active && anchor.Snapshot == a.review.Current {
			ranges = append(ranges, DiffCommentRange{anchor.Side, anchor.Start, anchor.End, thread.ID == a.activeThread && !(a.editing.Get() && !a.replying)})
		}
	}
	if a.editing.Get() && !a.replying {
		anchor := a.pending
		ranges = append(ranges, DiffCommentRange{anchor.Side, anchor.Start, anchor.End, true})
	}
	return ranges
}
func (d DiffView) commentRange(side string, number int) (DiffCommentRange, bool) {
	var found DiffCommentRange
	ok := false
	for _, anchor := range d.CommentRanges {
		if number > 0 && anchor.Side == side && number >= anchor.Start && number <= anchor.End {
			if !ok || anchor.Focused {
				found, ok = anchor, true
			}
			if anchor.Focused {
				break
			}
		}
	}
	return found, ok
}

// The active range also gets a subtle tint on its code, retaining the diff's
// red/green meaning and token/intraline colours.
func (d DiffView) commentLineBackground(side string, number int, kind RenderedLineKind) (t.Color, bool) {
	anchor, ok := d.commentRange(side, number)
	if !ok || !anchor.Focused {
		return t.Color{}, false
	}
	bg := d.commentTheme.Background
	if style, ok := d.Palette.LineStyleForKind(kind); ok && style.BackgroundColor != nil {
		bg = style.BackgroundColor.ColorAt(1, 1, 0, 0)
	}
	return bg.Blend(d.commentTheme.Primary, .16), true
}
func (d DiffView) renderCommentGutter(ctx *t.RenderContext, row, x, numWidth int, side string, number int, continuation bool) {
	anchor, ok := d.commentRange(side, number)
	if !ok {
		return
	}
	theme := d.commentTheme
	amount := .10
	if anchor.Focused {
		amount = .28
	}
	style := t.Style{ForegroundColor: commentAnchorColor(theme, anchor.Focused), BackgroundColor: theme.Background.Blend(theme.Primary, amount), Bold: true}
	label := lineNumberText(number, numWidth)
	if continuation {
		label = strings.Repeat(" ", numWidth)
	}
	rail := "│"
	if number == anchor.Start && !continuation {
		rail = "╭"
	}
	ctx.DrawStyledText(x, row, label+rail, style)
}

func (a *Aside) focusComment(id string) {
	if a.editing.Peek() {
		return
	}
	for _, thread := range a.review.Threads {
		if thread.ID != id {
			continue
		}
		a.activeThread = id
		anchor := thread.Anchor
		if anchor.Snapshot == a.review.Current {
			a.status.Set(fmt.Sprintf("Highlighted %s lines %d–%d", strings.ToUpper(anchor.Side), anchor.Start, anchor.End))
		} else {
			a.status.Set("Outdated comment · original code shown in the card")
		}
		a.version.Update(func(v int) int { return v + 1 })
		return
	}
}

func (a *Aside) sectionEndsAt(section reviewSection, anchor ReviewAnchor) bool {
	if section.Code == nil || anchor.Snapshot != a.review.Current || anchor.Path != a.active {
		return false
	}
	if a.mode.Peek() == DiffLayoutUnified {
		lines := section.Code.Rendered.Peek().Lines
		if len(lines) == 0 {
			return false
		}
		line := lines[len(lines)-1]
		if anchor.Side == "old" {
			return line.OldLine == anchor.End
		}
		return line.NewLine == anchor.End
	}
	rows := section.Code.SideBySide.Peek().Rows
	if len(rows) == 0 {
		return false
	}
	cell := rows[len(rows)-1].Right
	if anchor.Side == "old" {
		cell = rows[len(rows)-1].Left
	}
	return cell != nil && cell.LineNumber == anchor.End
}

// The connector occupies the hatch margin above a card. It starts at the same
// gutter rail as its source range and points down into the card. Only a card
// immediately after its anchor gets a connector: stacked or outdated cards must
// never appear connected to unrelated code or the preceding conversation.
type commentAnchorConnector struct {
	app     *Aside
	anchor  ReviewAnchor
	focused bool
	theme   t.ThemeData
}

func (c commentAnchorConnector) Build(ctx t.BuildContext) t.Widget { c.theme = ctx.Theme(); return c }
func (c commentAnchorConnector) GetStyle() t.Style {
	return t.Style{Width: t.Flex(1), Height: t.Cells(1)}
}
func (c commentAnchorConnector) Render(ctx *t.RenderContext) {
	if ctx.Width < 7 {
		return
	}
	state := c.app.view
	x, end := 0, ctx.Width
	if c.app.mode.Get() == DiffLayoutSideBySide {
		split := state.SideBySide.Get()
		panes := sideBySidePaneLayout(ctx.Width, split, c.app.hideSigns.Get(), c.app.diffRatio.Get())
		x, end = panes.LeftPaneX+max(1, split.LeftNumWidth), panes.DividerX
		if c.anchor.Side == "new" {
			x, end = panes.RightPaneX+max(1, split.RightNumWidth), ctx.Width
		}
	} else {
		rendered := state.Rendered.Get()
		x = rendered.OldNumWidth
		if c.anchor.Side == "new" {
			x += 1 + rendered.NewNumWidth
		}
	}
	// At least one cell inside the card border, including when +/- are hidden.
	target := max(3, x+2)
	if target >= ctx.Width-3 || x < 0 {
		return
	}
	style := t.Style{ForegroundColor: commentAnchorColor(c.theme, c.focused), BackgroundColor: c.theme.Background, Bold: true}
	ctx.DrawStyledText(x, 0, "╰"+strings.Repeat("─", target-x-1)+"▼", style)
	label := fmt.Sprintf(" %s %d–%d", strings.ToUpper(c.anchor.Side), c.anchor.Start, c.anchor.End)
	if c.anchor.Start == c.anchor.End {
		label = fmt.Sprintf(" %s %d", strings.ToUpper(c.anchor.Side), c.anchor.Start)
	}
	ctx.DrawStyledText(target+1, 0, ansi.Truncate(label, max(0, end-target-1), ""), style)
}
func (a *Aside) anchorFrame(theme t.ThemeData, card t.Widget, anchor ReviewAnchor, focused, connected bool) t.Widget {
	var lead t.Widget = t.Spacer{Height: t.Cells(1)}
	if connected && anchor.Snapshot == a.review.Current && anchor.Path == a.active {
		lead = commentAnchorConnector{app: a, anchor: anchor, focused: focused}
	}
	return hatchedFrame(theme, card, lead)
}
