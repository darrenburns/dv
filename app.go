package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	t "github.com/darrenburns/terma"
)

const (
	diffFilesTreeID       = "terma-diff-files-tree"
	diffFilesScrollID     = "terma-diff-files-scroll"
	diffFilesFilterID     = "terma-diff-files-filter"
	diffViewerID          = "terma-diff-viewer"
	diffViewerScrollID    = "terma-diff-viewer-scroll"
	diffSplitPaneID       = "terma-diff-split"
	diffCommandPaletteID  = "terma-diff-command-palette"
	diffThemesPalette     = "Themes"
	diffJumpScrollLines   = 10
	treeSummaryCountAlpha = 0.65
)

type DiffLayoutMode int

const (
	DiffLayoutUnified DiffLayoutMode = iota
	DiffLayoutSideBySide
)

type DvInitialState struct {
	LayoutMode       DiffLayoutMode
	SidebarVisible   bool
	ThemeName        string
	IntralineStyle   IntralineStyleMode
	ShowChangeSigns  bool
	IgnoreWhitespace bool
}

func DefaultDvInitialState() DvInitialState {
	return DvInitialState{
		LayoutMode:       DiffLayoutUnified,
		SidebarVisible:   true,
		ThemeName:        t.ThemeNameObsidianTide,
		IntralineStyle:   IntralineStyleModeBackground,
		ShowChangeSigns:  false,
		IgnoreWhitespace: false,
	}
}

func normalizeDvInitialState(initial DvInitialState) DvInitialState {
	defaults := DefaultDvInitialState()

	switch initial.LayoutMode {
	case DiffLayoutUnified, DiffLayoutSideBySide:
	default:
		initial.LayoutMode = defaults.LayoutMode
	}

	switch initial.IntralineStyle {
	case IntralineStyleModeBackground, IntralineStyleModeUnderline, IntralineStyleModeOff:
	default:
		initial.IntralineStyle = defaults.IntralineStyle
	}

	parsedThemeName, err := parseThemeName(initial.ThemeName)
	if err != nil {
		initial.ThemeName = defaults.ThemeName
	} else {
		initial.ThemeName = parsedThemeName
	}

	return initial
}

type diffScrollAnchor struct {
	kind    RenderedLineKind
	oldLine int
	newLine int
}

type fileScrollState struct {
	mode   DiffLayoutMode
	offset int
}

type diffSectionState struct {
	files              []*DiffFile
	roots              []t.TreeNode[DiffTreeNodeData]
	renderedByPath     map[string]*RenderedFile
	sideRenderedByPath map[string]*SideBySideRenderedFile
	fileByPath         map[string]*DiffFile
	filePathToTreePath map[string][]int
	orderedFilePaths   []string
	lastSelectedPath   string
	additions          int
	deletions          int
}

type infoCardStat struct {
	Label      string
	Value      string
	ValueColor t.Color
	Colorized  bool
}

type infoCardModel struct {
	Heading    string
	Details    string
	Stats      []infoCardStat
	Actions    []string
	Background t.ColorProvider
}

type indexCommandKind int

const (
	indexCommandStagePath indexCommandKind = iota
	indexCommandUnstagePath
	indexCommandStageAll
	indexCommandUnstageAll
)

type indexCommand struct {
	Kind indexCommandKind
	Path string
}

type indexResult struct {
	Refresh bool
}

// Dv is a syntax-highlighted git diff viewer.
type Dv struct {
	provider DiffProvider

	repoRoot string
	branch   string
	loadErr  string
	files    []*DiffFile

	copyPathToClipboard func(string) error

	activePath  string
	activeIsDir bool
	activeKind  DiffTreeNodeKind

	activeFileSection DiffSection

	renderedByPath     map[string]*RenderedFile
	sideRenderedByPath map[string]*SideBySideRenderedFile
	fileByPath         map[string]*DiffFile
	filePathToTreePath map[string][]int
	orderedFilePaths   []string
	sectionOrder       []DiffSection
	activeSection      DiffSection
	initialSection     DiffSection
	sections           map[DiffSection]*diffSectionState

	treeState           *t.TreeState[DiffTreeNodeData]
	treeScrollState     *t.ScrollState
	treeFilterState     *t.FilterState
	treeFilterInput     *t.TextInputState
	diffScrollState     *t.ScrollState
	diffViewState       *DiffViewState
	splitState          *t.SplitPaneState
	commandPalette      *t.CommandPaletteState
	indexPendingCount   t.Signal[int]
	indexResultVersion  t.Signal[int]
	indexCommandQueue   chan indexCommand
	indexResultMu       sync.Mutex
	pendingIndexResults []indexResult

	treeFilterVisible    bool
	treeFilterNoMatches  bool
	diffLayoutMode       DiffLayoutMode
	diffHardWrap         bool
	diffHideChangeSigns  bool
	diffIntralineStyle   IntralineStyleMode
	diffIgnoreWhitespace bool
	manualRefreshEnabled bool
	focusedWidgetID      string
	sidebarVisible       bool

	dividerFocused        bool
	dividerHovered        bool
	dividerFocusRequested bool
	lastNonDividerFocus   string
	focusReturnID         string
	themeCursorSynced     bool
	themePreviewBase      string

	layoutToggleScrollRestoreValid  bool
	layoutToggleScrollSourceMode    DiffLayoutMode
	layoutToggleScrollTargetMode    DiffLayoutMode
	layoutToggleScrollSourceOffset  int
	layoutToggleScrollTargetOffset  int
	layoutToggleScrollActivePath    string
	layoutToggleScrollActiveSection DiffSection

	fileScrollOffsets map[string]fileScrollState
	reviewedByFile    map[string]bool
}

func NewDv(provider DiffProvider, staged bool, initialState DvInitialState) *Dv {
	initialState = normalizeDvInitialState(initialState)
	t.SetTheme(initialState.ThemeName)

	sectionOrder := defaultDiffSections()
	if customSectionProvider, ok := provider.(DiffSectionsProvider); ok {
		sectionOrder = normalizeDiffSections(customSectionProvider.Sections())
	}

	initialSection := sectionOrder[0]
	if staged && containsSection(sectionOrder, DiffSectionStaged) {
		initialSection = DiffSectionStaged
	}

	manualRefreshEnabled := true
	if manualRefreshProvider, ok := provider.(ManualRefreshCapable); ok {
		manualRefreshEnabled = manualRefreshProvider.ManualRefreshEnabled()
	}

	app := &Dv{
		provider:             provider,
		renderedByPath:       map[string]*RenderedFile{},
		sideRenderedByPath:   map[string]*SideBySideRenderedFile{},
		fileByPath:           map[string]*DiffFile{},
		filePathToTreePath:   map[string][]int{},
		orderedFilePaths:     []string{},
		sectionOrder:         sectionOrder,
		activeSection:        initialSection,
		initialSection:       initialSection,
		sections:             newDiffSectionStateMap(sectionOrder),
		treeState:            t.NewTreeState([]t.TreeNode[DiffTreeNodeData]{}),
		treeScrollState:      t.NewScrollState(),
		treeFilterState:      t.NewFilterState(),
		treeFilterInput:      t.NewTextInputState(""),
		diffScrollState:      t.NewScrollState(),
		diffViewState:        NewDiffViewState(buildMetaRenderedFile("Diff", []string{"Loading diff..."})),
		splitState:           t.NewSplitPaneState(0.30),
		indexPendingCount:    t.NewSignal(0),
		indexResultVersion:   t.NewSignal(0),
		indexCommandQueue:    make(chan indexCommand, 256),
		pendingIndexResults:  []indexResult{},
		sidebarVisible:       initialState.SidebarVisible,
		diffLayoutMode:       initialState.LayoutMode,
		diffHideChangeSigns:  !initialState.ShowChangeSigns,
		diffIntralineStyle:   initialState.IntralineStyle,
		diffIgnoreWhitespace: initialState.IgnoreWhitespace,
		manualRefreshEnabled: manualRefreshEnabled,
		lastNonDividerFocus:  diffViewerScrollID,
		focusReturnID:        diffViewerScrollID,
		copyPathToClipboard:  copyPathToClipboardOSC52,
		fileScrollOffsets:    map[string]fileScrollState{},
		reviewedByFile:       map[string]bool{},
	}
	if app.isPipedDiffMode() {
		app.diffIgnoreWhitespace = false
	}
	go app.runIndexCommandQueue()
	app.configureDiffHorizontalScroll()
	app.commandPalette = app.newCommandPalette()
	app.refreshDiff()
	t.RequestFocus(diffViewerScrollID)
	return app
}

func copyPathToClipboardOSC52(path string) error {
	_, err := os.Stdout.WriteString(ansi.SetClipboard(uv.SystemClipboard, path))
	return err
}

func newDiffSectionState() *diffSectionState {
	return &diffSectionState{
		files:              nil,
		roots:              []t.TreeNode[DiffTreeNodeData]{},
		renderedByPath:     map[string]*RenderedFile{},
		sideRenderedByPath: map[string]*SideBySideRenderedFile{},
		fileByPath:         map[string]*DiffFile{},
		filePathToTreePath: map[string][]int{},
		orderedFilePaths:   []string{},
	}
}

func newDiffSectionStateMap(sectionOrder []DiffSection) map[DiffSection]*diffSectionState {
	states := map[DiffSection]*diffSectionState{}
	for _, section := range sectionOrder {
		states[section] = newDiffSectionState()
	}
	return states
}

func containsSection(sections []DiffSection, target DiffSection) bool {
	for _, section := range sections {
		if section == target {
			return true
		}
	}
	return false
}

func (a *Dv) hasSection(section DiffSection) bool {
	return containsSection(a.sectionOrder, section)
}

func (a *Dv) canSwitchSections() bool {
	return len(a.sectionOrder) > 1
}

func (a *Dv) sectionIndex(section DiffSection) int {
	for idx, value := range a.sectionOrder {
		if value == section {
			return idx
		}
	}
	return -1
}

func (a *Dv) orderedSectionsFrom(start DiffSection) []DiffSection {
	if len(a.sectionOrder) == 0 {
		return nil
	}
	startIdx := a.sectionIndex(start)
	if startIdx < 0 {
		out := make([]DiffSection, len(a.sectionOrder))
		copy(out, a.sectionOrder)
		return out
	}

	ordered := make([]DiffSection, 0, len(a.sectionOrder))
	for i := 0; i < len(a.sectionOrder); i++ {
		ordered = append(ordered, a.sectionOrder[(startIdx+i)%len(a.sectionOrder)])
	}
	return ordered
}

func (a *Dv) orderedSectionsAfter(start DiffSection) []DiffSection {
	ordered := a.orderedSectionsFrom(start)
	if len(ordered) <= 1 {
		return nil
	}
	return ordered[1:]
}

func (a *Dv) findSectionWithFiles(start DiffSection) (DiffSection, bool) {
	for _, section := range a.orderedSectionsFrom(start) {
		if a.sectionHasFiles(section) {
			return section, true
		}
	}
	return "", false
}

func (a *Dv) sectionState(section DiffSection) *diffSectionState {
	if a.sections == nil {
		return nil
	}
	state := a.sections[section]
	if state == nil {
		return nil
	}
	return state
}

func (a *Dv) setActiveSection(section DiffSection) {
	if section == "" || !a.hasSection(section) {
		section = a.initialSection
	}
	a.activeSection = section
	a.syncActiveSectionCaches()
}

func (a *Dv) syncActiveSectionCaches() {
	state := a.sectionState(a.activeSection)
	if state == nil {
		a.files = nil
		a.renderedByPath = map[string]*RenderedFile{}
		a.sideRenderedByPath = map[string]*SideBySideRenderedFile{}
		a.fileByPath = map[string]*DiffFile{}
		a.filePathToTreePath = map[string][]int{}
		a.orderedFilePaths = nil
		return
	}
	a.files = state.files
	a.renderedByPath = state.renderedByPath
	a.sideRenderedByPath = state.sideRenderedByPath
	a.fileByPath = state.fileByPath
	a.filePathToTreePath = state.filePathToTreePath
	a.orderedFilePaths = state.orderedFilePaths
}

func (a *Dv) sectionHasFiles(section DiffSection) bool {
	state := a.sectionState(section)
	return state != nil && len(state.orderedFilePaths) > 0
}

func (a *Dv) sectionFileCount(section DiffSection) int {
	state := a.sectionState(section)
	if state == nil {
		return 0
	}
	return len(state.orderedFilePaths)
}

func (a *Dv) totalFileCount() int {
	total := 0
	for _, section := range a.sectionOrder {
		total += a.sectionFileCount(section)
	}
	return total
}

func (a *Dv) filteredFilePathsForSection(section DiffSection, query string, options t.FilterOptions) []string {
	state := a.sectionState(section)
	if state == nil || len(state.orderedFilePaths) == 0 {
		return nil
	}
	if query == "" {
		return state.orderedFilePaths
	}
	return collectFilteredTreeFilePaths(state.roots, query, options)
}

func (a *Dv) switchToFirstSelectableFile(section DiffSection) bool {
	state := a.sectionState(section)
	if state == nil || len(state.orderedFilePaths) == 0 {
		return false
	}
	a.setActiveSection(section)
	return a.selectFilePath(state.orderedFilePaths[0])
}

func (a *Dv) setActiveSectionSummary(section DiffSection) {
	a.setActiveSection(section)
	state := a.sectionState(section)
	a.activePath = section.DisplayName() + " changes"
	a.activeIsDir = false
	a.activeKind = DiffTreeNodeSection
	a.activeFileSection = ""
	if state == nil {
		return
	}
	a.diffViewState.SetRendered(buildSectionSummaryRenderedFile(section, state))
	a.diffScrollState.SetOffset(0)
}

