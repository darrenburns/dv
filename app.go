package main

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	t "github.com/darrenburns/terma"
)

type DiffLayoutMode int

const (
	DiffLayoutUnified DiffLayoutMode = iota
	DiffLayoutSideBySide
)

type AsideInitialState struct {
	LayoutMode      DiffLayoutMode
	ThemeName       string
	IntralineStyle  IntralineStyleMode
	HideSidebar     bool
	NoWrap          bool
	HideChangeSigns bool
}

func DefaultAsideInitialState() AsideInitialState {
	return AsideInitialState{ThemeName: t.ThemeNameObsidianTide}
}
func normalizeAsideInitialState(s AsideInitialState) AsideInitialState {
	if s.LayoutMode != DiffLayoutUnified && s.LayoutMode != DiffLayoutSideBySide {
		s.LayoutMode = DiffLayoutUnified
	}
	if _, err := parseThemeName(s.ThemeName); err != nil {
		s.ThemeName = t.ThemeNameObsidianTide
	}
	if s.IntralineStyle < 0 || s.IntralineStyle > IntralineStyleModeOff {
		s.IntralineStyle = IntralineStyleModeBackground
	}
	return s
}

const (
	filesTreeID    = "aside-files"
	viewerID       = "aside-diff"
	viewerScrollID = "aside-scroll"
	editorID       = "aside-comment"
	filterID       = "aside-filter"
)

type Aside struct {
	palette            *t.CommandPaletteState
	themeBeforePreview string
	hardWrap           t.Signal[bool]
	hideSigns          t.Signal[bool]
	sidebarVisible     t.Signal[bool]
	dividerActive      t.Signal[bool]
	dividerHovered     t.Signal[bool]
	positions          map[browsePositionKey]browsePosition
	seen               map[string]string
	diffRatio          t.Signal[float64]
	store              ReviewStore
	review             *Review
	tree               *t.TreeState[DiffTreeNodeData]
	treeScroll         *t.ScrollState
	scroll             *t.ScrollState
	view               *DiffViewState
	split              *t.SplitPaneState
	editor             *t.TextAreaState
	filter             *t.TextInputState
	version            t.Signal[int]
	mode               t.Signal[DiffLayoutMode]
	status             t.Signal[string]
	editing            t.Signal[bool]
	filtering          t.Signal[bool]
	allFiles           t.Signal[bool]
	files              map[string]*DiffFile
	paths              []string
	treePaths          map[string][]int
	active             string
	activeThread       string
	pending            ReviewAnchor
	replying           bool
	intra              IntralineStyleMode
	copyFeedback       func(string) error
	sections           []reviewSection
	threadBounds       map[string]t.Rect
	pendingScroll      string
}

