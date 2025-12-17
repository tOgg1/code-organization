package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Guardrails to keep directory loading responsive.
var maxDirEntries = 500

// Styles for exclude picker
var (
	pickerTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	pickerHelpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	pickerSelectedStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("236")).
				Foreground(lipgloss.Color("212")).
				Bold(true)
	pickerExcludedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // red
	pickerIncludedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("34"))  // green
	pickerDirStyle      = lipgloss.NewStyle().Bold(true)
)

// ExcludePickerResult holds the result of the interactive exclude picker.
type ExcludePickerResult struct {
	Excludes      []string // final list of exclude patterns
	Confirmed     bool     // true if user confirmed, false if cancelled
	Aborted       bool     // true if user pressed ctrl+c/esc
	SaveRequested bool     // true if user wants to save excludes to project.json
}

// fileNode represents a node in the filesystem tree.
type fileNode struct {
	Name          string      // file/directory name
	Path          string      // absolute path
	RelPath       string      // path relative to workspace root
	IsDir         bool        // true if directory
	IsSymlink     bool        // true if entry is a symlink
	IsExpanded    bool        // true if directory is expanded (shows children)
	IsExcluded    bool        // true if this path is excluded from sync
	Depth         int         // indentation depth
	IsPlaceholder bool        // true if synthetic placeholder (e.g., truncated list)
	Children      []*fileNode // child nodes (only for directories)
}

// excludePickerModel is the Bubble Tea model for the exclude picker.
type excludePickerModel struct {
	workspacePath string              // absolute path to workspace
	root          *fileNode           // root of file tree
	flatTree      []*fileNode         // flattened tree for display
	selected      int                 // currently selected index in flatTree
	excludeSet    map[string]bool     // set of excluded relative paths
	defaultExcl   map[string]bool     // set of default excludes (patterns)
	width         int                 // terminal width
	height        int                 // terminal height
	done          bool                // true when picker is complete
	result        ExcludePickerResult // final result
	scrollOffset  int                 // for scrolling long lists
}

// newExcludePickerModel creates a new exclude picker model.
func newExcludePickerModel(workspacePath string, defaultExcludes []string) excludePickerModel {
	// Build default exclude set for pattern matching
	defaultSet := make(map[string]bool)
	for _, excl := range defaultExcludes {
		defaultSet[excl] = true
	}

	m := excludePickerModel{
		workspacePath: workspacePath,
		excludeSet:    make(map[string]bool),
		defaultExcl:   defaultSet,
		width:         80,
		height:        24,
	}

	// Build initial tree
	m.buildTree()

	return m
}

// buildTree builds the filesystem tree from the workspace root.
func (m *excludePickerModel) buildTree() {
	m.root = &fileNode{
		Name:       filepath.Base(m.workspacePath),
		Path:       m.workspacePath,
		RelPath:    ".",
		IsDir:      true,
		IsExpanded: true,
		Depth:      0,
		Children:   make([]*fileNode, 0),
	}

	// Build initial children (lazy loading)
	m.loadChildren(m.root)
	m.flattenTree()
}

// loadChildren loads the immediate children of a directory node.
// It avoids following symlinks and caps entries to prevent UI freezes.
func (m *excludePickerModel) loadChildren(node *fileNode) {
	if !node.IsDir {
		return
	}

	if node.IsExcluded {
		node.Children = []*fileNode{}
		return
	}

	if node.Children != nil && len(node.Children) > 0 {
		return
	}

	entries, err := os.ReadDir(node.Path)
	if err != nil {
		node.Children = []*fileNode{}
		return
	}

	node.Children = make([]*fileNode, 0, len(entries))

	// Sort entries: directories first, then files
	sort.Slice(entries, func(i, j int) bool {
		iDir := entries[i].IsDir()
		jDir := entries[j].IsDir()
		if iDir != jDir {
			return iDir // directories first
		}
		return entries[i].Name() < entries[j].Name()
	})

	addedCount := 0
	for _, entry := range entries {
		// Skip hidden files (except we want to show common ones like .git, .env)
		name := entry.Name()
		if strings.HasPrefix(name, ".") && name != ".git" && name != ".env" && name != ".gitignore" {
			continue
		}

		if addedCount >= maxDirEntries {
			node.Children = append(node.Children, &fileNode{
				Name:          "... more entries not shown",
				RelPath:       "",
				Depth:         node.Depth + 1,
				IsPlaceholder: true,
			})
			break
		}

		childPath := filepath.Join(node.Path, name)

		relPath := name
		if node.RelPath != "." {
			relPath = filepath.Join(node.RelPath, name)
		}

		isSymlink := entry.Type()&os.ModeSymlink != 0
		isDir := entry.IsDir() && !isSymlink

		child := &fileNode{
			Name:      name,
			Path:      childPath,
			RelPath:   relPath,
			IsDir:     isDir,
			IsSymlink: isSymlink,
			Depth:     node.Depth + 1,
		}

		// Check if this should be excluded by default
		if m.shouldDefaultExclude(child) {
			child.IsExcluded = true
			m.excludeSet[child.RelPath] = true
		}

		node.Children = append(node.Children, child)
		addedCount++
	}
}