func (a *Dv) setLoadError(message string) {
	a.loadErr = message
	a.sections = newDiffSectionStateMap(a.sectionOrder)
	a.setActiveSection(a.initialSection)
	a.activePath = ""
	a.activeIsDir = false
	a.activeKind = DiffTreeNodeUnknown
	a.activeFileSection = ""
	roots := make([]t.TreeNode[DiffTreeNodeData], 0, len(a.sectionOrder))
	for _, section := range a.sectionOrder {
		roots = append(roots, t.TreeNode[DiffTreeNodeData]{
			Data: DiffTreeNodeData{
				Name:         section.DisplayName(),
				Path:         string(section),
				IsDir:        true,
				Section:      section,
				NodeKind:     DiffTreeNodeSection,
				NodeKey:      diffSectionRootNodeKey(section),
				TouchedFiles: 0,
			},
			Children: []t.TreeNode[DiffTreeNodeData]{},
		})
	}
	a.treeState.Nodes.Set(roots)
	a.treeState.CursorPath.Set(nil)
	a.treeState.Collapsed.Set(map[string]bool{})
	a.treeFilterNoMatches = false
	a.diffViewState.SetRendered(messageToRendered("Error", a.errorMessage()))
	a.diffScrollState.SetOffset(0)
}

func (a *Dv) toggleMode() {
	a.switchSectionFocus()
}

func (a *Dv) Keybinds() []t.Keybind {
	showFilterFiles := a.focusedWidgetID == diffFilesTreeID || a.focusedWidgetID == diffViewerScrollID
	keybinds := []t.Keybind{
		{Key: "n", Name: "Next file", Action: func() { a.moveFileCursor(1) }},
		{Key: "]", Name: "Next file", Action: func() { a.moveFileCursor(1) }},
		{Key: "p", Name: "Prev file", Action: func() { a.moveFileCursor(-1) }},
		{Key: "[", Name: "Prev file", Action: func() { a.moveFileCursor(-1) }},
		{Key: "ctrl+j", Name: "Next file", Action: func() { a.moveFileCursor(1) }, Hidden: true},
		{Key: "ctrl+k", Name: "Prev file", Action: func() { a.moveFileCursor(-1) }, Hidden: true},
		{Key: "J", Name: "Jump down 10", Action: func() { a.jumpDiffVertical(diffJumpScrollLines) }, Hidden: true},
		{Key: "K", Name: "Jump up 10", Action: func() { a.jumpDiffVertical(-diffJumpScrollLines) }, Hidden: true},
		{Key: "/", Name: "Filter files", Action: a.openTreeFilter, Hidden: !showFilterFiles},
		{Key: "b", Name: "Toggle sidebar", Action: a.toggleSidebar, Hidden: true},
		{Key: "escape", Name: "Clear filter", Action: a.handleEscape, Hidden: true},
		{Key: "r", Name: "Refresh", Action: a.manualRefresh, Hidden: true},
		{Key: "y", Name: "Copy path", Action: a.copyActiveFilePath, Hidden: true},
		{Key: "w", Name: "Toggle line wrap", Action: a.toggleDiffWrap, Hidden: true},
		{Key: "v", Name: "Toggle split", Action: a.toggleDiffLayoutMode, Hidden: true},
		{Key: "ctrl+h", Name: "Shift split left", Action: a.shiftSideBySideSplitLeft, Hidden: true},
		{Key: "ctrl+l", Name: "Shift split right", Action: a.shiftSideBySideSplitRight, Hidden: true},
		{Key: "i", Name: "Toggle intraline style", Action: a.toggleDiffIntralineStyle, Hidden: true},
		{Key: "m", Name: "Toggle seen", Action: a.toggleActiveFileReviewed, Hidden: true},
		{Key: "M", Name: "Clear all seen", Action: a.clearAllReviewed, Hidden: true},
		{Key: "d", Name: "Focus divider", Action: a.focusDivider, Hidden: true},
		{Key: "ctrl+p", Name: "Command palette", Action: a.togglePalette},
		{Key: "t", Name: "Theme menu", Action: a.openThemePalette, Hidden: true},
		{Key: "q", Name: "Quit", Action: t.Quit},
	}
	if a.canToggleStageActiveFile() {
		keybinds = append(keybinds, t.Keybind{
			Key:    "s",
			Name:   a.activeFileStageActionName(),
			Action: a.toggleStageActiveFile,
		})
	}
	if a.canStageFiles() {
		keybinds = append(keybinds, t.Keybind{
			Key:    "S",
			Name:   "Stage all files",
			Action: a.stageAllFiles,
		})
	}
	if a.canUnstageFiles() {
		keybinds = append(keybinds, t.Keybind{
			Key:    "U",
			Name:   "Unstage all files",
			Action: a.unstageAllFiles,
		})
	}
	if a.canToggleDiffIgnoreWhitespace() {
		keybinds = append(keybinds, t.Keybind{
			Key:    "x",
			Name:   "Toggle ignore whitespace",
			Action: a.toggleDiffIgnoreWhitespace,
			Hidden: true,
		})
	}
	return keybinds
}

func (a *Dv) Build(ctx t.BuildContext) t.Widget {
	a.flushPendingIndexUpdates()
	a.syncFocusState(ctx)
	theme := ctx.Theme()
	body := a.buildRightPane(theme)
	if a.sidebarVisible {
		dividerFg := dividerForeground(theme)
		if a.dividerHovered {
			dividerFg = dividerHoverForeground(theme)
		}
		body = FocusAwareSplitPane{
			SplitPane: t.SplitPane{
				ID:                     diffSplitPaneID,
				State:                  a.splitState,
				Orientation:            t.SplitHorizontal,
				DividerSize:            1,
				MinPaneSize:            20,
				DividerBackground:      theme.Background,
				DividerForeground:      dividerFg,
				DividerFocusForeground: dividerFocusForeground(theme),
				Hover: func(event t.HoverEvent) {
					a.dividerHovered = event.Type == t.HoverEnter
				},
				OnExitFocus: a.exitDividerFocus,
				Style: t.Style{
					Width:           t.Flex(1),
					Height:          t.Flex(1),
					BackgroundColor: theme.Background,
				},
				First:  a.buildLeftPane(ctx, theme),
				Second: a.buildRightPane(theme),
			},
			AllowFocus:     a.dividerFocused || a.dividerFocusRequested,
			EnableKeybinds: a.dividerFocused,
		}
	}

	return t.Stack{
		Style: t.Style{
			Width:           t.Flex(1),
			Height:          t.Flex(1),
			BackgroundColor: theme.Background,
		},
		Children: []t.Widget{
			t.Dock{
				Style: t.Style{
					BackgroundColor: theme.Background,
				},
				Top: []t.Widget{a.buildHeader(theme)},
				Bottom: []t.Widget{
					t.Row{
						Style: t.Style{
							Width:           t.Flex(1),
							BackgroundColor: theme.Background,
						},
						Children: []t.Widget{
							t.Spacer{Width: t.Flex(1)},
							t.KeybindBar{
								Style: t.Style{
									Width:           t.Auto,
									BackgroundColor: theme.Background,
									Padding:         t.EdgeInsetsXY(1, 0),
								},
							},
							t.Spacer{Width: t.Flex(1)},
						},
					},
				},
				Body: body,
			},
			t.CommandPalette{
				ID:             diffCommandPaletteID,
				State:          a.commandPalette,
				Position:       t.FloatPositionTopCenter,
				Offset:         t.Offset{Y: 1},
				BackdropColor:  t.Black.WithAlpha(0.05),
				OnSelect:       a.handlePaletteSelect,
				OnCursorChange: a.handlePaletteCursorChange,
				OnDismiss:      a.handlePaletteDismiss,
			},
		},
	}
}

func (a *Dv) buildHeader(theme t.ThemeData) t.Widget {
	repoName := "(unknown repo)"
	if a.repoRoot != "" {
		repoName = filepath.Base(a.repoRoot)
	}

	rightWidget := t.Text{
		Content: themeDisplayName(t.CurrentThemeName()) + " [t]",
		Style: t.Style{
			Padding:         t.EdgeInsetsXY(1, 0),
			ForegroundColor: theme.SecondaryText,
		},
	}
	if a.loadErr != "" {
		rightWidget = t.Label("Error loading diff", t.LabelError, theme)
	}

	children := []t.Widget{
		t.Label(repoName, t.LabelPrimary, theme),
	}
	if a.branch != "" {
		children = append(children,
			t.Spacer{Width: t.Cells(1)},
			t.Text{
				Content: a.branch,
				Style: t.Style{
					ForegroundColor: theme.Accent,
				},
			},
		)
	}
	if a.loadErr != "" {
		children = append(children,
			t.Spacer{Width: t.Cells(1)},
			t.Label("Error", t.LabelError, theme),
		)
	}
	children = append(children,
		t.Spacer{Width: t.Flex(1)},
		a.buildHeaderModeIndicator(theme),
		t.Spacer{Width: t.Cells(1)},
		rightWidget,
	)

	return t.Row{
		Style: t.Style{
			Width:   t.Flex(1),
			Padding: t.EdgeInsets{Left: 1},
			BackgroundColor: t.NewGradient(
				theme.Surface,
				theme.Surface,
				theme.Background,
				theme.Background,
				theme.Background,
				theme.SecondaryBg,
			).WithAngle(90),
		},
		Children: children,
	}
}

func (a *Dv) buildLeftPane(ctx t.BuildContext, theme t.ThemeData) t.Widget {
	treeWidget := SplitFriendlyTree{
		Tree: t.Tree[DiffTreeNodeData]{
			ID:                diffFilesTreeID,
			State:             a.treeState,
			Filter:            a.treeFilterState,
			ScrollState:       a.treeScrollState,
			Style:             t.Style{Width: t.Flex(1), Padding: t.EdgeInsets{Left: 1}},
			ExpandIndicator:   "▼ ",
			CollapseIndicator: "▶ ",
			LeafIndicator:     " ",
			NodeID: func(node DiffTreeNodeData) string {
				if node.NodeKey != "" {
					return node.NodeKey
				}
				return node.Path
			},
			HasChildren: func(node DiffTreeNodeData) bool {
				return node.IsDir
			},
			MatchNode: func(node DiffTreeNodeData, query string, options t.FilterOptions) t.MatchResult {
				return t.MatchString(node.Name, query, options)
			},
			OnCursorChange: a.onTreeCursorChange,
		},
	}

	sidebarFocused := ctx.IsFocused(treeWidget)
	treeWidget.RenderNodeWithMatch = a.renderTreeNode(theme, sidebarFocused)

	children := []t.Widget{
		t.Row{
			Style: t.Style{
				Width:           t.Flex(1),
				Padding:         t.EdgeInsetsXY(1, 0),
				BackgroundColor: theme.Background,
			},
			Children: []t.Widget{
				t.Text{Spans: a.sidebarHeadingSpans(theme)},
				t.Spacer{Width: t.Flex(1)},
				t.Text{Spans: a.sidebarTotalsSpans(theme)},
			},
		},
	}

	if a.shouldShowTreeFilterInput() {
		children = append(children, t.TextInput{
			ID:          diffFilesFilterID,
			State:       a.treeFilterInput,
			Placeholder: "Filter files...",
			Width:       t.Flex(1),
			Style: t.Style{
				Padding:         t.EdgeInsetsXY(1, 0),
				BackgroundColor: theme.Background,
				ForegroundColor: theme.Text,
			},
			OnChange:      a.onTreeFilterChange,
			ExtraKeybinds: a.treeFilterInputKeybinds(),
		})
	}

	treeContent := t.Widget(treeWidget)
	if a.treeFilterNoMatches {
		treeContent = a.buildTreeFilterEmptyState(theme)
	}

	children = append(children, t.Scrollable{
		ID:    diffFilesScrollID,
		State: a.treeScrollState,
		Style: t.Style{
			Width:           t.Flex(1),
			Height:          t.Flex(1),
			BackgroundColor: theme.Background,
		},
		Child: treeContent,
	})

	return t.Column{
		Height: t.Flex(1),
		Style: t.Style{
			BackgroundColor: theme.Background,
		},
		Children: children,
	}
}

