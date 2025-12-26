package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tormodhaugland/co/internal/archive"
	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/fs"
	"github.com/tormodhaugland/co/internal/git"
	"github.com/tormodhaugland/co/internal/template"
	"github.com/tormodhaugland/co/internal/workspace"
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
	StateBrowse             ImportBrowserState = iota // Browsing the source folder tree
	StateImportConfig                                 // Configuring import (owner/project input)
	StateTemplateSelect                               // Selecting a template to apply
	StateTemplateVars                                 // Prompting for template variables
	StateExtraFiles                                   // Selecting extra non-git files to include
	StateImportPreview                                // Previewing import operation
	StateImportExecute                                // Executing import operation
	StatePostImport                                   // Post-import options (stash/delete source)
	StateStashConfirm                                 // Confirming stash operation
	StateStashExecute                                 // Executing stash operation
	StateAddToSelect                                  // Selecting workspace for add-to mode
	StateBatchImportConfirm                           // Confirming batch import of multiple folders
	StateBatchImportExecute                           // Executing batch import
	StateBatchImportSummary                           // Showing batch import results
	StateBatchStashConfirm                            // Confirming batch stash of multiple folders
	StateBatchStashExecute                            // Executing batch stash
	StateBatchStashSummary                            // Showing batch stash results
	StateDeleteConfirm                                // Confirming delete operation
	StateTrashConfirm                                 // Confirming trash operation
	StateComplete                                     // Operation completed
)

// String returns the string representation of the state.
func (s ImportBrowserState) String() string {
	switch s {
	case StateBrowse:
		return "Browse"
	case StateImportConfig:
		return "Import Config"
	case StateTemplateSelect:
		return "Template Select"
	case StateTemplateVars:
		return "Template Variables"
	case StateExtraFiles:
		return "Extra Files"
	case StateImportPreview:
		return "Import Preview"
	case StateImportExecute:
		return "Importing"
	case StatePostImport:
		return "Post Import"
	case StateStashConfirm:
		return "Stash Confirm"
	case StateStashExecute:
		return "Stashing"
	case StateAddToSelect:
		return "Add To Workspace"
	case StateBatchImportConfirm:
		return "Batch Import Confirm"
	case StateBatchImportExecute:
		return "Batch Importing"
	case StateBatchImportSummary:
		return "Batch Import Summary"
	case StateBatchStashConfirm:
		return "Batch Stash Confirm"
	case StateBatchStashExecute:
		return "Batch Stashing"
	case StateBatchStashSummary:
		return "Batch Stash Summary"
	case StateDeleteConfirm:
		return "Delete Confirm"
	case StateTrashConfirm:
		return "Trash Confirm"
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

	// Template results
	TemplateApplied      string // name of template applied (empty if none)
	TemplateFilesCreated int    // number of template files created
	TemplateError        error  // error if template application failed

	// Stash results
	ArchivePath   string // path to created archive
	SourceStashed string // path that was stashed

	// Status
	Success bool  // true if operation succeeded
	Error   error // error if operation failed
	Aborted bool  // true if user cancelled
}

// BatchImportItemResult holds the result of importing a single folder in a batch operation.
type BatchImportItemResult struct {
	SourcePath    string // Source folder path
	SourceName    string // Source folder name
	WorkspaceSlug string // Created workspace slug (empty on failure)
	WorkspacePath string // Created workspace path (empty on failure)
	RepoCount     int    // Number of repos imported
	Success       bool   // Whether this import succeeded
	Error         error  // Error if import failed
}

// BatchStashItemResult holds the result of stashing a single folder in a batch operation.
type BatchStashItemResult struct {
	SourcePath  string // Source folder path
	SourceName  string // Source folder name
	ArchivePath string // Created archive path (empty on failure)
	Deleted     bool   // Whether source was deleted after stashing
	Success     bool   // Whether this stash succeeded
	Error       error  // Error if stash failed
}

// sizeResultMsg is sent when an async directory size calculation completes.
type sizeResultMsg struct {
	Path string
	Size int64
	Err  error
}

// operationResultMsg is sent when an async operation (stash, delete, etc.) completes.
type operationResultMsg struct {
	Operation string // "stash", "delete", "trash", "import"
	Success   bool
	Message   string // Success or error message
	Err       error
}

// spinnerTickMsg is sent to animate the loading spinner.
type spinnerTickMsg struct{}

// spinnerFrames defines the animation frames for the loading spinner.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// maxSourceDirEntries limits entries per directory to keep UI responsive.
const maxSourceDirEntries = 500

// gitScanMaxDepth controls how deep the initial git repository scan goes.
// This improves startup performance for large directory trees.
// Set to -1 for unlimited depth (not recommended for large trees).
const gitScanMaxDepth = 4

// buildSourceTree creates the root node and detects git repositories.
// It scans for git repos first (up to gitScanMaxDepth levels), then builds the tree structure.
// If showHidden is true, hidden files (dotfiles) are included in the tree.
func buildSourceTree(rootPath string, showHidden bool) (*sourceNode, error) {
	info, err := os.Stat(rootPath)
	if err != nil {
		return nil, err
	}

	// Find git repositories up to a limited depth for performance
	gitRoots, err := git.FindGitRootsWithDepth(rootPath, gitScanMaxDepth)
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
		loadSourceChildren(root, gitRootSet, showHidden)
		root.HasGitChild = hasGitDescendant(root, gitRootSet)
	}

	return root, nil
}

