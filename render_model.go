package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/charmbracelet/x/ansi"
)

// RenderedLineKind is the visual category of a rendered line.
type RenderedLineKind int

const (
	RenderedLineFileHeader RenderedLineKind = iota
	RenderedLineHunkHeader
	RenderedLineContext
	RenderedLineAdd
	RenderedLineRemove
	RenderedLineMeta
)

// TokenRole is a semantic token role used to map to theme styles.
type TokenRole int

const (
	TokenRoleOldLineNumber TokenRole = iota
	TokenRoleNewLineNumber
	TokenRoleLineNumberAdd
	TokenRoleLineNumberRemove
	TokenRoleDiffPrefixAdd
	TokenRoleDiffPrefixRemove
	TokenRoleDiffPrefixContext
	TokenRoleDiffFileHeader
	TokenRoleDiffHunkHeader
	TokenRoleDiffMeta
	TokenRoleDiffHatch
	TokenRoleSyntaxPlain
	TokenRoleSyntaxKeyword
	TokenRoleSyntaxType
	TokenRoleSyntaxFunction
	TokenRoleSyntaxString
	TokenRoleSyntaxNumber
	TokenRoleSyntaxComment
	TokenRoleSyntaxPunctuation
)

const diffTabWidth = 4

// RenderedSegment is a tokenized text fragment with semantic styling.
type RenderedSegment struct {
	Text string
	Role TokenRole
}

// RenderedDiffLine is a single display line in the custom diff viewer.
type RenderedDiffLine struct {
	Kind         RenderedLineKind
	OldLine      int
	NewLine      int
	Prefix       string
	Segments     []RenderedSegment
	ContentWidth int
}

// RenderedFile is the display model for one file diff.
type RenderedFile struct {
	Title           string
	Lines           []RenderedDiffLine
	OldNumWidth     int
	NewNumWidth     int
	MaxContentWidth int
}

// RenderedSideCell is one side of a side-by-side row.
type RenderedSideCell struct {
	Kind         RenderedLineKind
	LineNumber   int
	Prefix       string
	Segments     []RenderedSegment
	ContentWidth int
}

// SideBySideRenderedRow is a row in side-by-side mode.
// Shared rows span both panes (for hunk headers and meta lines).
type SideBySideRenderedRow struct {
	Shared *RenderedDiffLine
	Left   *RenderedSideCell
	Right  *RenderedSideCell
}

// SideBySideRenderedFile is the display model for one file in side-by-side mode.
type SideBySideRenderedFile struct {
	Title                string
	Rows                 []SideBySideRenderedRow
	LeftNumWidth         int
	RightNumWidth        int
	LeftMaxContentWidth  int
	RightMaxContentWidth int
}

func buildRenderedFile(file *DiffFile) *RenderedFile {
	if file == nil {
		return nil
	}

	lexer := chooseLexer(file)
	lines := buildRenderLines(file, lexer)
	if len(lines) == 0 {
		lines = []RenderedDiffLine{
			newRenderedLine(RenderedLineMeta, 0, 0, " ", []RenderedSegment{{Text: "No changes to render", Role: TokenRoleDiffMeta}}),
		}
	}

	oldWidth, newWidth := lineNumberWidths(lines)
	maxContent := 0
	for i := range lines {
		if lines[i].ContentWidth > maxContent {
			maxContent = lines[i].ContentWidth
		}
	}

	return &RenderedFile{
		Title:           file.DisplayPath,
		Lines:           lines,
		OldNumWidth:     oldWidth,
		NewNumWidth:     newWidth,
		MaxContentWidth: maxContent,
	}
}

func buildMetaRenderedFile(title string, body []string) *RenderedFile {
	lines := make([]RenderedDiffLine, 0, len(body))
	for _, line := range body {
		lines = append(lines, newRenderedLine(RenderedLineMeta, 0, 0, " ", []RenderedSegment{{Text: line, Role: TokenRoleDiffMeta}}))
	}
	if len(lines) == 0 {
		lines = append(lines, newRenderedLine(RenderedLineMeta, 0, 0, " ", []RenderedSegment{{Text: "", Role: TokenRoleDiffMeta}}))
	}

	maxContent := 0
	for _, line := range lines {
		if line.ContentWidth > maxContent {
			maxContent = line.ContentWidth
		}
	}

	return &RenderedFile{
		Title:           title,
		Lines:           lines,
		OldNumWidth:     1,
		NewNumWidth:     1,
		MaxContentWidth: maxContent,
	}
}

