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
func commentGutterStyle(theme t.ThemeData, focused bool) t.Style {
	bg := theme.Background.Blend(theme.Primary, .12)
	fg := commentAnchorColor(theme, false)
	if focused {
		bg = theme.Primary
		// A solid gutter makes selection distinct from syntax and diff colours.
		fg = t.Black
		if t.BrightWhite.ContrastRatio(bg) > fg.ContrastRatio(bg) {
			fg = t.BrightWhite
		}
	}
	return t.Style{ForegroundColor: fg, BackgroundColor: bg, Bold: true}
}
func commentRail(anchor DiffCommentRange, number int, continuation bool) string {
	if number == anchor.Start && !continuation {
		return "╭"
	}
	return "│"
}
func (d DiffView) renderCommentGutter(ctx *t.RenderContext, row, x, numWidth int, side string, number int, continuation bool) {
	anchor, ok := d.commentRange(side, number)
	if !ok {
		return
	}
	label := lineNumberText(number, numWidth)
	if continuation {
		label = strings.Repeat(" ", numWidth)
	}
	ctx.DrawStyledText(x, row, label+commentRail(anchor, number, continuation), commentGutterStyle(d.commentTheme, anchor.Focused))
}
func (d DiffView) renderUnifiedCommentGutter(ctx *t.RenderContext, rendered *RenderedFile, row int, line RenderedDiffLine, continuation bool) {
	// Mark either source column, but fill the entire gutter for the active range.
	for _, side := range []string{"old", "new"} {
		number, x, width := line.OldLine, 0, rendered.OldNumWidth
		if side == "new" {
			number, x, width = line.NewLine, rendered.OldNumWidth+1, rendered.NewNumWidth
		}
		d.renderCommentGutter(ctx, row, x, width, side, number, continuation)
	}
	for _, side := range []string{"old", "new"} {
		number, railX := line.OldLine, rendered.OldNumWidth
		if side == "new" {
			number, railX = line.NewLine, rendered.OldNumWidth+1+rendered.NewNumWidth
		}
		anchor, ok := d.commentRange(side, number)
		if !ok || !anchor.Focused {
			continue
		}
		oldNumber, newNumber := line.OldLine, line.NewLine
		prefix := displayLinePrefix(line, d.HideChangeSigns)
		if continuation {
			oldNumber, newNumber, prefix = 0, 0, " "
		}
		text := lineNumberText(oldNumber, rendered.OldNumWidth) + " " + lineNumberText(newNumber, rendered.NewNumWidth) + " "
		if !d.HideChangeSigns {
			text += prefix + " "
		}
		cells := []rune(text)
		cells[railX] = []rune(commentRail(anchor, number, continuation))[0]
		ctx.DrawStyledText(0, row, string(cells), commentGutterStyle(d.commentTheme, true))
		break
	}
}
func (d DiffView) renderSideCommentGutter(ctx *t.RenderContext, row, x, numWidth, gutterWidth int, side string, cell *RenderedSideCell, continuation bool) {
	anchor, ok := d.commentRange(side, cell.LineNumber)
	if !ok {
		return
	}
	if !anchor.Focused {
		d.renderCommentGutter(ctx, row, x, numWidth, side, cell.LineNumber, continuation)
		return
	}
	number, prefix := cell.LineNumber, cell.Prefix
	if continuation {
		number, prefix = 0, " "
	}
	text := lineNumberText(number, numWidth) + commentRail(anchor, cell.LineNumber, continuation)
	if !d.HideChangeSigns {
		text += displayLinePrefix(RenderedDiffLine{Kind: cell.Kind, Prefix: prefix}, false) + " "
	}
	ctx.DrawStyledText(x, row, ansi.Truncate(text, gutterWidth, ""), commentGutterStyle(d.commentTheme, true))
}

// Missing cells inside an anchored range are alignment gaps, not breaks in the
// range. Carry only the rail across them; do not highlight nonexistent code.
// A negative number is a hunk/header boundary and must interrupt the rail.
func (d DiffView) commentGapRails(side string, numbers []int) map[int]DiffCommentRange {
	marks := map[int]DiffCommentRange{}
	prev := -1
	for i, number := range numbers {
		if number < 0 {
			prev = -1
			continue
		}
		if number == 0 {
			continue
		}
		if prev >= 0 && i > prev+1 {
			var mark DiffCommentRange
			found := false
			for _, anchor := range d.CommentRanges {
				if anchor.Side == side && numbers[prev] >= anchor.Start && number <= anchor.End {
					if !found || anchor.Focused {
						mark, found = anchor, true
					}
					if anchor.Focused {
						break
					}
				}
			}
			if found {
				for gap := prev + 1; gap < i; gap++ {
					marks[gap] = mark
				}
			}
		}
		prev = i
	}
	return marks
}
func (d DiffView) renderGapRail(ctx *t.RenderContext, row, x int, anchor DiffCommentRange) {
	ctx.DrawStyledText(x, row, "│", t.Style{ForegroundColor: commentAnchorColor(d.commentTheme, anchor.Focused), Bold: true})
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
		a.renderActive(true)
		a.scrollToThread(id)
		a.status.Set("")
		a.bump()
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