// loadSourceChildren loads the immediate children of a directory node.
// If showHidden is false, hidden files (dotfiles) are excluded except for common useful ones.
func loadSourceChildren(node *sourceNode, gitRootSet map[string]bool, showHidden bool) {
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

		// Skip hidden files unless showHidden is true
		// Always show .env, .gitignore, and .git for git detection
		if !showHidden && strings.HasPrefix(name, ".") && name != ".env" && name != ".gitignore" && name != ".git" {
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
func (node *sourceNode) expandNode(gitRootSet map[string]bool, showHidden bool) {
	if !node.IsDir || node.IsExpanded {
		return
	}

	node.IsExpanded = true

	// Load children if not already loaded
	if node.Children == nil {
		loadSourceChildren(node, gitRootSet, showHidden)
	}
}

// collapseNode collapses a directory node.
func (node *sourceNode) collapseNode() {
	if node.IsDir {
		node.IsExpanded = false
	}
}

// toggleExpand toggles the expanded state of a directory.
func (node *sourceNode) toggleExpand(gitRootSet map[string]bool, showHidden bool) {
	if !node.IsDir {
		return
	}

	if node.IsExpanded {
		node.collapseNode()
	} else {
		node.expandNode(gitRootSet, showHidden)
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

// selectByPath finds and selects a node by its path.
// If the exact path is not found, it tries to select a sibling in the same parent directory,
// or falls back to the parent directory itself.
// Returns true if a node was found and selected.
func (s *sourceTreeScroller) selectByPath(targetPath string) bool {
	if targetPath == "" || len(s.flatTree) == 0 {
		return false
	}

	// First, try to find exact match
	for i, node := range s.flatTree {
		if node.Path == targetPath {
			s.selected = i
			s.ensureVisible()
			return true
		}
	}

	// If exact path not found, look for a sibling in the same parent directory.
	// This provides better UX when a folder is deleted - we stay at the same level.
	parentDir := filepath.Dir(targetPath)
	if parentDir != "" && parentDir != "/" && parentDir != "." {
		// Find all siblings (nodes with the same parent directory)
		var siblingIdx int = -1
		for i, node := range s.flatTree {
			if filepath.Dir(node.Path) == parentDir {
				siblingIdx = i
				// Prefer the first sibling we find (could be the one after the deleted item)
				break
			}
		}
		if siblingIdx >= 0 {
			s.selected = siblingIdx
			s.ensureVisible()
			return true
		}
	}

	// If no sibling found, try to find the nearest parent that exists
	// Walk up the path hierarchy looking for a match
	for targetPath != "" && targetPath != "/" && targetPath != "." {
		targetPath = filepath.Dir(targetPath)
		for i, node := range s.flatTree {
			if node.Path == targetPath {
				s.selected = i
				s.ensureVisible()
				return true
			}
		}
	}

	return false
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

// getSelectedNodes returns all nodes that have been selected for batch operations.
// This traverses the entire tree (not just visible/flat nodes) to find selected directories.
func (s *sourceTreeScroller) getSelectedNodes() []*sourceNode {
	var selected []*sourceNode
	for _, node := range s.flatTree {
		if node.IsSelected && node.IsDir {
			selected = append(selected, node)
		}
	}
	return selected
}

// getSelectedCount returns the number of nodes selected for batch operations.
func (s *sourceTreeScroller) getSelectedCount() int {
	count := 0
	for _, node := range s.flatTree {
		if node.IsSelected && node.IsDir {
			count++
		}
	}
	return count
}

// clearAllSelections clears the IsSelected flag on all nodes.
func (s *sourceTreeScroller) clearAllSelections() {
	for _, node := range s.flatTree {
		node.IsSelected = false
	}
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

	// Loading state for async operations
	loading        bool   // True when an async operation is in progress
	loadingMessage string // Description of what's being done
	spinnerFrame   int    // Current spinner animation frame

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

	// Delete/trash state
	deleteTarget  *sourceNode // The folder being deleted/trashed
	deleteIsTrash bool        // True if using trash, false if permanent delete

	// Extra files state
	extraFilesItems        []extraFileItem  // Non-git items found
	extraFilesSelected     int              // Currently selected item index
	extraFilesScrollOffset int              // Scroll offset for long lists
	extraFilesShowDest     bool             // Show destination prompt
	extraFilesDestInput    textinput.Model  // Destination subfolder input
	extraFilesResult       ExtraFilesResult // Selected files result

	// Post-import state
	postImportSourcePath string // Source path that was imported
	postImportOption     int    // 0=keep, 1=stash, 2=delete

	// Add-to-workspace state
	addToWorkspaces   []string // List of available workspaces
	addToSelected     int      // Currently selected workspace index
	addToScrollOffset int      // Scroll offset for workspace list
	addToTargetSlug   string   // Selected workspace slug

	// Template selection state
	templateInfos        []template.TemplateInfo // Available templates
	templateSelected     int                     // Currently selected template index
	templateScrollOffset int                     // Scroll offset for template list
	selectedTemplate     string                  // Selected template name (empty = no template)

	// Template variable prompting state
	templateVars         []template.TemplateVar // Variables to prompt for
	templateVarValues    map[string]string      // Collected variable values
	templateVarIndex     int                    // Current variable being prompted
	templateVarInput     textinput.Model        // Text input for current variable
	templateVarBoolValue bool                   // Current boolean value
	templateVarChoiceIdx int                    // Current choice selection index
	templateVarError     string                 // Validation error for current variable

	// Size cache for directories
	sizeCache   map[string]int64    // path -> size in bytes
	sizePending map[string]struct{} // paths with in-flight size calculations

	// Display options
	showHidden bool // Show hidden files (dotfiles)

	// Filter state
	filterActive bool            // True when filter mode is active
	filterInput  textinput.Model // Filter text input
	filterText   string          // Current filter text (cached from input)

	// Dry-run mode
	dryRun bool // If true, show what would happen without making changes

	// Batch import state
	batchImportTargets []*sourceNode           // Folders selected for batch import
	batchImportResults []BatchImportItemResult // Results of each batch import
	batchImportCurrent int                     // Index of currently importing folder
	batchOwner         string                  // Owner for all batch imports

	// Batch stash state
	batchStashTargets     []*sourceNode          // Folders selected for batch stash
	batchStashResults     []BatchStashItemResult // Results of each batch stash
	batchStashCurrent     int                    // Index of currently stashing folder
	batchStashDeleteAfter bool                   // Whether to delete folders after stashing

	result ImportBrowserResult
}

// NewImportBrowser creates a new import browser model.
func NewImportBrowser(cfg *config.Config, rootPath string) (*ImportBrowserModel, error) {
	// Build the source tree (default: hidden files not shown)
	showHidden := false
	root, err := buildSourceTree(rootPath, showHidden)
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

	// Initialize text input for filter
	filterInput := textinput.New()
	filterInput.Placeholder = "filter..."
	filterInput.CharLimit = 64
	filterInput.Width = 30

	// Initialize text input for template variables
	templateVarInput := textinput.New()
	templateVarInput.Placeholder = "value"
	templateVarInput.CharLimit = 256
	templateVarInput.Width = 40

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
		filterInput:         filterInput,
		templateVarInput:    templateVarInput,
		templateVarValues:   make(map[string]string),
		sizeCache:           make(map[string]int64),
		sizePending:         make(map[string]struct{}),
	}, nil
}

// Init implements tea.Model.
func (m ImportBrowserModel) Init() tea.Cmd {
	// Start async size calculation for initially selected item
	return m.triggerSelectedSizeCalc()
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

	case sizeResultMsg:
		// Async size calculation completed
		delete(m.sizePending, msg.Path)
		if msg.Err == nil {
			m.sizeCache[msg.Path] = msg.Size
		}
		return m, nil

	case operationResultMsg:
		// Async operation completed
		m.loading = false
		m.loadingMessage = ""
		m.message = msg.Message
		m.messageIsError = !msg.Success
		if msg.Success {
			m.refresh() // Refresh tree after successful operation
		}
		m.state = StateBrowse
		// Clear operation-specific state
		m.deleteTarget = nil
		m.stashTarget = nil
		return m, nil

	case spinnerTickMsg:
		// Animate spinner while loading
		if m.loading {
			m.spinnerFrame = (m.spinnerFrame + 1) % len(spinnerFrames)
			return m, m.spinnerTick()
		}
		return m, nil

	case tea.KeyMsg:
		// Ignore key presses while loading
		if m.loading {
			return m, nil
		}
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
	case StateTemplateSelect:
		return m.handleTemplateSelectKeys(msg)
	case StateTemplateVars:
		return m.handleTemplateVarsKeys(msg)
	case StateImportPreview:
		return m.handleImportPreviewKeys(msg)
	case StateStashConfirm:
		return m.handleStashConfirmKeys(msg)
	case StateExtraFiles:
		return m.handleExtraFilesKeys(msg)
	case StatePostImport:
		return m.handlePostImportKeys(msg)
	case StateAddToSelect:
		return m.handleAddToSelectKeys(msg)
	case StateBatchImportConfirm:
		return m.handleBatchImportConfirmKeys(msg)
	case StateBatchImportSummary:
		return m.handleBatchImportSummaryKeys(msg)
	case StateBatchStashConfirm:
		return m.handleBatchStashConfirmKeys(msg)
	case StateBatchStashSummary:
		return m.handleBatchStashSummaryKeys(msg)
	case StateDeleteConfirm, StateTrashConfirm:
		return m.handleDeleteConfirmKeys(msg)
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
		// Go back - to extra files if there were any, otherwise to config/workspace select
		if len(m.extraFilesItems) > 0 {
			m.state = StateExtraFiles
		} else if m.addToTargetSlug != "" {
			// Add-to mode: go back to workspace selection
			m.state = StateAddToSelect
		} else {
			m.state = StateImportConfig
			return m, m.ownerInput.Focus()
		}
		return m, nil

	case "enter":
		// Execute import or add-to (or dry-run)
		if m.dryRun {
			return m.executeDryRun()
		}
		if m.addToTargetSlug != "" {
			return m.executeAddToWorkspace()
		}
		return m.executeImport()

	case "d":
		// Toggle dry-run mode
		m.dryRun = !m.dryRun
		return m, nil
	}
	return m, nil
}

// executeImport performs the actual import operation using the workspace package.
func (m ImportBrowserModel) executeImport() (tea.Model, tea.Cmd) {
	if m.importTarget == nil {
		m.message = "No folder selected for import"
		m.messageIsError = true
		return m, nil
	}

	m.state = StateImportExecute

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

	// Parse owner and project from slug
	parts := strings.SplitN(m.result.WorkspaceSlug, "--", 2)
	if len(parts) != 2 {
		m.message = "Invalid workspace slug"
		m.messageIsError = true
		m.state = StateImportConfig
		return m, m.ownerInput.Focus()
	}
	owner, project := parts[0], parts[1]

	// Build import options with progress callbacks
	var progressMessages []string
	opts := workspace.ImportOptions{
		Owner:          owner,
		Project:        project,
		ExtraFiles:     m.extraFilesResult.SelectedPaths,
		ExtraFilesDest: m.extraFilesResult.DestSubfolder,
		OnRepoMove: func(repoName, srcPath, dstPath string) {
			progressMessages = append(progressMessages, fmt.Sprintf("Moving repo: %s", repoName))
		},
		OnFileCopy: func(relPath, dstPath string) {
			progressMessages = append(progressMessages, fmt.Sprintf("Copying: %s", relPath))
		},
		OnWarning: func(msg string) {
			progressMessages = append(progressMessages, fmt.Sprintf("Warning: %s", msg))
		},
	}

	// Execute the import
	result, err := workspace.CreateWorkspace(m.cfg, m.importTarget.Path, gitRoots, opts)
	if err != nil {
		m.message = fmt.Sprintf("Import failed: %v", err)
		m.messageIsError = true
		m.state = StateImportPreview
		return m, nil
	}

	// Store results
	m.result.Action = "import"
	m.result.Success = true
	m.result.WorkspacePath = result.WorkspacePath
	m.result.WorkspaceSlug = result.WorkspaceSlug
	m.result.ReposImported = result.ReposImported
	m.result.FilesImported = result.FilesCopied

	// Apply template if one was selected
	if m.selectedTemplate != "" {
		templateOpts := template.CreateOptions{
			TemplateName: m.selectedTemplate,
			Variables:    m.templateVarValues,
		}
		templateResult, templateErr := template.ApplyTemplateToExisting(m.cfg, result.WorkspacePath, m.selectedTemplate, templateOpts)
		if templateErr != nil {
			// Template application failed, but workspace was created
			m.result.TemplateError = templateErr
			m.message = fmt.Sprintf("Workspace created but template failed: %v", templateErr)
			m.messageIsError = true
		} else {
			// Template applied successfully
			m.result.TemplateApplied = m.selectedTemplate
			m.result.TemplateFilesCreated = templateResult.FilesCreated
		}
	}

	// Check if source is now empty - if so, just clean up and go to browse
	if result.SourceEmpty {
		workspace.RemoveEmptySource(m.importTarget.Path)
		m.refresh()
		if m.selectedTemplate != "" && m.result.TemplateApplied != "" {
			m.message = fmt.Sprintf("Created workspace: %s (template: %s)", result.WorkspaceSlug, m.selectedTemplate)
		} else {
			m.message = fmt.Sprintf("Created workspace: %s", result.WorkspaceSlug)
		}
		m.messageIsError = false
		m.state = StateBrowse
		m.importTarget = nil
		return m, nil
	}

	// Source still has content - offer post-import options
	m.postImportSourcePath = m.importTarget.Path
	m.postImportOption = 0 // Default to "keep"
	m.state = StatePostImport

	return m, nil
}

// executeDryRun shows what would happen without making changes.
func (m ImportBrowserModel) executeDryRun() (tea.Model, tea.Cmd) {
	if m.importTarget == nil {
		m.message = "No folder selected for import"
		m.messageIsError = true
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

	// Build summary of what would happen
	var sb strings.Builder
	sb.WriteString("DRY-RUN: No changes will be made.\n\n")

	if m.addToTargetSlug != "" {
		sb.WriteString(fmt.Sprintf("Would add to existing workspace: %s\n", m.addToTargetSlug))
	} else {
		sb.WriteString(fmt.Sprintf("Would create new workspace: %s\n", m.result.WorkspaceSlug))
	}

	sb.WriteString(fmt.Sprintf("Source: %s\n\n", m.importTarget.Path))

	if len(gitRoots) > 0 {
		sb.WriteString(fmt.Sprintf("Repositories to move (%d):\n", len(gitRoots)))
		for _, root := range gitRoots {
			repoName := workspace.DeriveRepoName(root, m.importTarget.Path)
			sb.WriteString(fmt.Sprintf("  - %s -> repos/%s\n", filepath.Base(root), repoName))
		}
	}

	if len(m.extraFilesResult.SelectedPaths) > 0 {
		sb.WriteString(fmt.Sprintf("\nExtra files to copy (%d):\n", len(m.extraFilesResult.SelectedPaths)))
		dest := m.extraFilesResult.DestSubfolder
		if dest == "" {
			dest = "(project root)"
		}
		for _, path := range m.extraFilesResult.SelectedPaths {
			sb.WriteString(fmt.Sprintf("  - %s -> %s/%s\n", path, dest, path))
		}
	}

	m.message = sb.String()
	m.messageIsError = false
	m.dryRun = false // Reset dry-run after showing results

	return m, nil
}

// executeAddToWorkspace performs the add-to-workspace operation.
func (m ImportBrowserModel) executeAddToWorkspace() (tea.Model, tea.Cmd) {
	if m.importTarget == nil {
		m.message = "No folder selected"
		m.messageIsError = true
		return m, nil
	}

	if m.addToTargetSlug == "" {
		m.message = "No workspace selected"
		m.messageIsError = true
		return m, nil
	}

	m.state = StateImportExecute

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

	// Build import options with progress callbacks
	opts := workspace.ImportOptions{
		ExtraFiles:     m.extraFilesResult.SelectedPaths,
		ExtraFilesDest: m.extraFilesResult.DestSubfolder,
		OnRepoMove: func(repoName, srcPath, dstPath string) {
			// Progress: moving repo
		},
		OnRepoSkip: func(repoName, reason string) {
			// Progress: skipping repo
		},
		OnFileCopy: func(relPath, dstPath string) {
			// Progress: copying file
		},
		OnWarning: func(msg string) {
			// Warning
		},
	}

	// Execute the add-to operation
	result, err := workspace.AddToWorkspace(m.cfg, m.importTarget.Path, gitRoots, m.addToTargetSlug, opts)
	if err != nil {
		m.message = fmt.Sprintf("Add to workspace failed: %v", err)
		m.messageIsError = true
		m.state = StateImportPreview
		return m, nil
	}

	// Store results
	m.result.Action = "add-to"
	m.result.Success = true
	m.result.WorkspacePath = result.WorkspacePath
	m.result.WorkspaceSlug = result.WorkspaceSlug
	m.result.ReposImported = result.ReposImported
	m.result.FilesImported = result.FilesCopied

	// Check if source is now empty - if so, just clean up and go to browse
	if result.SourceEmpty {
		workspace.RemoveEmptySource(m.importTarget.Path)
		m.refresh()
		m.message = fmt.Sprintf("Added to workspace: %s (%d repos)", result.WorkspaceSlug, len(result.ReposImported))
		if len(result.ReposSkipped) > 0 {
			m.message += fmt.Sprintf(", %d skipped", len(result.ReposSkipped))
		}
		m.messageIsError = false
		m.state = StateBrowse
		m.clearAddToState()
		return m, nil
	}

	// Source still has content - offer post-import options
	m.postImportSourcePath = m.importTarget.Path
	m.postImportOption = 0 // Default to "keep"
	m.state = StatePostImport

	return m, nil
}

// clearAddToState resets add-to-workspace state.
func (m *ImportBrowserModel) clearAddToState() {
	m.importTarget = nil
	m.addToWorkspaces = nil
	m.addToTargetSlug = ""
	m.addToSelected = 0
	m.addToScrollOffset = 0
}

// handlePostImportKeys handles keyboard input in post-import options state.
func (m ImportBrowserModel) handlePostImportKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.result.Aborted = true
		return m, tea.Quit

	case "j", "down":
		if m.postImportOption < 2 {
			m.postImportOption++
		}
		return m, nil

	case "k", "up":
		if m.postImportOption > 0 {
			m.postImportOption--
		}
		return m, nil

	case "1":
		m.postImportOption = 0 // Keep
		return m, nil

	case "2":
		m.postImportOption = 1 // Stash
		return m, nil

	case "3":
		m.postImportOption = 2 // Delete
		return m, nil

	case "enter":
		return m.executePostImportAction()
	}

	return m, nil
}

// executePostImportAction executes the selected post-import action on the source folder.
func (m ImportBrowserModel) executePostImportAction() (tea.Model, tea.Cmd) {
	switch m.postImportOption {
	case 0: // Keep - do nothing
		m.message = fmt.Sprintf("Created workspace: %s (source kept)", m.result.WorkspaceSlug)
		m.messageIsError = false

	case 1: // Stash
		opts := archive.StashOptions{
			Name:        "",
			DeleteAfter: true, // Stash and delete
		}
		result, err := archive.StashFolder(m.cfg, m.postImportSourcePath, opts)
		if err != nil {
			m.message = fmt.Sprintf("Stash failed: %v", err)
			m.messageIsError = true
			return m, nil
		}
		m.result.ArchivePath = result.ArchivePath
		m.result.SourceStashed = result.SourcePath
		m.message = fmt.Sprintf("Created workspace: %s (source stashed)", m.result.WorkspaceSlug)
		m.messageIsError = false

	case 2: // Delete
		if err := os.RemoveAll(m.postImportSourcePath); err != nil {
			m.message = fmt.Sprintf("Delete failed: %v", err)
			m.messageIsError = true
			return m, nil
		}
		m.message = fmt.Sprintf("Created workspace: %s (source deleted)", m.result.WorkspaceSlug)
		m.messageIsError = false
	}

	// Refresh tree and return to browse
	m.refresh()
	m.state = StateBrowse
	m.importTarget = nil
	m.postImportSourcePath = ""

	return m, nil
}

// handleAddToSelectKeys handles keyboard input in workspace selection state.
func (m ImportBrowserModel) handleAddToSelectKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.result.Aborted = true
		return m, tea.Quit

	case "esc", "q":
		// Cancel, return to browse
		m.state = StateBrowse
		m.importTarget = nil
		m.addToWorkspaces = nil
		return m, nil

	case "j", "down":
		if m.addToSelected < len(m.addToWorkspaces)-1 {
			m.addToSelected++
			m.ensureAddToVisible()
		}
		return m, nil

	case "k", "up":
		if m.addToSelected > 0 {
			m.addToSelected--
			m.ensureAddToVisible()
		}
		return m, nil

	case "g":
		m.addToSelected = 0
		m.addToScrollOffset = 0
		return m, nil

	case "G":
		if len(m.addToWorkspaces) > 0 {
			m.addToSelected = len(m.addToWorkspaces) - 1
			m.ensureAddToVisible()
		}
		return m, nil

	case "enter":
		// Select workspace and proceed
		if m.addToSelected < len(m.addToWorkspaces) {
			m.addToTargetSlug = m.addToWorkspaces[m.addToSelected]
			m.result.WorkspaceSlug = m.addToTargetSlug
			m.result.WorkspacePath = filepath.Join(m.cfg.CodeRoot, m.addToTargetSlug)

			// Check for extra files before proceeding to preview
			return m.checkForExtraFilesAddTo()
		}
		return m, nil
	}

	return m, nil
}

