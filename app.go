package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	t "github.com/darrenburns/terma"
)

const (
	diffFilesTreeID       = "terma-diff-files-tree"
	diffFilesScrollID     = "terma-diff-files-scroll"
	diffFilesFilterID     = "terma-diff-files-filter"
	diffCommitMessageID   = "terma-diff-commit-message"
	diffMutationStatusID  = "terma-diff-mutation-status"
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
	indexCommandStageAllAndFocusCommit
	indexCommandUnstageAll
	indexCommandCommit
	indexCommandPush
	indexCommandCommitAndPush
)

type indexCommand struct {
	Kind    indexCommandKind
	Path    string
	Message string
}

type mutationState int

const (
	mutationStateRunning mutationState = iota
	mutationStateSuccess
	mutationStateError
)

type mutationStepResult struct {
	Action  string
	Command string
	Stdout  string
	Stderr  string
	Success bool
}

type mutationSessionResult struct {
	Action  string
	State   mutationState
	Summary string
	Steps   []mutationStepResult
}

type indexResult struct {
	Refresh            bool
	ClearCommitMessage bool
	FocusCommitMessage bool
	Session            *mutationSessionResult
}

type diffRefreshRequest struct {
	ignoreWhitespace   bool
	previousSelections map[DiffSection]string
	previousIndices    map[DiffSection]int
	previousActive     DiffSection
	previousActivePath string
}

type diffRefreshResult struct {
	repoRoot           string
	hasRepoRoot        bool
	branch             string
	hasBranch          bool
	sections           map[DiffSection]*diffSectionState
	previousActive     DiffSection
	previousActivePath string
	loadErr            string
}

