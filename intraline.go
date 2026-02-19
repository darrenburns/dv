package main

import (
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

const intralineMaxDPMatrixCells = 250000

// intralineChangeMasks returns per-grapheme change masks for old/new text.
// A value of true indicates that grapheme should receive intraline emphasis.
// The bool return is false when matching is skipped (for example due to size cutoff).
func intralineChangeMasks(oldText string, newText string) (oldMask []bool, newMask []bool, ok bool) {
	oldGraphemes := splitGraphemes(oldText)
	newGraphemes := splitGraphemes(newText)
	oldChunks := splitIntralineChunks(oldGraphemes)
	newChunks := splitIntralineChunks(newGraphemes)

	oldMask = make([]bool, len(oldGraphemes))
	newMask = make([]bool, len(newGraphemes))

	if len(oldChunks) == 0 && len(newChunks) == 0 {
		return oldMask, newMask, true
	}

	prefix := 0
	for prefix < len(oldChunks) && prefix < len(newChunks) && oldChunks[prefix].text == newChunks[prefix].text {
		prefix++
	}

	suffix := 0
	for suffix < len(oldChunks)-prefix &&
		suffix < len(newChunks)-prefix &&
		oldChunks[len(oldChunks)-1-suffix].text == newChunks[len(newChunks)-1-suffix].text {
		suffix++
	}

	coreOld := oldChunks[prefix : len(oldChunks)-suffix]
	coreNew := newChunks[prefix : len(newChunks)-suffix]

	if len(coreOld) == 0 && len(coreNew) == 0 {
		return oldMask, newMask, true
	}
	if len(coreOld) == 0 {
		for _, chunk := range coreNew {
			markRange(newMask, chunk.start, chunk.end)
		}
		return oldMask, newMask, true
	}
	if len(coreNew) == 0 {
		for _, chunk := range coreOld {
			markRange(oldMask, chunk.start, chunk.end)
		}
		return oldMask, newMask, true
	}

	if len(coreOld)*len(coreNew) > intralineMaxDPMatrixCells {
		return nil, nil, false
	}

	dp := make([][]int, len(coreOld)+1)
	for row := range dp {
		dp[row] = make([]int, len(coreNew)+1)
	}

	for oldIdx := len(coreOld) - 1; oldIdx >= 0; oldIdx-- {
		for newIdx := len(coreNew) - 1; newIdx >= 0; newIdx-- {
			if coreOld[oldIdx].text == coreNew[newIdx].text {
				dp[oldIdx][newIdx] = dp[oldIdx+1][newIdx+1] + 1
				continue
			}
			dp[oldIdx][newIdx] = max(dp[oldIdx+1][newIdx], dp[oldIdx][newIdx+1])
		}
	}

	matchedOld := make([]bool, len(coreOld))
	matchedNew := make([]bool, len(coreNew))
	oldIdx, newIdx := 0, 0
	for oldIdx < len(coreOld) && newIdx < len(coreNew) {
		switch {
		case coreOld[oldIdx].text == coreNew[newIdx].text && dp[oldIdx][newIdx] == dp[oldIdx+1][newIdx+1]+1:
			matchedOld[oldIdx] = true
			matchedNew[newIdx] = true
			oldIdx++
			newIdx++
		case dp[oldIdx+1][newIdx] >= dp[oldIdx][newIdx+1]:
			oldIdx++
		default:
			newIdx++
		}
	}

	for idx := range coreOld {
		if !matchedOld[idx] {
			markRange(oldMask, coreOld[idx].start, coreOld[idx].end)
		}
	}
	for idx := range coreNew {
		if !matchedNew[idx] {
			markRange(newMask, coreNew[idx].start, coreNew[idx].end)
		}
	}

	return oldMask, newMask, true
}

type intralineChunkKind int

const (
	intralineChunkWord intralineChunkKind = iota
	intralineChunkSpace
	intralineChunkSymbol
)

type intralineChunk struct {
	text       string
	kind       intralineChunkKind
	start      int
	end        int
	standalone bool
}

func splitIntralineChunks(graphemes []string) []intralineChunk {
	if len(graphemes) == 0 {
		return nil
	}

	chunks := make([]intralineChunk, 0, len(graphemes))
	for idx, grapheme := range graphemes {
		kind := classifyIntralineChunkKind(grapheme)
		standalone := isStandaloneIntralineChunk(grapheme)
		if len(chunks) == 0 {
			chunks = append(chunks, intralineChunk{
				text:       grapheme,
				kind:       kind,
				start:      idx,
				end:        idx + 1,
				standalone: standalone,
			})
			continue
		}

		last := &chunks[len(chunks)-1]
		if last.kind == kind && !last.standalone && !standalone {
			last.text += grapheme
			last.end = idx + 1
			continue
		}

		chunks = append(chunks, intralineChunk{
			text:       grapheme,
			kind:       kind,
			start:      idx,
			end:        idx + 1,
			standalone: standalone,
		})
	}

	return chunks
}

func classifyIntralineChunkKind(grapheme string) intralineChunkKind {
	r, _ := utf8.DecodeRuneInString(grapheme)
	if r == utf8.RuneError && grapheme == "" {
		return intralineChunkSymbol
	}
	if unicode.IsSpace(r) {
		return intralineChunkSpace
	}
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return intralineChunkWord
	}
	return intralineChunkSymbol
}

func isStandaloneIntralineChunk(grapheme string) bool {
	switch grapheme {
	case "(", ")", "[", "]", "{", "}":
		return true
	default:
		return false
	}
}

func markRange(mask []bool, start int, end int) {
	if start < 0 {
		start = 0
	}
	if end > len(mask) {
		end = len(mask)
	}
	for idx := start; idx < end; idx++ {
		mask[idx] = true
	}
}

func splitGraphemes(text string) []string {
	if text == "" {
		return nil
	}
	graphemes := make([]string, 0, len(text))
	remaining := text
	for len(remaining) > 0 {
		grapheme, _ := ansi.FirstGraphemeCluster(remaining, ansi.GraphemeWidth)
		if grapheme == "" {
			break
		}
		graphemes = append(graphemes, grapheme)
		remaining = remaining[len(grapheme):]
	}
	return graphemes
}