// ensureAddToVisible ensures the selected workspace is visible in the viewport.
func (m *ImportBrowserModel) ensureAddToVisible() {
	visibleLines := m.height - 10
	if visibleLines < 5 {
		visibleLines = 5
	}

	if m.addToSelected < m.addToScrollOffset {
		m.addToScrollOffset = m.addToSelected
	} else if m.addToSelected >= m.addToScrollOffset+visibleLines {
		m.addToScrollOffset = m.addToSelected - visibleLines + 1
	}
}

// checkForExtraFilesAddTo checks for extra files and transitions to appropriate state for add-to mode.
func (m ImportBrowserModel) checkForExtraFilesAddTo() (tea.Model, tea.Cmd) {
	if m.importTarget == nil {
		m.extraFilesResult = ExtraFilesResult{} // Clear previous results
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
		m.extraFilesResult = ExtraFilesResult{} // Clear previous results
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

// handleFilterKeys handles keyboard input when filter is active.
func (m ImportBrowserModel) handleFilterKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Exit filter mode and clear filter
		m.filterActive = false
		m.filterText = ""
		m.filterInput.Blur()
		m.applyFilter()
		return m, nil

	case "enter":
		// Confirm filter and exit filter mode
		m.filterActive = false
		m.filterInput.Blur()
		return m, nil

	case "ctrl+c":
		m.result.Aborted = true
		return m, tea.Quit
	}

	// Update filter input
	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)

	// Apply filter when text changes
	newText := m.filterInput.Value()
	if newText != m.filterText {
		m.filterText = newText
		m.applyFilter()
	}

	return m, cmd
}

// applyFilter filters the visible tree nodes based on filter text.
func (m *ImportBrowserModel) applyFilter() {
	// Rebuild flat tree from root
	flatTree := flattenSourceTree(m.root)

	if m.filterText == "" {
		// No filter, show all
		m.scroller.updateTree(flatTree)
		return
	}

	// Filter nodes by name (case-insensitive)
	filterLower := strings.ToLower(m.filterText)
	var filtered []*sourceNode

	for _, node := range flatTree {
		if strings.Contains(strings.ToLower(node.Name), filterLower) {
			filtered = append(filtered, node)
		}
	}

	m.scroller.updateTree(filtered)
}

// handleBrowseKeys handles keyboard input in browse state.
func (m ImportBrowserModel) handleBrowseKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If filter is active, handle filter input
	if m.filterActive {
		return m.handleFilterKeys(msg)
	}

	switch msg.String() {
	case "q", "ctrl+c":
		m.result.Aborted = true
		return m, tea.Quit

	case "/":
		// Enter filter mode
		m.filterActive = true
		m.filterInput.SetValue("")
		m.filterText = ""
		return m, m.filterInput.Focus()

	case "j", "down":
		m.scroller.moveDown()
		return m, m.triggerSelectedSizeCalc()

	case "k", "up":
		m.scroller.moveUp()
		return m, m.triggerSelectedSizeCalc()

	case "g":
		m.scroller.moveToTop()
		return m, m.triggerSelectedSizeCalc()

	case "G":
		m.scroller.moveToBottom()
		return m, m.triggerSelectedSizeCalc()

	case "l", "right":
		node := m.scroller.selectedNode()
		if node != nil && node.IsDir && !node.IsExpanded {
			node.expandNode(m.gitRootSet, m.showHidden)
			m.refreshTree()
		} else if m.activePane == IBPaneTree {
			m.activePane = IBPaneDetails
		}
		return m, m.triggerSelectedSizeCalc()

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
			node.toggleExpand(m.gitRootSet, m.showHidden)
			m.refreshTree()
		}
		return m, m.triggerSelectedSizeCalc()

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
		return m, m.triggerSelectedSizeCalc()

	case "r":
		// Refresh tree
		m.refresh()
		return m, nil

	case ".":
		// Toggle hidden files
		m.showHidden = !m.showHidden
		m.refresh()
		if m.showHidden {
			m.message = "Showing hidden files"
		} else {
			m.message = "Hiding hidden files"
		}
		m.messageIsError = false
		return m, nil

	case "i":
		// Check if multiple folders are selected for batch import
		selectedNodes := m.scroller.getSelectedNodes()
		if len(selectedNodes) > 1 {
			// Start batch import
			return m.startBatchImport(selectedNodes)
		}
		// Start single import for selected folder
		node := m.scroller.selectedNode()
		if node != nil && node.IsDir {
			m.startImport(node)
			return m, m.ownerInput.Focus()
		}
		return m, nil

	case "s":
		// Check if multiple folders are selected for batch stash
		selectedNodes := m.scroller.getSelectedNodes()
		if len(selectedNodes) > 1 {
			// Start batch stash (keep sources)
			return m.startBatchStash(selectedNodes, false)
		}
		// Start single stash for selected folder (keep source)
		node := m.scroller.selectedNode()
		if node != nil && node.IsDir {
			m.startStash(node, false)
			return m, m.stashNameInput.Focus()
		}
		return m, nil

	case "S":
		// Check if multiple folders are selected for batch stash
		selectedNodes := m.scroller.getSelectedNodes()
		if len(selectedNodes) > 1 {
			// Start batch stash (delete sources after)
			return m.startBatchStash(selectedNodes, true)
		}
		// Start single stash for selected folder (delete source after)
		node := m.scroller.selectedNode()
		if node != nil && node.IsDir {
			m.startStash(node, true)
			return m, m.stashNameInput.Focus()
		}
		return m, nil

	case "a":
		// Add selected folder to existing workspace
		node := m.scroller.selectedNode()
		if node != nil && node.IsDir {
			return m.startAddToWorkspace(node)
		}
		return m, nil

	case "d":
		// Delete selected folder (permanent)
		node := m.scroller.selectedNode()
		if node != nil && node.IsDir && node != m.root {
			m.deleteTarget = node
			m.deleteIsTrash = false
			m.state = StateDeleteConfirm
		}
		return m, nil

	case "t":
		// Trash selected folder (move to system trash)
		node := m.scroller.selectedNode()
		if node != nil && node.IsDir && node != m.root {
			m.deleteTarget = node
			m.deleteIsTrash = true
			m.state = StateTrashConfirm
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

// startBatchImport initializes batch import for multiple selected folders.
func (m ImportBrowserModel) startBatchImport(nodes []*sourceNode) (tea.Model, tea.Cmd) {
	m.batchImportTargets = nodes
	m.batchImportResults = nil
	m.batchImportCurrent = 0
	m.batchOwner = ""
	m.state = StateBatchImportConfirm
	m.ownerInput.SetValue("")
	return m, m.ownerInput.Focus()
}

// handleBatchImportConfirmKeys handles keyboard input in batch import confirm state.
func (m ImportBrowserModel) handleBatchImportConfirmKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.result.Aborted = true
		return m, tea.Quit

	case "esc":
		// Cancel batch import, go back to browse
		m.batchImportTargets = nil
		m.state = StateBrowse
		return m, nil

	case "enter":
		// Validate owner is set
		owner := strings.TrimSpace(m.ownerInput.Value())
		if owner == "" {
			m.configError = "Owner is required"
			return m, nil
		}
		if !isValidSlugPart(owner) {
			m.configError = "Owner must be lowercase letters, numbers, and hyphens"
			return m, nil
		}

		// Start batch import execution
		m.batchOwner = owner
		m.configError = ""
		return m.executeBatchImport()
	}

	// Handle text input
	var cmd tea.Cmd
	m.ownerInput, cmd = m.ownerInput.Update(msg)
	return m, cmd
}

