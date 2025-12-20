package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tormodhaugland/co/internal/archive"
	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/git"
)

// sourceNode represents a node in the source folder tree for the import browser.
// It tracks filesystem structure along with git repository detection.
type sourceNode struct {
	Name        string        // file/directory name
	Path        string        // absolute path
	RelPath     string        // path relative to browse root
	IsDir       bool          // true if directory
	IsExpanded  bool          // true if directory is expanded (shows children)
	IsSelected  bool          // true if selected for batch operations
	IsGitRepo   bool          // true if this directory is a git repository root
	GitInfo     *git.RepoInfo // git info if IsGitRepo is true, nil otherwise
	HasGitChild bool          // true if any descendant is a git repository
	IsSymlink   bool          // true if this is a symbolic link
	Depth       int           // indentation depth in tree
	Children    []*sourceNode // child nodes (only for directories)
}

// ImportBrowserState represents the current state of the import browser TUI.
type ImportBrowserState int

const (
	StateBrowse        ImportBrowserState = iota // Browsing the source folder tree
	StateImportConfig                            // Configuring import (owner/project input)
	StateExtraFiles                              // Selecting extra non-git files to include
	StateImportPreview                           // Previewing import operation
	StateImportExecute                           // Executing import operation
	StateStashConfirm                            // Confirming stash operation
	StateStashExecute                            // Executing stash operation
	StateAddToSelect                             // Selecting workspace for add-to mode
	StateComplete                                // Operation completed
)

// String returns the string representation of the state.
func (s ImportBrowserState) String() string {
	switch s {
	case StateBrowse:
		return "Browse"
	case StateImportConfig:
		return "Import Config"
	case StateExtraFiles:
		return "Extra Files"
	case StateImportPreview:
		return "Import Preview"
	case StateImportExecute:
		return "Importing"
	case StateStashConfirm:
		return "Stash Confirm"
	case StateStashExecute:
		return "Stashing"
	case StateAddToSelect:
		return "Add To Workspace"
	case StateComplete:
		return "Complete"
	default:
		return "Unknown"
	}
}

// ImportBrowserResult holds the result of the import browser session.
type ImportBrowserResult struct {
	// Action taken
	Action string // "import", "stash", "add-to", "none"

	// Import results
	WorkspacePath string   // path to created/updated workspace
	WorkspaceSlug string   // slug of created/updated workspace
	ReposImported []string // names of repos imported
	FilesImported []string // paths of extra files imported

	// Stash results
	ArchivePath   string // path to created archive
	SourceStashed string // path that was stashed

	// Status
	Success bool  // true if operation succeeded
	Error   error // error if operation failed
	Aborted bool  // true if user cancelled
}

// maxSourceDirEntries limits entries per directory to keep UI responsive.
const maxSourceDirEntries = 500

// buildSourceTree creates the root node and detects git repositories.
// It scans for git repos first, then builds the tree structure.
func buildSourceTree(rootPath string) (*sourceNode, error) {
	info, err := os.Stat(rootPath)
	if err != nil {
		return nil, err
	}

	// Find all git repositories in the tree
	gitRoots, err := git.FindGitRoots(rootPath)
	if err != nil {
		return nil, err
	}

	// Build set for quick lookup
	gitRootSet := make(map[string]bool)
	for _, root := range gitRoots {
		gitRootSet[root] = true
	}

	// Create root node
	root := &sourceNode{
		Name:       info.Name(),
		Path:       rootPath,
		RelPath:    ".",
		IsDir:      info.IsDir(),
		IsExpanded: true, // Root is expanded by default
		Depth:      0,
	}

	// Check if root itself is a git repo
	if gitRootSet[rootPath] {
		root.IsGitRepo = true
		if gitInfo, err := git.GetInfo(rootPath); err == nil {
			root.GitInfo = gitInfo
		}
	}

	// Load immediate children and mark HasGitChild
	if root.IsDir {
		loadSourceChildren(root, gitRootSet)
		root.HasGitChild = hasGitDescendant(root, gitRootSet)
	}

	return root, nil
}

// loadSourceChildren loads the immediate children of a directory node.
func loadSourceChildren(node *sourceNode, gitRootSet map[string]bool) {
	if !node.IsDir || node.IsSymlink {
		return
	}

	entries, err := os.ReadDir(node.Path)
	if err != nil {
		node.Children = []*sourceNode{}
		return
	}

	// Sort: directories first, then by name
	sort.Slice(entries, func(i, j int) bool {
		iDir := entries[i].IsDir()
		jDir := entries[j].IsDir()
		if iDir != jDir {
			return iDir
		}
		return entries[i].Name() < entries[j].Name()
	})

	node.Children = make([]*sourceNode, 0, len(entries))
	addedCount := 0

	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden files except common useful ones
		if strings.HasPrefix(name, ".") && name != ".env" && name != ".gitignore" && name != ".git" {
			continue
		}

		if addedCount >= maxSourceDirEntries {
			// Add placeholder for truncated list
			node.Children = append(node.Children, &sourceNode{
				Name:    "... more entries not shown",
				RelPath: "",
				Depth:   node.Depth + 1,
			})
			break
		}

		childPath := filepath.Join(node.Path, name)
		relPath := name
		if node.RelPath != "." {
			relPath = filepath.Join(node.RelPath, name)
		}

		// Check for symlink
		fileInfo, err := entry.Info()
		if err != nil {
			continue
		}
		isSymlink := fileInfo.Mode()&os.ModeSymlink != 0

		// For symlinks, don't follow them (prevent infinite loops)
		isDir := entry.IsDir() && !isSymlink

		child := &sourceNode{
			Name:      name,
			Path:      childPath,
			RelPath:   relPath,
			IsDir:     isDir,
			IsSymlink: isSymlink,
			Depth:     node.Depth + 1,
		}

		// Check if this is a git repo
		if isDir && gitRootSet[childPath] {
			child.IsGitRepo = true
			if gitInfo, err := git.GetInfo(childPath); err == nil {
				child.GitInfo = gitInfo
			}
		}

		// Check if any descendant is a git repo (for display purposes)
		if isDir {
			child.HasGitChild = hasGitDescendant(child, gitRootSet)
		}

		node.Children = append(node.Children, child)
		addedCount++
	}
}