func buildSideBySideRenderedFile(file *DiffFile) *SideBySideRenderedFile {
	if file == nil {
		return nil
	}

	lexer := chooseLexer(file)
	rows := buildSideBySideRows(file, lexer)
	if len(rows) == 0 {
		message := "No displayable content"
		if file.IsBinary {
			message = "Binary file changed"
		}
		line := newRenderedLine(
			RenderedLineMeta,
			0,
			0,
			" ",
			[]RenderedSegment{{Text: message, Role: TokenRoleDiffMeta}},
		)
		rows = append(rows, SideBySideRenderedRow{Shared: &line})
	}

	leftNumWidth, rightNumWidth, leftMax, rightMax := sideBySideMetrics(rows)
	return &SideBySideRenderedFile{
		Title:                file.DisplayPath,
		Rows:                 rows,
		LeftNumWidth:         leftNumWidth,
		RightNumWidth:        rightNumWidth,
		LeftMaxContentWidth:  leftMax,
		RightMaxContentWidth: rightMax,
	}
}

func buildSideBySideFromRendered(rendered *RenderedFile) *SideBySideRenderedFile {
	if rendered == nil {
		return nil
	}

	rows := make([]SideBySideRenderedRow, 0, len(rendered.Lines))
	leftMax := 1
	rightMax := 1
	for _, line := range rendered.Lines {
		copyLine := line
		rows = append(rows, SideBySideRenderedRow{Shared: &copyLine})
		if line.ContentWidth > leftMax {
			leftMax = line.ContentWidth
		}
		if line.ContentWidth > rightMax {
			rightMax = line.ContentWidth
		}
	}

	leftNumWidth := rendered.OldNumWidth
	if leftNumWidth <= 0 {
		leftNumWidth = 1
	}
	rightNumWidth := rendered.NewNumWidth
	if rightNumWidth <= 0 {
		rightNumWidth = 1
	}

	return &SideBySideRenderedFile{
		Title:                rendered.Title,
		Rows:                 rows,
		LeftNumWidth:         leftNumWidth,
		RightNumWidth:        rightNumWidth,
		LeftMaxContentWidth:  leftMax,
		RightMaxContentWidth: rightMax,
	}
}

func buildRenderLines(file *DiffFile, lexer chroma.Lexer) []RenderedDiffLine {
	lines := make([]RenderedDiffLine, 0, len(file.Headers)+len(file.Hunks)*8)
	for _, hunk := range file.Hunks {
		lines = append(lines, newRenderedLine(
			RenderedLineHunkHeader,
			0,
			0,
			" ",
			[]RenderedSegment{{Text: hunk.Header, Role: TokenRoleDiffHunkHeader}},
		))

		for _, line := range hunk.Lines {
			rendered := RenderedDiffLine{
				Prefix:  " ",
				OldLine: line.OldLine,
				NewLine: line.NewLine,
			}
			switch line.Kind {
			case DiffLineContext:
				rendered.Kind = RenderedLineContext
				rendered.Prefix = " "
				rendered = newRenderedLine(rendered.Kind, rendered.OldLine, rendered.NewLine, rendered.Prefix, lineSegmentsForCode(line.Content, lexer))
			case DiffLineAdd:
				rendered.Kind = RenderedLineAdd
				rendered.Prefix = "+"
				rendered = newRenderedLine(rendered.Kind, rendered.OldLine, rendered.NewLine, rendered.Prefix, lineSegmentsForCode(line.Content, lexer))
			case DiffLineRemove:
				rendered.Kind = RenderedLineRemove
				rendered.Prefix = "-"
				rendered = newRenderedLine(rendered.Kind, rendered.OldLine, rendered.NewLine, rendered.Prefix, lineSegmentsForCode(line.Content, lexer))
			default:
				rendered.Kind = RenderedLineMeta
				rendered.Prefix = " "
				rendered = newRenderedLine(rendered.Kind, 0, 0, rendered.Prefix, []RenderedSegment{{Text: line.Content, Role: TokenRoleDiffMeta}})
			}
			lines = append(lines, rendered)
		}
	}

	if len(lines) == 0 {
		message := "No displayable content"
		if file.IsBinary {
			message = "Binary file changed"
		}
		lines = append(lines, newRenderedLine(
			RenderedLineMeta,
			0,
			0,
			" ",
			[]RenderedSegment{{Text: message, Role: TokenRoleDiffMeta}},
		))
	}
	return lines
}

