package main

import (
	"path"
	"sort"
	"strings"

	t "github.com/darrenburns/terma"
)

// DiffTreeNodeKind describes what kind of sidebar node is being rendered.
type DiffTreeNodeKind int

const (
	DiffTreeNodeUnknown DiffTreeNodeKind = iota
	DiffTreeNodeSection
	DiffTreeNodeDirectory
	DiffTreeNodeFile
)

// DiffTreeNodeData is the tree sidebar model for diff files and directories.
type DiffTreeNodeData struct {
	Name         string
	Path         string
	IsDir        bool
	File         *DiffFile
	Additions    int
	Deletions    int
	TouchedFiles int
	Section      DiffSection
	NodeKind     DiffTreeNodeKind
	NodeKey      string
}

type treeBuildNode struct {
	Name         string
	Path         string
	IsDir        bool
	File         *DiffFile
	Additions    int
	Deletions    int
	TouchedFiles int
	Children     map[string]*treeBuildNode
}

func buildDiffTree(files []*DiffFile) (roots []t.TreeNode[DiffTreeNodeData], filePathToTreePath map[string][]int, orderedFilePaths []string) {
	return buildDiffTreeForSection(DiffSectionUnstaged, files)
}

func buildDiffTreeForSection(section DiffSection, files []*DiffFile) (roots []t.TreeNode[DiffTreeNodeData], filePathToTreePath map[string][]int, orderedFilePaths []string) {
	root := &treeBuildNode{
		IsDir:    true,
		Children: map[string]*treeBuildNode{},
	}

	for _, file := range files {
		if file == nil {
			continue
		}
		insertDiffFileIntoTree(root, file)
	}

	aggregateTreeStats(root)
	roots = buildSortedTreeNodes(section, root.Children)

	filePathToTreePath = map[string][]int{}
	orderedFilePaths = make([]string, 0, len(files))
	walkTreeForFileLookups(roots, nil, filePathToTreePath, &orderedFilePaths)

	return roots, filePathToTreePath, orderedFilePaths
}

func insertDiffFileIntoTree(root *treeBuildNode, file *DiffFile) {
	if root == nil || file == nil {
		return
	}

	filePath := strings.TrimSpace(file.DisplayPath)
	if filePath == "" {
		filePath = "(unknown file)"
	}

	parts := splitDiffPath(filePath)
	if len(parts) == 0 {
		parts = []string{filePath}
	}

	current := root
	currentPath := ""
	for idx, part := range parts {
		if part == "" {
			continue
		}
		if currentPath == "" {
			currentPath = part
		} else {
			currentPath = path.Join(currentPath, part)
		}

		isDir := idx < len(parts)-1
		child := current.Children[part]
		if child == nil {
			child = &treeBuildNode{
				Name:     part,
				Path:     currentPath,
				IsDir:    isDir,
				Children: map[string]*treeBuildNode{},
			}
			if !isDir {
				child.Children = nil
			}
			current.Children[part] = child
		}

		if isDir {
			child.IsDir = true
			child.File = nil
			if child.Children == nil {
				child.Children = map[string]*treeBuildNode{}
			}
		}

		current = child
	}

	current.IsDir = false
	current.File = file
	current.Additions = file.Additions
	current.Deletions = file.Deletions
	current.TouchedFiles = 1
	current.Children = nil
}

func splitDiffPath(filePath string) []string {
	normalized := strings.Trim(strings.ReplaceAll(filePath, "\\", "/"), "/")
	if normalized == "" {
		return nil
	}
	rawParts := strings.Split(normalized, "/")
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		parts = append(parts, part)
	}
	return parts
}

func aggregateTreeStats(node *treeBuildNode) (additions int, deletions int, touched int) {
	if node == nil {
		return 0, 0, 0
	}
	if !node.IsDir || len(node.Children) == 0 {
		if node.File != nil {
			node.TouchedFiles = 1
		}
		return node.Additions, node.Deletions, node.TouchedFiles
	}

	totalAdds := 0
	totalDels := 0
	totalTouched := 0
	for _, child := range node.Children {
		add, del, touchedCount := aggregateTreeStats(child)
		totalAdds += add
		totalDels += del
		totalTouched += touchedCount
	}

	node.Additions = totalAdds
	node.Deletions = totalDels
	node.TouchedFiles = totalTouched
	return node.Additions, node.Deletions, node.TouchedFiles
}

func buildSortedTreeNodes(section DiffSection, children map[string]*treeBuildNode) []t.TreeNode[DiffTreeNodeData] {
	if len(children) == 0 {
		return []t.TreeNode[DiffTreeNodeData]{}
	}

	keys := make([]string, 0, len(children))
	for key := range children {
		keys = append(keys, key)
	}

	sort.SliceStable(keys, func(i, j int) bool {
		left := children[keys[i]]
		right := children[keys[j]]

		if left.IsDir != right.IsDir {
			return left.IsDir && !right.IsDir
		}
		leftName := strings.ToLower(left.Name)
		rightName := strings.ToLower(right.Name)
		if leftName == rightName {
			return left.Name < right.Name
		}
		return leftName < rightName
	})

	nodes := make([]t.TreeNode[DiffTreeNodeData], 0, len(keys))
	for _, key := range keys {
		child := children[key]
		nodeKind := DiffTreeNodeFile
		nodeKey := diffFileNodeKey(section, child.Path)
		if child.IsDir {
			nodeKind = DiffTreeNodeDirectory
			nodeKey = diffDirectoryNodeKey(section, child.Path)
		}
		treeNode := t.TreeNode[DiffTreeNodeData]{
			Data: DiffTreeNodeData{
				Name:         child.Name,
				Path:         child.Path,
				IsDir:        child.IsDir,
				File:         child.File,
				Additions:    child.Additions,
				Deletions:    child.Deletions,
				TouchedFiles: child.TouchedFiles,
				Section:      section,
				NodeKind:     nodeKind,
				NodeKey:      nodeKey,
			},
		}
		if child.IsDir {
			treeNode.Children = buildSortedTreeNodes(section, child.Children)
		} else {
			treeNode.Children = []t.TreeNode[DiffTreeNodeData]{}
		}
		nodes = append(nodes, treeNode)
	}

	return nodes
}

func walkTreeForFileLookups(nodes []t.TreeNode[DiffTreeNodeData], parentPath []int, filePathToTreePath map[string][]int, orderedFilePaths *[]string) {
	for idx, node := range nodes {
		path := append(clonePath(parentPath), idx)
		if node.Data.IsDir {
			walkTreeForFileLookups(node.Children, path, filePathToTreePath, orderedFilePaths)
			continue
		}
		if node.Data.Path == "" {
			continue
		}
		filePathToTreePath[node.Data.Path] = clonePath(path)
		*orderedFilePaths = append(*orderedFilePaths, node.Data.Path)
	}
}

func clonePath(path []int) []int {
	if len(path) == 0 {
		return nil
	}
	cloned := make([]int, len(path))
	copy(cloned, path)
	return cloned
}
