package main

import t "github.com/darrenburns/terma"

const (
	themeMoonstone = "moonstone"
	themeBonsai    = "bonsai"
	themeGarnet    = "garnet"
	themeKintsugi  = "kintsugi"
	themeAmethyst  = "amethyst"
	themeLantern   = "lantern"
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
	{
		Name: themeKintsugi,
		// Glazed charcoal, gold seams, indigo glaze and persimmon.
		Primary: t.Hex("#e2b964"), Secondary: t.Hex("#93aee0"), Accent: t.Hex("#ef9471"),
		Background: t.Hex("#1b1a18"), Surface: t.Hex("#242220"), SurfaceHover: t.Hex("#38352f"),
		Surface2: t.Hex("#2c2a26"), Surface3: t.Hex("#36332e"),
		Text: t.Hex("#ece6da"), TextMuted: t.Hex("#aca496"), TextDisabled: t.Hex("#756e62"),
		Border: t.Hex("#4d4942"),
		Error:  t.Hex("#e8818f"), Warning: t.Hex("#e5d49b"), Success: t.Hex("#9fc8a6"), Info: t.Hex("#86c3c6"),
	},
	{
		Name: themeAmethyst,
		// Violet dusk, amethyst, clear sky and butter yellow.
		Primary: t.Hex("#c09cf5"), Secondary: t.Hex("#8ec5f2"), Accent: t.Hex("#ecd28c"),
		Background: t.Hex("#1c1624"), Surface: t.Hex("#241d2e"), SurfaceHover: t.Hex("#382e46"),
		Surface2: t.Hex("#2b2336"), Surface3: t.Hex("#352c42"),
		Text: t.Hex("#ebe4f2"), TextMuted: t.Hex("#aa9dbb"), TextDisabled: t.Hex("#736786"),
		Border: t.Hex("#4c405c"),
		Error:  t.Hex("#f08394"), Warning: t.Hex("#f2ad7c"), Success: t.Hex("#a8d59a"), Info: t.Hex("#8fd6cc"),
	},
	{
		Name: themeLantern,
		// Indigo night, lantern amber, river cyan and blossom pink.
		Primary: t.Hex("#f6ab6c"), Secondary: t.Hex("#86cfe0"), Accent: t.Hex("#f297b5"),
		Background: t.Hex("#17152b"), Surface: t.Hex("#201d37"), SurfaceHover: t.Hex("#322e4f"),
		Surface2: t.Hex("#282441"), Surface3: t.Hex("#332f4d"),
		Text: t.Hex("#e9e5f4"), TextMuted: t.Hex("#a7a1c4"), TextDisabled: t.Hex("#6f6992"),
		Border: t.Hex("#48426a"),
		Error:  t.Hex("#f07a82"), Warning: t.Hex("#f3d27c"), Success: t.Hex("#b3d68a"), Info: t.Hex("#b7a3f0"),
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