// hasGitDescendant checks if any path in gitRootSet is a descendant of node.
func hasGitDescendant(node *sourceNode, gitRootSet map[string]bool) bool {
	if node.IsGitRepo {
		return false // Don't count self
	}

	prefix := node.Path + string(filepath.Separator)
	for gitRoot := range gitRootSet {
		if strings.HasPrefix(gitRoot, prefix) {
			return true
		}
	}
	return false
}

// expandNode expands a directory node, loading its children if needed.
func (node *sourceNode) expandNode(gitRootSet map[string]bool) {
	if !node.IsDir || node.IsExpanded {
		return
	}

	node.IsExpanded = true

	// Load children if not already loaded
	if node.Children == nil {
		loadSourceChildren(node, gitRootSet)
	}
}

// collapseNode collapses a directory node.
func (node *sourceNode) collapseNode() {
	if node.IsDir {
		node.IsExpanded = false
	}
}

// toggleExpand toggles the expanded state of a directory.
func (node *sourceNode) toggleExpand(gitRootSet map[string]bool) {
	if !node.IsDir {
		return
	}

	if node.IsExpanded {
		node.collapseNode()
	} else {
		node.expandNode(gitRootSet)
	}
}

// flattenSourceTree flattens the tree into a display list.
// Only includes expanded directories' children.
func flattenSourceTree(root *sourceNode) []*sourceNode {
	if root == nil {
		return nil
	}

	var result []*sourceNode
	flattenSourceNode(root, &result)
	return result
}

// flattenSourceNode recursively flattens a node and its visible children.
func flattenSourceNode(node *sourceNode, result *[]*sourceNode) {
	*result = append(*result, node)

	if node.IsDir && node.IsExpanded {
		for _, child := range node.Children {
			flattenSourceNode(child, result)
		}
	}
}

// sourceTreeScroller manages scroll state for the flattened tree display.
type sourceTreeScroller struct {
	flatTree     []*sourceNode
	selected     int
	scrollOffset int
	height       int // visible lines for scrolling
}

// newSourceTreeScroller creates a new scroller with the given tree.
func newSourceTreeScroller(flatTree []*sourceNode, visibleHeight int) *sourceTreeScroller {
	return &sourceTreeScroller{
		flatTree: flatTree,
		selected: 0,
		height:   visibleHeight,
	}
}

// updateTree updates the flat tree and adjusts selection if needed.
func (s *sourceTreeScroller) updateTree(flatTree []*sourceNode) {
	s.flatTree = flatTree
	// Ensure selected is still valid
	if s.selected >= len(s.flatTree) {
		s.selected = len(s.flatTree) - 1
	}
	if s.selected < 0 {
		s.selected = 0
	}
	s.ensureVisible()
}

// setHeight updates the visible height.
func (s *sourceTreeScroller) setHeight(height int) {
	s.height = height
	s.ensureVisible()
}

// moveUp moves selection up one item.
func (s *sourceTreeScroller) moveUp() {
	if s.selected > 0 {
		s.selected--
		s.ensureVisible()
	}
}

// moveDown moves selection down one item.
func (s *sourceTreeScroller) moveDown() {
	if s.selected < len(s.flatTree)-1 {
		s.selected++
		s.ensureVisible()
	}
}

// moveToTop moves selection to the first item.
func (s *sourceTreeScroller) moveToTop() {
	s.selected = 0
	s.scrollOffset = 0
}

// moveToBottom moves selection to the last item.
func (s *sourceTreeScroller) moveToBottom() {
	if len(s.flatTree) > 0 {
		s.selected = len(s.flatTree) - 1
		s.ensureVisible()
	}
}

// ensureVisible ensures the selected item is visible in the viewport.
func (s *sourceTreeScroller) ensureVisible() {
	if s.height <= 0 {
		return
	}

	// Scroll up if needed
	if s.selected < s.scrollOffset {
		s.scrollOffset = s.selected
	}

	// Scroll down if needed
	if s.selected >= s.scrollOffset+s.height {
		s.scrollOffset = s.selected - s.height + 1
	}
}

// visibleRange returns the start and end indices of visible items.
func (s *sourceTreeScroller) visibleRange() (start, end int) {
	start = s.scrollOffset
	end = s.scrollOffset + s.height
	if end > len(s.flatTree) {
		end = len(s.flatTree)
	}
	return start, end
}

// selectedNode returns the currently selected node, or nil if none.
func (s *sourceTreeScroller) selectedNode() *sourceNode {
	if s.selected >= 0 && s.selected < len(s.flatTree) {
		return s.flatTree[s.selected]
	}
	return nil
}

// isSelected returns true if the given index is the selected item.
func (s *sourceTreeScroller) isSelected(index int) bool {
	return index == s.selected
}

// Styles for the import browser
var (
	ibTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212"))

	ibPaneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 1)

	ibActivePaneStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("212")).
				Padding(0, 1)

	ibHelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	ibSelectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("212")).
			Bold(true)

	ibDirStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39"))

	ibGitRepoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("40"))

	ibGitDirtyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

	ibSymlinkStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("141")).
			Italic(true)

	ibFileStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	ibHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212")).
			MarginBottom(1)

	ibErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	ibSuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("40"))
)

// ImportBrowserPane represents which pane is focused.
type ImportBrowserPane int

const (
	IBPaneTree ImportBrowserPane = iota
	IBPaneDetails
)

// ImportBrowserModel is the main model for the interactive import browser.
type ImportBrowserModel struct {
	cfg        *config.Config
	rootPath   string
	root       *sourceNode
	gitRootSet map[string]bool
	scroller   *sourceTreeScroller

	state      ImportBrowserState
	activePane ImportBrowserPane
	width      int
	height     int

	message        string
	messageIsError bool

	// Import config state
	importTarget   *sourceNode     // The folder being imported
	ownerInput     textinput.Model // Owner input field
	projectInput   textinput.Model // Project input field
	configFocusIdx int             // 0 = owner, 1 = project
	configError    string          // Validation error

	// Stash config state
	stashTarget      *sourceNode     // The folder being stashed
	stashNameInput   textinput.Model // Custom archive name input
	stashDeleteAfter bool            // Whether to delete after stashing
	stashFocusIdx    int             // 0 = name, 1 = delete option
	stashError       string          // Stash validation error

	// Extra files state
	extraFilesItems        []extraFileItem  // Non-git items found
	extraFilesSelected     int              // Currently selected item index
	extraFilesScrollOffset int              // Scroll offset for long lists
	extraFilesShowDest     bool             // Show destination prompt
	extraFilesDestInput    textinput.Model  // Destination subfolder input
	extraFilesResult       ExtraFilesResult // Selected files result

	result ImportBrowserResult
}

