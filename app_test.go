package main

import (
	"fmt"
	"strings"
	"testing"

	t "github.com/darrenburns/terma"

	"github.com/stretchr/testify/require"
)

func newTestDv(provider DiffProvider, staged bool, initialStates ...DvInitialState) *Dv {
	initialState := DefaultDvInitialState()
	if len(initialStates) > 0 {
		initialState = initialStates[0]
	}
	return NewDv(provider, staged, initialState)
}

func TestDv_RefreshPreservesActiveFileWhenStillPresent(t *testing.T) {
	provider := &scriptedDiffProvider{
		repoRoot: "/tmp/repo",
		diffs: []string{
			diffForPaths("a.txt", "b.txt"),
			diffForPaths("b.txt", "c.txt"),
		},
	}

	app := newTestDv(provider, false)
	require.True(t, app.selectFilePath("b.txt"))
	require.Equal(t, "b.txt", app.activePath)
	require.False(t, app.activeIsDir)

	app.refreshDiff()

	require.Equal(t, "b.txt", app.activePath)
	require.False(t, app.activeIsDir)
	require.Equal(t, app.filePathToTreePath["b.txt"], app.treeState.CursorPath.Peek())
}

func TestDv_NextPrevCycleFilesAndSyncTreeCursor(t *testing.T) {
	provider := &scriptedDiffProvider{
		repoRoot: "/tmp/repo",
		diffs: []string{
			diffForPaths("pkg/b.go", "pkg/c.go", "a.txt"),
		},
	}

	app := newTestDv(provider, false)
	require.GreaterOrEqual(t, len(app.orderedFilePaths), 3)

	first := app.orderedFilePaths[0]
	second := app.orderedFilePaths[1]
	last := app.orderedFilePaths[len(app.orderedFilePaths)-1]

	require.Equal(t, first, app.activePath)

	app.moveFileCursor(1)
	require.Equal(t, second, app.activePath)
	require.Equal(t, app.filePathToTreePath[second], app.treeState.CursorPath.Peek())

	app.moveFileCursor(-1)
	require.Equal(t, first, app.activePath)
	require.Equal(t, app.filePathToTreePath[first], app.treeState.CursorPath.Peek())

	app.moveFileCursor(-1)
	require.Equal(t, last, app.activePath)
	require.Equal(t, app.filePathToTreePath[last], app.treeState.CursorPath.Peek())
}

func TestDv_NextPrevCycleOnlyFilteredFiles(t *testing.T) {
	provider := &scriptedDiffProvider{
		repoRoot: "/tmp/repo",
		diffs: []string{
			diffForPaths("a.go", "b.go", "c.txt"),
		},
	}

	app := newTestDv(provider, false)
	app.onTreeFilterChange(".go")
	require.False(t, app.treeFilterNoMatches)
	require.Equal(t, "a.go", app.activePath)

	app.moveFileCursor(1)
	require.Equal(t, "b.go", app.activePath)

	app.moveFileCursor(1)
	require.Equal(t, "a.go", app.activePath)

	app.moveFileCursor(-1)
	require.Equal(t, "b.go", app.activePath)
}

func TestDv_NextPrevStartsAtFilteredSetWhenActiveFileExcluded(t *testing.T) {
	provider := &scriptedDiffProvider{
		repoRoot: "/tmp/repo",
		diffs: []string{
			diffForPaths("a.go", "b.go", "c.txt"),
		},
	}

	app := newTestDv(provider, false)
	require.True(t, app.selectFilePath("c.txt"))
	require.Equal(t, "c.txt", app.activePath)

	app.onTreeFilterChange(".go")
	require.False(t, app.treeFilterNoMatches)
	require.Equal(t, "a.go", app.activePath)

	app.moveFileCursor(1)
	require.Equal(t, "b.go", app.activePath)

	app.moveFileCursor(-1)
	require.Equal(t, "a.go", app.activePath)
}

func TestDv_DirectoryCursorShowsSummaryInViewer(t *testing.T) {
	provider := &scriptedDiffProvider{
		repoRoot: "/tmp/repo",
		diffs: []string{
			diffForPaths("pkg/a.go", "pkg/b.go", "README.md"),
		},
	}

	app := newTestDv(provider, false)
	dirPath, ok := findTreePathByDataPath(app.treeState.Nodes.Peek(), "pkg")
	require.True(t, ok)

	node, ok := app.treeState.NodeAtPath(dirPath)
	require.True(t, ok)
	app.treeState.CursorPath.Set(clonePath(dirPath))
	app.onTreeCursorChange(node.Data)

	require.True(t, app.activeIsDir)
	require.Equal(t, "pkg", app.activePath)

	rendered := app.diffViewState.Rendered.Peek()
	require.NotNil(t, rendered)
	require.GreaterOrEqual(t, len(rendered.Lines), 4)
	require.True(t, strings.Contains(lineText(rendered.Lines[0]), "Section: Unstaged"))
	require.True(t, strings.Contains(lineText(rendered.Lines[1]), "Directory: pkg"))
	require.True(t, strings.Contains(lineText(rendered.Lines[2]), "Touched files: 2"))
}

func TestDv_TreeAlwaysShowsUnstagedAndStagedSections(t *testing.T) {
	app := newTestDv(&scriptedDiffProvider{
		repoRoot:      "/tmp/repo",
		unstagedDiffs: []string{diffForPaths("unstaged.go")},
		stagedDiffs:   []string{diffForPaths("staged.go")},
	}, false)

	roots := app.treeState.Nodes.Peek()
	require.Len(t, roots, 2)
	require.Equal(t, "Unstaged", roots[0].Data.Name)
	require.Equal(t, DiffTreeNodeSection, roots[0].Data.NodeKind)
	require.Equal(t, DiffSectionUnstaged, roots[0].Data.Section)
	require.Equal(t, "Staged", roots[1].Data.Name)
	require.Equal(t, DiffTreeNodeSection, roots[1].Data.NodeKind)
	require.Equal(t, DiffSectionStaged, roots[1].Data.Section)
}

func TestDv_SwitchSectionFocusSwitchesViewerSelection(t *testing.T) {
	app := newTestDv(&scriptedDiffProvider{
		repoRoot:      "/tmp/repo",
		unstagedDiffs: []string{diffForPaths("unstaged.go")},
		stagedDiffs:   []string{diffForPaths("staged.go")},
	}, false)

	require.Equal(t, DiffSectionUnstaged, app.activeSection)
	require.Equal(t, "unstaged.go", app.activePath)

	app.switchSectionFocus()
	require.Equal(t, DiffSectionStaged, app.activeSection)
	require.Equal(t, "staged.go", app.activePath)

	app.switchSectionFocus()
	require.Equal(t, DiffSectionUnstaged, app.activeSection)
	require.Equal(t, "unstaged.go", app.activePath)
}

func TestDv_SwitchSectionFocusNoopWhenTargetSectionEmpty(t *testing.T) {
	app := newTestDv(&scriptedDiffProvider{
		repoRoot: "/tmp/repo",
		diffs:    []string{diffForPaths("only-unstaged.go")},
	}, false)

	require.Equal(t, DiffSectionUnstaged, app.activeSection)
	require.Equal(t, "only-unstaged.go", app.activePath)

	app.switchSectionFocus()

	require.Equal(t, DiffSectionUnstaged, app.activeSection)
	require.Equal(t, "only-unstaged.go", app.activePath)
}

func TestDv_SamePathCanExistInBothSectionsWithDistinctSelection(t *testing.T) {
	app := newTestDv(&scriptedDiffProvider{
		repoRoot:      "/tmp/repo",
		unstagedDiffs: []string{diffForPathWithStats("same.go", 1, 0)},
		stagedDiffs:   []string{diffForPathWithStats("same.go", 0, 2)},
	}, false)

	unstagedTreePath := app.sectionState(DiffSectionUnstaged).filePathToTreePath["same.go"]
	stagedTreePath := app.sectionState(DiffSectionStaged).filePathToTreePath["same.go"]
	require.NotEqual(t, unstagedTreePath, stagedTreePath)

	require.Equal(t, DiffSectionUnstaged, app.activeSection)
	require.Equal(t, "same.go", app.activePath)
	require.Equal(t, 1, app.fileByPath["same.go"].Additions)
	require.Equal(t, 0, app.fileByPath["same.go"].Deletions)

	app.switchSectionFocus()

	require.Equal(t, DiffSectionStaged, app.activeSection)
	require.Equal(t, "same.go", app.activePath)
	require.Equal(t, 0, app.fileByPath["same.go"].Additions)
	require.Equal(t, 2, app.fileByPath["same.go"].Deletions)
}

func TestDv_PipeModeTreeShowsSingleFilesSection(t *testing.T) {
	app := newTestDv(&scriptedDiffProvider{
		repoRoot:      "/tmp/repo",
		diffs:         []string{diffForPaths("piped.go")},
		sections:      []DiffSection{DiffSectionFiles},
		manualRefresh: boolPtr(false),
	}, false)

	roots := app.treeState.Nodes.Peek()
	require.Len(t, roots, 1)
	require.Equal(t, "Files", roots[0].Data.Name)
	require.Equal(t, DiffSectionFiles, roots[0].Data.Section)
	require.Equal(t, DiffSectionFiles, app.activeSection)
	require.Equal(t, "piped.go", app.activePath)
}

