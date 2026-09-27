package main

import (
	"fmt"
	"strings"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
	t "github.com/darrenburns/terma"
	"github.com/stretchr/testify/require"
)

func reviewBufferText(buffer *uv.Buffer, width, height int) string {
	var s strings.Builder
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			s.WriteString(buffer.CellAt(x, y).Content)
		}
		s.WriteByte('\n')
	}
	return s.String()
}
func TestFocusedCommentCoversWholeGutter(tt *testing.T) {
	for _, mode := range []DiffLayoutMode{DiffLayoutUnified, DiffLayoutSideBySide} {
		tt.Run(map[DiffLayoutMode]string{DiffLayoutUnified: "unified", DiffLayoutSideBySide: "split"}[mode], func(tt *testing.T) {
			a := appFixture(tt)
			a.mode.Set(mode)
			a.renderActive(false)
			theme, _ := t.GetTheme(t.ThemeNameObsidianTide)
			view := DiffView{State: a.view, LayoutMode: mode, Palette: NewThemePalette(theme), commentTheme: theme, CommentRanges: []DiffCommentRange{{Side: "new", Start: 4, End: 5, Focused: true}}}
			b := uv.NewBuffer(100, 30)
			view.Render(t.NewRenderContext(b, 100, 30, nil, nil, t.BuildContext{}, nil))
			x, gutter, row := 0, renderedGutterWidth(a.view.Rendered.Peek(), false), 0
			if mode == DiffLayoutUnified {
				for i, line := range a.view.Rendered.Peek().Lines {
					if line.NewLine == 4 {
						row = i
						break
					}
				}
			} else {
				split := a.view.SideBySide.Peek()
				panes := sideBySidePaneLayout(100, split, false, .5)
				x, gutter = panes.RightPaneX, panes.RightGutterWidth
				for i, r := range split.Rows {
					if r.Right != nil && r.Right.LineNumber == 4 {
						row = i
						break
					}
				}
			}
			bg := b.CellAt(x, row).Style.Bg
			for col := x; col < x+gutter; col++ {
				require.Equal(tt, bg, b.CellAt(col, row).Style.Bg, "gap in highlighted gutter at %d", col)
			}
			require.NotEqual(tt, bg, b.CellAt(x+gutter+1, row).Style.Bg, "gutter must be stronger than the code highlight")
		})
	}
}

func TestOldCommentRailCrossesInsertedRows(tt *testing.T) {
	a := appFixture(tt)
	diff := `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,4 +1,5 @@
 package main
 func greet() string {
+    name := "World"
     return "Hello"
 }
`
	r, err := a.store.Open(a.review.Repo, "gap", "worktree", ReviewSnapshot{Diff: diff, Base: "base", Head: "working"})
	require.NoError(tt, err)
	a.apply(r)
	a.mode.Set(DiffLayoutSideBySide)
	a.renderActive(false)
	theme, _ := t.GetTheme(t.ThemeNameObsidianTide)
	view := DiffView{State: a.view, LayoutMode: DiffLayoutSideBySide, Palette: NewThemePalette(theme), commentTheme: theme, CommentRanges: []DiffCommentRange{{Side: "old", Start: 2, End: 3, Focused: true}}}
	b := uv.NewBuffer(100, 30)
	view.Render(t.NewRenderContext(b, 100, 30, nil, nil, t.BuildContext{}, nil))
	split := a.view.SideBySide.Peek()
	for i, row := range split.Rows {
		if row.Left == nil && row.Right != nil && row.Right.LineNumber == 3 {
			require.Equal(tt, "│", b.CellAt(split.LeftNumWidth, i).Content, "the inserted row must not sever the old-side range bracket")
			return
		}
	}
	tt.Fatal("fixture lacks the inserted row")
}

func TestNextSameRangeCommentKeepsCodeAndThreadVisible(tt *testing.T) {
	a := appFixture(tt)
	first, second := annotation(a.review, "first"), annotation(a.review, "second")
	first.Body = "First review finding."
	second.Body = "Second review finding."
	r, err := a.store.Publish(a.review.ID, "agent", []AnnotationInput{first, second}, true)
	require.NoError(tt, err)
	a.apply(r)
	const width, height = 110, 26
	b := uv.NewBuffer(width, height)
	renderer := t.NewRenderer(b, width, height, t.NewFocusManager(), t.NewAnySignal[t.Focusable](nil), t.NewAnySignal[t.Widget](nil))
	renderer.Render(a)
	a.nextThread(1)
	renderer.Render(a)
	a.nextThread(1)
	for i := 0; i < 3; i++ {
		renderer.Render(a)
	}
	screen := reviewBufferText(b, width, height)
	require.Contains(tt, screen, "Second review finding.")
	require.Contains(tt, screen, "// Welcome everyone.", "navigating to the next comment must retain its code when both fit")
}

func TestReviewNavigationPromotesWithoutChangingStoredOrder(tt *testing.T) {
	a := appFixture(tt)
	r, err := a.store.Publish(a.review.ID, "agent", []AnnotationInput{annotation(a.review, "one"), annotation(a.review, "two"), annotation(a.review, "three")}, true)
	require.NoError(tt, err)
	a.apply(r)
	for i := 0; i < 6; i++ {
		a.nextThread(1)
		expected := r.Threads[i%3].ID
		require.Equal(tt, expected, a.activeThread)
		for j, section := range a.sections {
			if section.Thread != nil {
				require.Equal(tt, expected, section.Thread.ID)
				require.True(tt, a.sectionEndsAt(a.sections[j-1], section.Thread.Anchor))
				break
			}
		}
	}
	stored, err := a.store.Read(r.ID)
	require.NoError(tt, err)
	for i := range r.Threads {
		require.Equal(tt, r.Threads[i].ID, stored.Threads[i].ID)
	}
}

