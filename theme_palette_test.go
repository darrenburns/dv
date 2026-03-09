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

func TestThemePalette_SyntaxOverrides_KanagawaPartialOverride(tt *testing.T) {
	theme, ok := t.GetTheme(t.ThemeNameKanagawa)
	require.True(tt, ok)

	palette := NewThemePalette(theme)

	expected := map[TokenRole]t.Color{
		TokenRoleSyntaxKeyword:      theme.Secondary,
		TokenRoleSyntaxType:         theme.Accent,
		TokenRoleSyntaxFunction:     theme.Primary,
		TokenRoleSyntaxIdentifier:   theme.Text,
		TokenRoleSyntaxConstant:     t.Hex("#FFA066"),
		TokenRoleSyntaxBuiltin:      theme.Info,
		TokenRoleSyntaxPreprocessor: t.Hex("#E46876"),
		TokenRoleSyntaxAttribute:    t.Hex("#FFA066"),
		TokenRoleSyntaxParameter:    t.Hex("#B8B4D0"),
		TokenRoleSyntaxString:       theme.Success,
		TokenRoleSyntaxNumber:       t.Hex("#D27E99"),
		TokenRoleSyntaxRegex:        t.Hex("#C0A36E"),
		TokenRoleSyntaxStringEscape: t.Hex("#C0A36E"),
		TokenRoleSyntaxTag:          t.Hex("#E6C384"),
		TokenRoleSyntaxComment:      theme.TextDisabled,
		TokenRoleSyntaxOperator:     t.Hex("#C0A36E"),
		TokenRoleSyntaxPunctuation:  t.Hex("#9CABCA"),
	}

	for role, want := range expected {
		style := mustRoleStyle(tt, palette, role)
		require.Equal(tt, want, style.Foreground)
	}

	plainStyle := mustRoleStyle(tt, palette, TokenRoleSyntaxPlain)
	require.Equal(tt, theme.Text, plainStyle.Foreground, "plain should fall back to the default syntax color")

	keywordStyle := mustRoleStyle(tt, palette, TokenRoleSyntaxKeyword)
	require.True(tt, keywordStyle.Bold)

	commentStyle := mustRoleStyle(tt, palette, TokenRoleSyntaxComment)
	require.True(tt, commentStyle.Italic)
}

func TestThemePalette_SyntaxOverrides_LightThemesUseReadableProfile(tt *testing.T) {
	for _, themeName := range lightOverrideThemeNames() {
		themeName := themeName
		tt.Run(themeName, func(tt *testing.T) {
			theme, ok := t.GetTheme(themeName)
			require.True(tt, ok)

			palette := NewThemePalette(theme)
			expected := expectedLightReadableSyntaxForegrounds(theme)

			for _, role := range syntaxOverrideRoles() {
				style := mustRoleStyle(tt, palette, role)
				require.Equal(tt, expected[role], style.Foreground)
			}

			keywordStyle := mustRoleStyle(tt, palette, TokenRoleSyntaxKeyword)
			require.True(tt, keywordStyle.Bold)

			commentStyle := mustRoleStyle(tt, palette, TokenRoleSyntaxComment)
			require.True(tt, commentStyle.Italic)
		})
	}
}

func TestThemePalette_SyntaxOverrides_DarkThemeStructuralSyntaxUsesSharedProfile(tt *testing.T) {
	theme, ok := t.GetTheme(t.ThemeNameObsidianTide)
	require.True(tt, ok)

	palette := NewThemePalette(theme)
	expected := expectedDarkThemeSyntaxForegrounds(theme)
	for _, role := range syntaxOverrideRoles() {
		style := mustRoleStyle(tt, palette, role)
		require.Equal(tt, expected[role], style.Foreground)
	}
}

func TestThemePalette_SyntaxOverrides_AllDarkThemesUseSpecificOperatorAndPunctuationColors(tt *testing.T) {
	for _, themeName := range t.DarkThemeNames() {
		themeName := themeName
		tt.Run(themeName, func(tt *testing.T) {
			theme, ok := t.GetTheme(themeName)
			require.True(tt, ok)

			palette := NewThemePalette(theme)

			operatorStyle := mustRoleStyle(tt, palette, TokenRoleSyntaxOperator)
			require.Equal(tt, expectedDarkThemeOperatorForeground(theme), operatorStyle.Foreground)
			require.NotEqual(tt, theme.Text, operatorStyle.Foreground)

			punctuationStyle := mustRoleStyle(tt, palette, TokenRoleSyntaxPunctuation)
			require.Equal(tt, expectedDarkThemePunctuationForeground(theme), punctuationStyle.Foreground)
			require.NotEqual(tt, theme.Text, punctuationStyle.Foreground)
		})
	}
}