func (a *Dv) renderTreeNode(theme t.ThemeData, widgetFocused bool) func(node DiffTreeNodeData, nodeCtx t.TreeNodeContext, match t.MatchResult) t.Widget {
	highlightStyle := t.MatchHighlightStyle(theme)
	return func(node DiffTreeNodeData, nodeCtx t.TreeNodeContext, match t.MatchResult) t.Widget {
		rowStyle := t.Style{
			Width:   t.Flex(1),
			Padding: t.EdgeInsets{Right: 1},
		}
		labelColor := theme.Text
		labelStyle := t.Style{}
		addColor := theme.Success
		delColor := theme.Error
		addStyle := t.Style{ForegroundColor: addColor}
		delStyle := t.Style{ForegroundColor: delColor}

		if node.NodeKind == DiffTreeNodeSection {
			labelStyle.Bold = true
			labelColor = sectionColor(theme, node.Section)
		}

		if nodeCtx.FilteredAncestor && node.NodeKind != DiffTreeNodeSection {
			labelColor = theme.TextMuted
		}
		isReviewed := node.NodeKind == DiffTreeNodeFile && a.isReviewed(node.Section, node.Path)
		if isReviewed {
			labelStyle.Strikethrough = true
		}

		if nodeCtx.Active {
			if widgetFocused {
				rowStyle.BackgroundColor = theme.ActiveCursor
				labelColor = theme.SelectionText
				addColor = theme.SelectionText
				delColor = theme.SelectionText
			} else {
				rowStyle.BackgroundColor = unfocusedTreeCursorColor(theme)
			}
		}
		if node.NodeKind == DiffTreeNodeDirectory {
			labelColor = labelColor.WithAlpha(labelColor.Alpha() * treeSummaryCountAlpha)
		}
		if node.NodeKind == DiffTreeNodeSection || node.NodeKind == DiffTreeNodeDirectory {
			addColor = addColor.WithAlpha(addColor.Alpha() * treeSummaryCountAlpha)
			delColor = delColor.WithAlpha(delColor.Alpha() * treeSummaryCountAlpha)
		}
		labelStyle.ForegroundColor = labelColor
		addStyle.ForegroundColor = addColor
		delStyle.ForegroundColor = delColor

		label := node.Name
		labelSuffix := ""
		switch node.NodeKind {
		case DiffTreeNodeSection:
			labelSuffix = fmt.Sprintf(" (%d)", node.TouchedFiles)
		case DiffTreeNodeDirectory:
			labelSuffix = "/"
		}
		label += labelSuffix

		labelWidget := t.Text{Content: label, Style: labelStyle}
		if node.NodeKind != DiffTreeNodeSection && match.Matched && len(match.Ranges) > 0 {
			spans := t.HighlightSpans(node.Name, match.Ranges, highlightStyle)
			if labelSuffix != "" {
				spans = append(spans, t.Span{Text: labelSuffix})
			}
			if isReviewed {
				for i := range spans {
					spans[i].Style.Strikethrough = true
				}
			}
			labelWidget = t.Text{
				Spans: spans,
				Style: labelStyle,
			}
		}

		children := []t.Widget{
			labelWidget,
		}
		children = append(children, t.Spacer{Width: t.Flex(1)})
		if addText, delText := nonZeroChangeTexts(node.Additions, node.Deletions); addText != "" || delText != "" {
			if addText != "" {
				children = append(children, t.Text{Content: addText, Style: addStyle})
			}
			if delText != "" {
				if addText != "" {
					children = append(children, t.Text{Content: " "})
				}
				children = append(children, t.Text{Content: delText, Style: delStyle})
			}
		}

		return t.Row{
			Style:    rowStyle,
			Children: children,
		}
	}
}

func (a *Dv) buildRightPane(theme t.ThemeData) t.Widget {
	viewer := DiffView{
		ID:              diffViewerID,
		DisableFocus:    true,
		State:           a.diffViewState,
		VerticalScroll:  a.diffScrollState,
		LayoutMode:      a.diffLayoutMode,
		HardWrap:        a.diffHardWrap,
		HideChangeSigns: a.diffHideChangeSigns,
		IntralineStyle:  a.diffIntralineStyle,
		Palette:         NewThemePalette(theme),
		Style: t.Style{
			Width:           t.Flex(1),
			Padding:         t.EdgeInsets{},
			BackgroundColor: theme.Background,
		},
	}
	viewerContent := t.Widget(viewer)
	if infoCard, ok := a.buildNonFileInfoCard(theme); ok {
		viewerContent = infoCard
	}

	return t.Column{
		Height: t.Flex(1),
		Style: t.Style{
			BackgroundColor: theme.Background,
		},
		Children: []t.Widget{
			a.buildViewerTitle(theme),
			t.Scrollable{
				ID:        diffViewerScrollID,
				State:     a.diffScrollState,
				Focusable: true,
				Style: t.Style{
					Width:           t.Flex(1),
					BackgroundColor: theme.Background,
				},
				Child: viewerContent,
			},
			viewerEmptySpaceHatch{
				Style: t.Style{
					Width:           t.Flex(1),
					Height:          t.Flex(1),
					BackgroundColor: theme.Background,
				},
				Foreground: viewerEmptySpaceBackground(theme),
			},
		},
	}
}

type viewerEmptySpaceHatch struct {
	Style      t.Style
	Foreground t.ColorProvider
}

func (v viewerEmptySpaceHatch) Build(ctx t.BuildContext) t.Widget {
	return v
}

func (v viewerEmptySpaceHatch) GetStyle() t.Style {
	return v.Style
}

func (v viewerEmptySpaceHatch) Render(ctx *t.RenderContext) {
	if ctx.Width <= 0 || ctx.Height <= 0 {
		return
	}

	if v.Style.BackgroundColor != nil && v.Style.BackgroundColor.IsSet() {
		bg := v.Style.BackgroundColor.ColorAt(ctx.Width, ctx.Height, 0, 0)
		ctx.FillRect(0, 0, ctx.Width, ctx.Height, bg)
	}

	for row := 0; row < ctx.Height; row++ {
		for col := 0; col < ctx.Width; col++ {
			style := t.Style{}
			if v.Foreground != nil && v.Foreground.IsSet() {
				style.ForegroundColor = v.Foreground.ColorAt(ctx.Width, ctx.Height, col, row)
			}
			ctx.DrawStyledText(col, row, sideEmptyHatchRune, style)
		}
	}
}

func (a *Dv) shouldShowDiffEmptyState() bool {
	return a.loadErr == "" &&
		!a.treeFilterNoMatches &&
		a.activeKind == DiffTreeNodeUnknown &&
		a.totalFileCount() == 0
}

func (a *Dv) buildNonFileInfoCard(theme t.ThemeData) (t.Widget, bool) {
	if a.loadErr != "" {
		return nil, false
	}
	switch a.activeKind {
	case DiffTreeNodeSection:
		return a.buildSectionInfoCard(theme), true
	case DiffTreeNodeDirectory:
		return a.buildDirectoryInfoCard(theme), true
	case DiffTreeNodeUnknown:
		if a.shouldShowDiffEmptyState() {
			return a.buildDiffEmptyState(theme), true
		}
	}
	return nil, false
}

func (a *Dv) buildDiffEmptyState(theme t.ThemeData) t.Widget {
	heading, details := a.emptyMessageParts()
	return a.buildInfoCard(theme, infoCardModel{
		Heading: heading,
		Details: details,
		Actions: a.emptyStateActionHints(),
	})
}

func (a *Dv) buildSectionInfoCard(theme t.ThemeData) t.Widget {
	state := a.sectionState(a.activeSection)
	fileCount := 0
	additions := 0
	deletions := 0
	if state != nil {
		fileCount = len(state.orderedFilePaths)
		additions = state.additions
		deletions = state.deletions
	}

	details := fmt.Sprintf("Changed files in this section: %d.", fileCount)
	if fileCount == 0 {
		details = "No files in this section."
	}

	actions := []string{
		a.actionHint("Command palette", "Open command palette"),
		a.dualActionHint("Next file", "Prev file", "Jump between files"),
		a.actionHint("Filter files", "Filter files"),
	}
	if a.manualRefreshEnabled {
		actions = append(actions, a.actionHint("Refresh", "Refresh diff"))
	}

	return a.buildInfoCard(theme, infoCardModel{
		Details:    details,
		Background: sectionInfoCardBackground(theme, a.activeSection),
		Stats: []infoCardStat{
			{Label: "Touched files", Value: fmt.Sprintf("%d", fileCount)},
			{Label: "Additions", Value: fmt.Sprintf("+%d", additions), ValueColor: theme.Success, Colorized: true},
			{Label: "Deletions", Value: fmt.Sprintf("-%d", deletions), ValueColor: theme.Error, Colorized: true},
		},
		Actions: actions,
	})
}

func (a *Dv) buildDirectoryInfoCard(theme t.ThemeData) t.Widget {
	path := strings.TrimSpace(a.activePath)
	if path == "" {
		path = "(root)"
	}
	displayPath := strings.TrimSuffix(path, "/")
	if displayPath == "" {
		displayPath = "(root)"
	}

	touched := 0
	additions := 0
	deletions := 0
	if node, ok := a.findDirectoryNode(a.activeSection, path); ok {
		touched = node.TouchedFiles
		additions = node.Additions
		deletions = node.Deletions
	}

	return a.buildInfoCard(theme, infoCardModel{
		Heading: fmt.Sprintf("Directory: %s/", displayPath),
		Details: fmt.Sprintf("Changed files in this directory: %d.", touched),
		Stats: []infoCardStat{
			{Label: "Touched files", Value: fmt.Sprintf("%d", touched)},
			{Label: "Additions", Value: fmt.Sprintf("+%d", additions), ValueColor: theme.Success, Colorized: true},
			{Label: "Deletions", Value: fmt.Sprintf("-%d", deletions), ValueColor: theme.Error, Colorized: true},
		},
		Actions: []string{
			a.actionHint("Command palette", "Open command palette"),
			a.dualActionHint("Next file", "Prev file", "Jump between files"),
			a.actionHint("Filter files", "Filter files"),
			a.actionHint("Copy path", "Copy this directory path"),
		},
	})
}

func (a *Dv) buildInfoCard(theme t.ThemeData, model infoCardModel) t.Widget {
	children := []t.Widget{}

	if strings.TrimSpace(model.Heading) != "" {
		children = append(children, t.Text{
			Content: model.Heading,
			Wrap:    t.WrapSoft,
			Style: t.Style{
				ForegroundColor: theme.Text,
				Bold:            true,
			},
		})
	}

	if strings.TrimSpace(model.Details) != "" {
		if len(children) > 0 {
			children = append(children, t.Spacer{Height: t.Cells(1)})
		}
		children = append(children,
			t.Text{
				Content: model.Details,
				Wrap:    t.WrapSoft,
				Style: t.Style{
					ForegroundColor: theme.Text,
				},
			},
		)
	}

	if len(model.Stats) > 0 {
		if len(children) > 0 {
			children = append(children, t.Spacer{Height: t.Cells(1)})
		}
		children = append(children,
			t.Text{
				Content: "Stats",
				Style: t.Style{
					ForegroundColor: theme.Text,
					Bold:            true,
				},
			},
		)

		for _, stat := range model.Stats {
			valueStyle := t.SpanStyle{Foreground: theme.Text}
			if stat.Colorized {
				valueStyle.Foreground = stat.ValueColor
				valueStyle.Bold = true
			}
			children = append(children, t.Text{
				Spans: []t.Span{
					t.StyledSpan(stat.Label+": ", t.SpanStyle{Foreground: theme.Text}),
					t.StyledSpan(stat.Value, valueStyle),
				},
			})
		}
	}

	if len(model.Actions) > 0 {
		if len(children) > 0 {
			children = append(children, t.Spacer{Height: t.Cells(1)})
		}
		children = append(children,
			t.Text{
				Content: "Next actions",
				Style: t.Style{
					ForegroundColor: theme.Text,
					Bold:            true,
				},
			},
		)

		for _, action := range model.Actions {
			children = append(children, t.Text{
				Content: action,
				Wrap:    t.WrapSoft,
				Style: t.Style{
					ForegroundColor: theme.Text,
				},
			})
		}
	}

	return t.Column{
		Style: t.Style{
			Width:           t.Flex(1),
			Height:          t.Flex(1),
			Padding:         t.EdgeInsets{Top: 1, Left: 2, Right: 2},
			BackgroundColor: firstNonNilColorProvider(model.Background, theme.Background),
		},
		Children: children,
	}
}

func (a *Dv) emptyStateActionHints() []string {
	actions := []string{a.actionHint("Command palette", "Open command palette")}
	if a.manualRefreshEnabled {
		actions = append(actions, a.actionHint("Refresh", "Refresh diff"))
	}
	if a.canToggleDiffIgnoreWhitespace() {
		actions = append(actions, a.actionHint("Toggle ignore whitespace", "Toggle ignore whitespace"))
	}
	return actions
}

func (a *Dv) actionHint(actionName string, description string) string {
	key := a.keybindKeyByName(actionName)
	if key == "" {
		return description
	}
	return fmt.Sprintf("[%s] %s", key, description)
}

func (a *Dv) paletteHint(actionName string) string {
	key := a.keybindKeyByName(actionName)
	if key == "" {
		return ""
	}
	return fmt.Sprintf("[%s]", key)
}

func (a *Dv) dualActionHint(firstActionName string, secondActionName string, description string) string {
	firstKey := a.keybindKeyByName(firstActionName)
	secondKey := a.keybindKeyByName(secondActionName)
	switch {
	case firstKey != "" && secondKey != "":
		return fmt.Sprintf("[%s]/[%s] %s", firstKey, secondKey, description)
	case firstKey != "":
		return fmt.Sprintf("[%s] %s", firstKey, description)
	case secondKey != "":
		return fmt.Sprintf("[%s] %s", secondKey, description)
	default:
		return description
	}
}

func (a *Dv) keybindKeyByName(name string) string {
	for _, keybind := range a.Keybinds() {
		if keybind.Name == name {
			return keybind.Key
		}
	}
	return ""
}

func sectionInfoCardBackground(theme t.ThemeData, section DiffSection) t.ColorProvider {
	switch section {
	case DiffSectionUnstaged:
		tint := theme.Background.Blend(theme.Error, 0.08)
		return t.NewGradient(tint, theme.Background).WithAngle(45)
	case DiffSectionStaged:
		tint := theme.Background.Blend(theme.Success, 0.08)
		return t.NewGradient(tint, theme.Background).WithAngle(45)
	default:
		return theme.Background
	}
}

func firstNonNilColorProvider(primary t.ColorProvider, fallback t.ColorProvider) t.ColorProvider {
	if primary != nil {
		return primary
	}
	return fallback
}

func (a *Dv) findDirectoryNode(section DiffSection, directoryPath string) (DiffTreeNodeData, bool) {
	if directoryPath == "" {
		return DiffTreeNodeData{}, false
	}
	state := a.sectionState(section)
	if state == nil {
		return DiffTreeNodeData{}, false
	}
	return findDirectoryNodeInTree(state.roots, directoryPath)
}

