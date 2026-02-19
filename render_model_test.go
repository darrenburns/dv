package main

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestBuildRenderedFile_StructuredLinesAndGutters(t *testing.T) {
	file := &DiffFile{
		NewPath:     "main.go",
		DisplayPath: "main.go",
		Headers:     []string{"diff --git a/main.go b/main.go", "--- a/main.go", "+++ b/main.go"},
		Hunks: []DiffHunk{
			{
				Header: "@@ -1,2 +1,2 @@",
				Lines: []DiffLine{
					{Kind: DiffLineContext, Content: "package main", OldLine: 1, NewLine: 1},
					{Kind: DiffLineRemove, Content: "fmt.Println(\"old\")", OldLine: 2},
					{Kind: DiffLineAdd, Content: "fmt.Println(\"new\")", NewLine: 2},
				},
			},
		},
	}

	rendered := buildRenderedFile(file)
	require.NotNil(t, rendered)
	require.Equal(t, "main.go", rendered.Title)
	require.Len(t, rendered.Lines, 4)
	require.Equal(t, 1, rendered.OldNumWidth)
	require.Equal(t, 1, rendered.NewNumWidth)

	hunk := rendered.Lines[0]
	require.Equal(t, RenderedLineHunkHeader, hunk.Kind)
	require.Equal(t, "@@ -1,2 +1,2 @@", lineText(hunk))

	context := rendered.Lines[1]
	require.Equal(t, RenderedLineContext, context.Kind)
	require.Equal(t, 1, context.OldLine)
	require.Equal(t, 1, context.NewLine)
	require.Equal(t, " ", context.Prefix)
	require.Equal(t, "package main", lineText(context))

	remove := rendered.Lines[2]
	require.Equal(t, RenderedLineRemove, remove.Kind)
	require.Equal(t, 2, remove.OldLine)
	require.Equal(t, 0, remove.NewLine)
	require.Equal(t, "-", remove.Prefix)
	require.Equal(t, "fmt.Println(\"old\")", lineText(remove))

	add := rendered.Lines[3]
	require.Equal(t, RenderedLineAdd, add.Kind)
	require.Equal(t, 0, add.OldLine)
	require.Equal(t, 2, add.NewLine)
	require.Equal(t, "+", add.Prefix)
	require.Equal(t, "fmt.Println(\"new\")", lineText(add))
}

func TestBuildRenderedFile_ExpandsTabsWithTabStops(t *testing.T) {
	file := &DiffFile{
		NewPath:     "main.go",
		DisplayPath: "main.go",
		Hunks: []DiffHunk{
			{
				Header: "@@ -1,1 +1,1 @@",
				Lines: []DiffLine{
					{Kind: DiffLineContext, Content: "\tfoo\tbar", OldLine: 1, NewLine: 1},
				},
			},
		},
	}

	rendered := buildRenderedFile(file)
	require.NotNil(t, rendered)
	require.Len(t, rendered.Lines, 2)

	line := rendered.Lines[1]
	require.Equal(t, RenderedLineContext, line.Kind)
	require.Equal(t, "    foo bar", lineText(line))
	require.Equal(t, ansi.StringWidth("    foo bar"), line.ContentWidth)
}

func TestBuildMetaRenderedFile_ProducesMetaLines(t *testing.T) {
	rendered := buildMetaRenderedFile("Summary", []string{"Line one", "Line two"})
	require.NotNil(t, rendered)
	require.Equal(t, "Summary", rendered.Title)
	require.Len(t, rendered.Lines, 2)
	require.Equal(t, RenderedLineMeta, rendered.Lines[0].Kind)
	require.Equal(t, "Line one", lineText(rendered.Lines[0]))
	require.Equal(t, "Line two", lineText(rendered.Lines[1]))
}