func TestThemePalette_SyntaxOverrides_DoNotAffectNonSyntaxRoles(tt *testing.T) {
	theme, ok := t.GetTheme(t.ThemeNameKanagawa)
	require.True(tt, ok)

	palette := NewThemePalette(theme)
	expected := expectedDefaultNonSyntaxForegrounds(theme)
	for role, want := range expected {
		style := mustRoleStyle(tt, palette, role)
		require.Equal(tt, want, style.Foreground)
	}

	fileHeaderStyle := mustRoleStyle(tt, palette, TokenRoleDiffFileHeader)
	require.True(tt, fileHeaderStyle.Bold)

	diffMetaStyle := mustRoleStyle(tt, palette, TokenRoleDiffMeta)
	require.True(tt, diffMetaStyle.Italic)
}

func TestThemePalette_SyntaxOverrides_LightThemeSyntaxReadabilityFloor(tt *testing.T) {
	for _, themeName := range lightOverrideThemeNames() {
		themeName := themeName
		tt.Run(themeName, func(tt *testing.T) {
			theme, ok := t.GetTheme(themeName)
			require.True(tt, ok)

			palette := NewThemePalette(theme)
			for _, role := range syntaxOverrideRoles() {
				style := mustRoleStyle(tt, palette, role)
				ratio := style.Foreground.ContrastRatio(theme.Background)
				require.GreaterOrEqualf(tt, ratio, 3.0, "theme=%s role=%d", themeName, role)
			}
		})
	}
}

func mustRoleStyle(tt *testing.T, palette ThemePalette, role TokenRole) t.SpanStyle {
	tt.Helper()

	style, ok := palette.StyleForRole(role)
	require.True(tt, ok)
	require.True(tt, style.Foreground.IsSet())
	return style
}

func syntaxOverrideRoles() []TokenRole {
	return []TokenRole{
		TokenRoleSyntaxPlain,
		TokenRoleSyntaxKeyword,
		TokenRoleSyntaxType,
		TokenRoleSyntaxFunction,
		TokenRoleSyntaxIdentifier,
		TokenRoleSyntaxConstant,
		TokenRoleSyntaxBuiltin,
		TokenRoleSyntaxPreprocessor,
		TokenRoleSyntaxAttribute,
		TokenRoleSyntaxParameter,
		TokenRoleSyntaxString,
		TokenRoleSyntaxNumber,
		TokenRoleSyntaxRegex,
		TokenRoleSyntaxStringEscape,
		TokenRoleSyntaxTag,
		TokenRoleSyntaxComment,
		TokenRoleSyntaxOperator,
		TokenRoleSyntaxPunctuation,
	}
}

func lightOverrideThemeNames() []string {
	return []string{
		t.ThemeNameCatppuccinLatte,
		t.ThemeNameDraculaLight,
		t.ThemeNameGruvboxLight,
		t.ThemeNameMonokaiLight,
		t.ThemeNameNordLight,
		t.ThemeNameRosePineDawn,
		t.ThemeNameSolarizedLight,
		t.ThemeNameTokyoNightDay,
	}
}

func expectedLightReadableSyntaxForegrounds(theme t.ThemeData) map[TokenRole]t.Color {
	return map[TokenRole]t.Color{
		TokenRoleSyntaxPlain:        theme.Text,
		TokenRoleSyntaxKeyword:      theme.AccentText,
		TokenRoleSyntaxType:         theme.PrimaryText,
		TokenRoleSyntaxFunction:     theme.SecondaryText,
		TokenRoleSyntaxIdentifier:   theme.Text,
		TokenRoleSyntaxConstant:     theme.WarningText,
		TokenRoleSyntaxBuiltin:      theme.InfoText,
		TokenRoleSyntaxPreprocessor: theme.ErrorText,
		TokenRoleSyntaxAttribute:    theme.PrimaryText,
		TokenRoleSyntaxParameter:    theme.SecondaryText,
		TokenRoleSyntaxString:       theme.SuccessText,
		TokenRoleSyntaxNumber:       theme.WarningText,
		TokenRoleSyntaxRegex:        theme.WarningText,
		TokenRoleSyntaxStringEscape: theme.WarningText,
		TokenRoleSyntaxTag:          theme.AccentText,
		TokenRoleSyntaxComment:      theme.TextMuted.Blend(theme.Text, 0.4),
		TokenRoleSyntaxOperator:     theme.Text,
		TokenRoleSyntaxPunctuation:  theme.Text,
	}
}

