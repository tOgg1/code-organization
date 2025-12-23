package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/tormodhaugland/co/internal/template"
)

// TestBuildSourceTree tests the basic tree building functionality.
func TestBuildSourceTree(t *testing.T) {
	tmp := t.TempDir()

	// Create a simple directory structure
	dirs := []string{
		"project1",
		"project2",
		"project2/subdir",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(tmp, d), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	// Create some files
	files := []string{
		"readme.txt",
		"project1/main.go",
		"project2/index.js",
		"project2/subdir/helper.js",
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(tmp, f), []byte("content"), 0o644); err != nil {
			t.Fatalf("write %s: %v", f, err)
		}
	}

	root, err := buildSourceTree(tmp, false)
	if err != nil {
		t.Fatalf("buildSourceTree: %v", err)
	}

	if root == nil {
		t.Fatal("expected non-nil root")
	}

	if !root.IsDir {
		t.Error("root should be a directory")
	}

	if !root.IsExpanded {
		t.Error("root should be expanded by default")
	}

	if root.Depth != 0 {
		t.Errorf("root depth should be 0, got %d", root.Depth)
	}

	// Check children were loaded
	if len(root.Children) == 0 {
		t.Error("expected root to have children")
	}

	// Find project1 and project2
	var project1, project2 *sourceNode
	for _, child := range root.Children {
		switch child.Name {
		case "project1":
			project1 = child
		case "project2":
			project2 = child
		}
	}

	if project1 == nil {
		t.Error("project1 not found in children")
	} else {
		if !project1.IsDir {
			t.Error("project1 should be a directory")
		}
		if project1.Depth != 1 {
			t.Errorf("project1 depth should be 1, got %d", project1.Depth)
		}
	}

	if project2 == nil {
		t.Error("project2 not found in children")
	}
}

// TestBuildSourceTreeWithGitRepo tests git repo detection in tree building.
func TestBuildSourceTreeWithGitRepo(t *testing.T) {
	tmp := t.TempDir()

	// Create a directory structure with a git repo
	repoDir := filepath.Join(tmp, "my-repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}

	// Create a .git directory to mark it as a git repo
	gitDir := filepath.Join(repoDir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	// Create a HEAD file (minimal git structure)
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatalf("write HEAD: %v", err)
	}

	root, err := buildSourceTree(tmp, false)
	if err != nil {
		t.Fatalf("buildSourceTree: %v", err)
	}

	// Find the repo node
	var repoNode *sourceNode
	for _, child := range root.Children {
		if child.Name == "my-repo" {
			repoNode = child
			break
		}
	}

	if repoNode == nil {
		t.Fatal("my-repo not found in children")
	}

	if !repoNode.IsGitRepo {
		t.Error("my-repo should be marked as a git repo")
	}
}

// TestBuildSourceTreeWithNestedGitRepos tests detection of nested git repos.
func TestBuildSourceTreeWithNestedGitRepos(t *testing.T) {
	tmp := t.TempDir()

	// Create parent directory that contains repos
	parentDir := filepath.Join(tmp, "projects")
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		t.Fatalf("mkdir projects: %v", err)
	}

	// Create two nested repos
	for _, repoName := range []string{"repo1", "repo2"} {
		repoDir := filepath.Join(parentDir, repoName)
		gitDir := filepath.Join(repoDir, ".git")
		if err := os.MkdirAll(gitDir, 0o755); err != nil {
			t.Fatalf("mkdir %s/.git: %v", repoName, err)
		}
		if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
			t.Fatalf("write %s HEAD: %v", repoName, err)
		}
	}

	root, err := buildSourceTree(tmp, false)
	if err != nil {
		t.Fatalf("buildSourceTree: %v", err)
	}

	// Find the projects node
	var projectsNode *sourceNode
	for _, child := range root.Children {
		if child.Name == "projects" {
			projectsNode = child
			break
		}
	}

	if projectsNode == nil {
		t.Fatal("projects not found in children")
	}

	if !projectsNode.HasGitChild {
		t.Error("projects should have HasGitChild=true")
	}

	if projectsNode.IsGitRepo {
		t.Error("projects should not be marked as a git repo")
	}
}

// TestFlattenSourceTree tests the tree flattening functionality.
func TestFlattenSourceTree(t *testing.T) {
	// Build a simple tree manually
	root := &sourceNode{
		Name:       "root",
		IsDir:      true,
		IsExpanded: true,
		Depth:      0,
		Children: []*sourceNode{
			{
				Name:       "dir1",
				IsDir:      true,
				IsExpanded: true,
				Depth:      1,
				Children: []*sourceNode{
					{Name: "file1.txt", IsDir: false, Depth: 2},
					{Name: "file2.txt", IsDir: false, Depth: 2},
				},
			},
			{
				Name:       "dir2",
				IsDir:      true,
				IsExpanded: false, // Collapsed
				Depth:      1,
				Children: []*sourceNode{
					{Name: "hidden.txt", IsDir: false, Depth: 2},
				},
			},
			{Name: "top.txt", IsDir: false, Depth: 1},
		},
	}

	flat := flattenSourceTree(root)

	// Expected: root, dir1, file1.txt, file2.txt, dir2 (collapsed, so no children), top.txt
	expectedNames := []string{"root", "dir1", "file1.txt", "file2.txt", "dir2", "top.txt"}

	if len(flat) != len(expectedNames) {
		t.Errorf("expected %d nodes, got %d", len(expectedNames), len(flat))
	}

	for i, expected := range expectedNames {
		if i >= len(flat) {
			break
		}
		if flat[i].Name != expected {
			t.Errorf("flat[%d]: expected %s, got %s", i, expected, flat[i].Name)
		}
	}
}

// TestExpandCollapseNode tests the expand/collapse functionality.
func TestExpandCollapseNode(t *testing.T) {
	tmp := t.TempDir()

	// Create a directory with a subdirectory
	subdir := filepath.Join(tmp, "subdir")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "file.txt"), []byte("content"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	root, err := buildSourceTree(tmp, false)
	if err != nil {
		t.Fatalf("buildSourceTree: %v", err)
	}

	// Find subdir node
	var subdirNode *sourceNode
	for _, child := range root.Children {
		if child.Name == "subdir" {
			subdirNode = child
			break
		}
	}

	if subdirNode == nil {
		t.Fatal("subdir not found")
	}

	// Initially not expanded
	if subdirNode.IsExpanded {
		t.Error("subdir should not be expanded initially")
	}

	// Expand it
	gitRootSet := make(map[string]bool)
	subdirNode.expandNode(gitRootSet, false)

	if !subdirNode.IsExpanded {
		t.Error("subdir should be expanded after expandNode")
	}

	// Check children were loaded
	if len(subdirNode.Children) == 0 {
		t.Error("subdir should have children after expand")
	}

	// Collapse it
	subdirNode.collapseNode()

	if subdirNode.IsExpanded {
		t.Error("subdir should not be expanded after collapseNode")
	}

	// Children should still be there (lazy unload not implemented)
	if len(subdirNode.Children) == 0 {
		t.Error("subdir children should still be present after collapse")
	}
}

// TestSourceTreeScroller tests the scroller functionality.
func TestSourceTreeScroller(t *testing.T) {
	// Create a flat list of nodes
	nodes := make([]*sourceNode, 10)
	for i := 0; i < 10; i++ {
		nodes[i] = &sourceNode{Name: "node" + string(rune('0'+i))}
	}

	scroller := newSourceTreeScroller(nodes, 5)

	// Initial state
	if scroller.selected != 0 {
		t.Errorf("initial selected should be 0, got %d", scroller.selected)
	}

	// Move down
	scroller.moveDown()
	if scroller.selected != 1 {
		t.Errorf("after moveDown, selected should be 1, got %d", scroller.selected)
	}

	// Move to bottom
	scroller.moveToBottom()
	if scroller.selected != 9 {
		t.Errorf("after moveToBottom, selected should be 9, got %d", scroller.selected)
	}

	// Move up
	scroller.moveUp()
	if scroller.selected != 8 {
		t.Errorf("after moveUp, selected should be 8, got %d", scroller.selected)
	}

	// Move to top
	scroller.moveToTop()
	if scroller.selected != 0 {
		t.Errorf("after moveToTop, selected should be 0, got %d", scroller.selected)
	}

	// selectedNode
	node := scroller.selectedNode()
	if node != nodes[0] {
		t.Error("selectedNode should return the first node")
	}

	// isSelected
	if !scroller.isSelected(0) {
		t.Error("isSelected(0) should be true")
	}
	if scroller.isSelected(1) {
		t.Error("isSelected(1) should be false")
	}
}

