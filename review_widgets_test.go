package main

import (
	uv "github.com/charmbracelet/ultraviolet"
	t "github.com/darrenburns/terma"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestReviewCardMouseActions(tt *testing.T) {
	a := appFixture(tt)
	r, err := a.store.Publish(a.review.ID, "agent", []AnnotationInput{annotation(a.review, "click")}, true)
	require.NoError(tt, err)
	a.apply(r)
	width, height := 120, 45
	renderer := t.NewRenderer(uv.NewBuffer(width, height), width, height, t.NewFocusManager(), t.NewAnySignal[t.Focusable](nil), t.NewAnySignal[t.Widget](nil))
	renderer.Render(a)
	click := func(id string) {
		tt.Helper()
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				e := renderer.WidgetAt(x, y)
				if e != nil && e.ID == id {
					clickable, ok := e.EventWidget.(t.Clickable)
					require.True(tt, ok)
					clickable.OnClick(t.MouseEvent{X: x, Y: y, Button: uv.MouseLeft, WidgetID: id})
					return
				}
			}
		}
		tt.Fatalf("button %s has no mouse hit target", id)
	}
	click("resolve-" + r.Threads[0].ID)
	require.Equal(tt, "resolved", a.review.Threads[0].Status)
	renderer.Render(a)
	click("reply-" + r.Threads[0].ID)
	require.True(tt, a.editing.Peek())
	require.True(tt, a.replying)
}

func TestInlineComposerStaysWithItsContext(tt *testing.T) {
	a := appFixture(tt)
	selectNewLine(a)
	a.beginComment()
	require.True(tt, a.editing.Peek())
	composerIndex := -1
	for i, s := range a.sections {
		if s.Composer {
			composerIndex = i
		}
	}
	require.Greater(tt, composerIndex, 0)
	before := a.sections[composerIndex-1].Code.Rendered.Peek().Lines
	require.Equal(tt, a.pending.End, before[len(before)-1].NewLine)
	a.editor.SetText("Draft stays on the selected file.")
	a.selectFile("README.md")
	require.Equal(tt, "main.go", a.active)
	require.Equal(tt, "Draft stays on the selected file.", a.editor.GetText())
	a.cancelComment()
	require.False(tt, a.editing.Peek())
	for _, s := range a.sections {
		require.False(tt, s.Composer)
	}
}
func TestSnapshotAsideInlineReply(tt *testing.T) {
	t.SetTheme(t.ThemeNameObsidianTide)
	a := appFixture(tt)
	r, err := a.store.Publish(a.review.ID, "review-agent", []AnnotationInput{annotation(a.review, "reply")}, true)
	require.NoError(tt, err)
	a.apply(r)
	a.nextThread(1)
	a.beginReply()
	a.editor.SetText("Agreed. Please include a whitespace-only regression case.")
	t.AssertSnapshot(tt, a, 110, 38, "Reply editor belongs inside its comment card, after the original message")
}

func TestAsideSelectCodeAfterCommentCard(tt *testing.T) {
	for _, mode := range []DiffLayoutMode{DiffLayoutUnified, DiffLayoutSideBySide} {
		name := "unified"
		if mode == DiffLayoutSideBySide {
			name = "split"
		}
		tt.Run(name, func(tt *testing.T) {
			a := appFixture(tt)
			r, err := a.store.Publish(a.review.ID, "agent", []AnnotationInput{annotation(a.review, "before-tail")}, true)
			require.NoError(tt, err)
			a.apply(r)
			a.mode.Set(mode)
			a.renderActive(false)
			last := a.sections[len(a.sections)-1].Code
			require.NotNil(tt, last)
			require.NotSame(tt, a.view, last)
			a.selectCode(last)
			track, lane := DiffSelectionTrackUnified, DiffSelectionLaneUnified
			if mode == DiffLayoutSideBySide {
				track, lane = DiffSelectionTrackRight, DiffSelectionLaneRight
			}
			last.StartSelection(track, DiffSelectionPoint{Row: 0, Lane: lane})
			last.StopSelectionDrag()
			anchor, err := a.selectionAnchor()
			require.NoError(tt, err)
			require.Equal(tt, 6, anchor.Start)
			require.Equal(tt, "new", anchor.Side)
			require.Equal(tt, "}", anchor.Quote)
			a.beginComment()
			a.editor.SetText("A comment after an existing card.")
			a.saveComment()
			require.Len(tt, a.review.Threads, 2)
			require.Equal(tt, anchor, a.review.Threads[1].Anchor)
		})
	}
}
