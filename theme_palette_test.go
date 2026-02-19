package main

import (
	"testing"

	t "github.com/darrenburns/terma"
	"github.com/stretchr/testify/require"
)

func TestThemePalette_GutterTintIsDarkerForAddAndRemove(tt *testing.T) {
	theme, ok := t.GetTheme(t.CurrentThemeName())
	require.True(tt, ok)

	palette := NewThemePalette(theme)

	addLineStyle, ok := palette.LineStyleForKind(RenderedLineAdd)
	require.True(tt, ok)
	addGutterStyle, ok := palette.GutterStyleForKind(RenderedLineAdd)
	require.True(tt, ok)
	require.NotNil(tt, addLineStyle.BackgroundColor)
	require.NotNil(tt, addGutterStyle.BackgroundColor)
	addLineBg := addLineStyle.BackgroundColor.ColorAt(1, 1, 0, 0)
	addGutterBg := addGutterStyle.BackgroundColor.ColorAt(1, 1, 0, 0)
	require.Less(tt, addGutterBg.Luminance(), addLineBg.Luminance())

	removeLineStyle, ok := palette.LineStyleForKind(RenderedLineRemove)
	require.True(tt, ok)
	removeGutterStyle, ok := palette.GutterStyleForKind(RenderedLineRemove)
	require.True(tt, ok)
	require.NotNil(tt, removeLineStyle.BackgroundColor)
	require.NotNil(tt, removeGutterStyle.BackgroundColor)
	removeLineBg := removeLineStyle.BackgroundColor.ColorAt(1, 1, 0, 0)
	removeGutterBg := removeGutterStyle.BackgroundColor.ColorAt(1, 1, 0, 0)
	require.Less(tt, removeGutterBg.Luminance(), removeLineBg.Luminance())

	contextGutterStyle, ok := palette.GutterStyleForKind(RenderedLineContext)
	require.True(tt, ok)
	require.NotNil(tt, contextGutterStyle.BackgroundColor)
	contextGutterBg := contextGutterStyle.BackgroundColor.ColorAt(1, 1, 0, 0)
	require.Less(tt, contextGutterBg.Luminance(), theme.Background.Luminance())
}

func TestThemePalette_IntralineBackgroundAccentsAreStrongerThanBaseLineTint(tt *testing.T) {
	theme, ok := t.GetTheme(t.CurrentThemeName())
	require.True(tt, ok)

	palette := NewThemePalette(theme)

	addLineStyle, ok := palette.LineStyleForKind(RenderedLineAdd)
	require.True(tt, ok)
	require.NotNil(tt, addLineStyle.BackgroundColor)
	addLineBg := addLineStyle.BackgroundColor.ColorAt(1, 1, 0, 0)

	addOverlay, ok := palette.IntralineOverlayStyle(IntralineMarkAdd, IntralineStyleModeBackground)
	require.True(tt, ok)
	require.True(tt, addOverlay.Background.IsSet())
	require.Greater(
		tt,
		colorDistance(theme.Background, addOverlay.Background),
		colorDistance(theme.Background, addLineBg),
	)

	removeLineStyle, ok := palette.LineStyleForKind(RenderedLineRemove)
	require.True(tt, ok)
	require.NotNil(tt, removeLineStyle.BackgroundColor)
	removeLineBg := removeLineStyle.BackgroundColor.ColorAt(1, 1, 0, 0)

	removeOverlay, ok := palette.IntralineOverlayStyle(IntralineMarkRemove, IntralineStyleModeBackground)
	require.True(tt, ok)
	require.True(tt, removeOverlay.Background.IsSet())
	require.Greater(
		tt,
		colorDistance(theme.Background, removeOverlay.Background),
		colorDistance(theme.Background, removeLineBg),
	)
}

func TestThemePalette_IntralineUnderlineStylesUseSemanticColors(tt *testing.T) {
	theme, ok := t.GetTheme(t.CurrentThemeName())
	require.True(tt, ok)

	palette := NewThemePalette(theme)

	addUnderline, ok := palette.IntralineOverlayStyle(IntralineMarkAdd, IntralineStyleModeUnderline)
	require.True(tt, ok)
	require.Equal(tt, t.UnderlineSingle, addUnderline.Underline)
	require.Equal(tt, theme.Success, addUnderline.UnderlineColor)

	removeUnderline, ok := palette.IntralineOverlayStyle(IntralineMarkRemove, IntralineStyleModeUnderline)
	require.True(tt, ok)
	require.Equal(tt, t.UnderlineSingle, removeUnderline.Underline)
	require.Equal(tt, theme.Error, removeUnderline.UnderlineColor)
}

func colorDistance(a t.Color, b t.Color) float64 {
	ar, ag, ab := a.RGB()
	br, bg, bb := b.RGB()
	dr := float64(int(ar) - int(br))
	dg := float64(int(ag) - int(bg))
	db := float64(int(ab) - int(bb))
	return dr*dr + dg*dg + db*db
}