// TestSourceTreeScrollerVisibleRange tests scroll offset behavior.
func TestSourceTreeScrollerVisibleRange(t *testing.T) {
	nodes := make([]*sourceNode, 20)
	for i := 0; i < 20; i++ {
		nodes[i] = &sourceNode{Name: "node"}
	}

	scroller := newSourceTreeScroller(nodes, 5)

	// Initial visible range
	start, end := scroller.visibleRange()
	if start != 0 || end != 5 {
		t.Errorf("initial range should be 0-5, got %d-%d", start, end)
	}

	// Move to item 7 (should scroll)
	for i := 0; i < 7; i++ {
		scroller.moveDown()
	}

	start, end = scroller.visibleRange()
	// Selected is 7, with height 5, so offset should be 3 (7-5+1=3)
	if start != 3 {
		t.Errorf("after scrolling, start should be 3, got %d", start)
	}
	if end != 8 {
		t.Errorf("after scrolling, end should be 8, got %d", end)
	}
}

// TestSourceTreeScrollerUpdateTree tests tree update behavior.
func TestSourceTreeScrollerUpdateTree(t *testing.T) {
	nodes := make([]*sourceNode, 10)
	for i := 0; i < 10; i++ {
		nodes[i] = &sourceNode{Name: "node"}
	}

	scroller := newSourceTreeScroller(nodes, 5)
	scroller.moveToBottom() // selected = 9

	// Update with shorter tree
	shortNodes := make([]*sourceNode, 5)
	for i := 0; i < 5; i++ {
		shortNodes[i] = &sourceNode{Name: "node"}
	}

	scroller.updateTree(shortNodes)

	// Selected should be clamped to new max
	if scroller.selected != 4 {
		t.Errorf("selected should be clamped to 4, got %d", scroller.selected)
	}
}

// TestSourceTreeScrollerSelectByPath tests path-based selection.
func TestSourceTreeScrollerSelectByPath(t *testing.T) {
	// Create nodes with paths
	nodes := []*sourceNode{
		{Name: "root", Path: "/tmp/root"},
		{Name: "dir1", Path: "/tmp/root/dir1"},
		{Name: "file1", Path: "/tmp/root/dir1/file1.txt"},
		{Name: "dir2", Path: "/tmp/root/dir2"},
		{Name: "file2", Path: "/tmp/root/dir2/file2.txt"},
	}

	scroller := newSourceTreeScroller(nodes, 5)

	// Select by exact path
	if !scroller.selectByPath("/tmp/root/dir2") {
		t.Error("selectByPath should return true for existing path")
	}
	if scroller.selected != 3 {
		t.Errorf("expected selected=3 for dir2, got %d", scroller.selected)
	}

	// Select by path that doesn't exist - should find parent
	if !scroller.selectByPath("/tmp/root/dir1/subdir/missing.txt") {
		t.Error("selectByPath should return true and select parent")
	}
	if scroller.selected != 1 {
		t.Errorf("expected selected=1 for dir1 (parent), got %d", scroller.selected)
	}

	// Select by completely non-existent path
	scroller.selected = 0 // Reset
	if scroller.selectByPath("/nonexistent/path") {
		t.Error("selectByPath should return false for non-existent path with no parent")
	}

	// Empty path
	if scroller.selectByPath("") {
		t.Error("selectByPath should return false for empty path")
	}

	// Empty tree
	emptyScroller := newSourceTreeScroller([]*sourceNode{}, 5)
	if emptyScroller.selectByPath("/tmp/root") {
		t.Error("selectByPath should return false for empty tree")
	}
}

// TestSourceTreeScrollerSelectAfterDelete simulates the scenario where
// a folder is deleted and we want to select the nearest sibling or parent.
func TestSourceTreeScrollerSelectAfterDelete(t *testing.T) {
	// Initial tree structure:
	// /tmp/root (index 0)
	//   dir1 (index 1)
	//   dir2 (index 2) <- selected, then deleted
	//   dir3 (index 3)
	initialNodes := []*sourceNode{
		{Name: "root", Path: "/tmp/root"},
		{Name: "dir1", Path: "/tmp/root/dir1"},
		{Name: "dir2", Path: "/tmp/root/dir2"},
		{Name: "dir3", Path: "/tmp/root/dir3"},
	}

	scroller := newSourceTreeScroller(initialNodes, 5)

	// Select dir2
	scroller.selectByPath("/tmp/root/dir2")
	if scroller.selected != 2 {
		t.Fatalf("expected selected=2 for dir2, got %d", scroller.selected)
	}

	// Save path before delete
	previousPath := scroller.selectedNode().Path
	t.Logf("Previous path: %s", previousPath)

	// Simulate tree after dir2 is deleted
	afterDeleteNodes := []*sourceNode{
		{Name: "root", Path: "/tmp/root"},
		{Name: "dir1", Path: "/tmp/root/dir1"},
		{Name: "dir3", Path: "/tmp/root/dir3"},
	}

	// Update tree (this clamps selected to valid range)
	scroller.updateTree(afterDeleteNodes)
	t.Logf("After updateTree: selected=%d, tree len=%d", scroller.selected, len(scroller.flatTree))

	// Try to restore to previous path
	found := scroller.selectByPath(previousPath)
	t.Logf("selectByPath(%s) returned %v, selected=%d", previousPath, found, scroller.selected)

	if !found {
		t.Error("selectByPath should find sibling when exact path is deleted")
	}

	// Should select a sibling (dir1 or dir3) since dir2 no longer exists
	selectedNode := scroller.selectedNode()
	if selectedNode == nil {
		t.Fatal("selectedNode is nil")
	}
	t.Logf("Selected node: %s (path: %s)", selectedNode.Name, selectedNode.Path)

	// We should select dir1 (first sibling found in the same parent)
	if selectedNode.Path != "/tmp/root/dir1" {
		t.Errorf("expected to select sibling /tmp/root/dir1, got %s", selectedNode.Path)
	}
}

// TestSourceTreeScrollerSelectAfterDeleteNoSiblings tests fallback to parent when no siblings exist.
func TestSourceTreeScrollerSelectAfterDeleteNoSiblings(t *testing.T) {
	// Tree structure where the only child is deleted:
	// /tmp/root (index 0)
	//   onlychild (index 1) <- selected, then deleted
	initialNodes := []*sourceNode{
		{Name: "root", Path: "/tmp/root"},
		{Name: "onlychild", Path: "/tmp/root/onlychild"},
	}

	scroller := newSourceTreeScroller(initialNodes, 5)
	scroller.selectByPath("/tmp/root/onlychild")

	previousPath := scroller.selectedNode().Path

	// After deletion, only root remains
	afterDeleteNodes := []*sourceNode{
		{Name: "root", Path: "/tmp/root"},
	}

	scroller.updateTree(afterDeleteNodes)
	found := scroller.selectByPath(previousPath)

	if !found {
		t.Error("selectByPath should find parent when no siblings exist")
	}

	// Should select the parent since there are no siblings
	selectedNode := scroller.selectedNode()
	if selectedNode.Path != "/tmp/root" {
		t.Errorf("expected to select parent /tmp/root, got %s", selectedNode.Path)
	}
}

// TestSelectByPathWithNestedSiblings tests that siblings are found in nested expanded directories.
func TestSelectByPathWithNestedSiblings(t *testing.T) {
	// Tree structure with nested expanded directory:
	// /tmp/root (expanded)
	//   parent/ (expanded)
	//     child1/
	//     child2/ <- selected, then deleted
	//     child3/
	nodes := []*sourceNode{
		{Name: "root", Path: "/tmp/root"},
		{Name: "parent", Path: "/tmp/root/parent"},
		{Name: "child1", Path: "/tmp/root/parent/child1"},
		{Name: "child2", Path: "/tmp/root/parent/child2"},
		{Name: "child3", Path: "/tmp/root/parent/child3"},
	}

	scroller := newSourceTreeScroller(nodes, 10)
	scroller.selectByPath("/tmp/root/parent/child2")

	if scroller.selected != 3 {
		t.Fatalf("expected selected=3 for child2, got %d", scroller.selected)
	}

	previousPath := scroller.selectedNode().Path

	// Simulate tree after child2 is deleted (parent is still expanded)
	afterDeleteNodes := []*sourceNode{
		{Name: "root", Path: "/tmp/root"},
		{Name: "parent", Path: "/tmp/root/parent"},
		{Name: "child1", Path: "/tmp/root/parent/child1"},
		{Name: "child3", Path: "/tmp/root/parent/child3"},
	}

	scroller.updateTree(afterDeleteNodes)
	found := scroller.selectByPath(previousPath)

	if !found {
		t.Error("selectByPath should find sibling in nested directory")
	}

	// Should select child1 (first sibling with same parent)
	selectedNode := scroller.selectedNode()
	if selectedNode.Path != "/tmp/root/parent/child1" {
		t.Errorf("expected to select sibling /tmp/root/parent/child1, got %s", selectedNode.Path)
	}
}

