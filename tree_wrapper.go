package main

import t "github.com/darrenburns/terma"

// SplitFriendlyTree wraps Tree and removes left/right keybinds so they bubble
// to ancestor widgets (e.g. SplitPane divider controls).
type SplitFriendlyTree struct {
	t.Tree[DiffTreeNodeData]
}

func (s SplitFriendlyTree) Keybinds() []t.Keybind {
	keybinds := s.Tree.Keybinds()
	filtered := make([]t.Keybind, 0, len(keybinds))
	for _, keybind := range keybinds {
		if keybind.Key == "left" || keybind.Key == "right" {
			continue
		}
		filtered = append(filtered, keybind)
	}
	return filtered
}