// executeBatchImport processes all selected folders and imports them.
func (m ImportBrowserModel) executeBatchImport() (tea.Model, tea.Cmd) {
	m.state = StateBatchImportExecute
	m.batchImportResults = make([]BatchImportItemResult, 0, len(m.batchImportTargets))

	for i, node := range m.batchImportTargets {
		m.batchImportCurrent = i

		// Create workspace slug from owner and folder name
		project := sanitizeForSlug(node.Name)
		_ = fmt.Sprintf("%s--%s", m.batchOwner, project) // slug used for reference

		// Get git roots under this node
		var gitRoots []string
		if node.IsGitRepo {
			gitRoots = []string{node.Path}
		} else {
			prefix := node.Path + string(filepath.Separator)
			for gitRoot := range m.gitRootSet {
				if strings.HasPrefix(gitRoot, prefix) {
					gitRoots = append(gitRoots, gitRoot)
				}
			}
		}

		// Build import options
		opts := workspace.ImportOptions{
			Owner:   m.batchOwner,
			Project: project,
		}

		// Execute the import
		result, err := workspace.CreateWorkspace(m.cfg, node.Path, gitRoots, opts)

		itemResult := BatchImportItemResult{
			SourcePath: node.Path,
			SourceName: node.Name,
		}

		if err != nil {
			itemResult.Success = false
			itemResult.Error = err
		} else {
			itemResult.Success = true
			itemResult.WorkspaceSlug = result.WorkspaceSlug
			itemResult.WorkspacePath = result.WorkspacePath
			itemResult.RepoCount = len(result.ReposImported)

			// Clean up empty source if applicable
			if result.SourceEmpty {
				workspace.RemoveEmptySource(node.Path)
			}
		}

		m.batchImportResults = append(m.batchImportResults, itemResult)
	}

	// Clear selections and refresh tree
	m.scroller.clearAllSelections()
	m.refresh()

	// Go to summary
	m.state = StateBatchImportSummary
	return m, nil
}

// handleBatchImportSummaryKeys handles keyboard input in batch import summary state.
func (m ImportBrowserModel) handleBatchImportSummaryKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.result.Aborted = true
		return m, tea.Quit

	case "enter", "esc", "q":
		// Return to browse
		m.batchImportTargets = nil
		m.batchImportResults = nil
		m.state = StateBrowse
		return m, nil
	}

	return m, nil
}

// startBatchStash initializes batch stash for multiple selected folders.
func (m ImportBrowserModel) startBatchStash(nodes []*sourceNode, deleteAfter bool) (tea.Model, tea.Cmd) {
	m.batchStashTargets = nodes
	m.batchStashResults = nil
	m.batchStashCurrent = 0
	m.batchStashDeleteAfter = deleteAfter
	m.state = StateBatchStashConfirm
	return m, nil
}

// handleBatchStashConfirmKeys handles keyboard input in batch stash confirm state.
func (m ImportBrowserModel) handleBatchStashConfirmKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.result.Aborted = true
		return m, tea.Quit

	case "esc", "q":
		// Cancel batch stash, go back to browse
		m.batchStashTargets = nil
		m.state = StateBrowse
		return m, nil

	case "d", " ":
		// Toggle delete after stash
		m.batchStashDeleteAfter = !m.batchStashDeleteAfter
		return m, nil

	case "enter":
		// Start batch stash execution
		return m.executeBatchStash()
	}

	return m, nil
}

// executeBatchStash processes all selected folders and stashes them.
func (m ImportBrowserModel) executeBatchStash() (tea.Model, tea.Cmd) {
	m.state = StateBatchStashExecute
	m.batchStashResults = make([]BatchStashItemResult, 0, len(m.batchStashTargets))

	for i, node := range m.batchStashTargets {
		m.batchStashCurrent = i

		opts := archive.StashOptions{
			Name:        node.Name,
			DeleteAfter: m.batchStashDeleteAfter,
		}

		result, err := archive.StashFolder(m.cfg, node.Path, opts)

		itemResult := BatchStashItemResult{
			SourcePath: node.Path,
			SourceName: node.Name,
		}

		if err != nil {
			itemResult.Success = false
			itemResult.Error = err
		} else {
			itemResult.Success = true
			itemResult.ArchivePath = result.ArchivePath
			itemResult.Deleted = result.Deleted
		}

		m.batchStashResults = append(m.batchStashResults, itemResult)
	}

	// Clear selections and refresh tree
	m.scroller.clearAllSelections()
	m.refresh()

	// Go to summary
	m.state = StateBatchStashSummary
	return m, nil
}

// handleBatchStashSummaryKeys handles keyboard input in batch stash summary state.
func (m ImportBrowserModel) handleBatchStashSummaryKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.result.Aborted = true
		return m, tea.Quit

	case "enter", "esc", "q":
		// Return to browse
		m.batchStashTargets = nil
		m.batchStashResults = nil
		m.state = StateBrowse
		return m, nil
	}

	return m, nil
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

		// Store config and move to template selection
		m.result.WorkspaceSlug = slug
		m.result.WorkspacePath = workspacePath
		m.configError = ""

		// Proceed to template selection (which may skip to extra files if no templates)
		return m.startTemplateSelect()
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

// startTemplateSelect initializes the template selection state.
func (m ImportBrowserModel) startTemplateSelect() (tea.Model, tea.Cmd) {
	// Load available templates from all template directories
	templateInfos, err := template.ListTemplateInfosMulti(m.cfg.AllTemplatesDirs())
	if err != nil {
		// If we can't load templates, skip to extra files check
		return m.checkForExtraFiles()
	}

	// If no templates available, skip to extra files check
	if len(templateInfos) == 0 {
		return m.checkForExtraFiles()
	}

	m.templateInfos = templateInfos
	m.templateSelected = 0 // Start at "No template" option
	m.templateScrollOffset = 0
	m.selectedTemplate = ""
	m.state = StateTemplateSelect

	return m, nil
}

// handleTemplateSelectKeys handles keyboard input in template selection state.
func (m ImportBrowserModel) handleTemplateSelectKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.result.Aborted = true
		return m, tea.Quit

	case "esc", "q":
		// Cancel, return to import config
		m.state = StateImportConfig
		return m, m.ownerInput.Focus()

	case "j", "down":
		// +1 because first option is "No template"
		maxIdx := len(m.templateInfos)
		if m.templateSelected < maxIdx {
			m.templateSelected++
			m.ensureTemplateVisible()
		}
		return m, nil

	case "k", "up":
		if m.templateSelected > 0 {
			m.templateSelected--
			m.ensureTemplateVisible()
		}
		return m, nil

	case "g":
		m.templateSelected = 0
		m.templateScrollOffset = 0
		return m, nil

	case "G":
		m.templateSelected = len(m.templateInfos)
		m.ensureTemplateVisible()
		return m, nil

	case "enter":
		// Select template and proceed
		if m.templateSelected == 0 {
			// "No template" selected
			m.selectedTemplate = ""
			// No variables to prompt, go to extra files
			return m.checkForExtraFiles()
		}

		// Template selected (index is offset by 1 due to "No template" option)
		m.selectedTemplate = m.templateInfos[m.templateSelected-1].Name

		// Check if template has variables that need prompting
		return m.startTemplateVars()
	}

	return m, nil
}

// startTemplateVars loads template variables and transitions to variable prompting if needed.
func (m ImportBrowserModel) startTemplateVars() (tea.Model, tea.Cmd) {
	if m.selectedTemplate == "" {
		// No template selected, skip to extra files
		return m.checkForExtraFiles()
	}

	// Load the template to get its variables
	tmpl, _, err := template.LoadTemplateMulti(m.cfg.AllTemplatesDirs(), m.selectedTemplate)
	if err != nil {
		// If we can't load the template, show error and go back to template select
		m.message = fmt.Sprintf("Failed to load template: %v", err)
		m.messageIsError = true
		return m, nil
	}

	// Filter variables that need prompting (not built-in, need user input)
	var varsToPrompt []template.TemplateVar
	builtinVars := m.getBuiltinVariables()

	for _, v := range tmpl.Variables {
		// Skip if already has a builtin value
		if _, ok := builtinVars[v.Name]; ok {
			continue
		}
		varsToPrompt = append(varsToPrompt, v)
	}

	// If no variables need prompting, skip to extra files
	if len(varsToPrompt) == 0 {
		// Store template values (just builtins for now)
		m.templateVarValues = builtinVars
		return m.checkForExtraFiles()
	}

	// Initialize variable prompting state
	m.templateVars = varsToPrompt
	m.templateVarIndex = 0
	m.templateVarValues = builtinVars
	m.templateVarError = ""
	m.setupCurrentTemplateVar()
	m.state = StateTemplateVars

	return m, m.templateVarInput.Focus()
}

