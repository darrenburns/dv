package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
	t "github.com/darrenburns/terma"
)

// A single, width-aware status strip. Review context has a stable position;
// short action messages share the remaining space instead of adding a row.
type asideStatusBar struct {
	t.Text
	app   *Aside
	theme t.ThemeData
}

func (s asideStatusBar) Build(ctx t.BuildContext) t.Widget {
	s.theme = ctx.Theme()
	s.Text = t.Text{Content: " ", Style: t.Style{Width: t.Flex(1), Height: t.Cells(1), BackgroundColor: s.theme.Surface}}
	return s
}
func anchorSummary(anchor ReviewAnchor) string {
	if anchor.Start == anchor.End {
		return fmt.Sprintf("%s · line %d", strings.ToUpper(anchor.Side), anchor.Start)
	}
	return fmt.Sprintf("%s · lines %d–%d", strings.ToUpper(anchor.Side), anchor.Start, anchor.End)
}
func (s asideStatusBar) Render(ctx *t.RenderContext) {
	a, theme := s.app, s.theme
	badge, context := "BROWSE", a.status.Get()
	reviewing := false
	for i, thread := range a.review.Threads {
		if thread.ID == a.activeThread {
			reviewing = true
			badge = fmt.Sprintf("REVIEW %d/%d", i+1, len(a.review.Threads))
			if context == "" {
				context = anchorSummary(thread.Anchor)
			}
			if thread.Anchor.Snapshot != a.review.Current {
				context = "Outdated · original code in comment"
			}
			break
		}
	}
	if a.editing.Get() {
		badge = "DRAFT"
		if a.replying {
			badge = "REPLY"
		}
		context = anchorSummary(a.pending)
	}
	if context == "" {
		context = "Select code to leave feedback"
	}
	hints := []struct{ key, label string }{{"c", "comment"}, {"/", "filter"}, {"Ctrl+P", "commands"}}
	if reviewing {
		hints = []struct{ key, label string }{{"[/]", "comments"}, {"e", "reply"}, {"Ctrl+P", "commands"}}
	}
	if !reviewing && a.reviewList.ItemCount() > 0 {
		hints = append(hints[:1], append([]struct{ key, label string }{{"o", "reviews"}}, hints[1:]...)...)
	}
	if a.editing.Get() {
		hints = nil
	} // Editing shortcuts are already inside the buttons.
	hintWidth := func() int {
		n := 0
		for _, hint := range hints {
			n += ansi.StringWidth(hint.key) + ansi.StringWidth(hint.label) + 3
		}
		return max(0, n-2)
	}
	badgeWidth := ansi.StringWidth(badge) + 2
	for len(hints) > 0 && badgeWidth+hintWidth()+24 > ctx.Width {
		hints = hints[:len(hints)-1]
	}
	if ctx.Width < badgeWidth+6 {
		badge = ansi.Truncate(badge, max(0, ctx.Width-3), "")
		badgeWidth = ansi.StringWidth(badge) + 2
	}
	ctx.FillRect(0, 0, ctx.Width, 1, theme.Surface)
	ctx.DrawStyledText(0, 0, " "+badge+" ", t.Style{ForegroundColor: theme.PrimaryText, BackgroundColor: theme.PrimaryBg, Bold: true})
	hintX := ctx.Width - hintWidth() - 1
	available := max(0, hintX-badgeWidth-2)
	ctx.DrawStyledText(badgeWidth+1, 0, ansi.Truncate(singleLine(context), available, "…"), t.Style{ForegroundColor: theme.TextMuted})
	x := hintX
	for _, hint := range hints {
		ctx.DrawStyledText(x, 0, hint.key, t.Style{ForegroundColor: theme.Primary, Bold: true})
		x += ansi.StringWidth(hint.key)
		ctx.DrawStyledText(x, 0, " "+hint.label+"  ", t.Style{ForegroundColor: theme.TextMuted})
		x += ansi.StringWidth(hint.label) + 3
	}
}