// TestBuildSourceTreeSymlink tests symlink handling.
func TestBuildSourceTreeSymlink(t *testing.T) {
	tmp := t.TempDir()

	// Create a directory and a symlink to it
	realDir := filepath.Join(tmp, "realdir")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatalf("mkdir realdir: %v", err)
	}

	linkPath := filepath.Join(tmp, "linkdir")
	if err := os.Symlink(realDir, linkPath); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	root, err := buildSourceTree(tmp, false)
	if err != nil {
		t.Fatalf("buildSourceTree: %v", err)
	}

	var linkNode *sourceNode
	for _, child := range root.Children {
		if child.Name == "linkdir" {
			linkNode = child
			break
		}
	}

	if linkNode == nil {
		t.Fatal("linkdir not found")
	}

	if !linkNode.IsSymlink {
		t.Error("linkdir should be marked as symlink")
	}

	// Symlinks should not be treated as directories to prevent infinite loops
	if linkNode.IsDir {
		t.Error("symlink should not be treated as directory")
	}
}

// Note: TestLoadSourceChildrenTruncation is not included because maxSourceDirEntries
// is a const and cannot be modified in tests without refactoring to use dependency injection.

// TestToggleExpand tests the toggleExpand functionality.
func TestToggleExpand(t *testing.T) {
	node := &sourceNode{
		Name:       "dir",
		IsDir:      true,
		IsExpanded: false,
	}

	gitRootSet := make(map[string]bool)

	// Toggle should expand
	node.toggleExpand(gitRootSet, false)
	if !node.IsExpanded {
		t.Error("node should be expanded after first toggle")
	}

	// Toggle again should collapse
	node.toggleExpand(gitRootSet, false)
	if node.IsExpanded {
		t.Error("node should be collapsed after second toggle")
	}

	// Non-directory should not change
	fileNode := &sourceNode{
		Name:  "file.txt",
		IsDir: false,
	}
	fileNode.toggleExpand(gitRootSet, false)
	if fileNode.IsExpanded {
		t.Error("file node should not be expandable")
	}
}

// =============================================================================
// State Transition Tests
// =============================================================================

