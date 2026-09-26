package main

import t "github.com/darrenburns/terma"

// The same diagonal fill used in dvdev's unused diff space. The positioned
// backdrop takes its size from the real card, and never changes its hit targets.
type viewerEmptySpaceHatch struct {
	Style      t.Style
	Foreground t.ColorProvider
}

func (v viewerEmptySpaceHatch) Build(t.BuildContext) t.Widget { return v }
func (v viewerEmptySpaceHatch) GetStyle() t.Style             { return v.Style }
func (v viewerEmptySpaceHatch) Render(ctx *t.RenderContext) {
	for y := 0; y < ctx.Height; y++ {
		for x := 0; x < ctx.Width; x++ {
			ctx.DrawStyledText(x, y, sideEmptyHatchRune, t.Style{
				ForegroundColor: v.Foreground.ColorAt(ctx.Width, ctx.Height, x, y),
				BackgroundColor: v.Style.BackgroundColor,
			})
		}
	}
}
func hatchedFrame(theme t.ThemeData, card t.Widget) t.Widget {
	return t.Stack{Style: t.Style{Width: t.Flex(1), Height: t.Auto}, Children: []t.Widget{
		t.PositionedFill(viewerEmptySpaceHatch{Style: t.Style{BackgroundColor: theme.Background}, Foreground: t.NewGradient(theme.Background.Blend(theme.TextDisabled, .28), theme.Background.Blend(theme.Primary, .18)).WithAngle(45)}),
		t.Column{Style: t.Style{Width: t.Flex(1), Height: t.Auto, Padding: t.EdgeInsetsXY(2, 1)}, Children: []t.Widget{card}},
	}}
}

// A button keeps its normal Terma focus/mouse behavior and displays its own
// shortcut. The same shortcut works when the button itself has keyboard focus.
type shortcutButton struct {
	t.Button
	Shortcut, Key string
}

func (b shortcutButton) Build(ctx t.BuildContext) t.Widget {
	text := b.Button.Build(ctx).(t.Text)
	spans := text.Spans
	keyColor := ctx.Theme().TextMuted
	if ctx.IsFocused(b) || b.Variant == t.ButtonPrimary {
		keyColor = text.Style.ForegroundColor.ColorAt(1, 1, 0, 0)
	}
	text.Spans = append(spans[:len(spans)-1], t.StyledSpan("  "+b.Shortcut, t.SpanStyle{Foreground: keyColor}), spans[len(spans)-1])
	return text
}
func (b shortcutButton) Keybinds() []t.Keybind {
	return append(b.Button.Keybinds(), t.Keybind{Key: b.Key, Name: b.Label, Hidden: true, Action: b.OnPress})
}