// shouldDefaultExclude checks if a node matches any default exclude patterns.
func (m *excludePickerModel) shouldDefaultExclude(node *fileNode) bool {
	name := node.Name

	// Check exact matches
	if node.IsDir {
		if m.defaultExcl[name+"/"] {
			return true
		}
	} else {
		if m.defaultExcl[name] {
			return true
		}
	}

	// Check glob patterns (simple matching for common cases)
	for pattern := range m.defaultExcl {
		if strings.HasPrefix(pattern, "*.") {
			// Extension pattern like *.log
			ext := strings.TrimPrefix(pattern, "*")
			if strings.HasSuffix(name, ext) {
				return true
			}
		}
		if strings.HasPrefix(pattern, ".") && !strings.HasSuffix(pattern, "/") {
			// Hidden file pattern like .env.*
			if strings.HasPrefix(pattern, ".env") && strings.HasPrefix(name, ".env") {
				return true
			}
		}
	}

	return false
}

// flattenTree flattens the tree into a list for display.
func (m *excludePickerModel) flattenTree() {
	m.flatTree = make([]*fileNode, 0)
	if m.root != nil {
		m.flattenNode(m.root)
	}
}

// flattenNode recursively flattens a node and its visible children.
func (m *excludePickerModel) flattenNode(node *fileNode) {
	m.flatTree = append(m.flatTree, node)
	if node.IsDir && node.IsExpanded {
		for _, child := range node.Children {
			m.flattenNode(child)
		}
	}
}

// toggleExpand toggles the expanded state of a directory.
func (m *excludePickerModel) toggleExpand(node *fileNode) {
	if !node.IsDir {
		return
	}

	if node.IsExpanded {
		node.IsExpanded = false
	} else {
		node.IsExpanded = true
		// Lazy load children when expanding
		if len(node.Children) == 0 {
			m.loadChildren(node)
		}
	}
	m.flattenTree()
}

// toggleExclude toggles the exclude state of a node.
func (m *excludePickerModel) toggleExclude(node *fileNode) {
	if node.IsPlaceholder {
		return
	}

	node.IsExcluded = !node.IsExcluded

	if node.IsExcluded {
		m.excludeSet[node.RelPath] = true
	} else {
		delete(m.excludeSet, node.RelPath)
	}

	// If it's a directory, propagate to children
	if node.IsDir {
		m.propagateExclude(node, node.IsExcluded)
	}
}

// propagateExclude propagates exclude state to all children.
func (m *excludePickerModel) propagateExclude(node *fileNode, excluded bool) {
	for _, child := range node.Children {
		child.IsExcluded = excluded
		if excluded {
			m.excludeSet[child.RelPath] = true
		} else {
			delete(m.excludeSet, child.RelPath)
		}
		if child.IsDir {
			m.propagateExclude(child, excluded)
		}
	}
}

// getExcludePatterns returns the final list of exclude patterns.
func (m *excludePickerModel) getExcludePatterns() []string {
	// Convert excluded paths to patterns
	patterns := make([]string, 0)
	for path := range m.excludeSet {
		// Check if it's a directory by looking it up in the tree
		node := m.findNode(path)
		if node != nil && node.IsDir {
			patterns = append(patterns, path+"/")
		} else {
			patterns = append(patterns, path)
		}
	}
	sort.Strings(patterns)
	return patterns
}

// findNode finds a node by its relative path.
func (m *excludePickerModel) findNode(relPath string) *fileNode {
	for _, node := range m.flatTree {
		if node.RelPath == relPath {
			return node
		}
	}
	return nil
}

// clearExcludes removes all exclusions.
func (m *excludePickerModel) clearExcludes() {
	m.excludeSet = make(map[string]bool)
	m.clearExcludeState(m.root)
}

// clearExcludeState recursively clears exclude state.
func (m *excludePickerModel) clearExcludeState(node *fileNode) {
	node.IsExcluded = false
	for _, child := range node.Children {
		m.clearExcludeState(child)
	}
}

// resetToDefaults resets exclusions to default patterns.
func (m *excludePickerModel) resetToDefaults() {
	m.excludeSet = make(map[string]bool)
	m.resetDefaultState(m.root)
	m.flattenTree()
}

// resetDefaultState recursively resets nodes to default exclude state.
func (m *excludePickerModel) resetDefaultState(node *fileNode) {
	if m.shouldDefaultExclude(node) {
		node.IsExcluded = true
		m.excludeSet[node.RelPath] = true
	} else {
		node.IsExcluded = false
	}
	for _, child := range node.Children {
		m.resetDefaultState(child)
	}
}