func buildSideBySideRows(file *DiffFile, lexer chroma.Lexer) []SideBySideRenderedRow {
	rows := make([]SideBySideRenderedRow, 0, len(file.Headers)+len(file.Hunks)*8)
	for _, hunk := range file.Hunks {
		header := newRenderedLine(
			RenderedLineHunkHeader,
			0,
			0,
			" ",
			[]RenderedSegment{{Text: hunk.Header, Role: TokenRoleDiffHunkHeader}},
		)
		rows = append(rows, SideBySideRenderedRow{Shared: &header})

		for idx := 0; idx < len(hunk.Lines); {
			line := hunk.Lines[idx]
			switch line.Kind {
			case DiffLineContext:
				rendered := renderedLineFromDiffLine(line, lexer)
				rows = append(rows, SideBySideRenderedRow{
					Left:  leftCellFromRenderedLine(rendered),
					Right: rightCellFromRenderedLine(rendered),
				})
				idx++
			case DiffLineRemove:
				removeLines := make([]RenderedDiffLine, 0, 4)
				for idx < len(hunk.Lines) && hunk.Lines[idx].Kind == DiffLineRemove {
					removeLines = append(removeLines, renderedLineFromDiffLine(hunk.Lines[idx], lexer))
					idx++
				}

				addLines := make([]RenderedDiffLine, 0, 4)
				addIdx := idx
				for addIdx < len(hunk.Lines) && hunk.Lines[addIdx].Kind == DiffLineAdd {
					addLines = append(addLines, renderedLineFromDiffLine(hunk.Lines[addIdx], lexer))
					addIdx++
				}
				if len(addLines) > 0 {
					idx = addIdx
				}

				rowCount := max(len(removeLines), len(addLines))
				for pairIdx := 0; pairIdx < rowCount; pairIdx++ {
					row := SideBySideRenderedRow{}
					if pairIdx < len(removeLines) {
						row.Left = leftCellFromRenderedLine(removeLines[pairIdx])
					}
					if pairIdx < len(addLines) {
						row.Right = rightCellFromRenderedLine(addLines[pairIdx])
					}
					rows = append(rows, row)
				}
			case DiffLineAdd:
				addLines := make([]RenderedDiffLine, 0, 4)
				for idx < len(hunk.Lines) && hunk.Lines[idx].Kind == DiffLineAdd {
					addLines = append(addLines, renderedLineFromDiffLine(hunk.Lines[idx], lexer))
					idx++
				}
				for _, addLine := range addLines {
					rows = append(rows, SideBySideRenderedRow{Right: rightCellFromRenderedLine(addLine)})
				}
			default:
				rendered := renderedLineFromDiffLine(line, lexer)
				copyLine := rendered
				rows = append(rows, SideBySideRenderedRow{Shared: &copyLine})
				idx++
			}
		}
	}
	return rows
}