func findDirectoryNodeInTree(nodes []t.TreeNode[DiffTreeNodeData], directoryPath string) (DiffTreeNodeData, bool) {
	for _, node := range nodes {
		if node.Data.NodeKind == DiffTreeNodeDirectory && node.Data.Path == directoryPath {
			return node.Data, true
		}
		if found, ok := findDirectoryNodeInTree(node.Children, directoryPath); ok {
			return found, true
		}
	}
	return DiffTreeNodeData{}, false
}

func (a *Dv) buildViewerTitle(theme t.ThemeData) t.Widget {
	background := t.ColorProvider(theme.Background)
	if section, path, ok := a.activeReviewTarget(); ok && a.isReviewed(section, path) {
		background = reviewedViewerTitleBackground(theme)
	}

	style := t.Style{
		Padding:         t.EdgeInsetsXY(1, 0),
		BackgroundColor: background,
		ForegroundColor: theme.Text,
		Bold:            true,
	}

	title := a.viewerTitle()
	if a.activeKind != DiffTreeNodeFile {
		return t.Text{
			Content: title,
			Style:   style,
		}
	}

	file, ok := a.fileByPath[a.activePath]
	if !ok || file == nil {
		return viewerPathText{
			Text: t.Text{
				Content: title,
				Style:   style,
			},
			FullPath: title,
		}
	}

	metaSpans := make([]t.Span, 0, 8)
	if statSpans := nonZeroChangeStatSpans(file.Additions, file.Deletions, theme, true); len(statSpans) > 0 {
		metaSpans = append(metaSpans, statSpans...)
	}

	current, total, hasPosition := a.viewerFilePosition()
	if hasPosition {
		if len(metaSpans) > 0 {
			metaSpans = append(metaSpans, t.PlainSpan(" "))
		}
		metaSpans = append(metaSpans, t.StyledSpan(
			fmt.Sprintf("%s %d/%d", a.activeSection.DisplayName(), current, total),
			t.SpanStyle{Foreground: theme.TextMuted},
		))
	}

	if len(metaSpans) == 0 {
		return viewerPathText{
			Text: t.Text{
				Content: title,
				Style:   style,
			},
			FullPath: title,
		}
	}

	return t.Row{
		Style: t.Style{
			Padding:         style.Padding,
			BackgroundColor: style.BackgroundColor,
			ForegroundColor: style.ForegroundColor,
		},
		Children: []t.Widget{
			viewerPathText{
				Text: t.Text{
					Content: title,
					Style: t.Style{
						Width:           t.Flex(1),
						ForegroundColor: theme.Text,
						Bold:            true,
					},
				},
				FullPath: title,
			},
			t.Spacer{Width: t.Cells(1)},
			t.Text{
				Spans: metaSpans,
				Style: t.Style{
					ForegroundColor: theme.TextMuted,
				},
			},
		},
	}
}

func (a *Dv) viewerFilePosition() (current int, total int, ok bool) {
	if a.activeKind != DiffTreeNodeFile || a.activePath == "" {
		return 0, 0, false
	}

	state := a.sectionState(a.activeSection)
	if state == nil || len(state.orderedFilePaths) == 0 {
		return 0, 0, false
	}

	index := indexOfPath(state.orderedFilePaths, a.activePath)
	if index < 0 {
		return 0, 0, false
	}

	return index + 1, len(state.orderedFilePaths), true
}

func (a *Dv) buildHeaderModeIndicator(theme t.ThemeData) t.Widget {
	spans := []t.Span{
		t.StyledSpan(a.diffLayoutModeLabel(), t.SpanStyle{
			Foreground: theme.Text,
		}),
		t.PlainSpan(" "),
		t.StyledSpan("[v]", t.SpanStyle{
			Foreground: theme.Text,
		}),
	}
	if a.canToggleDiffIgnoreWhitespace() {
		ignoreWsLabel := "whitespace:off"
		if a.diffIgnoreWhitespace {
			ignoreWsLabel = "whitespace:on"
		}
		spans = append(spans,
			t.PlainSpan(" "),
			t.StyledSpan(ignoreWsLabel, t.SpanStyle{
				Foreground: theme.Text,
			}),
			t.PlainSpan(" "),
			t.StyledSpan("[x]", t.SpanStyle{
				Foreground: theme.Text,
			}),
		)
	}
	return t.Text{Spans: spans}
}

func (a *Dv) diffLayoutModeLabel() string {
	if a.diffLayoutMode == DiffLayoutSideBySide {
		return "split"
	}
	return "unified"
}

func (a *Dv) manualRefresh() {
	if !a.manualRefreshEnabled {
		return
	}
	a.refreshDiff()
}

func (a *Dv) canToggleDiffIgnoreWhitespace() bool {
	return !a.isPipedDiffMode()
}

func (a *Dv) indexProvider() IndexCapable {
	if a.isPipedDiffMode() {
		return nil
	}
	provider, ok := a.provider.(IndexCapable)
	if !ok {
		return nil
	}
	return provider
}

func (a *Dv) canStageFiles() bool {
	return a.indexProvider() != nil
}

func (a *Dv) canStageActiveFile() bool {
	if !a.canStageFiles() {
		return false
	}
	return a.activeKind == DiffTreeNodeFile && a.activePath != ""
}

func (a *Dv) canToggleStageActiveFile() bool {
	return a.canStageActiveFile()
}

func (a *Dv) canUnstageFiles() bool {
	return a.indexProvider() != nil
}

func (a *Dv) canCopyActiveFilePath() bool {
	if a.activePath == "" {
		return false
	}
	return a.activeKind == DiffTreeNodeFile || a.activeKind == DiffTreeNodeDirectory
}

func (a *Dv) activeFileIsStaged() bool {
	return a.activeKind == DiffTreeNodeFile && a.activeFileSection == DiffSectionStaged
}

func (a *Dv) activeFileStageActionName() string {
	if a.activeFileIsStaged() {
		return "Unstage file"
	}
	return "Stage file"
}

func (a *Dv) toggleStageActiveFile() {
	if !a.canToggleStageActiveFile() {
		return
	}
	path := a.activePath
	staged := a.activeFileIsStaged()
	a.rememberActiveFileScrollOffset()
	commandKind := indexCommandStagePath
	if staged {
		commandKind = indexCommandUnstagePath
	}
	a.enqueueIndexCommand(indexCommand{
		Kind: commandKind,
		Path: path,
	})
}

func (a *Dv) stageAllFiles() {
	a.enqueueIndexCommand(indexCommand{Kind: indexCommandStageAll})
}

func (a *Dv) unstageAllFiles() {
	a.enqueueIndexCommand(indexCommand{Kind: indexCommandUnstageAll})
}

func (a *Dv) enqueueIndexCommand(command indexCommand) {
	if a.indexProvider() == nil {
		return
	}
	a.indexPendingCount.Update(func(count int) int { return count + 1 })
	a.indexCommandQueue <- command
}

func (a *Dv) runIndexCommandQueue() {
	for command := range a.indexCommandQueue {
		result := a.executeIndexCommand(command)
		if result.Refresh {
			a.indexResultMu.Lock()
			a.pendingIndexResults = append(a.pendingIndexResults, result)
			a.indexResultMu.Unlock()
			a.indexResultVersion.Update(func(version int) int { return version + 1 })
		}
		a.indexPendingCount.Update(func(count int) int {
			if count <= 0 {
				return 0
			}
			return count - 1
		})
	}
}

func (a *Dv) executeIndexCommand(command indexCommand) indexResult {
	provider := a.indexProvider()
	if provider == nil {
		return indexResult{Refresh: true}
	}

	var err error
	switch command.Kind {
	case indexCommandStagePath:
		err = provider.StagePath(command.Path)
	case indexCommandUnstagePath:
		err = provider.UnstagePath(command.Path)
	case indexCommandStageAll:
		err = provider.StageAll()
	case indexCommandUnstageAll:
		err = provider.UnstageAll()
	default:
		return indexResult{Refresh: true}
	}
	if err != nil {
		return indexResult{Refresh: true}
	}
	return indexResult{Refresh: true}
}

func (a *Dv) flushPendingIndexUpdates() {
	_ = a.indexResultVersion.Get()
	results := a.drainPendingIndexResults()
	refreshNeeded := false
	for _, result := range results {
		if result.Refresh {
			refreshNeeded = true
		}
	}
	if refreshNeeded {
		a.refreshDiff()
	}
}

func (a *Dv) drainPendingIndexResults() []indexResult {
	a.indexResultMu.Lock()
	defer a.indexResultMu.Unlock()
	if len(a.pendingIndexResults) == 0 {
		return nil
	}
	results := append([]indexResult(nil), a.pendingIndexResults...)
	a.pendingIndexResults = a.pendingIndexResults[:0]
	return results
}

func (a *Dv) copyActiveFilePath() {
	if !a.canCopyActiveFilePath() {
		return
	}
	if a.copyPathToClipboard == nil {
		return
	}
	_ = a.copyPathToClipboard(a.activePath)
}

func (a *Dv) refreshDiff() {
	a.rememberActiveFileScrollOffset()

	if repoRoot, err := a.provider.RepoRoot(); err == nil {
		a.repoRoot = repoRoot
	}
	if branch, err := a.provider.CurrentBranch(); err == nil {
		a.branch = branch
	}

	previousSelections := map[DiffSection]string{}
	for _, section := range a.sectionOrder {
		state := a.sectionState(section)
		if state == nil {
			continue
		}
		if state.lastSelectedPath != "" {
			previousSelections[section] = state.lastSelectedPath
		}
	}
	if a.activeKind == DiffTreeNodeFile && a.activePath != "" {
		previousSelections[a.activeSection] = a.activePath
	}
	previousActiveSection := a.activeSection
	if previousActiveSection == "" || !a.hasSection(previousActiveSection) {
		previousActiveSection = a.initialSection
	}
	sectionRoots := map[DiffSection][]t.TreeNode[DiffTreeNodeData]{}
	for _, section := range a.sectionOrder {
		sectionRoots[section] = []t.TreeNode[DiffTreeNodeData]{}
	}
	nextSections := newDiffSectionStateMap(a.sectionOrder)

	for idx, section := range a.sectionOrder {
		raw, err := a.provider.LoadDiff(section == DiffSectionStaged, a.diffIgnoreWhitespace)
		if err != nil {
			a.setLoadError(fmt.Sprintf("%s diff: %v", strings.ToLower(section.DisplayName()), err))
			return
		}

		doc, err := parseUnifiedDiff(raw)
		if err != nil {
			a.setLoadError(fmt.Sprintf("%s parse error: %v", strings.ToLower(section.DisplayName()), err))
			return
		}

		state := nextSections[section]
		if state == nil {
			state = newDiffSectionState()
		}
		state.files = doc.Files
		state.renderedByPath = make(map[string]*RenderedFile, len(state.files))
		state.sideRenderedByPath = make(map[string]*SideBySideRenderedFile, len(state.files))
		state.fileByPath = make(map[string]*DiffFile, len(state.files))
		for _, file := range state.files {
			if file == nil {
				continue
			}
			state.fileByPath[file.DisplayPath] = file
			state.renderedByPath[file.DisplayPath] = buildRenderedFile(file)
			state.sideRenderedByPath[file.DisplayPath] = buildSideBySideRenderedFile(file)
			state.additions += file.Additions
			state.deletions += file.Deletions
		}

		roots, localTreePaths, orderedFilePaths := buildDiffTreeForSection(section, state.files)
		state.roots = roots
		state.orderedFilePaths = orderedFilePaths
		state.filePathToTreePath = make(map[string][]int, len(localTreePaths))
		for filePath, localPath := range localTreePaths {
			globalPath := make([]int, 0, len(localPath)+1)
			globalPath = append(globalPath, idx)
			globalPath = append(globalPath, localPath...)
			state.filePathToTreePath[filePath] = globalPath
		}

		if previous, ok := previousSelections[section]; ok {
			if _, exists := state.fileByPath[previous]; exists {
				state.lastSelectedPath = previous
			}
		}
		if state.lastSelectedPath == "" && len(state.orderedFilePaths) > 0 {
			state.lastSelectedPath = state.orderedFilePaths[0]
		}

		sectionRoots[section] = roots
		nextSections[section] = state
	}

	a.loadErr = ""
	a.sections = nextSections

	roots := make([]t.TreeNode[DiffTreeNodeData], 0, len(a.sectionOrder))
	for _, section := range a.sectionOrder {
		state := a.sectionState(section)
		if state == nil {
			state = newDiffSectionState()
		}
		roots = append(roots, t.TreeNode[DiffTreeNodeData]{
			Data: DiffTreeNodeData{
				Name:         section.DisplayName(),
				Path:         string(section),
				IsDir:        true,
				Additions:    state.additions,
				Deletions:    state.deletions,
				TouchedFiles: len(state.orderedFilePaths),
				Section:      section,
				NodeKind:     DiffTreeNodeSection,
				NodeKey:      diffSectionRootNodeKey(section),
			},
			Children: sectionRoots[section],
		})
	}
	a.treeState.Nodes.Set(roots)
	a.treeState.Collapsed.Set(map[string]bool{})

	if a.totalFileCount() == 0 {
		a.activeSection = a.initialSection
		a.syncActiveSectionCaches()
		a.activePath = ""
		a.activeIsDir = false
		a.activeKind = DiffTreeNodeUnknown
		a.activeFileSection = ""
		a.treeState.CursorPath.Set(nil)
		a.treeFilterNoMatches = false
		a.diffViewState.SetRendered(messageToRendered("Diff", a.emptyMessage()))
		a.diffScrollState.SetOffset(0)
		return
	}

	targetSection := previousActiveSection
	if !a.sectionHasFiles(targetSection) {
		if sectionWithFiles, ok := a.findSectionWithFiles(previousActiveSection); ok {
			targetSection = sectionWithFiles
		} else {
			targetSection = a.initialSection
		}
	}
	a.setActiveSection(targetSection)

	targetPath := ""
	state := a.sectionState(targetSection)
	if state != nil {
		targetPath = state.lastSelectedPath
		if targetPath == "" && len(state.orderedFilePaths) > 0 {
			targetPath = state.orderedFilePaths[0]
		}
	}
	if targetPath != "" {
		a.selectFilePath(targetPath)
	}
	a.syncTreeFilterSelection()
}