// NewImportBrowser creates a new import browser model.
func NewImportBrowser(cfg *config.Config, rootPath string) (*ImportBrowserModel, error) {
	// Build the source tree
	root, err := buildSourceTree(rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build source tree: %w", err)
	}

	// Build git root set for expand operations
	gitRoots, _ := git.FindGitRoots(rootPath)
	gitRootSet := make(map[string]bool)
	for _, r := range gitRoots {
		gitRootSet[r] = true
	}

	// Flatten tree and create scroller
	flatTree := flattenSourceTree(root)
	scroller := newSourceTreeScroller(flatTree, 20) // Default height, updated on resize

	// Initialize text inputs for import config
	ownerInput := textinput.New()
	ownerInput.Placeholder = "owner"
	ownerInput.CharLimit = 64
	ownerInput.Width = 30

	projectInput := textinput.New()
	projectInput.Placeholder = "project"
	projectInput.CharLimit = 64
	projectInput.Width = 30

	// Initialize text input for stash config
	stashNameInput := textinput.New()
	stashNameInput.Placeholder = "archive name (optional)"
	stashNameInput.CharLimit = 128
	stashNameInput.Width = 40

	// Initialize text input for extra files destination
	extraFilesDestInput := textinput.New()
	extraFilesDestInput.Placeholder = "subfolder (leave empty for project root)"
	extraFilesDestInput.CharLimit = 128
	extraFilesDestInput.Width = 50

	return &ImportBrowserModel{
		cfg:                 cfg,
		rootPath:            rootPath,
		root:                root,
		gitRootSet:          gitRootSet,
		scroller:            scroller,
		state:               StateBrowse,
		activePane:          IBPaneTree,
		ownerInput:          ownerInput,
		projectInput:        projectInput,
		stashNameInput:      stashNameInput,
		extraFilesDestInput: extraFilesDestInput,
	}, nil
}

// Init implements tea.Model.
func (m ImportBrowserModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m ImportBrowserModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Update scroller height (leave room for header, footer, borders)
		visibleHeight := msg.Height - 8
		if visibleHeight < 5 {
			visibleHeight = 5
		}
		m.scroller.setHeight(visibleHeight)
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	}

	return m, nil
}

// handleKeyPress handles keyboard input based on current state.
func (m ImportBrowserModel) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case StateBrowse:
		return m.handleBrowseKeys(msg)
	case StateImportConfig:
		return m.handleImportConfigKeys(msg)
	case StateImportPreview:
		return m.handleImportPreviewKeys(msg)
	case StateStashConfirm:
		return m.handleStashConfirmKeys(msg)
	case StateExtraFiles:
		return m.handleExtraFilesKeys(msg)
	default:
		// Other states will be handled in future tasks
		return m, nil
	}
}

// handleImportPreviewKeys handles keyboard input in import preview state.
func (m ImportBrowserModel) handleImportPreviewKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.result.Aborted = true
		return m, tea.Quit

	case "esc":
		// Go back - to extra files if there were any, otherwise to config
		if len(m.extraFilesItems) > 0 {
			m.state = StateExtraFiles
		} else {
			m.state = StateImportConfig
			return m, m.ownerInput.Focus()
		}
		return m, nil

	case "enter":
		// Execute import
		m.state = StateImportExecute
		// For now, just mark as complete (actual execution will be added later)
		m.result.Action = "import"
		m.result.Success = true
		m.message = fmt.Sprintf("Would create workspace: %s", m.result.WorkspaceSlug)
		m.messageIsError = false
		m.state = StateBrowse // Return to browse for now
		return m, nil
	}
	return m, nil
}

// handleBrowseKeys handles keyboard input in browse state.
func (m ImportBrowserModel) handleBrowseKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.result.Aborted = true
		return m, tea.Quit

	case "j", "down":
		m.scroller.moveDown()
		return m, nil

	case "k", "up":
		m.scroller.moveUp()
		return m, nil

	case "g":
		m.scroller.moveToTop()
		return m, nil

	case "G":
		m.scroller.moveToBottom()
		return m, nil

	case "l", "right":
		node := m.scroller.selectedNode()
		if node != nil && node.IsDir && !node.IsExpanded {
			node.expandNode(m.gitRootSet)
			m.refreshTree()
		} else if m.activePane == IBPaneTree {
			m.activePane = IBPaneDetails
		}
		return m, nil

	case "h", "left":
		node := m.scroller.selectedNode()
		if node != nil && node.IsDir && node.IsExpanded {
			node.collapseNode()
			m.refreshTree()
		} else if m.activePane == IBPaneDetails {
			m.activePane = IBPaneTree
		}
		return m, nil

	case "enter":
		node := m.scroller.selectedNode()
		if node != nil && node.IsDir {
			node.toggleExpand(m.gitRootSet)
			m.refreshTree()
		}
		return m, nil

	case " ":
		// Toggle selection for batch operations
		node := m.scroller.selectedNode()
		if node != nil && node.IsDir {
			node.IsSelected = !node.IsSelected
		}
		return m, nil

	case "tab":
		// Switch panes
		if m.activePane == IBPaneTree {
			m.activePane = IBPaneDetails
		} else {
			m.activePane = IBPaneTree
		}
		return m, nil

	case "r":
		// Refresh tree
		m.refresh()
		return m, nil

	case "i":
		// Start import for selected folder
		node := m.scroller.selectedNode()
		if node != nil && node.IsDir {
			m.startImport(node)
			return m, m.ownerInput.Focus()
		}
		return m, nil

	case "s":
		// Start stash for selected folder (keep source)
		node := m.scroller.selectedNode()
		if node != nil && node.IsDir {
			m.startStash(node, false)
			return m, m.stashNameInput.Focus()
		}
		return m, nil

	case "S":
		// Start stash for selected folder (delete source after)
		node := m.scroller.selectedNode()
		if node != nil && node.IsDir {
			m.startStash(node, true)
			return m, m.stashNameInput.Focus()
		}
		return m, nil
	}

	return m, nil
}

