package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitIntralineChunks_TreatsBracketsAsStandaloneChunks(t *testing.T) {
	graphemes := splitGraphemes("foo(bar)[baz]{qux}==")
	chunks := splitIntralineChunks(graphemes)

	require.Equal(t, []string{
		"foo",
		"(",
		"bar",
		")",
		"[",
		"baz",
		"]",
		"{",
		"qux",
		"}",
		"==",
	}, intralineChunkTexts(chunks))
}

func TestSplitIntralineChunks_DoesNotMergeConsecutiveBrackets(t *testing.T) {
	graphemes := splitGraphemes("([]){}")
	chunks := splitIntralineChunks(graphemes)

	require.Equal(t, []string{"(", "[", "]", ")", "{", "}"}, intralineChunkTexts(chunks))
}

func intralineChunkTexts(chunks []intralineChunk) []string {
	texts := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		texts = append(texts, chunk.text)
	}
	return texts
}
