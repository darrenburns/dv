package main

// StdinDiffProvider serves a single diff payload captured from stdin.
type StdinDiffProvider struct {
	WorkDir string
	Diff    string
}

func (p StdinDiffProvider) LoadDiff(staged bool) (string, error) {
	if staged {
		return "", nil
	}
	return p.Diff, nil
}

func (p StdinDiffProvider) RepoRoot() (string, error) {
	return GitDiffProvider{WorkDir: p.WorkDir}.RepoRoot()
}

func (p StdinDiffProvider) CurrentBranch() (string, error) {
	return GitDiffProvider{WorkDir: p.WorkDir}.CurrentBranch()
}

func (p StdinDiffProvider) Sections() []DiffSection {
	return []DiffSection{DiffSectionFiles}
}

func (p StdinDiffProvider) ManualRefreshEnabled() bool {
	return false
}