// startImport initializes the import config state for the selected folder.
func (m *ImportBrowserModel) startImport(node *sourceNode) {
	m.state = StateImportConfig
	m.importTarget = node
	m.configFocusIdx = 0
	m.configError = ""

	// Pre-populate project name from folder name
	suggestedProject := sanitizeForSlug(node.Name)
	m.projectInput.SetValue(suggestedProject)
	m.ownerInput.SetValue("")
}

// sanitizeForSlug converts a string to a valid slug part.
func sanitizeForSlug(s string) string {
	s = strings.ToLower(s)
	var result strings.Builder
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			result.WriteRune(c)
		} else if c == '_' || c == ' ' {
			result.WriteRune('-')
		}
	}
	return result.String()
}

// handleImportConfigKeys handles keyboard input in import config state.
func (m ImportBrowserModel) handleImportConfigKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.result.Aborted = true
		return m, tea.Quit

	case "esc":
		// Cancel import, return to browse
		m.state = StateBrowse
		m.importTarget = nil
		m.configError = ""
		m.ownerInput.Blur()
		m.projectInput.Blur()
		return m, nil

	case "tab", "down":
		// Move to next field
		m.configFocusIdx = (m.configFocusIdx + 1) % 2
		m.ownerInput.Blur()
		m.projectInput.Blur()
		if m.configFocusIdx == 0 {
			return m, m.ownerInput.Focus()
		}
		return m, m.projectInput.Focus()

	case "shift+tab", "up":
		// Move to previous field
		m.configFocusIdx = (m.configFocusIdx + 1) % 2
		m.ownerInput.Blur()
		m.projectInput.Blur()
		if m.configFocusIdx == 0 {
			return m, m.ownerInput.Focus()
		}
		return m, m.projectInput.Focus()

	case "enter":
		// Validate and proceed
		owner := strings.ToLower(strings.TrimSpace(m.ownerInput.Value()))
		project := strings.ToLower(strings.TrimSpace(m.projectInput.Value()))

		if owner == "" {
			m.configError = "owner is required"
			return m, nil
		}
		if project == "" {
			m.configError = "project is required"
			return m, nil
		}
		if !isValidSlugPart(owner) {
			m.configError = "owner must be lowercase alphanumeric with hyphens"
			return m, nil
		}
		if !isValidSlugPart(project) {
			m.configError = "project must be lowercase alphanumeric with hyphens"
			return m, nil
		}

		// Check if workspace already exists
		slug := owner + "--" + project
		workspacePath := filepath.Join(m.cfg.CodeRoot, slug)
		if _, err := os.Stat(workspacePath); err == nil {
			m.configError = fmt.Sprintf("workspace already exists: %s", slug)
			return m, nil
		}

		// Store config and move to preview (or execute directly for now)
		m.result.WorkspaceSlug = slug
		m.result.WorkspacePath = workspacePath
		m.configError = ""

		// Check for non-git files
		return m.checkForExtraFiles()
	}

	// Update the focused input
	var cmd tea.Cmd
	if m.configFocusIdx == 0 {
		m.ownerInput, cmd = m.ownerInput.Update(msg)
	} else {
		m.projectInput, cmd = m.projectInput.Update(msg)
	}
	return m, cmd
}

// startStash initializes the stash config state for the selected folder.
func (m *ImportBrowserModel) startStash(node *sourceNode, deleteAfter bool) {
	m.state = StateStashConfirm
	m.stashTarget = node
	m.stashDeleteAfter = deleteAfter
	m.stashFocusIdx = 0
	m.stashError = ""

	// Pre-populate archive name from folder name
	suggestedName := archive.SanitizeArchiveName(node.Name)
	m.stashNameInput.SetValue(suggestedName)
}

