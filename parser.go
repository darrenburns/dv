package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var hunkHeaderPattern = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

func parseUnifiedDiff(raw string) (*DiffDocument, error) {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	if strings.TrimSpace(normalized) == "" {
		return &DiffDocument{}, nil
	}

	lines := strings.Split(normalized, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	doc := &DiffDocument{}

	var currentFile *DiffFile
	var currentHunk *DiffHunk
	var oldLineCursor int
	var newLineCursor int

	flushHunk := func() {
		if currentFile == nil || currentHunk == nil {
			return
		}
		currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
		currentHunk = nil
	}

	flushFile := func() {
		if currentFile == nil {
			return
		}
		flushHunk()
		currentFile.DisplayPath = chooseDisplayPath(currentFile)
		doc.Files = append(doc.Files, currentFile)
		currentFile = nil
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			flushFile()
			oldPath, newPath := parseDiffGitPaths(line)
			currentFile = &DiffFile{
				OldPath: oldPath,
				NewPath: newPath,
				Headers: []string{line},
			}
			continue
		}

		if currentFile == nil {
			continue
		}

		if strings.HasPrefix(line, "@@ ") {
			flushHunk()
			hunk, err := parseHunkHeader(line)
			if err != nil {
				return nil, err
			}
			oldLineCursor = hunk.OldStart
			newLineCursor = hunk.NewStart
			currentHunk = &hunk
			continue
		}

		if currentHunk != nil {
			diffLine := parseHunkLine(line, &oldLineCursor, &newLineCursor)
			switch diffLine.Kind {
			case DiffLineAdd:
				currentFile.Additions++
			case DiffLineRemove:
				currentFile.Deletions++
			}
			currentHunk.Lines = append(currentHunk.Lines, diffLine)
			continue
		}

		currentFile.Headers = append(currentFile.Headers, line)
		applyFileHeaderMetadata(currentFile, line)
	}

	flushFile()
	return doc, nil
}

func parseDiffGitPaths(line string) (oldPath string, newPath string) {
	parts := strings.Fields(line)
	if len(parts) >= 4 {
		return parseDiffPath(parts[2]), parseDiffPath(parts[3])
	}
	return "", ""
}

func parseDiffPath(path string) string {
	path = strings.Trim(path, `"`)
	if path == "/dev/null" {
		return ""
	}
	if strings.HasPrefix(path, "a/") || strings.HasPrefix(path, "b/") {
		return path[2:]
	}
	return path
}

func parseHunkHeader(header string) (DiffHunk, error) {
	matches := hunkHeaderPattern.FindStringSubmatch(header)
	if matches == nil {
		return DiffHunk{}, fmt.Errorf("invalid hunk header: %q", header)
	}

	oldStart, _ := strconv.Atoi(matches[1])
	oldCount := 1
	if matches[2] != "" {
		oldCount, _ = strconv.Atoi(matches[2])
	}

	newStart, _ := strconv.Atoi(matches[3])
	newCount := 1
	if matches[4] != "" {
		newCount, _ = strconv.Atoi(matches[4])
	}

	return DiffHunk{
		Header:   header,
		OldStart: oldStart,
		OldCount: oldCount,
		NewStart: newStart,
		NewCount: newCount,
	}, nil
}

func parseHunkLine(line string, oldCursor *int, newCursor *int) DiffLine {
	if line == "" {
		return DiffLine{Kind: DiffLineMeta, Content: ""}
	}

	prefix := line[0]
	content := ""
	if len(line) > 1 {
		content = line[1:]
	}

	switch prefix {
	case ' ':
		old := *oldCursor
		newLine := *newCursor
		*oldCursor = *oldCursor + 1
		*newCursor = *newCursor + 1
		return DiffLine{Kind: DiffLineContext, Content: content, OldLine: old, NewLine: newLine}
	case '+':
		newLine := *newCursor
		*newCursor = *newCursor + 1
		return DiffLine{Kind: DiffLineAdd, Content: content, NewLine: newLine}
	case '-':
		old := *oldCursor
		*oldCursor = *oldCursor + 1
		return DiffLine{Kind: DiffLineRemove, Content: content, OldLine: old}
	case '\\':
		return DiffLine{Kind: DiffLineMeta, Content: line}
	default:
		return DiffLine{Kind: DiffLineMeta, Content: line}
	}
}

func applyFileHeaderMetadata(file *DiffFile, line string) {
	switch {
	case strings.HasPrefix(line, "--- "):
		file.OldPath = parseDiffPath(strings.TrimSpace(strings.TrimPrefix(line, "--- ")))
	case strings.HasPrefix(line, "+++ "):
		file.NewPath = parseDiffPath(strings.TrimSpace(strings.TrimPrefix(line, "+++ ")))
	case strings.HasPrefix(line, "rename from "):
		file.OldPath = strings.TrimSpace(strings.TrimPrefix(line, "rename from "))
	case strings.HasPrefix(line, "rename to "):
		file.NewPath = strings.TrimSpace(strings.TrimPrefix(line, "rename to "))
	case strings.HasPrefix(line, "Binary files "), strings.HasPrefix(line, "GIT binary patch"):
		file.IsBinary = true
	}
}

func chooseDisplayPath(file *DiffFile) string {
	if file.NewPath != "" {
		return file.NewPath
	}
	if file.OldPath != "" {
		return file.OldPath
	}
	return "(unknown file)"
}
