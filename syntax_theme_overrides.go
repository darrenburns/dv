package main

import t "github.com/darrenburns/terma"

type syntaxColorResolver func(theme t.ThemeData) t.Color

var lightThemeReadableSyntaxResolvers = map[TokenRole]syntaxColorResolver{
	TokenRoleSyntaxPlain: func(theme t.ThemeData) t.Color { return theme.Text },
	TokenRoleSyntaxKeyword: func(theme t.ThemeData) t.Color {
		return theme.AccentText
	},
	TokenRoleSyntaxType: func(theme t.ThemeData) t.Color { return theme.PrimaryText },
	TokenRoleSyntaxFunction: func(theme t.ThemeData) t.Color {
		return theme.SecondaryText
	},
	TokenRoleSyntaxIdentifier: func(theme t.ThemeData) t.Color { return theme.Text },
	TokenRoleSyntaxConstant:   func(theme t.ThemeData) t.Color { return theme.WarningText },
	TokenRoleSyntaxBuiltin:    func(theme t.ThemeData) t.Color { return theme.InfoText },
	TokenRoleSyntaxPreprocessor: func(theme t.ThemeData) t.Color {
		return theme.ErrorText
	},
	TokenRoleSyntaxAttribute: func(theme t.ThemeData) t.Color { return theme.PrimaryText },
	TokenRoleSyntaxParameter: func(theme t.ThemeData) t.Color { return theme.SecondaryText },
	TokenRoleSyntaxString:    func(theme t.ThemeData) t.Color { return theme.SuccessText },
	TokenRoleSyntaxNumber:    func(theme t.ThemeData) t.Color { return theme.WarningText },
	TokenRoleSyntaxRegex:     func(theme t.ThemeData) t.Color { return theme.WarningText },
	TokenRoleSyntaxStringEscape: func(theme t.ThemeData) t.Color {
		return theme.WarningText
	},
	TokenRoleSyntaxTag: func(theme t.ThemeData) t.Color { return theme.AccentText },
	TokenRoleSyntaxComment: func(theme t.ThemeData) t.Color {
		return theme.TextMuted.Blend(theme.Text, 0.4)
	},
	TokenRoleSyntaxOperator:    func(theme t.ThemeData) t.Color { return theme.Text },
	TokenRoleSyntaxPunctuation: func(theme t.ThemeData) t.Color { return theme.Text },
}