// Init implements tea.Model.
func (m excludePickerModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m excludePickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			m.result.Aborted = true
			m.done = true
			return m, tea.Quit

		case "enter":
			m.result.Excludes = m.getExcludePatterns()
			m.result.Confirmed = true
			m.done = true
			return m, tea.Quit

		case "S":
			// Save excludes to project.json and sync
			m.result.Excludes = m.getExcludePatterns()
			m.result.Confirmed = true
			m.result.SaveRequested = true
			m.done = true
			return m, tea.Quit

		case "j", "down":
			if m.selected < len(m.flatTree)-1 {
				m.selected++
				m.ensureVisible()
			}
			return m, nil

		case "k", "up":
			if m.selected > 0 {
				m.selected--
				m.ensureVisible()
			}
			return m, nil

		case "g":
			// Go to top
			m.selected = 0
			m.scrollOffset = 0
			return m, nil

		case "G":
			// Go to bottom
			if len(m.flatTree) > 0 {
				m.selected = len(m.flatTree) - 1
				m.ensureVisible()
			}
			return m, nil

		case " ":
			// Toggle exclude
			if m.selected < len(m.flatTree) {
				node := m.flatTree[m.selected]
				m.toggleExclude(node)
			}
			return m, nil

		case "l", "right":
			// Expand directory
			if m.selected < len(m.flatTree) {
				node := m.flatTree[m.selected]
				if node.IsDir && !node.IsExpanded {
					m.toggleExpand(node)
				}
			}
			return m, nil

		case "h", "left":
			// Collapse directory
			if m.selected < len(m.flatTree) {
				node := m.flatTree[m.selected]
				if node.IsDir && node.IsExpanded {
					m.toggleExpand(node)
				}
			}
			return m, nil

		case "c":
			// Clear all exclusions
			m.clearExcludes()
			return m, nil

		case "r":
			// Reset to defaults
			m.resetToDefaults()
			return m, nil
		}
	}

	return m, nil
}

// ensureVisible ensures the selected item is visible in the viewport.
func (m *excludePickerModel) ensureVisible() {
	visibleLines := m.height - 8 // account for header/footer
	if visibleLines < 5 {
		visibleLines = 5
	}

	if m.selected < m.scrollOffset {
		m.scrollOffset = m.selected
	} else if m.selected >= m.scrollOffset+visibleLines {
		m.scrollOffset = m.selected - visibleLines + 1
	}
}

// View implements tea.Model.
func (m excludePickerModel) View() string {
	var sb strings.Builder

	// Title
	sb.WriteString(pickerTitleStyle.Render("Sync Exclude Picker") + "\n")
	sb.WriteString(pickerHelpStyle.Render(fmt.Sprintf("Workspace: %s", filepath.Base(m.workspacePath))) + "\n\n")

	// Calculate visible area
	visibleLines := m.height - 8
	if visibleLines < 5 {
		visibleLines = 5
	}

	// Render tree
	startIdx := m.scrollOffset
	endIdx := startIdx + visibleLines
	if endIdx > len(m.flatTree) {
		endIdx = len(m.flatTree)
	}

	for i := startIdx; i < endIdx; i++ {
		node := m.flatTree[i]
		line := m.renderNode(node, i == m.selected)
		sb.WriteString(line + "\n")
	}

	// Scroll indicator
	if len(m.flatTree) > visibleLines {
		sb.WriteString(fmt.Sprintf("\n(%d/%d)", m.selected+1, len(m.flatTree)))
	}

	// Summary
	excludeCount := len(m.excludeSet)
	sb.WriteString(fmt.Sprintf("\n\n%d paths excluded", excludeCount))

	// Help
	sb.WriteString("\n\n" + pickerHelpStyle.Render("j/k: navigate • space: toggle • h/l: collapse/expand • c: clear • r: reset"))
	sb.WriteString("\n" + pickerHelpStyle.Render("enter: sync • S: save & sync • q: cancel"))

	return sb.String()
}

// renderNode renders a single tree node.
func (m excludePickerModel) renderNode(node *fileNode, isSelected bool) string {
	// Indentation
	indent := strings.Repeat("  ", node.Depth)

	// Icon
	var icon string
	if node.IsDir {
		if node.IsExpanded {
			icon = "▼ "
		} else {
			icon = "▶ "
		}
	} else {
		icon = "  "
	}

	// Exclude marker
	var marker string
	if node.IsExcluded {
		marker = "✗ "
	} else {
		marker = "✓ "
	}

	// Name styling
	name := node.Name
	if node.IsDir {
		name = pickerDirStyle.Render(name + "/")
	}

	// Build line
	line := fmt.Sprintf("%s%s%s%s", indent, marker, icon, name)

	// Apply selection/exclude styling
	if isSelected {
		line = pickerSelectedStyle.Render(line)
	} else if node.IsExcluded {
		line = pickerExcludedStyle.Render(line)
	} else {
		line = pickerIncludedStyle.Render(line)
	}

	return line
}

// RunExcludePicker runs the interactive exclude picker TUI.
func RunExcludePicker(workspacePath string, defaultExcludes []string) (ExcludePickerResult, error) {
	m := newExcludePickerModel(workspacePath, defaultExcludes)
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return ExcludePickerResult{Aborted: true}, err
	}

	result := finalModel.(excludePickerModel).result
	return result, nil
}