func (a *Dv) moveFileCursor(delta int) {
	filePaths := a.filePathsForNavigation()
	if len(filePaths) == 0 {
		return
	}

	currentIdx := -1
	if a.activeKind == DiffTreeNodeFile && !a.activeIsDir {
		currentIdx = indexOfPath(filePaths, a.activePath)
	}

	nextIdx := 0
	if currentIdx < 0 {
		if delta < 0 {
			nextIdx = len(filePaths) - 1
		}
	} else {
		nextIdx = currentIdx + delta
		for nextIdx < 0 {
			nextIdx += len(filePaths)
		}
		nextIdx = nextIdx % len(filePaths)
	}

	a.selectFilePath(filePaths[nextIdx])
}

func (a *Dv) treeFilterInputKeybinds() []t.Keybind {
	return []t.Keybind{
		{Key: "up", Action: func() { a.moveFileCursor(-1) }, Hidden: true},
		{Key: "down", Action: func() { a.moveFileCursor(1) }, Hidden: true},
	}
}

func (a *Dv) selectFilePath(filePath string) bool {
	if filePath == "" {
		return false
	}
	treePath, ok := a.filePathToTreePath[filePath]
	if !ok {
		return false
	}
	a.treeState.CursorPath.Set(clonePath(treePath))
	node, ok := a.treeState.NodeAtPath(treePath)
	if !ok {
		return false
	}
	a.onTreeCursorChange(node.Data)
	return true
}

func (a *Dv) onTreeCursorChange(node DiffTreeNodeData) {
	a.rememberActiveFileScrollOffset()

	if node.Section != "" {
		a.setActiveSection(node.Section)
	}
	switch node.NodeKind {
	case DiffTreeNodeSection:
		a.setActiveSectionSummary(node.Section)
		return
	case DiffTreeNodeDirectory:
		a.setActiveDirectory(node)
		return
	case DiffTreeNodeFile:
		if node.File != nil {
			a.setActiveFile(node.File)
			if state := a.sectionState(node.Section); state != nil {
				state.lastSelectedPath = node.Path
			}
			return
		}
	}
	if node.File != nil {
		a.setActiveFile(node.File)
		return
	}
	if rendered, ok := a.renderedByPath[node.Path]; ok {
		a.activePath = node.Path
		a.activeIsDir = false
		sideRendered := a.sideRenderedByPath[node.Path]
		if sideRendered == nil {
			sideRendered = buildSideBySideFromRendered(rendered)
		}
		a.activeKind = DiffTreeNodeFile
		a.activeFileSection = a.activeSection
		if state := a.sectionState(a.activeSection); state != nil {
			state.lastSelectedPath = node.Path
		}
		a.diffViewState.SetRenderedPair(rendered, sideRendered)
		a.restoreFileScrollOffset(node.Path)
	}
}

func (a *Dv) setActiveFile(file *DiffFile) {
	if file == nil {
		return
	}
	a.activePath = file.DisplayPath
	a.activeIsDir = false
	a.activeKind = DiffTreeNodeFile
	a.activeFileSection = a.activeSection
	if state := a.sectionState(a.activeSection); state != nil {
		state.lastSelectedPath = file.DisplayPath
	}
	rendered, ok := a.renderedByPath[file.DisplayPath]
	if !ok {
		rendered = buildRenderedFile(file)
		a.renderedByPath[file.DisplayPath] = rendered
	}
	sideRendered, ok := a.sideRenderedByPath[file.DisplayPath]
	if !ok {
		sideRendered = buildSideBySideRenderedFile(file)
		a.sideRenderedByPath[file.DisplayPath] = sideRendered
	}
	a.diffViewState.SetRenderedPair(rendered, sideRendered)
	a.restoreFileScrollOffset(file.DisplayPath)
}

func (a *Dv) setActiveDirectory(node DiffTreeNodeData) {
	if node.Section != "" {
		a.setActiveSection(node.Section)
	}
	a.activePath = node.Path
	a.activeIsDir = true
	a.activeKind = DiffTreeNodeDirectory
	a.activeFileSection = ""
	a.diffViewState.SetRendered(buildDirectorySummaryRenderedFile(node))
	a.diffScrollState.SetOffset(0)
}

func (a *Dv) switchSectionFocus() {
	if !a.canSwitchSections() {
		return
	}

	var targetSection DiffSection
	targetPath := ""
	query := ""
	options := t.FilterOptions{}
	if a.treeFilterState != nil {
		query = a.treeFilterState.PeekQuery()
		options = a.treeFilterState.PeekOptions()
	}

	for _, candidateSection := range a.orderedSectionsAfter(a.activeSection) {
		if query != "" {
			filtered := a.filteredFilePathsForSection(candidateSection, query, options)
			if len(filtered) == 0 {
				continue
			}
			targetSection = candidateSection
			targetPath = filtered[0]
			if state := a.sectionState(candidateSection); state != nil && state.lastSelectedPath != "" {
				if indexOfPath(filtered, state.lastSelectedPath) >= 0 {
					targetPath = state.lastSelectedPath
				}
			}
			break
		}

		if !a.sectionHasFiles(candidateSection) {
			continue
		}
		targetSection = candidateSection
		if state := a.sectionState(candidateSection); state != nil {
			targetPath = state.lastSelectedPath
			if targetPath == "" && len(state.orderedFilePaths) > 0 {
				targetPath = state.orderedFilePaths[0]
			}
		}
		if targetPath != "" {
			break
		}
	}

	if targetSection == "" || targetPath == "" {
		return
	}

	a.setActiveSection(targetSection)
	a.selectFilePath(targetPath)
	t.RequestFocus(diffFilesTreeID)
}

func (a *Dv) toggleDiffWrap() {
	a.diffHardWrap = !a.diffHardWrap
	if a.diffViewState != nil {
		a.diffViewState.ScrollX.Set(0)
	}
}

func (a *Dv) toggleDiffLayoutMode() {
	sourceMode := a.diffLayoutMode
	targetMode := DiffLayoutSideBySide
	if sourceMode == DiffLayoutSideBySide {
		targetMode = DiffLayoutUnified
	}

	sourceOffset := a.currentDiffVerticalOffset()
	targetOffset := 0
	if a.canRestoreToggleLayoutScroll(sourceMode, targetMode, sourceOffset) {
		targetOffset = a.layoutToggleScrollSourceOffset
	} else {
		targetOffset = a.mapDiffVerticalOffsetForLayoutToggle(sourceMode, targetMode, sourceOffset)
	}

	a.rememberToggleLayoutScroll(sourceMode, targetMode, sourceOffset, targetOffset)
	a.diffLayoutMode = targetMode
	a.clampDiffHorizontalScroll()
	a.setDiffVerticalOffset(targetOffset)
}

func (a *Dv) resetSideBySideSplit() {
	if a.diffLayoutMode != DiffLayoutSideBySide || a.diffViewState == nil {
		return
	}
	if a.diffViewState.SideBySideSplitRatio() == 0.5 {
		return
	}
	a.diffViewState.SetSideBySideSplitRatio(0.5)
	a.diffViewState.MarkSideDividerResized()
	a.clampDiffHorizontalScroll()
}

func (a *Dv) shiftSideBySideSplitLeft() {
	a.shiftSideBySideSplit(-1)
}

func (a *Dv) shiftSideBySideSplitRight() {
	a.shiftSideBySideSplit(1)
}

func (a *Dv) shiftSideBySideSplit(delta int) {
	if delta == 0 || a.diffLayoutMode != DiffLayoutSideBySide || a.diffViewState == nil {
		return
	}
	sideBySide := a.diffViewState.SideBySide.Peek()
	if sideBySide == nil {
		return
	}
	viewportWidth := a.diffViewState.ViewportWidth()
	if viewportWidth <= 0 {
		return
	}

	metrics := sideBySideDividerMetrics(viewportWidth, sideBySide, a.diffHideChangeSigns)
	panes := sideBySidePaneLayout(
		viewportWidth,
		sideBySide,
		a.diffHideChangeSigns,
		a.diffViewState.SideBySideSplitRatio(),
	)
	nextOffset := clampInt(panes.DividerX+delta, metrics.MinOffset, metrics.MaxOffset)
	if nextOffset == panes.DividerX {
		return
	}

	ratio := 0.5
	if metrics.Available > 0 {
		ratio = float64(nextOffset) / float64(metrics.Available)
	}
	a.diffViewState.SetSideBySideSplitRatio(ratio)
	a.diffViewState.MarkSideDividerResized()
	a.clampDiffHorizontalScroll()
}

func (a *Dv) currentDiffVerticalOffset() int {
	scrollOffset := 0
	if a.diffScrollState != nil {
		scrollOffset = a.diffScrollState.Offset.Peek()
		if scrollOffset != 0 {
			return scrollOffset
		}
	}
	if a.diffViewState != nil {
		viewOffset := a.diffViewState.ScrollY.Peek()
		if viewOffset != 0 {
			return viewOffset
		}
		return viewOffset
	}
	return scrollOffset
}

func (a *Dv) jumpDiffVertical(delta int) {
	if delta == 0 || a.focusedWidgetID != diffViewerScrollID {
		return
	}
	a.setDiffVerticalOffset(a.currentDiffVerticalOffset() + delta)
}

func diffFileScrollKey(section DiffSection, filePath string) string {
	if filePath == "" {
		return ""
	}
	return string(section) + "\x00" + filePath
}

func diffFileReviewKey(section DiffSection, filePath string) string {
	if filePath == "" {
		return ""
	}
	return string(section) + "\x00" + filePath
}

func (a *Dv) activeReviewTarget() (section DiffSection, filePath string, ok bool) {
	if a.activeKind != DiffTreeNodeFile || a.activeIsDir || a.activePath == "" {
		return "", "", false
	}
	section = a.activeSection
	if a.activeFileSection != "" {
		section = a.activeFileSection
	}
	return section, a.activePath, true
}

func (a *Dv) isReviewed(section DiffSection, filePath string) bool {
	if a.reviewedByFile == nil {
		return false
	}
	key := diffFileReviewKey(section, filePath)
	if key == "" {
		return false
	}
	return a.reviewedByFile[key]
}

func (a *Dv) rememberActiveFileScrollOffset() {
	if a.activeKind != DiffTreeNodeFile || a.activeIsDir || a.activePath == "" {
		return
	}
	section := a.activeSection
	if a.activeFileSection != "" {
		section = a.activeFileSection
	}
	a.rememberFileScrollOffset(section, a.activePath)
}

func (a *Dv) rememberFileScrollOffset(section DiffSection, filePath string) {
	key := diffFileScrollKey(section, filePath)
	if key == "" {
		return
	}
	if a.fileScrollOffsets == nil {
		a.fileScrollOffsets = map[string]fileScrollState{}
	}

	offset := a.currentDiffVerticalOffset()
	offset = a.clampDiffOffsetForViewport(a.diffLayoutMode, offset)
	a.fileScrollOffsets[key] = fileScrollState{
		mode:   a.diffLayoutMode,
		offset: offset,
	}
}

func (a *Dv) restoreFileScrollOffset(filePath string) {
	targetOffset := 0
	if a.fileScrollOffsets != nil {
		key := diffFileScrollKey(a.activeSection, filePath)
		if state, ok := a.fileScrollOffsets[key]; ok {
			targetOffset = state.offset
			if state.mode != a.diffLayoutMode {
				targetOffset = a.mapDiffVerticalOffsetForLayoutToggle(state.mode, a.diffLayoutMode, targetOffset)
			} else {
				targetOffset = a.clampDiffOffsetForLayout(a.diffLayoutMode, targetOffset)
			}
		}
	}
	a.setDiffVerticalOffset(targetOffset)
}

func (a *Dv) setDiffVerticalOffset(offset int) {
	offset = a.clampDiffOffsetForViewport(a.diffLayoutMode, offset)
	if a.diffScrollState != nil {
		a.diffScrollState.Offset.Set(offset)
	}
	if a.diffViewState != nil {
		a.diffViewState.ScrollY.Set(offset)
	}
}

func (a *Dv) clampDiffOffsetForViewport(mode DiffLayoutMode, offset int) int {
	if offset <= 0 {
		return 0
	}

	rows := a.diffLayoutVisualRows(mode)
	if rows <= 0 {
		return 0
	}

	maxOffset := rows - 1
	if a.diffViewState != nil {
		viewportHeight := a.diffViewState.ViewportHeight()
		if viewportHeight > 0 {
			maxOffset = rows - viewportHeight
			if maxOffset < 0 {
				maxOffset = 0
			}
		}
	}
	return clampInt(offset, 0, maxOffset)
}

func (a *Dv) canRestoreToggleLayoutScroll(sourceMode DiffLayoutMode, targetMode DiffLayoutMode, sourceOffset int) bool {
	return a.layoutToggleScrollRestoreValid &&
		a.activePath == a.layoutToggleScrollActivePath &&
		a.activeSection == a.layoutToggleScrollActiveSection &&
		sourceMode == a.layoutToggleScrollTargetMode &&
		targetMode == a.layoutToggleScrollSourceMode &&
		sourceOffset == a.layoutToggleScrollTargetOffset
}