var syntaxThemeOverrides = map[string]map[TokenRole]syntaxColorResolver{
	t.ThemeNameCatppuccinLatte: lightThemeReadableSyntaxResolvers,
	t.ThemeNameDraculaLight:    lightThemeReadableSyntaxResolvers,
	t.ThemeNameGruvboxLight:    lightThemeReadableSyntaxResolvers,
	t.ThemeNameMonokaiLight:    lightThemeReadableSyntaxResolvers,
	t.ThemeNameNordLight:       lightThemeReadableSyntaxResolvers,
	t.ThemeNameRosePineDawn:    lightThemeReadableSyntaxResolvers,
	t.ThemeNameSolarizedLight:  lightThemeReadableSyntaxResolvers,
	t.ThemeNameTokyoNightDay:   lightThemeReadableSyntaxResolvers,
	t.ThemeNameTokyoNight: {
		TokenRoleSyntaxPlain:       func(theme t.ThemeData) t.Color { return t.Hex("#c0caf5") },
		TokenRoleSyntaxKeyword:     func(theme t.ThemeData) t.Color { return t.Hex("#bb9af7") },
		TokenRoleSyntaxType:        func(theme t.ThemeData) t.Color { return t.Hex("#7dcfff") },
		TokenRoleSyntaxFunction:    func(theme t.ThemeData) t.Color { return t.Hex("#7aa2f7") },
		TokenRoleSyntaxIdentifier:  func(theme t.ThemeData) t.Color { return t.Hex("#c0caf5") },
		TokenRoleSyntaxConstant:    func(theme t.ThemeData) t.Color { return t.Hex("#ff9e64") },
		TokenRoleSyntaxBuiltin:     func(theme t.ThemeData) t.Color { return t.Hex("#7dcfff") },
		TokenRoleSyntaxPreprocessor: func(theme t.ThemeData) t.Color { return t.Hex("#f7768e") },
		TokenRoleSyntaxAttribute:   func(theme t.ThemeData) t.Color { return t.Hex("#bb9af7") },
		TokenRoleSyntaxParameter:   func(theme t.ThemeData) t.Color { return t.Hex("#e0af68") },
		TokenRoleSyntaxString:      func(theme t.ThemeData) t.Color { return t.Hex("#9ece6a") },
		TokenRoleSyntaxNumber:      func(theme t.ThemeData) t.Color { return t.Hex("#ff9e64") },
		TokenRoleSyntaxRegex:       func(theme t.ThemeData) t.Color { return t.Hex("#e0af68") },
		TokenRoleSyntaxStringEscape: func(theme t.ThemeData) t.Color { return t.Hex("#89ddff") },
		TokenRoleSyntaxTag:         func(theme t.ThemeData) t.Color { return t.Hex("#f7768e") },
		TokenRoleSyntaxComment:     func(theme t.ThemeData) t.Color { return t.Hex("#565f89") },
		TokenRoleSyntaxOperator:    func(theme t.ThemeData) t.Color { return t.Hex("#89ddff") },
		TokenRoleSyntaxPunctuation: func(theme t.ThemeData) t.Color { return t.Hex("#c0caf5") },
	},
	t.ThemeNameKanagawa: {
		TokenRoleSyntaxKeyword:    func(theme t.ThemeData) t.Color { return theme.Secondary },
		TokenRoleSyntaxType:       func(theme t.ThemeData) t.Color { return theme.Accent },
		TokenRoleSyntaxFunction:   func(theme t.ThemeData) t.Color { return theme.Primary },
		TokenRoleSyntaxIdentifier: func(theme t.ThemeData) t.Color { return theme.Text },
		TokenRoleSyntaxConstant:   func(theme t.ThemeData) t.Color { return t.Hex("#FFA066") },
			TokenRoleSyntaxBuiltin:    func(theme t.ThemeData) t.Color { return theme.Info },
		TokenRoleSyntaxPreprocessor: func(theme t.ThemeData) t.Color {
			return t.Hex("#E46876")
		},
		TokenRoleSyntaxAttribute: func(theme t.ThemeData) t.Color { return t.Hex("#FFA066") },
		TokenRoleSyntaxParameter: func(theme t.ThemeData) t.Color { return t.Hex("#B8B4D0") },
		TokenRoleSyntaxString:    func(theme t.ThemeData) t.Color { return theme.Success },
		TokenRoleSyntaxNumber:    func(theme t.ThemeData) t.Color { return t.Hex("#D27E99") },
		TokenRoleSyntaxRegex:     func(theme t.ThemeData) t.Color { return t.Hex("#C0A36E") },
		TokenRoleSyntaxStringEscape: func(theme t.ThemeData) t.Color {
			return t.Hex("#C0A36E")
		},
		TokenRoleSyntaxTag:         func(theme t.ThemeData) t.Color { return t.Hex("#E6C384") },
		TokenRoleSyntaxComment:     func(theme t.ThemeData) t.Color { return theme.TextDisabled },
		TokenRoleSyntaxOperator:    func(theme t.ThemeData) t.Color { return t.Hex("#C0A36E") },
		TokenRoleSyntaxPunctuation: func(theme t.ThemeData) t.Color { return t.Hex("#9CABCA") },
	},
}

func applySyntaxThemeOverrides(theme t.ThemeData, roleStyles map[TokenRole]t.SpanStyle) {
	overrides, ok := syntaxThemeOverrides[theme.Name]
	if !ok {
		return
	}

	for role, resolver := range overrides {
		if !isSyntaxOverrideableRole(role) || resolver == nil {
			continue
		}

		style, ok := roleStyles[role]
		if !ok {
			continue
		}

		style.Foreground = resolver(theme)
		roleStyles[role] = style
	}
}

func isSyntaxOverrideableRole(role TokenRole) bool {
	switch role {
	case TokenRoleSyntaxPlain,
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
		TokenRoleSyntaxPunctuation:
		return true
	default:
		return false
	}
}