func NewAside(store ReviewStore, r *Review, initial AsideInitialState) *Aside {
	a := &Aside{store: store, review: r, tree: t.NewTreeState[DiffTreeNodeData](nil), treeScroll: t.NewScrollState(), scroll: t.NewScrollState(), view: NewDiffViewState(nil), split: t.NewSplitPaneState(.25), editor: t.NewTextAreaState(""), filter: t.NewTextInputState(""), version: t.NewSignal(0), mode: t.NewSignal(initial.LayoutMode), status: t.NewSignal("Select code, then c to comment"), editing: t.NewSignal(false), filtering: t.NewSignal(false), allFiles: t.NewSignal(false), intra: initial.IntralineStyle, copyFeedback: copyToClipboard, diffRatio: t.NewSignal(.5)}

	a.hardWrap = t.NewSignal(!initial.NoWrap)
	a.hideSigns = t.NewSignal(initial.HideChangeSigns)
	a.sidebarVisible = t.NewSignal(!initial.HideSidebar)
	a.dividerActive, a.dividerHovered = t.NewSignal(false), t.NewSignal(false)
	a.positions, a.seen = map[browsePositionKey]browsePosition{}, map[string]string{}
	a.palette = t.NewCommandPaletteState("Commands", nil)
	a.scroll.OnScrollLeft = func(n int) bool { return a.scrollHorizontal(-n) }
	a.scroll.OnScrollRight = a.scrollHorizontal
	a.rebuildFiles()
	return a
}
func singleLine(s string) string {
	return strings.NewReplacer("\n", "↵", "\r", "", "\t", "⇥").Replace(safeText(s))
}
func safeText(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return '�'
		}
		return r
	}, s)
}
func (a *Aside) apply(r *Review) {
	a.review = r
	a.rebuildFiles()
	a.version.Update(func(v int) int { return v + 1 })
}
func (a *Aside) rebuildFiles() {
	snapshot := a.review.Snapshots[a.review.Current]
	doc, err := snapshotDocument(snapshot)
	if err != nil {
		a.status.Set(err.Error())
		return
	}
	a.files = map[string]*DiffFile{}
	for _, file := range doc.Files {
		a.files[file.DisplayPath] = file
	}
	if a.allFiles.Peek() {
		for _, path := range snapshot.Files {
			if a.files[path] == nil {
				a.files[path] = &DiffFile{DisplayPath: path}
			}
		}
	}
	// Keep outdated threads reachable even after a file disappears from the diff.
	for _, thread := range a.review.Threads {
		p := thread.Anchor.Path
		if a.files[p] == nil {
			a.files[p] = &DiffFile{DisplayPath: p}
		}
	}
	var files []*DiffFile
	query := strings.ToLower(a.filter.GetText())
	for path, file := range a.files {
		if query == "" || t.MatchString(path, query, t.FilterOptions{Mode: t.FilterFuzzy}).Matched {
			files = append(files, file)
		}
	}
	roots, paths, ordered := buildDiffTree(files)
	a.tree.Nodes.Set(roots)
	a.treePaths = paths
	a.paths = ordered
	if _, ok := paths[a.active]; !ok {
		a.active = ""
		if len(ordered) > 0 {
			a.active = ordered[0]
		}
	}
	if p, ok := paths[a.active]; ok {
		a.tree.CursorPath.Set(p)
	} else {
		a.tree.CursorPath.Set([]int{})
	}
	a.renderActive(true)
}
func (a *Aside) renderActive(preserve bool) {
	y := a.scroll.Offset.Peek()
	file := a.files[a.active]
	if file == nil {
		u := buildMetaRenderedFile("Aside", []string{"No files in this view.", "Press r to refresh, / to filter, or a to browse all files."})
		a.setSections(u, buildSideBySideFromRendered(u))
		return
	}
	unified := buildRenderedFileWithIntraline(file, true)
	split := buildSideBySideRenderedFile(file)
	if len(file.Hunks) == 0 {
		message := "No text changes in this snapshot."
		if file.IsBinary {
			message = "Binary file changed. Line comments are unavailable."
		}
		unified = buildMetaRenderedFile(file.DisplayPath, []string{message})
		split = buildSideBySideFromRendered(unified)
	}
	a.setSections(unified, split)
	if preserve {
		a.scroll.Offset.Set(y)
	} else {
		a.scroll.Offset.Set(0)
	}
}
func (a *Aside) selectFile(path string) {
	if a.editing.Peek() {
		if p, ok := a.treePaths[a.active]; ok {
			a.tree.CursorPath.Set(p)
		}
		return
	}
	if path == a.active {
		return
	}
	a.rememberPosition()
	a.active = path
	a.activeThread = ""
	if p, ok := a.treePaths[path]; ok {
		a.tree.CursorPath.Set(p)
	}
	a.renderActive(false)
	a.restorePosition(0)
	a.version.Update(func(v int) int { return v + 1 })
}
func (a *Aside) nextFile(delta int) {
	if len(a.paths) == 0 {
		return
	}
	i := 0
	for n, p := range a.paths {
		if p == a.active {
			i = n
			break
		}
	}
	a.selectFile(a.paths[(i+delta+len(a.paths))%len(a.paths)])
}
func (a *Aside) Keybinds() []t.Keybind {
	if a.editing.Get() || a.filtering.Get() || a.palette.Visible.Get() {
		return nil
	}
	return append(a.browseKeybinds(), []t.Keybind{
		{Key: "q", Name: "Quit", Action: t.Quit},
		{Key: "escape", Name: "Clear selection", Hidden: true, Action: a.escape},
		{Key: "c", Name: "Comment", Action: a.beginComment},
		{Key: "f", Name: "Copy feedback", Action: a.exportFeedback},
		{Key: "r", Name: "Refresh", Action: a.refresh},
		{Key: "v", Name: "Split/unified", Hidden: true, Action: a.toggleLayout},
		{Key: "n", Name: "Next file", Hidden: true, Action: func() { a.nextFile(1) }},
		{Key: "p", Name: "Previous file", Hidden: true, Action: func() { a.nextFile(-1) }},
		{Key: "/", Name: "Filter", Action: a.openFilter},
		{Key: "a", Name: "All/changed files", Hidden: true, Action: a.toggleAllFiles},
		{Key: "]", Name: "Next comment", Hidden: true, Action: func() { a.nextThread(1) }},
		{Key: "[", Name: "Previous comment", Hidden: true, Action: func() { a.nextThread(-1) }},
		{Key: "e", Name: "Reply", Hidden: true, Action: a.beginReply},
		{Key: "x", Name: "Resolve/reopen", Hidden: true, Action: a.toggleResolved},
	}...)
}
func (a *Aside) selectionAnchor() (ReviewAnchor, error) {
	track, start, end, ok := a.view.normalizedSelection()
	if !ok {
		return ReviewAnchor{}, fmt.Errorf("Select a code line or range with the mouse first")
	}
	side := "new"
	if track == DiffSelectionTrackLeft {
		side = "old"
	}
	var oldNums, newNums []int
	add := func(old, new int) {
		if old > 0 {
			oldNums = append(oldNums, old)
		}
		if new > 0 {
			newNums = append(newNums, new)
		}
	}
	// Ending at column zero excludes the last line, except for a single click.
	last := end.Row
	if last > start.Row && end.Grapheme == 0 {
		last--
	}
	if track == DiffSelectionTrackUnified {
		r := a.view.Rendered.Peek()
		for i := start.Row; i <= last && i < len(r.Lines); i++ {
			line := r.Lines[i]
			add(line.OldLine, line.NewLine)
		}
	} else {
		r := a.view.SideBySide.Peek()
		for i := start.Row; i <= last && i < len(r.Rows); i++ {
			row := r.Rows[i]
			if row.Left != nil {
				add(row.Left.LineNumber, 0)
			}
			if row.Right != nil {
				add(0, row.Right.LineNumber)
			}
		}
	}
	nums := newNums
	if side == "old" {
		nums = oldNums
	}
	if track == DiffSelectionTrackUnified && len(newNums) == 0 {
		side = "old"
		nums = oldNums
	}
	if len(nums) == 0 {
		return ReviewAnchor{}, fmt.Errorf("Select code, rather than a header or comment")
	}
	if track == DiffSelectionTrackUnified && len(oldNums) > 0 && len(newNums) > 0 {
		// A replacement spans both sides; ask for an unambiguous side.
		r := a.view.Rendered.Peek()
		for i := start.Row; i <= last && i < len(r.Lines); i++ {
			if r.Lines[i].Kind == RenderedLineRemove {
				return ReviewAnchor{}, fmt.Errorf("Select only one diff side; use split view for replacements")
			}
		}
	}
	return anchorFor(a.review.Snapshots[a.review.Current], a.active, side, nums[0], nums[len(nums)-1])
}
func (a *Aside) beginComment() {
	anchor, err := a.selectionAnchor()
	if err != nil {
		a.status.Set(err.Error())
		return
	}
	a.pending = anchor
	a.replying = false
	a.editor.SetText("")
	a.editing.Set(true)
	a.renderActive(true)
	a.pendingScroll = editorID
	a.version.Update(func(v int) int { return v + 1 })
	t.RequestFocus(editorID)
}
func (a *Aside) cancelComment() {
	a.editing.Set(false)
	a.pendingScroll = ""
	a.renderActive(true)
	a.version.Update(func(v int) int { return v + 1 })
	t.RequestFocus(viewerScrollID)
}
func (a *Aside) saveComment() {
	var r *Review
	var err error
	if a.replying {
		r, err = a.store.Reply(a.review.ID, a.activeThread, "you", a.editor.GetText())
	} else {
		p := a.pending
		r, err = a.store.Publish(a.review.ID, "you", []AnnotationInput{{Key: fmt.Sprintf("draft-%d", time.Now().UnixNano()), Snapshot: p.Snapshot, Path: p.Path, Side: p.Side, Start: p.Start, End: p.End, Kind: "feedback", Body: a.editor.GetText()}}, false)
	}
	if err != nil {
		a.status.Set(err.Error())
		return
	}
	a.editing.Set(false)
	a.apply(r)
	if a.replying {
		a.status.Set("Reply saved to thread")
	} else {
		a.status.Set("Saved locally · f submits and copies feedback")
	}
	t.RequestFocus(viewerScrollID)
}
func (a *Aside) exportFeedback() {
	r, err := a.store.Submit(a.review.ID)
	if err != nil {
		a.status.Set(err.Error())
		return
	}
	a.apply(r)
	if err = a.copyFeedback(feedbackMarkdown(r, 0)); err != nil {
		a.status.Set("Feedback submitted · " + err.Error())
		return
	}
	a.status.Set("Feedback submitted · copied for your agent (also available via aside review feedback)")
}
func (a *Aside) nextThread(delta int) {
	if len(a.review.Threads) == 0 {
		a.status.Set("No comments yet")
		return
	}
	i := -1
	for n, thread := range a.review.Threads {
		if thread.ID == a.activeThread {
			i = n
		}
	}
	i = (i + delta + len(a.review.Threads)) % len(a.review.Threads)
	thread := a.review.Threads[i]
	if _, visible := a.treePaths[thread.Anchor.Path]; !visible {
		a.filter.SetText("")
		a.rebuildFiles()
	}
	a.selectFile(thread.Anchor.Path)
	a.activeThread = thread.ID
	a.renderActive(false)
	a.version.Update(func(v int) int { return v + 1 })
	a.status.Set(fmt.Sprintf("Comment %d/%d", i+1, len(a.review.Threads)))
	a.scrollToThread(thread.ID)
}
func (a *Aside) scrollToThread(id string) {
	a.pendingScroll = id
	if bounds, ok := a.threadBounds[id]; ok {
		a.scroll.Offset.Set(max(0, bounds.Y-2))
		a.pendingScroll = ""
	}
}
func (a *Aside) setSections(unified *RenderedFile, split *SideBySideRenderedFile) {
	a.clearSelection()
	var pending *ReviewAnchor
	if a.editing.Peek() && !a.replying {
		pending = &a.pending
	}
	a.sections = reviewSections(unified, split, a.review, a.active, a.mode.Peek(), pending)
	a.threadBounds = nil
	for _, section := range a.sections {
		if section.Code != nil {
			section.Code.SplitRatio = a.diffRatio
		}
	}
	for _, section := range a.sections {
		if section.Code != nil {
			section.Code.SplitRatio = a.diffRatio
			a.view = section.Code
			break
		}
	}
}

