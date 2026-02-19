package main

import t "github.com/darrenburns/terma"

type IntralineStyleMode int

const (
	IntralineStyleModeBackground IntralineStyleMode = iota
	IntralineStyleModeUnderline
)

// ThemePalette stores all styles needed by the diff renderer for one theme.
type ThemePalette struct {
	roleStyles      map[TokenRole]t.SpanStyle
	lineStyles      map[RenderedLineKind]t.Style
	gutterStyles    map[RenderedLineKind]t.Style
	intralineStyles map[intralineStyleKey]t.SpanStyle
}

type intralineStyleKey struct {
	mark IntralineMarkKind
	mode IntralineStyleMode
}

func NewThemePalette(theme t.ThemeData) ThemePalette {
	const gutterDarkenAmount = 0.08

	addBg := theme.Background.Blend(theme.Success, 0.14)
	removeBg := theme.Background.Blend(theme.Error, 0.14)
	contextGutterBg := theme.Background.Darken(gutterDarkenAmount)
	hunkBg := theme.Background.Blend(theme.Info, 0.1)
	hunkFg := theme.TextMuted.Blend(theme.InfoText, 0.35)
	headerBg := theme.Background.Blend(theme.Primary, 0.11)
	lineNumberFg := theme.TextMuted.Blend(theme.TextDisabled, 0.35)
	hatchFg := theme.Background.Blend(theme.TextDisabled, 0.26)
	addIntralineBg := theme.Background.Blend(theme.Success, 0.28)
	removeIntralineBg := theme.Background.Blend(theme.Error, 0.28)

	return ThemePalette{
		roleStyles: map[TokenRole]t.SpanStyle{
			TokenRoleOldLineNumber:     {Foreground: lineNumberFg},
			TokenRoleNewLineNumber:     {Foreground: lineNumberFg},
			TokenRoleLineNumberAdd:     {Foreground: theme.Success},
			TokenRoleLineNumberRemove:  {Foreground: theme.Error},
			TokenRoleDiffPrefixAdd:     {Foreground: theme.Success},
			TokenRoleDiffPrefixRemove:  {Foreground: theme.Error},
			TokenRoleDiffPrefixContext: {Foreground: theme.TextMuted},
			TokenRoleDiffFileHeader:    {Foreground: theme.PrimaryText, Bold: true},
			TokenRoleDiffHunkHeader:    {Foreground: hunkFg},
			TokenRoleDiffMeta:          {Foreground: theme.WarningText, Italic: true},
			TokenRoleDiffHatch:         {Foreground: hatchFg},
			TokenRoleSyntaxPlain:       {Foreground: theme.Text},
			TokenRoleSyntaxKeyword:     {Foreground: theme.Accent, Bold: true},
			TokenRoleSyntaxType:        {Foreground: theme.Primary},
			TokenRoleSyntaxFunction:    {Foreground: theme.Secondary},
			TokenRoleSyntaxString:      {Foreground: theme.Success},
			TokenRoleSyntaxNumber:      {Foreground: theme.Accent},
			TokenRoleSyntaxComment:     {Foreground: theme.TextMuted, Italic: true},
			TokenRoleSyntaxPunctuation: {Foreground: theme.Text},
		},
		lineStyles: map[RenderedLineKind]t.Style{
			RenderedLineFileHeader: {BackgroundColor: headerBg},
			RenderedLineHunkHeader: {BackgroundColor: hunkBg},
			RenderedLineAdd:        {BackgroundColor: addBg},
			RenderedLineRemove:     {BackgroundColor: removeBg},
		},
		gutterStyles: map[RenderedLineKind]t.Style{
			RenderedLineContext: {BackgroundColor: contextGutterBg},
			RenderedLineAdd:     {BackgroundColor: addBg.Darken(gutterDarkenAmount)},
			RenderedLineRemove:  {BackgroundColor: removeBg.Darken(gutterDarkenAmount)},
		},
		intralineStyles: map[intralineStyleKey]t.SpanStyle{
			{mark: IntralineMarkAdd, mode: IntralineStyleModeBackground}:    {Background: addIntralineBg},
			{mark: IntralineMarkRemove, mode: IntralineStyleModeBackground}: {Background: removeIntralineBg},
			{mark: IntralineMarkAdd, mode: IntralineStyleModeUnderline}: {
				Underline:      t.UnderlineSingle,
				UnderlineColor: theme.Success,
			},
			{mark: IntralineMarkRemove, mode: IntralineStyleModeUnderline}: {
				Underline:      t.UnderlineSingle,
				UnderlineColor: theme.Error,
			},
		},
	}
}

func (p ThemePalette) StyleForRole(role TokenRole) (t.SpanStyle, bool) {
	style, ok := p.roleStyles[role]
	return style, ok
}

func (p ThemePalette) LineStyleForKind(kind RenderedLineKind) (t.Style, bool) {
	style, ok := p.lineStyles[kind]
	return style, ok
}

func (p ThemePalette) GutterStyleForKind(kind RenderedLineKind) (t.Style, bool) {
	style, ok := p.gutterStyles[kind]
	return style, ok
}

func (p ThemePalette) IntralineOverlayStyle(mark IntralineMarkKind, mode IntralineStyleMode) (t.SpanStyle, bool) {
	if mark == IntralineMarkNone {
		return t.SpanStyle{}, false
	}
	style, ok := p.intralineStyles[intralineStyleKey{mark: mark, mode: mode}]
	return style, ok
}