func (a *Dv) rememberToggleLayoutScroll(sourceMode DiffLayoutMode, targetMode DiffLayoutMode, sourceOffset int, targetOffset int) {
	a.layoutToggleScrollRestoreValid = true
	a.layoutToggleScrollSourceMode = sourceMode
	a.layoutToggleScrollTargetMode = targetMode
	a.layoutToggleScrollSourceOffset = sourceOffset
	a.layoutToggleScrollTargetOffset = targetOffset
	a.layoutToggleScrollActivePath = a.activePath
	a.layoutToggleScrollActiveSection = a.activeSection
}

func (a *Dv) mapDiffVerticalOffsetForLayoutToggle(sourceMode DiffLayoutMode, targetMode DiffLayoutMode, sourceOffset int) int {
	if sourceMode == targetMode {
		return a.clampDiffOffsetForLayout(targetMode, sourceOffset)
	}
	if sourceOffset < 0 {
		sourceOffset = 0
	}

	if !a.diffHardWrap {
		anchor, ok := a.diffScrollAnchorForOffset(sourceMode, sourceOffset)
		if ok {
			targetOffset, ok := a.diffOffsetForAnchor(targetMode, anchor)
			if ok {
				return a.clampDiffOffsetForLayout(targetMode, targetOffset)
			}
		}
	}

	return a.mapDiffOffsetByRatio(sourceMode, targetMode, sourceOffset)
}

func (a *Dv) mapDiffOffsetByRatio(sourceMode DiffLayoutMode, targetMode DiffLayoutMode, sourceOffset int) int {
	targetRows := a.diffLayoutVisualRows(targetMode)
	if targetRows <= 0 {
		return 0
	}

	sourceRows := a.diffLayoutVisualRows(sourceMode)
	if sourceRows <= 1 {
		return a.clampDiffOffsetForLayout(targetMode, sourceOffset)
	}

	clampedSource := clampInt(sourceOffset, 0, sourceRows-1)
	numerator := clampedSource*(targetRows-1) + (sourceRows-1)/2
	mapped := numerator / (sourceRows - 1)
	return clampInt(mapped, 0, targetRows-1)
}

func (a *Dv) clampDiffOffsetForLayout(mode DiffLayoutMode, offset int) int {
	rows := a.diffLayoutVisualRows(mode)
	if rows <= 0 {
		return 0
	}
	return clampInt(offset, 0, rows-1)
}

func (a *Dv) diffLayoutVisualRows(mode DiffLayoutMode) int {
	if a.diffViewState == nil {
		return 0
	}

	rendered := a.diffViewState.Rendered.Peek()
	sideBySide := a.diffViewState.SideBySide.Peek()
	if sideBySide == nil && rendered != nil {
		sideBySide = buildSideBySideFromRendered(rendered)
	}

	if mode == DiffLayoutSideBySide {
		if sideBySide == nil || len(sideBySide.Rows) == 0 {
			return 0
		}
		if !a.diffHardWrap {
			return len(sideBySide.Rows)
		}
		viewportWidth := a.diffViewState.ViewportWidth()
		if viewportWidth <= 0 {
			return len(sideBySide.Rows)
		}
		panes := sideBySidePaneLayout(
			viewportWidth,
			sideBySide,
			a.diffHideChangeSigns,
			a.diffViewState.SideBySideSplitRatio(),
		)
		return wrappedSideContentHeight(sideBySide.Rows, panes, viewportWidth)
	}

	if rendered == nil || len(rendered.Lines) == 0 {
		return 0
	}
	if !a.diffHardWrap {
		return len(rendered.Lines)
	}
	viewportWidth := a.diffViewState.ViewportWidth()
	if viewportWidth <= 0 {
		return len(rendered.Lines)
	}
	wrapWidth := max(1, viewportWidth-renderedGutterWidth(rendered, a.diffHideChangeSigns))
	return wrappedContentHeight(rendered.Lines, wrapWidth)
}

func (a *Dv) diffScrollAnchorForOffset(mode DiffLayoutMode, offset int) (diffScrollAnchor, bool) {
	if a.diffViewState == nil {
		return diffScrollAnchor{}, false
	}

	if mode == DiffLayoutSideBySide {
		sideBySide := a.diffViewState.SideBySide.Peek()
		if sideBySide == nil || len(sideBySide.Rows) == 0 {
			return diffScrollAnchor{}, false
		}
		idx := clampInt(offset, 0, len(sideBySide.Rows)-1)
		return diffScrollAnchorForSideRow(sideBySide.Rows[idx])
	}

	rendered := a.diffViewState.Rendered.Peek()
	if rendered == nil || len(rendered.Lines) == 0 {
		return diffScrollAnchor{}, false
	}
	idx := clampInt(offset, 0, len(rendered.Lines)-1)
	line := rendered.Lines[idx]
	return diffScrollAnchor{
		kind:    line.Kind,
		oldLine: line.OldLine,
		newLine: line.NewLine,
	}, true
}

func diffScrollAnchorForSideRow(row SideBySideRenderedRow) (diffScrollAnchor, bool) {
	if row.Shared != nil {
		return diffScrollAnchor{
			kind:    row.Shared.Kind,
			oldLine: row.Shared.OldLine,
			newLine: row.Shared.NewLine,
		}, true
	}

	if row.Left == nil && row.Right == nil {
		return diffScrollAnchor{}, false
	}

	anchor := diffScrollAnchor{
		kind: RenderedLineContext,
	}
	if row.Right != nil {
		anchor.kind = row.Right.Kind
		anchor.newLine = row.Right.LineNumber
	}
	if row.Left != nil {
		if row.Right == nil {
			anchor.kind = row.Left.Kind
		}
		anchor.oldLine = row.Left.LineNumber
	}
	return anchor, true
}

func (a *Dv) diffOffsetForAnchor(mode DiffLayoutMode, anchor diffScrollAnchor) (int, bool) {
	if a.diffViewState == nil {
		return 0, false
	}

	if mode == DiffLayoutSideBySide {
		sideBySide := a.diffViewState.SideBySide.Peek()
		if sideBySide == nil || len(sideBySide.Rows) == 0 {
			return 0, false
		}
		row := findSideBySideRowForAnchor(sideBySide.Rows, anchor)
		if row < 0 {
			return 0, false
		}
		return row, true
	}

	rendered := a.diffViewState.Rendered.Peek()
	if rendered == nil || len(rendered.Lines) == 0 {
		return 0, false
	}
	row := findRenderedRowForAnchor(rendered.Lines, anchor)
	if row < 0 {
		return 0, false
	}
	return row, true
}

func findRenderedRowForAnchor(lines []RenderedDiffLine, anchor diffScrollAnchor) int {
	if len(lines) == 0 {
		return -1
	}

	find := func(match func(RenderedDiffLine) bool) int {
		for idx, line := range lines {
			if match(line) {
				return idx
			}
		}
		return -1
	}

	if anchor.oldLine > 0 && anchor.newLine > 0 {
		if idx := find(func(line RenderedDiffLine) bool {
			return line.OldLine == anchor.oldLine && line.NewLine == anchor.newLine
		}); idx >= 0 {
			return idx
		}
	}

	switch anchor.kind {
	case RenderedLineAdd:
		if anchor.newLine > 0 {
			if idx := find(func(line RenderedDiffLine) bool {
				return line.Kind == RenderedLineAdd && line.NewLine == anchor.newLine
			}); idx >= 0 {
				return idx
			}
		}
	case RenderedLineRemove:
		if anchor.oldLine > 0 {
			if idx := find(func(line RenderedDiffLine) bool {
				return line.Kind == RenderedLineRemove && line.OldLine == anchor.oldLine
			}); idx >= 0 {
				return idx
			}
		}
	case RenderedLineContext:
		if anchor.oldLine > 0 && anchor.newLine > 0 {
			if idx := find(func(line RenderedDiffLine) bool {
				return line.Kind == RenderedLineContext && line.OldLine == anchor.oldLine && line.NewLine == anchor.newLine
			}); idx >= 0 {
				return idx
			}
		}
	}

	if anchor.oldLine > 0 {
		if idx := find(func(line RenderedDiffLine) bool {
			return line.OldLine == anchor.oldLine
		}); idx >= 0 {
			return idx
		}
	}
	if anchor.newLine > 0 {
		if idx := find(func(line RenderedDiffLine) bool {
			return line.NewLine == anchor.newLine
		}); idx >= 0 {
			return idx
		}
	}
	if idx := find(func(line RenderedDiffLine) bool {
		return line.Kind == anchor.kind
	}); idx >= 0 {
		return idx
	}
	return -1
}

func findSideBySideRowForAnchor(rows []SideBySideRenderedRow, anchor diffScrollAnchor) int {
	if len(rows) == 0 {
		return -1
	}

	find := func(match func(diffScrollAnchor) bool) int {
		for idx, row := range rows {
			rowAnchor, ok := diffScrollAnchorForSideRow(row)
			if !ok {
				continue
			}
			if match(rowAnchor) {
				return idx
			}
		}
		return -1
	}

	if anchor.oldLine > 0 && anchor.newLine > 0 {
		if idx := find(func(rowAnchor diffScrollAnchor) bool {
			return rowAnchor.oldLine == anchor.oldLine && rowAnchor.newLine == anchor.newLine
		}); idx >= 0 {
			return idx
		}
	}

	switch anchor.kind {
	case RenderedLineAdd:
		if anchor.newLine > 0 {
			if idx := find(func(rowAnchor diffScrollAnchor) bool {
				return rowAnchor.kind == RenderedLineAdd && rowAnchor.newLine == anchor.newLine
			}); idx >= 0 {
				return idx
			}
		}
	case RenderedLineRemove:
		if anchor.oldLine > 0 {
			if idx := find(func(rowAnchor diffScrollAnchor) bool {
				return rowAnchor.kind == RenderedLineRemove && rowAnchor.oldLine == anchor.oldLine
			}); idx >= 0 {
				return idx
			}
		}
	case RenderedLineContext:
		if anchor.oldLine > 0 && anchor.newLine > 0 {
			if idx := find(func(rowAnchor diffScrollAnchor) bool {
				return rowAnchor.kind == RenderedLineContext && rowAnchor.oldLine == anchor.oldLine && rowAnchor.newLine == anchor.newLine
			}); idx >= 0 {
				return idx
			}
		}
	}

	if anchor.oldLine > 0 {
		if idx := find(func(rowAnchor diffScrollAnchor) bool {
			return rowAnchor.oldLine == anchor.oldLine
		}); idx >= 0 {
			return idx
		}
	}
	if anchor.newLine > 0 {
		if idx := find(func(rowAnchor diffScrollAnchor) bool {
			return rowAnchor.newLine == anchor.newLine
		}); idx >= 0 {
			return idx
		}
	}
	if idx := find(func(rowAnchor diffScrollAnchor) bool {
		return rowAnchor.kind == anchor.kind
	}); idx >= 0 {
		return idx
	}
	return -1
}

func (a *Dv) configureDiffHorizontalScroll() {
	if a.diffScrollState == nil {
		return
	}
	a.diffScrollState.OnScrollLeft = func(cols int) bool {
		return a.scrollDiffHorizontal(-cols)
	}
	a.diffScrollState.OnScrollRight = func(cols int) bool {
		return a.scrollDiffHorizontal(cols)
	}
}

func (a *Dv) scrollDiffHorizontal(delta int) bool {
	if delta == 0 || a.diffHardWrap || a.diffViewState == nil {
		return false
	}
	gutterWidth := a.diffScrollGutterWidth()
	before := a.diffViewState.ScrollX.Peek()
	a.diffViewState.MoveX(delta, gutterWidth)
	return a.diffViewState.ScrollX.Peek() != before
}

func (a *Dv) toggleDiffChangeSigns() {
	a.diffHideChangeSigns = !a.diffHideChangeSigns
	a.clampDiffHorizontalScroll()
}

func (a *Dv) toggleDiffIgnoreWhitespace() {
	if !a.canToggleDiffIgnoreWhitespace() {
		return
	}
	a.diffIgnoreWhitespace = !a.diffIgnoreWhitespace
	a.refreshDiff()
}

func (a *Dv) toggleDiffIntralineStyle() {
	switch a.diffIntralineStyle {
	case IntralineStyleModeBackground:
		a.diffIntralineStyle = IntralineStyleModeUnderline
	case IntralineStyleModeUnderline:
		a.diffIntralineStyle = IntralineStyleModeOff
	default:
		a.diffIntralineStyle = IntralineStyleModeBackground
	}
}

func (a *Dv) toggleActiveFileReviewed() {
	section, filePath, ok := a.activeReviewTarget()
	if !ok {
		return
	}
	if a.reviewedByFile == nil {
		a.reviewedByFile = map[string]bool{}
	}
	key := diffFileReviewKey(section, filePath)
	if key == "" {
		return
	}
	if a.reviewedByFile[key] {
		delete(a.reviewedByFile, key)
		return
	}
	a.reviewedByFile[key] = true
}

func (a *Dv) clearAllReviewed() {
	if len(a.reviewedByFile) == 0 {
		return
	}
	clear(a.reviewedByFile)
}

func (a *Dv) clampDiffHorizontalScroll() {
	if a.diffViewState == nil {
		return
	}
	a.diffViewState.Clamp(a.diffScrollGutterWidth())
}

func (a *Dv) diffScrollGutterWidth() int {
	if a.diffViewState == nil {
		return 0
	}
	if a.diffLayoutMode == DiffLayoutSideBySide {
		return sideBySideStateGutterWidth(
			a.diffViewState.Rendered.Peek(),
			a.diffViewState.SideBySide.Peek(),
			a.diffHideChangeSigns,
			a.diffViewState.ViewportWidth(),
			a.diffViewState.SideBySideSplitRatio(),
		)
	}
	return renderedGutterWidth(a.diffViewState.Rendered.Peek(), a.diffHideChangeSigns)
}