func TestBuildSideBySideRenderedFile_RunPairingAndSharedRows(t *testing.T) {
	file := &DiffFile{
		DisplayPath: "main.go",
		Hunks: []DiffHunk{
			{
				Header: "@@ -1,4 +1,4 @@",
				Lines: []DiffLine{
					{Kind: DiffLineRemove, Content: "old first", OldLine: 1},
					{Kind: DiffLineRemove, Content: "old second", OldLine: 2},
					{Kind: DiffLineAdd, Content: "new first", NewLine: 1},
					{Kind: DiffLineContext, Content: "same", OldLine: 3, NewLine: 2},
					{Kind: DiffLineMeta, Content: `\\ No newline at end of file`},
					{Kind: DiffLineAdd, Content: "tail", NewLine: 3},
				},
			},
		},
	}

	side := buildSideBySideRenderedFile(file)
	require.NotNil(t, side)
	require.Equal(t, "main.go", side.Title)
	require.Len(t, side.Rows, 6)

	require.NotNil(t, side.Rows[0].Shared)
	require.Equal(t, RenderedLineHunkHeader, side.Rows[0].Shared.Kind)
	require.Equal(t, "@@ -1,4 +1,4 @@", lineText(*side.Rows[0].Shared))

	require.NotNil(t, side.Rows[1].Left)
	require.NotNil(t, side.Rows[1].Right)
	require.Equal(t, RenderedLineRemove, side.Rows[1].Left.Kind)
	require.Equal(t, RenderedLineAdd, side.Rows[1].Right.Kind)
	require.Equal(t, "old first", sideCellText(side.Rows[1].Left))
	require.Equal(t, "new first", sideCellText(side.Rows[1].Right))

	require.NotNil(t, side.Rows[2].Left)
	require.Nil(t, side.Rows[2].Right)
	require.Equal(t, "old second", sideCellText(side.Rows[2].Left))

	require.NotNil(t, side.Rows[3].Left)
	require.NotNil(t, side.Rows[3].Right)
	require.Equal(t, RenderedLineContext, side.Rows[3].Left.Kind)
	require.Equal(t, RenderedLineContext, side.Rows[3].Right.Kind)

	require.NotNil(t, side.Rows[4].Shared)
	require.Equal(t, RenderedLineMeta, side.Rows[4].Shared.Kind)
	require.Equal(t, `\\ No newline at end of file`, lineText(*side.Rows[4].Shared))

	require.Nil(t, side.Rows[5].Left)
	require.NotNil(t, side.Rows[5].Right)
	require.Equal(t, "tail", sideCellText(side.Rows[5].Right))
	require.GreaterOrEqual(t, side.LeftNumWidth, 1)
	require.GreaterOrEqual(t, side.RightNumWidth, 1)
	require.GreaterOrEqual(t, side.LeftMaxContentWidth, len("old second"))
	require.GreaterOrEqual(t, side.RightMaxContentWidth, len("new first"))
}

func TestBuildSideBySideFromRendered_UsesSharedRows(t *testing.T) {
	rendered := buildMetaRenderedFile("Summary", []string{"line one", "line two"})

	side := buildSideBySideFromRendered(rendered)
	require.NotNil(t, side)
	require.Equal(t, "Summary", side.Title)
	require.Len(t, side.Rows, 2)
	require.NotNil(t, side.Rows[0].Shared)
	require.NotNil(t, side.Rows[1].Shared)
	require.Equal(t, "line one", lineText(*side.Rows[0].Shared))
	require.Equal(t, "line two", lineText(*side.Rows[1].Shared))
	require.Equal(t, rendered.OldNumWidth, side.LeftNumWidth)
	require.Equal(t, rendered.NewNumWidth, side.RightNumWidth)
	require.GreaterOrEqual(t, side.LeftMaxContentWidth, rendered.MaxContentWidth)
	require.GreaterOrEqual(t, side.RightMaxContentWidth, rendered.MaxContentWidth)
}

func sideCellText(cell *RenderedSideCell) string {
	if cell == nil || len(cell.Segments) == 0 {
		return ""
	}
	var out string
	for _, segment := range cell.Segments {
		out += segment.Text
	}
	return out
}