func TestDv_PipeModeSectionLabelUsesAccentColor(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{
		repoRoot:      "/tmp/repo",
		diffs:         []string{diffForPaths("a.txt")},
		sections:      []DiffSection{DiffSectionFiles},
		manualRefresh: boolPtr(false),
	}, false)
	theme, ok := t.GetTheme(t.CurrentThemeName())
	require.True(tt, ok)

	render := app.renderTreeNode(theme, false)
	widget := render(
		DiffTreeNodeData{
			Name:         "Files",
			Path:         string(DiffSectionFiles),
			IsDir:        true,
			Section:      DiffSectionFiles,
			NodeKind:     DiffTreeNodeSection,
			TouchedFiles: 1,
		},
		t.TreeNodeContext{},
		t.MatchResult{},
	)
	row, ok := widget.(t.Row)
	require.True(tt, ok)
	label, ok := row.Children[0].(t.Text)
	require.True(tt, ok)
	require.Equal(tt, theme.Accent, label.Style.ForegroundColor)
}

func TestDv_PipeModeSidebarHeadingOmitsSectionSwitchHint(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{
		repoRoot:      "/tmp/repo",
		diffs:         []string{diffForPaths("a.txt")},
		sections:      []DiffSection{DiffSectionFiles},
		manualRefresh: boolPtr(false),
	}, false)
	theme, ok := t.GetTheme(t.CurrentThemeName())
	require.True(tt, ok)

	spans := app.sidebarHeadingSpans(theme)
	require.Len(tt, spans, 2)
	require.Equal(tt, "Files: ", spans[0].Text)
	require.Equal(tt, "1", spans[1].Text)
	require.Equal(tt, theme.Accent, spans[1].Style.Foreground)

	joined := strings.Join(spanTexts(spans), "")
	require.NotContains(tt, joined, "[s]")
}

func TestDv_PipeModeCommandPaletteOmitsSwitchSection(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{
		repoRoot:      "/tmp/repo",
		diffs:         []string{diffForPaths("a.txt")},
		sections:      []DiffSection{DiffSectionFiles},
		manualRefresh: boolPtr(false),
	}, false)

	level := app.commandPalette.CurrentLevel()
	require.NotNil(tt, level)

	switchSection := findPaletteItemByLabel(level.Items, "Switch section")
	require.Empty(tt, switchSection.Label)

	refresh := findPaletteItemByLabel(level.Items, "Refresh")
	require.True(tt, refresh.IsSelectable())
}

func TestDv_PipeModeSectionSummaryUsesFilesCopy(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{
		repoRoot:      "/tmp/repo",
		sections:      []DiffSection{DiffSectionFiles},
		manualRefresh: boolPtr(false),
	}, false)

	app.setActiveSectionSummary(DiffSectionFiles)

	rendered := app.diffViewState.Rendered.Peek()
	require.NotNil(tt, rendered)
	found := false
	for _, line := range rendered.Lines {
		if strings.Contains(lineText(line), "No files in this diff.") {
			found = true
			break
		}
	}
	require.True(tt, found)
}

func TestDv_PipeModeManualRefreshIsNoop(tt *testing.T) {
	provider := &scriptedDiffProvider{
		repoRoot:      "/tmp/repo",
		diffs:         []string{diffForPaths("first.txt"), diffForPaths("second.txt")},
		sections:      []DiffSection{DiffSectionFiles},
		manualRefresh: boolPtr(false),
	}
	app := newTestDv(provider, false)
	require.Equal(tt, "first.txt", app.activePath)
	require.Equal(tt, 1, provider.index)

	app.manualRefresh()

	require.Equal(tt, "first.txt", app.activePath)
	require.Equal(tt, 1, provider.index)
}

func TestDv_CommandPaletteIncludesCommonActions(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo"}, false)
	level := app.commandPalette.CurrentLevel()
	require.NotNil(tt, level)

	toggle := findPaletteItemByLabel(level.Items, "Switch section")
	require.True(tt, toggle.IsSelectable())
	require.Equal(tt, "[s]", toggle.Hint)

	refresh := findPaletteItemByLabel(level.Items, "Refresh")
	require.True(tt, refresh.IsSelectable())
	require.Equal(tt, "[r]", refresh.Hint)

	sidebar := findPaletteItemByLabel(level.Items, "Toggle sidebar")
	require.True(tt, sidebar.IsSelectable())
	require.Equal(tt, "[b]", sidebar.Hint)

	wrap := findPaletteItemByLabel(level.Items, "Toggle line wrap")
	require.True(tt, wrap.IsSelectable())
	require.Equal(tt, "[w]", wrap.Hint)

	layoutMode := findPaletteItemByLabel(level.Items, "Toggle side-by-side mode")
	require.True(tt, layoutMode.IsSelectable())
	require.Equal(tt, "[v]", layoutMode.Hint)

	signs := findPaletteItemByLabel(level.Items, "Toggle +/- symbols")
	require.True(tt, signs.IsSelectable())

	intraline := findPaletteItemByLabel(level.Items, "Toggle intraline style")
	require.True(tt, intraline.IsSelectable())
	require.Equal(tt, "[i]", intraline.Hint)

	divider := findPaletteItemByLabel(level.Items, "Focus divider")
	require.True(tt, divider.IsSelectable())
	require.Equal(tt, "[d]", divider.Hint)
}

func TestDv_CommandPaletteShowsResetSplitOnlyInSideBySideMode(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo"}, false)
	level := app.commandPalette.CurrentLevel()
	require.NotNil(tt, level)

	reset := findPaletteItemByLabel(level.Items, "Reset pane split")
	require.Empty(tt, reset.Label)

	app.toggleDiffLayoutMode()
	level = app.commandPalette.CurrentLevel()
	require.NotNil(tt, level)
	reset = findPaletteItemByLabel(level.Items, "Reset pane split")
	require.Equal(tt, "Reset pane split", reset.Label)
	require.True(tt, reset.IsSelectable())

	app.toggleDiffLayoutMode()
	level = app.commandPalette.CurrentLevel()
	require.NotNil(tt, level)
	reset = findPaletteItemByLabel(level.Items, "Reset pane split")
	require.Empty(tt, reset.Label)
}

func TestDv_CommandPaletteResetSplitActionResetsToEvenRatio(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo"}, false)
	app.toggleDiffLayoutMode()
	app.diffViewState.SetSideBySideSplitRatio(0.73)

	level := app.commandPalette.CurrentLevel()
	require.NotNil(tt, level)
	reset := findPaletteItemByLabel(level.Items, "Reset pane split")
	require.Equal(tt, "Reset pane split", reset.Label)
	require.NotNil(tt, reset.Action)

	reset.Action()
	require.InDelta(tt, 0.5, app.diffViewState.SideBySideSplitRatio(), 0.00001)
}

func TestDv_CommandPaletteUsesLayoutAndAppearanceSections(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo"}, false)
	level := app.commandPalette.CurrentLevel()
	require.NotNil(tt, level)

	layoutDivider := -1
	appearanceDivider := -1
	sidebarIdx := -1
	dividerIdx := -1
	wrapIdx := -1
	layoutModeIdx := -1
	signsIdx := -1
	intralineIdx := -1
	themeIdx := -1

	for idx, item := range level.Items {
		switch {
		case item.Divider == "Layout":
			layoutDivider = idx
		case item.Divider == "Appearance":
			appearanceDivider = idx
		case item.Label == "Toggle sidebar":
			sidebarIdx = idx
		case item.Label == "Focus divider":
			dividerIdx = idx
		case item.Label == "Toggle line wrap":
			wrapIdx = idx
		case item.Label == "Toggle side-by-side mode":
			layoutModeIdx = idx
		case item.Label == "Toggle +/- symbols":
			signsIdx = idx
		case item.Label == "Toggle intraline style":
			intralineIdx = idx
		case item.Label == "Theme":
			themeIdx = idx
		}
	}

	require.GreaterOrEqual(tt, layoutDivider, 0)
	require.GreaterOrEqual(tt, appearanceDivider, 0)
	require.Greater(tt, sidebarIdx, layoutDivider)
	require.Greater(tt, dividerIdx, layoutDivider)
	require.Greater(tt, appearanceDivider, dividerIdx)
	require.Greater(tt, wrapIdx, appearanceDivider)
	require.Greater(tt, layoutModeIdx, wrapIdx)
	require.Greater(tt, signsIdx, layoutModeIdx)
	require.Greater(tt, intralineIdx, signsIdx)
	require.Greater(tt, themeIdx, intralineIdx)
}

func TestDv_KeybindsHideCommandsExposedInPalette(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo"}, false)
	keybinds := app.Keybinds()

	require.True(tt, keybindIsHidden(keybinds, "s"))
	require.True(tt, keybindIsHidden(keybinds, "r"))
	require.True(tt, keybindIsHidden(keybinds, "d"))
	require.True(tt, keybindIsHidden(keybinds, "ctrl+h"))
	require.True(tt, keybindIsHidden(keybinds, "ctrl+l"))
	require.True(tt, keybindIsHidden(keybinds, "w"))
	require.True(tt, keybindIsHidden(keybinds, "v"))
	require.True(tt, keybindIsHidden(keybinds, "i"))
	require.True(tt, keybindIsHidden(keybinds, "b"))
	require.False(tt, keybindIsHidden(keybinds, "ctrl+p"))
	require.True(tt, keybindIsHidden(keybinds, "t"))
}

func TestDv_KeybindsIncludeSidebarToggle(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo"}, false)
	keybind, ok := findKeybindByKey(app.Keybinds(), "b")
	require.True(tt, ok)
	require.Equal(tt, "Toggle sidebar", keybind.Name)
	require.True(tt, keybind.Hidden)
}

func TestDv_KeybindsIncludeWrapToggle(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo"}, false)
	keybind, ok := findKeybindByKey(app.Keybinds(), "w")
	require.True(tt, ok)
	require.Equal(tt, "Toggle line wrap", keybind.Name)
	require.True(tt, keybind.Hidden)
}