func (a *Aside) beginReply() {
	if a.editing.Peek() {
		return
	}
	for _, thread := range a.review.Threads {
		if thread.ID == a.activeThread {
			a.pending = thread.Anchor
			a.replying = true
			a.editor.SetText("")
			a.editing.Set(true)
			a.pendingScroll = thread.ID
			a.version.Update(func(v int) int { return v + 1 })
			t.RequestFocus(editorID)
			return
		}
	}
	a.status.Set("Use [ or ] to select a comment first")
}
func (a *Aside) toggleResolved() {
	for _, thread := range a.review.Threads {
		if thread.ID == a.activeThread {
			status := "resolved"
			if thread.Status == status {
				status = "open"
			}
			r, err := a.store.SetStatus(a.review.ID, thread.ID, status)
			if err != nil {
				a.status.Set(err.Error())
				return
			}
			a.apply(r)
			a.status.Set("Comment " + status)
			return
		}
	}
	a.status.Set("Use [ or ] to select a comment first")
}
func (a *Aside) refresh() {
	// Refresh the review store in all modes; only working changes are live.
	if a.review.Source != "worktree" {
		r, err := a.store.Read(a.review.ID)
		if err != nil {
			a.status.Set(err.Error())
			return
		}
		a.apply(r)
		a.status.Set("Review reloaded (pinned snapshot)")
		return
	}
	r, err := openReview(a.store, a.review.Repo, "", "", "")
	if err != nil {
		a.status.Set(err.Error())
		return
	}
	a.apply(r)
	a.status.Set("Refreshed · comments on earlier snapshots are marked outdated")
}

// Store polling reads in the background; every UI mutation runs on Terma's
// event loop. It never alters selection while the user is composing or dragging.
func (a *Aside) Watch(ctx context.Context) {
	id := a.review.ID
	go func() {
		ticker := time.NewTicker(700 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r, err := a.store.Read(id)
				if err != nil {
					continue
				}
				t.Dispatch(func() {
					a.receiveReview(r)
				})
			}
		}
	}()
}

// Defer incoming updates until the user finishes a selection or composition.
// Polling retries the same revision, so no update is lost.
func (a *Aside) receiveReview(r *Review) {
	if r.Revision > a.review.Revision && !a.editing.Peek() && a.view.selectionAnchor.Peek() == nil {
		a.apply(r)
		a.status.Set("Review updated by another client")
	}
}