func (a *Dv) toggleSidebar() {
	a.sidebarVisible = !a.sidebarVisible
	if a.sidebarVisible {
		return
	}

	a.dividerHovered = false
	a.dividerFocusRequested = false
	a.dividerFocused = false

	switch a.focusedWidgetID {
	case diffSplitPaneID, diffFilesTreeID, diffFilesFilterID, diffFilesScrollID:
		t.RequestFocus(diffViewerScrollID)
	}
}

func (a *Dv) openTreeFilter() {
	if !a.sidebarVisible {
		a.sidebarVisible = true
		a.dividerFocusRequested = false
		a.dividerFocused = false
	}
	a.treeFilterVisible = true
	if a.treeFilterInput != nil {
		a.treeFilterInput.ClearSelection()
		a.treeFilterInput.CursorEnd()
	}
	t.RequestFocus(diffFilesFilterID)
}

func (a *Dv) handleEscape() {
	if a.clearTreeFilter() {
		return
	}
	if a.focusedWidgetID == diffFilesFilterID && a.treeFilterVisible {
		a.treeFilterVisible = false
		t.RequestFocus(diffFilesTreeID)
	}
}

func (a *Dv) onTreeFilterChange(text string) {
	a.treeFilterVisible = true
	if a.treeFilterState != nil {
		a.treeFilterState.Query.Set(text)
	}
	a.syncTreeFilterSelection()
}

func (a *Dv) clearTreeFilter() bool {
	if a.treeFilterState == nil {
		return false
	}
	if a.treeFilterState.PeekQuery() == "" {
		return false
	}
	if a.treeFilterInput != nil {
		a.treeFilterInput.SetText("")
	}
	a.treeFilterState.Query.Set("")
	a.treeFilterVisible = false
	a.syncTreeFilterSelection()
	t.RequestFocus(diffFilesTreeID)
	return true
}

func (a *Dv) shouldShowTreeFilterInput() bool {
	if a.treeFilterVisible {
		return true
	}
	if a.focusedWidgetID == diffFilesFilterID {
		return true
	}
	if a.treeFilterState == nil {
		return false
	}
	return a.treeFilterState.PeekQuery() != ""
}

func (a *Dv) syncTreeFilterSelection() {
	query := ""
	options := t.FilterOptions{}
	if a.treeFilterState != nil {
		query = a.treeFilterState.PeekQuery()
		options = a.treeFilterState.PeekOptions()
	}
	if query == "" {
		a.treeFilterNoMatches = false
		if a.activeKind == DiffTreeNodeFile {
			if a.treeState.CursorPath.Peek() == nil {
				if treePath, ok := a.filePathToTreePath[a.activePath]; ok {
					a.treeState.CursorPath.Set(clonePath(treePath))
				}
			}
			return
		}
		if !a.switchToFirstSelectableFile(a.activeSection) {
			for _, section := range a.orderedSectionsAfter(a.activeSection) {
				if a.switchToFirstSelectableFile(section) {
					break
				}
			}
		}
		return
	}

	targetSection := DiffSection("")
	filtered := []string(nil)
	for _, section := range a.orderedSectionsFrom(a.activeSection) {
		candidateFiltered := a.filteredFilePathsForSection(section, query, options)
		if len(candidateFiltered) == 0 {
			continue
		}
		targetSection = section
		filtered = candidateFiltered
		break
	}
	if targetSection == "" || len(filtered) == 0 {
		a.setTreeFilterNoMatches()
		return
	}

	a.treeFilterNoMatches = false
	a.setActiveSection(targetSection)
	a.selectFilePath(filtered[0])
}

func (a *Dv) setTreeFilterNoMatches() {
	a.treeFilterNoMatches = true
	a.treeState.CursorPath.Set(nil)
}

func (a *Dv) buildTreeFilterEmptyState(theme t.ThemeData) t.Widget {
	query := ""
	if a.treeFilterState != nil {
		query = a.treeFilterState.PeekQuery()
	}

	message := "No files match the current filter."
	if query != "" {
		message = fmt.Sprintf("No files match %q.", query)
	}

	return t.Column{
		Style: t.Style{
			Width:           t.Flex(1),
			Padding:         t.EdgeInsets{Top: 1, Left: 1, Right: 1},
			BackgroundColor: theme.Background,
		},
		Children: []t.Widget{
			t.Text{
				Content: message,
				Wrap:    t.WrapSoft,
				Style: t.Style{
					ForegroundColor: theme.TextMuted,
					Bold:            true,
				},
			},
			t.Text{
				Content: "Press escape to clear the filter.",
				Wrap:    t.WrapSoft,
				Style: t.Style{
					ForegroundColor: theme.TextMuted,
				},
			},
		},
	}
}

func (a *Dv) focusDivider() {
	if !a.sidebarVisible {
		return
	}
	target := a.dividerReturnTarget()
	a.dividerFocusRequested = true
	a.focusReturnID = target
	t.RequestFocus(diffSplitPaneID)
}

func (a *Dv) focusDividerFromPalette() {
	if !a.sidebarVisible {
		return
	}
	a.dividerFocusRequested = true
	a.focusReturnID = a.dividerReturnTarget()
	if a.commandPalette != nil {
		a.cancelThemePreview()
		a.commandPalette.SetNextFocusIDOnClose(diffSplitPaneID)
		a.commandPalette.Close(false)
	}
}

func (a *Dv) exitDividerFocus() {
	a.dividerFocusRequested = false
	target := a.focusReturnID
	if isInvalidDividerReturnTarget(target) {
		target = diffViewerScrollID
	}
	t.RequestFocus(target)
}

func (a *Dv) togglePalette() {
	if a.commandPalette == nil {
		return
	}
	if a.commandPalette.Visible.Peek() {
		a.cancelThemePreview()
		a.commandPalette.Close(false)
		return
	}
	a.themePreviewBase = ""
	a.themeCursorSynced = false
	a.commandPalette.SetItems(a.commandPaletteItems())
	a.commandPalette.Open()
}

func (a *Dv) openThemePalette() {
	if a.commandPalette == nil {
		return
	}

	a.cancelThemePreview()
	a.commandPalette.Close(false)
	a.themePreviewBase = ""
	a.themeCursorSynced = false
	a.commandPalette.SetItems(a.commandPaletteItems())
	a.commandPalette.Open()
	a.commandPalette.PushLevel(diffThemesPalette, a.themeItems())
	a.setPaletteLevelFilterMode(a.commandPalette.CurrentLevel())
	if item, ok := a.commandPalette.CurrentItem(); ok {
		a.handlePaletteCursorChange(item)
	}
}

func (a *Dv) syncFocusState(ctx t.BuildContext) {
	wasDividerFocused := a.dividerFocused
	focusedID := focusedWidgetID(ctx)
	a.focusedWidgetID = focusedID
	a.dividerFocused = a.sidebarVisible && focusedID == diffSplitPaneID
	if wasDividerFocused && !a.dividerFocused {
		a.dividerFocusRequested = false
	}
	if !a.sidebarVisible {
		a.dividerFocusRequested = false
	}
	if focusedID != "" && focusedID != diffSplitPaneID {
		a.lastNonDividerFocus = focusedID
	}
}

func (a *Dv) dividerReturnTarget() string {
	target := a.lastNonDividerFocus
	if isInvalidDividerReturnTarget(target) {
		target = diffViewerScrollID
	}
	return target
}

// We can't assume that the previous widget that was focused is still available (e.g. command palette).
func isInvalidDividerReturnTarget(target string) bool {
	if target == "" || target == diffSplitPaneID {
		return true
	}
	if target == diffCommandPaletteID {
		return true
	}
	return strings.HasPrefix(target, diffCommandPaletteID+"-")
}

func dividerFocusForeground(theme t.ThemeData) t.ColorProvider {
	return dividerGradient(theme, theme.Accent)
}

func dividerHoverForeground(theme t.ThemeData) t.ColorProvider {
	return dividerGradient(theme, dividerHoverColor(theme))
}

func dividerHoverColor(theme t.ThemeData) t.Color {
	return theme.Accent.WithAlpha(theme.Accent.Alpha() * 0.5)
}

func dividerForeground(theme t.ThemeData) t.ColorProvider {
	return dividerGradient(theme, theme.TextDisabled)
}

func dividerGradient(theme t.ThemeData, center t.Color) t.ColorProvider {
	return t.NewGradient(theme.Background, center, theme.Background).WithAngle(0)
}

func unfocusedTreeCursorColor(theme t.ThemeData) t.Color {
	alpha := theme.ActiveCursor.Alpha()
	if alpha <= 0 {
		alpha = 1.0
	}
	alpha = alpha * 0.35
	if alpha < 0.12 {
		alpha = 0.12
	}
	if alpha > 0.35 {
		alpha = 0.35
	}
	return theme.ActiveCursor.WithAlpha(alpha)
}

func sectionColor(theme t.ThemeData, section DiffSection) t.Color {
	switch section {
	case DiffSectionStaged:
		return theme.Success
	case DiffSectionFiles:
		return theme.Accent
	default:
		return theme.Error
	}
}

func reviewedViewerTitleBackground(theme t.ThemeData) t.ColorProvider {
	return t.NewGradient(
		theme.SuccessBg,
		theme.Background,
	).WithAngle(90)
}

func viewerEmptySpaceBackground(theme t.ThemeData) t.ColorProvider {
	return t.NewGradient(
		theme.Background,
		theme.PrimaryBg,
	).WithAngle(0)
}

func focusedWidgetID(ctx t.BuildContext) string {
	focused := ctx.Focused()
	if focused == nil {
		return ""
	}
	if identifiable, ok := focused.(t.Identifiable); ok {
		return identifiable.WidgetID()
	}
	return ""
}

func (a *Dv) newCommandPalette() *t.CommandPaletteState {
	state := t.NewCommandPaletteState("Commands", a.commandPaletteItems())
	a.setPaletteLevelFilterMode(state.CurrentLevel())
	return state
}

func (a *Dv) setPaletteLevelFilterMode(level *t.CommandPaletteLevel) {
	if level == nil || level.FilterState == nil {
		return
	}
	level.FilterState.Mode.Set(t.FilterFuzzy)
}

func (a *Dv) handlePaletteSelect(item t.CommandPaletteItem) {
	if item.Children != nil {
		title := item.ChildrenTitle
		if title == "" {
			title = item.Label
		}
		if a.commandPalette == nil {
			return
		}
		a.commandPalette.PushLevel(title, item.Children())
		a.setPaletteLevelFilterMode(a.commandPalette.CurrentLevel())
		if current, ok := a.commandPalette.CurrentItem(); ok {
			a.handlePaletteCursorChange(current)
		}
		t.RequestFocus(diffCommandPaletteID + "-input")
		return
	}

	if item.Action != nil {
		item.Action()
	}
}

func (a *Dv) commandPaletteItems() []t.CommandPaletteItem {
	items := []t.CommandPaletteItem{}
	if a.canSwitchSections() {
		items = append(items, t.CommandPaletteItem{
			Label:      "Switch section",
			FilterText: "Switch section staged unstaged files",
			Hint:       a.paletteHint("Switch section"),
			Action:     a.paletteAction(a.switchSectionFocus),
		})
	}
	if a.canToggleStageActiveFile() {
		items = append(items, t.CommandPaletteItem{
			Label:      currentFileStagePaletteLabel(a.activeFileIsStaged()),
			FilterText: "Stage current file unstage current file selected git add restore staged",
			Hint:       a.paletteHint(a.activeFileStageActionName()),
			Action:     a.paletteAction(a.toggleStageActiveFile),
		})
	}
	if a.canStageFiles() {
		items = append(items, t.CommandPaletteItem{
			Label:      "Stage all files",
			FilterText: "Stage all files git add all",
			Hint:       a.paletteHint("Stage all files"),
			Action:     a.paletteAction(a.stageAllFiles),
		})
	}
	if a.canUnstageFiles() {
		items = append(items, t.CommandPaletteItem{
			Label:      "Unstage all files",
			FilterText: "Unstage all files git restore staged reset",
			Hint:       a.paletteHint("Unstage all files"),
			Action:     a.paletteAction(a.unstageAllFiles),
		})
	}
	items = append(items, t.CommandPaletteItem{
		Label:      "Refresh",
		FilterText: "Refresh reload diff",
		Hint:       a.paletteHint("Refresh"),
		Action:     a.paletteAction(a.manualRefresh),
	})
	if a.canCopyActiveFilePath() {
		items = append(items, t.CommandPaletteItem{
			Label:      "Copy path",
			FilterText: "Copy path clipboard file directory",
			Hint:       a.paletteHint("Copy path"),
			Action:     a.paletteAction(a.copyActiveFilePath),
		})
	}
	items = append(items,
		t.CommandPaletteItem{Divider: "Layout"},
		t.CommandPaletteItem{
			Label:      "Toggle sidebar",
			FilterText: "Toggle sidebar layout panel",
			Hint:       a.paletteHint("Toggle sidebar"),
			Action:     a.paletteAction(a.toggleSidebar),
		},
		t.CommandPaletteItem{
			Label:      "Focus divider",
			FilterText: "Focus divider split resize",
			Hint:       a.paletteHint("Focus divider"),
			Action:     a.focusDividerFromPalette,
		},
		t.CommandPaletteItem{Divider: "Appearance"},
		t.CommandPaletteItem{
			Label:      "Toggle line wrap",
			FilterText: "Toggle line wrap hard wrap soft wrap",
			Hint:       a.paletteHint("Toggle line wrap"),
			Action:     a.paletteAction(a.toggleDiffWrap),
		},
		t.CommandPaletteItem{
			Label:      "Toggle split mode",
			FilterText: "Toggle split mode side by side unified layout view",
			Hint:       a.paletteHint("Toggle split"),
			Action:     a.paletteAction(a.toggleDiffLayoutMode),
		},
	)
	if a.diffLayoutMode == DiffLayoutSideBySide {
		items = append(items, t.CommandPaletteItem{
			Label:      "Reset pane split",
			FilterText: "Reset pane split divider even ratio 50 50",
			Action:     a.paletteAction(a.resetSideBySideSplit),
		})
	}

	items = append(items,
		t.CommandPaletteItem{
			Label:      "Toggle +/- symbols",
			FilterText: "Toggle plus minus symbols signs prefixes add remove",
			Action:     a.paletteAction(a.toggleDiffChangeSigns),
		},
	)
	if a.canToggleDiffIgnoreWhitespace() {
		items = append(items, t.CommandPaletteItem{
			Label:      "Toggle ignore whitespace",
			FilterText: "Toggle ignore whitespace differences -w ignore-all-space",
			Hint:       a.paletteHint("Toggle ignore whitespace"),
			Action:     a.paletteAction(a.toggleDiffIgnoreWhitespace),
		})
	}
	items = append(items,
		t.CommandPaletteItem{
			Label:      "Toggle seen",
			FilterText: "Toggle seen mark file seen reviewed done checked",
			Hint:       a.paletteHint("Toggle seen"),
			Action:     a.paletteAction(a.toggleActiveFileReviewed),
		},
		t.CommandPaletteItem{
			Label:      "Clear all seen",
			FilterText: "Clear all seen marks reset seen reviewed",
			Hint:       a.paletteHint("Clear all seen"),
			Action:     a.paletteAction(a.clearAllReviewed),
		},
		t.CommandPaletteItem{
			Label:      "Toggle intraline style",
			FilterText: "Toggle intraline style highlight background underline off disable changed characters",
			Hint:       a.paletteHint("Toggle intraline style"),
			Action:     a.paletteAction(a.toggleDiffIntralineStyle),
		},
		t.CommandPaletteItem{
			Label:         "Theme",
			Hint:          a.paletteHint("Theme menu"),
			ChildrenTitle: diffThemesPalette,
			Children:      a.themeItems,
		},
	)
	return items
}