func TestDv_KeybindsIncludeSideBySideToggle(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo"}, false)
	keybind, ok := findKeybindByKey(app.Keybinds(), "v")
	require.True(tt, ok)
	require.Equal(tt, "Toggle side-by-side", keybind.Name)
	require.True(tt, keybind.Hidden)
}

func TestDv_KeybindsIncludeIntralineStyleToggle(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo"}, false)
	keybind, ok := findKeybindByKey(app.Keybinds(), "i")
	require.True(tt, ok)
	require.Equal(tt, "Toggle intraline style", keybind.Name)
	require.True(tt, keybind.Hidden)
}

func TestDv_KeybindsIncludeSideBySideSplitShiftShortcuts(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo"}, false)

	left, ok := findKeybindByKey(app.Keybinds(), "ctrl+h")
	require.True(tt, ok)
	require.Equal(tt, "Shift split left", left.Name)
	require.True(tt, left.Hidden)

	right, ok := findKeybindByKey(app.Keybinds(), "ctrl+l")
	require.True(tt, ok)
	require.Equal(tt, "Shift split right", right.Name)
	require.True(tt, right.Hidden)

	app.diffLayoutMode = DiffLayoutSideBySide

	left, ok = findKeybindByKey(app.Keybinds(), "ctrl+h")
	require.True(tt, ok)
	require.True(tt, left.Hidden)

	right, ok = findKeybindByKey(app.Keybinds(), "ctrl+l")
	require.True(tt, ok)
	require.True(tt, right.Hidden)
}

func TestDv_KeybindsIncludeThemeMenuShortcut(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo"}, false)
	keybind, ok := findKeybindByKey(app.Keybinds(), "t")
	require.True(tt, ok)
	require.Equal(tt, "Theme menu", keybind.Name)
	require.True(tt, keybind.Hidden)
}

func TestDv_FilterFilesKeybindVisibleWhenTreeOrViewerFocused(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo", diffs: []string{diffForPaths("a.txt")}}, false)

	app.focusedWidgetID = diffViewerScrollID
	keybind, ok := findKeybindByKey(app.Keybinds(), "/")
	require.True(tt, ok)
	require.False(tt, keybind.Hidden)

	app.focusedWidgetID = diffFilesTreeID
	keybind, ok = findKeybindByKey(app.Keybinds(), "/")
	require.True(tt, ok)
	require.False(tt, keybind.Hidden)

	app.focusedWidgetID = diffFilesFilterID
	keybind, ok = findKeybindByKey(app.Keybinds(), "/")
	require.True(tt, ok)
	require.True(tt, keybind.Hidden)
}

func TestDv_ThemeMenuShortcutOpensThemesSubmenu(tt *testing.T) {
	originalTheme := t.CurrentThemeName()
	defer t.SetTheme(originalTheme)

	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo"}, false)
	app.togglePalette()
	app.commandPalette.PushLevel("Nested", []t.CommandPaletteItem{
		{Label: "Nested action", Action: func() {}},
	})

	keybind, ok := findKeybindByKey(app.Keybinds(), "t")
	require.True(tt, ok)
	require.NotNil(tt, keybind.Action)
	keybind.Action()

	require.True(tt, app.commandPalette.Visible.Peek())

	level := app.commandPalette.CurrentLevel()
	require.NotNil(tt, level)
	require.Equal(tt, diffThemesPalette, level.Title)

	currentItem, ok := app.commandPalette.CurrentItem()
	require.True(tt, ok)
	selectedTheme, ok := currentItem.Data.(string)
	require.True(tt, ok)
	require.Equal(tt, t.CurrentThemeName(), selectedTheme)

	require.True(tt, app.commandPalette.PopLevel())
	level = app.commandPalette.CurrentLevel()
	require.NotNil(tt, level)
	require.Equal(tt, "Commands", level.Title)
	require.False(tt, app.commandPalette.PopLevel())
}

func TestDv_ThemePreviewOnCursorChange(tt *testing.T) {
	originalTheme := t.CurrentThemeName()
	defer t.SetTheme(originalTheme)

	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo"}, false)
	app.commandPalette.PushLevel(diffThemesPalette, app.themeItems())
	level := app.commandPalette.CurrentLevel()
	require.NotNil(tt, level)

	// First cursor-change in Themes should sync to current theme.
	initialItem, ok := app.commandPalette.CurrentItem()
	require.True(tt, ok)
	app.handlePaletteCursorChange(initialItem)

	preview := t.CommandPaletteItem{}
	previewIdx := -1
	for idx, item := range level.Items {
		themeName, ok := item.Data.(string)
		if !ok || themeName == "" || themeName == t.CurrentThemeName() {
			continue
		}
		preview = item
		previewIdx = idx
		break
	}
	require.NotEmpty(tt, preview.Label, "expected at least one theme item different from current theme")
	require.GreaterOrEqual(tt, previewIdx, 0)

	level.ListState.SelectIndex(previewIdx)
	app.handlePaletteCursorChange(preview)

	themeName, _ := preview.Data.(string)
	require.Equal(tt, themeName, t.CurrentThemeName())
}

func TestDv_ThemesMenuSelectsCurrentThemeByDefault(tt *testing.T) {
	originalTheme := t.CurrentThemeName()
	defer t.SetTheme(originalTheme)

	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo"}, false)
	items := app.themeItems()

	selectableThemes := make([]string, 0, len(items))
	for _, item := range items {
		themeName, ok := item.Data.(string)
		if ok && themeName != "" {
			selectableThemes = append(selectableThemes, themeName)
		}
	}
	require.GreaterOrEqual(tt, len(selectableThemes), 2)

	currentTheme := selectableThemes[len(selectableThemes)-1]
	t.SetTheme(currentTheme)

	themeItems := app.themeItems()
	app.commandPalette.PushLevel(diffThemesPalette, themeItems)
	level := app.commandPalette.CurrentLevel()
	require.NotNil(tt, level)

	initialItem, ok := app.commandPalette.CurrentItem()
	require.True(tt, ok)
	app.handlePaletteCursorChange(initialItem)

	selectedItem, ok := app.commandPalette.CurrentItem()
	require.True(tt, ok)
	selectedTheme, ok := selectedItem.Data.(string)
	require.True(tt, ok)
	require.Equal(tt, currentTheme, selectedTheme)
	require.Equal(tt, currentTheme, t.CurrentThemeName())
}

func TestDv_ThemePreviewRevertsWhenLeavingThemesMenu(tt *testing.T) {
	originalTheme := t.CurrentThemeName()
	defer t.SetTheme(originalTheme)

	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo"}, false)
	themeNames := paletteThemeNames(app.themeItems())
	require.GreaterOrEqual(tt, len(themeNames), 2)

	baseTheme := themeNames[0]
	t.SetTheme(baseTheme)

	app.commandPalette.PushLevel(diffThemesPalette, app.themeItems())
	level := app.commandPalette.CurrentLevel()
	require.NotNil(tt, level)

	initialItem, ok := app.commandPalette.CurrentItem()
	require.True(tt, ok)
	app.handlePaletteCursorChange(initialItem)

	previewItem := t.CommandPaletteItem{}
	previewIdx := -1
	for idx, item := range level.Items {
		themeName, ok := item.Data.(string)
		if !ok || themeName == "" || themeName == baseTheme {
			continue
		}
		previewItem = item
		previewIdx = idx
		break
	}
	require.GreaterOrEqual(tt, previewIdx, 0)

	level.ListState.SelectIndex(previewIdx)
	app.handlePaletteCursorChange(previewItem)
	previewTheme, _ := previewItem.Data.(string)
	require.Equal(tt, previewTheme, t.CurrentThemeName())

	popped := app.commandPalette.PopLevel()
	require.True(tt, popped)
	rootItem, ok := app.commandPalette.CurrentItem()
	require.True(tt, ok)
	app.handlePaletteCursorChange(rootItem)

	require.Equal(tt, baseTheme, t.CurrentThemeName())
	require.Equal(tt, "", app.themePreviewBase)
}

func TestDv_ThemeSelectionPersistsOnEnter(tt *testing.T) {
	originalTheme := t.CurrentThemeName()
	defer t.SetTheme(originalTheme)

	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo"}, false)
	themeNames := paletteThemeNames(app.themeItems())
	require.GreaterOrEqual(tt, len(themeNames), 2)

	baseTheme := themeNames[0]
	selectedTheme := themeNames[1]
	t.SetTheme(baseTheme)

	app.commandPalette.PushLevel(diffThemesPalette, app.themeItems())
	level := app.commandPalette.CurrentLevel()
	require.NotNil(tt, level)

	initialItem, ok := app.commandPalette.CurrentItem()
	require.True(tt, ok)
	app.handlePaletteCursorChange(initialItem)

	var selectedItem t.CommandPaletteItem
	selectedIdx := -1
	for idx, item := range level.Items {
		themeName, ok := item.Data.(string)
		if ok && themeName == selectedTheme {
			selectedItem = item
			selectedIdx = idx
			break
		}
	}
	require.GreaterOrEqual(tt, selectedIdx, 0)
	require.NotNil(tt, selectedItem.Action)

	level.ListState.SelectIndex(selectedIdx)
	app.handlePaletteCursorChange(selectedItem)
	require.Equal(tt, selectedTheme, t.CurrentThemeName())

	selectedItem.Action()

	require.Equal(tt, selectedTheme, t.CurrentThemeName())
	require.Equal(tt, "", app.themePreviewBase)
}

func TestDv_OpenTreeFilterAllowsViewerFocus(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo", diffs: []string{diffForPaths("a.txt")}}, false)

	app.focusedWidgetID = diffViewerScrollID
	app.openTreeFilter()
	require.True(tt, app.treeFilterVisible)

	app.treeFilterVisible = false
	app.focusedWidgetID = diffFilesTreeID
	app.openTreeFilter()
	require.True(tt, app.treeFilterVisible)
}