func renderedLineFromDiffLine(line DiffLine, lexer chroma.Lexer) RenderedDiffLine {
	switch line.Kind {
	case DiffLineContext:
		return newRenderedLine(
			RenderedLineContext,
			line.OldLine,
			line.NewLine,
			" ",
			lineSegmentsForCode(line.Content, lexer),
		)
	case DiffLineAdd:
		return newRenderedLine(
			RenderedLineAdd,
			0,
			line.NewLine,
			"+",
			lineSegmentsForCode(line.Content, lexer),
		)
	case DiffLineRemove:
		return newRenderedLine(
			RenderedLineRemove,
			line.OldLine,
			0,
			"-",
			lineSegmentsForCode(line.Content, lexer),
		)
	default:
		return newRenderedLine(
			RenderedLineMeta,
			0,
			0,
			" ",
			[]RenderedSegment{{Text: line.Content, Role: TokenRoleDiffMeta}},
		)
	}
}

func leftCellFromRenderedLine(line RenderedDiffLine) *RenderedSideCell {
	return &RenderedSideCell{
		Kind:         line.Kind,
		LineNumber:   line.OldLine,
		Prefix:       line.Prefix,
		Segments:     line.Segments,
		ContentWidth: line.ContentWidth,
	}
}

func rightCellFromRenderedLine(line RenderedDiffLine) *RenderedSideCell {
	return &RenderedSideCell{
		Kind:         line.Kind,
		LineNumber:   line.NewLine,
		Prefix:       line.Prefix,
		Segments:     line.Segments,
		ContentWidth: line.ContentWidth,
	}
}

func sideBySideMetrics(rows []SideBySideRenderedRow) (leftNumWidth int, rightNumWidth int, leftMax int, rightMax int) {
	maxLeftLine := 1
	maxRightLine := 1
	leftMax = 1
	rightMax = 1
	for _, row := range rows {
		if row.Left != nil {
			if row.Left.LineNumber > maxLeftLine {
				maxLeftLine = row.Left.LineNumber
			}
			if row.Left.ContentWidth > leftMax {
				leftMax = row.Left.ContentWidth
			}
		}
		if row.Right != nil {
			if row.Right.LineNumber > maxRightLine {
				maxRightLine = row.Right.LineNumber
			}
			if row.Right.ContentWidth > rightMax {
				rightMax = row.Right.ContentWidth
			}
		}
		if row.Shared != nil {
			if row.Shared.ContentWidth > leftMax {
				leftMax = row.Shared.ContentWidth
			}
			if row.Shared.ContentWidth > rightMax {
				rightMax = row.Shared.ContentWidth
			}
		}
	}

	leftNumWidth = len(strconv.Itoa(maxLeftLine))
	rightNumWidth = len(strconv.Itoa(maxRightLine))
	return leftNumWidth, rightNumWidth, leftMax, rightMax
}

func renderedMaxContentWidth(rendered *RenderedFile, side *SideBySideRenderedFile) int {
	maxContent := 1
	if rendered != nil && rendered.MaxContentWidth > maxContent {
		maxContent = rendered.MaxContentWidth
	}
	if side != nil {
		if side.LeftMaxContentWidth > maxContent {
			maxContent = side.LeftMaxContentWidth
		}
		if side.RightMaxContentWidth > maxContent {
			maxContent = side.RightMaxContentWidth
		}
	}
	return maxContent
}

func lineSegmentsForCode(content string, lexer chroma.Lexer) []RenderedSegment {
	if content == "" {
		return []RenderedSegment{}
	}
	if lexer == nil {
		return []RenderedSegment{{Text: content, Role: TokenRoleSyntaxPlain}}
	}
	segments := lexLineSegments(lexer, content)
	if len(segments) == 0 {
		return []RenderedSegment{{Text: content, Role: TokenRoleSyntaxPlain}}
	}
	return segments
}

func newRenderedLine(kind RenderedLineKind, oldLine int, newLine int, prefix string, segments []RenderedSegment) RenderedDiffLine {
	expanded, width := expandTabsInSegments(segments, diffTabWidth)
	return RenderedDiffLine{
		Kind:         kind,
		OldLine:      oldLine,
		NewLine:      newLine,
		Prefix:       prefix,
		Segments:     expanded,
		ContentWidth: width,
	}
}

