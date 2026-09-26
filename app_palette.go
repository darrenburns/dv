package main

import t "github.com/darrenburns/terma"

const paletteID = "aside-palette"

func (a *Aside) openPalette() {
	if a.editing.Peek() {
		return
	}
	if a.palette.Visible.Peek() {
		a.dismissPalette()
		a.palette.Close()
		return
	}
	a.palette.SetItems(a.paletteItems())
	a.palette.CurrentLevel().FilterState.Mode.Set(t.FilterFuzzy)
	a.palette.Open()
}
func (a *Aside) openThemes() {
	if a.editing.Peek() {
		return
	}
	if !a.palette.Visible.Peek() {
		a.openPalette()
	}
	a.beginThemePreview()
	a.palette.PushLevel("Themes", a.themeItems())
	a.palette.CurrentLevel().FilterState.Mode.Set(t.FilterFuzzy)
	for i, item := range a.palette.CurrentLevel().Items {
		if name, ok := item.Data.(string); ok && name == t.CurrentThemeName() {
			a.palette.CurrentLevel().ListState.SelectIndex(i)
			a.palette.CurrentLevel().ScrollState.Offset.Set(i)
			break
		}
	}
	t.RequestFocus(paletteID + "-input")
}
func (a *Aside) paletteAction(action func()) func() {
	return func() { a.dismissPalette(); a.palette.Close(); action() }
}
func (a *Aside) paletteItems() []t.CommandPaletteItem {
	items := []t.CommandPaletteItem{}
	add := func(label, key string, action func()) {
		items = append(items, t.CommandPaletteItem{Label: label, Hint: key, Action: a.paletteAction(action)})
	}
	add("Comment on selected code", "c", a.beginComment)
	add("Submit and copy feedback", "f", a.exportFeedback)
	add("Copy code or file path", "y", a.copySelectionOrPath)
	add("Next comment", "]", func() { a.nextThread(1) })
	add("Previous comment", "[", func() { a.nextThread(-1) })
	add("Refresh review", "r", a.refresh)
	items = append(items, t.CommandPaletteItem{Divider: "Browse"})
	add("Filter files", "/", a.openFilter)
	add("Next file", "n", func() { a.nextFile(1) })
	add("Previous file", "p", func() { a.nextFile(-1) })
	add("Toggle all / changed files", "a", a.toggleAllFiles)
	add("Toggle file seen", "m", a.markSeen)
	add("Clear all seen marks", "M", a.clearSeen)
	items = append(items, t.CommandPaletteItem{Divider: "Layout and appearance"})
	add("Toggle sidebar", "b", a.toggleSidebar)
	add("Resize sidebar", "d", func() {
		a.sidebarVisible.Set(true)
		a.dividerActive.Set(true)
		a.palette.SetNextFocusIDOnClose("aside-split")
		t.RequestFocus("aside-split")
	})
	add("Toggle split / unified", "v", a.toggleLayout)
	add("Reset pane split", "50:50", func() { a.diffRatio.Set(.5) })
	add("Toggle line wrap", "w", a.toggleWrap)
	add("Toggle +/- symbols", "", func() { a.hideSigns.Update(func(v bool) bool { return !v }) })
	add("Cycle intraline style", "i", a.cycleIntraline)
	items = append(items, t.CommandPaletteItem{Label: "Theme", Hint: "t", Action: a.openThemes})
	return items
}
func (a *Aside) toggleAllFiles() {
	a.allFiles.Update(func(v bool) bool { return !v })
	a.rebuildFiles()
	a.bump()
}
func (a *Aside) beginThemePreview() {
	if a.themeBeforePreview == "" {
		a.themeBeforePreview = t.CurrentThemeName()
	}
}
func (a *Aside) themeItems() []t.CommandPaletteItem {
	items := []t.CommandPaletteItem{}
	group := func(label string, names []string) {
		items = append(items, t.CommandPaletteItem{Divider: label})
		for _, name := range names {
			hint := ""
			if name == a.themeBeforePreview {
				hint = "current"
			}
			items = append(items, t.CommandPaletteItem{Label: themeDisplayName(name), FilterText: name + " " + themeDisplayName(name), Hint: hint, Data: name, Action: func() { t.SetTheme(name); a.themeBeforePreview = ""; a.palette.Close() }})
		}
	}
	group("Dark themes", t.DarkThemeNames())
	group("Light themes", t.LightThemeNames())
	return items
}
func (a *Aside) previewTheme(item t.CommandPaletteItem) {
	if a.palette.CurrentLevel().Title != "Themes" {
		a.dismissPalette()
		return
	}
	if name, ok := item.Data.(string); ok {
		a.beginThemePreview()
		t.SetTheme(name)
	}
}
func (a *Aside) dismissPalette() {
	if a.themeBeforePreview != "" {
		t.SetTheme(a.themeBeforePreview)
		a.themeBeforePreview = ""
	}
}
