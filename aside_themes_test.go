package main

import (
	"strings"
	"testing"

	t "github.com/darrenburns/terma"
	"github.com/stretchr/testify/require"
)

func TestAsideThemesReadability(tt *testing.T) {
	for _, definition := range asideThemes {
		tt.Run(definition.Name, func(tt *testing.T) {
			theme, ok := t.GetTheme(definition.Name)
			require.True(tt, ok)
			name, err := parseThemeName(strings.ToUpper(definition.Name))
			require.NoError(tt, err)
			require.Equal(tt, definition.Name, name)
			if theme.IsLight {
				require.Contains(tt, t.LightThemeNames(), name)
			} else {
				require.Contains(tt, t.DarkThemeNames(), name)
			}
			for _, background := range []t.Color{theme.Background, theme.Surface, theme.SurfaceHover} {
				require.GreaterOrEqual(tt, theme.Text.ContrastRatio(background), 4.5, "body text")
				require.GreaterOrEqual(tt, theme.TextMuted.ContrastRatio(background), 4.5, "comment previews and secondary text")
			}
			require.GreaterOrEqual(tt, theme.SelectionText.ContrastRatio(theme.ActiveCursor), 4.5, "focused navigator row")
			palette := NewThemePalette(theme)
			for role := TokenRoleSyntaxPlain; role <= TokenRoleSyntaxPunctuation; role++ {
				style, exists := palette.StyleForRole(role)
				require.True(tt, exists)
				for _, background := range []t.Color{theme.Background, theme.Background.Blend(theme.Success, .14), theme.Background.Blend(theme.Error, .14)} {
					require.GreaterOrEqualf(tt, style.Foreground.ContrastRatio(background), 3.0, "syntax role %d on context/add/remove backgrounds", role)
				}
			}
		})
	}
}

func snapshotAsideTheme(tt *testing.T, name string) {
	tt.Helper()
	previous := t.CurrentThemeName()
	tt.Cleanup(func() { t.SetTheme(previous) })
	t.SetTheme(name)
	a := navigatorFixture(tt, 2)
	r, err := a.store.Update(a.review.ID, nil, func(r *Review) error {
		r.Threads[0].Messages[0].Body = "Handle empty names before formatting the greeting."
		r.Threads[1].Kind = "explanation"
		r.Threads[1].Messages[0].Body = "Keep the greeting consistent for existing callers."
		return nil
	})
	require.NoError(tt, err)
	a.apply(r)
	a.mode.Set(DiffLayoutSideBySide)
	a.renderActive(false)
	a.focusComment(r.Threads[0].ID)
	a.reviewSplit.SetPosition(.4)
	t.AssertSnapshot(tt, a, 120, 36, name+": split diff, selection gutter, comment cards and review navigator")
}

func TestSnapshotAsideMoonstone(tt *testing.T) { snapshotAsideTheme(tt, themeMoonstone) }
func TestSnapshotAsideBonsai(tt *testing.T)    { snapshotAsideTheme(tt, themeBonsai) }
func TestSnapshotAsideGarnet(tt *testing.T)    { snapshotAsideTheme(tt, themeGarnet) }
