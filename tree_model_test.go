package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildDiffTree_BuildsHierarchyAndAggregatesStats(t *testing.T) {
	files := []*DiffFile{
		testDiffFile("pkg/internal/z.go", 1, 2),
		testDiffFile("pkg/api/a.go", 3, 0),
		testDiffFile("pkg/api/b.go", 2, 1),
		testDiffFile("cmd/main.go", 4, 0),
		testDiffFile("README.md", 5, 0),
		testDiffFile("z-last.txt", 1, 1),
	}

	roots, filePathToTreePath, orderedFilePaths := buildDiffTree(files)
	require.Len(t, roots, 4)

	require.True(t, roots[0].Data.IsDir)
	require.Equal(t, "cmd", roots[0].Data.Name)
	require.True(t, roots[1].Data.IsDir)
	require.Equal(t, "pkg", roots[1].Data.Name)
	require.False(t, roots[2].Data.IsDir)
	require.Equal(t, "README.md", roots[2].Data.Name)
	require.False(t, roots[3].Data.IsDir)
	require.Equal(t, "z-last.txt", roots[3].Data.Name)

	pkg := roots[1]
	require.Equal(t, 6, pkg.Data.Additions)
	require.Equal(t, 3, pkg.Data.Deletions)
	require.Equal(t, 3, pkg.Data.TouchedFiles)
	require.Len(t, pkg.Children, 2)
	require.Equal(t, "api", pkg.Children[0].Data.Name)
	require.Equal(t, "internal", pkg.Children[1].Data.Name)

	api := pkg.Children[0]
	require.Len(t, api.Children, 2)
	require.Equal(t, "a.go", api.Children[0].Data.Name)
	require.Equal(t, "b.go", api.Children[1].Data.Name)

	require.Equal(t, []string{
		"cmd/main.go",
		"pkg/api/a.go",
		"pkg/api/b.go",
		"pkg/internal/z.go",
		"README.md",
		"z-last.txt",
	}, orderedFilePaths)

	require.Equal(t, []int{0, 0}, filePathToTreePath["cmd/main.go"])
	require.Equal(t, []int{1, 0, 0}, filePathToTreePath["pkg/api/a.go"])
	require.Equal(t, []int{1, 1, 0}, filePathToTreePath["pkg/internal/z.go"])
	require.Equal(t, []int{2}, filePathToTreePath["README.md"])
}

func TestBuildDiffTree_SortsDirectoriesBeforeFilesInEachFolder(t *testing.T) {
	files := []*DiffFile{
		testDiffFile("src/z-last.go", 1, 0),
		testDiffFile("src/a-first.go", 1, 0),
		testDiffFile("src/pkg/util.go", 1, 0),
	}

	roots, _, _ := buildDiffTree(files)
	require.Len(t, roots, 1)
	src := roots[0]
	require.Len(t, src.Children, 3)

	require.True(t, src.Children[0].Data.IsDir)
	require.Equal(t, "pkg", src.Children[0].Data.Name)
	require.False(t, src.Children[1].Data.IsDir)
	require.Equal(t, "a-first.go", src.Children[1].Data.Name)
	require.False(t, src.Children[2].Data.IsDir)
	require.Equal(t, "z-last.go", src.Children[2].Data.Name)
}

func testDiffFile(path string, additions int, deletions int) *DiffFile {
	return &DiffFile{
		DisplayPath: path,
		Additions:   additions,
		Deletions:   deletions,
	}
}
