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

func TestIntralineChangeMasks_BridgesInnerSpacesBetweenChangedWords(t *testing.T) {
	oldMask, newMask, ok := intralineChangeMasks("alpha beta gamma zeta", "alpha delta epsilon zeta")
	require.True(t, ok)

	require.Equal(t, indexRange(6, 16), boolMaskIndices(oldMask))
	require.Equal(t, indexRange(6, 19), boolMaskIndices(newMask))
}

func TestIntralineChangeMasks_DoesNotBridgeEdgeSpacesAroundChangedRegion(t *testing.T) {
	oldMask, _, ok := intralineChangeMasks("alpha beta gamma zeta", "alpha delta epsilon zeta")
	require.True(t, ok)
	require.False(t, oldMask[5])
	require.False(t, oldMask[16])
}

func intralineChunkTexts(chunks []intralineChunk) []string {
	texts := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		texts = append(texts, chunk.text)
	}
	return texts
}

func boolMaskIndices(mask []bool) []int {
	indices := make([]int, 0, len(mask))
	for idx, marked := range mask {
		if marked {
			indices = append(indices, idx)
		}
	}
	return indices
}