func TestDv_OpenTreeFilterShowsHiddenSidebarAndKeepsItAfterDismiss(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo", diffs: []string{diffForPaths("a.txt")}}, false)
	app.sidebarVisible = false
	app.focusedWidgetID = diffViewerScrollID

	app.openTreeFilter()
	require.True(tt, app.sidebarVisible)
	require.True(tt, app.treeFilterVisible)

	app.focusedWidgetID = diffFilesFilterID
	app.handleEscape()

	require.False(tt, app.treeFilterVisible)
	require.True(tt, app.sidebarVisible)
}

func TestDv_HandleEscapeClearsActiveTreeFilter(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo", diffs: []string{diffForPaths("a.txt")}}, false)
	app.treeFilterVisible = true
	app.onTreeFilterChange("a")

	require.Equal(tt, "a", app.treeFilterState.PeekQuery())
	require.Equal(tt, "", app.treeFilterInput.GetText())

	app.treeFilterInput.SetText("a")
	app.handleEscape()

	require.Equal(tt, "", app.treeFilterState.PeekQuery())
	require.Equal(tt, "", app.treeFilterInput.GetText())
	require.False(tt, app.treeFilterVisible)
}

func TestDv_FilterNoMatchesSetsExplicitState(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo", diffs: []string{diffForPaths("a.txt", "b.txt")}}, false)

	app.onTreeFilterChange("zzz")

	require.True(tt, app.treeFilterNoMatches)
	require.Equal(tt, "", app.activePath)
	require.False(tt, app.activeIsDir)
	require.Equal(tt, "No matches", app.viewerTitle())
	require.Equal(tt, "Unstaged: 2 Staged: 0", app.sidebarSummaryLabel())

	rendered := app.diffViewState.Rendered.Peek()
	require.NotNil(tt, rendered)
	require.Equal(tt, "No matches", rendered.Title)
	require.GreaterOrEqual(tt, len(rendered.Lines), 1)
	require.Contains(tt, lineText(rendered.Lines[0]), `No files match "zzz".`)
}

func TestDv_ClearTreeFilterResetsNoMatchesState(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo", diffs: []string{diffForPaths("a.txt", "b.txt")}}, false)
	app.treeFilterInput.SetText("zzz")
	app.onTreeFilterChange("zzz")
	require.True(tt, app.treeFilterNoMatches)

	cleared := app.clearTreeFilter()

	require.True(tt, cleared)
	require.False(tt, app.treeFilterNoMatches)
	require.Equal(tt, "", app.treeFilterState.PeekQuery())
	require.Equal(tt, "", app.treeFilterInput.GetText())
	require.False(tt, app.treeFilterVisible)
	require.Equal(tt, app.orderedFilePaths[0], app.activePath)
}

func TestDv_FilterInputExposesArrowNavigationKeybinds(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo", diffs: []string{diffForPaths("a.txt", "b.txt")}}, false)
	app.treeFilterVisible = true

	filterInput := findFilterInput(tt, app)
	up, ok := findKeybindByKey(filterInput.ExtraKeybinds, "up")
	require.True(tt, ok)
	require.True(tt, up.Hidden)
	require.NotNil(tt, up.Action)

	down, ok := findKeybindByKey(filterInput.ExtraKeybinds, "down")
	require.True(tt, ok)
	require.True(tt, down.Hidden)
	require.NotNil(tt, down.Action)
}

func TestDv_FilterInputArrowKeybindsNavigateFilteredFiles(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{
		repoRoot: "/tmp/repo",
		diffs:    []string{diffForPaths("a.go", "b.go", "c.txt")},
	}, false)
	app.treeFilterVisible = true
	app.onTreeFilterChange(".go")
	require.Equal(tt, "a.go", app.activePath)

	filterInput := findFilterInput(tt, app)
	down, ok := findKeybindByKey(filterInput.ExtraKeybinds, "down")
	require.True(tt, ok)
	up, ok := findKeybindByKey(filterInput.ExtraKeybinds, "up")
	require.True(tt, ok)

	down.Action()
	require.Equal(tt, "b.go", app.activePath)

	down.Action()
	require.Equal(tt, "a.go", app.activePath)

	up.Action()
	require.Equal(tt, "b.go", app.activePath)
}

func TestDv_FilterInputArrowKeybindsKeepInputFocus(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{
		repoRoot: "/tmp/repo",
		diffs:    []string{diffForPaths("a.txt", "b.txt")},
	}, false)
	app.treeFilterVisible = true
	app.focusedWidgetID = diffFilesFilterID
	require.GreaterOrEqual(tt, len(app.orderedFilePaths), 2)
	require.Equal(tt, app.orderedFilePaths[0], app.activePath)

	filterInput := findFilterInput(tt, app)
	down, ok := findKeybindByKey(filterInput.ExtraKeybinds, "down")
	require.True(tt, ok)
	up, ok := findKeybindByKey(filterInput.ExtraKeybinds, "up")
	require.True(tt, ok)

	down.Action()
	require.Equal(tt, app.orderedFilePaths[1], app.activePath)
	require.Equal(tt, diffFilesFilterID, app.focusedWidgetID)

	up.Action()
	require.Equal(tt, app.orderedFilePaths[0], app.activePath)
	require.Equal(tt, diffFilesFilterID, app.focusedWidgetID)
}

func TestDv_RenderTreeNodeHighlightsMatchWithDefaultStyle(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo", diffs: []string{diffForPaths("server.go")}}, false)
	theme, ok := t.GetTheme(t.CurrentThemeName())
	require.True(tt, ok)

	render := app.renderTreeNode(theme, false)
	rowWidget := render(
		DiffTreeNodeData{Name: "server.go", Path: "server.go", Additions: 1, Deletions: 1},
		t.TreeNodeContext{},
		t.MatchResult{
			Matched: true,
			Ranges:  []t.MatchRange{{Start: 0, End: len("server")}},
		},
	)

	row, ok := rowWidget.(t.Row)
	require.True(tt, ok)
	require.NotEmpty(tt, row.Children)

	label, ok := row.Children[0].(t.Text)
	require.True(tt, ok)
	require.NotEmpty(tt, label.Spans)

	highlight := t.MatchHighlightStyle(theme)
	found := false
	for _, span := range label.Spans {
		if span.Style == highlight {
			found = true
			break
		}
	}
	require.True(tt, found, "expected at least one highlighted span")
}

func TestDv_RenderTreeNodeOmitsZeroStats(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo", diffs: []string{diffForPaths("server.go")}}, false)
	theme, ok := t.GetTheme(t.CurrentThemeName())
	require.True(tt, ok)

	render := app.renderTreeNode(theme, false)

	addOnlyWidget := render(
		DiffTreeNodeData{Name: "added.go", Path: "added.go", Additions: 2, Deletions: 0},
		t.TreeNodeContext{},
		t.MatchResult{},
	)
	addOnlyRow, ok := addOnlyWidget.(t.Row)
	require.True(tt, ok)
	addOnlyText := strings.Join(rowTextContents(addOnlyRow), "|")
	require.Contains(tt, addOnlyText, "+2")
	require.NotContains(tt, addOnlyText, "-0")

	delOnlyWidget := render(
		DiffTreeNodeData{Name: "removed.go", Path: "removed.go", Additions: 0, Deletions: 3},
		t.TreeNodeContext{},
		t.MatchResult{},
	)
	delOnlyRow, ok := delOnlyWidget.(t.Row)
	require.True(tt, ok)
	delOnlyText := strings.Join(rowTextContents(delOnlyRow), "|")
	require.Contains(tt, delOnlyText, "-3")
	require.NotContains(tt, delOnlyText, "+0")
}

func TestDv_RenderTreeNodeSectionIgnoresFilterHighlightAndDimming(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo", diffs: []string{diffForPaths("a.txt")}}, false)
	theme, ok := t.GetTheme(t.CurrentThemeName())
	require.True(tt, ok)

	render := app.renderTreeNode(theme, false)
	rowWidget := render(
		DiffTreeNodeData{
			Name:         "Unstaged",
			Path:         "unstaged",
			IsDir:        true,
			Section:      DiffSectionUnstaged,
			NodeKind:     DiffTreeNodeSection,
			TouchedFiles: 2,
		},
		t.TreeNodeContext{
			FilteredAncestor: true,
		},
		t.MatchResult{
			Matched: true,
			Ranges:  []t.MatchRange{{Start: 0, End: 3}},
		},
	)

	row, ok := rowWidget.(t.Row)
	require.True(tt, ok)
	require.NotEmpty(tt, row.Children)

	label, ok := row.Children[0].(t.Text)
	require.True(tt, ok)
	require.Equal(tt, "Unstaged (2)", label.Content)
	require.Empty(tt, label.Spans)
	require.Equal(tt, theme.Error, label.Style.ForegroundColor)
	require.True(tt, label.Style.Bold)
}

func TestDv_LeftPaneTreeHasOneCellLeftPadding(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo", diffs: []string{diffForPaths("a.txt")}}, false)
	theme, ok := t.GetTheme(t.CurrentThemeName())
	require.True(tt, ok)

	ctx := t.NewBuildContext(nil, t.AnySignal[t.Focusable]{}, t.AnySignal[t.Widget]{}, nil)
	widget := app.buildLeftPane(ctx, theme)

	column, ok := widget.(t.Column)
	require.True(tt, ok)

	foundScrollable := false
	for _, child := range column.Children {
		scrollable, isScrollable := child.(t.Scrollable)
		if !isScrollable {
			continue
		}
		foundScrollable = true
		treeWidget, isTree := scrollable.Child.(SplitFriendlyTree)
		require.True(tt, isTree)
		require.Equal(tt, 1, treeWidget.Tree.Style.Padding.Left)
		break
	}
	require.True(tt, foundScrollable)
}

