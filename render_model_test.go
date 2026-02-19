package main

import (
	"strings"
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

func TestBuildRenderedFile_IntralineMarksChangedWordChunks(t *testing.T) {
	file := &DiffFile{
		DisplayPath: "main.go",
		Hunks: []DiffHunk{
			{
				Header: "@@ -1,1 +1,1 @@",
				Lines: []DiffLine{
					{Kind: DiffLineRemove, Content: "prefix value suffix", OldLine: 1},
					{Kind: DiffLineAdd, Content: "prefix valve suffix", NewLine: 1},
				},
			},
		},
	}

	rendered := buildRenderedFile(file)
	require.NotNil(t, rendered)
	require.Len(t, rendered.Lines, 3)

	remove := rendered.Lines[1]
	add := rendered.Lines[2]

	require.Equal(t, indexRange(7, 12), markedIndicesForLine(remove, IntralineMarkRemove))
	require.Equal(t, indexRange(7, 12), markedIndicesForLine(add, IntralineMarkAdd))
}

func TestBuildRenderedFile_IntralineMarksInsertionsAndDeletionsAtEdges(t *testing.T) {
	file := &DiffFile{
		DisplayPath: "main.go",
		Hunks: []DiffHunk{
			{
				Header: "@@ -1,2 +1,2 @@",
				Lines: []DiffLine{
					{Kind: DiffLineRemove, Content: "middle", OldLine: 1},
					{Kind: DiffLineAdd, Content: "xmiddle", NewLine: 1},
					{Kind: DiffLineRemove, Content: "trailx", OldLine: 2},
					{Kind: DiffLineAdd, Content: "trail", NewLine: 2},
				},
			},
		},
	}

	rendered := buildRenderedFile(file)
	require.NotNil(t, rendered)
	require.Len(t, rendered.Lines, 5)

	require.Empty(t, markedIndicesForLine(rendered.Lines[1], IntralineMarkRemove))
	require.Empty(t, markedIndicesForLine(rendered.Lines[2], IntralineMarkAdd))

	require.Empty(t, markedIndicesForLine(rendered.Lines[3], IntralineMarkRemove))
	require.Empty(t, markedIndicesForLine(rendered.Lines[4], IntralineMarkAdd))
}

func TestBuildRenderedFile_IntralineMarksOnlyPairedLinesInUnevenRuns(t *testing.T) {
	file := &DiffFile{
		DisplayPath: "main.go",
		Hunks: []DiffHunk{
			{
				Header: "@@ -1,2 +1,1 @@",
				Lines: []DiffLine{
					{Kind: DiffLineRemove, Content: "alphaX", OldLine: 1},
					{Kind: DiffLineRemove, Content: "second-old", OldLine: 2},
					{Kind: DiffLineAdd, Content: "alphaY", NewLine: 1},
				},
			},
		},
	}

	rendered := buildRenderedFile(file)
	require.NotNil(t, rendered)
	require.Len(t, rendered.Lines, 4)

	require.Empty(t, markedIndicesForLine(rendered.Lines[1], IntralineMarkRemove))
	require.Empty(t, markedIndicesForLine(rendered.Lines[2], IntralineMarkRemove))
	require.Empty(t, markedIndicesForLine(rendered.Lines[3], IntralineMarkAdd))

	side := buildSideBySideRenderedFile(file)
	require.NotNil(t, side)
	require.Len(t, side.Rows, 3)

	require.NotNil(t, side.Rows[1].Left)
	require.NotNil(t, side.Rows[1].Right)
	require.Empty(t, markedIndicesForSideCell(side.Rows[1].Left, IntralineMarkRemove))
	require.Empty(t, markedIndicesForSideCell(side.Rows[1].Right, IntralineMarkAdd))

	require.NotNil(t, side.Rows[2].Left)
	require.Nil(t, side.Rows[2].Right)
	require.Empty(t, markedIndicesForSideCell(side.Rows[2].Left, IntralineMarkRemove))
}

func TestBuildRenderedFile_IntralineMarksSimilarPrefixBeforeDivergence(t *testing.T) {
	file := &DiffFile{
		DisplayPath: "main.go",
		Hunks: []DiffHunk{
			{
				Header: "@@ -1,3 +1,8 @@",
				Lines: []DiffLine{
					{Kind: DiffLineRemove, Content: "alpha one", OldLine: 1},
					{Kind: DiffLineRemove, Content: "beta two", OldLine: 2},
					{Kind: DiffLineRemove, Content: "gamma three", OldLine: 3},
					{Kind: DiffLineAdd, Content: "alpha won", NewLine: 1},
					{Kind: DiffLineAdd, Content: "beta too", NewLine: 2},
					{Kind: DiffLineAdd, Content: "gamma tree", NewLine: 3},
					{Kind: DiffLineAdd, Content: "completely brand new line A", NewLine: 4},
					{Kind: DiffLineAdd, Content: "completely brand new line B", NewLine: 5},
					{Kind: DiffLineAdd, Content: "completely brand new line C", NewLine: 6},
					{Kind: DiffLineAdd, Content: "completely brand new line D", NewLine: 7},
					{Kind: DiffLineAdd, Content: "completely brand new line E", NewLine: 8},
				},
			},
		},
	}

	rendered := buildRenderedFile(file)
	require.NotNil(t, rendered)
	require.Len(t, rendered.Lines, 12)

	require.NotEmpty(t, markedIndicesForLine(rendered.Lines[1], IntralineMarkRemove))
	require.NotEmpty(t, markedIndicesForLine(rendered.Lines[2], IntralineMarkRemove))
	require.NotEmpty(t, markedIndicesForLine(rendered.Lines[3], IntralineMarkRemove))
	require.NotEmpty(t, markedIndicesForLine(rendered.Lines[4], IntralineMarkAdd))
	require.NotEmpty(t, markedIndicesForLine(rendered.Lines[5], IntralineMarkAdd))
	require.NotEmpty(t, markedIndicesForLine(rendered.Lines[6], IntralineMarkAdd))

	// Unpaired additions after the shared prefix should not receive intraline marks.
	for idx := 7; idx <= 11; idx++ {
		require.Empty(t, markedIndicesForLine(rendered.Lines[idx], IntralineMarkAdd))
	}

	side := buildSideBySideRenderedFile(file)
	require.NotNil(t, side)
	require.Len(t, side.Rows, 9)

	for idx := 1; idx <= 3; idx++ {
		require.NotNil(t, side.Rows[idx].Left)
		require.NotNil(t, side.Rows[idx].Right)
		require.NotEmpty(t, markedIndicesForSideCell(side.Rows[idx].Left, IntralineMarkRemove))
		require.NotEmpty(t, markedIndicesForSideCell(side.Rows[idx].Right, IntralineMarkAdd))
	}

	for idx := 4; idx <= 8; idx++ {
		require.Nil(t, side.Rows[idx].Left)
		require.NotNil(t, side.Rows[idx].Right)
		require.Empty(t, markedIndicesForSideCell(side.Rows[idx].Right, IntralineMarkAdd))
	}
}

func TestBuildRenderedFile_IntralineDPFallbackKeepsWholeLineHighlighting(t *testing.T) {
	oldTokens := make([]string, 0, 450)
	newTokens := make([]string, 0, 450)
	for idx := 0; idx < 450; idx++ {
		oldTokens = append(oldTokens, "a")
		newTokens = append(newTokens, "b")
	}
	oldLine := strings.Join(oldTokens, " ")
	newLine := strings.Join(newTokens, " ")

	file := &DiffFile{
		DisplayPath: "main.go",
		Hunks: []DiffHunk{
			{
				Header: "@@ -1,1 +1,1 @@",
				Lines: []DiffLine{
					{Kind: DiffLineRemove, Content: oldLine, OldLine: 1},
					{Kind: DiffLineAdd, Content: newLine, NewLine: 1},
				},
			},
		},
	}

	rendered := buildRenderedFile(file)
	require.NotNil(t, rendered)
	require.Len(t, rendered.Lines, 3)
	require.Empty(t, markedIndicesForLine(rendered.Lines[1], IntralineMarkRemove))
	require.Empty(t, markedIndicesForLine(rendered.Lines[2], IntralineMarkAdd))
}

func TestBuildRenderedFile_IntralineSuppressesWhenMostOfBothLinesWouldBeMarked(t *testing.T) {
	file := &DiffFile{
		DisplayPath: "main.go",
		Hunks: []DiffHunk{
			{
				Header: "@@ -1,1 +1,1 @@",
				Lines: []DiffLine{
					{Kind: DiffLineRemove, Content: "aaaaaaaaaa", OldLine: 1},
					{Kind: DiffLineAdd, Content: "bbbbbbbbbb", NewLine: 1},
				},
			},
		},
	}

	rendered := buildRenderedFile(file)
	require.NotNil(t, rendered)
	require.Len(t, rendered.Lines, 3)
	require.Empty(t, markedIndicesForLine(rendered.Lines[1], IntralineMarkRemove))
	require.Empty(t, markedIndicesForLine(rendered.Lines[2], IntralineMarkAdd))
}

func TestBuildRenderedFile_IntralineStopsOnEmptyVsNonEmptyPair(t *testing.T) {
	file := &DiffFile{
		DisplayPath: "main.go",
		Hunks: []DiffHunk{
			{
				Header: "@@ -1,2 +1,2 @@",
				Lines: []DiffLine{
					{Kind: DiffLineRemove, Content: "", OldLine: 1},
					{Kind: DiffLineRemove, Content: "return valueA", OldLine: 2},
					{Kind: DiffLineAdd, Content: "        blocks := buildHunkRenderBlocks(hunk, lexer)", NewLine: 1},
					{Kind: DiffLineAdd, Content: "return valueB", NewLine: 2},
				},
			},
		},
	}

	rendered := buildRenderedFile(file)
	require.NotNil(t, rendered)
	require.Len(t, rendered.Lines, 5)

	// First pair should be treated as divergent, so no intraline on the addition.
	require.Empty(t, markedIndicesForLine(rendered.Lines[3], IntralineMarkAdd))
	// Matching should stop for the rest of the run.
	require.Empty(t, markedIndicesForLine(rendered.Lines[2], IntralineMarkRemove))
	require.Empty(t, markedIndicesForLine(rendered.Lines[4], IntralineMarkAdd))

	side := buildSideBySideRenderedFile(file)
	require.NotNil(t, side)
	require.Len(t, side.Rows, 3)
	require.NotNil(t, side.Rows[1].Left)
	require.NotNil(t, side.Rows[1].Right)
	require.Empty(t, markedIndicesForSideCell(side.Rows[1].Left, IntralineMarkRemove))
	require.Empty(t, markedIndicesForSideCell(side.Rows[1].Right, IntralineMarkAdd))
	require.NotNil(t, side.Rows[2].Left)
	require.NotNil(t, side.Rows[2].Right)
	require.Empty(t, markedIndicesForSideCell(side.Rows[2].Left, IntralineMarkRemove))
	require.Empty(t, markedIndicesForSideCell(side.Rows[2].Right, IntralineMarkAdd))
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

func markedIndicesForLine(line RenderedDiffLine, mark IntralineMarkKind) []int {
	return markedIndicesForSegments(line.Segments, mark)
}

func markedIndicesForSideCell(cell *RenderedSideCell, mark IntralineMarkKind) []int {
	if cell == nil {
		return nil
	}
	return markedIndicesForSegments(cell.Segments, mark)
}

func markedIndicesForSegments(segments []RenderedSegment, mark IntralineMarkKind) []int {
	indices := []int{}
	graphemeIndex := 0
	for _, segment := range segments {
		remaining := segment.Text
		for len(remaining) > 0 {
			grapheme, _ := ansi.FirstGraphemeCluster(remaining, ansi.GraphemeWidth)
			if grapheme == "" {
				break
			}
			if segment.Intraline == mark {
				indices = append(indices, graphemeIndex)
			}
			graphemeIndex++
			remaining = remaining[len(grapheme):]
		}
	}
	return indices
}

func indexRange(start int, endExclusive int) []int {
	if endExclusive <= start {
		return nil
	}
	indices := make([]int, 0, endExclusive-start)
	for idx := start; idx < endExclusive; idx++ {
		indices = append(indices, idx)
	}
	return indices
}