// handleStashConfirmKeys handles keyboard input in stash confirm state.
func (m ImportBrowserModel) handleStashConfirmKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.result.Aborted = true
		return m, tea.Quit

	case "esc":
		// Cancel stash, return to browse
		m.state = StateBrowse
		m.stashTarget = nil
		m.stashError = ""
		m.stashNameInput.Blur()
		return m, nil

	case "tab", "down":
		// Toggle between name field and delete option
		m.stashFocusIdx = (m.stashFocusIdx + 1) % 2
		if m.stashFocusIdx == 0 {
			return m, m.stashNameInput.Focus()
		}
		m.stashNameInput.Blur()
		return m, nil

	case "shift+tab", "up":
		// Toggle between name field and delete option
		m.stashFocusIdx = (m.stashFocusIdx + 1) % 2
		if m.stashFocusIdx == 0 {
			return m, m.stashNameInput.Focus()
		}
		m.stashNameInput.Blur()
		return m, nil

	case " ":
		// Toggle delete option when focused on it
		if m.stashFocusIdx == 1 {
			m.stashDeleteAfter = !m.stashDeleteAfter
		}
		return m, nil

	case "d", "D":
		// Quick toggle delete option
		m.stashDeleteAfter = !m.stashDeleteAfter
		return m, nil

	case "enter":
		// Execute stash
		return m.executeStash()
	}

	// Update the name input if focused
	if m.stashFocusIdx == 0 {
		var cmd tea.Cmd
		m.stashNameInput, cmd = m.stashNameInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

// executeStash performs the actual stash operation.
func (m ImportBrowserModel) executeStash() (tea.Model, tea.Cmd) {
	if m.stashTarget == nil {
		m.stashError = "no folder selected"
		return m, nil
	}

	m.state = StateStashExecute

	// Get archive name
	name := strings.TrimSpace(m.stashNameInput.Value())
	if name == "" {
		name = m.stashTarget.Name
	}

	opts := archive.StashOptions{
		Name:        name,
		DeleteAfter: m.stashDeleteAfter,
	}

	result, err := archive.StashFolder(m.cfg, m.stashTarget.Path, opts)
	if err != nil {
		m.stashError = err.Error()
		m.state = StateStashConfirm
		return m, nil
	}

	// Success - store results
	m.result.Action = "stash"
	m.result.Success = true
	m.result.ArchivePath = result.ArchivePath
	m.result.SourceStashed = result.SourcePath

	// If deleted, refresh tree
	if result.Deleted {
		m.refresh()
	}

	// Show success message and return to browse
	m.message = fmt.Sprintf("Stashed: %s", result.ArchivePath)
	if result.Deleted {
		m.message += " (source deleted)"
	}
	m.messageIsError = false
	m.state = StateBrowse
	m.stashTarget = nil

	return m, nil
}

// checkForExtraFiles looks for non-git files and transitions to the appropriate state.
func (m ImportBrowserModel) checkForExtraFiles() (tea.Model, tea.Cmd) {
	if m.importTarget == nil {
		m.state = StateImportPreview
		return m, nil
	}

	// Get git roots under the import target
	var gitRoots []string
	if m.importTarget.IsGitRepo {
		gitRoots = []string{m.importTarget.Path}
	} else {
		prefix := m.importTarget.Path + string(filepath.Separator)
		for gitRoot := range m.gitRootSet {
			if strings.HasPrefix(gitRoot, prefix) {
				gitRoots = append(gitRoots, gitRoot)
			}
		}
	}

	// Find non-git items
	items, err := FindNonGitItems(m.importTarget.Path, gitRoots)
	if err != nil || len(items) == 0 {
		// No extra files or error finding them, skip to preview
		m.state = StateImportPreview
		return m, nil
	}

	// Initialize extra files state
	m.extraFilesItems = items
	m.extraFilesSelected = 0
	m.extraFilesScrollOffset = 0
	m.extraFilesShowDest = false
	m.extraFilesDestInput.SetValue("")
	m.extraFilesResult = ExtraFilesResult{}
	m.state = StateExtraFiles

	return m, nil
}

// handleExtraFilesKeys handles keyboard input in extra files selection state.
func (m ImportBrowserModel) handleExtraFilesKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle destination prompt mode
	if m.extraFilesShowDest {
		return m.handleExtraFilesDestKeys(msg)
	}

	switch msg.String() {
	case "ctrl+c":
		m.result.Aborted = true
		return m, tea.Quit

	case "esc", "q":
		// Skip extra files, go directly to preview
		m.extraFilesResult.Confirmed = true
		m.extraFilesResult.SelectedPaths = nil
		m.state = StateImportPreview
		return m, nil

	case "enter":
		// If nothing is selected, skip to preview
		selected := m.getExtraFilesSelectedPaths()
		if len(selected) == 0 {
			m.extraFilesResult.Confirmed = true
			m.state = StateImportPreview
			return m, nil
		}
		// Move to destination prompt
		m.extraFilesShowDest = true
		return m, m.extraFilesDestInput.Focus()

	case "j", "down":
		if m.extraFilesSelected < len(m.extraFilesItems)-1 {
			m.extraFilesSelected++
			m.ensureExtraFilesVisible()
		}
		return m, nil

	case "k", "up":
		if m.extraFilesSelected > 0 {
			m.extraFilesSelected--
			m.ensureExtraFilesVisible()
		}
		return m, nil

	case "g":
		m.extraFilesSelected = 0
		m.extraFilesScrollOffset = 0
		return m, nil

	case "G":
		if len(m.extraFilesItems) > 0 {
			m.extraFilesSelected = len(m.extraFilesItems) - 1
			m.ensureExtraFilesVisible()
		}
		return m, nil

	case " ":
		// Toggle selection
		if m.extraFilesSelected < len(m.extraFilesItems) {
			m.extraFilesItems[m.extraFilesSelected].Checked = !m.extraFilesItems[m.extraFilesSelected].Checked
		}
		return m, nil

	case "a":
		// Select all
		for i := range m.extraFilesItems {
			m.extraFilesItems[i].Checked = true
		}
		return m, nil

	case "n":
		// Select none
		for i := range m.extraFilesItems {
			m.extraFilesItems[i].Checked = false
		}
		return m, nil
	}

	return m, nil
}

// handleExtraFilesDestKeys handles keyboard input in extra files destination prompt.
func (m ImportBrowserModel) handleExtraFilesDestKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.result.Aborted = true
		return m, tea.Quit

	case "esc":
		// Go back to file selection
		m.extraFilesShowDest = false
		m.extraFilesDestInput.Blur()
		return m, nil

	case "enter":
		// Confirm destination and proceed
		dest := strings.TrimSpace(m.extraFilesDestInput.Value())
		dest = strings.Trim(dest, "/\\")

		m.extraFilesResult.SelectedPaths = m.getExtraFilesSelectedPaths()
		m.extraFilesResult.DestSubfolder = dest
		m.extraFilesResult.Confirmed = true

		m.state = StateImportPreview
		return m, nil
	}

	var cmd tea.Cmd
	m.extraFilesDestInput, cmd = m.extraFilesDestInput.Update(msg)
	return m, cmd
}

// getExtraFilesSelectedPaths returns the relative paths of all checked extra files.
func (m *ImportBrowserModel) getExtraFilesSelectedPaths() []string {
	var paths []string
	for _, item := range m.extraFilesItems {
		if item.Checked {
			paths = append(paths, item.RelPath)
		}
	}
	return paths
}

// ensureExtraFilesVisible ensures the selected extra file is visible in the viewport.
func (m *ImportBrowserModel) ensureExtraFilesVisible() {
	visibleLines := m.height - 10
	if visibleLines < 5 {
		visibleLines = 5
	}

	if m.extraFilesSelected < m.extraFilesScrollOffset {
		m.extraFilesScrollOffset = m.extraFilesSelected
	} else if m.extraFilesSelected >= m.extraFilesScrollOffset+visibleLines {
		m.extraFilesScrollOffset = m.extraFilesSelected - visibleLines + 1
	}
}

// refreshTree updates the flat tree after expand/collapse.
func (m *ImportBrowserModel) refreshTree() {
	flatTree := flattenSourceTree(m.root)
	m.scroller.updateTree(flatTree)
}