func currentFileStagePaletteLabel(staged bool) string {
	if staged {
		return "Unstage current file"
	}
	return "Stage current file"
}

func (a *Dv) themeItems() []t.CommandPaletteItem {
	items := make([]t.CommandPaletteItem, 0, len(t.ThemeNames())+2)
	addGroup := func(title string, names []string) {
		if len(names) == 0 {
			return
		}
		items = append(items, t.CommandPaletteItem{Divider: title})
		for _, name := range names {
			label := themeDisplayName(name)
			hint := ""
			if name == t.CurrentThemeName() {
				hint = "current"
			}
			themeName := name
			items = append(items, t.CommandPaletteItem{
				Label:      label,
				FilterText: label + " " + themeName,
				Hint:       hint,
				Data:       themeName,
				Action:     a.setThemeAction(themeName),
			})
		}
	}

	addGroup("Dark themes", t.DarkThemeNames())
	addGroup("Light themes", t.LightThemeNames())

	return items
}

func (a *Dv) setThemeAction(themeName string) func() {
	return func() {
		t.SetTheme(themeName)
		a.commitThemePreview()
		if a.commandPalette != nil {
			a.commandPalette.Close(false)
		}
	}
}

func (a *Dv) paletteAction(action func()) func() {
	return func() {
		if action != nil {
			action()
		}
		a.cancelThemePreview()
		if a.commandPalette != nil {
			a.commandPalette.Close(false)
		}
	}
}

func (a *Dv) handlePaletteCursorChange(item t.CommandPaletteItem) {
	if a.commandPalette == nil {
		return
	}
	level := a.commandPalette.CurrentLevel()
	if level == nil || level.Title != diffThemesPalette {
		a.cancelThemePreview()
		return
	}
	if a.themePreviewBase == "" {
		a.themePreviewBase = t.CurrentThemeName()
	}
	themeName, ok := item.Data.(string)
	if !ok || themeName == "" {
		return
	}
	if !a.themeCursorSynced {
		currentItem, hasCurrent := a.commandPalette.CurrentItem()
		if hasCurrent {
			currentThemeName, _ := currentItem.Data.(string)
			if currentThemeName == themeName {
				a.themeCursorSynced = true
				if selectPaletteTheme(level, t.CurrentThemeName()) {
					return
				}
			}
		}
	}
	t.SetTheme(themeName)
}

func (a *Dv) handlePaletteDismiss() {
	a.cancelThemePreview()
}

func (a *Dv) commitThemePreview() {
	a.finishThemePreview(true)
}

func (a *Dv) cancelThemePreview() {
	a.finishThemePreview(false)
}

func (a *Dv) finishThemePreview(commit bool) {
	if !commit && a.themePreviewBase != "" && t.CurrentThemeName() != a.themePreviewBase {
		t.SetTheme(a.themePreviewBase)
	}
	a.themePreviewBase = ""
	a.themeCursorSynced = false
}

func selectPaletteTheme(level *t.CommandPaletteLevel, themeName string) bool {
	if level == nil || level.ListState == nil || themeName == "" {
		return false
	}
	for idx, item := range level.Items {
		name, ok := item.Data.(string)
		if !ok || name != themeName {
			continue
		}
		if level.ListState.CursorIndex.Peek() == idx {
			return false
		}
		level.ListState.SelectIndex(idx)
		return true
	}
	return false
}

func themeDisplayName(name string) string {
	parts := strings.Split(name, "-")
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func (a *Dv) sidebarSummaryLabel() string {
	percentage, hasSeen := a.sidebarSeenPercentage()
	if !hasSeen {
		return ""
	}
	return fmt.Sprintf("%d%% seen", percentage)
}

func (a *Dv) sidebarHeadingSpans(theme t.ThemeData) []t.Span {
	percentage, hasSeen := a.sidebarSeenPercentage()
	if !hasSeen {
		return nil
	}

	color := theme.TextMuted
	if percentage >= 100 {
		color = theme.Success
	}
	return []t.Span{
		t.StyledSpan(fmt.Sprintf("%d%% seen", percentage), t.SpanStyle{
			Foreground: color,
		}),
	}
}

func (a *Dv) sidebarSeenPercentage() (percentage int, hasSeen bool) {
	seenLines, totalLines, seenFiles := a.sidebarSeenLineTotals()
	if seenFiles == 0 {
		return 0, false
	}
	if totalLines <= 0 {
		return 0, true
	}
	return seenLines * 100 / totalLines, true
}

func (a *Dv) sidebarSeenLineTotals() (seenLines int, totalLines int, seenFiles int) {
	for _, section := range a.sectionOrder {
		state := a.sectionState(section)
		if state == nil {
			continue
		}
		for _, filePath := range state.orderedFilePaths {
			file := state.fileByPath[filePath]
			if file == nil {
				continue
			}
			lineCount := file.Additions + file.Deletions
			totalLines += lineCount
			if !a.isReviewed(section, filePath) {
				continue
			}
			seenFiles++
			seenLines += lineCount
		}
	}
	return seenLines, totalLines, seenFiles
}

func (a *Dv) sidebarTotals() (additions int, deletions int) {
	for _, section := range a.sectionOrder {
		state := a.sectionState(section)
		if state == nil {
			continue
		}
		additions += state.additions
		deletions += state.deletions
	}
	return additions, deletions
}

func (a *Dv) sidebarTotalsSpans(theme t.ThemeData) []t.Span {
	additions, deletions := a.sidebarTotals()
	return nonZeroChangeStatSpans(additions, deletions, theme, true)
}

func (a *Dv) viewerTitle() string {
	switch a.activeKind {
	case DiffTreeNodeSection:
		return a.activeSection.DisplayName() + " changes"
	case DiffTreeNodeDirectory:
		return a.activePath + " (directory)"
	case DiffTreeNodeFile:
		return a.activePath
	}
	if a.activePath == "" {
		if a.loadErr != "" {
			return "Error"
		}
		if a.treeFilterNoMatches {
			return "No matches"
		}
		return "Diff"
	}
	return a.activePath
}

func (a *Dv) emptyMessage() string {
	heading, details := a.emptyMessageParts()
	return heading + "\n\n" + details
}

func (a *Dv) isPipedDiffMode() bool {
	return len(a.sectionOrder) == 1 && a.sectionOrder[0] == DiffSectionFiles
}

func (a *Dv) emptyMessageParts() (heading string, details string) {
	if a.isPipedDiffMode() {
		return "No files in piped diff.", "Run your diff command again and pipe it into dv."
	}
	if a.diffIgnoreWhitespace {
		return "No staged or unstaged changes (ignoring whitespace).", "Whitespace-only changes are hidden. Press x to toggle ignore whitespace."
	}
	return "No staged or unstaged changes.", "Make edits or stage files, then press r to refresh."
}

func (a *Dv) errorMessage() string {
	msg := strings.TrimSpace(a.loadErr)
	if msg == "" {
		msg = "Unknown error"
	}
	if !a.manualRefreshEnabled {
		return "Failed to load git diff:\n\n" + msg + "\n\nRun the command again to retry."
	}
	return "Failed to load git diff:\n\n" + msg + "\n\nPress r to retry."
}

func (a *Dv) filePathsForNavigation() []string {
	if len(a.orderedFilePaths) == 0 {
		return nil
	}
	query := ""
	options := t.FilterOptions{}
	if a.treeFilterState != nil {
		query = a.treeFilterState.PeekQuery()
		options = a.treeFilterState.PeekOptions()
	}
	if query == "" {
		return a.orderedFilePaths
	}
	return a.filteredFilePathsForSection(a.activeSection, query, options)
}

func indexOfPath(paths []string, path string) int {
	if path == "" {
		return -1
	}
	for idx, value := range paths {
		if value == path {
			return idx
		}
	}
	return -1
}

func messageToRendered(title string, text string) *RenderedFile {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	return buildMetaRenderedFile(title, strings.Split(normalized, "\n"))
}

func emptySectionSummaryMessage(section DiffSection) string {
	if section == DiffSectionFiles {
		return "No files in this diff."
	}
	return fmt.Sprintf("No %s files in this diff.", strings.ToLower(section.DisplayName()))
}

func buildSectionSummaryRenderedFile(section DiffSection, state *diffSectionState) *RenderedFile {
	fileCount := 0
	additions := 0
	deletions := 0
	if state != nil {
		fileCount = len(state.orderedFilePaths)
		additions = state.additions
		deletions = state.deletions
	}
	title := section.DisplayName() + " changes"
	lines := []string{
		fmt.Sprintf("Section: %s", section.DisplayName()),
		fmt.Sprintf("Touched files: %d", fileCount),
		fmt.Sprintf("Additions: +%d", additions),
		fmt.Sprintf("Deletions: -%d", deletions),
		"",
		"Use n/p to jump between files in this section.",
	}
	if fileCount == 0 {
		lines = append(lines,
			"",
			emptySectionSummaryMessage(section),
		)
	}
	return buildMetaRenderedFile(title, lines)
}

func buildDirectorySummaryRenderedFile(node DiffTreeNodeData) *RenderedFile {
	path := node.Path
	if path == "" {
		path = node.Name
	}
	if path == "" {
		path = "(root)"
	}
	return buildMetaRenderedFile(path, []string{
		fmt.Sprintf("Section: %s", node.Section.DisplayName()),
		fmt.Sprintf("Directory: %s", path),
		fmt.Sprintf("Touched files: %d", node.TouchedFiles),
		fmt.Sprintf("Additions: +%d", node.Additions),
		fmt.Sprintf("Deletions: -%d", node.Deletions),
		"",
		"Use n/p to jump between changed files.",
	})
}

func collectFilteredTreeFilePaths(nodes []t.TreeNode[DiffTreeNodeData], query string, options t.FilterOptions) []string {
	paths := make([]string, 0)
	appendFilteredTreeFilePaths(nodes, query, options, &paths)
	return paths
}

func appendFilteredTreeFilePaths(nodes []t.TreeNode[DiffTreeNodeData], query string, options t.FilterOptions, paths *[]string) bool {
	hasMatch := false
	for _, node := range nodes {
		childHasMatch := appendFilteredTreeFilePaths(node.Children, query, options, paths)
		matched := t.MatchString(node.Data.Name, query, options).Matched
		if matched || childHasMatch {
			if !node.Data.IsDir && node.Data.Path != "" {
				*paths = append(*paths, node.Data.Path)
			}
			hasMatch = true
		}
	}
	return hasMatch
}

func nonZeroChangeTexts(additions int, deletions int) (addText string, delText string) {
	if additions > 0 {
		addText = fmt.Sprintf("+%d", additions)
	}
	if deletions > 0 {
		delText = fmt.Sprintf("-%d", deletions)
	}
	return addText, delText
}

func nonZeroChangeStatSpans(additions int, deletions int, theme t.ThemeData, bold bool) []t.Span {
	addText, delText := nonZeroChangeTexts(additions, deletions)
	if addText == "" && delText == "" {
		return nil
	}

	spans := make([]t.Span, 0, 3)
	if addText != "" {
		if bold {
			spans = append(spans, t.BoldSpan(addText, theme.Success))
		} else {
			spans = append(spans, t.ColorSpan(addText, theme.Success))
		}
	}
	if delText != "" {
		if len(spans) > 0 {
			spans = append(spans, t.PlainSpan(" "))
		}
		if bold {
			spans = append(spans, t.BoldSpan(delText, theme.Error))
		} else {
			spans = append(spans, t.ColorSpan(delText, theme.Error))
		}
	}
	return spans
}