func TestDv_LeftPaneHeaderRightAlignsTotals(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo", diffs: []string{diffForPaths("a.txt", "b.txt")}}, false)
	theme, ok := t.GetTheme(t.CurrentThemeName())
	require.True(tt, ok)

	ctx := t.NewBuildContext(nil, t.AnySignal[t.Focusable]{}, t.AnySignal[t.Widget]{}, nil)
	widget := app.buildLeftPane(ctx, theme)

	column, ok := widget.(t.Column)
	require.True(tt, ok)
	require.NotEmpty(tt, column.Children)

	header, ok := column.Children[0].(t.Row)
	require.True(tt, ok)
	require.Len(tt, header.Children, 3)

	left, ok := header.Children[0].(t.Text)
	require.True(tt, ok)
	require.NotEmpty(tt, left.Spans)

	spacer, ok := header.Children[1].(t.Spacer)
	require.True(tt, ok)
	require.True(tt, spacer.Width.IsFlex())

	right, ok := header.Children[2].(t.Text)
	require.True(tt, ok)
	require.Len(tt, right.Spans, 3)
	require.Equal(tt, "+2", right.Spans[0].Text)
	require.Equal(tt, "-2", right.Spans[2].Text)
}

func TestDv_SidebarSummaryLabelIncludesBothSections(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{
		repoRoot:      "/tmp/repo",
		unstagedDiffs: []string{diffForPaths("a.txt")},
		stagedDiffs:   []string{diffForPaths("b.txt", "c.txt")},
	}, false)
	require.Equal(tt, "Unstaged: 1 Staged: 2", app.sidebarSummaryLabel())

	app.toggleMode()
	require.Equal(tt, "Unstaged: 1 Staged: 2", app.sidebarSummaryLabel())
}

func TestDv_SidebarHeadingIncludesStagedHint(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{
		repoRoot:      "/tmp/repo",
		unstagedDiffs: []string{diffForPaths("a.txt")},
		stagedDiffs:   []string{diffForPaths("b.txt")},
	}, false)
	theme, ok := t.GetTheme(t.CurrentThemeName())
	require.True(tt, ok)

	spans := app.sidebarHeadingSpans(theme)
	require.Len(tt, spans, 7)
	require.Equal(tt, "Unstaged: ", spans[0].Text)
	require.Equal(tt, "1", spans[1].Text)
	require.True(tt, spans[1].Style.Bold)
	require.Equal(tt, theme.Error, spans[1].Style.Foreground)
	require.Equal(tt, "Staged: ", spans[3].Text)
	require.Equal(tt, "1", spans[4].Text)
	require.True(tt, spans[4].Style.Bold)
	require.Equal(tt, theme.Success, spans[4].Style.Foreground)
	require.Equal(tt, "[s]", spans[6].Text)
	require.True(tt, spans[6].Style.Faint)
	require.Equal(tt, theme.TextMuted, spans[6].Style.Foreground)
}

func TestDv_SidebarTotalsSpansAggregatesAllFiles(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo", diffs: []string{diffForPaths("a.txt", "b.txt")}}, false)
	theme, ok := t.GetTheme(t.CurrentThemeName())
	require.True(tt, ok)

	spans := app.sidebarTotalsSpans(theme)
	require.Len(tt, spans, 3)
	require.Equal(tt, "+2", spans[0].Text)
	require.True(tt, spans[0].Style.Bold)
	require.Equal(tt, theme.Success, spans[0].Style.Foreground)
	require.Equal(tt, "-2", spans[2].Text)
	require.True(tt, spans[2].Style.Bold)
	require.Equal(tt, theme.Error, spans[2].Style.Foreground)

	app.onTreeFilterChange("zzz")
	require.True(tt, app.treeFilterNoMatches)

	spans = app.sidebarTotalsSpans(theme)
	require.Len(tt, spans, 3)
	require.Equal(tt, "+2", spans[0].Text)
	require.Equal(tt, "-2", spans[2].Text)
}

func TestDv_SidebarTotalsSpansOmitsZeroValues(tt *testing.T) {
	theme, ok := t.GetTheme(t.CurrentThemeName())
	require.True(tt, ok)

	addOnlyApp := newTestDv(&scriptedDiffProvider{
		repoRoot: "/tmp/repo",
		diffs:    []string{diffForPathWithStats("added.go", 4, 0)},
	}, false)
	addOnlySpans := addOnlyApp.sidebarTotalsSpans(theme)
	require.Len(tt, addOnlySpans, 1)
	require.Equal(tt, "+4", addOnlySpans[0].Text)
	require.Equal(tt, theme.Success, addOnlySpans[0].Style.Foreground)

	delOnlyApp := newTestDv(&scriptedDiffProvider{
		repoRoot: "/tmp/repo",
		diffs:    []string{diffForPathWithStats("removed.go", 0, 5)},
	}, false)
	delOnlySpans := delOnlyApp.sidebarTotalsSpans(theme)
	require.Len(tt, delOnlySpans, 1)
	require.Equal(tt, "-5", delOnlySpans[0].Text)
	require.Equal(tt, theme.Error, delOnlySpans[0].Style.Foreground)
}

func TestDv_ViewerTitleIncludesLineStats(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo", diffs: []string{diffForPaths("a.txt")}}, false)
	theme, ok := t.GetTheme(t.CurrentThemeName())
	require.True(tt, ok)

	require.Equal(tt, "a.txt", app.viewerTitle())

	widget := app.buildViewerTitle(theme)
	row, ok := widget.(t.Row)
	require.True(tt, ok)
	require.Len(tt, row.Children, 3)

	titleText, ok := row.Children[0].(t.Text)
	require.True(tt, ok)
	require.Len(tt, titleText.Spans, 5)
	require.Equal(tt, "a.txt", titleText.Spans[0].Text)
	require.True(tt, titleText.Spans[0].Style.Bold)
	require.Equal(tt, "+1", titleText.Spans[2].Text)
	require.True(tt, titleText.Spans[2].Style.Bold)
	require.Equal(tt, theme.Success, titleText.Spans[2].Style.Foreground)
	require.Equal(tt, "-1", titleText.Spans[4].Text)
	require.True(tt, titleText.Spans[4].Style.Bold)
	require.Equal(tt, theme.Error, titleText.Spans[4].Style.Foreground)

	_, ok = row.Children[1].(t.Spacer)
	require.True(tt, ok)

	positionText, ok := row.Children[2].(t.Text)
	require.True(tt, ok)
	require.Equal(tt, "1/1", positionText.Content)
}

func TestDv_ViewerTitleOmitsZeroStats(tt *testing.T) {
	theme, ok := t.GetTheme(t.CurrentThemeName())
	require.True(tt, ok)

	addOnlyApp := newTestDv(&scriptedDiffProvider{
		repoRoot: "/tmp/repo",
		diffs:    []string{diffForPathWithStats("added.go", 3, 0)},
	}, false)
	addOnlyWidget := addOnlyApp.buildViewerTitle(theme)
	addOnlyRow, ok := addOnlyWidget.(t.Row)
	require.True(tt, ok)
	addOnlyTitle, ok := addOnlyRow.Children[0].(t.Text)
	require.True(tt, ok)
	addOnlySpanTexts := spanTexts(addOnlyTitle.Spans)
	require.Equal(tt, []string{"added.go", " ", "+3"}, addOnlySpanTexts)
	require.NotContains(tt, strings.Join(addOnlySpanTexts, ""), "-0")
	addOnlyPosition, ok := addOnlyRow.Children[2].(t.Text)
	require.True(tt, ok)
	require.Equal(tt, "1/1", addOnlyPosition.Content)

	delOnlyApp := newTestDv(&scriptedDiffProvider{
		repoRoot: "/tmp/repo",
		diffs:    []string{diffForPathWithStats("removed.go", 0, 2)},
	}, false)
	delOnlyWidget := delOnlyApp.buildViewerTitle(theme)
	delOnlyRow, ok := delOnlyWidget.(t.Row)
	require.True(tt, ok)
	delOnlyTitle, ok := delOnlyRow.Children[0].(t.Text)
	require.True(tt, ok)
	delOnlySpanTexts := spanTexts(delOnlyTitle.Spans)
	require.Equal(tt, []string{"removed.go", " ", "-2"}, delOnlySpanTexts)
	require.NotContains(tt, strings.Join(delOnlySpanTexts, ""), "+0")
	delOnlyPosition, ok := delOnlyRow.Children[2].(t.Text)
	require.True(tt, ok)
	require.Equal(tt, "1/1", delOnlyPosition.Content)
}

func TestDv_ViewerTitleShowsFilePositionBySectionOrder(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{
		repoRoot: "/tmp/repo",
		diffs:    []string{diffForPaths("a.txt", "b.txt", "c.txt")},
	}, false)
	theme, ok := t.GetTheme(t.CurrentThemeName())
	require.True(tt, ok)

	require.True(tt, app.selectFilePath("b.txt"))
	widget := app.buildViewerTitle(theme)
	row, ok := widget.(t.Row)
	require.True(tt, ok)
	positionText, ok := row.Children[2].(t.Text)
	require.True(tt, ok)
	require.Equal(tt, "2/3", positionText.Content)
}