// refresh rebuilds the entire tree from the filesystem.
func (m *ImportBrowserModel) refresh() {
	root, err := buildSourceTree(m.rootPath)
	if err != nil {
		m.message = fmt.Sprintf("Refresh failed: %v", err)
		m.messageIsError = true
		return
	}

	// Rebuild git root set
	gitRoots, _ := git.FindGitRoots(m.rootPath)
	m.gitRootSet = make(map[string]bool)
	for _, r := range gitRoots {
		m.gitRootSet[r] = true
	}

	m.root = root
	m.refreshTree()
	m.message = "Refreshed"
	m.messageIsError = false
}

// View implements tea.Model.
func (m ImportBrowserModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	switch m.state {
	case StateImportConfig:
		return m.renderImportConfigView()
	case StateImportPreview:
		return m.renderImportPreviewView()
	case StateStashConfirm:
		return m.renderStashConfirmView()
	case StateExtraFiles:
		return m.renderExtraFilesView()
	default:
		return m.renderBrowseView()
	}
}

// renderBrowseView renders the main browse view with two panes.
func (m ImportBrowserModel) renderBrowseView() string {
	// Calculate pane dimensions
	leftWidth := m.width/2 - 2
	rightWidth := m.width - leftWidth - 4
	paneHeight := m.height - 4 // Leave room for help

	// Build left pane (tree view)
	leftContent := m.renderTreePane()
	leftPane := ibPaneStyle.Width(leftWidth).Height(paneHeight)
	if m.activePane == IBPaneTree {
		leftPane = ibActivePaneStyle.Width(leftWidth).Height(paneHeight)
	}
	leftRendered := leftPane.Render(leftContent)

	// Build right pane (details)
	rightContent := m.renderDetailsPane()
	rightPane := ibPaneStyle.Width(rightWidth).Height(paneHeight)
	if m.activePane == IBPaneDetails {
		rightPane = ibActivePaneStyle.Width(rightWidth).Height(paneHeight)
	}
	rightRendered := rightPane.Render(rightContent)

	// Join panes horizontally
	main := lipgloss.JoinHorizontal(lipgloss.Top, leftRendered, rightRendered)

	// Build help bar
	help := m.renderHelp()

	// Join main and help
	return lipgloss.JoinVertical(lipgloss.Left, main, help)
}

// renderImportConfigView renders the import configuration form.
func (m ImportBrowserModel) renderImportConfigView() string {
	var sb strings.Builder

	sb.WriteString(ibHeaderStyle.Render("Import Folder as Workspace") + "\n\n")

	if m.importTarget != nil {
		sb.WriteString(fmt.Sprintf("Source: %s\n", m.importTarget.Path))

		// Count git repos in target
		repoCount := 0
		if m.importTarget.IsGitRepo {
			repoCount = 1
		} else {
			prefix := m.importTarget.Path + string(filepath.Separator)
			for gitRoot := range m.gitRootSet {
				if strings.HasPrefix(gitRoot, prefix) {
					repoCount++
				}
			}
		}

		if repoCount == 0 {
			sb.WriteString("Repos:  none (files only)\n\n")
		} else if repoCount == 1 {
			sb.WriteString("Repos:  1 git repository\n\n")
		} else {
			sb.WriteString(fmt.Sprintf("Repos:  %d git repositories\n\n", repoCount))
		}
	}

	// Owner input
	ownerLabel := "Owner:   "
	if m.configFocusIdx == 0 {
		ownerLabel = ibSelectedStyle.Render(ownerLabel)
	}
	sb.WriteString(ownerLabel + m.ownerInput.View() + "\n")

	// Project input
	projectLabel := "Project: "
	if m.configFocusIdx == 1 {
		projectLabel = ibSelectedStyle.Render(projectLabel)
	}
	sb.WriteString(projectLabel + m.projectInput.View() + "\n")

	// Show resulting slug
	owner := strings.ToLower(strings.TrimSpace(m.ownerInput.Value()))
	project := strings.ToLower(strings.TrimSpace(m.projectInput.Value()))
	if owner != "" && project != "" {
		sb.WriteString(fmt.Sprintf("\nWorkspace: %s--%s\n", owner, project))
	}

	// Error
	if m.configError != "" {
		sb.WriteString("\n" + ibErrorStyle.Render("Error: "+m.configError) + "\n")
	}

	// Help
	sb.WriteString("\n" + ibHelpStyle.Render("tab: next field • enter: confirm • esc: cancel"))

	return sb.String()
}

