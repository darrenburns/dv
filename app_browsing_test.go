package main

import (
	uv "github.com/charmbracelet/ultraviolet"
	t "github.com/darrenburns/terma"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestAsideRestoredBrowsingControls(tt *testing.T) {
	a := appFixture(tt)
	a.allFiles.Set(true)
	a.rebuildFiles()
	a.scroll.Offset.Set(12)
	a.selectFile("README.md")
	a.scroll.Offset.Set(3)
	a.selectFile("main.go")
	require.Equal(tt, 12, a.scroll.Offset.Peek())
	a.toggleLayout()
	a.scroll.Offset.Set(7)
	a.toggleLayout()
	require.Equal(tt, 12, a.scroll.Offset.Peek())
	a.toggleLayout()
	require.Equal(tt, 7, a.scroll.Offset.Peek())
	a.markSeen()
	require.True(tt, a.isSeen("main.go"))
	a.files["README.md"].Additions++
	require.True(tt, a.isSeen("main.go"))
	a.files["main.go"].Hunks[0].Lines[0].Content = "package changed"
	require.False(tt, a.isSeen("main.go"))
	a.markSeen()
	a.clearSeen()
	require.False(tt, a.isSeen("main.go"))
	a.toggleSidebar()
	require.False(tt, a.sidebarVisible.Peek())
	a.openFilter()
	require.True(tt, a.sidebarVisible.Peek())
	a.filter.SetText("mng")
	a.rebuildFiles()
	require.Equal(tt, []string{"main.go"}, a.paths)
	a.filtering.Set(false)
	a.escape()
	require.Empty(tt, a.filter.GetText())
	require.Len(tt, a.paths, 2)
}
func TestAsideCopySelectionBeforePath(tt *testing.T) {
	a := appFixture(tt)
	var copied string
	a.copyFeedback = func(text string) error { copied = text; return nil }
	a.copySelectionOrPath()
	require.Equal(tt, "main.go", copied)
	selectNewLine(a)
	a.view.UpdateSelection(DiffSelectionPoint{Row: a.view.selectionAnchor.Peek().Row, Grapheme: 100, Lane: DiffSelectionLaneUnified})
	a.copySelectionOrPath()
	require.Contains(tt, copied, "世界")
}
func TestAsideThemePreviewCancelAndAccept(tt *testing.T) {
	t.SetTheme(t.ThemeNameObsidianTide)
	tt.Cleanup(func() { t.SetTheme(t.ThemeNameObsidianTide) })
	a := appFixture(tt)
	a.openThemes()
	var candidate t.CommandPaletteItem
	for _, item := range a.palette.CurrentLevel().Items {
		if item.Data == t.ThemeNameCatppuccin {
			candidate = item
			break
		}
	}
	require.NotNil(tt, candidate.Action)
	a.previewTheme(candidate)
	require.Equal(tt, t.ThemeNameCatppuccin, t.CurrentThemeName())
	a.dismissPalette()
	a.palette.Close()
	require.Equal(tt, t.ThemeNameObsidianTide, t.CurrentThemeName())
	a.openThemes()
	a.previewTheme(candidate)
	candidate.Action()
	require.Equal(tt, t.ThemeNameCatppuccin, t.CurrentThemeName())
	a.dismissPalette()
	require.Equal(tt, t.ThemeNameCatppuccin, t.CurrentThemeName())
}
func TestAsidePaletteHasOnlyReadOnlyRepositoryActions(tt *testing.T) {
	a := appFixture(tt)
	for _, item := range a.paletteItems() {
		label := strings.ToLower(item.Label)
		for _, forbidden := range []string{"stage", "commit", "push", "checkout"} {
			require.NotContains(tt, label, forbidden)
		}
	}
}
func TestSnapshotAsideCommandPalette(tt *testing.T) {
	t.SetTheme(t.ThemeNameObsidianTide)
	a := appFixture(tt)
	a.openPalette()
	t.AssertSnapshot(tt, a, 110, 30, "Restored searchable read-only command palette over the file browser")
}
func TestSnapshotAsideSeenFiles(tt *testing.T) {
	t.SetTheme(t.ThemeNameObsidianTide)
	a := appFixture(tt)
	a.markSeen()
	t.AssertSnapshot(tt, a, 110, 26, "Seen file strikethrough, progress, colored statistics, and unused-space hatch")
}

func TestAsideThemeMenuDoesNotInterruptComposition(tt *testing.T) {
	a := appFixture(tt)
	selectNewLine(a)
	a.beginComment()
	a.editor.SetText("Keep this draft.")
	a.openPalette()
	a.openThemes()
	require.False(tt, a.palette.Visible.Peek())
	require.Equal(tt, "Keep this draft.", a.editor.GetText())
}
func TestSnapshotAsideDraftNarrow(tt *testing.T) {
	t.SetTheme(t.ThemeNameObsidianTide)
	a := appFixture(tt)
	selectNewLine(a)
	a.beginComment()
	a.editor.SetText("Preserve the greeting.")
	a.saveComment()
	t.AssertSnapshot(tt, a, 65, 30, "Draft action buttons remain visible at 65 columns with integrated shortcuts")
}

func TestAsideHorizontalScrollWithLongLines(tt *testing.T) {
	a := appFixture(tt)
	diff := strings.Replace(reviewFixture, "Hello, 世界", strings.Repeat("abcdefghij", 20), 1)
	r, err := a.store.Open(a.review.Repo, a.review.Title, a.review.Source, ReviewSnapshot{Diff: diff, Base: "base", Head: "working-tree"})
	require.NoError(tt, err)
	a.apply(r)
	a.toggleWrap()
	renderer := t.NewRenderer(uv.NewBuffer(100, 30), 100, 30, t.NewFocusManager(), t.NewAnySignal[t.Focusable](nil), t.NewAnySignal[t.Widget](nil))
	renderer.Render(a)
	require.True(tt, a.scrollHorizontal(12))
	require.Equal(tt, 12, a.view.ScrollX.Peek())
	a.toggleWrap()
	renderer.Render(a)
	require.Equal(tt, 0, a.view.ScrollX.Peek())
	require.False(tt, a.scrollHorizontal(12))
}
