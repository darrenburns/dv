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