// TestImportBrowserStateString tests the String() method for states.
func TestImportBrowserStateString(t *testing.T) {
	tests := []struct {
		state    ImportBrowserState
		expected string
	}{
		{StateBrowse, "Browse"},
		{StateImportConfig, "Import Config"},
		{StateTemplateSelect, "Template Select"},
		{StateTemplateVars, "Template Variables"},
		{StateExtraFiles, "Extra Files"},
		{StateImportPreview, "Import Preview"},
		{StateImportExecute, "Importing"},
		{StatePostImport, "Post Import"},
		{StateStashConfirm, "Stash Confirm"},
		{StateStashExecute, "Stashing"},
		{StateAddToSelect, "Add To Workspace"},
		{StateBatchImportConfirm, "Batch Import Confirm"},
		{StateBatchImportExecute, "Batch Importing"},
		{StateBatchImportSummary, "Batch Import Summary"},
		{StateBatchStashConfirm, "Batch Stash Confirm"},
		{StateBatchStashExecute, "Batch Stashing"},
		{StateBatchStashSummary, "Batch Stash Summary"},
		{StateDeleteConfirm, "Delete Confirm"},
		{StateTrashConfirm, "Trash Confirm"},
		{StateComplete, "Complete"},
		{ImportBrowserState(999), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.state.String(); got != tt.expected {
				t.Errorf("state.String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestStartImport tests the transition from Browse to ImportConfig state.
func TestStartImport(t *testing.T) {
	model := &ImportBrowserModel{
		state: StateBrowse,
	}

	node := &sourceNode{
		Name:  "myproject",
		Path:  "/tmp/myproject",
		IsDir: true,
	}

	model.startImport(node)

	if model.state != StateImportConfig {
		t.Errorf("state should be StateImportConfig, got %s", model.state)
	}

	if model.importTarget != node {
		t.Error("importTarget should be set to the node")
	}

	if model.configFocusIdx != 0 {
		t.Errorf("configFocusIdx should be 0, got %d", model.configFocusIdx)
	}

	// Project name should be pre-populated from folder name
	if model.projectInput.Value() != "myproject" {
		t.Errorf("projectInput should be pre-populated with 'myproject', got %q", model.projectInput.Value())
	}
}

// TestStartStash tests the transition from Browse to StashConfirm state.
func TestStartStash(t *testing.T) {
	model := &ImportBrowserModel{
		state: StateBrowse,
	}

	node := &sourceNode{
		Name:  "oldproject",
		Path:  "/tmp/oldproject",
		IsDir: true,
	}

	// Start stash without delete
	model.startStash(node, false)

	if model.state != StateStashConfirm {
		t.Errorf("state should be StateStashConfirm, got %s", model.state)
	}

	if model.stashTarget != node {
		t.Error("stashTarget should be set to the node")
	}

	if model.stashDeleteAfter {
		t.Error("stashDeleteAfter should be false")
	}

	// Start stash with delete
	model.startStash(node, true)

	if !model.stashDeleteAfter {
		t.Error("stashDeleteAfter should be true when started with delete=true")
	}
}

// TestSanitizeForSlug tests the slug sanitization function.
func TestSanitizeForSlug(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"myproject", "myproject"},
		{"MyProject", "myproject"},
		{"my-project", "my-project"},
		{"my_project", "my-project"},
		{"my project", "my-project"},
		{"My Project 123", "my-project-123"},
		{"project@#$%", "project"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := sanitizeForSlug(tt.input); got != tt.expected {
				t.Errorf("sanitizeForSlug(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// TestIsValidSlugPart tests the slug validation function.
func TestIsValidSlugPart(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"myproject", true},
		{"my-project", true},
		{"my123", true},
		{"123", true},
		{"my_project", false}, // underscores not allowed
		{"MyProject", false},  // uppercase not allowed
		{"my project", false}, // spaces not allowed
		{"my@project", false}, // special chars not allowed
		{"", false},           // empty not allowed
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isValidSlugPart(tt.input); got != tt.expected {
				t.Errorf("isValidSlugPart(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

// TestExtraFilesNavigation tests navigation in extra files state.
func TestExtraFilesNavigation(t *testing.T) {
	model := &ImportBrowserModel{
		state: StateExtraFiles,
		extraFilesItems: []extraFileItem{
			{Name: "file1.txt", RelPath: "file1.txt"},
			{Name: "file2.txt", RelPath: "file2.txt"},
			{Name: "file3.txt", RelPath: "file3.txt"},
		},
		extraFilesSelected: 0,
		height:             20,
	}

	// Move down
	model.extraFilesSelected = 0
	if model.extraFilesSelected < len(model.extraFilesItems)-1 {
		model.extraFilesSelected++
	}
	if model.extraFilesSelected != 1 {
		t.Errorf("expected selected=1 after move down, got %d", model.extraFilesSelected)
	}

	// Move to end
	model.extraFilesSelected = len(model.extraFilesItems) - 1
	if model.extraFilesSelected != 2 {
		t.Errorf("expected selected=2 at end, got %d", model.extraFilesSelected)
	}

	// Try to move past end
	if model.extraFilesSelected < len(model.extraFilesItems)-1 {
		model.extraFilesSelected++
	}
	if model.extraFilesSelected != 2 {
		t.Errorf("expected selected=2 (clamped), got %d", model.extraFilesSelected)
	}
}

// TestExtraFilesToggle tests selection toggling in extra files state.
func TestExtraFilesToggle(t *testing.T) {
	model := &ImportBrowserModel{
		state: StateExtraFiles,
		extraFilesItems: []extraFileItem{
			{Name: "file1.txt", RelPath: "file1.txt", Checked: false},
			{Name: "file2.txt", RelPath: "file2.txt", Checked: false},
		},
		extraFilesSelected: 0,
	}

	// Toggle first item
	model.extraFilesItems[model.extraFilesSelected].Checked = !model.extraFilesItems[model.extraFilesSelected].Checked

	if !model.extraFilesItems[0].Checked {
		t.Error("first item should be checked after toggle")
	}

	// Select all
	for i := range model.extraFilesItems {
		model.extraFilesItems[i].Checked = true
	}

	for i, item := range model.extraFilesItems {
		if !item.Checked {
			t.Errorf("item %d should be checked after select all", i)
		}
	}

	// Select none
	for i := range model.extraFilesItems {
		model.extraFilesItems[i].Checked = false
	}

	for i, item := range model.extraFilesItems {
		if item.Checked {
			t.Errorf("item %d should not be checked after select none", i)
		}
	}
}

// TestGetExtraFilesSelectedPaths tests the path extraction function.
func TestGetExtraFilesSelectedPaths(t *testing.T) {
	model := &ImportBrowserModel{
		extraFilesItems: []extraFileItem{
			{Name: "file1.txt", RelPath: "file1.txt", Checked: true},
			{Name: "file2.txt", RelPath: "file2.txt", Checked: false},
			{Name: "dir1", RelPath: "dir1", Checked: true, IsDir: true},
		},
	}

	paths := model.getExtraFilesSelectedPaths()

	if len(paths) != 2 {
		t.Errorf("expected 2 selected paths, got %d", len(paths))
	}

	expected := []string{"file1.txt", "dir1"}
	for i, path := range paths {
		if path != expected[i] {
			t.Errorf("path[%d] = %q, want %q", i, path, expected[i])
		}
	}
}

// TestPostImportOptions tests the post-import option selection.
func TestPostImportOptions(t *testing.T) {
	model := &ImportBrowserModel{
		state:            StatePostImport,
		postImportOption: 0,
	}

	// Initial option should be 0 (keep)
	if model.postImportOption != 0 {
		t.Errorf("initial option should be 0, got %d", model.postImportOption)
	}

	// Move down
	model.postImportOption++
	if model.postImportOption != 1 {
		t.Errorf("after move down, option should be 1, got %d", model.postImportOption)
	}

	// Move to max
	model.postImportOption = 2
	if model.postImportOption != 2 {
		t.Errorf("max option should be 2, got %d", model.postImportOption)
	}

	// Clamp at max
	if model.postImportOption < 2 {
		model.postImportOption++
	}
	if model.postImportOption != 2 {
		t.Errorf("option should be clamped at 2, got %d", model.postImportOption)
	}
}

// TestAddToWorkspaceNavigation tests navigation in workspace selection.
func TestAddToWorkspaceNavigation(t *testing.T) {
	model := &ImportBrowserModel{
		state: StateAddToSelect,
		addToWorkspaces: []string{
			"owner1--project1",
			"owner1--project2",
			"owner2--project1",
		},
		addToSelected:     0,
		addToScrollOffset: 0,
		height:            20,
	}

	// Move down
	if model.addToSelected < len(model.addToWorkspaces)-1 {
		model.addToSelected++
	}
	if model.addToSelected != 1 {
		t.Errorf("expected selected=1, got %d", model.addToSelected)
	}

	// Move to bottom
	model.addToSelected = len(model.addToWorkspaces) - 1
	if model.addToSelected != 2 {
		t.Errorf("expected selected=2 at bottom, got %d", model.addToSelected)
	}

	// Move up
	if model.addToSelected > 0 {
		model.addToSelected--
	}
	if model.addToSelected != 1 {
		t.Errorf("expected selected=1 after move up, got %d", model.addToSelected)
	}

	// Move to top
	model.addToSelected = 0
	if model.addToSelected != 0 {
		t.Errorf("expected selected=0 at top, got %d", model.addToSelected)
	}
}

// TestClearAddToState tests the state cleanup function.
func TestClearAddToState(t *testing.T) {
	model := &ImportBrowserModel{
		importTarget:      &sourceNode{Name: "test"},
		addToWorkspaces:   []string{"ws1", "ws2"},
		addToTargetSlug:   "owner--project",
		addToSelected:     5,
		addToScrollOffset: 3,
	}

	model.clearAddToState()

	if model.importTarget != nil {
		t.Error("importTarget should be nil after clear")
	}
	if model.addToWorkspaces != nil {
		t.Error("addToWorkspaces should be nil after clear")
	}
	if model.addToTargetSlug != "" {
		t.Error("addToTargetSlug should be empty after clear")
	}
	if model.addToSelected != 0 {
		t.Error("addToSelected should be 0 after clear")
	}
	if model.addToScrollOffset != 0 {
		t.Error("addToScrollOffset should be 0 after clear")
	}
}

// TestImportBrowserResult tests the result struct initialization.
func TestImportBrowserResult(t *testing.T) {
	result := ImportBrowserResult{
		Action:        "import",
		WorkspacePath: "/code/owner--project",
		WorkspaceSlug: "owner--project",
		ReposImported: []string{"repo1", "repo2"},
		Success:       true,
	}

	if result.Action != "import" {
		t.Errorf("Action = %q, want 'import'", result.Action)
	}
	if len(result.ReposImported) != 2 {
		t.Errorf("expected 2 repos, got %d", len(result.ReposImported))
	}
	if !result.Success {
		t.Error("Success should be true")
	}
	if result.Aborted {
		t.Error("Aborted should be false")
	}
}

// TestImportBrowserResultWithTemplate tests the result struct with template fields.
func TestImportBrowserResultWithTemplate(t *testing.T) {
	result := ImportBrowserResult{
		Action:               "import",
		WorkspacePath:        "/code/owner--project",
		WorkspaceSlug:        "owner--project",
		ReposImported:        []string{"repo1"},
		Success:              true,
		TemplateApplied:      "go-library",
		TemplateFilesCreated: 3,
	}

	if result.TemplateApplied != "go-library" {
		t.Errorf("TemplateApplied = %q, want 'go-library'", result.TemplateApplied)
	}
	if result.TemplateFilesCreated != 3 {
		t.Errorf("TemplateFilesCreated = %d, want 3", result.TemplateFilesCreated)
	}
}

// TestFormatSize tests the size formatting function.
func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1572864, "1.5 MB"},
		{1073741824, "1.0 GB"},
		{1610612736, "1.5 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := formatSize(tt.bytes); got != tt.expected {
				t.Errorf("formatSize(%d) = %q, want %q", tt.bytes, got, tt.expected)
			}
		})
	}
}

// TestGetSizeStatus tests async size calculation and caching.
func TestGetSizeStatus(t *testing.T) {
	tmp := t.TempDir()

	// Create a file with known content
	testFile := filepath.Join(tmp, "test.txt")
	content := []byte("hello world") // 11 bytes
	if err := os.WriteFile(testFile, content, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	// Create a subdirectory with files
	subdir := filepath.Join(tmp, "subdir")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "file1.txt"), []byte("abc"), 0o644); err != nil {
		t.Fatalf("write file1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "file2.txt"), []byte("defgh"), 0o644); err != nil {
		t.Fatalf("write file2: %v", err)
	}

	model := &ImportBrowserModel{
		sizeCache:   make(map[string]int64),
		sizePending: make(map[string]struct{}),
	}

	// Test file size (synchronous for files)
	size, cached, pending := model.getSizeStatus(testFile, false)
	if !cached {
		t.Error("expected cached=true for file")
	}
	if pending {
		t.Error("expected pending=false for file")
	}
	if size != 11 {
		t.Errorf("file size = %d, want 11", size)
	}

	// Test directory size - should not be cached initially
	size, cached, pending = model.getSizeStatus(subdir, true)
	if cached {
		t.Error("expected cached=false for directory initially")
	}
	if pending {
		t.Error("expected pending=false for directory initially")
	}

	// Trigger async calculation
	cmd := model.triggerSizeCalc(subdir)
	if cmd == nil {
		t.Error("expected non-nil command from triggerSizeCalc")
	}

	// Now it should be pending
	size, cached, pending = model.getSizeStatus(subdir, true)
	if cached {
		t.Error("expected cached=false while pending")
	}
	if !pending {
		t.Error("expected pending=true after triggerSizeCalc")
	}

	// Execute the command to simulate async completion
	msg := cmd()
	sizeMsg, ok := msg.(sizeResultMsg)
	if !ok {
		t.Fatalf("expected sizeResultMsg, got %T", msg)
	}
	if sizeMsg.Path != subdir {
		t.Errorf("path = %s, want %s", sizeMsg.Path, subdir)
	}
	if sizeMsg.Size != 8 {
		t.Errorf("size = %d, want 8", sizeMsg.Size)
	}
	if sizeMsg.Err != nil {
		t.Errorf("unexpected error: %v", sizeMsg.Err)
	}

	// Simulate handling the result (as Update would do)
	delete(model.sizePending, sizeMsg.Path)
	model.sizeCache[sizeMsg.Path] = sizeMsg.Size

	// Now it should be cached
	size, cached, pending = model.getSizeStatus(subdir, true)
	if !cached {
		t.Error("expected cached=true after result")
	}
	if pending {
		t.Error("expected pending=false after result")
	}
	if size != 8 {
		t.Errorf("directory size = %d, want 8", size)
	}

	// Modify cache and verify it's used
	model.sizeCache[subdir] = 999
	size, cached, _ = model.getSizeStatus(subdir, true)
	if !cached {
		t.Error("expected cached=true")
	}
	if size != 999 {
		t.Errorf("should use cached value, got %d", size)
	}

	// triggerSizeCalc should return nil if already cached
	cmd = model.triggerSizeCalc(subdir)
	if cmd != nil {
		t.Error("expected nil command for cached path")
	}
}

// TestApplyFilter tests the tree filtering functionality.
func TestApplyFilter(t *testing.T) {
	// Create a flat tree manually
	nodes := []*sourceNode{
		{Name: "root", IsDir: true},
		{Name: "project1", IsDir: true},
		{Name: "project2", IsDir: true},
		{Name: "readme.txt", IsDir: false},
		{Name: "config.json", IsDir: false},
	}

	model := &ImportBrowserModel{
		scroller: newSourceTreeScroller(nodes, 10),
		root: &sourceNode{
			Name:       "root",
			IsDir:      true,
			IsExpanded: true,
			Children:   nodes[1:], // All except root
		},
	}

	// No filter - all nodes visible
	model.filterText = ""
	model.applyFilter()
	if len(model.scroller.flatTree) != 5 {
		t.Errorf("expected 5 nodes with no filter, got %d", len(model.scroller.flatTree))
	}

	// Filter by "project"
	model.filterText = "project"
	model.applyFilter()
	if len(model.scroller.flatTree) != 2 {
		t.Errorf("expected 2 nodes matching 'project', got %d", len(model.scroller.flatTree))
	}

	// Filter is case-insensitive
	model.filterText = "PROJECT"
	model.applyFilter()
	if len(model.scroller.flatTree) != 2 {
		t.Errorf("expected 2 nodes matching 'PROJECT' (case-insensitive), got %d", len(model.scroller.flatTree))
	}

	// Filter by "json"
	model.filterText = "json"
	model.applyFilter()
	if len(model.scroller.flatTree) != 1 {
		t.Errorf("expected 1 node matching 'json', got %d", len(model.scroller.flatTree))
	}

	// Filter with no matches
	model.filterText = "nonexistent"
	model.applyFilter()
	if len(model.scroller.flatTree) != 0 {
		t.Errorf("expected 0 nodes matching 'nonexistent', got %d", len(model.scroller.flatTree))
	}

	// Clear filter
	model.filterText = ""
	model.applyFilter()
	if len(model.scroller.flatTree) != 5 {
		t.Errorf("expected 5 nodes after clearing filter, got %d", len(model.scroller.flatTree))
	}
}

// TestBuildSourceTreeHiddenFiles tests hidden file filtering.
func TestBuildSourceTreeHiddenFiles(t *testing.T) {
	tmp := t.TempDir()

	// Create visible and hidden files
	if err := os.WriteFile(filepath.Join(tmp, "visible.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write visible: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".hidden"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write hidden: %v", err)
	}
	// .env and .gitignore should always show
	if err := os.WriteFile(filepath.Join(tmp, ".env"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".gitignore"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}

	// Test with showHidden=false
	root, err := buildSourceTree(tmp, false)
	if err != nil {
		t.Fatalf("buildSourceTree: %v", err)
	}

	hasVisible := false
	hasHidden := false
	hasEnv := false
	hasGitignore := false

	for _, child := range root.Children {
		switch child.Name {
		case "visible.txt":
			hasVisible = true
		case ".hidden":
			hasHidden = true
		case ".env":
			hasEnv = true
		case ".gitignore":
			hasGitignore = true
		}
	}

	if !hasVisible {
		t.Error("visible.txt should be present")
	}
	if hasHidden {
		t.Error(".hidden should NOT be present when showHidden=false")
	}
	if !hasEnv {
		t.Error(".env should be present (exception)")
	}
	if !hasGitignore {
		t.Error(".gitignore should be present (exception)")
	}

	// Test with showHidden=true
	root, err = buildSourceTree(tmp, true)
	if err != nil {
		t.Fatalf("buildSourceTree with showHidden: %v", err)
	}

	hasHidden = false
	for _, child := range root.Children {
		if child.Name == ".hidden" {
			hasHidden = true
			break
		}
	}

	if !hasHidden {
		t.Error(".hidden SHOULD be present when showHidden=true")
	}
}

// TestMultiSelect tests the multi-selection functionality for batch operations.
func TestMultiSelect(t *testing.T) {
	tmp := t.TempDir()

	// Create directory structure
	dirs := []string{"dir1", "dir2", "dir3"}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(tmp, d), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	root, err := buildSourceTree(tmp, false)
	if err != nil {
		t.Fatalf("buildSourceTree: %v", err)
	}

	flatTree := flattenSourceTree(root)
	scroller := newSourceTreeScroller(flatTree, 20)

	// Initially no selections
	if count := scroller.getSelectedCount(); count != 0 {
		t.Errorf("expected 0 selected, got %d", count)
	}

	// Select first directory (dir1)
	for _, node := range flatTree {
		if node.Name == "dir1" {
			node.IsSelected = true
			break
		}
	}

	if count := scroller.getSelectedCount(); count != 1 {
		t.Errorf("expected 1 selected, got %d", count)
	}

	// Select second directory (dir2)
	for _, node := range flatTree {
		if node.Name == "dir2" {
			node.IsSelected = true
			break
		}
	}

	if count := scroller.getSelectedCount(); count != 2 {
		t.Errorf("expected 2 selected, got %d", count)
	}

	// Get selected nodes
	selected := scroller.getSelectedNodes()
	if len(selected) != 2 {
		t.Errorf("expected 2 selected nodes, got %d", len(selected))
	}

	// Verify selected nodes are the ones we selected
	names := make(map[string]bool)
	for _, node := range selected {
		names[node.Name] = true
	}
	if !names["dir1"] || !names["dir2"] {
		t.Error("selected nodes should contain dir1 and dir2")
	}

	// Clear all selections
	scroller.clearAllSelections()

	if count := scroller.getSelectedCount(); count != 0 {
		t.Errorf("expected 0 selected after clear, got %d", count)
	}

	for _, node := range flatTree {
		if node.IsSelected {
			t.Errorf("node %s should not be selected after clear", node.Name)
		}
	}
}

// TestMultiSelectOnlyDirs tests that only directories count as selections.
func TestMultiSelectOnlyDirs(t *testing.T) {
	tmp := t.TempDir()

	// Create a directory with files
	if err := os.MkdirAll(filepath.Join(tmp, "dir1"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "file1.txt"), []byte("content"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "dir1", "file2.txt"), []byte("content"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	root, err := buildSourceTree(tmp, false)
	if err != nil {
		t.Fatalf("buildSourceTree: %v", err)
	}

	// Expand root to see dir1's children
	root.IsExpanded = true

	flatTree := flattenSourceTree(root)
	scroller := newSourceTreeScroller(flatTree, 20)

	// Mark all nodes as selected (simulating user selecting everything)
	for _, node := range flatTree {
		node.IsSelected = true
	}

	// getSelectedCount should only count directories
	selected := scroller.getSelectedNodes()

	// Should only get directories (root and dir1)
	for _, node := range selected {
		if !node.IsDir {
			t.Errorf("getSelectedNodes returned non-directory: %s", node.Name)
		}
	}

	// All selected nodes should be directories
	count := scroller.getSelectedCount()
	dirCount := 0
	for _, node := range flatTree {
		if node.IsDir {
			dirCount++
		}
	}

	if count != dirCount {
		t.Errorf("selected count (%d) should match directory count (%d)", count, dirCount)
	}
}

// TestTemplateSelectNavigation tests navigation in template selection.
func TestTemplateSelectNavigation(t *testing.T) {
	model := &ImportBrowserModel{
		state: StateTemplateSelect,
		templateInfos: []template.TemplateInfo{
			{Name: "template1", Description: "First template", VarCount: 2},
			{Name: "template2", Description: "Second template", RepoCount: 1},
			{Name: "template3", Description: "Third template", VarCount: 1, RepoCount: 2},
		},
		templateSelected:     0, // "No template" option
		templateScrollOffset: 0,
		height:               20,
	}

	// Initial selection is "No template" (index 0)
	if model.templateSelected != 0 {
		t.Errorf("expected initial selection=0, got %d", model.templateSelected)
	}

	// Move down to first template
	maxIdx := len(model.templateInfos)
	if model.templateSelected < maxIdx {
		model.templateSelected++
	}
	if model.templateSelected != 1 {
		t.Errorf("expected selected=1 after move down, got %d", model.templateSelected)
	}

	// Move to last template
	model.templateSelected = len(model.templateInfos)
	if model.templateSelected != 3 {
		t.Errorf("expected selected=3 at bottom, got %d", model.templateSelected)
	}

	// Try to move beyond last (should stay at max)
	if model.templateSelected < maxIdx {
		model.templateSelected++
	}
	if model.templateSelected != 3 {
		t.Errorf("should stay at max, got %d", model.templateSelected)
	}

	// Move up
	if model.templateSelected > 0 {
		model.templateSelected--
	}
	if model.templateSelected != 2 {
		t.Errorf("expected selected=2 after move up, got %d", model.templateSelected)
	}

	// Move to top
	model.templateSelected = 0
	model.templateScrollOffset = 0
	if model.templateSelected != 0 {
		t.Errorf("expected selected=0 at top, got %d", model.templateSelected)
	}
}

// TestTemplateSelectResult tests template selection results.
func TestTemplateSelectResult(t *testing.T) {
	model := &ImportBrowserModel{
		state: StateTemplateSelect,
		templateInfos: []template.TemplateInfo{
			{Name: "template1", Description: "First template"},
			{Name: "template2", Description: "Second template"},
		},
		templateSelected: 0,
		selectedTemplate: "",
	}

	// Select "No template" (index 0)
	model.templateSelected = 0
	if model.templateSelected == 0 {
		model.selectedTemplate = ""
	} else {
		model.selectedTemplate = model.templateInfos[model.templateSelected-1].Name
	}
	if model.selectedTemplate != "" {
		t.Errorf("expected empty template name for 'No template', got %q", model.selectedTemplate)
	}

	// Select first template (index 1)
	model.templateSelected = 1
	if model.templateSelected == 0 {
		model.selectedTemplate = ""
	} else {
		model.selectedTemplate = model.templateInfos[model.templateSelected-1].Name
	}
	if model.selectedTemplate != "template1" {
		t.Errorf("expected 'template1', got %q", model.selectedTemplate)
	}

	// Select second template (index 2)
	model.templateSelected = 2
	if model.templateSelected == 0 {
		model.selectedTemplate = ""
	} else {
		model.selectedTemplate = model.templateInfos[model.templateSelected-1].Name
	}
	if model.selectedTemplate != "template2" {
		t.Errorf("expected 'template2', got %q", model.selectedTemplate)
	}
}

// TestMultiSelectToggle tests toggling selection on and off.
func TestMultiSelectToggle(t *testing.T) {
	tmp := t.TempDir()

	if err := os.MkdirAll(filepath.Join(tmp, "testdir"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	root, err := buildSourceTree(tmp, false)
	if err != nil {
		t.Fatalf("buildSourceTree: %v", err)
	}

	flatTree := flattenSourceTree(root)
	scroller := newSourceTreeScroller(flatTree, 20)

	// Find testdir
	var testdir *sourceNode
	for _, node := range flatTree {
		if node.Name == "testdir" {
			testdir = node
			break
		}
	}

	if testdir == nil {
		t.Fatal("testdir not found in tree")
	}

	// Initially not selected
	if testdir.IsSelected {
		t.Error("testdir should not be selected initially")
	}
	if scroller.getSelectedCount() != 0 {
		t.Error("should have 0 selected initially")
	}

	// Toggle on
	testdir.IsSelected = !testdir.IsSelected
	if !testdir.IsSelected {
		t.Error("testdir should be selected after toggle on")
	}
	if scroller.getSelectedCount() != 1 {
		t.Errorf("should have 1 selected, got %d", scroller.getSelectedCount())
	}

	// Toggle off
	testdir.IsSelected = !testdir.IsSelected
	if testdir.IsSelected {
		t.Error("testdir should not be selected after toggle off")
	}
	if scroller.getSelectedCount() != 0 {
		t.Errorf("should have 0 selected after toggle off, got %d", scroller.getSelectedCount())
	}
}

// TestTemplateVarsState tests the template variable prompting state.
func TestTemplateVarsState(t *testing.T) {
	model := &ImportBrowserModel{
		state: StateTemplateVars,
		templateVars: []template.TemplateVar{
			{Name: "var1", Type: template.VarTypeString, Required: true},
			{Name: "var2", Type: template.VarTypeBoolean},
			{Name: "var3", Type: template.VarTypeChoice, Choices: []string{"opt1", "opt2", "opt3"}},
		},
		templateVarIndex:  0,
		templateVarValues: make(map[string]string),
		height:            20,
	}

	// Initial index should be 0
	if model.templateVarIndex != 0 {
		t.Errorf("expected templateVarIndex=0, got %d", model.templateVarIndex)
	}

	// Simulate completing first variable
	model.templateVarValues["var1"] = "value1"
	model.templateVarIndex++

	if model.templateVarIndex != 1 {
		t.Errorf("expected templateVarIndex=1 after first var, got %d", model.templateVarIndex)
	}

	// Simulate boolean variable
	model.templateVarBoolValue = true
	model.templateVarValues["var2"] = "true"
	model.templateVarIndex++

	if model.templateVarIndex != 2 {
		t.Errorf("expected templateVarIndex=2 after second var, got %d", model.templateVarIndex)
	}

	// Simulate choice variable
	model.templateVarChoiceIdx = 1
	model.templateVarValues["var3"] = "opt2"
	model.templateVarIndex++

	if model.templateVarIndex != 3 {
		t.Errorf("expected templateVarIndex=3 after third var, got %d", model.templateVarIndex)
	}

	// Verify all values collected
	if len(model.templateVarValues) != 3 {
		t.Errorf("expected 3 values, got %d", len(model.templateVarValues))
	}
	if model.templateVarValues["var1"] != "value1" {
		t.Errorf("expected var1='value1', got %q", model.templateVarValues["var1"])
	}
	if model.templateVarValues["var2"] != "true" {
		t.Errorf("expected var2='true', got %q", model.templateVarValues["var2"])
	}
	if model.templateVarValues["var3"] != "opt2" {
		t.Errorf("expected var3='opt2', got %q", model.templateVarValues["var3"])
	}
}

// TestGetBuiltinVariables tests the builtin variable extraction.
func TestGetBuiltinVariables(t *testing.T) {
	model := &ImportBrowserModel{
		result: ImportBrowserResult{
			WorkspaceSlug: "myowner--myproject",
		},
	}

	vars := model.getBuiltinVariables()

	if vars["owner"] != "myowner" {
		t.Errorf("expected owner='myowner', got %q", vars["owner"])
	}
	if vars["project"] != "myproject" {
		t.Errorf("expected project='myproject', got %q", vars["project"])
	}
}

// TestStartBatchImport tests the transition to batch import state.
func TestStartBatchImport(t *testing.T) {
	// Create a model with initialized text inputs
	ownerInput := textinput.New()
	ownerInput.Placeholder = "owner"

	model := ImportBrowserModel{
		state:      StateBrowse,
		ownerInput: ownerInput,
	}

	nodes := []*sourceNode{
		{Name: "project1", Path: "/tmp/project1", IsDir: true},
		{Name: "project2", Path: "/tmp/project2", IsDir: true},
	}

	result, _ := model.startBatchImport(nodes)
	m := result.(ImportBrowserModel)

	if m.state != StateBatchImportConfirm {
		t.Errorf("expected state=StateBatchImportConfirm, got %v", m.state)
	}
	if len(m.batchImportTargets) != 2 {
		t.Errorf("expected 2 batch targets, got %d", len(m.batchImportTargets))
	}
}

// TestBatchImportItemResult tests the batch import result struct.
func TestBatchImportItemResult(t *testing.T) {
	result := BatchImportItemResult{
		SourcePath:    "/tmp/source",
		SourceName:    "source",
		WorkspaceSlug: "owner--source",
		WorkspacePath: "/workspaces/owner--source",
		RepoCount:     3,
		Success:       true,
	}

	if !result.Success {
		t.Error("expected Success=true")
	}
	if result.RepoCount != 3 {
		t.Errorf("expected RepoCount=3, got %d", result.RepoCount)
	}
}

// TestStartBatchStash tests the transition to batch stash state.
func TestStartBatchStash(t *testing.T) {
	model := ImportBrowserModel{
		state: StateBrowse,
	}

	nodes := []*sourceNode{
		{Name: "project1", Path: "/tmp/project1", IsDir: true},
		{Name: "project2", Path: "/tmp/project2", IsDir: true},
	}

	result, _ := model.startBatchStash(nodes, true)
	m := result.(ImportBrowserModel)

	if m.state != StateBatchStashConfirm {
		t.Errorf("expected state=StateBatchStashConfirm, got %v", m.state)
	}
	if len(m.batchStashTargets) != 2 {
		t.Errorf("expected 2 batch targets, got %d", len(m.batchStashTargets))
	}
	if !m.batchStashDeleteAfter {
		t.Error("expected batchStashDeleteAfter=true")
	}
}

// TestBatchStashItemResult tests the batch stash result struct.
func TestBatchStashItemResult(t *testing.T) {
	result := BatchStashItemResult{
		SourcePath:  "/tmp/source",
		SourceName:  "source",
		ArchivePath: "/archives/source--2025-01-01--stash.tar.gz",
		Deleted:     true,
		Success:     true,
	}

	if !result.Success {
		t.Error("expected Success=true")
	}
	if !result.Deleted {
		t.Error("expected Deleted=true")
	}
}

// =============================================================================
// Integration Tests - Simulate key presses through Update method
// =============================================================================

// TestIntegrationBrowseNavigation tests browsing navigation via key presses.
func TestIntegrationBrowseNavigation(t *testing.T) {
	tmp := t.TempDir()

	// Create directory structure
	for _, d := range []string{"dir1", "dir2", "dir3"} {
		if err := os.MkdirAll(filepath.Join(tmp, d), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}

	root, err := buildSourceTree(tmp, false)
	if err != nil {
		t.Fatalf("buildSourceTree: %v", err)
	}

	flatTree := flattenSourceTree(root)
	scroller := newSourceTreeScroller(flatTree, 20)

	model := ImportBrowserModel{
		state:        StateBrowse,
		root:         root,
		scroller:     scroller,
		rootPath:     tmp,
		sizeCache:    make(map[string]int64),
		sizePending:  make(map[string]struct{}),
		gitRootSet:   make(map[string]bool),
		ownerInput:   textinput.New(),
		projectInput: textinput.New(),
		height:       30,
		width:        80,
	}

	// Verify initial position
	if model.scroller.selected != 0 {
		t.Errorf("initial selected should be 0, got %d", model.scroller.selected)
	}

	// Press 'j' to move down
	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m := result.(ImportBrowserModel)

	if m.scroller.selected != 1 {
		t.Errorf("after 'j', selected should be 1, got %d", m.scroller.selected)
	}

	// Press 'k' to move up
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = result.(ImportBrowserModel)

	if m.scroller.selected != 0 {
		t.Errorf("after 'k', selected should be 0, got %d", m.scroller.selected)
	}

	// Press 'G' to move to bottom
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	m = result.(ImportBrowserModel)

	expectedBottom := len(flatTree) - 1
	if m.scroller.selected != expectedBottom {
		t.Errorf("after 'G', selected should be %d, got %d", expectedBottom, m.scroller.selected)
	}

	// Press 'g' twice to move to top
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = result.(ImportBrowserModel)
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = result.(ImportBrowserModel)

	if m.scroller.selected != 0 {
		t.Errorf("after 'gg', selected should be 0, got %d", m.scroller.selected)
	}
}

// TestIntegrationBrowseExpandCollapse tests expand/collapse via key presses.
func TestIntegrationBrowseExpandCollapse(t *testing.T) {
	tmp := t.TempDir()

	// Create nested directory structure
	subdir := filepath.Join(tmp, "parent", "child")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	root, err := buildSourceTree(tmp, false)
	if err != nil {
		t.Fatalf("buildSourceTree: %v", err)
	}

	flatTree := flattenSourceTree(root)
	scroller := newSourceTreeScroller(flatTree, 20)

	model := ImportBrowserModel{
		state:        StateBrowse,
		root:         root,
		scroller:     scroller,
		rootPath:     tmp,
		sizeCache:    make(map[string]int64),
		sizePending:  make(map[string]struct{}),
		gitRootSet:   make(map[string]bool),
		ownerInput:   textinput.New(),
		projectInput: textinput.New(),
		height:       30,
		width:        80,
	}

	// Navigate to parent directory
	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m := result.(ImportBrowserModel)

	selectedNode := m.scroller.selectedNode()
	if selectedNode.Name != "parent" {
		t.Errorf("expected 'parent' selected, got %q", selectedNode.Name)
	}

	// Parent should not be expanded initially
	if selectedNode.IsExpanded {
		t.Error("parent should not be expanded initially")
	}

	// Press 'l' or Enter to expand
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = result.(ImportBrowserModel)

	selectedNode = m.scroller.selectedNode()
	if !selectedNode.IsExpanded {
		t.Error("parent should be expanded after 'l'")
	}

	// Press 'h' to collapse
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m = result.(ImportBrowserModel)

	selectedNode = m.scroller.selectedNode()
	if selectedNode.IsExpanded {
		t.Error("parent should be collapsed after 'h'")
	}
}

// TestIntegrationBrowseToImportConfig tests transition from Browse to ImportConfig.
func TestIntegrationBrowseToImportConfig(t *testing.T) {
	tmp := t.TempDir()

	// Create a project directory
	projectDir := filepath.Join(tmp, "myproject")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	root, err := buildSourceTree(tmp, false)
	if err != nil {
		t.Fatalf("buildSourceTree: %v", err)
	}

	flatTree := flattenSourceTree(root)
	scroller := newSourceTreeScroller(flatTree, 20)

	model := ImportBrowserModel{
		state:        StateBrowse,
		root:         root,
		scroller:     scroller,
		rootPath:     tmp,
		sizeCache:    make(map[string]int64),
		sizePending:  make(map[string]struct{}),
		gitRootSet:   make(map[string]bool),
		ownerInput:   textinput.New(),
		projectInput: textinput.New(),
		height:       30,
		width:        80,
	}

	// Navigate to myproject
	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m := result.(ImportBrowserModel)

	// Press 'i' to start import
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	m = result.(ImportBrowserModel)

	if m.state != StateImportConfig {
		t.Errorf("expected state=StateImportConfig, got %v", m.state)
	}

	if m.importTarget == nil {
		t.Error("importTarget should be set")
	} else if m.importTarget.Name != "myproject" {
		t.Errorf("importTarget.Name should be 'myproject', got %q", m.importTarget.Name)
	}

	// Project input should be pre-populated
	if m.projectInput.Value() != "myproject" {
		t.Errorf("projectInput should be 'myproject', got %q", m.projectInput.Value())
	}
}

// TestIntegrationImportConfigNavigation tests navigation in import config state.
func TestIntegrationImportConfigNavigation(t *testing.T) {
	model := ImportBrowserModel{
		state:          StateImportConfig,
		configFocusIdx: 0,
		ownerInput:     textinput.New(),
		projectInput:   textinput.New(),
		importTarget:   &sourceNode{Name: "project", Path: "/tmp/project"},
		height:         30,
		width:          80,
	}
	model.ownerInput.Focus()

	// Initially focused on owner (index 0)
	if model.configFocusIdx != 0 {
		t.Errorf("expected configFocusIdx=0, got %d", model.configFocusIdx)
	}

	// Press Tab to move to next field
	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	m := result.(ImportBrowserModel)

	if m.configFocusIdx != 1 {
		t.Errorf("expected configFocusIdx=1 after Tab, got %d", m.configFocusIdx)
	}

	// Press Tab again to cycle back
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = result.(ImportBrowserModel)

	if m.configFocusIdx != 0 {
		t.Errorf("expected configFocusIdx=0 after second Tab, got %d", m.configFocusIdx)
	}

	// Press Escape to go back to Browse
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = result.(ImportBrowserModel)

	if m.state != StateBrowse {
		t.Errorf("expected state=StateBrowse after Escape, got %v", m.state)
	}
}

// TestIntegrationStashFlow tests the stash confirmation flow.
func TestIntegrationStashFlow(t *testing.T) {
	tmp := t.TempDir()

	stashDir := filepath.Join(tmp, "tostash")
	if err := os.MkdirAll(stashDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	root, err := buildSourceTree(tmp, false)
	if err != nil {
		t.Fatalf("buildSourceTree: %v", err)
	}

	flatTree := flattenSourceTree(root)
	scroller := newSourceTreeScroller(flatTree, 20)

	model := ImportBrowserModel{
		state:          StateBrowse,
		root:           root,
		scroller:       scroller,
		rootPath:       tmp,
		sizeCache:      make(map[string]int64),
		sizePending:    make(map[string]struct{}),
		gitRootSet:     make(map[string]bool),
		ownerInput:     textinput.New(),
		projectInput:   textinput.New(),
		stashNameInput: textinput.New(),
		height:         30,
		width:          80,
	}

	// Navigate to tostash directory
	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m := result.(ImportBrowserModel)

	// Press 's' to start stash
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = result.(ImportBrowserModel)

	if m.state != StateStashConfirm {
		t.Errorf("expected state=StateStashConfirm, got %v", m.state)
	}

	if m.stashTarget == nil {
		t.Error("stashTarget should be set")
	}

	// Press Escape to cancel
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = result.(ImportBrowserModel)

	if m.state != StateBrowse {
		t.Errorf("expected state=StateBrowse after cancel, got %v", m.state)
	}
}

// TestIntegrationMultiSelectFlow tests multi-selection via key presses.
func TestIntegrationMultiSelectFlow(t *testing.T) {
	tmp := t.TempDir()

	for _, d := range []string{"dir1", "dir2", "dir3"} {
		if err := os.MkdirAll(filepath.Join(tmp, d), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}

	root, err := buildSourceTree(tmp, false)
	if err != nil {
		t.Fatalf("buildSourceTree: %v", err)
	}

	flatTree := flattenSourceTree(root)
	scroller := newSourceTreeScroller(flatTree, 20)

	model := ImportBrowserModel{
		state:        StateBrowse,
		root:         root,
		scroller:     scroller,
		rootPath:     tmp,
		sizeCache:    make(map[string]int64),
		sizePending:  make(map[string]struct{}),
		gitRootSet:   make(map[string]bool),
		ownerInput:   textinput.New(),
		projectInput: textinput.New(),
		height:       30,
		width:        80,
	}

	// Navigate to first directory
	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m := result.(ImportBrowserModel)

	// Initially no selections
	if m.scroller.getSelectedCount() != 0 {
		t.Errorf("expected 0 selected initially, got %d", m.scroller.getSelectedCount())
	}

	// Press Space to toggle selection
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = result.(ImportBrowserModel)

	if m.scroller.getSelectedCount() != 1 {
		t.Errorf("expected 1 selected after Space, got %d", m.scroller.getSelectedCount())
	}

	// Move down and select another
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = result.(ImportBrowserModel)
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = result.(ImportBrowserModel)

	if m.scroller.getSelectedCount() != 2 {
		t.Errorf("expected 2 selected, got %d", m.scroller.getSelectedCount())
	}

	// Toggle selection off on current item
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = result.(ImportBrowserModel)

	if m.scroller.getSelectedCount() != 1 {
		t.Errorf("expected 1 selected after toggle off, got %d", m.scroller.getSelectedCount())
	}
}

// TestIntegrationPostImportNavigation tests post-import option navigation.
func TestIntegrationPostImportNavigation(t *testing.T) {
	model := ImportBrowserModel{
		state:                StatePostImport,
		postImportOption:     0,
		postImportSourcePath: "/tmp/source",
		result: ImportBrowserResult{
			WorkspacePath: "/code/owner--project",
			WorkspaceSlug: "owner--project",
		},
		height: 30,
		width:  80,
	}

	// Initial option is 0 (Keep)
	if model.postImportOption != 0 {
		t.Errorf("expected postImportOption=0, got %d", model.postImportOption)
	}

	// Press 'j' to move down
	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m := result.(ImportBrowserModel)

	if m.postImportOption != 1 {
		t.Errorf("expected postImportOption=1 after 'j', got %d", m.postImportOption)
	}

	// Press '3' to quick-select option 3 (Delete)
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	m = result.(ImportBrowserModel)

	if m.postImportOption != 2 {
		t.Errorf("expected postImportOption=2 after '3', got %d", m.postImportOption)
	}

	// Press 'k' to move up
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = result.(ImportBrowserModel)

	if m.postImportOption != 1 {
		t.Errorf("expected postImportOption=1 after 'k', got %d", m.postImportOption)
	}
}

// TestIntegrationTemplateSelectFlow tests template selection via key presses.
func TestIntegrationTemplateSelectFlow(t *testing.T) {
	model := ImportBrowserModel{
		state: StateTemplateSelect,
		templateInfos: []template.TemplateInfo{
			{Name: "template1", Description: "First"},
			{Name: "template2", Description: "Second"},
		},
		templateSelected:     0,
		templateScrollOffset: 0,
		selectedTemplate:     "",
		importTarget:         &sourceNode{Name: "project", Path: "/tmp/project"},
		ownerInput:           textinput.New(),
		projectInput:         textinput.New(),
		height:               30,
		width:                80,
	}

	// Initial selection is "No template" (index 0)
	if model.templateSelected != 0 {
		t.Errorf("expected templateSelected=0, got %d", model.templateSelected)
	}

	// Press 'j' to move to first template
	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m := result.(ImportBrowserModel)

	if m.templateSelected != 1 {
		t.Errorf("expected templateSelected=1 after 'j', got %d", m.templateSelected)
	}

	// Press 'j' again to move to second template
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = result.(ImportBrowserModel)

	if m.templateSelected != 2 {
		t.Errorf("expected templateSelected=2 after second 'j', got %d", m.templateSelected)
	}

	// Press 'k' to move back up
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = result.(ImportBrowserModel)

	if m.templateSelected != 1 {
		t.Errorf("expected templateSelected=1 after 'k', got %d", m.templateSelected)
	}

	// Press Escape to go back
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = result.(ImportBrowserModel)

	if m.state != StateImportConfig {
		t.Errorf("expected state=StateImportConfig after Escape, got %v", m.state)
	}
}

// TestIntegrationQuitFromBrowse tests quit via q or ctrl+c.
func TestIntegrationQuitFromBrowse(t *testing.T) {
	tmp := t.TempDir()

	root, _ := buildSourceTree(tmp, false)
	flatTree := flattenSourceTree(root)
	scroller := newSourceTreeScroller(flatTree, 20)

	model := ImportBrowserModel{
		state:        StateBrowse,
		root:         root,
		scroller:     scroller,
		rootPath:     tmp,
		sizeCache:    make(map[string]int64),
		sizePending:  make(map[string]struct{}),
		gitRootSet:   make(map[string]bool),
		ownerInput:   textinput.New(),
		projectInput: textinput.New(),
		height:       30,
		width:        80,
	}

	// Press 'q' to quit
	result, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m := result.(ImportBrowserModel)

	// Result should have Aborted=true
	if !m.result.Aborted {
		t.Error("expected result.Aborted=true after 'q'")
	}

	// Command should be tea.Quit (non-nil)
	if cmd == nil {
		t.Error("expected non-nil cmd after quit")
	}
}

// TestIntegrationWindowResize tests window resize handling.
func TestIntegrationWindowResize(t *testing.T) {
	tmp := t.TempDir()

	root, _ := buildSourceTree(tmp, false)
	flatTree := flattenSourceTree(root)
	scroller := newSourceTreeScroller(flatTree, 20)

	model := ImportBrowserModel{
		state:       StateBrowse,
		root:        root,
		scroller:    scroller,
		rootPath:    tmp,
		sizeCache:   make(map[string]int64),
		sizePending: make(map[string]struct{}),
		gitRootSet:  make(map[string]bool),
		height:      30,
		width:       80,
	}

	// Send window size message
	result, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m := result.(ImportBrowserModel)

	if m.width != 120 {
		t.Errorf("expected width=120, got %d", m.width)
	}
	if m.height != 40 {
		t.Errorf("expected height=40, got %d", m.height)
	}
}
