package main

import "fmt"

// DiffSection identifies which git diff space a node belongs to.
type DiffSection string

const (
	DiffSectionUnstaged DiffSection = "unstaged"
	DiffSectionStaged   DiffSection = "staged"
)

func allDiffSections() []DiffSection {
	return []DiffSection{DiffSectionUnstaged, DiffSectionStaged}
}

func (s DiffSection) Opposite() DiffSection {
	if s == DiffSectionStaged {
		return DiffSectionUnstaged
	}
	return DiffSectionStaged
}

func (s DiffSection) DisplayName() string {
	if s == DiffSectionStaged {
		return "Staged"
	}
	return "Unstaged"
}

func diffSectionRootNodeKey(section DiffSection) string {
	return fmt.Sprintf("%s::section", section)
}

func diffFileNodeKey(section DiffSection, path string) string {
	return fmt.Sprintf("%s::%s", section, path)
}

func diffDirectoryNodeKey(section DiffSection, path string) string {
	return fmt.Sprintf("%s::dir::%s", section, path)
}
