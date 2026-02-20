package main

import "fmt"

// DiffSection identifies which git diff space a node belongs to.
type DiffSection string

const (
	DiffSectionUnstaged DiffSection = "unstaged"
	DiffSectionStaged   DiffSection = "staged"
	DiffSectionFiles    DiffSection = "files"
)

func defaultDiffSections() []DiffSection {
	return []DiffSection{DiffSectionUnstaged, DiffSectionStaged}
}

func allDiffSections() []DiffSection {
	return defaultDiffSections()
}

func normalizeDiffSections(sections []DiffSection) []DiffSection {
	if len(sections) == 0 {
		return defaultDiffSections()
	}

	seen := map[DiffSection]bool{}
	normalized := make([]DiffSection, 0, len(sections))
	for _, section := range sections {
		if !isKnownDiffSection(section) || seen[section] {
			continue
		}
		seen[section] = true
		normalized = append(normalized, section)
	}
	if len(normalized) == 0 {
		return defaultDiffSections()
	}
	return normalized
}

func isKnownDiffSection(section DiffSection) bool {
	switch section {
	case DiffSectionUnstaged, DiffSectionStaged, DiffSectionFiles:
		return true
	default:
		return false
	}
}

func (s DiffSection) Opposite() DiffSection {
	if s == DiffSectionStaged {
		return DiffSectionUnstaged
	}
	if s == DiffSectionUnstaged {
		return DiffSectionStaged
	}
	if s == DiffSectionFiles {
		return DiffSectionFiles
	}
	return DiffSectionStaged
}

func (s DiffSection) DisplayName() string {
	if s == DiffSectionStaged {
		return "Staged"
	}
	if s == DiffSectionFiles {
		return "Files"
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