// getBuiltinVariables returns the built-in variables for the import context.
func (m *ImportBrowserModel) getBuiltinVariables() map[string]string {
	vars := make(map[string]string)

	// Extract owner and project from workspace slug
	if parts := strings.SplitN(m.result.WorkspaceSlug, "--", 2); len(parts) == 2 {
		vars["owner"] = parts[0]
		vars["project"] = parts[1]
	}

	return vars
}

// setupCurrentTemplateVar sets up the input for the current variable.
func (m *ImportBrowserModel) setupCurrentTemplateVar() {
	if m.templateVarIndex >= len(m.templateVars) {
		return
	}

	v := m.templateVars[m.templateVarIndex]

	// Get default value
	defaultVal := ""
	if v.Default != nil {
		defaultVal = fmt.Sprintf("%v", v.Default)
		// Substitute any variable references in default
		if substituted, err := template.SubstituteVariables(defaultVal, m.templateVarValues); err == nil {
			defaultVal = substituted
		}
	}

	switch v.Type {
	case template.VarTypeBoolean:
		m.templateVarBoolValue = defaultVal == "true" || defaultVal == "yes" || defaultVal == "1"
	case template.VarTypeChoice:
		// Find default in choices
		m.templateVarChoiceIdx = 0
		for i, choice := range v.Choices {
			if choice == defaultVal {
				m.templateVarChoiceIdx = i
				break
			}
		}
	default: // string or integer
		m.templateVarInput.SetValue(defaultVal)
	}
}

// handleTemplateVarsKeys handles keyboard input in template variable prompting state.
func (m ImportBrowserModel) handleTemplateVarsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.templateVarIndex >= len(m.templateVars) {
		// All variables collected, proceed
		return m.checkForExtraFiles()
	}

	v := m.templateVars[m.templateVarIndex]

	switch msg.String() {
	case "ctrl+c":
		m.result.Aborted = true
		return m, tea.Quit

	case "esc":
		// Go back to template selection
		m.state = StateTemplateSelect
		m.templateVarError = ""
		return m, nil
	}

	// Handle input based on variable type
	switch v.Type {
	case template.VarTypeBoolean:
		return m.handleTemplateVarBoolKeys(msg, v)
	case template.VarTypeChoice:
		return m.handleTemplateVarChoiceKeys(msg, v)
	default:
		return m.handleTemplateVarTextKeys(msg, v)
	}
}

// handleTemplateVarBoolKeys handles boolean variable input.
func (m ImportBrowserModel) handleTemplateVarBoolKeys(msg tea.KeyMsg, v template.TemplateVar) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "t", "1":
		m.templateVarBoolValue = true
	case "n", "N", "f", "0":
		m.templateVarBoolValue = false
	case "left", "h", "right", "l", "tab", " ":
		m.templateVarBoolValue = !m.templateVarBoolValue
	case "enter":
		if m.templateVarBoolValue {
			m.templateVarValues[v.Name] = "true"
		} else {
			m.templateVarValues[v.Name] = "false"
		}
		m.templateVarError = ""
		m.templateVarIndex++
		if m.templateVarIndex >= len(m.templateVars) {
			return m.checkForExtraFiles()
		}
		m.setupCurrentTemplateVar()
	}
	return m, nil
}

// handleTemplateVarChoiceKeys handles choice variable input.
func (m ImportBrowserModel) handleTemplateVarChoiceKeys(msg tea.KeyMsg, v template.TemplateVar) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.templateVarChoiceIdx < len(v.Choices)-1 {
			m.templateVarChoiceIdx++
		}
	case "k", "up":
		if m.templateVarChoiceIdx > 0 {
			m.templateVarChoiceIdx--
		}
	case "enter":
		m.templateVarValues[v.Name] = v.Choices[m.templateVarChoiceIdx]
		m.templateVarError = ""
		m.templateVarIndex++
		if m.templateVarIndex >= len(m.templateVars) {
			return m.checkForExtraFiles()
		}
		m.setupCurrentTemplateVar()
	}
	return m, nil
}

// handleTemplateVarTextKeys handles text/integer variable input.
func (m ImportBrowserModel) handleTemplateVarTextKeys(msg tea.KeyMsg, v template.TemplateVar) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		value := strings.TrimSpace(m.templateVarInput.Value())

		// Validate required
		if v.Required && value == "" {
			m.templateVarError = fmt.Sprintf("%s is required", v.Name)
			return m, nil
		}

		// Validate integer type
		if v.Type == template.VarTypeInteger && value != "" {
			if _, err := fmt.Sscanf(value, "%d", new(int)); err != nil {
				m.templateVarError = "must be a valid integer"
				return m, nil
			}
		}

		// Validate pattern if specified
		if value != "" {
			if err := template.ValidateVarValue(v, value); err != nil {
				m.templateVarError = err.Error()
				return m, nil
			}
		}

		m.templateVarValues[v.Name] = value
		m.templateVarError = ""
		m.templateVarInput.SetValue("")
		m.templateVarIndex++
		if m.templateVarIndex >= len(m.templateVars) {
			return m.checkForExtraFiles()
		}
		m.setupCurrentTemplateVar()
		return m, m.templateVarInput.Focus()
	}

	// Update text input
	var cmd tea.Cmd
	m.templateVarInput, cmd = m.templateVarInput.Update(msg)
	return m, cmd
}

// ensureTemplateVisible ensures the selected template is visible in the viewport.
func (m *ImportBrowserModel) ensureTemplateVisible() {
	visibleLines := m.height - 12
	if visibleLines < 5 {
		visibleLines = 5
	}

	if m.templateSelected < m.templateScrollOffset {
		m.templateScrollOffset = m.templateSelected
	} else if m.templateSelected >= m.templateScrollOffset+visibleLines {
		m.templateScrollOffset = m.templateSelected - visibleLines + 1
	}
}

// startAddToWorkspace initializes the add-to-workspace state for the selected folder.
func (m ImportBrowserModel) startAddToWorkspace(node *sourceNode) (tea.Model, tea.Cmd) {
	// Load available workspaces
	workspaces, err := fs.ListWorkspaces(m.cfg.CodeRoot)
	if err != nil {
		m.message = fmt.Sprintf("Failed to list workspaces: %v", err)
		m.messageIsError = true
		return m, nil
	}

	if len(workspaces) == 0 {
		m.message = "No existing workspaces found. Use 'i' to create a new workspace."
		m.messageIsError = true
		return m, nil
	}

	m.state = StateAddToSelect
	m.importTarget = node
	m.addToWorkspaces = workspaces
	m.addToSelected = 0
	m.addToScrollOffset = 0
	m.addToTargetSlug = ""

	return m, nil
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

// executeStash performs the actual stash operation asynchronously.
func (m ImportBrowserModel) executeStash() (tea.Model, tea.Cmd) {
	if m.stashTarget == nil {
		m.stashError = "no folder selected"
		return m, nil
	}

	// Get archive name
	name := strings.TrimSpace(m.stashNameInput.Value())
	if name == "" {
		name = m.stashTarget.Name
	}

	// Capture values for async operation
	cfg := m.cfg
	targetPath := m.stashTarget.Path
	targetName := m.stashTarget.Name
	deleteAfter := m.stashDeleteAfter

	// Set loading state
	m.loading = true
	if deleteAfter {
		m.loadingMessage = fmt.Sprintf("Stashing and deleting: %s...", targetName)
	} else {
		m.loadingMessage = fmt.Sprintf("Stashing: %s...", targetName)
	}
	m.spinnerFrame = 0

	// Return commands: one for the operation, one for spinner animation
	operationCmd := func() tea.Msg {
		opts := archive.StashOptions{
			Name:        name,
			DeleteAfter: deleteAfter,
		}

		result, err := archive.StashFolder(cfg, targetPath, opts)
		if err != nil {
			return operationResultMsg{
				Operation: "stash",
				Success:   false,
				Message:   fmt.Sprintf("Stash failed: %v", err),
				Err:       err,
			}
		}

		msg := fmt.Sprintf("Stashed: %s", result.ArchivePath)
		if result.Deleted {
			msg += " (source deleted)"
		}
		return operationResultMsg{
			Operation: "stash",
			Success:   true,
			Message:   msg,
		}
	}

	return m, tea.Batch(operationCmd, m.spinnerTick())
}

// handleDeleteConfirmKeys handles keyboard input in delete/trash confirm states.
func (m ImportBrowserModel) handleDeleteConfirmKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.result.Aborted = true
		return m, tea.Quit

	case "esc", "n", "N":
		// Cancel, return to browse
		m.state = StateBrowse
		m.deleteTarget = nil
		return m, nil

	case "y", "Y", "enter":
		// Confirm delete/trash
		return m.executeDelete()
	}

	return m, nil
}

// executeDelete performs the delete or trash operation.
func (m ImportBrowserModel) executeDelete() (tea.Model, tea.Cmd) {
	if m.deleteTarget == nil {
		m.state = StateBrowse
		return m, nil
	}

	targetPath := m.deleteTarget.Path
	targetName := m.deleteTarget.Name

	var err error
	if m.deleteIsTrash {
		err = trashPath(targetPath)
	} else {
		err = os.RemoveAll(targetPath)
	}

	if err != nil {
		if m.deleteIsTrash {
			m.message = fmt.Sprintf("Trash failed: %v", err)
		} else {
			m.message = fmt.Sprintf("Delete failed: %v", err)
		}
		m.messageIsError = true
		m.state = StateBrowse
		m.deleteTarget = nil
		return m, nil
	}

	// Success - refresh tree and show message
	m.refresh()
	if m.deleteIsTrash {
		m.message = fmt.Sprintf("Moved to trash: %s", targetName)
	} else {
		m.message = fmt.Sprintf("Deleted: %s", targetName)
	}
	m.messageIsError = false
	m.state = StateBrowse
	m.deleteTarget = nil

	return m, nil
}

