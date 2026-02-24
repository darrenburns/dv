package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	t "github.com/darrenburns/terma"
	"github.com/stretchr/testify/require"
)

func TestCompactPathMiddle_PathFitsUnchanged(tt *testing.T) {
	path := "src/main.go"
	require.Equal(tt, path, compactPathMiddle(path, ansi.StringWidth(path)))
	require.Equal(tt, path, compactPathMiddle(path, ansi.StringWidth(path)+4))
	require.Equal(tt, "a", compactPathMiddle("a", 1))
}

func TestCompactPathMiddle_DeepPathPreservesFilenameTail(tt *testing.T) {
	path := "src/github.com/org/project/internal/very/deep/path/file.go"
	got := compactPathMiddle(path, 20)

	require.Contains(tt, got, pathEllipsis)
	require.True(tt, strings.HasSuffix(got, "file.go"))
	require.LessOrEqual(tt, ansi.StringWidth(got), 20)

	parts := strings.SplitN(got, pathEllipsis, 2)
	require.Len(tt, parts, 2)
	require.True(tt, strings.HasPrefix(path, parts[0]))
	require.True(tt, strings.HasSuffix(path, parts[1]))
}

func TestCompactPathMiddle_VeryLongFilenameUsesTail(tt *testing.T) {
	path := "dir/supercalifragilisticexpialidocious.txt"
	got := compactPathMiddle(path, 12)

	require.Contains(tt, got, pathEllipsis)
	require.LessOrEqual(tt, ansi.StringWidth(got), 12)

	parts := strings.SplitN(got, pathEllipsis, 2)
	require.Len(tt, parts, 2)
	tail := parts[1]
	require.NotEmpty(tt, tail)
	require.True(tt, strings.HasSuffix(path, tail))
	require.True(tt, strings.HasSuffix(tail, ".txt"))
}

func TestCompactPathMiddle_EdgeWidths(tt *testing.T) {
	path := "abcdef"
	require.Equal(tt, "", compactPathMiddle(path, 0))
	require.Equal(tt, pathEllipsis, compactPathMiddle(path, 1))
	require.Equal(tt, "a"+pathEllipsis, compactPathMiddle(path, 2))
	require.Equal(tt, "a"+pathEllipsis+"f", compactPathMiddle(path, 3))
}

func TestCompactPathMiddle_WindowsPathSeparators(tt *testing.T) {
	path := `C:\projects\really\long\windows\path\file.txt`
	got := compactPathMiddle(path, 18)

	require.Contains(tt, got, pathEllipsis)
	require.True(tt, strings.HasSuffix(got, "file.txt"))
	require.LessOrEqual(tt, ansi.StringWidth(got), 18)
}

func TestViewerPathText_BuildReturnsWrapper(tt *testing.T) {
	widget := viewerPathText{
		Text: t.Text{
			Content: "src/main.go",
		},
		FullPath: "src/main.go",
	}

	var ctx t.BuildContext
	built := widget.Build(ctx)
	_, ok := built.(viewerPathText)
	require.True(tt, ok)
}
