package main

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
	t "github.com/darrenburns/terma"
)

const (
	commitMessageSubjectWidthLimit = 50
	commitMessageBodyWidthLimit    = 72
)

type commitMessageSeverity int

const (
	commitMessageSeverityWarning commitMessageSeverity = iota
	commitMessageSeverityError
)

type commitMessageHighlight struct {
	Start    int
	End      int
	Severity commitMessageSeverity
}

type commitMessageLine struct {
	Start int
	End   int
	Text  string
}

func commitMessageHighlighter(theme t.ThemeData) t.HighlighterFunc {
	return func(_ string, graphemes []string) []t.TextHighlight {
		specs := commitMessageHighlights(graphemes)
		if len(specs) == 0 {
			return nil
		}

		highlights := make([]t.TextHighlight, 0, len(specs))
		for _, spec := range specs {
			if spec.Start >= spec.End {
				continue
			}
			highlights = append(highlights, t.TextHighlight{
				Start: spec.Start,
				End:   spec.End,
				Style: commitMessageHighlightStyle(theme, spec.Severity),
			})
		}
		return highlights
	}
}

func commitMessageHighlightStyle(theme t.ThemeData, severity commitMessageSeverity) t.SpanStyle {
	switch severity {
	case commitMessageSeverityError:
		return t.SpanStyle{
			Foreground: theme.ErrorText,
			Background: theme.ErrorBg.WithAlpha(0.5),
		}
	default:
		return t.SpanStyle{
			Foreground: theme.WarningText,
			Background: theme.WarningBg.WithAlpha(0.4),
		}
	}
}

func commitMessageHighlights(graphemes []string) []commitMessageHighlight {
	if len(graphemes) == 0 {
		return nil
	}

	lines := splitCommitMessageLines(graphemes)
	if len(lines) == 0 {
		return nil
	}

	highlights := make([]commitMessageHighlight, 0, 3)

	if overflow, ok := commitMessageOverflowHighlight(
		lines[0].Start,
		lines[0].End,
		graphemes,
		commitMessageSubjectWidthLimit,
	); ok {
		highlights = append(highlights, overflow)
	}

	bodyHasContent := false
	for _, line := range lines[1:] {
		if strings.TrimSpace(line.Text) != "" {
			bodyHasContent = true
			break
		}
	}

	if bodyHasContent && len(lines) > 1 && strings.TrimSpace(lines[1].Text) != "" {
		highlights = append(highlights, commitMessageHighlight{
			Start:    lines[1].Start,
			End:      lines[1].End,
			Severity: commitMessageSeverityError,
		})
	}

	for idx := 2; idx < len(lines); idx++ {
		if overflow, ok := commitMessageOverflowHighlight(
			lines[idx].Start,
			lines[idx].End,
			graphemes,
			commitMessageBodyWidthLimit,
		); ok {
			highlights = append(highlights, overflow)
		}
	}

	return highlights
}

func splitCommitMessageLines(graphemes []string) []commitMessageLine {
	lines := make([]commitMessageLine, 0, 4)
	lineStart := 0
	var builder strings.Builder

	for idx, grapheme := range graphemes {
		if grapheme == "\n" {
			lines = append(lines, commitMessageLine{
				Start: lineStart,
				End:   idx,
				Text:  builder.String(),
			})
			builder.Reset()
			lineStart = idx + 1
			continue
		}
		builder.WriteString(grapheme)
	}

	lines = append(lines, commitMessageLine{
		Start: lineStart,
		End:   len(graphemes),
		Text:  builder.String(),
	})

	return lines
}

func commitMessageOverflowHighlight(start int, end int, graphemes []string, limit int) (commitMessageHighlight, bool) {
	if start >= end || limit < 0 {
		return commitMessageHighlight{}, false
	}

	width := 0
	for idx := start; idx < end; idx++ {
		width += ansi.StringWidth(graphemes[idx])
		if width > limit {
			return commitMessageHighlight{
				Start:    idx,
				End:      end,
				Severity: commitMessageSeverityWarning,
			}, true
		}
	}

	return commitMessageHighlight{}, false
}