func TestDv_ViewerTitleFilePositionIsSectionScoped(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{
		repoRoot:      "/tmp/repo",
		unstagedDiffs: []string{diffForPaths("unstaged.txt")},
		stagedDiffs:   []string{diffForPaths("a-staged.txt", "b-staged.txt")},
	}, false)
	theme, ok := t.GetTheme(t.CurrentThemeName())
	require.True(tt, ok)

	app.switchSectionFocus()
	widget := app.buildViewerTitle(theme)
	row, ok := widget.(t.Row)
	require.True(tt, ok)
	positionText, ok := row.Children[2].(t.Text)
	require.True(tt, ok)
	require.Equal(tt, "1/2", positionText.Content)
	require.NotEqual(tt, "1/3", positionText.Content)
}

func TestDv_ViewerTitleFilePositionUsesUnfilteredSectionCount(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{
		repoRoot: "/tmp/repo",
		diffs:    []string{diffForPaths("a.go", "b.go", "c.txt")},
	}, false)
	theme, ok := t.GetTheme(t.CurrentThemeName())
	require.True(tt, ok)

	app.onTreeFilterChange(".go")
	require.Equal(tt, "a.go", app.activePath)

	widget := app.buildViewerTitle(theme)
	row, ok := widget.(t.Row)
	require.True(tt, ok)
	positionText, ok := row.Children[2].(t.Text)
	require.True(tt, ok)
	require.Equal(tt, "1/3", positionText.Content)
}

func TestDv_ViewerTitleNonFileStateHasNoFilePosition(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{
		repoRoot: "/tmp/repo",
		diffs:    []string{diffForPaths("a.txt")},
	}, false)
	theme, ok := t.GetTheme(t.CurrentThemeName())
	require.True(tt, ok)

	roots := app.treeState.Nodes.Peek()
	require.Len(tt, roots, 2)
	app.onTreeCursorChange(roots[0].Data)

	widget := app.buildViewerTitle(theme)
	text, ok := widget.(t.Text)
	require.True(tt, ok)
	require.Equal(tt, "Unstaged changes", text.Content)
}

func TestDv_RightPaneUsesPaddedEmptyStateWhenNoDiffs(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo"}, false)
	theme, ok := t.GetTheme(t.CurrentThemeName())
	require.True(tt, ok)

	widget := app.buildRightPane(theme)
	column, ok := widget.(t.Column)
	require.True(tt, ok)
	require.Len(tt, column.Children, 2)

	scrollable, ok := column.Children[1].(t.Scrollable)
	require.True(tt, ok)

	emptyState, ok := scrollable.Child.(t.Column)
	require.True(tt, ok, "expected a padded empty-state widget when no diff files exist")
	require.Equal(tt, 1, emptyState.Style.Padding.Top)
	require.Equal(tt, 2, emptyState.Style.Padding.Left)
	require.Equal(tt, 2, emptyState.Style.Padding.Right)
	require.Len(tt, emptyState.Children, 3)

	heading, ok := emptyState.Children[0].(t.Text)
	require.True(tt, ok)
	require.Equal(tt, "No staged or unstaged changes.", heading.Content)
	require.True(tt, heading.Style.Bold)

	details, ok := emptyState.Children[2].(t.Text)
	require.True(tt, ok)
	require.Equal(tt, "Make edits or stage files, then press r to refresh.", details.Content)

	app.toggleMode()
	widget = app.buildRightPane(theme)
	column, ok = widget.(t.Column)
	require.True(tt, ok)
	scrollable, ok = column.Children[1].(t.Scrollable)
	require.True(tt, ok)
	emptyState, ok = scrollable.Child.(t.Column)
	require.True(tt, ok)

	heading, ok = emptyState.Children[0].(t.Text)
	require.True(tt, ok)
	require.Equal(tt, "No staged or unstaged changes.", heading.Content)
}

func TestDv_PipeModeEmptyStateDoesNotMentionRefreshKey(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{
		repoRoot:      "/tmp/repo",
		sections:      []DiffSection{DiffSectionFiles},
		manualRefresh: boolPtr(false),
	}, false)
	theme, ok := t.GetTheme(t.CurrentThemeName())
	require.True(tt, ok)

	widget := app.buildRightPane(theme)
	column, ok := widget.(t.Column)
	require.True(tt, ok)
	scrollable, ok := column.Children[1].(t.Scrollable)
	require.True(tt, ok)
	emptyState, ok := scrollable.Child.(t.Column)
	require.True(tt, ok)
	require.Len(tt, emptyState.Children, 3)

	heading, ok := emptyState.Children[0].(t.Text)
	require.True(tt, ok)
	require.Equal(tt, "No files in piped diff.", heading.Content)

	details, ok := emptyState.Children[2].(t.Text)
	require.True(tt, ok)
	require.NotContains(tt, details.Content, "press r")
	require.NotContains(tt, details.Content, "Press r")
}

func TestDv_ToggleSidebarVisibility(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo", diffs: []string{diffForPaths("a.txt")}}, false)
	require.True(tt, app.sidebarVisible)

	app.toggleSidebar()
	require.False(tt, app.sidebarVisible)

	app.toggleSidebar()
	require.True(tt, app.sidebarVisible)
}

func TestDv_ToggleDiffWrap(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo", diffs: []string{diffForPaths("a.txt")}}, false)
	require.False(tt, app.diffHardWrap)

	app.toggleDiffWrap()
	require.True(tt, app.diffHardWrap)

	app.toggleDiffWrap()
	require.False(tt, app.diffHardWrap)
}

func TestDv_ToggleDiffLayoutModePreservesSelection(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo", diffs: []string{diffForPaths("a.txt", "b.txt")}}, false)
	require.True(tt, app.selectFilePath("b.txt"))
	require.Equal(tt, DiffLayoutUnified, app.diffLayoutMode)

	activePath := app.activePath
	activeIsDir := app.activeIsDir
	cursorPath := clonePath(app.treeState.CursorPath.Peek())

	app.toggleDiffLayoutMode()
	require.Equal(tt, DiffLayoutSideBySide, app.diffLayoutMode)
	require.Equal(tt, activePath, app.activePath)
	require.Equal(tt, activeIsDir, app.activeIsDir)
	require.Equal(tt, cursorPath, app.treeState.CursorPath.Peek())

	app.toggleDiffLayoutMode()
	require.Equal(tt, DiffLayoutUnified, app.diffLayoutMode)
	require.Equal(tt, activePath, app.activePath)
	require.Equal(tt, activeIsDir, app.activeIsDir)
	require.Equal(tt, cursorPath, app.treeState.CursorPath.Peek())
}

func TestDv_ToggleDiffLayoutModeMapsVerticalScrollBetweenLayouts(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo", diffs: []string{diffForPaths("a.txt")}}, false)
	require.Equal(tt, DiffLayoutUnified, app.diffLayoutMode)

	// Unified rows for diffForPaths: hunk header, removed line, added line.
	app.diffScrollState.SetOffset(2)
	app.diffViewState.ScrollY.Set(2)

	app.toggleDiffLayoutMode()
	require.Equal(tt, DiffLayoutSideBySide, app.diffLayoutMode)
	// Side-by-side rows collapse remove+add into one paired row.
	require.Equal(tt, 1, app.diffViewState.ScrollY.Peek())
}

func TestDv_ToggleDiffLayoutModeRoundTripRestoresExactVerticalScroll(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo", diffs: []string{diffForPaths("a.txt")}}, false)
	require.Equal(tt, DiffLayoutUnified, app.diffLayoutMode)

	// Start on the removed line row in unified mode.
	app.diffScrollState.SetOffset(1)
	app.diffViewState.ScrollY.Set(1)

	app.toggleDiffLayoutMode()
	require.Equal(tt, DiffLayoutSideBySide, app.diffLayoutMode)
	require.Equal(tt, 1, app.diffViewState.ScrollY.Peek())

	// Without scrolling in-between, toggling back should return to the exact same row.
	app.toggleDiffLayoutMode()
	require.Equal(tt, DiffLayoutUnified, app.diffLayoutMode)
	require.Equal(tt, 1, app.diffViewState.ScrollY.Peek())
}

func TestDv_DiffScrollStateHorizontalCallbacksMoveAndClamp(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo"}, false)

	rendered := buildTestRenderedFile(20, 120)
	app.diffViewState.SetRendered(rendered)
	gutterWidth := renderedGutterWidth(rendered, app.diffHideChangeSigns)
	app.diffViewState.SetViewport(40, 10, gutterWidth)
	require.NotNil(tt, app.diffScrollState.OnScrollRight)
	require.NotNil(tt, app.diffScrollState.OnScrollLeft)

	handled := app.diffScrollState.ScrollRight(5)
	require.True(tt, handled)
	require.Equal(tt, 5, app.diffViewState.ScrollX.Peek())

	handled = app.diffScrollState.ScrollRight(1000)
	require.True(tt, handled)
	require.Equal(tt, app.diffViewState.MaxScrollX(gutterWidth), app.diffViewState.ScrollX.Peek())

	handled = app.diffScrollState.ScrollLeft(1000)
	require.True(tt, handled)
	require.Equal(tt, 0, app.diffViewState.ScrollX.Peek())
}

func TestDv_DiffScrollStateHorizontalCallbacksNoopWhenWrapped(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo"}, false)

	rendered := buildTestRenderedFile(20, 120)
	app.diffViewState.SetRendered(rendered)
	app.diffViewState.ScrollX.Set(9)

	app.diffHardWrap = true
	handled := app.diffScrollState.ScrollRight(1)
	require.False(tt, handled)
	require.Equal(tt, 9, app.diffViewState.ScrollX.Peek())
}

