package main

// DiffLineKind describes the type of a single line inside a unified diff hunk.
type DiffLineKind int

const (
	DiffLineContext DiffLineKind = iota
	DiffLineAdd
	DiffLineRemove
	DiffLineMeta
)

// DiffLine is a single parsed line from a hunk.
type DiffLine struct {
	Kind    DiffLineKind
	Content string
	OldLine int
	NewLine int
}

// DiffHunk is a parsed unified diff hunk.
type DiffHunk struct {
	Header   string
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	Lines    []DiffLine
}

// DiffFile is a parsed diff for a single file.
type DiffFile struct {
	OldPath     string
	NewPath     string
	DisplayPath string
	Headers     []string
	Hunks       []DiffHunk
	IsBinary    bool
	Additions   int
	Deletions   int
}

// DiffDocument is the parsed representation of a full git diff output.
type DiffDocument struct {
	Files []*DiffFile
}