func lineNumberWidths(lines []RenderedDiffLine) (oldWidth int, newWidth int) {
	maxOld := 1
	maxNew := 1
	for _, line := range lines {
		if line.OldLine > maxOld {
			maxOld = line.OldLine
		}
		if line.NewLine > maxNew {
			maxNew = line.NewLine
		}
	}
	return len(strconv.Itoa(maxOld)), len(strconv.Itoa(maxNew))
}

func lineNumberText(number int, width int) string {
	if number <= 0 {
		return strings.Repeat(" ", width)
	}
	return fmt.Sprintf("%*d", width, number)
}

func prefixRoleForLine(kind RenderedLineKind) (TokenRole, bool) {
	switch kind {
	case RenderedLineAdd:
		return TokenRoleDiffPrefixAdd, true
	case RenderedLineRemove:
		return TokenRoleDiffPrefixRemove, true
	case RenderedLineContext:
		return TokenRoleDiffPrefixContext, true
	default:
		return 0, false
	}
}

func chooseLexer(file *DiffFile) chroma.Lexer {
	path := file.NewPath
	if path == "" {
		path = file.OldPath
	}
	if path == "" {
		return nil
	}
	lexer := lexers.Match(path)
	if lexer == nil {
		return nil
	}
	return chroma.Coalesce(lexer)
}

func lexLineSegments(lexer chroma.Lexer, content string) []RenderedSegment {
	iterator, err := lexer.Tokenise(nil, content)
	if err != nil {
		return nil
	}

	result := make([]RenderedSegment, 0, 8)
	for token := iterator(); token != chroma.EOF; token = iterator() {
		if token.Value == "" {
			continue
		}
		role := tokenRoleFromChroma(token.Type)
		result = append(result, RenderedSegment{
			Text: token.Value,
			Role: role,
		})
	}

	return result
}

func expandTabsInSegments(segments []RenderedSegment, tabWidth int) ([]RenderedSegment, int) {
	if tabWidth <= 0 {
		tabWidth = diffTabWidth
	}

	expanded := make([]RenderedSegment, 0, len(segments))
	column := 0
	for _, segment := range segments {
		remaining := segment.Text
		for len(remaining) > 0 {
			grapheme, width := ansi.FirstGraphemeCluster(remaining, ansi.GraphemeWidth)
			if grapheme == "" {
				break
			}
			if grapheme == "\t" {
				spaces := tabWidth - (column % tabWidth)
				if spaces <= 0 {
					spaces = tabWidth
				}
				appendRoleText(&expanded, segment.Role, strings.Repeat(" ", spaces))
				column += spaces
			} else {
				appendRoleText(&expanded, segment.Role, grapheme)
				if width <= 0 {
					width = ansi.StringWidth(grapheme)
				}
				if width <= 0 {
					width = 1
				}
				column += width
			}
			remaining = remaining[len(grapheme):]
		}
	}

	return expanded, column
}

func appendRoleText(segments *[]RenderedSegment, role TokenRole, text string) {
	if text == "" {
		return
	}
	existing := *segments
	if len(existing) > 0 && existing[len(existing)-1].Role == role {
		existing[len(existing)-1].Text += text
		*segments = existing
		return
	}
	*segments = append(existing, RenderedSegment{Text: text, Role: role})
}

func tokenRoleFromChroma(token chroma.TokenType) TokenRole {
	switch {
	case token.InCategory(chroma.Comment):
		return TokenRoleSyntaxComment
	case token.InCategory(chroma.Keyword):
		return TokenRoleSyntaxKeyword
	case token.InCategory(chroma.LiteralString):
		return TokenRoleSyntaxString
	case token.InCategory(chroma.LiteralNumber):
		return TokenRoleSyntaxNumber
	case token.InSubCategory(chroma.NameFunction):
		return TokenRoleSyntaxFunction
	case token == chroma.NameClass || token.InSubCategory(chroma.NameBuiltin) || token == chroma.KeywordType:
		return TokenRoleSyntaxType
	case token == chroma.Punctuation || token.InCategory(chroma.Operator) || token == chroma.TextPunctuation:
		return TokenRoleSyntaxPunctuation
	default:
		return TokenRoleSyntaxPlain
	}
}