func TestDv_DiffScrollStateHorizontalCallbacksMoveAndClampInSideBySideMode(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo"}, false)
	app.diffLayoutMode = DiffLayoutSideBySide

	rendered := buildTestRenderedFile(20, 120)
	side := &SideBySideRenderedFile{
		Title:                "test",
		Rows:                 []SideBySideRenderedRow{{Left: &RenderedSideCell{Kind: RenderedLineContext, LineNumber: 1, Prefix: " ", ContentWidth: 120}, Right: &RenderedSideCell{Kind: RenderedLineContext, LineNumber: 1, Prefix: " ", ContentWidth: 96}}},
		LeftNumWidth:         2,
		RightNumWidth:        2,
		LeftMaxContentWidth:  120,
		RightMaxContentWidth: 96,
	}
	app.diffViewState.SetRenderedPair(rendered, side)
	gutterWidth := sideBySideStateGutterWidth(
		rendered,
		side,
		app.diffHideChangeSigns,
		60,
		app.diffViewState.SideBySideSplitRatio(),
	)
	app.diffViewState.SetViewport(60, 10, gutterWidth)

	require.NotNil(tt, app.diffScrollState.OnScrollRight)
	require.NotNil(tt, app.diffScrollState.OnScrollLeft)

	handled := app.diffScrollState.ScrollRight(7)
	require.True(tt, handled)
	require.Equal(tt, 7, app.diffViewState.ScrollX.Peek())

	handled = app.diffScrollState.ScrollRight(1000)
	require.True(tt, handled)
	require.Equal(tt, sideBySideMaxScrollX(side, app.diffHideChangeSigns, 60, app.diffViewState.SideBySideSplitRatio()), app.diffViewState.ScrollX.Peek())

	handled = app.diffScrollState.ScrollLeft(1000)
	require.True(tt, handled)
	require.Equal(tt, 0, app.diffViewState.ScrollX.Peek())
}

func TestDv_DiffScrollStateHorizontalCallbacksNoopWhenWrappedInSideBySideMode(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo"}, false)
	app.diffLayoutMode = DiffLayoutSideBySide

	rendered := buildTestRenderedFile(20, 120)
	side := &SideBySideRenderedFile{
		Title:                "test",
		Rows:                 []SideBySideRenderedRow{{Left: &RenderedSideCell{Kind: RenderedLineContext, LineNumber: 1, Prefix: " ", ContentWidth: 120}, Right: &RenderedSideCell{Kind: RenderedLineContext, LineNumber: 1, Prefix: " ", ContentWidth: 96}}},
		LeftNumWidth:         2,
		RightNumWidth:        2,
		LeftMaxContentWidth:  120,
		RightMaxContentWidth: 96,
	}
	app.diffViewState.SetRenderedPair(rendered, side)
	app.diffViewState.ScrollX.Set(9)

	app.diffHardWrap = true
	handled := app.diffScrollState.ScrollRight(1)
	require.False(tt, handled)
	require.Equal(tt, 9, app.diffViewState.ScrollX.Peek())
}

func TestDv_ShiftSideBySideSplitActionsMoveDividerByOneCell(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo"}, false)
	app.diffLayoutMode = DiffLayoutSideBySide

	rendered := buildTestRenderedFile(20, 120)
	side := &SideBySideRenderedFile{
		Title:                "test",
		Rows:                 []SideBySideRenderedRow{{Left: &RenderedSideCell{Kind: RenderedLineContext, LineNumber: 1, Prefix: " ", ContentWidth: 120}, Right: &RenderedSideCell{Kind: RenderedLineContext, LineNumber: 1, Prefix: " ", ContentWidth: 96}}},
		LeftNumWidth:         2,
		RightNumWidth:        2,
		LeftMaxContentWidth:  120,
		RightMaxContentWidth: 96,
	}
	app.diffViewState.SetRenderedPair(rendered, side)
	gutterWidth := sideBySideStateGutterWidth(
		rendered,
		side,
		app.diffHideChangeSigns,
		60,
		app.diffViewState.SideBySideSplitRatio(),
	)
	app.diffViewState.SetViewport(60, 10, gutterWidth)

	before := sideBySidePaneLayout(60, side, app.diffHideChangeSigns, app.diffViewState.SideBySideSplitRatio())
	right, ok := findKeybindByKey(app.Keybinds(), "ctrl+l")
	require.True(tt, ok)
	require.NotNil(tt, right.Action)
	right.Action()
	require.True(tt, app.diffViewState.SideDividerOverlayVisible())

	afterRight := sideBySidePaneLayout(60, side, app.diffHideChangeSigns, app.diffViewState.SideBySideSplitRatio())
	require.Equal(tt, before.DividerX+1, afterRight.DividerX)

	left, ok := findKeybindByKey(app.Keybinds(), "ctrl+h")
	require.True(tt, ok)
	require.NotNil(tt, left.Action)
	left.Action()
	require.True(tt, app.diffViewState.SideDividerOverlayVisible())

	afterLeft := sideBySidePaneLayout(60, side, app.diffHideChangeSigns, app.diffViewState.SideBySideSplitRatio())
	require.Equal(tt, before.DividerX, afterLeft.DividerX)
}

func TestDv_ShiftSideBySideSplitActionsNoopOutsideSideBySide(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo"}, false)
	app.diffViewState.SetSideBySideSplitRatio(0.71)

	right, ok := findKeybindByKey(app.Keybinds(), "ctrl+l")
	require.True(tt, ok)
	require.NotNil(tt, right.Action)
	right.Action()

	require.InDelta(tt, 0.71, app.diffViewState.SideBySideSplitRatio(), 0.00001)
	require.False(tt, app.diffViewState.SideDividerOverlayVisible())
}

func TestDv_ToggleDiffChangeSigns(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo", diffs: []string{diffForPaths("a.txt")}}, false)
	require.True(tt, app.diffHideChangeSigns)

	app.toggleDiffChangeSigns()
	require.False(tt, app.diffHideChangeSigns)

	app.toggleDiffChangeSigns()
	require.True(tt, app.diffHideChangeSigns)
}

func TestDv_NewDvAppliesProvidedDefaults(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{
		repoRoot: "/tmp/repo",
		diffs:    []string{diffForPaths("a.txt")},
	}, false, DvInitialState{
		LayoutMode:      DiffLayoutSideBySide,
		SidebarVisible:  false,
		ThemeName:       t.ThemeNameDracula,
		IntralineStyle:  IntralineStyleModeUnderline,
		ShowChangeSigns: true,
	})

	require.Equal(tt, DiffLayoutSideBySide, app.diffLayoutMode)
	require.False(tt, app.sidebarVisible)
	require.Equal(tt, IntralineStyleModeUnderline, app.diffIntralineStyle)
	require.False(tt, app.diffHideChangeSigns)
	require.Equal(tt, t.ThemeNameDracula, t.CurrentThemeName())
}

func TestDv_NewDvNormalizesInvalidValues(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{
		repoRoot: "/tmp/repo",
		diffs:    []string{diffForPaths("a.txt")},
	}, false, DvInitialState{
		LayoutMode:      DiffLayoutMode(123),
		SidebarVisible:  false,
		ThemeName:       "nope",
		IntralineStyle:  IntralineStyleMode(456),
		ShowChangeSigns: false,
	})

	require.Equal(tt, DiffLayoutUnified, app.diffLayoutMode)
	require.False(tt, app.sidebarVisible)
	require.Equal(tt, IntralineStyleModeBackground, app.diffIntralineStyle)
	require.True(tt, app.diffHideChangeSigns)
	require.Equal(tt, t.ThemeNameCatppuccin, t.CurrentThemeName())
}

func TestDv_DefaultIntralineStyleModeIsBackground(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo", diffs: []string{diffForPaths("a.txt")}}, false)
	require.Equal(tt, IntralineStyleModeBackground, app.diffIntralineStyle)

	theme, ok := t.GetTheme(t.CurrentThemeName())
	require.True(tt, ok)
	widget := app.buildRightPane(theme)
	column, ok := widget.(t.Column)
	require.True(tt, ok)
	scrollable, ok := column.Children[1].(t.Scrollable)
	require.True(tt, ok)
	view, ok := scrollable.Child.(DiffView)
	require.True(tt, ok)
	require.Equal(tt, IntralineStyleModeBackground, view.IntralineStyle)
}

func TestDv_ToggleDiffIntralineStyle(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo", diffs: []string{diffForPaths("a.txt")}}, false)
	require.Equal(tt, IntralineStyleModeBackground, app.diffIntralineStyle)

	app.toggleDiffIntralineStyle()
	require.Equal(tt, IntralineStyleModeUnderline, app.diffIntralineStyle)

	app.toggleDiffIntralineStyle()
	require.Equal(tt, IntralineStyleModeBackground, app.diffIntralineStyle)
}

func TestDv_CommandPaletteIntralineStyleActionTogglesMode(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo", diffs: []string{diffForPaths("a.txt")}}, false)
	level := app.commandPalette.CurrentLevel()
	require.NotNil(tt, level)

	item := findPaletteItemByLabel(level.Items, "Toggle intraline style")
	require.True(tt, item.IsSelectable())
	require.NotNil(tt, item.Action)
	require.Equal(tt, IntralineStyleModeBackground, app.diffIntralineStyle)

	item.Action()
	require.Equal(tt, IntralineStyleModeUnderline, app.diffIntralineStyle)
}

func TestDv_FocusDividerNoopWhenSidebarHidden(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{repoRoot: "/tmp/repo"}, false)
	app.sidebarVisible = false
	app.dividerFocusRequested = false

	app.focusDivider()
	require.False(tt, app.dividerFocusRequested)

	app.focusDividerFromPalette()
	require.False(tt, app.dividerFocusRequested)
}

