package main

import t "github.com/darrenburns/terma"

// FocusAwareSplitPane makes SplitPane focus opt-in for tab traversal.
type FocusAwareSplitPane struct {
	t.SplitPane
	AllowFocus     bool
	EnableKeybinds bool
}

func (s FocusAwareSplitPane) IsFocusable() bool {
	return s.AllowFocus && s.SplitPane.IsFocusable()
}

func (s FocusAwareSplitPane) Keybinds() []t.Keybind {
	if !s.EnableKeybinds {
		return nil
	}
	keybinds := s.SplitPane.Keybinds()
	for i := range keybinds {
		switch keybinds[i].Key {
		case "left", "h":
			keybinds[i].Name = "Shrink sidebar"
			keybinds[i].Hidden = true
		case "right", "l":
			keybinds[i].Name = "Grow sidebar"
			keybinds[i].Hidden = true
		}
	}
	return keybinds
}