// Dv is a syntax-highlighted git diff viewer.
type Dv struct {
	provider DiffProvider

	repoRoot string
	branch   string
	loadErr  string
	files    []*DiffFile

	copyPathToClipboard func(string) error
	openURL             func(string) error

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
	commitMessageInput  *t.TextAreaState
	diffScrollState     *t.ScrollState
	diffViewState       *DiffViewState
	refreshTask         *t.Task[struct{}]
	refreshApplied      t.Signal[int]
	initialLoadResolved t.Signal[bool]
	splitState          *t.SplitPaneState
	commandPalette      *t.CommandPaletteState
	indexPendingCount   t.Signal[int]
	indexCommandQueue   chan indexCommand
	refreshGeneration   atomic.Uint64
	mutationStatusNonce atomic.Uint64

	treeFilterVisible       t.Signal[bool]
	treeFilterNoMatches     t.Signal[bool]
	diffLayoutMode          t.Signal[DiffLayoutMode]
	diffHardWrap            t.Signal[bool]
	diffHideChangeSigns     t.Signal[bool]
	diffIntralineStyle      t.Signal[IntralineStyleMode]
	diffIgnoreWhitespace    t.Signal[bool]
	manualRefreshEnabled    bool
	focusedWidgetID         string
	sidebarVisible          t.Signal[bool]
	showMutationStatus      t.Signal[bool]
	showMutationOutput      t.Signal[bool]
	lastMutationSession     t.AnySignal[*mutationSessionResult]
	mutationStatusHideDelay time.Duration
	mutationSpinner         *t.SpinnerState

	dividerFocused        bool
	dividerHovered        t.Signal[bool]
	dividerFocusRequested t.Signal[bool]
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
	reviewedByFile    t.AnySignal[map[string]bool]
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
		provider:                provider,
		renderedByPath:          map[string]*RenderedFile{},
		sideRenderedByPath:      map[string]*SideBySideRenderedFile{},
		fileByPath:              map[string]*DiffFile{},
		filePathToTreePath:      map[string][]int{},
		orderedFilePaths:        []string{},
		sectionOrder:            sectionOrder,
		activeSection:           initialSection,
		initialSection:          initialSection,
		sections:                newDiffSectionStateMap(sectionOrder),
		treeState:               t.NewTreeState([]t.TreeNode[DiffTreeNodeData]{}),
		treeScrollState:         t.NewScrollState(),
		treeFilterState:         t.NewFilterState(),
		treeFilterInput:         t.NewTextInputState(""),
		commitMessageInput:      t.NewTextAreaState(""),
		diffScrollState:         t.NewScrollState(),
		diffViewState:           NewDiffViewState(buildMetaRenderedFile("Diff", []string{"Loading diff..."})),
		refreshTask:             t.NewTask[struct{}](),
		refreshApplied:          t.NewSignal(0),
		initialLoadResolved:     t.NewSignal(false),
		splitState:              t.NewSplitPaneState(0.30),
		indexPendingCount:       t.NewSignal(0),
		indexCommandQueue:       make(chan indexCommand, 256),
		treeFilterVisible:       t.NewSignal(false),
		treeFilterNoMatches:     t.NewSignal(false),
		sidebarVisible:          t.NewSignal(initialState.SidebarVisible),
		showMutationStatus:      t.NewSignal(false),
		showMutationOutput:      t.NewSignal(false),
		lastMutationSession:     t.NewAnySignal((*mutationSessionResult)(nil)),
		diffLayoutMode:          t.NewSignal(initialState.LayoutMode),
		diffHardWrap:            t.NewSignal(false),
		diffHideChangeSigns:     t.NewSignal(!initialState.ShowChangeSigns),
		diffIntralineStyle:      t.NewSignal(initialState.IntralineStyle),
		diffIgnoreWhitespace:    t.NewSignal(initialState.IgnoreWhitespace),
		manualRefreshEnabled:    manualRefreshEnabled,
		mutationStatusHideDelay: 2 * time.Second,
		mutationSpinner:         t.NewSpinnerState(t.SpinnerBraille),
		dividerHovered:          t.NewSignal(false),
		dividerFocusRequested:   t.NewSignal(false),
		lastNonDividerFocus:     diffViewerScrollID,
		focusReturnID:           diffViewerScrollID,
		copyPathToClipboard:     copyPathToClipboardOSC52,
		openURL:                 openURLInBrowser,
		fileScrollOffsets:       map[string]fileScrollState{},
		reviewedByFile:          t.NewAnySignal(map[string]bool{}),
	}
	if app.isPipedDiffMode() {
		app.diffIgnoreWhitespace.Set(false)
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
	a.treeFilterNoMatches.Set(false)
	a.diffViewState.SetRendered(messageToRendered("Error", a.errorMessage()))
	a.diffScrollState.SetOffset(0)
	a.initialLoadResolved.Set(true)
	a.refreshApplied.Update(func(v int) int { return v + 1 })
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
		{Key: "o", Name: "Show last git output", Action: a.toggleMutationOutputViewer, Hidden: true},
		{Key: "y", Name: a.copyActionName(), Action: a.copySelectionOrPath, Hidden: true},
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
			Hidden: true,
		})
	}
	if a.canStageFiles() {
		keybinds = append(keybinds, t.Keybind{
			Key:    "S",
			Name:   "Stage all files",
			Action: a.stageAllFiles,
			Hidden: true,
		})
	}
	if a.canStageFiles() && a.canCommitChanges() {
		keybinds = append(keybinds, t.Keybind{
			Key:    "C",
			Name:   "Stage all & focus commit message",
			Action: a.stageAllAndFocusCommitMessage,
			Hidden: true,
		})
	}
	if a.canUnstageFiles() {
		keybinds = append(keybinds, t.Keybind{
			Key:    "U",
			Name:   "Unstage all files",
			Action: a.unstageAllFiles,
			Hidden: true,
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
	if a.canCommitChanges() {
		keybinds = append(keybinds, t.Keybind{
			Key:    "c",
			Name:   "Focus commit message",
			Action: a.focusCommitMessage,
			Hidden: true,
		})
	}
	if a.canPushCurrentBranch() {
		keybinds = append(keybinds, t.Keybind{
			Key:    "P",
			Name:   "Push current branch",
			Action: a.pushCurrentBranch,
			Hidden: true,
		})
	}
	if a.canOpenPullRequest() {
		keybinds = append(keybinds, t.Keybind{
			Key:    "O",
			Name:   "Open pull request",
			Action: a.openPullRequest,
			Hidden: true,
		})
	}
	return keybinds
}

func (a *Dv) Build(ctx t.BuildContext) t.Widget {
	_ = a.refreshApplied.Get()
	a.syncFocusState(ctx)
	theme := ctx.Theme()
	body := a.buildRightPane(theme)
	if a.sidebarVisible.Get() {
		dividerFg := dividerForeground(theme)
		if a.dividerHovered.Get() {
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
					a.dividerHovered.Set(event.Type == t.HoverEnter)
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
			AllowFocus:     a.dividerFocused || a.dividerFocusRequested.Get(),
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
	if a.treeFilterNoMatches.Get() {
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

	if a.canCommitChanges() {
		commitHeaderBackground := theme.AccentBg
		commitHeaderForeground := theme.AccentText
		commitHeaderText := "Commit [c]"
		commitInput := t.TextArea{
			ID:            diffCommitMessageID,
			State:         a.commitMessageInput,
			Placeholder:   "Write a commit message. Ctrl+Enter to commit. Ctrl+Shift+Enter to commit & push.",
			Highlighter:   commitMessageHighlighter(theme),
			Width:         t.Flex(1),
			OnSubmit:      a.submitCommitMessage,
			ExtraKeybinds: a.commitMessageExtraKeybinds(),
			Style: t.Style{
				Width:           t.Flex(1),
				MinHeight:       t.Cells(3),
				MaxHeight:       t.Percent(50),
				Padding:         t.EdgeInsets{Left: 1, Right: 1},
				BackgroundColor: theme.Surface,
				ForegroundColor: theme.Text,
			},
		}
		if ctx.IsFocused(commitInput) {
			commitHeaderBackground = theme.ActiveCursor
			commitHeaderForeground = theme.SelectionText
			commitHeaderText = "Commit [ctrl+enter]"
		}

		if statusBar, ok := a.buildMutationStatusBar(theme); ok {
			children = append(children, statusBar)
		}
		children = append(children, t.Column{
			Style: t.Style{
				Width: t.Flex(1),
			},
			Children: []t.Widget{
				t.Row{
					Style: t.Style{
						Width:           t.Flex(1),
						Height:          t.Cells(1),
						Padding:         t.EdgeInsets{Left: 1, Right: 1},
						BackgroundColor: commitHeaderBackground,
					},
					Children: []t.Widget{
						t.Text{
							Content: commitHeaderText,
							Style: t.Style{
								ForegroundColor: commitHeaderForeground,
							},
						},
					},
				},
				commitInput,
			},
		})
	}

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
		LayoutMode:      a.diffLayoutMode.Get(),
		HardWrap:        a.diffHardWrap.Get(),
		HideChangeSigns: a.diffHideChangeSigns.Get(),
		IntralineStyle:  a.diffIntralineStyle.Get(),
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
	return a.initialLoadResolved.Get() &&
		a.loadErr == "" &&
		!a.treeFilterNoMatches.Get() &&
		a.activeKind == DiffTreeNodeUnknown &&
		a.totalFileCount() == 0
}

func (a *Dv) buildNonFileInfoCard(theme t.ThemeData) (t.Widget, bool) {
	if a.showMutationOutput.Get() && a.currentMutationSession() != nil {
		return nil, false
	}
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
	if !a.showMutationOutput.Get() {
		if section, path, ok := a.activeReviewTarget(); ok && a.isReviewed(section, path) {
			background = reviewedViewerTitleBackground(theme)
		}
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
			FullPath:      title,
			EllipsisColor: theme.Error,
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
			FullPath:      title,
			EllipsisColor: theme.Error,
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
				FullPath:      title,
				EllipsisColor: theme.Error,
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
		if a.diffIgnoreWhitespace.Get() {
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
	if a.diffLayoutMode.Get() == DiffLayoutSideBySide {
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

func (a *Dv) providerLoadDiff(ctx context.Context, staged bool, ignoreWhitespace bool) (string, error) {
	if provider, ok := a.provider.(ContextDiffProvider); ok {
		return provider.LoadDiffContext(ctx, staged, ignoreWhitespace)
	}
	return a.provider.LoadDiff(staged, ignoreWhitespace)
}

func (a *Dv) providerRepoRoot(ctx context.Context) (string, error) {
	if provider, ok := a.provider.(ContextDiffProvider); ok {
		return provider.RepoRootContext(ctx)
	}
	return a.provider.RepoRoot()
}

func (a *Dv) providerCurrentBranch(ctx context.Context) (string, error) {
	if provider, ok := a.provider.(ContextDiffProvider); ok {
		return provider.CurrentBranchContext(ctx)
	}
	return a.provider.CurrentBranch()
}

func (a *Dv) canStageFiles() bool {
	return a.indexProvider() != nil
}

func (a *Dv) commitProvider() CommitCapable {
	if a.isPipedDiffMode() {
		return nil
	}
	provider, ok := a.provider.(CommitCapable)
	if !ok {
		return nil
	}
	return provider
}

func (a *Dv) canCommitChanges() bool {
	return a.commitProvider() != nil
}

func (a *Dv) pushProvider() PushCapable {
	if a.isPipedDiffMode() {
		return nil
	}
	provider, ok := a.provider.(PushCapable)
	if !ok {
		return nil
	}
	return provider
}

func (a *Dv) pullRequestURLProvider() PullRequestURLCapable {
	if a.isPipedDiffMode() {
		return nil
	}
	provider, ok := a.provider.(PullRequestURLCapable)
	if !ok {
		return nil
	}
	return provider
}

func (a *Dv) canPushChanges() bool {
	return a.pushProvider() != nil && strings.TrimSpace(a.branch) != ""
}

func (a *Dv) mutationRunning() bool {
	return a.indexPendingCount.Peek() > 0
}

func (a *Dv) canPushCurrentBranch() bool {
	return a.canPushChanges() && !a.mutationRunning()
}

func (a *Dv) canOpenPullRequest() bool {
	return a.pullRequestURLProvider() != nil && strings.TrimSpace(a.branch) != ""
}

func (a *Dv) canCommitAndPush() bool {
	if !a.canPushCurrentBranch() || a.commitMessageInput == nil {
		return false
	}
	return strings.TrimSpace(a.commitMessageInput.GetText()) != ""
}

func (a *Dv) hasMutationSession() bool {
	return a.currentMutationSession() != nil
}

func (a *Dv) currentMutationSession() *mutationSessionResult {
	return a.lastMutationSession.Peek()
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

func (a *Dv) hasCopyableDiffSelection() bool {
	return a.diffViewState != nil && a.diffViewState.HasSelection()
}

func (a *Dv) canCopySelectionOrPath() bool {
	return a.hasCopyableDiffSelection() || a.canCopyActiveFilePath()
}

func (a *Dv) copyActionName() string {
	if a.hasCopyableDiffSelection() {
		return "Copy selection"
	}
	return "Copy path"
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

func (a *Dv) stageAllAndFocusCommitMessage() {
	a.enqueueIndexCommand(indexCommand{Kind: indexCommandStageAllAndFocusCommit})
}

func (a *Dv) unstageAllFiles() {
	a.enqueueIndexCommand(indexCommand{Kind: indexCommandUnstageAll})
}

func (a *Dv) enqueueIndexCommand(command indexCommand) {
	switch command.Kind {
	case indexCommandCommit:
		if !a.canCommitChanges() {
			return
		}
	case indexCommandPush:
		if !a.canPushCurrentBranch() {
			return
		}
	case indexCommandCommitAndPush:
		if !a.canCommitAndPush() {
			return
		}
	default:
		if a.indexProvider() == nil {
			return
		}
	}
	a.indexPendingCount.Update(func(count int) int { return count + 1 })
	a.indexCommandQueue <- command
}

func (a *Dv) runIndexCommandQueue() {
	for command := range a.indexCommandQueue {
		result := a.executeIndexCommand(command, func(session mutationSessionResult) {
			sessionCopy := cloneMutationSession(&session)
			t.Dispatch(func() {
				a.setMutationSession(sessionCopy)
			})
		})
		t.Dispatch(func() {
			if result.Session != nil {
				a.setMutationSession(result.Session)
			}
			if result.ClearCommitMessage && a.commitMessageInput != nil {
				a.commitMessageInput.SetText("")
				a.returnFocusToTreeAfterCommit()
			}
			if result.Refresh {
				a.refreshDiff()
			}
			if result.FocusCommitMessage {
				a.focusCommitMessageAfterStageAll()
			}
			a.indexPendingCount.Update(func(count int) int {
				if count <= 0 {
					return 0
				}
				return count - 1
			})
		})
	}
}

func (a *Dv) executeIndexCommand(command indexCommand, report func(mutationSessionResult)) indexResult {
	switch command.Kind {
	case indexCommandStagePath:
		report(mutationSessionResult{Action: "stage", State: mutationStateRunning, Summary: "Staging..."})
		step := a.stagePathMutationStep(command.Path)
		return indexResult{Refresh: true, Session: singleStepSession("stage", step, "Staged file", "Stage failed")}
	case indexCommandUnstagePath:
		report(mutationSessionResult{Action: "unstage", State: mutationStateRunning, Summary: "Unstaging..."})
		step := a.unstagePathMutationStep(command.Path)
		return indexResult{Refresh: true, Session: singleStepSession("unstage", step, "Unstaged file", "Unstage failed")}
	case indexCommandStageAll:
		report(mutationSessionResult{Action: "stage", State: mutationStateRunning, Summary: "Staging..."})
		step := a.stageAllMutationStep()
		return indexResult{Refresh: true, Session: singleStepSession("stage", step, "Staged all files", "Stage failed")}
	case indexCommandStageAllAndFocusCommit:
		report(mutationSessionResult{Action: "stage", State: mutationStateRunning, Summary: "Staging..."})
		step := a.stageAllMutationStep()
		return indexResult{
			Refresh:            true,
			FocusCommitMessage: step.Success,
			Session:            singleStepSession("stage", step, "Staged all files", "Stage failed"),
		}
	case indexCommandUnstageAll:
		report(mutationSessionResult{Action: "unstage", State: mutationStateRunning, Summary: "Unstaging..."})
		step := a.unstageAllMutationStep()
		return indexResult{Refresh: true, Session: singleStepSession("unstage", step, "Unstaged all files", "Unstage failed")}
	case indexCommandCommit:
		report(mutationSessionResult{Action: "commit", State: mutationStateRunning, Summary: "Committing..."})
		step := a.commitMutationStep(command.Message)
		return indexResult{
			Refresh:            true,
			ClearCommitMessage: step.Success,
			Session:            singleStepSession("commit", step, "Committed", "Commit failed"),
		}
	case indexCommandPush:
		report(mutationSessionResult{Action: "push", State: mutationStateRunning, Summary: "Pushing..."})
		step := a.pushMutationStep()
		return indexResult{Refresh: true, Session: singleStepSession("push", step, "Pushed", "Push failed")}
	case indexCommandCommitAndPush:
		report(mutationSessionResult{Action: "commit_and_push", State: mutationStateRunning, Summary: "Committing..."})
		commitStep := a.commitMutationStep(command.Message)
		if !commitStep.Success {
			return indexResult{
				Refresh: true,
				Session: singleStepSession("commit_and_push", commitStep, "Committed", "Commit failed"),
			}
		}
		report(mutationSessionResult{
			Action:  "commit_and_push",
			State:   mutationStateRunning,
			Summary: "Pushing...",
			Steps:   []mutationStepResult{commitStep},
		})
		pushStep := a.pushMutationStep()
		session := &mutationSessionResult{
			Action: "commit_and_push",
			Steps:  []mutationStepResult{commitStep, pushStep},
		}
		if pushStep.Success {
			session.State = mutationStateSuccess
			session.Summary = "Committed and pushed"
		} else {
			session.State = mutationStateError
			session.Summary = "Committed locally, push failed: " + mutationFailureDetail(pushStep)
		}
		return indexResult{
			Refresh:            true,
			ClearCommitMessage: true,
			Session:            session,
		}
	default:
		return indexResult{Refresh: true}
	}
}

func (a *Dv) stagePathMutationStep(path string) mutationStepResult {
	provider := a.indexProvider()
	if provider == nil {
		return mutationStepFromError("Stage file", buildStagePathArgs(path), fmt.Errorf("stage provider unavailable"))
	}
	if contextProvider, ok := provider.(contextMutationOutputProvider); ok {
		return mutationStepFromGitResult("Stage file", contextProvider.stagePathResultContext(context.Background(), path), buildStagePathArgs(path))
	}
	if outputProvider, ok := provider.(mutationOutputProvider); ok {
		return mutationStepFromGitResult("Stage file", outputProvider.stagePathResult(path), buildStagePathArgs(path))
	}
	if contextProvider, ok := provider.(ContextIndexCapable); ok {
		return mutationStepFromError("Stage file", buildStagePathArgs(path), contextProvider.StagePathContext(context.Background(), path))
	}
	return mutationStepFromError("Stage file", buildStagePathArgs(path), provider.StagePath(path))
}

func (a *Dv) stageAllMutationStep() mutationStepResult {
	provider := a.indexProvider()
	if provider == nil {
		return mutationStepFromError("Stage all files", buildStageAllArgs(), fmt.Errorf("stage provider unavailable"))
	}
	if contextProvider, ok := provider.(contextMutationOutputProvider); ok {
		return mutationStepFromGitResult("Stage all files", contextProvider.stageAllResultContext(context.Background()), buildStageAllArgs())
	}
	if outputProvider, ok := provider.(mutationOutputProvider); ok {
		return mutationStepFromGitResult("Stage all files", outputProvider.stageAllResult(), buildStageAllArgs())
	}
	if contextProvider, ok := provider.(ContextIndexCapable); ok {
		return mutationStepFromError("Stage all files", buildStageAllArgs(), contextProvider.StageAllContext(context.Background()))
	}
	return mutationStepFromError("Stage all files", buildStageAllArgs(), provider.StageAll())
}

func (a *Dv) unstagePathMutationStep(path string) mutationStepResult {
	provider := a.indexProvider()
	if provider == nil {
		return mutationStepFromError("Unstage file", buildUnstagePathArgs(path), fmt.Errorf("stage provider unavailable"))
	}
	if contextProvider, ok := provider.(contextMutationOutputProvider); ok {
		return mutationStepFromGitResult("Unstage file", contextProvider.unstagePathResultContext(context.Background(), path), buildUnstagePathArgs(path))
	}
	if outputProvider, ok := provider.(mutationOutputProvider); ok {
		return mutationStepFromGitResult("Unstage file", outputProvider.unstagePathResult(path), buildUnstagePathArgs(path))
	}
	if contextProvider, ok := provider.(ContextIndexCapable); ok {
		return mutationStepFromError("Unstage file", buildUnstagePathArgs(path), contextProvider.UnstagePathContext(context.Background(), path))
	}
	return mutationStepFromError("Unstage file", buildUnstagePathArgs(path), provider.UnstagePath(path))
}

func (a *Dv) unstageAllMutationStep() mutationStepResult {
	provider := a.indexProvider()
	if provider == nil {
		return mutationStepFromError("Unstage all files", buildUnstageAllArgs(), fmt.Errorf("stage provider unavailable"))
	}
	if contextProvider, ok := provider.(contextMutationOutputProvider); ok {
		return mutationStepFromGitResult("Unstage all files", contextProvider.unstageAllResultContext(context.Background()), buildUnstageAllArgs())
	}
	if outputProvider, ok := provider.(mutationOutputProvider); ok {
		return mutationStepFromGitResult("Unstage all files", outputProvider.unstageAllResult(), buildUnstageAllArgs())
	}
	if contextProvider, ok := provider.(ContextIndexCapable); ok {
		return mutationStepFromError("Unstage all files", buildUnstageAllArgs(), contextProvider.UnstageAllContext(context.Background()))
	}
	return mutationStepFromError("Unstage all files", buildUnstageAllArgs(), provider.UnstageAll())
}

func (a *Dv) commitMutationStep(message string) mutationStepResult {
	provider := a.commitProvider()
	if provider == nil {
		return mutationStepFromError("Commit", buildCommitMessageArgs(), fmt.Errorf("commit provider unavailable"))
	}
	if contextProvider, ok := provider.(contextMutationOutputProvider); ok {
		return mutationStepFromGitResult("Commit", contextProvider.commitMessageResultContext(context.Background(), message), buildCommitMessageArgs())
	}
	if outputProvider, ok := provider.(mutationOutputProvider); ok {
		return mutationStepFromGitResult("Commit", outputProvider.commitMessageResult(message), buildCommitMessageArgs())
	}
	if contextProvider, ok := provider.(ContextCommitCapable); ok {
		return mutationStepFromError("Commit", buildCommitMessageArgs(), contextProvider.CommitMessageContext(context.Background(), message))
	}
	return mutationStepFromError("Commit", buildCommitMessageArgs(), provider.CommitMessage(message))
}

func (a *Dv) pushMutationStep() mutationStepResult {
	provider := a.pushProvider()
	if provider == nil {
		return mutationStepFromError("Push", buildPushArgs("", false), fmt.Errorf("push provider unavailable"))
	}
	if contextProvider, ok := provider.(contextMutationOutputProvider); ok {
		return mutationStepFromGitResult("Push", contextProvider.pushCurrentBranchResultContext(context.Background()), buildPushArgs("", false))
	}
	if outputProvider, ok := provider.(mutationOutputProvider); ok {
		return mutationStepFromGitResult("Push", outputProvider.pushCurrentBranchResult(), buildPushArgs("", false))
	}
	if contextProvider, ok := provider.(ContextPushCapable); ok {
		return mutationStepFromError("Push", buildPushArgs("", false), contextProvider.PushCurrentBranchContext(context.Background()))
	}
	return mutationStepFromError("Push", buildPushArgs("", false), provider.PushCurrentBranch())
}

func cloneMutationSession(session *mutationSessionResult) *mutationSessionResult {
	if session == nil {
		return nil
	}
	cloned := *session
	if len(session.Steps) > 0 {
		cloned.Steps = append([]mutationStepResult(nil), session.Steps...)
	}
	return &cloned
}

func singleStepSession(action string, step mutationStepResult, successSummary string, failurePrefix string) *mutationSessionResult {
	session := &mutationSessionResult{
		Action: action,
		Steps:  []mutationStepResult{step},
	}
	if step.Success {
		session.State = mutationStateSuccess
		session.Summary = successSummary
		return session
	}
	session.State = mutationStateError
	session.Summary = failurePrefix + ": " + mutationFailureDetail(step)
	return session
}

func mutationStepFromGitResult(action string, result gitMutationResult, fallbackArgs []string) mutationStepResult {
	command := renderGitCommand(result.Args)
	if command == "" {
		command = renderGitCommand(fallbackArgs)
	}
	return mutationStepResult{
		Action:  action,
		Command: command,
		Stdout:  strings.TrimRight(result.Stdout, "\n"),
		Stderr:  strings.TrimRight(result.Stderr, "\n"),
		Success: result.Err == nil,
	}
}

func mutationStepFromError(action string, args []string, err error) mutationStepResult {
	stderr := ""
	if err != nil {
		stderr = err.Error()
	}
	return mutationStepResult{
		Action:  action,
		Command: renderGitCommand(args),
		Stderr:  stderr,
		Success: err == nil,
	}
}

func renderGitCommand(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return "git " + strings.Join(args, " ")
}

func mutationFailureDetail(step mutationStepResult) string {
	for _, candidate := range []string{step.Stderr, step.Stdout} {
		text := strings.TrimSpace(strings.ReplaceAll(candidate, "\r\n", "\n"))
		if text == "" {
			continue
		}
		lines := strings.Split(text, "\n")
		return strings.TrimSpace(lines[0])
	}
	return "unknown error"
}

func (a *Dv) copySelectionOrPath() {
	if a.copyPathToClipboard == nil {
		return
	}
	if a.hasCopyableDiffSelection() {
		if text := a.diffViewState.SelectedText(); text != "" {
			_ = a.copyPathToClipboard(text)
			return
		}
	}
	a.copyActiveFilePath()
}

func (a *Dv) copyActiveFilePath() {
	if !a.canCopyActiveFilePath() || a.copyPathToClipboard == nil {
		return
	}
	_ = a.copyPathToClipboard(a.activePath)
}

func (a *Dv) refreshDiff() {
	a.rememberActiveFileScrollOffset()
	request := a.buildRefreshRequest()
	generation := a.refreshGeneration.Add(1)
	a.refreshTask.Start(func(ctx context.Context) (struct{}, error) {
		result, err := a.loadRefreshResult(ctx, request)
		if err != nil {
			return struct{}{}, err
		}
		if ctx.Err() != nil {
			return struct{}{}, ctx.Err()
		}
		t.Dispatch(func() {
			if generation != a.refreshGeneration.Load() {
				return
			}
			a.applyRefreshResult(result)
		})
		return struct{}{}, nil
	})
}

func (a *Dv) buildRefreshRequest() diffRefreshRequest {
	previousSelections := map[DiffSection]string{}
	previousIndices := map[DiffSection]int{}
	for _, section := range a.sectionOrder {
		state := a.sectionState(section)
		if state == nil || state.lastSelectedPath == "" {
			continue
		}
		previousSelections[section] = state.lastSelectedPath
		if idx := indexOfPath(state.orderedFilePaths, state.lastSelectedPath); idx >= 0 {
			previousIndices[section] = idx
		}
	}
	previousActivePath := ""
	if a.activeKind == DiffTreeNodeFile && a.activePath != "" {
		previousSelections[a.activeSection] = a.activePath
		previousActivePath = a.activePath
		if state := a.sectionState(a.activeSection); state != nil {
			if idx := indexOfPath(state.orderedFilePaths, a.activePath); idx >= 0 {
				previousIndices[a.activeSection] = idx
			}
		}
	}

	previousActive := a.activeSection
	if previousActive == "" || !a.hasSection(previousActive) {
		previousActive = a.initialSection
	}

	return diffRefreshRequest{
		ignoreWhitespace:   a.diffIgnoreWhitespace.Peek(),
		previousSelections: previousSelections,
		previousIndices:    previousIndices,
		previousActive:     previousActive,
		previousActivePath: previousActivePath,
	}
}

func (a *Dv) loadRefreshResult(ctx context.Context, request diffRefreshRequest) (diffRefreshResult, error) {
	result := diffRefreshResult{
		sections:           newDiffSectionStateMap(a.sectionOrder),
		previousActive:     request.previousActive,
		previousActivePath: request.previousActivePath,
	}

	if repoRoot, err := a.providerRepoRoot(ctx); err == nil {
		result.repoRoot = repoRoot
		result.hasRepoRoot = true
	}
	if branch, err := a.providerCurrentBranch(ctx); err == nil {
		result.branch = branch
		result.hasBranch = true
	}

	for idx, section := range a.sectionOrder {
		if ctx.Err() != nil {
			return diffRefreshResult{}, ctx.Err()
		}

		raw, err := a.providerLoadDiff(ctx, section == DiffSectionStaged, request.ignoreWhitespace)
		if err != nil {
			result.loadErr = fmt.Sprintf("%s diff: %v", strings.ToLower(section.DisplayName()), err)
			return result, nil
		}

		doc, err := parseUnifiedDiff(raw)
		if err != nil {
			result.loadErr = fmt.Sprintf("%s parse error: %v", strings.ToLower(section.DisplayName()), err)
			return result, nil
		}

		state := result.sections[section]
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

		if previous, ok := request.previousSelections[section]; ok {
			if _, exists := state.fileByPath[previous]; exists {
				state.lastSelectedPath = previous
			}
		}
		if state.lastSelectedPath == "" {
			if previousIdx, ok := request.previousIndices[section]; ok && len(state.orderedFilePaths) > 0 {
				if previousIdx >= len(state.orderedFilePaths) {
					previousIdx = len(state.orderedFilePaths) - 1
				}
				if previousIdx >= 0 {
					state.lastSelectedPath = state.orderedFilePaths[previousIdx]
				}
			}
		}
		if state.lastSelectedPath == "" && len(state.orderedFilePaths) > 0 {
			state.lastSelectedPath = state.orderedFilePaths[0]
		}

		result.sections[section] = state
	}

	return result, nil
}

func (a *Dv) applyRefreshResult(result diffRefreshResult) {
	a.initialLoadResolved.Set(true)
	if result.hasRepoRoot {
		a.repoRoot = result.repoRoot
	}
	if result.hasBranch {
		a.branch = result.branch
	}
	if result.loadErr != "" {
		a.setLoadError(result.loadErr)
		return
	}

	a.loadErr = ""
	a.sections = result.sections

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
			Children: state.roots,
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
		a.treeFilterNoMatches.Set(false)
		if a.showMutationOutput.Peek() && a.currentMutationSession() != nil {
			a.renderMutationOutputViewer()
		} else {
			a.diffViewState.SetRendered(messageToRendered("Diff", a.emptyMessage()))
		}
		a.diffScrollState.SetOffset(0)
		a.refreshApplied.Update(func(v int) int { return v + 1 })
		return
	}

	targetSectionKey := result.previousActive
	targetPath := ""
	if targetSectionKey == "" || !a.sectionHasFiles(targetSectionKey) {
		if result.previousActivePath != "" {
			if section, ok := a.findSectionForFilePath(result.previousActivePath); ok {
				targetSectionKey = section
				targetPath = result.previousActivePath
			}
		}
		if targetSectionKey == "" || !a.sectionHasFiles(targetSectionKey) {
			targetSectionKey = a.initialSection
		}
	}
	if !a.sectionHasFiles(targetSectionKey) {
		if sectionWithFiles, ok := a.findSectionWithFiles(targetSectionKey); ok {
			targetSectionKey = sectionWithFiles
		}
	}
	a.setActiveSection(targetSectionKey)

	state := a.sectionState(targetSectionKey)
	if state != nil {
		if targetPath == "" {
			targetPath = state.lastSelectedPath
		}
		if targetPath == "" && len(state.orderedFilePaths) > 0 {
			targetPath = state.orderedFilePaths[0]
		}
	}
	if targetPath != "" {
		a.selectFilePathWithoutClosingOutput(targetPath)
	}
	if a.showMutationOutput.Peek() && a.currentMutationSession() != nil {
		a.renderMutationOutputViewer()
	}
	a.syncTreeFilterSelection()
	a.refreshApplied.Update(func(v int) int { return v + 1 })
}

func (a *Dv) findSectionForFilePath(filePath string) (DiffSection, bool) {
	if filePath == "" {
		return "", false
	}
	for _, section := range a.sectionOrder {
		state := a.sectionState(section)
		if state == nil {
			continue
		}
		if _, ok := state.fileByPath[filePath]; ok {
			return section, true
		}
	}
	return "", false
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

	a.closeMutationOutputViewer()
	a.selectFilePath(filePaths[nextIdx])
}

func (a *Dv) treeFilterInputKeybinds() []t.Keybind {
	return []t.Keybind{
		{Key: "up", Action: func() { a.moveFileCursor(-1) }, Hidden: true},
		{Key: "down", Action: func() { a.moveFileCursor(1) }, Hidden: true},
	}
}

func (a *Dv) selectFilePath(filePath string) bool {
	return a.selectFilePathWithOutput(filePath, true)
}

func (a *Dv) selectFilePathWithoutClosingOutput(filePath string) bool {
	return a.selectFilePathWithOutput(filePath, false)
}

func (a *Dv) selectFilePathWithOutput(filePath string, closeOutput bool) bool {
	if filePath == "" {
		return false
	}
	treePath, ok := a.filePathToTreePath[filePath]
	if !ok {
		return false
	}
	if closeOutput {
		a.closeMutationOutputViewer()
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
	if a.focusedWidgetID == diffFilesTreeID {
		a.closeMutationOutputViewer()
	}

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
	a.diffHardWrap.Update(func(v bool) bool { return !v })
	if a.diffViewState != nil {
		a.diffViewState.ScrollX.Set(0)
		a.diffViewState.ClearSelection()
	}
}

func (a *Dv) toggleDiffLayoutMode() {
	sourceMode := a.diffLayoutMode.Get()
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
	a.diffLayoutMode.Set(targetMode)
	a.clampDiffHorizontalScroll()
	a.setDiffVerticalOffset(targetOffset)
	if a.diffViewState != nil {
		a.diffViewState.ClearSelection()
	}
}

func (a *Dv) resetSideBySideSplit() {
	if a.diffLayoutMode.Get() != DiffLayoutSideBySide || a.diffViewState == nil {
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
	if delta == 0 || a.diffLayoutMode.Get() != DiffLayoutSideBySide || a.diffViewState == nil {
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

	metrics := sideBySideDividerMetrics(viewportWidth, sideBySide, a.diffHideChangeSigns.Peek())
	panes := sideBySidePaneLayout(
		viewportWidth,
		sideBySide,
		a.diffHideChangeSigns.Peek(),
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
	key := diffFileReviewKey(section, filePath)
	if key == "" {
		return false
	}
	reviewed := a.reviewedByFile.Get()
	return reviewed[key]
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
	offset = a.clampDiffOffsetForViewport(a.diffLayoutMode.Get(), offset)
	a.fileScrollOffsets[key] = fileScrollState{
		mode:   a.diffLayoutMode.Get(),
		offset: offset,
	}
}

func (a *Dv) restoreFileScrollOffset(filePath string) {
	targetOffset := 0
	if a.fileScrollOffsets != nil {
		key := diffFileScrollKey(a.activeSection, filePath)
		if state, ok := a.fileScrollOffsets[key]; ok {
			targetOffset = state.offset
			if state.mode != a.diffLayoutMode.Get() {
				targetOffset = a.mapDiffVerticalOffsetForLayoutToggle(state.mode, a.diffLayoutMode.Get(), targetOffset)
			} else {
				targetOffset = a.clampDiffOffsetForLayout(a.diffLayoutMode.Get(), targetOffset)
			}
		}
	}
	a.setDiffVerticalOffset(targetOffset)
}

func (a *Dv) setDiffVerticalOffset(offset int) {
	offset = a.clampDiffOffsetForViewport(a.diffLayoutMode.Get(), offset)
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

	if !a.diffHardWrap.Peek() {
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
		if !a.diffHardWrap.Peek() {
			return len(sideBySide.Rows)
		}
		viewportWidth := a.diffViewState.ViewportWidth()
		if viewportWidth <= 0 {
			return len(sideBySide.Rows)
		}
		panes := sideBySidePaneLayout(
			viewportWidth,
			sideBySide,
			a.diffHideChangeSigns.Peek(),
			a.diffViewState.SideBySideSplitRatio(),
		)
		return wrappedSideContentHeight(sideBySide.Rows, panes, viewportWidth)
	}

	if rendered == nil || len(rendered.Lines) == 0 {
		return 0
	}
	if !a.diffHardWrap.Peek() {
		return len(rendered.Lines)
	}
	viewportWidth := a.diffViewState.ViewportWidth()
	if viewportWidth <= 0 {
		return len(rendered.Lines)
	}
	wrapWidth := max(1, viewportWidth-renderedGutterWidth(rendered, a.diffHideChangeSigns.Peek()))
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
	if delta == 0 || a.diffHardWrap.Peek() || a.diffViewState == nil {
		return false
	}
	gutterWidth := a.diffScrollGutterWidth()
	before := a.diffViewState.ScrollX.Peek()
	a.diffViewState.MoveX(delta, gutterWidth)
	return a.diffViewState.ScrollX.Peek() != before
}

func (a *Dv) toggleDiffChangeSigns() {
	a.diffHideChangeSigns.Update(func(v bool) bool { return !v })
	a.clampDiffHorizontalScroll()
}

func (a *Dv) toggleDiffIgnoreWhitespace() {
	if !a.canToggleDiffIgnoreWhitespace() {
		return
	}
	a.diffIgnoreWhitespace.Update(func(v bool) bool { return !v })
	a.refreshDiff()
}

func (a *Dv) toggleDiffIntralineStyle() {
	switch a.diffIntralineStyle.Peek() {
	case IntralineStyleModeBackground:
		a.diffIntralineStyle.Set(IntralineStyleModeUnderline)
	case IntralineStyleModeUnderline:
		a.diffIntralineStyle.Set(IntralineStyleModeOff)
	default:
		a.diffIntralineStyle.Set(IntralineStyleModeBackground)
	}
}

func (a *Dv) toggleActiveFileReviewed() {
	section, filePath, ok := a.activeReviewTarget()
	if !ok {
		return
	}
	key := diffFileReviewKey(section, filePath)
	if key == "" {
		return
	}
	current := a.reviewedByFile.Peek()
	next := make(map[string]bool, len(current))
	for k, v := range current {
		next[k] = v
	}
	if next[key] {
		delete(next, key)
		a.reviewedByFile.Set(next)
		return
	}
	next[key] = true
	a.reviewedByFile.Set(next)
}

func (a *Dv) clearAllReviewed() {
	if len(a.reviewedByFile.Peek()) == 0 {
		return
	}
	a.reviewedByFile.Set(map[string]bool{})
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
	if a.diffLayoutMode.Get() == DiffLayoutSideBySide {
		return sideBySideStateGutterWidth(
			a.diffViewState.Rendered.Peek(),
			a.diffViewState.SideBySide.Peek(),
			a.diffHideChangeSigns.Peek(),
			a.diffViewState.ViewportWidth(),
			a.diffViewState.SideBySideSplitRatio(),
		)
	}
	return renderedGutterWidth(a.diffViewState.Rendered.Peek(), a.diffHideChangeSigns.Peek())
}

func (a *Dv) toggleSidebar() {
	nextVisible := !a.sidebarVisible.Get()
	a.sidebarVisible.Set(nextVisible)
	if nextVisible {
		return
	}

	a.dividerHovered.Set(false)
	a.dividerFocusRequested.Set(false)
	a.dividerFocused = false

	switch a.focusedWidgetID {
	case diffSplitPaneID, diffFilesTreeID, diffFilesFilterID, diffFilesScrollID, diffCommitMessageID:
		t.RequestFocus(diffViewerScrollID)
	}
}

func (a *Dv) openTreeFilter() {
	if !a.sidebarVisible.Get() {
		a.sidebarVisible.Set(true)
		a.dividerFocusRequested.Set(false)
		a.dividerFocused = false
	}
	a.treeFilterVisible.Set(true)
	if a.treeFilterInput != nil {
		a.treeFilterInput.ClearSelection()
		a.treeFilterInput.CursorEnd()
	}
	t.RequestFocus(diffFilesFilterID)
}

func (a *Dv) focusCommitMessage() {
	if !a.canCommitChanges() {
		return
	}
	if a.focusedWidgetID != diffCommitMessageID {
		a.focusReturnID = a.commitReturnTarget()
	}
	if !a.sidebarVisible.Get() {
		a.sidebarVisible.Set(true)
		a.dividerFocusRequested.Set(false)
		a.dividerFocused = false
	}
	if a.commitMessageInput != nil {
		a.commitMessageInput.CursorEnd()
	}
	t.RequestFocus(diffCommitMessageID)
}

func (a *Dv) focusCommitMessageAfterStageAll() {
	if !a.canCommitChanges() {
		return
	}
	if a.focusedWidgetID != diffCommitMessageID {
		a.focusReturnID = a.commitReturnTarget()
	}
	if !a.sidebarVisible.Get() {
		a.sidebarVisible.Set(true)
		a.dividerFocusRequested.Set(false)
		a.dividerFocused = false
	}
	if a.commitMessageInput != nil {
		a.commitMessageInput.CursorEnd()
	}
	a.focusedWidgetID = diffCommitMessageID
	t.RequestFocus(diffCommitMessageID)
}

func (a *Dv) submitCommitMessage(_ string) {
	if !a.canCommitChanges() || a.commitMessageInput == nil {
		return
	}
	message := a.commitMessageInput.GetText()
	if strings.TrimSpace(message) == "" {
		return
	}
	a.enqueueIndexCommand(indexCommand{
		Kind:    indexCommandCommit,
		Message: message,
	})
}

func (a *Dv) submitCommitAndPush() {
	if !a.canCommitAndPush() || a.commitMessageInput == nil {
		return
	}
	message := a.commitMessageInput.GetText()
	if strings.TrimSpace(message) == "" {
		return
	}
	a.enqueueIndexCommand(indexCommand{
		Kind:    indexCommandCommitAndPush,
		Message: message,
	})
}

func (a *Dv) commitMessageExtraKeybinds() []t.Keybind {
	if !a.canPushChanges() {
		return nil
	}
	return []t.Keybind{
		{
			Key:    "ctrl+shift+enter",
			Name:   "Commit & push",
			Action: a.submitCommitAndPush,
		},
	}
}

func (a *Dv) pushCurrentBranch() {
	if !a.canPushCurrentBranch() {
		return
	}
	a.enqueueIndexCommand(indexCommand{Kind: indexCommandPush})
}

func (a *Dv) providerPullRequestURL(ctx context.Context) (string, error) {
	if provider, ok := a.provider.(ContextPullRequestURLCapable); ok {
		return provider.PullRequestURLContext(ctx)
	}
	return a.pullRequestURLProvider().PullRequestURL()
}

func (a *Dv) openPullRequest() {
	if !a.canOpenPullRequest() || a.openURL == nil {
		return
	}
	url, err := a.providerPullRequestURL(context.Background())
	if err != nil || strings.TrimSpace(url) == "" {
		return
	}
	_ = a.openURL(url)
}

func (a *Dv) toggleMutationOutputViewer() {
	if !a.hasMutationSession() {
		return
	}
	if a.showMutationOutput.Peek() {
		a.closeMutationOutputViewer()
		return
	}
	a.showMutationOutput.Set(true)
	a.renderMutationOutputViewer()
}

func (a *Dv) closeMutationOutputViewer() {
	if !a.showMutationOutput.Peek() {
		return
	}
	a.showMutationOutput.Set(false)
	a.renderActiveViewerContent()
}

func (a *Dv) setMutationSession(session *mutationSessionResult) {
	a.lastMutationSession.Set(cloneMutationSession(session))
	a.showMutationStatus.Set(session != nil)
	if a.mutationSpinner != nil {
		if session != nil && session.State == mutationStateRunning {
			a.mutationSpinner.Start()
		} else {
			a.mutationSpinner.Stop()
		}
	}
	nonce := a.mutationStatusNonce.Add(1)
	if session != nil && session.State == mutationStateSuccess {
		delay := a.mutationStatusHideDelay
		time.AfterFunc(delay, func() {
			t.Dispatch(func() {
				currentSession := a.currentMutationSession()
				if a.mutationStatusNonce.Load() != nonce || currentSession == nil || currentSession.State != mutationStateSuccess {
					return
				}
				a.showMutationStatus.Set(false)
			})
		})
	}
	if a.showMutationOutput.Peek() {
		a.renderMutationOutputViewer()
	}
}

func (a *Dv) renderMutationOutputViewer() {
	session := a.currentMutationSession()
	if a.diffViewState == nil || session == nil {
		return
	}
	a.diffViewState.SetRendered(buildMutationOutputRenderedFile(session))
	a.diffScrollState.SetOffset(0)
}

func (a *Dv) renderActiveViewerContent() {
	if a.diffViewState == nil {
		return
	}
	if a.loadErr != "" {
		a.diffViewState.SetRendered(messageToRendered("Error", a.errorMessage()))
		a.diffScrollState.SetOffset(0)
		return
	}
	switch a.activeKind {
	case DiffTreeNodeFile:
		rendered, ok := a.renderedByPath[a.activePath]
		if !ok {
			file := a.fileByPath[a.activePath]
			if file == nil {
				return
			}
			rendered = buildRenderedFile(file)
			a.renderedByPath[a.activePath] = rendered
			a.sideRenderedByPath[a.activePath] = buildSideBySideRenderedFile(file)
		}
		sideRendered := a.sideRenderedByPath[a.activePath]
		if sideRendered == nil {
			sideRendered = buildSideBySideFromRendered(rendered)
		}
		a.diffViewState.SetRenderedPair(rendered, sideRendered)
	case DiffTreeNodeDirectory:
		a.diffViewState.SetRendered(buildDirectorySummaryRenderedFile(DiffTreeNodeData{
			Name:         filepath.Base(a.activePath),
			Path:         a.activePath,
			IsDir:        true,
			Additions:    0,
			Deletions:    0,
			TouchedFiles: 0,
			Section:      a.activeSection,
			NodeKind:     DiffTreeNodeDirectory,
		}))
		if node, ok := a.findDirectoryNode(a.activeSection, a.activePath); ok {
			a.diffViewState.SetRendered(buildDirectorySummaryRenderedFile(node))
		}
	case DiffTreeNodeSection:
		a.diffViewState.SetRendered(buildSectionSummaryRenderedFile(a.activeSection, a.sectionState(a.activeSection)))
	case DiffTreeNodeUnknown:
		a.diffViewState.SetRendered(messageToRendered("Diff", a.emptyMessage()))
	}
	a.diffScrollState.SetOffset(0)
}

func (a *Dv) buildMutationStatusBar(theme t.ThemeData) (t.Widget, bool) {
	session := a.lastMutationSession.Get()
	if session == nil || !a.showMutationStatus.Get() {
		return nil, false
	}

	background := t.ColorProvider(theme.WarningBg)
	foreground := t.ColorProvider(theme.WarningText)
	switch session.State {
	case mutationStateSuccess:
		background = t.ColorProvider(theme.SuccessBg)
		foreground = t.ColorProvider(theme.SuccessText)
	case mutationStateError:
		background = t.ColorProvider(theme.ErrorBg)
		foreground = t.ColorProvider(theme.ErrorText)
	}

	message := strings.TrimSpace(session.Summary)
	if message == "" {
		return nil, false
	}

	return t.Row{
		Style: t.Style{
			Width:           t.Flex(1),
			Padding:         t.EdgeInsetsXY(1, 0),
			BackgroundColor: background,
		},
		Children: func() []t.Widget {
			children := []t.Widget{}
			children = append(children, t.Text{
				Content: message,
				Style: t.Style{
					ForegroundColor: foreground,
				},
			})
			if session.State == mutationStateRunning && a.mutationSpinner != nil {
				children = append(children,
					t.Spacer{Width: t.Flex(1)},
					t.Spinner{
						ID:    diffMutationStatusID,
						State: a.mutationSpinner,
						Style: t.Style{
							ForegroundColor: foreground,
						},
					},
				)
			}
			return children
		}(),
	}, true
}

func (a *Dv) returnFocusToTreeAfterCommit() {
	a.focusReturnID = diffFilesTreeID
	a.focusedWidgetID = diffFilesTreeID
	t.RequestFocus(diffFilesTreeID)
}

func (a *Dv) handleEscape() {
	if a.showMutationOutput.Peek() {
		a.closeMutationOutputViewer()
		return
	}
	if a.focusedWidgetID == diffCommitMessageID {
		a.exitCommitMessageFocus()
		return
	}
	if a.clearTreeFilter() {
		return
	}
	if a.focusedWidgetID == diffFilesFilterID && a.treeFilterVisible.Get() {
		a.treeFilterVisible.Set(false)
		t.RequestFocus(diffFilesTreeID)
	}
}

func (a *Dv) onTreeFilterChange(text string) {
	a.treeFilterVisible.Set(true)
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
	a.treeFilterVisible.Set(false)
	a.syncTreeFilterSelection()
	t.RequestFocus(diffFilesTreeID)
	return true
}

func (a *Dv) commitReturnTarget() string {
	target := a.focusedWidgetID
	if target == diffSplitPaneID {
		target = a.dividerReturnTarget()
	}
	if isInvalidCommitReturnTarget(target) {
		target = a.lastNonDividerFocus
		if target == diffSplitPaneID {
			target = a.dividerReturnTarget()
		}
	}
	if isInvalidCommitReturnTarget(target) {
		target = diffViewerScrollID
	}
	return target
}

func (a *Dv) exitCommitMessageFocus() {
	target := a.focusReturnID
	if isInvalidCommitReturnTarget(target) {
		target = diffViewerScrollID
	}
	t.RequestFocus(target)
}

func (a *Dv) shouldShowTreeFilterInput() bool {
	if a.treeFilterVisible.Get() {
		return true
	}
	if a.focusedWidgetID == diffFilesFilterID {
		return true
	}
	if a.treeFilterState == nil {
		return false
	}
	return a.treeFilterState.QueryText() != ""
}

func (a *Dv) syncTreeFilterSelection() {
	query := ""
	options := t.FilterOptions{}
	if a.treeFilterState != nil {
		query = a.treeFilterState.PeekQuery()
		options = a.treeFilterState.PeekOptions()
	}
	if query == "" {
		a.treeFilterNoMatches.Set(false)
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

	a.treeFilterNoMatches.Set(false)
	a.setActiveSection(targetSection)
	a.selectFilePath(filtered[0])
}

func (a *Dv) setTreeFilterNoMatches() {
	a.treeFilterNoMatches.Set(true)
	a.treeState.CursorPath.Set(nil)
}

func (a *Dv) buildTreeFilterEmptyState(theme t.ThemeData) t.Widget {
	query := ""
	if a.treeFilterState != nil {
		query = a.treeFilterState.QueryText()
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
	if !a.sidebarVisible.Get() {
		return
	}
	target := a.dividerReturnTarget()
	a.dividerFocusRequested.Set(true)
	a.focusReturnID = target
	t.RequestFocus(diffSplitPaneID)
}

func (a *Dv) focusDividerFromPalette() {
	if !a.sidebarVisible.Get() {
		return
	}
	a.dividerFocusRequested.Set(true)
	a.focusReturnID = a.dividerReturnTarget()
	if a.commandPalette != nil {
		a.cancelThemePreview()
		a.commandPalette.SetNextFocusIDOnClose(diffSplitPaneID)
		a.commandPalette.Close(false)
	}
}

func (a *Dv) exitDividerFocus() {
	a.dividerFocusRequested.Set(false)
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
	a.dividerFocused = a.sidebarVisible.Get() && focusedID == diffSplitPaneID
	if wasDividerFocused && !a.dividerFocused {
		a.dividerFocusRequested.Set(false)
	}
	if !a.sidebarVisible.Get() {
		a.dividerFocusRequested.Set(false)
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

func isInvalidCommitReturnTarget(target string) bool {
	if target == "" || target == diffCommitMessageID {
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
	if a.canPushCurrentBranch() {
		items = append(items, t.CommandPaletteItem{
			Label:      "Push current branch",
			FilterText: "Push current branch git push upstream remote publish",
			Hint:       a.paletteHint("Push current branch"),
			Action:     a.paletteAction(a.pushCurrentBranch),
		})
	}
	if a.canOpenPullRequest() {
		items = append(items, t.CommandPaletteItem{
			Label:      "Open pull request",
			FilterText: "Open pull request github pr compare current branch browser",
			Hint:       a.paletteHint("Open pull request"),
			Action:     a.paletteAction(a.openPullRequest),
		})
	}
	if a.canCommitAndPush() {
		items = append(items, t.CommandPaletteItem{
			Label:      "Commit & push",
			FilterText: "Commit and push git commit push current branch",
			Hint:       "[ctrl+shift+enter]",
			Action:     a.paletteAction(a.submitCommitAndPush),
		})
	}
	items = append(items, t.CommandPaletteItem{
		Label:      "Refresh",
		FilterText: "Refresh reload diff",
		Hint:       a.paletteHint("Refresh"),
		Action:     a.paletteAction(a.manualRefresh),
	})
	if a.hasMutationSession() {
		items = append(items, t.CommandPaletteItem{
			Label:      "Show last git output",
			FilterText: "Show last git output stdout stderr command output",
			Hint:       a.paletteHint("Show last git output"),
			Action:     a.paletteAction(a.toggleMutationOutputViewer),
		})
	}
	if a.canCopySelectionOrPath() {
		items = append(items, t.CommandPaletteItem{
			Label:      a.copyActionName(),
			FilterText: "Copy path clipboard file directory selection text diff",
			Hint:       a.paletteHint(a.copyActionName()),
			Action:     a.paletteAction(a.copySelectionOrPath),
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
	if a.diffLayoutMode.Get() == DiffLayoutSideBySide {
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
	if a.showMutationOutput.Get() && a.currentMutationSession() != nil {
		return "Git output"
	}
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
		if a.treeFilterNoMatches.Get() {
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
	if a.diffIgnoreWhitespace.Get() {
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

func buildMutationOutputRenderedFile(session *mutationSessionResult) *RenderedFile {
	lines := []string{
		"Session: " + mutationSessionDisplayName(session.Action),
		"Result: " + mutationStateDisplayName(session.State),
	}
	if summary := strings.TrimSpace(session.Summary); summary != "" {
		lines = append(lines, "Summary: "+summary)
	}

	for idx, step := range session.Steps {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("Step %d: %s", idx+1, step.Action))
		lines = append(lines, "Result: "+mutationStepResultDisplayName(step.Success))
		if strings.TrimSpace(step.Command) != "" {
			lines = append(lines, "Command: "+step.Command)
		}

		stdout := strings.TrimRight(strings.ReplaceAll(step.Stdout, "\r\n", "\n"), "\n")
		stderr := strings.TrimRight(strings.ReplaceAll(step.Stderr, "\r\n", "\n"), "\n")
		if stdout == "" && stderr == "" {
			lines = append(lines, "Output: (no output)")
			continue
		}
		if stdout != "" {
			lines = append(lines, "stdout:")
			lines = append(lines, strings.Split(stdout, "\n")...)
		}
		if stderr != "" {
			lines = append(lines, "stderr:")
			lines = append(lines, strings.Split(stderr, "\n")...)
		}
	}

	return buildMetaRenderedFile("Git output", lines)
}

func mutationSessionDisplayName(action string) string {
	switch action {
	case "commit_and_push":
		return "Commit & Push"
	case "commit":
		return "Commit"
	case "push":
		return "Push"
	case "stage":
		return "Stage"
	case "unstage":
		return "Unstage"
	default:
		return "Git mutation"
	}
}

func mutationStateDisplayName(state mutationState) string {
	switch state {
	case mutationStateSuccess:
		return "Success"
	case mutationStateError:
		return "Error"
	default:
		return "Running"
	}
}

func mutationStepResultDisplayName(success bool) string {
	if success {
		return "Success"
	}
	return "Error"
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
