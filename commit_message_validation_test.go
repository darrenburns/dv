package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommitMessageHighlightsValidShapeHasNoHighlights(t *testing.T) {
	highlights := commitMessageHighlights(splitGraphemes("feat: add commit validation\n\nWrap body at a reasonable width."))

	require.Empty(t, highlights)
}

func TestCommitMessageHighlightsWarnsOnLongSubject(t *testing.T) {
	subject := strings.Repeat("a", commitMessageSubjectWidthLimit+1)

	highlights := commitMessageHighlights(splitGraphemes(subject))

	require.Len(t, highlights, 1)
	require.Equal(t, commitMessageSeverityWarning, highlights[0].Severity)
	require.Equal(t, commitMessageSubjectWidthLimit, highlights[0].Start)
	require.Equal(t, len(splitGraphemes(subject)), highlights[0].End)
}

func TestCommitMessageHighlightsErrorsOnMissingBlankSeparator(t *testing.T) {
	message := "feat: add commit validation\nBody starts too early."

	highlights := commitMessageHighlights(splitGraphemes(message))

	require.Len(t, highlights, 1)
	require.Equal(t, commitMessageSeverityError, highlights[0].Severity)
	require.Equal(t, len(splitGraphemes("feat: add commit validation\n")), highlights[0].Start)
	require.Equal(t, len(splitGraphemes(message)), highlights[0].End)
}

func TestCommitMessageHighlightsWarnsOnLongBodyLine(t *testing.T) {
	message := "feat: add commit validation\n\n" + strings.Repeat("b", commitMessageBodyWidthLimit+1)

	highlights := commitMessageHighlights(splitGraphemes(message))

	require.Len(t, highlights, 1)
	require.Equal(t, commitMessageSeverityWarning, highlights[0].Severity)
	require.Equal(t, len(splitGraphemes("feat: add commit validation\n\n"))+commitMessageBodyWidthLimit, highlights[0].Start)
	require.Equal(t, len(splitGraphemes(message)), highlights[0].End)
}