func TestReviewNavigationFitsReplyAndCodeWhenPossible(tt *testing.T) {
	for _, width := range []int{80, 120} {
		tt.Run(fmt.Sprint(width), func(tt *testing.T) {
			a := appFixture(tt)
			r, err := a.store.Publish(a.review.ID, "agent", []AnnotationInput{annotation(a.review, "one"), annotation(a.review, "two")}, true)
			require.NoError(tt, err)
			a.apply(r)
			a.mode.Set(DiffLayoutSideBySide)
			a.renderActive(false)
			const height = 32
			b := uv.NewBuffer(width, height)
			renderer := t.NewRenderer(b, width, height, t.NewFocusManager(), t.NewAnySignal[t.Focusable](nil), t.NewAnySignal[t.Widget](nil))
			renderer.Render(a)
			a.nextThread(1)
			a.nextThread(1)
			a.beginReply()
			a.editor.SetText("Reply remains beside its code.")
			for i := 0; i < 3; i++ {
				renderer.Render(a)
			}
			source, ok := a.sourceRangeBounds(a.pending)
			require.True(tt, ok)
			card := a.threadBounds[a.activeThread]
			offset := a.scroll.Offset.Peek()
			require.LessOrEqual(tt, offset, source.Y)
			require.GreaterOrEqual(tt, offset+a.reviewViewportHeight, card.Y+card.Height)
			screen := reviewBufferText(b, width, height)
			require.Contains(tt, screen, "Reply remains beside its code.")
			require.Contains(tt, screen, "Send reply")
		})
	}
}

func TestOversizedReviewShowsCodeThenPrioritisesReplyEditor(tt *testing.T) {
	a := appFixture(tt)
	input := annotation(a.review, "long")
	input.Body = strings.Repeat("A long review explanation.\n", 30)
	r, err := a.store.Publish(a.review.ID, "agent", []AnnotationInput{input}, true)
	require.NoError(tt, err)
	a.apply(r)
	const width, height = 100, 24
	b := uv.NewBuffer(width, height)
	renderer := t.NewRenderer(b, width, height, t.NewFocusManager(), t.NewAnySignal[t.Focusable](nil), t.NewAnySignal[t.Widget](nil))
	renderer.Render(a)
	a.nextThread(1)
	for i := 0; i < 3; i++ {
		renderer.Render(a)
	}
	screen := reviewBufferText(b, width, height)
	require.Contains(tt, screen, "// Welcome everyone.")
	require.Contains(tt, screen, "Review finding")
	a.beginReply()
	a.editor.SetText("Reply at the end of the long conversation.")
	for i := 0; i < 3; i++ {
		renderer.Render(a)
	}
	screen = reviewBufferText(b, width, height)
	require.Contains(tt, screen, "Reply at the end of the long conversation.")
	require.Contains(tt, screen, "Send reply")
}

func TestFocusedGutterContrastAcrossThemes(tt *testing.T) {
	for _, name := range t.ThemeNames() {
		theme, ok := t.GetTheme(name)
		require.True(tt, ok)
		style := commentGutterStyle(theme, true)
		fg, bg := style.ForegroundColor.ColorAt(1, 1, 0, 0), style.BackgroundColor.ColorAt(1, 1, 0, 0)
		require.GreaterOrEqual(tt, fg.ContrastRatio(bg), 4.5, "unreadable gutter in %s", name)
	}
}

func TestSnapshotAsideSecondCommentAtCode(tt *testing.T) {
	t.SetTheme(t.ThemeNameObsidianTide)
	a := appFixture(tt)
	one, two := annotation(a.review, "one"), annotation(a.review, "two")
	one.Kind = "explanation"
	one.Body = "Keep the original greeting."
	two.Body = "Handle empty input before formatting."
	r, err := a.store.Publish(a.review.ID, "review-agent", []AnnotationInput{one, two}, true)
	require.NoError(tt, err)
	a.apply(r)
	a.mode.Set(DiffLayoutSideBySide)
	a.renderActive(false)
	a.nextThread(1)
	a.nextThread(1)
	t.AssertSnapshot(tt, a, 110, 28, "Second same-range comment is promoted beneath its code with compact spacing and a one-row status strip")
}

func TestSnapshotAsideOldRangeAcrossInsertion(tt *testing.T) {
	t.SetTheme(t.ThemeNameCatppuccinLatte)
	defer t.SetTheme(t.ThemeNameObsidianTide)
	a := appFixture(tt)
	diff := `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,4 +1,5 @@
 package main
 func greet() string {
+    name := "World"
     return "Hello"
 }
`
	r, err := a.store.Open(a.review.Repo, "Old-side review", "worktree", ReviewSnapshot{Diff: diff, Base: "base", Head: "working"})
	require.NoError(tt, err)
	input := annotation(r, "old")
	input.Side = "old"
	input.Start = 2
	input.End = 3
	input.Kind = "explanation"
	input.Body = "Preserve the greeting while adding a name."
	r, err = a.store.Publish(r.ID, "review-agent", []AnnotationInput{input}, true)
	require.NoError(tt, err)
	a.apply(r)
	a.mode.Set(DiffLayoutSideBySide)
	a.renderActive(false)
	a.nextThread(1)
	t.AssertSnapshot(tt, a, 100, 24, "Continuous old-side rail across an inserted row; solid gutter remains readable in a light theme")
}
