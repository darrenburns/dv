package main

import (
	"fmt"
	t "github.com/darrenburns/terma"
	"strings"
)

type browsePosition struct{ Y int }
type browsePositionKey struct {
	Path string
	Mode DiffLayoutMode
	Wrap bool
}

func (a *Aside) rememberPosition() {
	if a.active != "" {
		a.positions[a.positionKey()] = browsePosition{Y: a.scroll.Offset.Peek()}
	}
}
func (a *Aside) positionKey() browsePositionKey {
	return browsePositionKey{a.active, a.mode.Peek(), a.hardWrap.Peek()}
}
func (a *Aside) restorePosition(fallback int) {
	if p, ok := a.positions[a.positionKey()]; ok {
		fallback = p.Y
	}
	a.scroll.Offset.Set(fallback)
}
func (a *Aside) toggleLayout() {
	a.rememberPosition()
	y := a.scroll.Offset.Peek()
	a.mode.Update(func(mode DiffLayoutMode) DiffLayoutMode {
		if mode == DiffLayoutUnified {
			return DiffLayoutSideBySide
		}
		return DiffLayoutUnified
	})
	a.renderActive(false)
	a.restorePosition(y)
	a.bump()
}
func (a *Aside) toggleWrap() {
	a.rememberPosition()
	y := a.scroll.Offset.Peek()
	a.hardWrap.Update(func(v bool) bool { return !v })
	a.restorePosition(y)
	if a.hardWrap.Peek() {
		a.status.Set("Line wrap on")
	} else {
		a.status.Set("Line wrap off · left/right to scroll")
	}
}
func (a *Aside) bump() { a.version.Update(func(v int) int { return v + 1 }) }
func (a *Aside) toggleSidebar() {
	a.sidebarVisible.Update(func(v bool) bool { return !v })
	if !a.sidebarVisible.Peek() {
		t.RequestFocus(viewerScrollID)
	}
}
func (a *Aside) openFilter() {
	a.sidebarVisible.Set(true)
	a.filtering.Set(true)
	t.RequestFocus(filterID)
}
func (a *Aside) clearFilter() {
	a.filter.SetText("")
	a.filtering.Set(false)
	a.rebuildFiles()
	a.bump()
	t.RequestFocus(filesTreeID)
}
func (a *Aside) escape() {
	if a.filter.GetText() != "" {
		a.clearFilter()
		return
	}
	a.clearSelection()
}
func (a *Aside) copySelectionOrPath() {
	text := a.view.SelectedText()
	label := "Selected code"
	if text == "" {
		text, label = a.active, "File path"
	}
	if text == "" {
		return
	}
	if err := a.copyFeedback(text); err != nil {
		a.status.Set(err.Error())
		return
	}
	a.status.Set(label + " copied")
}
func (a *Aside) scrollHorizontal(delta int) bool {
	if a.hardWrap.Peek() {
		return false
	}
	moved := false
	for _, section := range a.sections {
		state := section.Code
		if state == nil {
			continue
		}
		before := state.ScrollX.Peek()
		d := DiffView{State: state, LayoutMode: a.mode.Peek(), HideChangeSigns: a.hideSigns.Peek()}
		state.MoveX(delta, d.currentSelectionGutterWidth())
		moved = moved || before != state.ScrollX.Peek()
	}
	return moved
}
func (a *Aside) cycleIntraline() { a.intra = (a.intra + 1) % 3; a.bump() }
func (a *Aside) shiftSplit(delta float64) {
	a.diffRatio.Set(clampSideBySideSplitRatio(a.diffRatio.Peek() + delta))
}
func (a *Aside) markSeen() {
	if file := a.files[a.active]; file != nil {
		if a.isSeen(a.active) {
			delete(a.seen, a.active)
		} else {
			a.seen[a.active] = fileFingerprint(file)
		}
		a.bump()
	}
}
func fileFingerprint(file *DiffFile) string { return digest(fmt.Sprintf("%#v", *file)) }
func (a *Aside) isSeen(path string) bool {
	file := a.files[path]
	return file != nil && a.seen[path] == fileFingerprint(file)
}
func (a *Aside) clearSeen() { a.seen = map[string]string{}; a.bump() }
func (a *Aside) seenProgress() (seen, total int) {
	for _, path := range a.paths {
		total++
		if a.isSeen(path) {
			seen++
		}
	}
	return
}
func (a *Aside) browseKeybinds() []t.Keybind {
	return []t.Keybind{
		{Key: "ctrl+p", Name: "Commands", Action: a.openPalette},
		{Key: "t", Name: "Themes", Hidden: true, Action: a.openThemes},
		{Key: "b", Name: "Toggle sidebar", Hidden: true, Action: a.toggleSidebar},
		{Key: "w", Name: "Toggle line wrap", Hidden: true, Action: a.toggleWrap},
		{Key: "i", Name: "Intraline style", Hidden: true, Action: a.cycleIntraline},
		{Key: "y", Name: "Copy selection/path", Hidden: true, Action: a.copySelectionOrPath},
		{Key: "m", Name: "Mark file seen", Hidden: true, Action: a.markSeen},
		{Key: "M", Name: "Clear seen marks", Hidden: true, Action: a.clearSeen},
		{Key: "ctrl+j", Name: "Next file", Hidden: true, Action: func() { a.nextFile(1) }},
		{Key: "ctrl+k", Name: "Previous file", Hidden: true, Action: func() { a.nextFile(-1) }},
		{Key: "J", Name: "Down ten lines", Hidden: true, Action: func() { a.scroll.ScrollDown(10) }},
		{Key: "K", Name: "Up ten lines", Hidden: true, Action: func() { a.scroll.ScrollUp(10) }},
		{Key: "ctrl+h", Name: "Shift split left", Hidden: true, Action: func() { a.shiftSplit(-.05) }},
		{Key: "ctrl+l", Name: "Shift split right", Hidden: true, Action: func() { a.shiftSplit(.05) }},
		{Key: "d", Name: "Resize sidebar", Hidden: true, Action: func() { a.sidebarVisible.Set(true); a.dividerActive.Set(true); t.RequestFocus("aside-split") }},
	}
}
func themeDisplayName(name string) string {
	parts := strings.Split(name, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}
