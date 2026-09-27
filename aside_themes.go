package main

import t "github.com/darrenburns/terma"

const (
	themeMoonstone = "moonstone"
	themeBonsai    = "bonsai"
	themeGarnet    = "garnet"
)

// Aside-specific palettes use Terma's registry, so the picker, live preview,
// configuration and command-line theme selection all share the same source.
var asideThemes = []t.ThemeData{
	{
		Name: themeMoonstone,
		// Blue slate, moonlit lavender, sea glass and warm peach.
		Primary: t.Hex("#a7b8ff"), Secondary: t.Hex("#83d5ce"), Accent: t.Hex("#efbd93"),
		Background: t.Hex("#171c2b"), Surface: t.Hex("#20283b"), SurfaceHover: t.Hex("#303d54"),
		Surface2: t.Hex("#283248"), Surface3: t.Hex("#343f55"),
		Text: t.Hex("#e4e9f2"), TextMuted: t.Hex("#9aaac1"), TextDisabled: t.Hex("#65758f"),
		Border: t.Hex("#46516b"),
		Error:  t.Hex("#f18f9b"), Warning: t.Hex("#e9cc87"), Success: t.Hex("#a6d49a"), Info: t.Hex("#8fcaed"),
	},
	{
		Name: themeBonsai,
		// Forest charcoal, fresh sage, jade and fired copper.
		Primary: t.Hex("#b8ce83"), Secondary: t.Hex("#80c6b2"), Accent: t.Hex("#eca982"),
		Background: t.Hex("#181e1b"), Surface: t.Hex("#212b25"), SurfaceHover: t.Hex("#344137"),
		Surface2: t.Hex("#2a352d"), Surface3: t.Hex("#374238"),
		Text: t.Hex("#e5e6d7"), TextMuted: t.Hex("#a7b29e"), TextDisabled: t.Hex("#6d7d6e"),
		Border: t.Hex("#475849"),
		Error:  t.Hex("#eb9894"), Warning: t.Hex("#e2c383"), Success: t.Hex("#98c889"), Info: t.Hex("#92c0cd"),
	},
	{
		Name: themeGarnet,
		// Dark burgundy, antique rose, lavender and champagne.
		Primary: t.Hex("#eda4b1"), Secondary: t.Hex("#b7abe8"), Accent: t.Hex("#edca99"),
		Background: t.Hex("#24191e"), Surface: t.Hex("#302127"), SurfaceHover: t.Hex("#49343c"),
		Surface2: t.Hex("#3a2931"), Surface3: t.Hex("#49343c"),
		Text: t.Hex("#efe0dd"), TextMuted: t.Hex("#c0a6ac"), TextDisabled: t.Hex("#876973"),
		Border: t.Hex("#63454f"),
		Error:  t.Hex("#ef9790"), Warning: t.Hex("#e4c284"), Success: t.Hex("#a4c7a1"), Info: t.Hex("#98cbd2"),
	},
}

func init() {
	for _, theme := range asideThemes {
		theme.TextOnPrimary = theme.Primary.AutoText()
		theme.TextOnSecondary = theme.Secondary.AutoText()
		theme.TextOnAccent = theme.Accent.AutoText()
		theme.TextOnError = theme.Error.AutoText()
		theme.TextOnWarning = theme.Warning.AutoText()
		theme.TextOnSuccess = theme.Success.AutoText()
		theme.TextOnInfo = theme.Info.AutoText()
		theme.FocusRing = theme.Primary
		theme.ActiveCursor = theme.Primary
		theme.Selection = theme.Primary.WithAlpha(t.DefaultSelectionAlpha)
		theme.SelectionText = theme.TextOnPrimary
		theme.ScrollbarTrack = theme.Surface
		theme.ScrollbarThumb = theme.TextDisabled
		theme.Overlay = theme.Background.WithAlpha(.85)
		theme.Placeholder = theme.TextMuted
		theme.Cursor = theme.Primary
		theme.Link = theme.Primary
		t.RegisterTheme(theme.Name, theme)
	}
}