// renderStashConfirmView renders the stash confirmation dialog.
func (m ImportBrowserModel) renderStashConfirmView() string {
	var sb strings.Builder

	// Header
	if m.stashDeleteAfter {
		sb.WriteString(ibHeaderStyle.Render("Stash & Delete Folder") + "\n\n")
	} else {
		sb.WriteString(ibHeaderStyle.Render("Stash Folder") + "\n\n")
	}

	// Source info
	if m.stashTarget != nil {
		sb.WriteString(fmt.Sprintf("Source: %s\n", m.stashTarget.Path))

		// Show git info if it's a repo
		if m.stashTarget.IsGitRepo && m.stashTarget.GitInfo != nil {
			sb.WriteString(fmt.Sprintf("Git:    %s", m.stashTarget.GitInfo.Branch))
			if m.stashTarget.GitInfo.Dirty {
				sb.WriteString(" (uncommitted changes)")
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Archive name input
	nameLabel := "Name:   "
	if m.stashFocusIdx == 0 {
		nameLabel = ibSelectedStyle.Render(nameLabel)
	}
	sb.WriteString(nameLabel + m.stashNameInput.View() + "\n\n")

	// Delete after checkbox
	deleteLabel := "Delete after stash: "
	if m.stashFocusIdx == 1 {
		deleteLabel = ibSelectedStyle.Render(deleteLabel)
	}

	deleteValue := "[ ] No"
	if m.stashDeleteAfter {
		deleteValue = "[x] Yes"
		deleteValue = ibGitDirtyStyle.Render(deleteValue) // Highlight in warning color
	}
	sb.WriteString(deleteLabel + deleteValue + "\n")

	// Preview archive name
	name := strings.TrimSpace(m.stashNameInput.Value())
	if name == "" && m.stashTarget != nil {
		name = archive.SanitizeArchiveName(m.stashTarget.Name)
	}
	sb.WriteString(fmt.Sprintf("\nArchive: %s--<timestamp>--stash.tar.gz\n", name))

	// Warning if deleting
	if m.stashDeleteAfter {
		sb.WriteString("\n" + ibErrorStyle.Render("WARNING: Source folder will be DELETED after archiving!") + "\n")
	}

	// Error
	if m.stashError != "" {
		sb.WriteString("\n" + ibErrorStyle.Render("Error: "+m.stashError) + "\n")
	}

	// Help
	sb.WriteString("\n" + ibHelpStyle.Render("tab: switch field • space/d: toggle delete • enter: stash • esc: cancel"))

	return sb.String()
}

// renderExtraFilesView renders the extra files selection view.
func (m ImportBrowserModel) renderExtraFilesView() string {
	if m.extraFilesShowDest {
		return m.renderExtraFilesDestView()
	}

	var sb strings.Builder

	// Title
	sb.WriteString(ibHeaderStyle.Render("Include Extra Files") + "\n")
	sb.WriteString(ibHelpStyle.Render("Found files/folders not managed by git. Select which to include.") + "\n\n")

	// Calculate visible area
	visibleLines := m.height - 12
	if visibleLines < 5 {
		visibleLines = 5
	}

	// Render items
	startIdx := m.extraFilesScrollOffset
	endIdx := startIdx + visibleLines
	if endIdx > len(m.extraFilesItems) {
		endIdx = len(m.extraFilesItems)
	}

	for i := startIdx; i < endIdx; i++ {
		item := m.extraFilesItems[i]
		line := m.renderExtraFileItem(item, i == m.extraFilesSelected)
		sb.WriteString(line + "\n")
	}

	// Scroll indicator
	if len(m.extraFilesItems) > visibleLines {
		sb.WriteString(fmt.Sprintf("\n(%d/%d)", m.extraFilesSelected+1, len(m.extraFilesItems)))
	}

	// Summary
	selectedCount := 0
	for _, item := range m.extraFilesItems {
		if item.Checked {
			selectedCount++
		}
	}
	sb.WriteString(fmt.Sprintf("\n\n%d of %d selected", selectedCount, len(m.extraFilesItems)))

	// Help
	sb.WriteString("\n\n" + ibHelpStyle.Render("j/k: navigate • space: toggle • a: all • n: none"))
	sb.WriteString("\n" + ibHelpStyle.Render("enter: continue • q/esc: skip extra files"))

	return sb.String()
}

// renderExtraFilesDestView renders the destination folder prompt for extra files.
func (m ImportBrowserModel) renderExtraFilesDestView() string {
	var sb strings.Builder

	sb.WriteString(ibHeaderStyle.Render("Destination Folder") + "\n\n")

	selectedCount := 0
	for _, item := range m.extraFilesItems {
		if item.Checked {
			selectedCount++
		}
	}
	sb.WriteString(fmt.Sprintf("Copying %d file(s)/folder(s) to workspace.\n\n", selectedCount))

	sb.WriteString("Enter destination subfolder:\n")
	sb.WriteString(ibHelpStyle.Render("(leave empty to place at project root)") + "\n\n")
	sb.WriteString(m.extraFilesDestInput.View() + "\n")

	sb.WriteString("\n" + ibHelpStyle.Render("enter: confirm • esc: back to selection"))

	return sb.String()
}

// renderExtraFileItem renders a single extra file item.
func (m ImportBrowserModel) renderExtraFileItem(item extraFileItem, isSelected bool) string {
	// Checkbox
	var checkbox string
	if item.Checked {
		checkbox = "[x] "
	} else {
		checkbox = "[ ] "
	}

	// Name with dir indicator
	name := item.Name
	if item.IsDir {
		name = ibDirStyle.Render(name + "/")
	}

	line := checkbox + name

	// Apply styling
	if isSelected {
		line = ibSelectedStyle.Render(line)
	} else if item.Checked {
		line = ibSuccessStyle.Render(line)
	} else {
		line = ibHelpStyle.Render(line)
	}

	return line
}

// renderImportPreviewView renders the import preview.
func (m ImportBrowserModel) renderImportPreviewView() string {
	var sb strings.Builder

	sb.WriteString(ibHeaderStyle.Render("Import Preview") + "\n\n")

	sb.WriteString(fmt.Sprintf("Workspace: %s\n", m.result.WorkspaceSlug))
	sb.WriteString(fmt.Sprintf("Path:      %s\n", m.result.WorkspacePath))

	if m.importTarget != nil {
		sb.WriteString(fmt.Sprintf("\nSource: %s\n", m.importTarget.Path))

		// Count and list repos
		var repos []string
		if m.importTarget.IsGitRepo {
			repos = append(repos, m.importTarget.Name)
		} else {
			prefix := m.importTarget.Path + string(filepath.Separator)
			for gitRoot := range m.gitRootSet {
				if strings.HasPrefix(gitRoot, prefix) {
					repos = append(repos, filepath.Base(gitRoot))
				}
			}
		}

		if len(repos) > 0 {
			sb.WriteString(fmt.Sprintf("\nRepositories (%d):\n", len(repos)))
			for _, repo := range repos {
				sb.WriteString(fmt.Sprintf("  • %s\n", repo))
			}
		}
	}

	// Show extra files if any selected
	if len(m.extraFilesResult.SelectedPaths) > 0 {
		sb.WriteString(fmt.Sprintf("\nExtra files (%d):\n", len(m.extraFilesResult.SelectedPaths)))
		dest := m.extraFilesResult.DestSubfolder
		if dest == "" {
			dest = "(project root)"
		} else {
			dest = dest + "/"
		}
		sb.WriteString(fmt.Sprintf("  Destination: %s\n", dest))
		for _, path := range m.extraFilesResult.SelectedPaths {
			sb.WriteString(fmt.Sprintf("  • %s\n", path))
		}
	}

	sb.WriteString("\n" + ibHelpStyle.Render("enter: execute import • esc: back"))

	return sb.String()
}

// renderTreePane renders the tree view pane.
func (m ImportBrowserModel) renderTreePane() string {
	var sb strings.Builder

	sb.WriteString(ibHeaderStyle.Render("Source Folder") + "\n")

	start, end := m.scroller.visibleRange()
	for i := start; i < end; i++ {
		node := m.scroller.flatTree[i]
		line := m.renderNode(node, m.scroller.isSelected(i))
		sb.WriteString(line + "\n")
	}

	// Scroll indicator
	if len(m.scroller.flatTree) > m.scroller.height {
		sb.WriteString(fmt.Sprintf("\n(%d/%d)", m.scroller.selected+1, len(m.scroller.flatTree)))
	}

	return sb.String()
}

// renderNode renders a single tree node.
func (m ImportBrowserModel) renderNode(node *sourceNode, isSelected bool) string {
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

	// Selection marker
	selectMarker := "  "
	if node.IsSelected {
		selectMarker = "● "
	}

	// Name with styling
	name := node.Name
	var styledName string

	if node.IsSymlink {
		styledName = ibSymlinkStyle.Render(name + " →")
	} else if node.IsGitRepo {
		gitInfo := ""
		if node.GitInfo != nil {
			gitInfo = fmt.Sprintf(" [%s", node.GitInfo.Branch)
			if node.GitInfo.Dirty {
				gitInfo += "*"
			}
			gitInfo += "]"
		}
		if node.GitInfo != nil && node.GitInfo.Dirty {
			styledName = ibGitDirtyStyle.Render(name + gitInfo)
		} else {
			styledName = ibGitRepoStyle.Render(name + gitInfo)
		}
	} else if node.IsDir {
		suffix := ""
		if node.HasGitChild {
			suffix = " •"
		}
		styledName = ibDirStyle.Render(name + "/" + suffix)
	} else {
		styledName = ibFileStyle.Render(name)
	}

	line := fmt.Sprintf("%s%s%s%s", indent, selectMarker, icon, styledName)

	if isSelected {
		line = ibSelectedStyle.Render(line)
	}

	return line
}

// renderDetailsPane renders the details pane for the selected item.
func (m ImportBrowserModel) renderDetailsPane() string {
	var sb strings.Builder

	node := m.scroller.selectedNode()
	if node == nil {
		sb.WriteString("No item selected")
		return sb.String()
	}

	sb.WriteString(ibHeaderStyle.Render("Details") + "\n\n")

	// Name
	sb.WriteString(fmt.Sprintf("Name:   %s\n", node.Name))
	sb.WriteString(fmt.Sprintf("Path:   %s\n", node.Path))

	if node.IsDir {
		sb.WriteString("Type:   Directory\n")
	} else {
		sb.WriteString("Type:   File\n")
	}

	if node.IsSymlink {
		sb.WriteString("Note:   Symbolic link\n")
	}

	if node.IsGitRepo {
		sb.WriteString("\n" + ibGitRepoStyle.Render("Git Repository") + "\n")
		if node.GitInfo != nil {
			sb.WriteString(fmt.Sprintf("Branch: %s\n", node.GitInfo.Branch))
			if node.GitInfo.Dirty {
				sb.WriteString(ibGitDirtyStyle.Render("Status: Uncommitted changes") + "\n")
			} else {
				sb.WriteString("Status: Clean\n")
			}
			if node.GitInfo.Remote != "" {
				sb.WriteString(fmt.Sprintf("Remote: %s\n", node.GitInfo.Remote))
			}
		}
	} else if node.HasGitChild {
		sb.WriteString("\n" + ibDirStyle.Render("Contains git repositories") + "\n")
	}

	// Count git repos if directory
	if node.IsDir && !node.IsGitRepo {
		repoCount := 0
		for gitRoot := range m.gitRootSet {
			if strings.HasPrefix(gitRoot, node.Path+string(filepath.Separator)) || gitRoot == node.Path {
				repoCount++
			}
		}
		if repoCount > 0 {
			sb.WriteString(fmt.Sprintf("\nRepos:  %d\n", repoCount))
		}
	}

	// Show message if any
	if m.message != "" {
		sb.WriteString("\n")
		if m.messageIsError {
			sb.WriteString(ibErrorStyle.Render(m.message))
		} else {
			sb.WriteString(ibSuccessStyle.Render(m.message))
		}
	}

	// Actions hint
	sb.WriteString("\n\n" + ibHelpStyle.Render("Actions:"))
	if node.IsDir {
		sb.WriteString("\n" + ibHelpStyle.Render("i - import as workspace"))
		sb.WriteString("\n" + ibHelpStyle.Render("a - add to workspace"))
		sb.WriteString("\n" + ibHelpStyle.Render("s - stash (archive)"))
		sb.WriteString("\n" + ibHelpStyle.Render("S - stash & delete"))
	}

	return sb.String()
}

// renderHelp renders the help bar.
func (m ImportBrowserModel) renderHelp() string {
	var help string
	switch m.state {
	case StateBrowse:
		help = "j/k: navigate • h/l: collapse/expand • i: import • s/S: stash • r: refresh • q: quit"
	case StateImportConfig:
		help = "tab: next field • enter: confirm • esc: cancel"
	case StateImportPreview:
		help = "enter: execute • esc: back"
	case StateStashConfirm:
		help = "tab: switch field • space/d: toggle delete • enter: stash • esc: cancel"
	case StateExtraFiles:
		if m.extraFilesShowDest {
			help = "enter: confirm • esc: back to selection"
		} else {
			help = "j/k: navigate • space: toggle • a: all • n: none • enter: continue • q/esc: skip"
		}
	default:
		help = "q: quit"
	}
	return ibHelpStyle.Render(help)
}

// RunImportBrowser runs the interactive import browser TUI.
func RunImportBrowser(cfg *config.Config, rootPath string) (ImportBrowserResult, error) {
	m, err := NewImportBrowser(cfg, rootPath)
	if err != nil {
		return ImportBrowserResult{Error: err}, err
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return ImportBrowserResult{Error: err}, err
	}

	result := finalModel.(ImportBrowserModel).result
	return result, nil
}
