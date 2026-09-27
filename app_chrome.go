package main

import (
	"fmt"
	t "github.com/darrenburns/terma"
)

func (a *Aside) Build(ctx t.BuildContext) t.Widget {
	_ = a.version.Get()
	theme := ctx.Theme()
	content := a.buildReviewContent()
	right := t.Dock{Top: []t.Widget{a.viewerTitle(theme)}, Body: reviewViewport{Column: t.Column{Style: t.Style{Width: t.Flex(1), Height: t.Flex(1)}, Children: []t.Widget{
		t.Scrollable{ID: viewerScrollID, State: a.scroll, Focusable: true, Style: t.Style{Width: t.Flex(1), Height: t.Auto, MaxHeight: t.Percent(100)}, Child: content},
		viewerEmptySpaceHatch{Style: t.Style{Width: t.Flex(1), Height: t.Flex(1), BackgroundColor: theme.Background}, Foreground: t.NewGradient(theme.Background, theme.PrimaryBg).WithAngle(0)},
	}}, app: a}}
	var body t.Widget = right
	if a.sidebarVisible.Get() {
		divider := theme.TextDisabled
		if a.dividerHovered.Get() {
			divider = theme.Accent.WithAlpha(.5)
		}
		focused := ctx.Focused()
		dividerFocused := false
		if id, ok := focused.(t.Identifiable); ok {
			dividerFocused = id.WidgetID() == "aside-split"
		}
		body = FocusAwareSplitPane{SplitPane: t.SplitPane{ID: "aside-split", State: a.split, Orientation: t.SplitHorizontal, MinPaneSize: 12, DividerSize: 1,
			DividerBackground: theme.Background, DividerForeground: t.NewGradient(theme.Background, divider, theme.Background).WithAngle(0),
			DividerFocusForeground: t.NewGradient(theme.Background, theme.Accent, theme.Background).WithAngle(0),
			Hover:                  func(e t.HoverEvent) { a.dividerHovered.Set(e.Type == t.HoverEnter) },
			OnExitFocus:            func() { a.dividerActive.Set(false); t.RequestFocus(filesTreeID) },
			First:                  a.buildSidebar(ctx, theme), Second: right, Style: t.Style{Width: t.Flex(1), Height: t.Flex(1)},
		}, AllowFocus: a.dividerActive.Get(), EnableKeybinds: dividerFocused}
	}
	footer := []t.Widget{asideStatusBar{app: a}}
	return t.Stack{Style: t.Style{Width: t.Flex(1), Height: t.Flex(1), BackgroundColor: theme.Background}, Children: []t.Widget{
		t.Dock{Style: t.Style{Width: t.Flex(1), Height: t.Flex(1)}, Top: []t.Widget{a.buildHeader(theme)}, Bottom: footer, Body: body},
		t.CommandPalette{ID: paletteID, State: a.palette, Position: t.FloatPositionTopCenter, Offset: t.Offset{Y: 1}, BackdropColor: t.Black.WithAlpha(.05), OnCursorChange: a.previewTheme, OnDismiss: a.dismissPalette},
	}}
}
func (a *Aside) buildHeader(theme t.ThemeData) t.Widget {
	mode := "Unified"
	if a.mode.Get() == DiffLayoutSideBySide {
		mode = "Split"
	}
	return t.Row{Style: t.Style{Width: t.Flex(1), Padding: t.EdgeInsetsXY(1, 0), BackgroundColor: t.NewGradient(theme.Surface, theme.Background, theme.SecondaryBg).WithAngle(90)}, Spacing: 1, Children: []t.Widget{
		t.Label("ASIDE", t.LabelPrimary, theme),
		t.Text{Content: singleLine(a.review.Title), Style: t.Style{Width: t.Flex(1), ForegroundColor: theme.TextMuted}},
		t.Text{Content: commentCountLabel(len(a.review.Threads)), Style: t.Style{ForegroundColor: theme.Secondary}},
		t.Text{Content: mode, Style: t.Style{ForegroundColor: theme.Accent}},
		shortcutButton{Button: t.Button{ID: "aside-theme", Label: themeDisplayName(t.CurrentThemeName()), DisableFocus: true, OnPress: a.openThemes, Click: func(t.MouseEvent) { a.openThemes() }, Style: t.Style{ForegroundColor: theme.SecondaryText}}, Shortcut: "t", Key: "t"},
	}}
}
func changeStatSpans(add, del int, theme t.ThemeData) []t.Span {
	spans := []t.Span{}
	if add > 0 {
		spans = append(spans, t.ColorSpan(fmt.Sprintf("+%d", add), theme.Success))
	}
	if del > 0 {
		if add > 0 {
			spans = append(spans, t.PlainSpan(" "))
		}
		spans = append(spans, t.ColorSpan(fmt.Sprintf("−%d", del), theme.Error))
	}
	return spans
}
func (a *Aside) buildSidebar(ctx t.BuildContext, theme t.ThemeData) t.Widget {
	fileTree := t.Tree[DiffTreeNodeData]{ID: filesTreeID, State: a.tree, ScrollState: a.treeScroll, NodeID: func(n DiffTreeNodeData) string { return n.NodeKey }, HasChildren: func(n DiffTreeNodeData) bool { return n.IsDir },
		OnCursorChange: func(n DiffTreeNodeData) {
			if !n.IsDir {
				a.selectFile(n.Path)
			}
		}, Style: t.Style{Width: t.Flex(1)}}
	focused := ctx.IsFocused(fileTree)
	fileTree.RenderNode = func(n DiffTreeNodeData, c t.TreeNodeContext) t.Widget {
		rowStyle := t.Style{Width: t.Flex(1), Padding: t.EdgeInsets{Right: 1}}
		textStyle := t.Style{Width: t.Flex(1), ForegroundColor: theme.Text, Strikethrough: !n.IsDir && a.isSeen(n.Path)}
		if n.IsDir {
			textStyle.ForegroundColor = theme.TextMuted
		}
		if c.Active {
			rowStyle.BackgroundColor = theme.ActiveCursor.WithAlpha(.25)
			if focused {
				rowStyle.BackgroundColor = theme.ActiveCursor
				textStyle.ForegroundColor = theme.SelectionText
			}
		}
		name := singleLine(n.Name)
		if n.IsDir {
			name += "/"
		}
		label := t.Text{Content: name, Style: textStyle}
		if match := t.MatchString(name, a.filter.GetText(), t.FilterOptions{Mode: t.FilterFuzzy}); a.filter.GetText() != "" && match.Matched {
			label.Content = ""
			label.Spans = t.HighlightSpans(name, match.Ranges, t.MatchHighlightStyle(theme))
		}
		stats := changeStatSpans(n.Additions, n.Deletions, theme)
		if n.IsDir {
			for i := range stats {
				stats[i].Style.Foreground = theme.TextMuted
			}
		}
		if c.Active && focused {
			for i := range stats {
				stats[i].Style.Foreground = theme.SelectionText
			}
		}
		return t.Row{Style: rowStyle, Children: []t.Widget{label, t.Spacer{Width: t.Cells(1)}, t.Text{Spans: stats}}}
	}
	add, del := 0, 0
	for _, p := range a.paths {
		add += a.files[p].Additions
		del += a.files[p].Deletions
	}
	mode := "Changes"
	if a.allFiles.Get() {
		mode = "Files"
	}
	top := []t.Widget{t.Row{Style: t.Style{Width: t.Flex(1), Padding: t.EdgeInsetsXY(1, 0)}, Children: []t.Widget{
		t.Text{Content: fmt.Sprintf("%s · %d", mode, len(a.paths)), Style: t.Style{Width: t.Flex(1), ForegroundColor: theme.TextMuted}}, t.Spacer{Width: t.Cells(1)}, t.Text{Spans: changeStatSpans(add, del, theme)},
	}}}
	if seen, total := a.seenProgress(); seen > 0 {
		top = append(top, t.Text{Content: fmt.Sprintf(" %d/%d seen · %d%%", seen, total, 100*seen/max(1, total)), Style: t.Style{ForegroundColor: theme.Success}})
	}
	if a.filtering.Get() || a.filter.GetText() != "" {
		top = append(top, t.TextInput{ID: filterID, State: a.filter, Placeholder: "Filter files…", Style: t.Style{Width: t.Flex(1), Padding: t.EdgeInsetsXY(1, 0), BackgroundColor: theme.Surface},
			OnChange: func(string) { a.rebuildFiles(); a.bump() }, ExtraKeybinds: []t.Keybind{
				{Key: "escape", Name: "Clear filter", Action: a.clearFilter},
				{Key: "enter", Name: "Browse", Action: func() { a.filtering.Set(false); t.RequestFocus(filesTreeID) }},
				{Key: "down", Hidden: true, Action: func() { a.nextFile(1) }},
				{Key: "up", Hidden: true, Action: func() { a.nextFile(-1) }},
			}})
	}
	return t.Dock{Top: top, Body: t.Scrollable{ID: "aside-tree-scroll", State: a.treeScroll, Style: t.Style{Width: t.Flex(1), Height: t.Flex(1)}, Child: fileTree}}
}
func (a *Aside) viewerTitle(theme t.ThemeData) t.Widget {
	bg := t.ColorProvider(theme.Surface)
	if a.isSeen(a.active) {
		bg = t.NewGradient(theme.SuccessBg, theme.Background).WithAngle(90)
	}
	meta := []t.Span{}
	if file := a.files[a.active]; file != nil {
		meta = changeStatSpans(file.Additions, file.Deletions, theme)
	}
	for i, p := range a.paths {
		if p == a.active {
			meta = append(meta, t.ColorSpan(fmt.Sprintf("  %d/%d", i+1, len(a.paths)), theme.TextMuted))
			break
		}
	}
	return t.Row{Style: t.Style{Width: t.Flex(1), Padding: t.EdgeInsetsXY(1, 0), BackgroundColor: bg}, Children: []t.Widget{
		viewerPathText{Text: t.Text{Content: singleLine(a.active), Style: t.Style{Width: t.Flex(1), Bold: true, ForegroundColor: theme.Text}}, FullPath: singleLine(a.active), EllipsisColor: theme.TextMuted},
		t.Spacer{Width: t.Cells(1)}, t.Text{Spans: meta},
	}}
}

func commentCountLabel(count int) string {
	if count == 1 {
		return "1 comment"
	}
	return fmt.Sprintf("%d comments", count)
}