func TestDv_RefreshLoadsCurrentBranch(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{
		repoRoot: "/tmp/repo",
		branch:   "feature/header-branch",
		diffs:    []string{diffForPaths("a.txt")},
	}, false)

	require.Equal(tt, "feature/header-branch", app.branch)
}

func TestDv_HeaderShowsLayoutModeAndToggleHint(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{
		repoRoot: "/tmp/repo",
		branch:   "feature/layout-mode",
		diffs:    []string{diffForPaths("a.txt")},
	}, false)
	theme, ok := t.GetTheme(t.CurrentThemeName())
	require.True(tt, ok)

	header := app.buildHeader(theme)
	row, ok := header.(t.Row)
	require.True(tt, ok)
	texts := rowTextContents(row)
	text := strings.Join(texts, " ")
	require.NotContains(tt, text, "Mode:")
	require.Contains(tt, text, "[t]")
	require.Contains(tt, text, "unified [v]")
	branchIdx := indexOfTextContaining(texts, "feature/layout-mode")
	themeIdx := indexOfTextContaining(texts, "[t]")
	modeIdx := indexOfTextContaining(texts, "unified [v]")
	require.GreaterOrEqual(tt, branchIdx, 0)
	require.GreaterOrEqual(tt, themeIdx, 0)
	require.Greater(tt, modeIdx, branchIdx)
	require.Greater(tt, themeIdx, modeIdx)

	app.toggleDiffLayoutMode()
	header = app.buildHeader(theme)
	row, ok = header.(t.Row)
	require.True(tt, ok)
	texts = rowTextContents(row)
	text = strings.Join(texts, " ")
	require.Contains(tt, text, "side-by-side [v]")
	branchIdx = indexOfTextContaining(texts, "feature/layout-mode")
	themeIdx = indexOfTextContaining(texts, "[t]")
	modeIdx = indexOfTextContaining(texts, "side-by-side [v]")
	require.GreaterOrEqual(tt, branchIdx, 0)
	require.GreaterOrEqual(tt, themeIdx, 0)
	require.Greater(tt, modeIdx, branchIdx)
	require.Greater(tt, themeIdx, modeIdx)
}

func TestDv_ViewerTitleDoesNotIncludeLayoutMode(tt *testing.T) {
	app := newTestDv(&scriptedDiffProvider{
		repoRoot: "/tmp/repo",
		diffs:    []string{diffForPaths("a.txt")},
	}, false)
	theme, ok := t.GetTheme(t.CurrentThemeName())
	require.True(tt, ok)

	app.diffLayoutMode = DiffLayoutSideBySide
	widget := app.buildViewerTitle(theme)
	row, ok := widget.(t.Row)
	require.True(tt, ok)
	joined := strings.Join(rowTextContents(row), "")
	require.NotContains(tt, joined, "side-by-side")
	require.NotContains(tt, joined, "unified")
}

type scriptedDiffProvider struct {
	repoRoot      string
	branch        string
	diffs         []string
	unstagedDiffs []string
	stagedDiffs   []string
	sections      []DiffSection
	manualRefresh *bool
	index         int
	unstagedIndex int
	stagedIndex   int
}

func (p *scriptedDiffProvider) LoadDiff(staged bool) (string, error) {
	if len(p.unstagedDiffs) > 0 || len(p.stagedDiffs) > 0 {
		if staged {
			if len(p.stagedDiffs) == 0 {
				return "", nil
			}
			if p.stagedIndex >= len(p.stagedDiffs) {
				return p.stagedDiffs[len(p.stagedDiffs)-1], nil
			}
			value := p.stagedDiffs[p.stagedIndex]
			p.stagedIndex++
			return value, nil
		}
		if len(p.unstagedDiffs) == 0 {
			return "", nil
		}
		if p.unstagedIndex >= len(p.unstagedDiffs) {
			return p.unstagedDiffs[len(p.unstagedDiffs)-1], nil
		}
		value := p.unstagedDiffs[p.unstagedIndex]
		p.unstagedIndex++
		return value, nil
	}

	// Legacy fixture path: `diffs` represent only unstaged changes.
	if staged || len(p.diffs) == 0 {
		return "", nil
	}
	if p.index >= len(p.diffs) {
		return p.diffs[len(p.diffs)-1], nil
	}
	value := p.diffs[p.index]
	p.index++
	return value, nil
}

func (p *scriptedDiffProvider) RepoRoot() (string, error) {
	return p.repoRoot, nil
}

func (p *scriptedDiffProvider) CurrentBranch() (string, error) {
	return p.branch, nil
}

func (p *scriptedDiffProvider) Sections() []DiffSection {
	if len(p.sections) == 0 {
		return nil
	}
	return p.sections
}

func (p *scriptedDiffProvider) ManualRefreshEnabled() bool {
	if p.manualRefresh == nil {
		return true
	}
	return *p.manualRefresh
}

func boolPtr(value bool) *bool {
	return &value
}

func diffForPaths(paths ...string) string {
	var builder strings.Builder
	for _, path := range paths {
		builder.WriteString("diff --git a/")
		builder.WriteString(path)
		builder.WriteString(" b/")
		builder.WriteString(path)
		builder.WriteString("\n")
		builder.WriteString("index 1111111..2222222 100644\n")
		builder.WriteString("--- a/")
		builder.WriteString(path)
		builder.WriteString("\n")
		builder.WriteString("+++ b/")
		builder.WriteString(path)
		builder.WriteString("\n")
		builder.WriteString("@@ -1 +1 @@\n")
		builder.WriteString("-old\n")
		builder.WriteString("+new\n")
	}
	return builder.String()
}

func diffForPathWithStats(path string, additions int, deletions int) string {
	var builder strings.Builder
	builder.WriteString("diff --git a/")
	builder.WriteString(path)
	builder.WriteString(" b/")
	builder.WriteString(path)
	builder.WriteString("\n")
	builder.WriteString("index 1111111..2222222 100644\n")
	builder.WriteString("--- a/")
	builder.WriteString(path)
	builder.WriteString("\n")
	builder.WriteString("+++ b/")
	builder.WriteString(path)
	builder.WriteString("\n")

	switch {
	case additions > 0 && deletions == 0:
		builder.WriteString(fmt.Sprintf("@@ -0,0 +1,%d @@\n", additions))
		for i := 0; i < additions; i++ {
			builder.WriteString(fmt.Sprintf("+new%d\n", i+1))
		}
	case additions == 0 && deletions > 0:
		builder.WriteString(fmt.Sprintf("@@ -1,%d +0,0 @@\n", deletions))
		for i := 0; i < deletions; i++ {
			builder.WriteString(fmt.Sprintf("-old%d\n", i+1))
		}
	default:
		builder.WriteString("@@ -1 +1 @@\n")
		builder.WriteString("-old\n")
		builder.WriteString("+new\n")
	}

	return builder.String()
}

func findTreePathByDataPath(nodes []t.TreeNode[DiffTreeNodeData], target string) ([]int, bool) {
	var walk func(items []t.TreeNode[DiffTreeNodeData], prefix []int) ([]int, bool)
	walk = func(items []t.TreeNode[DiffTreeNodeData], prefix []int) ([]int, bool) {
		for idx, node := range items {
			next := append(clonePath(prefix), idx)
			if node.Data.Path == target {
				return next, true
			}
			if path, ok := walk(node.Children, next); ok {
				return path, true
			}
		}
		return nil, false
	}
	return walk(nodes, nil)
}

func findPaletteItemByLabel(items []t.CommandPaletteItem, label string) t.CommandPaletteItem {
	for _, item := range items {
		if item.Label == label {
			return item
		}
	}
	return t.CommandPaletteItem{}
}

func findFilterInput(tt *testing.T, app *Dv) t.TextInput {
	tt.Helper()
	theme, ok := t.GetTheme(t.CurrentThemeName())
	require.True(tt, ok)
	ctx := t.NewBuildContext(nil, t.AnySignal[t.Focusable]{}, t.AnySignal[t.Widget]{}, nil)
	widget := app.buildLeftPane(ctx, theme)
	column, ok := widget.(t.Column)
	require.True(tt, ok)
	for _, child := range column.Children {
		input, ok := child.(t.TextInput)
		if !ok {
			continue
		}
		if input.ID == diffFilesFilterID {
			return input
		}
	}
	require.FailNow(tt, "expected filter input in left pane")
	return t.TextInput{}
}

func keybindIsHidden(keybinds []t.Keybind, key string) bool {
	for _, keybind := range keybinds {
		if keybind.Key == key {
			return keybind.Hidden
		}
	}
	return false
}

func findKeybindByKey(keybinds []t.Keybind, key string) (t.Keybind, bool) {
	for _, keybind := range keybinds {
		if keybind.Key == key {
			return keybind, true
		}
	}
	return t.Keybind{}, false
}

func paletteThemeNames(items []t.CommandPaletteItem) []string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		themeName, ok := item.Data.(string)
		if ok && themeName != "" {
			names = append(names, themeName)
		}
	}
	return names
}

func spanTexts(spans []t.Span) []string {
	texts := make([]string, 0, len(spans))
	for _, span := range spans {
		texts = append(texts, span.Text)
	}
	return texts
}

func rowTextContents(row t.Row) []string {
	texts := []string{}
	for _, child := range row.Children {
		text, ok := child.(t.Text)
		if !ok {
			continue
		}
		if len(text.Spans) > 0 {
			var builder strings.Builder
			for _, span := range text.Spans {
				builder.WriteString(span.Text)
			}
			texts = append(texts, builder.String())
			continue
		}
		texts = append(texts, text.Content)
	}
	return texts
}

func indexOfTextContaining(texts []string, needle string) int {
	for idx, text := range texts {
		if strings.Contains(text, needle) {
			return idx
		}
	}
	return -1
}