// trashPath moves a file or directory to the system trash.
// On macOS, it uses the 'trash' command if available, otherwise falls back to AppleScript.
// On other systems, it falls back to permanent deletion with a warning.
func trashPath(path string) error {
	// Try the 'trash' command first (from Homebrew: brew install trash)
	if _, err := exec.LookPath("trash"); err == nil {
		cmd := exec.Command("trash", path)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	// On macOS, try AppleScript as fallback
	if isRunningOnMac() {
		// Use AppleScript to move to trash
		script := fmt.Sprintf(`tell application "Finder" to delete POSIX file %q`, path)
		cmd := exec.Command("osascript", "-e", script)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	// Try freedesktop trash (gio trash) on Linux
	if _, err := exec.LookPath("gio"); err == nil {
		cmd := exec.Command("gio", "trash", path)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	// Try trash-cli on Linux
	if _, err := exec.LookPath("trash-put"); err == nil {
		cmd := exec.Command("trash-put", path)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	// No trash available - return error suggesting permanent delete
	return fmt.Errorf("no trash utility available; use 'd' for permanent delete")
}

// isRunningOnMac returns true if running on macOS.
func isRunningOnMac() bool {
	cmd := exec.Command("uname", "-s")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "Darwin"
}

// checkForExtraFiles looks for non-git files and transitions to the appropriate state.
func (m ImportBrowserModel) checkForExtraFiles() (tea.Model, tea.Cmd) {
	if m.importTarget == nil {
		m.extraFilesResult = ExtraFilesResult{} // Clear previous results
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
		m.extraFilesResult = ExtraFilesResult{} // Clear previous results
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
// It preserves the current selection position and expansion state.
func (m *ImportBrowserModel) refresh() {
	// Save current selection path before rebuilding
	var previousPath string
	if node := m.scroller.selectedNode(); node != nil {
		previousPath = node.Path
	}

	// Collect all expanded paths from the current tree
	expandedPaths := m.collectExpandedPaths()

	root, err := buildSourceTree(m.rootPath, m.showHidden)
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

	// Restore expansion state to the new tree
	m.restoreExpandedPaths(expandedPaths)

	m.refreshTree()

	// Restore selection to previous path (or nearest sibling/parent)
	if previousPath != "" {
		m.scroller.selectByPath(previousPath)
	}

	m.message = "Refreshed"
	m.messageIsError = false
}

// collectExpandedPaths returns a set of paths for all expanded directories.
func (m *ImportBrowserModel) collectExpandedPaths() map[string]bool {
	expanded := make(map[string]bool)
	if m.root != nil {
		collectExpandedPathsRecursive(m.root, expanded)
	}
	return expanded
}

// collectExpandedPathsRecursive walks the tree and collects expanded paths.
func collectExpandedPathsRecursive(node *sourceNode, expanded map[string]bool) {
	if node.IsDir && node.IsExpanded {
		expanded[node.Path] = true
		for _, child := range node.Children {
			collectExpandedPathsRecursive(child, expanded)
		}
	}
}

// restoreExpandedPaths expands directories in the new tree that were previously expanded.
func (m *ImportBrowserModel) restoreExpandedPaths(expandedPaths map[string]bool) {
	if m.root != nil {
		restoreExpandedPathsRecursive(m.root, expandedPaths, m.gitRootSet, m.showHidden)
	}
}

// restoreExpandedPathsRecursive walks the new tree and expands matching paths.
func restoreExpandedPathsRecursive(node *sourceNode, expandedPaths map[string]bool, gitRootSet map[string]bool, showHidden bool) {
	if node.IsDir && expandedPaths[node.Path] {
		// Expand this node (load its children if not already loaded)
		node.expandNode(gitRootSet, showHidden)
		// Recursively restore children
		for _, child := range node.Children {
			restoreExpandedPathsRecursive(child, expandedPaths, gitRootSet, showHidden)
		}
	}
}

// spinnerTick returns a command that triggers a spinner animation tick.
func (m ImportBrowserModel) spinnerTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}

// renderLoadingView renders a loading indicator overlay.
func (m ImportBrowserModel) renderLoadingView() string {
	var sb strings.Builder

	sb.WriteString("\n\n")
	sb.WriteString(ibHeaderStyle.Render("Working...") + "\n\n")

	// Show animated spinner
	spinner := spinnerFrames[m.spinnerFrame%len(spinnerFrames)]
	sb.WriteString(fmt.Sprintf("  %s %s\n", spinner, m.loadingMessage))

	sb.WriteString("\n\n")
	sb.WriteString(ibHelpStyle.Render("Please wait..."))

	return sb.String()
}

// View implements tea.Model.
func (m ImportBrowserModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Show loading overlay if an async operation is in progress
	if m.loading {
		return m.renderLoadingView()
	}

	switch m.state {
	case StateImportConfig:
		return m.renderImportConfigView()
	case StateTemplateSelect:
		return m.renderTemplateSelectView()
	case StateTemplateVars:
		return m.renderTemplateVarsView()
	case StateImportPreview:
		return m.renderImportPreviewView()
	case StateStashConfirm:
		return m.renderStashConfirmView()
	case StateExtraFiles:
		return m.renderExtraFilesView()
	case StatePostImport:
		return m.renderPostImportView()
	case StateAddToSelect:
		return m.renderAddToSelectView()
	case StateBatchImportConfirm:
		return m.renderBatchImportConfirmView()
	case StateBatchImportExecute:
		return m.renderBatchImportExecuteView()
	case StateBatchImportSummary:
		return m.renderBatchImportSummaryView()
	case StateBatchStashConfirm:
		return m.renderBatchStashConfirmView()
	case StateBatchStashExecute:
		return m.renderBatchStashExecuteView()
	case StateBatchStashSummary:
		return m.renderBatchStashSummaryView()
	case StateDeleteConfirm:
		return m.renderDeleteConfirmView()
	case StateTrashConfirm:
		return m.renderTrashConfirmView()
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

// renderTemplateSelectView renders the template selection view.
func (m ImportBrowserModel) renderTemplateSelectView() string {
	var sb strings.Builder

	sb.WriteString(ibHeaderStyle.Render("Select Template (Optional)") + "\n")
	sb.WriteString(ibHelpStyle.Render("Choose a template to apply to the workspace, or skip.") + "\n\n")

	// Show workspace info
	sb.WriteString(fmt.Sprintf("Workspace: %s\n\n", m.result.WorkspaceSlug))

	// Calculate visible area
	visibleLines := m.height - 12
	if visibleLines < 5 {
		visibleLines = 5
	}

	// Total items = "No template" + actual templates
	totalItems := 1 + len(m.templateInfos)

	// Render items
	startIdx := m.templateScrollOffset
	endIdx := startIdx + visibleLines
	if endIdx > totalItems {
		endIdx = totalItems
	}

	for i := startIdx; i < endIdx; i++ {
		var line string
		isSelected := i == m.templateSelected

		if i == 0 {
			// "No template" option
			line = m.renderTemplateItem("No template", "Create workspace without applying a template", 0, 0, isSelected)
		} else {
			// Actual template (index offset by 1)
			tmpl := m.templateInfos[i-1]
			line = m.renderTemplateItem(tmpl.Name, tmpl.Description, tmpl.VarCount, tmpl.RepoCount, isSelected)
		}
		sb.WriteString(line + "\n")
	}

	// Scroll indicator
	if totalItems > visibleLines {
		sb.WriteString(fmt.Sprintf("\n(%d/%d)", m.templateSelected+1, totalItems))
	}

	// Help
	sb.WriteString("\n\n" + ibHelpStyle.Render("j/k: navigate • g/G: top/bottom • enter: select • esc: back"))

	return sb.String()
}

// renderTemplateItem renders a single template item in the selection list.
func (m ImportBrowserModel) renderTemplateItem(name, description string, varCount, repoCount int, isSelected bool) string {
	prefix := "  "
	if isSelected {
		prefix = "> "
	}

	// Build description with counts
	desc := description
	if repoCount > 0 && varCount > 0 {
		desc = fmt.Sprintf("%s (%d vars, %d repos)", description, varCount, repoCount)
	} else if varCount > 0 {
		desc = fmt.Sprintf("%s (%d vars)", description, varCount)
	} else if repoCount > 0 {
		desc = fmt.Sprintf("%s (%d repos)", description, repoCount)
	}

	line := fmt.Sprintf("%s%s", prefix, name)
	if desc != "" {
		line += "\n    " + ibHelpStyle.Render(desc)
	}

	if isSelected {
		// Style the entire line for selected item
		lines := strings.Split(line, "\n")
		lines[0] = ibSelectedStyle.Render(lines[0])
		line = strings.Join(lines, "\n")
	}

	return line
}

// renderTemplateVarsView renders the template variable prompting view.
func (m ImportBrowserModel) renderTemplateVarsView() string {
	var sb strings.Builder

	sb.WriteString(ibHeaderStyle.Render("Template Variables") + "\n")

	// Show progress
	if len(m.templateVars) > 0 {
		sb.WriteString(ibHelpStyle.Render(fmt.Sprintf("Variable %d of %d", m.templateVarIndex+1, len(m.templateVars))) + "\n\n")
	}

	// Show workspace and template context
	sb.WriteString(fmt.Sprintf("Workspace: %s\n", m.result.WorkspaceSlug))
	sb.WriteString(fmt.Sprintf("Template:  %s\n\n", m.selectedTemplate))

	// Check bounds
	if m.templateVarIndex >= len(m.templateVars) {
		sb.WriteString("All variables collected.\n")
		return sb.String()
	}

	v := m.templateVars[m.templateVarIndex]

	// Variable name with required indicator
	nameStyle := ibHeaderStyle
	sb.WriteString(nameStyle.Render(v.Name))
	if v.Required {
		sb.WriteString(ibErrorStyle.Render(" *"))
	}
	sb.WriteString("\n")

	// Description
	if v.Description != "" {
		sb.WriteString(ibHelpStyle.Render(v.Description) + "\n")
	}
	sb.WriteString("\n")

	// Input based on type
	switch v.Type {
	case template.VarTypeBoolean:
		yes := "  yes  "
		no := "  no  "
		if m.templateVarBoolValue {
			yes = ibSelectedStyle.Render(" [yes] ")
		} else {
			no = ibSelectedStyle.Render(" [no] ")
		}
		sb.WriteString(fmt.Sprintf("  %s    %s\n", yes, no))

	case template.VarTypeChoice:
		// Render choice list
		for i, choice := range v.Choices {
			prefix := "  "
			if i == m.templateVarChoiceIdx {
				prefix = "> "
				sb.WriteString(ibSelectedStyle.Render(prefix+choice) + "\n")
			} else {
				sb.WriteString(prefix + choice + "\n")
			}
		}

	default: // string or integer
		sb.WriteString(m.templateVarInput.View() + "\n")
		if v.Type == template.VarTypeInteger {
			sb.WriteString(ibHelpStyle.Render("(integer value)") + "\n")
		}
	}

	// Error message
	if m.templateVarError != "" {
		sb.WriteString("\n" + ibErrorStyle.Render("Error: "+m.templateVarError) + "\n")
	}

	// Help
	sb.WriteString("\n" + m.renderHelp())

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

// renderPostImportView renders the post-import options view.
func (m ImportBrowserModel) renderPostImportView() string {
	var sb strings.Builder

	sb.WriteString(ibHeaderStyle.Render("Import Complete!") + "\n\n")

	// Show what was created
	sb.WriteString(ibSuccessStyle.Render(fmt.Sprintf("Created workspace: %s", m.result.WorkspaceSlug)) + "\n")
	sb.WriteString(fmt.Sprintf("Path: %s\n", m.result.WorkspacePath))

	if len(m.result.ReposImported) > 0 {
		sb.WriteString(fmt.Sprintf("Repos: %d imported\n", len(m.result.ReposImported)))
	}
	if len(m.result.FilesImported) > 0 {
		sb.WriteString(fmt.Sprintf("Files: %d copied\n", len(m.result.FilesImported)))
	}

	// Show template application results
	if m.result.TemplateApplied != "" {
		sb.WriteString(ibSuccessStyle.Render(fmt.Sprintf("Template: %s applied", m.result.TemplateApplied)) + "\n")
		if m.result.TemplateFilesCreated > 0 {
			sb.WriteString(fmt.Sprintf("Template files: %d created\n", m.result.TemplateFilesCreated))
		}
	} else if m.selectedTemplate != "" {
		// Template was selected but failed to apply
		errMsg := "unknown error"
		if m.result.TemplateError != nil {
			errMsg = m.result.TemplateError.Error()
		}
		sb.WriteString(ibErrorStyle.Render(fmt.Sprintf("Template: %s (failed: %s)", m.selectedTemplate, errMsg)) + "\n")
	}

	sb.WriteString(fmt.Sprintf("\nSource folder: %s\n", m.postImportSourcePath))
	sb.WriteString(ibHelpStyle.Render("(contains remaining files after import)") + "\n\n")

	sb.WriteString("What would you like to do with the source folder?\n\n")

	// Options
	options := []string{
		"Keep source folder",
		"Stash source (archive and delete)",
		"Delete source folder",
	}

	for i, opt := range options {
		prefix := "  "
		if i == m.postImportOption {
			prefix = "> "
			sb.WriteString(ibSelectedStyle.Render(fmt.Sprintf("%s[%d] %s", prefix, i+1, opt)) + "\n")
		} else {
			sb.WriteString(fmt.Sprintf("%s[%d] %s\n", prefix, i+1, opt))
		}
	}

	// Warning for destructive options
	if m.postImportOption == 2 {
		sb.WriteString("\n" + ibErrorStyle.Render("WARNING: This will permanently delete the source folder!") + "\n")
	}

	sb.WriteString("\n" + ibHelpStyle.Render("j/k: select • 1/2/3: quick select • enter: confirm"))

	return sb.String()
}

// renderAddToSelectView renders the workspace selection view for add-to mode.
func (m ImportBrowserModel) renderAddToSelectView() string {
	var sb strings.Builder

	sb.WriteString(ibHeaderStyle.Render("Add to Existing Workspace") + "\n")
	sb.WriteString(ibHelpStyle.Render("Select a workspace to add the folder to.") + "\n\n")

	// Show source info
	if m.importTarget != nil {
		sb.WriteString(fmt.Sprintf("Source: %s\n", m.importTarget.Path))

		// Count repos
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
		if repoCount > 0 {
			sb.WriteString(fmt.Sprintf("Repos:  %d\n", repoCount))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Workspaces:\n")

	// Calculate visible area
	visibleLines := m.height - 14
	if visibleLines < 5 {
		visibleLines = 5
	}

	// Render workspace list
	startIdx := m.addToScrollOffset
	endIdx := startIdx + visibleLines
	if endIdx > len(m.addToWorkspaces) {
		endIdx = len(m.addToWorkspaces)
	}

	for i := startIdx; i < endIdx; i++ {
		ws := m.addToWorkspaces[i]
		prefix := "  "
		if i == m.addToSelected {
			prefix = "> "
			sb.WriteString(ibSelectedStyle.Render(fmt.Sprintf("%s%s", prefix, ws)) + "\n")
		} else {
			sb.WriteString(fmt.Sprintf("%s%s\n", prefix, ws))
		}
	}

	// Scroll indicator
	if len(m.addToWorkspaces) > visibleLines {
		sb.WriteString(fmt.Sprintf("\n(%d/%d)", m.addToSelected+1, len(m.addToWorkspaces)))
	}

	// Help
	sb.WriteString("\n\n" + ibHelpStyle.Render("j/k: navigate • g/G: top/bottom • enter: select • esc: cancel"))

	return sb.String()
}

// renderBatchImportConfirmView renders the batch import confirmation view.
func (m ImportBrowserModel) renderBatchImportConfirmView() string {
	var sb strings.Builder

	sb.WriteString(ibHeaderStyle.Render("Batch Import") + "\n")
	sb.WriteString(ibHelpStyle.Render(fmt.Sprintf("Import %d folders as separate workspaces", len(m.batchImportTargets))) + "\n\n")

	// List folders to import
	sb.WriteString("Folders to import:\n")
	maxShow := 10
	for i, node := range m.batchImportTargets {
		if i >= maxShow {
			sb.WriteString(fmt.Sprintf("  ... and %d more\n", len(m.batchImportTargets)-maxShow))
			break
		}
		sb.WriteString(fmt.Sprintf("  • %s\n", node.Name))
	}
	sb.WriteString("\n")

	// Owner input (shared for all)
	sb.WriteString("Owner (for all workspaces):\n")
	sb.WriteString(m.ownerInput.View() + "\n")

	// Show example slug
	if len(m.batchImportTargets) > 0 {
		owner := strings.TrimSpace(m.ownerInput.Value())
		if owner != "" {
			example := fmt.Sprintf("%s--%s", owner, sanitizeForSlug(m.batchImportTargets[0].Name))
			sb.WriteString(ibHelpStyle.Render(fmt.Sprintf("Example: %s", example)) + "\n")
		}
	}

	// Error message
	if m.configError != "" {
		sb.WriteString("\n" + ibErrorStyle.Render("Error: "+m.configError) + "\n")
	}

	// Help
	sb.WriteString("\n" + ibHelpStyle.Render("enter: start import • esc: cancel"))

	return sb.String()
}

// renderBatchImportExecuteView renders the batch import progress view.
func (m ImportBrowserModel) renderBatchImportExecuteView() string {
	var sb strings.Builder

	sb.WriteString(ibHeaderStyle.Render("Batch Import in Progress...") + "\n\n")

	total := len(m.batchImportTargets)
	current := m.batchImportCurrent + 1
	if current > total {
		current = total
	}

	sb.WriteString(fmt.Sprintf("Importing folder %d of %d...\n", current, total))

	if m.batchImportCurrent < len(m.batchImportTargets) {
		sb.WriteString(fmt.Sprintf("Current: %s\n", m.batchImportTargets[m.batchImportCurrent].Name))
	}

	return sb.String()
}

// renderBatchImportSummaryView renders the batch import results summary.
func (m ImportBrowserModel) renderBatchImportSummaryView() string {
	var sb strings.Builder

	sb.WriteString(ibHeaderStyle.Render("Batch Import Complete") + "\n\n")

	// Count successes and failures
	successCount := 0
	failCount := 0
	for _, r := range m.batchImportResults {
		if r.Success {
			successCount++
		} else {
			failCount++
		}
	}

	// Summary line
	if failCount == 0 {
		sb.WriteString(ibSuccessStyle.Render(fmt.Sprintf("All %d imports succeeded!", successCount)) + "\n\n")
	} else if successCount == 0 {
		sb.WriteString(ibErrorStyle.Render(fmt.Sprintf("All %d imports failed!", failCount)) + "\n\n")
	} else {
		sb.WriteString(fmt.Sprintf("%s, %s\n\n",
			ibSuccessStyle.Render(fmt.Sprintf("%d succeeded", successCount)),
			ibErrorStyle.Render(fmt.Sprintf("%d failed", failCount))))
	}

	// Detailed results
	sb.WriteString("Results:\n")
	maxShow := 15
	for i, r := range m.batchImportResults {
		if i >= maxShow {
			remaining := len(m.batchImportResults) - maxShow
			sb.WriteString(fmt.Sprintf("  ... and %d more\n", remaining))
			break
		}

		if r.Success {
			sb.WriteString(fmt.Sprintf("  ✓ %s → %s (%d repos)\n", r.SourceName, r.WorkspaceSlug, r.RepoCount))
		} else {
			errMsg := "unknown error"
			if r.Error != nil {
				errMsg = r.Error.Error()
				// Truncate long error messages
				if len(errMsg) > 50 {
					errMsg = errMsg[:47] + "..."
				}
			}
			sb.WriteString(ibErrorStyle.Render(fmt.Sprintf("  ✗ %s: %s", r.SourceName, errMsg)) + "\n")
		}
	}

	// Help
	sb.WriteString("\n" + ibHelpStyle.Render("enter/esc: return to browse"))

	return sb.String()
}

// renderBatchStashConfirmView renders the batch stash confirmation view.
func (m ImportBrowserModel) renderBatchStashConfirmView() string {
	var sb strings.Builder

	sb.WriteString(ibHeaderStyle.Render("Batch Stash") + "\n")
	sb.WriteString(ibHelpStyle.Render(fmt.Sprintf("Stash %d folders to archives", len(m.batchStashTargets))) + "\n\n")

	// List folders to stash
	sb.WriteString("Folders to stash:\n")
	maxShow := 10
	for i, node := range m.batchStashTargets {
		if i >= maxShow {
			sb.WriteString(fmt.Sprintf("  ... and %d more\n", len(m.batchStashTargets)-maxShow))
			break
		}
		sb.WriteString(fmt.Sprintf("  • %s\n", node.Name))
	}
	sb.WriteString("\n")

	// Delete option
	deleteLabel := "Delete after stash: "
	if m.batchStashDeleteAfter {
		sb.WriteString(deleteLabel + ibErrorStyle.Render("[YES - sources will be deleted]") + "\n")
	} else {
		sb.WriteString(deleteLabel + ibSuccessStyle.Render("[no - sources kept]") + "\n")
	}

	// Warning if deleting
	if m.batchStashDeleteAfter {
		sb.WriteString("\n" + ibErrorStyle.Render("WARNING: All source folders will be DELETED after archiving!") + "\n")
	}

	// Help
	sb.WriteString("\n" + ibHelpStyle.Render("d/space: toggle delete • enter: start stash • esc: cancel"))

	return sb.String()
}

// renderBatchStashExecuteView renders the batch stash progress view.
func (m ImportBrowserModel) renderBatchStashExecuteView() string {
	var sb strings.Builder

	sb.WriteString(ibHeaderStyle.Render("Batch Stash in Progress...") + "\n\n")

	total := len(m.batchStashTargets)
	current := m.batchStashCurrent + 1
	if current > total {
		current = total
	}

	sb.WriteString(fmt.Sprintf("Stashing folder %d of %d...\n", current, total))

	if m.batchStashCurrent < len(m.batchStashTargets) {
		sb.WriteString(fmt.Sprintf("Current: %s\n", m.batchStashTargets[m.batchStashCurrent].Name))
	}

	return sb.String()
}

// renderBatchStashSummaryView renders the batch stash results summary.
func (m ImportBrowserModel) renderBatchStashSummaryView() string {
	var sb strings.Builder

	sb.WriteString(ibHeaderStyle.Render("Batch Stash Complete") + "\n\n")

	// Count successes and failures
	successCount := 0
	failCount := 0
	deletedCount := 0
	for _, r := range m.batchStashResults {
		if r.Success {
			successCount++
			if r.Deleted {
				deletedCount++
			}
		} else {
			failCount++
		}
	}

	// Summary line
	if failCount == 0 {
		summary := fmt.Sprintf("All %d stashes succeeded!", successCount)
		if deletedCount > 0 {
			summary += fmt.Sprintf(" (%d sources deleted)", deletedCount)
		}
		sb.WriteString(ibSuccessStyle.Render(summary) + "\n\n")
	} else if successCount == 0 {
		sb.WriteString(ibErrorStyle.Render(fmt.Sprintf("All %d stashes failed!", failCount)) + "\n\n")
	} else {
		sb.WriteString(fmt.Sprintf("%s, %s\n\n",
			ibSuccessStyle.Render(fmt.Sprintf("%d succeeded", successCount)),
			ibErrorStyle.Render(fmt.Sprintf("%d failed", failCount))))
	}

	// Detailed results
	sb.WriteString("Results:\n")
	maxShow := 15
	for i, r := range m.batchStashResults {
		if i >= maxShow {
			remaining := len(m.batchStashResults) - maxShow
			sb.WriteString(fmt.Sprintf("  ... and %d more\n", remaining))
			break
		}

		if r.Success {
			archiveName := filepath.Base(r.ArchivePath)
			suffix := ""
			if r.Deleted {
				suffix = " (deleted)"
			}
			sb.WriteString(fmt.Sprintf("  ✓ %s → %s%s\n", r.SourceName, archiveName, suffix))
		} else {
			errMsg := "unknown error"
			if r.Error != nil {
				errMsg = r.Error.Error()
				// Truncate long error messages
				if len(errMsg) > 50 {
					errMsg = errMsg[:47] + "..."
				}
			}
			sb.WriteString(ibErrorStyle.Render(fmt.Sprintf("  ✗ %s: %s", r.SourceName, errMsg)) + "\n")
		}
	}

	// Help
	sb.WriteString("\n" + ibHelpStyle.Render("enter/esc: return to browse"))

	return sb.String()
}

// renderDeleteConfirmView renders the delete confirmation dialog.
func (m ImportBrowserModel) renderDeleteConfirmView() string {
	var sb strings.Builder

	sb.WriteString(ibErrorStyle.Render("⚠ PERMANENT DELETE") + "\n\n")

	if m.deleteTarget != nil {
		sb.WriteString(fmt.Sprintf("Folder: %s\n", ibSelectedStyle.Render(m.deleteTarget.Name)))
		sb.WriteString(fmt.Sprintf("Path:   %s\n\n", m.deleteTarget.Path))
	}

	sb.WriteString(ibErrorStyle.Render("This will PERMANENTLY delete the folder and all its contents.") + "\n")
	sb.WriteString(ibErrorStyle.Render("This action cannot be undone!") + "\n\n")

	sb.WriteString("Are you sure you want to continue?\n\n")

	sb.WriteString(ibHelpStyle.Render("y/enter: confirm delete • n/esc: cancel"))

	return sb.String()
}

// renderTrashConfirmView renders the trash confirmation dialog.
func (m ImportBrowserModel) renderTrashConfirmView() string {
	var sb strings.Builder

	sb.WriteString(ibHeaderStyle.Render("Move to Trash") + "\n\n")

	if m.deleteTarget != nil {
		sb.WriteString(fmt.Sprintf("Folder: %s\n", ibSelectedStyle.Render(m.deleteTarget.Name)))
		sb.WriteString(fmt.Sprintf("Path:   %s\n\n", m.deleteTarget.Path))
	}

	sb.WriteString("This will move the folder to your system's trash.\n")
	sb.WriteString("You can recover it from the trash if needed.\n\n")

	sb.WriteString("Move to trash?\n\n")

	sb.WriteString(ibHelpStyle.Render("y/enter: confirm • n/esc: cancel"))

	return sb.String()
}

// renderImportPreviewView renders the import preview.
func (m ImportBrowserModel) renderImportPreviewView() string {
	var sb strings.Builder

	// Show different header for add-to vs create
	if m.addToTargetSlug != "" {
		sb.WriteString(ibHeaderStyle.Render("Add to Workspace Preview") + "\n\n")
		sb.WriteString(fmt.Sprintf("Workspace: %s (existing)\n", m.result.WorkspaceSlug))
	} else {
		sb.WriteString(ibHeaderStyle.Render("Import Preview") + "\n\n")
		sb.WriteString(fmt.Sprintf("Workspace: %s (new)\n", m.result.WorkspaceSlug))
	}
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

	// Show selected template
	if m.selectedTemplate != "" {
		sb.WriteString(fmt.Sprintf("\nTemplate: %s\n", ibSuccessStyle.Render(m.selectedTemplate)))
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

	// Show dry-run mode indicator
	if m.dryRun {
		sb.WriteString("\n" + ibGitDirtyStyle.Render("[DRY-RUN MODE - will show what would happen]") + "\n")
	}

	// Show message (dry-run results)
	if m.message != "" && !m.messageIsError {
		sb.WriteString("\n" + m.message)
	}

	if m.dryRun {
		sb.WriteString("\n" + ibHelpStyle.Render("enter: show dry-run • d: disable dry-run • esc: back"))
	} else {
		sb.WriteString("\n" + ibHelpStyle.Render("enter: execute import • d: dry-run • esc: back"))
	}

	return sb.String()
}

// renderTreePane renders the tree view pane.
func (m ImportBrowserModel) renderTreePane() string {
	var sb strings.Builder

	sb.WriteString(ibHeaderStyle.Render("Source Folder") + "\n")

	// Show filter input if active
	if m.filterActive {
		sb.WriteString("Filter: " + m.filterInput.View() + "\n")
	} else if m.filterText != "" {
		sb.WriteString(ibHelpStyle.Render(fmt.Sprintf("Filter: %s (esc to clear)", m.filterText)) + "\n")
	}

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

// formatSize formats a byte count as a human-readable string.
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// getSizeStatus returns the size of a path if cached, or indicates if calculation is pending.
// Returns (size, cached, pending). If cached is true, size is valid. If pending is true,
// calculation is in progress. If both are false, triggerSizeCalc should be called.
func (m *ImportBrowserModel) getSizeStatus(path string, isDir bool) (size int64, cached bool, pending bool) {
	if !isDir {
		// For files, just stat (fast enough to do synchronously)
		info, err := os.Stat(path)
		if err != nil {
			return 0, false, false
		}
		return info.Size(), true, false
	}

	// Check cache
	if size, ok := m.sizeCache[path]; ok {
		return size, true, false
	}

	// Check if calculation is in progress
	if _, ok := m.sizePending[path]; ok {
		return 0, false, true
	}

	return 0, false, false
}

// triggerSizeCalc starts an async size calculation for a directory if not already cached or pending.
// Returns a tea.Cmd that will send a sizeResultMsg when complete.
func (m *ImportBrowserModel) triggerSizeCalc(path string) tea.Cmd {
	// Check if already cached
	if _, ok := m.sizeCache[path]; ok {
		return nil
	}

	// Check if already pending
	if _, ok := m.sizePending[path]; ok {
		return nil
	}

	// Mark as pending
	m.sizePending[path] = struct{}{}

	// Return command that calculates size asynchronously
	return func() tea.Msg {
		size, err := fs.CalculateSize(path)
		return sizeResultMsg{Path: path, Size: size, Err: err}
	}
}

// triggerSelectedSizeCalc triggers async size calculation for the currently selected node.
func (m *ImportBrowserModel) triggerSelectedSizeCalc() tea.Cmd {
	node := m.scroller.selectedNode()
	if node == nil || !node.IsDir {
		return nil
	}
	return m.triggerSizeCalc(node.Path)
}

// renderDetailsPane renders the details pane for the selected item.
func (m *ImportBrowserModel) renderDetailsPane() string {
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

	// Show size (async for directories)
	if size, cached, pending := m.getSizeStatus(node.Path, node.IsDir); cached {
		sb.WriteString(fmt.Sprintf("Size:   %s\n", formatSize(size)))
	} else if pending {
		sb.WriteString("Size:   Calculating...\n")
	} else if node.IsDir {
		sb.WriteString("Size:   —\n") // Will be calculated async
	}

	if node.IsSymlink {
		sb.WriteString("Note:   Symbolic link\n")
		// Show symlink target
		if target, err := os.Readlink(node.Path); err == nil {
			sb.WriteString(fmt.Sprintf("Target: %s\n", target))
		}
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
		if m.filterActive {
			help = "type to filter • enter: confirm • esc: clear"
		} else {
			help = "j/k: nav • space: select • /: filter • i: import • a: add • s/S: stash • .: hidden • q: quit"
		}
	case StateImportConfig:
		help = "tab: next field • enter: confirm • esc: cancel"
	case StateTemplateSelect:
		help = "j/k: navigate • g/G: top/bottom • enter: select • esc: back"
	case StateTemplateVars:
		if len(m.templateVars) > 0 && m.templateVarIndex < len(m.templateVars) {
			v := m.templateVars[m.templateVarIndex]
			switch v.Type {
			case template.VarTypeBoolean:
				help = "y/n: set value • tab/space: toggle • enter: confirm • esc: back"
			case template.VarTypeChoice:
				help = "j/k: navigate • enter: select • esc: back"
			default:
				help = "type value • enter: confirm • esc: back"
			}
		} else {
			help = "enter: continue • esc: back"
		}
	case StateImportPreview:
		if m.dryRun {
			help = "enter: show dry-run • d: disable dry-run • esc: back"
		} else {
			help = "enter: execute import • d: dry-run • esc: back"
		}
	case StateStashConfirm:
		help = "tab: switch field • space/d: toggle delete • enter: stash • esc: cancel"
	case StateExtraFiles:
		if m.extraFilesShowDest {
			help = "enter: confirm • esc: back to selection"
		} else {
			help = "j/k: navigate • space: toggle • a: all • n: none • enter: continue • q/esc: skip"
		}
	case StatePostImport:
		help = "j/k: select • 1/2/3: quick select • enter: confirm"
	case StateAddToSelect:
		help = "j/k: navigate • g/G: top/bottom • enter: select • esc: cancel"
	case StateBatchImportConfirm:
		help = "enter: start import • esc: cancel"
	case StateBatchImportSummary:
		help = "enter/esc: return to browse"
	case StateBatchStashConfirm:
		help = "d/space: toggle delete • enter: start stash • esc: cancel"
	case StateBatchStashSummary:
		help = "enter/esc: return to browse"
	default:
		help = "q: quit"
	}

	// Add selection count for browse state
	if m.state == StateBrowse && !m.filterActive {
		if count := m.scroller.getSelectedCount(); count > 0 {
			help = fmt.Sprintf("[%d selected] %s", count, help)
		}
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