func expectedDarkThemeSyntaxForegrounds(theme t.ThemeData) map[TokenRole]t.Color {
	return map[TokenRole]t.Color{
		TokenRoleSyntaxPlain:        theme.Text,
		TokenRoleSyntaxKeyword:      theme.Accent,
		TokenRoleSyntaxType:         theme.Primary,
		TokenRoleSyntaxFunction:     theme.Secondary,
		TokenRoleSyntaxIdentifier:   theme.Text,
		TokenRoleSyntaxConstant:     theme.Warning,
		TokenRoleSyntaxBuiltin:      theme.Primary,
		TokenRoleSyntaxPreprocessor: theme.Error,
		TokenRoleSyntaxAttribute:    theme.Info,
		TokenRoleSyntaxParameter:    theme.Secondary,
		TokenRoleSyntaxString:       theme.Success,
		TokenRoleSyntaxNumber:       theme.Accent,
		TokenRoleSyntaxRegex:        theme.Warning,
		TokenRoleSyntaxStringEscape: theme.Warning,
		TokenRoleSyntaxTag:          theme.Accent,
		TokenRoleSyntaxComment:      theme.TextMuted,
		TokenRoleSyntaxOperator:     expectedDarkThemeOperatorForeground(theme),
		TokenRoleSyntaxPunctuation:  expectedDarkThemePunctuationForeground(theme),
	}
}

func expectedDarkThemeOperatorForeground(theme t.ThemeData) t.Color {
	switch theme.Name {
	case t.ThemeNameTokyoNight:
		return t.Hex("#89ddff")
	case t.ThemeNameKanagawa:
		return t.Hex("#C0A36E")
	default:
		return theme.Text.Blend(theme.Info, 0.35)
	}
}

func expectedDarkThemePunctuationForeground(theme t.ThemeData) t.Color {
	switch theme.Name {
	case t.ThemeNameTokyoNight:
		return theme.TextMuted.Blend(theme.Primary, 0.25)
	case t.ThemeNameKanagawa:
		return t.Hex("#9CABCA")
	default:
		return theme.TextMuted.Blend(theme.Primary, 0.25)
	}
}

func expectedDefaultNonSyntaxForegrounds(theme t.ThemeData) map[TokenRole]t.Color {
	lineNumberFg := theme.TextMuted.Blend(theme.TextDisabled, 0.35)
	hunkFg := theme.TextMuted.Blend(theme.InfoText, 0.35)
	hatchFg := theme.Background.Blend(theme.TextDisabled, 0.26)
	return map[TokenRole]t.Color{
		TokenRoleOldLineNumber:     lineNumberFg,
		TokenRoleNewLineNumber:     lineNumberFg,
		TokenRoleLineNumberAdd:     theme.Success,
		TokenRoleLineNumberRemove:  theme.Error,
		TokenRoleDiffPrefixAdd:     theme.Success,
		TokenRoleDiffPrefixRemove:  theme.Error,
		TokenRoleDiffPrefixContext: theme.TextMuted,
		TokenRoleDiffFileHeader:    theme.PrimaryText,
		TokenRoleDiffHunkHeader:    hunkFg,
		TokenRoleDiffMeta:          theme.WarningText,
		TokenRoleDiffHatch:         hatchFg,
	}
}

func TestThemePalette_IntralineOffHasNoOverlay(tt *testing.T) {
	theme, ok := t.GetTheme(t.CurrentThemeName())
	require.True(tt, ok)

	palette := NewThemePalette(theme)

	_, ok = palette.IntralineOverlayStyle(IntralineMarkAdd, IntralineStyleModeOff)
	require.False(tt, ok)
	_, ok = palette.IntralineOverlayStyle(IntralineMarkRemove, IntralineStyleModeOff)
	require.False(tt, ok)
}

func colorDistance(a t.Color, b t.Color) float64 {
	ar, ag, ab := a.RGB()
	br, bg, bb := b.RGB()
	dr := float64(int(ar) - int(br))
	dg := float64(int(ag) - int(bg))
	db := float64(int(ab) - int(bb))
	return dr*dr + dg*dg + db*db
}
