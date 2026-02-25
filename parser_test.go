package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseUnifiedDiff_MultiFile(t *testing.T) {
	raw := `diff --git a/main.go b/main.go
index 1234567..89abcde 100644
--- a/main.go
+++ b/main.go
@@ -1,3 +1,4 @@
 package main
-fmt.Println("old")
+fmt.Println("new")
+fmt.Println("extra")
 return
diff --git a/README.md b/README.md
new file mode 100644
index 0000000..1111111
--- /dev/null
+++ b/README.md
@@ -0,0 +1,2 @@
+# Title
+body
`

	doc, err := parseUnifiedDiff(raw)
	require.NoError(t, err)
	require.Len(t, doc.Files, 2)

	first := doc.Files[0]
	require.Equal(t, "main.go", first.DisplayPath)
	require.Equal(t, 2, first.Additions)
	require.Equal(t, 1, first.Deletions)
	require.Len(t, first.Hunks, 1)
	require.Len(t, first.Hunks[0].Lines, 5)

	require.Equal(t, DiffLineContext, first.Hunks[0].Lines[0].Kind)
	require.Equal(t, 1, first.Hunks[0].Lines[0].OldLine)
	require.Equal(t, 1, first.Hunks[0].Lines[0].NewLine)

	require.Equal(t, DiffLineRemove, first.Hunks[0].Lines[1].Kind)
	require.Equal(t, 2, first.Hunks[0].Lines[1].OldLine)
	require.Equal(t, 0, first.Hunks[0].Lines[1].NewLine)

	require.Equal(t, DiffLineAdd, first.Hunks[0].Lines[2].Kind)
	require.Equal(t, 0, first.Hunks[0].Lines[2].OldLine)
	require.Equal(t, 2, first.Hunks[0].Lines[2].NewLine)

	require.Equal(t, DiffLineAdd, first.Hunks[0].Lines[3].Kind)
	require.Equal(t, 0, first.Hunks[0].Lines[3].OldLine)
	require.Equal(t, 3, first.Hunks[0].Lines[3].NewLine)

	require.Equal(t, DiffLineContext, first.Hunks[0].Lines[4].Kind)
	require.Equal(t, 3, first.Hunks[0].Lines[4].OldLine)
	require.Equal(t, 4, first.Hunks[0].Lines[4].NewLine)

	second := doc.Files[1]
	require.Equal(t, "README.md", second.DisplayPath)
	require.Equal(t, 2, second.Additions)
	require.Equal(t, 0, second.Deletions)
}

func TestParseUnifiedDiff_RenameAndBinary(t *testing.T) {
	raw := `diff --git a/old.png b/new.png
similarity index 100%
rename from old.png
rename to new.png
Binary files a/old.png and b/new.png differ
`

	doc, err := parseUnifiedDiff(raw)
	require.NoError(t, err)
	require.Len(t, doc.Files, 1)

	file := doc.Files[0]
	require.True(t, file.IsBinary)
	require.Equal(t, "old.png", file.OldPath)
	require.Equal(t, "new.png", file.NewPath)
	require.Equal(t, "new.png", file.DisplayPath)
	require.Empty(t, file.Hunks)
}

func TestParseUnifiedDiff_StripsANSIAroundStructuralLines(t *testing.T) {
	esc := "\x1b"
	color := esc + "[38;2;117;113;94m"
	reset := esc + "[0m"

	raw := strings.Join([]string{
		color + "diff --git a/justfile b/justfile" + reset,
		color + "new file mode 100644" + reset,
		color + "index 0000000000..2570325b6a" + reset,
		color + "--- /dev/null" + reset,
		color + "+++ b/justfile" + reset,
		color + "@@ -0,0 +1,1 @@" + reset,
		color + "+set shell := [\"bash\", \"-eu\", \"-o\", \"pipefail\", \"-c\"]" + reset,
	}, "\n") + "\n"

	doc, err := parseUnifiedDiff(raw)
	require.NoError(t, err)
	require.Len(t, doc.Files, 1)

	file := doc.Files[0]
	require.Equal(t, "justfile", file.DisplayPath)
	require.Equal(t, 1, file.Additions)
	require.Equal(t, 0, file.Deletions)
	require.Len(t, file.Hunks, 1)
	require.Len(t, file.Hunks[0].Lines, 1)
	require.Equal(t, DiffLineAdd, file.Hunks[0].Lines[0].Kind)
	require.Equal(t, `set shell := ["bash", "-eu", "-o", "pipefail", "-c"]`, file.Hunks[0].Lines[0].Content)
}

func TestParseUnifiedDiff_StripsANSIFromInlineHunkContent(t *testing.T) {
	esc := "\x1b"

	raw := strings.Join([]string{
		"diff --git a/main.go b/main.go",
		"index 1234567..89abcde 100644",
		"--- a/main.go",
		"+++ b/main.go",
		"@@ -1 +1 @@",
		"-" + esc + "[31mfmt.Println(\"old\")" + esc + "[0m",
		"+" + esc + "[32mfmt.Println(\"new\")" + esc + "[0m",
	}, "\n") + "\n"

	doc, err := parseUnifiedDiff(raw)
	require.NoError(t, err)
	require.Len(t, doc.Files, 1)
	require.Len(t, doc.Files[0].Hunks, 1)
	require.Len(t, doc.Files[0].Hunks[0].Lines, 2)

	remove := doc.Files[0].Hunks[0].Lines[0]
	require.Equal(t, DiffLineRemove, remove.Kind)
	require.Equal(t, `fmt.Println("old")`, remove.Content)
	require.NotContains(t, remove.Content, esc)

	add := doc.Files[0].Hunks[0].Lines[1]
	require.Equal(t, DiffLineAdd, add.Kind)
	require.Equal(t, `fmt.Println("new")`, add.Content)
	require.NotContains(t, add.Content, esc)
}
