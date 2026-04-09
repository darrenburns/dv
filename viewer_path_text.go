package main

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
	t "github.com/darrenburns/terma"
)

const pathEllipsis = "…"

// viewerPathText renders a file path with middle-ellipsis compaction based on
// the final allocated render width.
type viewerPathText struct {
	t.Text
	FullPath      string
	EllipsisColor t.Color
}

func (v viewerPathText) Build(ctx t.BuildContext) t.Widget {
	return v
}

func (v viewerPathText) Render(ctx *t.RenderContext) {
	path := v.FullPath
	if path == "" {
		path = v.Content
	}

	text := v.Text
	text.Content = ""
	text.Spans = compactPathMiddleSpans(path, ctx.Width, v.EllipsisColor, text.Style.Bold)
	text.Render(ctx)
}

func compactPathMiddle(path string, maxWidth int) string {
	head, tail, truncated := compactPathMiddleParts(path, maxWidth)
	if !truncated {
		return head
	}
	return head + pathEllipsis + tail
}

func compactPathMiddleSpans(path string, maxWidth int, ellipsisColor t.Color, bold bool) []t.Span {
	head, tail, truncated := compactPathMiddleParts(path, maxWidth)
	if head == "" && tail == "" && !truncated {
		return nil
	}

	baseStyle := t.SpanStyle{Bold: bold}
	spans := make([]t.Span, 0, 3)
	if head != "" {
		spans = append(spans, t.StyledSpan(head, baseStyle))
	}
	if truncated {
		if ellipsisColor.IsSet() {
			ellipsisStyle := baseStyle
			ellipsisStyle.Foreground = ellipsisColor
			spans = append(spans, t.StyledSpan(pathEllipsis, ellipsisStyle))
		} else {
			spans = append(spans, t.StyledSpan(pathEllipsis, baseStyle))
		}
	}
	if tail != "" {
		spans = append(spans, t.StyledSpan(tail, baseStyle))
	}
	return spans
}

func compactPathMiddleParts(path string, maxWidth int) (head string, tail string, truncated bool) {
	if maxWidth <= 0 {
		return "", "", false
	}
	if ansi.StringWidth(path) <= maxWidth {
		return path, "", false
	}

	if maxWidth == 1 {
		return "", "", true
	}
	if maxWidth == 2 {
		head = ansi.Truncate(path, 1, "")
		if head != "" {
			return head, "", true
		}
		tail = ansi.TruncateLeft(path, 1, "")
		return "", tail, true
	}

	ellipsisWidth := ansi.StringWidth(pathEllipsis)
	tailBudget := maxWidth - ellipsisWidth
	if tailBudget <= 0 {
		return "", "", true
	}

	filename := path
	if sep := strings.LastIndexAny(path, `/\`); sep >= 0 {
		if sep < len(path)-1 {
			filename = path[sep:]
		} else {
			filename = ""
		}
	}

	tailSource := filename
	if tailSource == "" {
		tailSource = path
	}
	tail = pathTailByWidth(tailSource, tailBudget)
	tailTruncated := ansi.StringWidth(tailSource) > tailBudget

	headBudget := maxWidth - ellipsisWidth - ansi.StringWidth(tail)
	if headBudget < 0 {
		headBudget = 0
	}

	if headBudget == 0 && tailTruncated && maxWidth > 2 {
		headBudget = 1
		tail = pathTailByWidth(tailSource, maxWidth-ellipsisWidth-headBudget)
	}

	head = ansi.Truncate(path, headBudget, "")

	result := head + pathEllipsis + tail
	if ansi.StringWidth(result) > maxWidth {
		return ansi.Truncate(result, maxWidth, ""), "", false
	}
	return head, tail, true
}

func pathTailByWidth(value string, width int) string {
	if value == "" || width <= 0 {
		return ""
	}
	fullWidth := ansi.StringWidth(value)
	if fullWidth <= width {
		return value
	}
	return ansi.TruncateLeft(value, fullWidth-width, "")
}
