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
	FullPath string
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
	text.Content = compactPathMiddle(path, ctx.Width)
	text.Spans = nil
	text.Render(ctx)
}

func compactPathMiddle(path string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if ansi.StringWidth(path) <= maxWidth {
		return path
	}

	if maxWidth == 1 {
		return pathEllipsis
	}
	if maxWidth == 2 {
		head := ansi.Truncate(path, 1, "")
		if head != "" {
			return head + pathEllipsis
		}
		tail := ansi.TruncateLeft(path, 1, "")
		return pathEllipsis + tail
	}

	ellipsisWidth := ansi.StringWidth(pathEllipsis)
	tailBudget := maxWidth - ellipsisWidth
	if tailBudget <= 0 {
		return pathEllipsis
	}

	filename := path
	if sep := strings.LastIndexAny(path, `/\`); sep >= 0 {
		if sep+1 < len(path) {
			filename = path[sep+1:]
		} else {
			filename = ""
		}
	}

	tailSource := filename
	if tailSource == "" {
		tailSource = path
	}
	tail := pathTailByWidth(tailSource, tailBudget)
	tailTruncated := ansi.StringWidth(tailSource) > tailBudget

	headBudget := maxWidth - ellipsisWidth - ansi.StringWidth(tail)
	if headBudget < 0 {
		headBudget = 0
	}

	if headBudget == 0 && tailTruncated && maxWidth > 2 {
		headBudget = 1
		tail = pathTailByWidth(tailSource, maxWidth-ellipsisWidth-headBudget)
	}

	head := ansi.Truncate(path, headBudget, "")

	result := head + pathEllipsis + tail
	if ansi.StringWidth(result) > maxWidth {
		return ansi.Truncate(result, maxWidth, "")
	}
	return result
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
