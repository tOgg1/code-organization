package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/template"
)

// Tab represents the currently active tab in the explorer.
type Tab int

const (
	TabBrowse Tab = iota
	TabFiles
	TabOutput
	TabCreate
	TabValidate
)

func (t Tab) String() string {
	switch t {
	case TabBrowse:
		return "Browse"
	case TabFiles:
		return "Files"
	case TabOutput:
		return "Output"
	case TabCreate:
		return "Create"
	case TabValidate:
		return "Validate"
	default:
		return "Unknown"
	}
}

// Pane represents the currently focused pane.
type Pane int

const (
	PaneList Pane = iota
	PaneDetails
)

// explorerKeyMap defines keybindings for the template explorer.
type explorerKeyMap struct {
	NextTab    key.Binding
	PrevTab    key.Binding
	SwitchPane key.Binding
	Open       key.Binding
	Validate   key.Binding
	Quit       key.Binding
}

var explorerKeys = explorerKeyMap{
	NextTab:    key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next tab")),
	PrevTab:    key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev tab")),
	SwitchPane: key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("l/→", "switch pane")),
	Open:       key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open in editor")),
	Validate:   key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "validate")),
	Quit:       key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}

// ExplorerState represents the current state of the explorer.
type ExplorerState int

const (
	StateNormal ExplorerState = iota
	StateVariablePrompt
	StateConfirmCreate
	StateCreating
	StateCreateComplete
)

// CreateFocus represents which element is focused in the Create tab.
type CreateFocus int

const (
	CreateFocusOwner CreateFocus = iota
	CreateFocusProject
	CreateFocusDryRun
	CreateFocusNoHooks
	CreateFocusSubmit
)

// Styles for the template explorer.
var (
	tabStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(lipgloss.Color("241"))

	activeTabStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(lipgloss.Color("212")).
			Bold(true).
			Underline(true)

	tabBarStyle = lipgloss.NewStyle().
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("63")).
			MarginBottom(1)

	// Create tab specific styles
	inputLabelStyle = lipgloss.NewStyle().
			Width(12).
			Foreground(lipgloss.Color("212"))

	inputFocusedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("212"))

	checkboxStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	checkboxFocusedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("212")).
				Bold(true)

	buttonStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Background(lipgloss.Color("63")).
			Foreground(lipgloss.Color("255"))

	buttonFocusedStyle = lipgloss.NewStyle().
				Padding(0, 2).
				Background(lipgloss.Color("212")).
				Foreground(lipgloss.Color("255")).
				Bold(true)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true)
)

// explorerTemplateItem is a list item for the explorer.
type explorerTemplateItem struct {
	listing template.TemplateListing
}

// fileTreeNode represents a node in the file tree.
type fileTreeNode struct {
	Name       string          // display name
	Path       string          // absolute path
	RelPath    string          // relative path from template root
	IsDir      bool            // true if directory
	IsExpanded bool            // true if directory is expanded
	Source     string          // "template", "_global", or source dir name
	Children   []*fileTreeNode // child nodes
	Depth      int             // indentation depth
}

func (i explorerTemplateItem) Title() string { return i.listing.Info.Name }
func (i explorerTemplateItem) Description() string {
	desc := i.listing.Info.Description
	if len(desc) > 40 {
		desc = desc[:37] + "..."
	}
	source := filepath.Base(i.listing.SourceDir)
	return fmt.Sprintf("%s (%d vars, %d repos) • %s", desc, i.listing.Info.VarCount, i.listing.Info.RepoCount, source)
}
func (i explorerTemplateItem) FilterValue() string {
	return i.listing.Info.Name + " " + i.listing.Info.Description + " " + i.listing.SourceDir
}

// TemplateExplorerModel is the main model for the template explorer TUI.
type TemplateExplorerModel struct {
	cfg            *config.Config
	listings       []template.TemplateListing
	globalPaths    []string
	list           list.Model
	activeTab      Tab
	activePane     Pane
	selected       *template.TemplateListing
	width          int
	height         int
	message        string
	messageIsError bool

	// Create tab state
	ownerInput   textinput.Model
	projectInput textinput.Model
	createFocus  CreateFocus
	dryRun       bool
	noHooks      bool
	createError  string

	// Explorer state machine
	state ExplorerState

	// Validate tab state
	validationResults  []validationResult
	validationSelected int
	validating         bool

	// Files tab state
	fileTree            *fileTreeNode   // root of file tree
	flatFileTree        []*fileTreeNode // flattened tree for display
	fileTreeSelected    int             // selected index in flat tree
	filesFocusPane      int             // 0=tree, 1=viewer
	fileViewport        viewport.Model  // viewport for file content
	fileContent         string          // cached file content (raw)
	fileRenderedContent string          // cached rendered content (for templates)
	fileContentPath     string          // path of currently loaded file
	fileContentError    string          // error message for file loading
	fileIsBinary        bool            // true if file is binary
	fileIsLarge         bool            // true if file exceeds size limit
	fileIsTemplate      bool            // true if file is a template (.tmpl etc)
	fileRenderMode      bool            // true = show rendered, false = show raw
	fileSize            int64           // size of current file
	showLineNumbers     bool            // toggle for line numbers

	// Output tab state
	outputMappings     []template.OutputMapping // merged output file list
	outputSelected     int                      // selected index in output list
	outputFocusPane    int                      // 0=list, 1=details
	outputViewport     viewport.Model           // viewport for output details
	outputContent      string                   // cached content of selected file
	outputContentPath  string                   // path of loaded content
	outputContentError string                   // error loading content
	outputShowSource   bool                     // true = show source file, false = rendered output

	// Variable prompting state (embedded for sub-state)
	varPromptVars     []template.TemplateVar
	varPromptBuiltins map[string]string
	varPromptIndex    int
	varPromptValues   map[string]string
	varPromptInput    textinput.Model
	varPromptChoice   list.Model
	varPromptBool     bool
	varPromptMode     inputMode
	varPromptError    string
	loadedTemplate    *template.Template

	// Workspace creation state
	createResult *template.CreateResult
	createErr    error

	createVars map[string]string

	// Diagnostics state
	diagMode         bool                            // true when showing diagnostics overlay
	diagReport       *template.DiagnosticReport      // placeholder scan report
	diagFileDiags    []template.FileDiagnostic       // file pattern diagnostics
	diagSelected     int                             // selected item in diagnostics list
	diagViewport     viewport.Model                  // viewport for diagnostics
	diagShowPatterns bool                            // true = show patterns, false = show placeholders

	// Compare state
	compareMode      bool                       // true when showing compare overlay
	compareMarked    *template.TemplateListing  // template marked for comparison
	compareResult    *template.CompareResult    // comparison result
	compareSelected  int                        // selected item in compare list
	compareSection   int                        // 0=vars, 1=repos, 2=hooks, 3=files
	compareViewport  viewport.Model             // viewport for compare content
}

// NewTemplateExplorer creates a new template explorer model.
func NewTemplateExplorer(cfg *config.Config, listings []template.TemplateListing, globalPaths []string) TemplateExplorerModel {
	items := make([]list.Item, len(listings))
	for i, t := range listings {
		items[i] = explorerTemplateItem{listing: t}
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(lipgloss.Color("212"))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(lipgloss.Color("241"))

	l := list.New(items, delegate, 40, 20)
	l.Title = "Templates"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)

	var selected *template.TemplateListing
	if len(listings) > 0 {
		selected = &listings[0]
	}

	// Initialize owner input
	oi := textinput.New()
	oi.Placeholder = "owner"
	oi.CharLimit = 64
	oi.Width = 30

	// Initialize project input
	pi := textinput.New()
	pi.Placeholder = "project"
	pi.CharLimit = 64
	pi.Width = 30

	// Initialize variable prompt input
	vi := textinput.New()
	vi.Placeholder = "value"
	vi.CharLimit = 256
	vi.Width = 40

	// Initialize file viewer viewport
	vp := viewport.New(40, 20)
	vp.SetContent("")

	// Initialize diagnostics viewport
	dvp := viewport.New(40, 20)
	dvp.SetContent("")

	// Initialize compare viewport
	cvp := viewport.New(40, 20)
	cvp.SetContent("")

	return TemplateExplorerModel{
		cfg:             cfg,
		listings:        listings,
		globalPaths:     globalPaths,
		list:            l,
		activeTab:       TabBrowse,
		activePane:      PaneList,
		selected:        selected,
		ownerInput:      oi,
		projectInput:    pi,
		createFocus:     CreateFocusOwner,
		dryRun:          false,
		noHooks:         false,
		state:           StateNormal,
		varPromptInput:  vi,
		createVars:      make(map[string]string),
		fileViewport:    vp,
		showLineNumbers: true,
		diagViewport:    dvp,
		compareViewport: cvp,
	}
}

// Init implements tea.Model.
func (m TemplateExplorerModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model.
func (m TemplateExplorerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Leave room for tab bar (2 lines) and help (2 lines)
		listHeight := msg.Height - 8
		if listHeight < 5 {
			listHeight = 5
		}
		m.list.SetSize(msg.Width/2-4, listHeight)
		// Initialize/resize file viewport for Files tab
		viewerHeight := listHeight - 4 // Room for header
		viewerWidth := msg.Width/2 - 4
		if viewerHeight < 5 {
			viewerHeight = 5
		}
		if viewerWidth < 20 {
			viewerWidth = 20
		}
		m.fileViewport = viewport.New(viewerWidth, viewerHeight)
		m.fileViewport.SetContent(m.formatFileContent())
		return m, nil

	case tea.KeyMsg:
		// Handle variable prompting state
		if m.state == StateVariablePrompt {
			return m.updateVariablePrompt(msg)
		}

		// Handle confirmation state
		if m.state == StateConfirmCreate {
			return m.updateConfirmCreate(msg)
		}

		// Handle creation in progress - only allow quit
		if m.state == StateCreating {
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			return m, nil
		}

		// Handle creation complete state
		if m.state == StateCreateComplete {
			return m.updateCreateComplete(msg)
		}

		// Handle diagnostics overlay mode
		if m.diagMode {
			return m.updateDiagnosticsOverlay(msg)
		}

		// Handle compare overlay mode
		if m.compareMode {
			return m.updateCompareOverlay(msg)
		}

		// Handle Create tab specially
		if m.activeTab == TabCreate {
			return m.updateCreateTab(msg)
		}

		// Handle Validate tab specially
		if m.activeTab == TabValidate {
			return m.updateValidateTab(msg)
		}

		// Handle Files tab specially
		if m.activeTab == TabFiles {
			return m.updateFilesTab(msg)
		}

		// Handle Output tab specially
		if m.activeTab == TabOutput {
			return m.updateOutputTab(msg)
		}

		// Don't handle keys when filtering
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch {
		case key.Matches(msg, explorerKeys.Quit):
			return m, tea.Quit

		case key.Matches(msg, explorerKeys.NextTab):
			return m.switchTab((m.activeTab + 1) % 5)

		case key.Matches(msg, explorerKeys.PrevTab):
			return m.switchTab((m.activeTab + 4) % 5) // +4 is same as -1 mod 5

		case key.Matches(msg, explorerKeys.SwitchPane):
			if m.activePane == PaneList {
				m.activePane = PaneDetails
			} else {
				m.activePane = PaneList
			}
			return m, nil

		case msg.String() == "h" || msg.String() == "left":
			if m.activePane == PaneDetails {
				m.activePane = PaneList
			}
			return m, nil

		case key.Matches(msg, explorerKeys.Validate):
			if m.selected != nil && m.activeTab == TabBrowse {
				return m, m.validateSelected()
			}

		case key.Matches(msg, explorerKeys.Open):
			if m.selected != nil {
				return m, m.openSelected()
			}

		case msg.String() == "c":
			// Mark template for comparison or compare if one is already marked
			if m.selected != nil && m.activeTab == TabBrowse {
				if m.compareMarked == nil {
					// First template selected - mark it
					m.compareMarked = m.selected
					m.message = fmt.Sprintf("Marked '%s' for comparison. Select another template and press 'c' to compare.", m.selected.Info.Name)
					m.messageIsError = false
				} else if m.compareMarked.Info.Name == m.selected.Info.Name {
					// Same template - unmark it
					m.compareMarked = nil
					m.message = "Comparison cancelled"
					m.messageIsError = false
				} else {
					// Second template selected - start comparison
					return m, m.compareTemplates()
				}
				return m, nil
			}

		// Number keys for quick tab switching
		case msg.String() == "1":
			return m.switchTab(TabBrowse)
		case msg.String() == "2":
			return m.switchTab(TabFiles)
		case msg.String() == "3":
			return m.switchTab(TabCreate)
		case msg.String() == "4":
			return m.switchTab(TabValidate)
		}

	case validationResultMsg:
		if msg.err != nil {
			m.message = fmt.Sprintf("Validation failed for %s: %v", msg.name, msg.err)
			m.messageIsError = true
		} else {
			m.message = fmt.Sprintf("Validated %s successfully", msg.name)
			m.messageIsError = false
		}
		return m, nil

	case openTemplateMsg:
		if msg.err != nil {
			m.message = fmt.Sprintf("Open failed: %v", msg.err)
			m.messageIsError = true
		} else {
			m.message = fmt.Sprintf("Opened %s", msg.path)
			m.messageIsError = false
		}
		return m, nil

	case validateAllResultMsg:
		m.validating = false
		m.validationResults = msg.results
		m.validationSelected = 0
		// Count successes
		valid := 0
		for _, r := range msg.results {
			if r.isValid {
				valid++
			}
		}
		if valid == len(msg.results) {
			m.message = fmt.Sprintf("All %d templates are valid", len(msg.results))
			m.messageIsError = false
		} else {
			m.message = fmt.Sprintf("%d/%d templates have issues", len(msg.results)-valid, len(msg.results))
			m.messageIsError = true
		}
		return m, nil

	case createWorkspaceResultMsg:
		m.createResult = msg.result
		m.createErr = msg.err
		m.state = StateCreateComplete
		return m, nil

	case fileContentMsg:
		// Only update if this is the file we're waiting for
		if msg.path == m.fileContentPath {
			if msg.err != nil {
				m.fileContentError = msg.err.Error()
				m.fileContent = ""
				m.fileRenderedContent = ""
			} else {
				m.fileContent = msg.content
				m.fileRenderedContent = msg.renderedContent
				m.fileContentError = ""
			}
			m.fileIsBinary = msg.isBinary
			m.fileIsLarge = msg.isLarge
			m.fileIsTemplate = msg.isTemplate
			m.fileSize = msg.size
			// Update viewport content
			m.fileViewport.SetContent(m.formatFileContent())
			m.fileViewport.GotoTop()
		}
		return m, nil

	case outputContentMsg:
		m.outputContentPath = msg.path
		if msg.err != nil {
			m.outputContentError = msg.err.Error()
			m.outputContent = ""
		} else {
			m.outputContent = msg.content
			m.outputContentError = ""
			// If showing rendered and we have rendered content, use that
			if !m.outputShowSource && msg.rendered != "" {
				m.outputContent = msg.rendered
			}
		}
		return m, nil

	case diagFileDiagsMsg:
		if msg.err != nil {
			m.message = "Error loading diagnostics: " + msg.err.Error()
			m.messageIsError = true
		} else {
			m.diagFileDiags = msg.diags
			m.diagMode = true
			m.diagSelected = 0
			m.diagViewport.SetContent(m.formatDiagnosticsContent())
		}
		return m, nil

	case diagPlaceholdersMsg:
		if msg.err != nil {
			m.message = "Error loading diagnostics: " + msg.err.Error()
			m.messageIsError = true
		} else {
			m.diagReport = msg.report
			m.diagMode = true
			m.diagSelected = 0
			m.diagViewport.SetContent(m.formatDiagnosticsContent())
		}
		return m, nil

	case compareResultMsg:
		if msg.err != nil {
			m.message = "Error comparing templates: " + msg.err.Error()
			m.messageIsError = true
		} else {
			m.compareResult = msg.result
			m.compareMode = true
			m.compareSelected = 0
			m.compareSection = 0
			m.compareViewport.SetContent(m.formatCompareContent())
		}
		return m, nil
	}

	// Update list and track selection changes
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	if item, ok := m.list.SelectedItem().(explorerTemplateItem); ok {
		m.selected = &item.listing
	}

	return m, cmd
}

// View implements tea.Model.
func (m TemplateExplorerModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Handle variable prompting state
	if m.state == StateVariablePrompt {
		return m.renderVariablePrompt()
	}

	// Handle confirmation state
	if m.state == StateConfirmCreate {
		return m.renderConfirmCreate()
	}

	// Handle creation in progress
	if m.state == StateCreating {
		return m.renderCreating()
	}

	// Handle creation complete
	if m.state == StateCreateComplete {
		return m.renderCreateComplete()
	}

	// Handle diagnostics overlay
	if m.diagMode {
		return m.renderDiagnosticsOverlay()
	}

	// Handle compare overlay
	if m.compareMode {
		return m.renderCompareOverlay()
	}

	// Build tab bar
	tabBar := m.renderTabBar()

	// Build main content based on active tab
	var content string
	switch m.activeTab {
	case TabBrowse:
		content = m.renderBrowseTab()
	case TabFiles:
		content = m.renderFilesTab()
	case TabOutput:
		content = m.renderOutputTab()
	case TabCreate:
		content = m.renderCreateTab()
	case TabValidate:
		content = m.renderValidateTab()
	}

	// Build help line
	help := m.renderHelp()

	return lipgloss.JoinVertical(lipgloss.Left, tabBar, content, help)
}

func (m TemplateExplorerModel) renderTabBar() string {
	tabs := []Tab{TabBrowse, TabFiles, TabOutput, TabCreate, TabValidate}
	var renderedTabs []string

	for i, tab := range tabs {
		style := tabStyle
		if tab == m.activeTab {
			style = activeTabStyle
		}
		label := fmt.Sprintf("%d:%s", i+1, tab.String())
		renderedTabs = append(renderedTabs, style.Render(label))
	}

	tabRow := lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)
	return tabBarStyle.Width(m.width - 2).Render(tabRow)
}

func (m TemplateExplorerModel) renderBrowseTab() string {
	// Left pane: template list
	leftStyle := paneStyle
	rightStyle := paneStyle
	if m.activePane == PaneList {
		leftStyle = activePaneStyle
	} else {
		rightStyle = activePaneStyle
	}

	paneHeight := m.height - 10
	if paneHeight < 5 {
		paneHeight = 5
	}

	// Handle no-templates case
	var leftContent string
	if len(m.listings) == 0 {
		leftContent = m.renderNoTemplatesView()
	} else {
		leftContent = m.list.View()
	}

	leftPane := leftStyle.Width(m.width/2 - 2).Height(paneHeight).Render(leftContent)
	rightPane := rightStyle.Width(m.width/2 - 2).Height(paneHeight).Render(m.templateDetailsView())

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
}

func (m TemplateExplorerModel) renderNoTemplatesView() string {
	var sb strings.Builder

	sb.WriteString(headerStyle.Render("No Templates Found") + "\n\n")
	sb.WriteString("No templates were found in the configured directories.\n\n")
	sb.WriteString("Searched locations:\n")

	for _, dir := range m.cfg.AllTemplatesDirs() {
		sb.WriteString(fmt.Sprintf("  • %s\n", dir))
	}

	sb.WriteString("\n")
	sb.WriteString(helpStyle.Render("To create a template, add a directory with a template.json file"))
	sb.WriteString("\n")
	sb.WriteString(helpStyle.Render("in one of the locations above."))

	return sb.String()
}

func (m TemplateExplorerModel) renderFilesTab() string {
	paneHeight := m.height - 10
	if paneHeight < 5 {
		paneHeight = 5
	}

	if m.selected == nil {
		var sb strings.Builder
		sb.WriteString(headerStyle.Render("Files") + "\n\n")
		sb.WriteString("No template selected.\n")
		sb.WriteString("Select a template in the Browse tab first.\n")
		return paneStyle.Width(m.width - 4).Height(paneHeight).Render(sb.String())
	}

	if len(m.flatFileTree) == 0 {
		var sb strings.Builder
		sb.WriteString(headerStyle.Render("Files") + "\n\n")
		sb.WriteString("No files found in template.\n")
		return paneStyle.Width(m.width - 4).Height(paneHeight).Render(sb.String())
	}

	// Split pane: tree on left, viewer on right
	leftWidth := m.width/2 - 2
	rightWidth := m.width - leftWidth - 6
	if leftWidth < 20 {
		leftWidth = 20
	}
	if rightWidth < 20 {
		rightWidth = 20
	}

	// Render tree pane (left)
	leftPane := m.renderFileTree(leftWidth, paneHeight)

	// Render viewer pane (right)
	rightPane := m.renderFileViewer(rightWidth, paneHeight)

	// Style for active/inactive panes
	activeBorder := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("212"))
	inactiveBorder := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("241"))

	var leftStyled, rightStyled string
	if m.filesFocusPane == 0 {
		leftStyled = activeBorder.Width(leftWidth).Height(paneHeight).Render(leftPane)
		rightStyled = inactiveBorder.Width(rightWidth).Height(paneHeight).Render(rightPane)
	} else {
		leftStyled = inactiveBorder.Width(leftWidth).Height(paneHeight).Render(leftPane)
		rightStyled = activeBorder.Width(rightWidth).Height(paneHeight).Render(rightPane)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, leftStyled, rightStyled)
}

// renderFileTree renders the file tree for the left pane.
func (m TemplateExplorerModel) renderFileTree(width, height int) string {
	var sb strings.Builder
	sb.WriteString(headerStyle.Render("Files") + "\n\n")

	// Calculate how many lines we can show (leave room for header)
	maxLines := height - 4
	if maxLines < 3 {
		maxLines = 3
	}

	// Calculate visible window
	startIdx := 0
	if m.fileTreeSelected >= maxLines {
		startIdx = m.fileTreeSelected - maxLines + 1
	}
	endIdx := startIdx + maxLines
	if endIdx > len(m.flatFileTree) {
		endIdx = len(m.flatFileTree)
	}

	// Render visible nodes
	for i := startIdx; i < endIdx; i++ {
		node := m.flatFileTree[i]

		// Indentation
		indent := strings.Repeat("  ", node.Depth)

		// Icon
		icon := "📄"
		if node.IsDir {
			if node.IsExpanded {
				icon = "📂"
			} else {
				icon = "📁"
			}
		}

		// Source badge
		sourceBadge := ""
		if node.Source == "_global" {
			sourceBadge = " [g]"
		}

		// Truncate name if too long
		maxNameLen := width - node.Depth*2 - 6 - len(sourceBadge)
		name := node.Name
		if maxNameLen > 0 && len(name) > maxNameLen {
			name = name[:maxNameLen-1] + "…"
		}

		// Selection marker
		line := fmt.Sprintf("%s%s %s%s", indent, icon, name, sourceBadge)

		if i == m.fileTreeSelected {
			line = lipgloss.NewStyle().
				Background(lipgloss.Color("236")).
				Foreground(lipgloss.Color("212")).
				Bold(true).
				Render(line)
		}

		sb.WriteString(line + "\n")
	}

	// Scrollbar indicator
	if len(m.flatFileTree) > maxLines {
		sb.WriteString(fmt.Sprintf("\n(%d/%d)", m.fileTreeSelected+1, len(m.flatFileTree)))
	}

	return sb.String()
}

// renderFileViewer renders the file content viewer for the right pane.
func (m TemplateExplorerModel) renderFileViewer(width, height int) string {
	var sb strings.Builder

	// Header with file name or placeholder
	if m.fileContentPath != "" {
		name := filepath.Base(m.fileContentPath)
		header := fmt.Sprintf("Viewer: %s", name)
		if m.fileSize > 0 {
			header += fmt.Sprintf(" (%s)", humanizeFileSize(m.fileSize))
		}
		// Show RAW/RENDERED indicator for template files
		if m.fileIsTemplate {
			if m.fileRenderMode {
				header += " [RENDERED]"
			} else {
				header += " [RAW]"
			}
		}
		sb.WriteString(headerStyle.Render(header) + "\n\n")
	} else {
		sb.WriteString(headerStyle.Render("Viewer") + "\n\n")
	}

	// Show viewport content
	sb.WriteString(m.fileViewport.View())

	// Show scroll position if content is scrollable
	if m.fileViewport.TotalLineCount() > m.fileViewport.VisibleLineCount() {
		percent := int(m.fileViewport.ScrollPercent() * 100)
		sb.WriteString(fmt.Sprintf("\n\n%d%%", percent))
	}

	return sb.String()
}

func (m TemplateExplorerModel) renderCreateTab() string {
	paneHeight := m.height - 10
	if paneHeight < 5 {
		paneHeight = 5
	}

	var sb strings.Builder

	sb.WriteString(headerStyle.Render("Create Workspace") + "\n\n")

	if m.selected == nil {
		sb.WriteString("No template selected.\n")
		sb.WriteString("Select a template in the Browse tab first.")
		return paneStyle.Width(m.width - 4).Height(paneHeight).Render(sb.String())
	}

	// Show selected template
	sb.WriteString(fmt.Sprintf("Template: %s\n", titleStyle.Render(m.selected.Info.Name)))
	sb.WriteString(fmt.Sprintf("Source:   %s\n\n", filepath.Base(m.selected.SourceDir)))

	// Owner input
	ownerLabel := inputLabelStyle.Render("Owner:")
	if m.createFocus == CreateFocusOwner {
		ownerLabel = inputFocusedStyle.Render("▶ Owner:")
	}
	sb.WriteString(fmt.Sprintf("%s %s\n", ownerLabel, m.ownerInput.View()))

	// Project input
	projectLabel := inputLabelStyle.Render("Project:")
	if m.createFocus == CreateFocusProject {
		projectLabel = inputFocusedStyle.Render("▶ Project:")
	}
	sb.WriteString(fmt.Sprintf("%s %s\n\n", projectLabel, m.projectInput.View()))

	// Dry-run checkbox
	dryRunCheck := "[ ]"
	if m.dryRun {
		dryRunCheck = "[✓]"
	}
	dryRunStyle := checkboxStyle
	if m.createFocus == CreateFocusDryRun {
		dryRunStyle = checkboxFocusedStyle
		dryRunCheck = "▶ " + dryRunCheck
	} else {
		dryRunCheck = "  " + dryRunCheck
	}
	sb.WriteString(dryRunStyle.Render(dryRunCheck+" Dry-run (preview changes without creating)") + "\n")

	// No-hooks checkbox
	noHooksCheck := "[ ]"
	if m.noHooks {
		noHooksCheck = "[✓]"
	}
	noHooksStyle := checkboxStyle
	if m.createFocus == CreateFocusNoHooks {
		noHooksStyle = checkboxFocusedStyle
		noHooksCheck = "▶ " + noHooksCheck
	} else {
		noHooksCheck = "  " + noHooksCheck
	}
	sb.WriteString(noHooksStyle.Render(noHooksCheck+" Skip hooks (don't run post-create scripts)") + "\n\n")

	if len(m.createVars) > 0 && m.state != StateVariablePrompt {
		sb.WriteString(promptHintStyle.Render(fmt.Sprintf("Captured variables: %d\n\n", len(m.createVars))))
	}

	// Submit button
	submitText := "Create Workspace"
	submitStyle := buttonStyle
	if m.createFocus == CreateFocusSubmit {
		submitStyle = buttonFocusedStyle
		submitText = "▶ " + submitText
	}
	sb.WriteString(submitStyle.Render(submitText) + "\n")

	// Show error if any
	if m.createError != "" {
		sb.WriteString("\n" + promptErrorStyle.Render("Error: "+m.createError))
	}

	// Show workspace slug preview
	owner := strings.ToLower(strings.TrimSpace(m.ownerInput.Value()))
	project := strings.ToLower(strings.TrimSpace(m.projectInput.Value()))
	if owner != "" || project != "" {
		slug := owner + "--" + project
		if owner == "" {
			slug = "<owner>--" + project
		}
		if project == "" {
			slug = owner + "--<project>"
		}
		sb.WriteString("\n" + helpStyle.Render(fmt.Sprintf("Workspace slug: %s", slug)))
	}

	return paneStyle.Width(m.width - 4).Height(paneHeight).Render(sb.String())
}

func (m TemplateExplorerModel) renderOutputTab() string {
	paneHeight := m.height - 10
	if paneHeight < 5 {
		paneHeight = 5
	}

	// Two-pane layout: output list on left, details on right
	leftStyle := paneStyle
	rightStyle := paneStyle
	if m.outputFocusPane == 0 {
		leftStyle = activePaneStyle
	} else {
		rightStyle = activePaneStyle
	}

	leftPane := leftStyle.Width(m.width/2 - 2).Height(paneHeight).Render(m.renderOutputList())
	rightPane := rightStyle.Width(m.width/2 - 2).Height(paneHeight).Render(m.renderOutputDetails())

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
}

func (m TemplateExplorerModel) renderOutputList() string {
	var sb strings.Builder

	sb.WriteString(headerStyle.Render("Output Files") + "\n\n")

	if m.selected == nil {
		sb.WriteString("No template selected.\n")
		sb.WriteString("Select a template in the Browse tab first.")
		return sb.String()
	}

	if len(m.outputMappings) == 0 {
		sb.WriteString("No output files.\n")
		sb.WriteString("This template has no global or template files configured.")
		return sb.String()
	}

	// Calculate visible area
	maxVisible := m.height - 14
	if maxVisible < 5 {
		maxVisible = 5
	}

	// Calculate scroll window
	start := 0
	if m.outputSelected >= maxVisible {
		start = m.outputSelected - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(m.outputMappings) {
		end = len(m.outputMappings)
	}

	// Show scroll indicator if needed
	if start > 0 {
		sb.WriteString(helpStyle.Render("  ↑ more above") + "\n")
	}

	for i := start; i < end; i++ {
		mapping := m.outputMappings[i]
		prefix := "  "
		style := lipgloss.NewStyle()
		if i == m.outputSelected {
			prefix = "▶ "
			style = selectedStyle
		}

		// Build the display line with origin indicator
		originBadge := ""
		switch mapping.OriginType {
		case template.OriginGlobal:
			originBadge = "[G]"
		case template.OriginTemplate:
			originBadge = "[T]"
		}

		overrideBadge := ""
		if mapping.IsOverride {
			overrideBadge = " ⚡" // Override indicator
		}

		line := fmt.Sprintf("%s%s %s%s", prefix, originBadge, mapping.OutputPath, overrideBadge)
		sb.WriteString(style.Render(line) + "\n")
	}

	// Show scroll indicator if needed
	if end < len(m.outputMappings) {
		sb.WriteString(helpStyle.Render("  ↓ more below") + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString(helpStyle.Render(fmt.Sprintf("%d files • [G]=Global [T]=Template ⚡=Override", len(m.outputMappings))))

	return sb.String()
}

func (m TemplateExplorerModel) renderOutputDetails() string {
	var sb strings.Builder

	sb.WriteString(headerStyle.Render("Details") + "\n\n")

	if m.selected == nil || len(m.outputMappings) == 0 {
		sb.WriteString("Select an output file to view details.")
		return sb.String()
	}

	if m.outputSelected >= len(m.outputMappings) {
		return sb.String()
	}

	mapping := m.outputMappings[m.outputSelected]

	// Show mapping details
	sb.WriteString(titleStyle.Render("Output: "+mapping.OutputPath) + "\n\n")

	// Origin info
	originLabel := "Global"
	if mapping.OriginType == template.OriginTemplate {
		originLabel = "Template"
	}
	sb.WriteString(fmt.Sprintf("Origin:   %s\n", originLabel))
	sb.WriteString(fmt.Sprintf("Source:   %s\n", mapping.SourceRel))

	if mapping.IsTemplate {
		sb.WriteString("Type:     Template file (.tmpl)\n")
	} else {
		sb.WriteString("Type:     Static file\n")
	}

	if mapping.IsOverride {
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("⚡ Overrides global file") + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString(helpStyle.Render("Press 'enter' to view source file"))

	return sb.String()
}

func (m TemplateExplorerModel) renderValidateTab() string {
	paneHeight := m.height - 10
	if paneHeight < 5 {
		paneHeight = 5
	}

	// Two-pane layout: results list on left, detail on right
	leftStyle := paneStyle
	rightStyle := paneStyle
	if m.activePane == PaneList {
		leftStyle = activePaneStyle
	} else {
		rightStyle = activePaneStyle
	}

	leftPane := leftStyle.Width(m.width/2 - 2).Height(paneHeight).Render(m.renderValidationResults())
	rightPane := rightStyle.Width(m.width/2 - 2).Height(paneHeight).Render(m.renderValidationDetail())

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
}

func (m TemplateExplorerModel) renderValidationResults() string {
	var sb strings.Builder

	sb.WriteString(headerStyle.Render("Validation Results") + "\n\n")

	if m.validating {
		sb.WriteString("Validating templates...\n")
		return sb.String()
	}

	if len(m.validationResults) == 0 {
		sb.WriteString("No validation results yet.\n\n")
		sb.WriteString("Press 'v' to validate selected template\n")
		sb.WriteString("Press 'V' to validate all templates\n")
		return sb.String()
	}

	// Show results list
	for i, r := range m.validationResults {
		prefix := "  "
		style := lipgloss.NewStyle()
		if i == m.validationSelected {
			prefix = "▶ "
			style = style.Bold(true).Foreground(lipgloss.Color("212"))
		}

		icon := "✓"
		iconStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82")) // green
		if !r.isValid {
			icon = "✗"
			iconStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // red
		}

		source := filepath.Base(r.sourceDir)
		line := fmt.Sprintf("%s%s %s (%s)", prefix, iconStyle.Render(icon), r.name, source)
		sb.WriteString(style.Render(line) + "\n")
	}

	return sb.String()
}

func (m TemplateExplorerModel) renderValidationDetail() string {
	var sb strings.Builder

	sb.WriteString(headerStyle.Render("Details") + "\n\n")

	if len(m.validationResults) == 0 {
		sb.WriteString("Select a validation result to see details.\n")
		return sb.String()
	}

	if m.validationSelected >= len(m.validationResults) {
		return sb.String()
	}

	result := m.validationResults[m.validationSelected]

	sb.WriteString(fmt.Sprintf("Template:   %s\n", result.name))
	sb.WriteString(fmt.Sprintf("Source dir: %s\n\n", result.sourceDir))

	if result.isValid {
		validStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Bold(true)
		sb.WriteString(validStyle.Render("✓ Valid") + "\n\n")
		sb.WriteString("No issues found.\n")
	} else {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
		sb.WriteString(errorStyle.Render("✗ Invalid") + "\n\n")
		sb.WriteString("Errors:\n")
		// Format error message nicely
		errMsg := result.err.Error()
		wrapped := lipgloss.NewStyle().Width(m.width/2 - 4).Render(errMsg)
		sb.WriteString(promptErrorStyle.Render(wrapped) + "\n")
	}

	return sb.String()
}

func (m TemplateExplorerModel) templateDetailsView() string {
	if m.selected == nil {
		return "No template selected"
	}

	info := m.selected.Info
	var sb strings.Builder

	sb.WriteString(titleStyle.Render(info.Name) + "\n\n")
	sb.WriteString(fmt.Sprintf("Description: %s\n\n", info.Description))
	sb.WriteString(fmt.Sprintf("Variables:   %d\n", info.VarCount))
	sb.WriteString(fmt.Sprintf("Repos:       %d\n", info.RepoCount))
	sb.WriteString(fmt.Sprintf("Hooks:       %d\n", info.HookCount))
	sb.WriteString(fmt.Sprintf("Source dir:  %s\n", m.selected.SourceDir))
	sb.WriteString(fmt.Sprintf("Path:        %s\n", m.selected.TemplatePath))

	sb.WriteString("\n")
	sb.WriteString(helpStyle.Render("Press 'o' to open in editor"))
	sb.WriteString("\n")
	sb.WriteString(helpStyle.Render("Press 'v' to validate"))

	return sb.String()
}

func (m TemplateExplorerModel) renderHelp() string {
	var help string
	switch m.activeTab {
	case TabBrowse:
		help = "j/k: navigate • tab: next tab • 1-4: jump to tab • h/l: switch pane • /: filter • o: open • v: validate • c: compare • q: quit"
	case TabFiles:
		if m.filesFocusPane == 0 {
			help = "j/k: navigate • enter: expand/view • l: expand/viewer • h: collapse • d: patterns • D: placeholders • tab: pane • q: quit"
		} else {
			help = "j/k: scroll • d/u: page • g/G: top/bottom • h: back to tree • r: toggle render • d: patterns • D: placeholders • tab: pane • q: quit"
		}
	case TabOutput:
		if m.outputFocusPane == 0 {
			help = "j/k: navigate • l: view details • enter: open source • tab: next tab • q: quit"
		} else {
			help = "j/k: scroll • d/u: page • g/G: top/bottom • h: back to list • s: toggle source/rendered • tab: next tab • q: quit"
		}
	case TabCreate:
		help = "tab/↓: next field • shift+tab/↑: prev field • space: toggle • enter: proceed • esc: back • q: quit"
	case TabValidate:
		help = "j/k: navigate • h/l: pane • v: validate selected • V: validate all • tab: next tab • q: quit"
	}

	if m.message != "" {
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
		if m.messageIsError {
			style = promptErrorStyle
		}
		return style.Render(m.message) + "\n" + helpStyle.Render(help)
	}

	return helpStyle.Render(help)
}

// switchTab handles tab switching and focus management.
func (m TemplateExplorerModel) switchTab(newTab Tab) (tea.Model, tea.Cmd) {
	oldTab := m.activeTab
	m.activeTab = newTab
	m.message = ""

	// When leaving Create tab, blur inputs
	if oldTab == TabCreate {
		m.ownerInput.Blur()
		m.projectInput.Blur()
	}

	// When entering Create tab, focus the appropriate input
	if newTab == TabCreate {
		return m, m.focusCreateInput()
	}

	// When entering Files tab, build the file tree
	if newTab == TabFiles {
		m.buildFileTree()
		m.fileTreeSelected = 0
	}

	// When entering Output tab, build output mappings
	if newTab == TabOutput {
		m.buildOutputMappings()
		m.outputSelected = 0
	}

	return m, nil
}

// focusCreateInput returns a command to focus the current Create tab input.
func (m TemplateExplorerModel) focusCreateInput() tea.Cmd {
	m.ownerInput.Blur()
	m.projectInput.Blur()

	switch m.createFocus {
	case CreateFocusOwner:
		return m.ownerInput.Focus()
	case CreateFocusProject:
		return m.projectInput.Focus()
	default:
		return nil
	}
}

// updateCreateTab handles key events for the Create tab.
func (m TemplateExplorerModel) updateCreateTab(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Clear error on any key press
	if m.createError != "" {
		m.createError = ""
	}

	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "q":
		// Only quit if not in a text input
		if m.createFocus != CreateFocusOwner && m.createFocus != CreateFocusProject {
			return m, tea.Quit
		}

	case "esc":
		// Go back to Browse tab
		return m.switchTab(TabBrowse)

	case "tab", "down":
		return m.nextCreateFocus()

	case "shift+tab", "up":
		return m.prevCreateFocus()

	case " ":
		// Toggle checkboxes
		switch m.createFocus {
		case CreateFocusDryRun:
			m.dryRun = !m.dryRun
			return m, nil
		case CreateFocusNoHooks:
			m.noHooks = !m.noHooks
			return m, nil
		}

	case "enter":
		// Submit or toggle based on focus
		switch m.createFocus {
		case CreateFocusDryRun:
			m.dryRun = !m.dryRun
			return m, nil
		case CreateFocusNoHooks:
			m.noHooks = !m.noHooks
			return m, nil
		case CreateFocusSubmit:
			return m.validateAndProceed()
		case CreateFocusOwner, CreateFocusProject:
			// Move to next field on enter in text inputs
			return m.nextCreateFocus()
		}

	// Number keys for quick tab switching (only when not in text input)
	case "1", "2", "3", "4":
		if m.createFocus != CreateFocusOwner && m.createFocus != CreateFocusProject {
			tabNum := int(msg.String()[0] - '1')
			return m.switchTab(Tab(tabNum))
		}
	}

	// Update the focused text input
	var cmd tea.Cmd
	switch m.createFocus {
	case CreateFocusOwner:
		m.ownerInput, cmd = m.ownerInput.Update(msg)
	case CreateFocusProject:
		m.projectInput, cmd = m.projectInput.Update(msg)
	}

	return m, cmd
}

// nextCreateFocus moves focus to the next element in the Create tab.
func (m TemplateExplorerModel) nextCreateFocus() (tea.Model, tea.Cmd) {
	m.ownerInput.Blur()
	m.projectInput.Blur()

	m.createFocus = (m.createFocus + 1) % 5

	switch m.createFocus {
	case CreateFocusOwner:
		return m, m.ownerInput.Focus()
	case CreateFocusProject:
		return m, m.projectInput.Focus()
	default:
		return m, nil
	}
}

// prevCreateFocus moves focus to the previous element in the Create tab.
func (m TemplateExplorerModel) prevCreateFocus() (tea.Model, tea.Cmd) {
	m.ownerInput.Blur()
	m.projectInput.Blur()

	m.createFocus = (m.createFocus + 4) % 5 // +4 is same as -1 mod 5

	switch m.createFocus {
	case CreateFocusOwner:
		return m, m.ownerInput.Focus()
	case CreateFocusProject:
		return m, m.projectInput.Focus()
	default:
		return m, nil
	}
}

// validateAndProceed validates inputs and proceeds to variable prompting.
func (m TemplateExplorerModel) validateAndProceed() (tea.Model, tea.Cmd) {
	owner := strings.ToLower(strings.TrimSpace(m.ownerInput.Value()))
	project := strings.ToLower(strings.TrimSpace(m.projectInput.Value()))

	// Validate owner
	if owner == "" {
		m.createError = "Owner is required"
		m.createFocus = CreateFocusOwner
		return m, m.ownerInput.Focus()
	}
	if !isValidSlugPart(owner) {
		m.createError = "Owner must be lowercase alphanumeric with hyphens only"
		m.createFocus = CreateFocusOwner
		return m, m.ownerInput.Focus()
	}

	// Validate project
	if project == "" {
		m.createError = "Project is required"
		m.createFocus = CreateFocusProject
		return m, m.projectInput.Focus()
	}
	if !isValidSlugPart(project) {
		m.createError = "Project must be lowercase alphanumeric with hyphens only"
		m.createFocus = CreateFocusProject
		return m, m.projectInput.Focus()
	}

	// Ensure we have a template selected
	if m.selected == nil {
		m.createError = "No template selected"
		return m, nil
	}

	// Load the full template using multi-dir lookup
	tmpl, _, err := template.LoadTemplateMulti(m.cfg.AllTemplatesDirs(), m.selected.Info.Name)
	if err != nil {
		m.createError = fmt.Sprintf("Failed to load template: %v", err)
		return m, nil
	}
	m.loadedTemplate = tmpl

	// Compute builtin variables
	slug := owner + "--" + project
	workspacePath := filepath.Join(m.cfg.CodeRoot, slug)
	builtins := template.GetBuiltinVariables(owner, project, workspacePath, m.cfg.CodeRoot)

	// Seed values with builtins and any previously captured vars
	values := copyStringMap(m.createVars)
	for k, v := range builtins {
		values[k] = v
	}

	// Apply defaults for any variables not yet set
	values = m.applyDefaults(tmpl.Variables, values)

	// Determine which required variables still need prompting (no value after defaults/builtins)
	promptVars := make([]template.TemplateVar, 0)
	for _, v := range tmpl.Variables {
		if _, ok := values[v.Name]; ok {
			continue
		}
		if !v.Required {
			continue
		}
		promptVars = append(promptVars, v)
	}

	// If no variables need prompting, proceed directly
	if len(promptVars) == 0 {
		m.createVars = values
		m.message = fmt.Sprintf("Variables captured: %d (builtins/defaults)", len(values))
		m.messageIsError = false
		return m, nil
	}

	// Initialize variable prompting state
	m.varPromptVars = promptVars
	m.varPromptBuiltins = builtins
	m.varPromptIndex = 0
	m.varPromptValues = values
	m.varPromptError = ""
	m.state = StateVariablePrompt

	// Setup the first variable
	m.setupCurrentVariable()

	return m, m.varPromptInput.Focus()
}

func (m TemplateExplorerModel) validateSelected() tea.Cmd {
	return func() tea.Msg {
		if m.selected == nil {
			return validationResultMsg{err: fmt.Errorf("no template selected")}
		}
		err := template.ValidateTemplateDir(m.selected.SourceDir, m.selected.Info.Name)
		return validationResultMsg{name: m.selected.Info.Name, err: err}
	}
}

func (m TemplateExplorerModel) openSelected() tea.Cmd {
	return func() tea.Msg {
		if m.selected == nil {
			return nil
		}
		templatePath := m.selected.TemplatePath
		var cmd *exec.Cmd
		if m.cfg.Editor != "" {
			cmd = exec.Command(m.cfg.Editor, templatePath)
		} else if runtime.GOOS == "darwin" {
			cmd = exec.Command("open", templatePath)
		} else {
			cmd = exec.Command("xdg-open", templatePath)
		}
		return tea.ExecProcess(cmd, func(err error) tea.Msg {
			return openTemplateMsg{path: templatePath, err: err}
		})
	}
}

// updateValidateTab handles key events for the Validate tab.
func (m TemplateExplorerModel) updateValidateTab(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "tab":
		return m.switchTab((m.activeTab + 1) % 5)
	case "shift+tab":
		return m.switchTab((m.activeTab + 4) % 5)
	case "1", "2", "3", "4":
		tabNum := int(msg.String()[0] - '1')
		return m.switchTab(Tab(tabNum))

	// Navigation between list and detail panes
	case "h", "left":
		m.activePane = PaneList
		return m, nil
	case "l", "right":
		m.activePane = PaneDetails
		return m, nil

	// Navigate validation results
	case "j", "down":
		if len(m.validationResults) > 0 {
			m.validationSelected = (m.validationSelected + 1) % len(m.validationResults)
		}
		return m, nil
	case "k", "up":
		if len(m.validationResults) > 0 {
			m.validationSelected = (m.validationSelected - 1 + len(m.validationResults)) % len(m.validationResults)
		}
		return m, nil

	// Validation actions
	case "v":
		// Validate selected template
		if m.selected != nil {
			m.validating = true
			return m, m.validateSelectedForTab()
		}
		return m, nil
	case "V":
		// Validate all templates
		m.validating = true
		return m, m.validateAllTemplates()
	}
	return m, nil
}

// validateSelectedForTab validates the selected template and updates the Validate tab results.
func (m TemplateExplorerModel) validateSelectedForTab() tea.Cmd {
	return func() tea.Msg {
		if m.selected == nil {
			return validateAllResultMsg{results: nil}
		}

		err := template.ValidateTemplateDir(m.selected.SourceDir, m.selected.Info.Name)
		result := validationResult{
			name:      m.selected.Info.Name,
			sourceDir: m.selected.SourceDir,
			err:       err,
			isValid:   err == nil,
		}
		return validateAllResultMsg{results: []validationResult{result}}
	}
}

// validateAllTemplates validates all templates and returns results.
func (m TemplateExplorerModel) validateAllTemplates() tea.Cmd {
	return func() tea.Msg {
		results := make([]validationResult, len(m.listings))
		for i, listing := range m.listings {
			err := template.ValidateTemplateDir(listing.SourceDir, listing.Info.Name)
			results[i] = validationResult{
				name:      listing.Info.Name,
				sourceDir: listing.SourceDir,
				err:       err,
				isValid:   err == nil,
			}
		}
		return validateAllResultMsg{results: results}
	}
}

// validationResult represents the result of validating a single template.
type validationResult struct {
	name      string
	sourceDir string
	err       error
	isValid   bool
}

// Message types for async operations.
type validationResultMsg struct {
	name string
	err  error
}

type validateAllResultMsg struct {
	results []validationResult
}

type openTemplateMsg struct {
	path string
	err  error
}

type createWorkspaceResultMsg struct {
	result *template.CreateResult
	err    error
}

// fileContentMsg is sent when file content is loaded.
type fileContentMsg struct {
	path            string
	content         string
	renderedContent string // rendered content for template files
	size            int64
	isBinary        bool
	isLarge         bool
	isTemplate      bool // true if file has template extension
	err             error
}

// outputContentMsg is sent when output file content is loaded.
type outputContentMsg struct {
	path     string
	content  string
	rendered string
	err      error
}

// diagFileDiagsMsg is sent when file pattern diagnostics are loaded.
type diagFileDiagsMsg struct {
	diags []template.FileDiagnostic
	err   error
}

// diagPlaceholdersMsg is sent when placeholder diagnostics are loaded.
type diagPlaceholdersMsg struct {
	report *template.DiagnosticReport
	err    error
}

// compareResultMsg is sent when template comparison is complete.
type compareResultMsg struct {
	result *template.CompareResult
	err    error
}

// maxFileViewerSize is the maximum file size to display in the viewer (1MB).
const maxFileViewerSize = 1024 * 1024

// formatFileContent formats the file content for display in the viewport.
func (m TemplateExplorerModel) formatFileContent() string {
	if m.fileContentPath == "" {
		return "Select a file to view its contents.\n\nUse Tab to switch focus to the viewer."
	}

	if m.fileContentError != "" {
		return fmt.Sprintf("Error loading file:\n%s", m.fileContentError)
	}

	if m.fileIsBinary {
		return fmt.Sprintf("Binary file (%s)\n\nCannot display binary content.", humanizeFileSize(m.fileSize))
	}

	if m.fileIsLarge {
		return fmt.Sprintf("File too large to display (%s)\n\nMaximum viewable size: %s", humanizeFileSize(m.fileSize), humanizeFileSize(maxFileViewerSize))
	}

	if m.fileContent == "" {
		return "(empty file)"
	}

	// Choose content based on render mode
	content := m.fileContent
	if m.fileRenderMode && m.fileIsTemplate {
		content = m.fileRenderedContent
		if content == "" {
			content = "(no rendered content - press 'r' to render)"
		}
	}

	if !m.showLineNumbers {
		return content
	}

	// Add line numbers
	lines := strings.Split(content, "\n")
	width := len(fmt.Sprintf("%d", len(lines)))
	var sb strings.Builder
	for i, line := range lines {
		sb.WriteString(fmt.Sprintf("%*d │ %s\n", width, i+1, line))
	}
	return strings.TrimSuffix(sb.String(), "\n")
}

// humanizeFileSize formats a file size in a human-readable way.
func humanizeFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// copyStringMap creates a shallow copy of a string map.
func copyStringMap(m map[string]string) map[string]string {
	result := make(map[string]string, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

// applyDefaults applies default values for variables that don't have values yet.
func (m TemplateExplorerModel) applyDefaults(vars []template.TemplateVar, values map[string]string) map[string]string {
	for _, v := range vars {
		if _, ok := values[v.Name]; ok {
			continue
		}
		if v.Default != nil {
			defaultVal := fmt.Sprintf("%v", v.Default)
			// Substitute any variable references in default
			if substituted, err := template.SubstituteVariables(defaultVal, values); err == nil {
				values[v.Name] = substituted
			} else {
				values[v.Name] = defaultVal
			}
		}
	}
	return values
}

// setupCurrentVariable sets up the input for the current variable being prompted.
func (m *TemplateExplorerModel) setupCurrentVariable() {
	if m.varPromptIndex >= len(m.varPromptVars) {
		return
	}

	v := m.varPromptVars[m.varPromptIndex]

	switch v.Type {
	case template.VarTypeChoice:
		m.varPromptMode = modeChoice
		items := make([]list.Item, len(v.Choices))
		for i, c := range v.Choices {
			items[i] = choiceItem{value: c}
		}
		delegate := list.NewDefaultDelegate()
		delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(lipgloss.Color("212"))
		m.varPromptChoice = list.New(items, delegate, 40, 10)
		m.varPromptChoice.Title = v.Name
		m.varPromptChoice.SetShowStatusBar(false)
		m.varPromptChoice.SetShowHelp(false)
		m.varPromptChoice.SetFilteringEnabled(false)

	case template.VarTypeBoolean:
		m.varPromptMode = modeBoolean
		m.varPromptBool = false

	default:
		m.varPromptMode = modeText
		m.varPromptInput.Reset()
		m.varPromptInput.Placeholder = v.Name
		if v.Default != nil {
			m.varPromptInput.SetValue(fmt.Sprintf("%v", v.Default))
		}
	}
}

// updateVariablePrompt handles key events during variable prompting.
func (m TemplateExplorerModel) updateVariablePrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "esc":
		m.state = StateNormal
		m.createVars = make(map[string]string)
		return m, nil

	case "enter":
		return m.submitCurrentVariable()
	}

	// Update the appropriate input based on mode
	var cmd tea.Cmd
	switch m.varPromptMode {
	case modeText:
		m.varPromptInput, cmd = m.varPromptInput.Update(msg)
	case modeChoice:
		m.varPromptChoice, cmd = m.varPromptChoice.Update(msg)
	case modeBoolean:
		switch msg.String() {
		case "j", "k", "up", "down", " ":
			m.varPromptBool = !m.varPromptBool
		}
	}

	return m, cmd
}

// submitCurrentVariable validates and stores the current variable value.
func (m TemplateExplorerModel) submitCurrentVariable() (tea.Model, tea.Cmd) {
	v := m.varPromptVars[m.varPromptIndex]
	var value string

	switch m.varPromptMode {
	case modeText:
		value = m.varPromptInput.Value()
	case modeChoice:
		if item, ok := m.varPromptChoice.SelectedItem().(choiceItem); ok {
			value = item.value
		}
	case modeBoolean:
		if m.varPromptBool {
			value = "true"
		} else {
			value = "false"
		}
	}

	// Validate value
	if err := template.ValidateVarValue(v, value); err != nil {
		m.varPromptError = err.Error()
		return m, nil
	}

	// Store value
	m.varPromptValues[v.Name] = value
	m.varPromptError = ""

	// Move to next variable
	m.varPromptIndex++
	if m.varPromptIndex >= len(m.varPromptVars) {
		// All variables collected, proceed to confirmation
		m.createVars = m.varPromptValues
		m.state = StateConfirmCreate
		return m, nil
	}

	// Setup next variable
	m.setupCurrentVariable()
	if m.varPromptMode == modeText {
		return m, m.varPromptInput.Focus()
	}
	return m, nil
}

// updateConfirmCreate handles key events during creation confirmation.
func (m TemplateExplorerModel) updateConfirmCreate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "n":
		m.state = StateNormal
		return m, nil
	case "enter", "y":
		return m.startCreation()
	}
	return m, nil
}

// startCreation initiates workspace creation.
func (m TemplateExplorerModel) startCreation() (tea.Model, tea.Cmd) {
	m.state = StateCreating

	return m, func() tea.Msg {
		owner := strings.ToLower(strings.TrimSpace(m.ownerInput.Value()))
		project := strings.ToLower(strings.TrimSpace(m.projectInput.Value()))

		opts := template.CreateOptions{
			TemplateName: m.selected.Info.Name,
			Variables:    m.createVars,
			NoHooks:      m.noHooks,
			DryRun:       m.dryRun,
			Verbose:      false,
		}

		result, err := template.CreateWorkspace(m.cfg, owner, project, opts)
		return createWorkspaceResultMsg{result: result, err: err}
	}
}

// updateCreateComplete handles key events after creation is complete.
func (m TemplateExplorerModel) updateCreateComplete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "enter", "esc":
		// Reset state for another creation
		m.state = StateNormal
		m.createResult = nil
		m.createErr = nil
		m.createVars = make(map[string]string)
		m.ownerInput.Reset()
		m.projectInput.Reset()
		m.createFocus = CreateFocusOwner
		return m, m.ownerInput.Focus()
	case "o":
		// Open created workspace
		if m.createResult != nil && m.createErr == nil {
			return m, m.openWorkspace(m.createResult.WorkspacePath)
		}
	}
	return m, nil
}

// openWorkspace opens the workspace directory in the configured editor.
func (m TemplateExplorerModel) openWorkspace(path string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		if m.cfg.Editor != "" {
			cmd = exec.Command(m.cfg.Editor, path)
		} else if runtime.GOOS == "darwin" {
			cmd = exec.Command("open", path)
		} else {
			cmd = exec.Command("xdg-open", path)
		}
		err := cmd.Run()
		return openTemplateMsg{path: path, err: err}
	}
}

// renderVariablePrompt renders the variable prompting UI.
func (m TemplateExplorerModel) renderVariablePrompt() string {
	var sb strings.Builder

	sb.WriteString(headerStyle.Render("Configure Variables") + "\n\n")

	// Progress indicator
	sb.WriteString(fmt.Sprintf("Variable %d of %d\n\n", m.varPromptIndex+1, len(m.varPromptVars)))

	if m.varPromptIndex >= len(m.varPromptVars) {
		sb.WriteString("All variables configured.\n")
		return sb.String()
	}

	v := m.varPromptVars[m.varPromptIndex]

	// Variable info
	sb.WriteString(titleStyle.Render(v.Name) + "\n")
	if v.Description != "" {
		sb.WriteString(v.Description + "\n")
	}
	sb.WriteString(fmt.Sprintf("Type: %s", v.Type))
	if v.Required {
		sb.WriteString(" (required)")
	}
	sb.WriteString("\n\n")

	// Input based on mode
	switch m.varPromptMode {
	case modeText:
		sb.WriteString("Value: " + m.varPromptInput.View() + "\n")
	case modeChoice:
		sb.WriteString(m.varPromptChoice.View() + "\n")
	case modeBoolean:
		yesStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		noStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		if m.varPromptBool {
			yesStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
		} else {
			noStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
		}
		sb.WriteString("  " + noStyle.Render("[ ] No") + "   " + yesStyle.Render("[✓] Yes") + "\n")
		sb.WriteString("\nUse j/k or space to toggle\n")
	}

	// Error message
	if m.varPromptError != "" {
		sb.WriteString("\n" + promptErrorStyle.Render("Error: "+m.varPromptError) + "\n")
	}

	sb.WriteString("\n" + helpStyle.Render("enter: submit • esc: cancel"))

	return lipgloss.NewStyle().Padding(2).Render(sb.String())
}

// renderConfirmCreate renders the creation confirmation UI.
func (m TemplateExplorerModel) renderConfirmCreate() string {
	var sb strings.Builder

	sb.WriteString(headerStyle.Render("Confirm Workspace Creation") + "\n\n")

	owner := strings.ToLower(strings.TrimSpace(m.ownerInput.Value()))
	project := strings.ToLower(strings.TrimSpace(m.projectInput.Value()))
	slug := owner + "--" + project

	sb.WriteString(fmt.Sprintf("Template:  %s\n", titleStyle.Render(m.selected.Info.Name)))
	sb.WriteString(fmt.Sprintf("Owner:     %s\n", owner))
	sb.WriteString(fmt.Sprintf("Project:   %s\n", project))
	sb.WriteString(fmt.Sprintf("Slug:      %s\n", slug))
	sb.WriteString(fmt.Sprintf("Dry-run:   %v\n", m.dryRun))
	sb.WriteString(fmt.Sprintf("No hooks:  %v\n", m.noHooks))
	sb.WriteString("\n")

	// Show collected variables
	if len(m.createVars) > 0 {
		sb.WriteString("Variables:\n")
		for k, v := range m.createVars {
			// Skip builtins for cleaner display
			if k == "OWNER" || k == "PROJECT" || k == "SLUG" || k == "CODE_ROOT" || k == "WORKSPACE_PATH" {
				continue
			}
			if k == "CREATED_DATE" || k == "CREATED_DATETIME" || k == "YEAR" || k == "HOME" {
				continue
			}
			if k == "GIT_USER_NAME" || k == "GIT_USER_EMAIL" {
				continue
			}
			displayVal := v
			if len(displayVal) > 40 {
				displayVal = displayVal[:37] + "..."
			}
			sb.WriteString(fmt.Sprintf("  %s: %s\n", k, displayVal))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(helpStyle.Render("Press 'y' or enter to create, 'n' or esc to cancel"))

	return lipgloss.NewStyle().Padding(2).Render(sb.String())
}

// renderCreating renders the creation in progress UI.
func (m TemplateExplorerModel) renderCreating() string {
	var sb strings.Builder

	sb.WriteString(headerStyle.Render("Creating Workspace...") + "\n\n")

	owner := strings.ToLower(strings.TrimSpace(m.ownerInput.Value()))
	project := strings.ToLower(strings.TrimSpace(m.projectInput.Value()))
	slug := owner + "--" + project

	sb.WriteString(fmt.Sprintf("Creating %s from template %s\n\n", slug, m.selected.Info.Name))
	sb.WriteString("Please wait...\n")

	return lipgloss.NewStyle().Padding(2).Render(sb.String())
}

// renderCreateComplete renders the creation complete UI.
func (m TemplateExplorerModel) renderCreateComplete() string {
	var sb strings.Builder

	if m.createErr != nil {
		sb.WriteString(headerStyle.Render("Creation Failed") + "\n\n")
		sb.WriteString(promptErrorStyle.Render("Error: "+m.createErr.Error()) + "\n\n")
		sb.WriteString(helpStyle.Render("Press enter or esc to go back"))
		return lipgloss.NewStyle().Padding(2).Render(sb.String())
	}

	result := m.createResult

	sb.WriteString(headerStyle.Render("Workspace Created Successfully!") + "\n\n")

	sb.WriteString(titleStyle.Render(result.WorkspaceSlug) + "\n\n")

	sb.WriteString(fmt.Sprintf("Path:           %s\n", result.WorkspacePath))
	if result.TemplateUsed != "" {
		sb.WriteString(fmt.Sprintf("Template:       %s\n", result.TemplateUsed))
	}
	sb.WriteString(fmt.Sprintf("Files created:  %d (%d global, %d template)\n",
		result.FilesCreated, result.GlobalFiles, result.TemplateFiles))
	sb.WriteString(fmt.Sprintf("Repos:          %d created, %d cloned\n",
		result.ReposCreated, result.ReposCloned))

	if len(result.HooksRun) > 0 {
		sb.WriteString(fmt.Sprintf("Hooks run:      %s\n", strings.Join(result.HooksRun, ", ")))
	}
	if len(result.HooksSkipped) > 0 {
		sb.WriteString(fmt.Sprintf("Hooks skipped:  %s\n", strings.Join(result.HooksSkipped, ", ")))
	}

	if len(result.Warnings) > 0 {
		sb.WriteString("\nWarnings:\n")
		for _, w := range result.Warnings {
			sb.WriteString("  ⚠ " + w + "\n")
		}
	}

	sb.WriteString("\n" + helpStyle.Render("Press 'o' to open in editor, enter/esc to continue"))

	return lipgloss.NewStyle().Padding(2).Render(sb.String())
}

// updateFilesTab handles key events for the Files tab.
func (m TemplateExplorerModel) updateFilesTab(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "tab":
		// Toggle between tree and viewer panes
		m.filesFocusPane = (m.filesFocusPane + 1) % 2
		return m, nil

	case "shift+tab":
		return m.switchTab((m.activeTab + 4) % 5)

	case "1", "2", "3", "4":
		tabNum := int(msg.String()[0] - '1')
		return m.switchTab(Tab(tabNum))

	case "L":
		m.showLineNumbers = !m.showLineNumbers
		m.fileViewport.SetContent(m.formatFileContent())
		return m, nil

	case "r":
		// Toggle render mode for template files
		if m.fileIsTemplate {
			m.fileRenderMode = !m.fileRenderMode
			m.fileViewport.SetContent(m.formatFileContent())
		}
		return m, nil

	case "d":
		// Show file pattern diagnostics
		if m.selected != nil {
			m.diagShowPatterns = true
			return m, m.loadFileDiagnostics()
		}
		return m, nil

	case "D":
		// Show placeholder scan diagnostics
		if m.selected != nil {
			m.diagShowPatterns = false
			return m, m.loadPlaceholderDiagnostics()
		}
		return m, nil
	}

	// Delegate to focused pane
	if m.filesFocusPane == 0 {
		return m.updateFilesTreePane(msg)
	}
	return m.updateFilesViewerPane(msg)
}

// updateFilesTreePane handles key events for the file tree pane.
func (m TemplateExplorerModel) updateFilesTreePane(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.fileTreeSelected < len(m.flatFileTree)-1 {
			m.fileTreeSelected++
		}
		return m, nil

	case "k", "up":
		if m.fileTreeSelected > 0 {
			m.fileTreeSelected--
		}
		return m, nil

	case "g":
		m.fileTreeSelected = 0
		return m, nil

	case "G":
		if len(m.flatFileTree) > 0 {
			m.fileTreeSelected = len(m.flatFileTree) - 1
		}
		return m, nil

	case "l", "right":
		// Expand directory or switch to viewer
		if m.fileTreeSelected < len(m.flatFileTree) {
			node := m.flatFileTree[m.fileTreeSelected]
			if node.IsDir {
				if !node.IsExpanded {
					m.toggleFileTreeNode(node)
				}
			} else {
				// Switch to viewer pane
				m.filesFocusPane = 1
			}
		}
		return m, nil

	case "h", "left":
		// Collapse directory
		if m.fileTreeSelected < len(m.flatFileTree) {
			node := m.flatFileTree[m.fileTreeSelected]
			if node.IsDir && node.IsExpanded {
				m.toggleFileTreeNode(node)
			}
		}
		return m, nil

	case "enter":
		// Toggle expand/collapse for dirs, load file for files
		if m.fileTreeSelected < len(m.flatFileTree) {
			node := m.flatFileTree[m.fileTreeSelected]
			if node.IsDir {
				m.toggleFileTreeNode(node)
			} else {
				// Load file content
				m.fileContentPath = node.Path
				m.fileContentError = ""
				m.fileContent = ""
				m.fileRenderedContent = ""
				m.fileRenderMode = false
				return m, m.loadFileContent(node.Path)
			}
		}
		return m, nil
	}

	return m, nil
}

// updateFilesViewerPane handles key events for the file viewer pane.
func (m TemplateExplorerModel) updateFilesViewerPane(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "h", "left":
		m.filesFocusPane = 0
		return m, nil

	case "j", "down":
		m.fileViewport.LineDown(1)
	case "k", "up":
		m.fileViewport.LineUp(1)
	case "d":
		m.fileViewport.HalfViewDown()
	case "u":
		m.fileViewport.HalfViewUp()
	case "g":
		m.fileViewport.GotoTop()
	case "G":
		m.fileViewport.GotoBottom()
	default:
		m.fileViewport, cmd = m.fileViewport.Update(msg)
	}

	return m, cmd
}

// updateOutputTab handles key events for the Output tab.
func (m TemplateExplorerModel) updateOutputTab(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "tab":
		// Toggle between list and details panes
		m.outputFocusPane = (m.outputFocusPane + 1) % 2
		return m, nil

	case "shift+tab":
		return m.switchTab((m.activeTab + 4) % 5)

	case "1", "2", "3", "4", "5":
		tabNum := int(msg.String()[0] - '1')
		return m.switchTab(Tab(tabNum))

	case "j", "down":
		if m.outputFocusPane == 0 && m.outputSelected < len(m.outputMappings)-1 {
			m.outputSelected++
		}
		return m, nil

	case "k", "up":
		if m.outputFocusPane == 0 && m.outputSelected > 0 {
			m.outputSelected--
		}
		return m, nil

	case "g":
		if m.outputFocusPane == 0 {
			m.outputSelected = 0
		}
		return m, nil

	case "G":
		if m.outputFocusPane == 0 && len(m.outputMappings) > 0 {
			m.outputSelected = len(m.outputMappings) - 1
		}
		return m, nil

	case "h", "left":
		if m.outputFocusPane == 1 {
			m.outputFocusPane = 0
		}
		return m, nil

	case "l", "right":
		if m.outputFocusPane == 0 {
			m.outputFocusPane = 1
		}
		return m, nil

	case "enter":
		// Navigate to source file in Files tab
		if len(m.outputMappings) > 0 && m.outputSelected < len(m.outputMappings) {
			mapping := m.outputMappings[m.outputSelected]
			// Switch to Files tab and try to navigate to the source file
			m.activeTab = TabFiles
			m.buildFileTree()
			m.fileTreeSelected = 0
			// Try to find and select the file in the tree
			m.selectFileInTree(mapping.SourcePath)
			return m, nil
		}
		return m, nil
	}

	return m, nil
}

// selectFileInTree attempts to find and select a file in the file tree by path.
func (m *TemplateExplorerModel) selectFileInTree(targetPath string) {
	// Expand directories and find the target file
	for i, node := range m.flatFileTree {
		if node.Path == targetPath {
			m.fileTreeSelected = i
			// Load the file content
			if !node.IsDir {
				m.loadFileContent(node.Path)
			}
			return
		}
	}
	// If not found in current view, try expanding directories
	m.expandPathToFile(targetPath)
}

// expandPathToFile expands the tree to reveal a specific file path.
func (m *TemplateExplorerModel) expandPathToFile(targetPath string) {
	if m.fileTree == nil {
		return
	}

	// Simple search: expand nodes until we find the target
	// This is a basic implementation that may not work for all cases
	m.expandToPath(m.fileTree, targetPath)
	m.flattenFileTree()

	// Try to select the file again
	for i, node := range m.flatFileTree {
		if node.Path == targetPath {
			m.fileTreeSelected = i
			if !node.IsDir {
				m.loadFileContent(node.Path)
			}
			return
		}
	}
}

// expandToPath recursively expands nodes to reveal a target path.
func (m *TemplateExplorerModel) expandToPath(node *fileTreeNode, targetPath string) bool {
	if node.Path == targetPath {
		return true
	}

	if node.IsDir && strings.HasPrefix(targetPath, node.Path+string(filepath.Separator)) {
		node.IsExpanded = true
		for _, child := range node.Children {
			if m.expandToPath(child, targetPath) {
				return true
			}
		}
	}

	return false
}

// toggleFileTreeNode toggles the expanded state of a directory node.
func (m *TemplateExplorerModel) toggleFileTreeNode(node *fileTreeNode) {
	if !node.IsDir {
		return
	}
	node.IsExpanded = !node.IsExpanded
	m.flattenFileTree()
}

// buildFileTree builds the file tree for the currently selected template.
func (m *TemplateExplorerModel) buildFileTree() {
	if m.selected == nil {
		m.fileTree = nil
		m.flatFileTree = nil
		return
	}

	root := &fileTreeNode{
		Name:       m.selected.Info.Name,
		Path:       m.selected.TemplatePath,
		IsDir:      true,
		IsExpanded: true,
		Source:     "template",
		Children:   make([]*fileTreeNode, 0),
	}

	// Add template files directory
	filesPath := filepath.Join(m.selected.TemplatePath, "files")
	if info, err := os.Stat(filesPath); err == nil && info.IsDir() {
		filesNode := &fileTreeNode{
			Name:       "files",
			Path:       filesPath,
			IsDir:      true,
			IsExpanded: true,
			Source:     "template",
			Depth:      1,
		}
		m.buildTreeFromDir(filesNode, filesPath, 2, "template")
		root.Children = append(root.Children, filesNode)
	}

	// Add template hooks directory
	hooksPath := filepath.Join(m.selected.TemplatePath, "hooks")
	if info, err := os.Stat(hooksPath); err == nil && info.IsDir() {
		hooksNode := &fileTreeNode{
			Name:       "hooks",
			Path:       hooksPath,
			IsDir:      true,
			IsExpanded: false,
			Source:     "template",
			Depth:      1,
		}
		m.buildTreeFromDir(hooksNode, hooksPath, 2, "template")
		root.Children = append(root.Children, hooksNode)
	}

	// Add template.json manifest
	manifestPath := filepath.Join(m.selected.TemplatePath, "template.json")
	if _, err := os.Stat(manifestPath); err == nil {
		root.Children = append(root.Children, &fileTreeNode{
			Name:   "template.json",
			Path:   manifestPath,
			IsDir:  false,
			Source: "template",
			Depth:  1,
		})
	}

	// Add global files if any
	for _, globalPath := range m.globalPaths {
		globalNode := &fileTreeNode{
			Name:       "_global",
			Path:       globalPath,
			IsDir:      true,
			IsExpanded: false,
			Source:     "_global",
			Depth:      1,
		}
		m.buildGlobalTreeFromDir(globalNode, globalPath, 2)
		root.Children = append(root.Children, globalNode)
	}

	m.fileTree = root
	m.flattenFileTree()
}

// buildOutputMappings builds the output mappings for the currently selected template.
func (m *TemplateExplorerModel) buildOutputMappings() {
	if m.selected == nil {
		m.outputMappings = nil
		return
	}

	// Load the full template
	tmpl, err := template.LoadTemplate(m.selected.SourceDir, m.selected.Info.Name)
	if err != nil {
		m.outputMappings = nil
		return
	}

	mappings, err := template.BuildOutputMapping(tmpl, m.cfg.AllTemplatesDirs(), m.selected.TemplatePath)
	if err != nil {
		m.outputMappings = nil
		return
	}

	m.outputMappings = mappings
	m.outputSelected = 0
}

// buildTreeFromDir recursively builds tree nodes from a directory.
func (m *TemplateExplorerModel) buildTreeFromDir(parent *fileTreeNode, dirPath string, depth int, source string) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return
	}

	parent.Children = make([]*fileTreeNode, 0)
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}

		nodePath := filepath.Join(dirPath, name)
		node := &fileTreeNode{
			Name:       name,
			Path:       nodePath,
			RelPath:    strings.TrimPrefix(nodePath, m.selected.TemplatePath+"/"),
			IsDir:      entry.IsDir(),
			IsExpanded: false,
			Source:     source,
			Depth:      depth,
		}

		if entry.IsDir() {
			m.buildTreeFromDir(node, nodePath, depth+1, source)
		}

		parent.Children = append(parent.Children, node)
	}
}

// buildGlobalTreeFromDir recursively builds tree nodes from a global directory.
func (m *TemplateExplorerModel) buildGlobalTreeFromDir(parent *fileTreeNode, dirPath string, depth int) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return
	}

	parent.Children = make([]*fileTreeNode, 0)
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}

		nodePath := filepath.Join(dirPath, name)
		node := &fileTreeNode{
			Name:       name,
			Path:       nodePath,
			IsDir:      entry.IsDir(),
			IsExpanded: false,
			Source:     "_global",
			Depth:      depth,
		}

		if entry.IsDir() {
			m.buildGlobalTreeFromDir(node, nodePath, depth+1)
		}

		parent.Children = append(parent.Children, node)
	}
}

// flattenFileTree flattens the tree into a list for display.
func (m *TemplateExplorerModel) flattenFileTree() {
	m.flatFileTree = make([]*fileTreeNode, 0)
	if m.fileTree != nil {
		m.flattenNode(m.fileTree)
	}
}

// flattenNode recursively flattens a tree node.
func (m *TemplateExplorerModel) flattenNode(node *fileTreeNode) {
	m.flatFileTree = append(m.flatFileTree, node)
	if node.IsDir && node.IsExpanded {
		for _, child := range node.Children {
			m.flattenNode(child)
		}
	}
}

// loadFileContent loads the content of a file asynchronously.
func (m TemplateExplorerModel) loadFileContent(path string) tea.Cmd {
	return func() tea.Msg {
		info, err := os.Stat(path)
		if err != nil {
			return fileContentMsg{path: path, err: err}
		}

		size := info.Size()
		isBinary := false
		isLarge := size > maxFileViewerSize

		if isLarge {
			return fileContentMsg{
				path:    path,
				size:    size,
				isLarge: true,
			}
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fileContentMsg{path: path, err: err}
		}

		// Check if binary
		isBinary = isBinaryData(content)
		if isBinary {
			return fileContentMsg{
				path:     path,
				size:     size,
				isBinary: true,
			}
		}

		// Check if template file
		isTemplate := isTemplateFile(path, m.getTemplateExtensions())
		contentStr := string(content)

		// Render template if applicable
		var renderedContent string
		if isTemplate {
			vars := m.getPreviewVariables()
			rendered, _ := template.ProcessTemplateContent(contentStr, vars)
			renderedContent = rendered
		}

		return fileContentMsg{
			path:            path,
			content:         contentStr,
			renderedContent: renderedContent,
			size:            size,
			isBinary:        false,
			isLarge:         false,
			isTemplate:      isTemplate,
		}
	}
}

// isBinaryData checks if content appears to be binary.
func isBinaryData(data []byte) bool {
	// Check first 512 bytes for null bytes
	checkLen := 512
	if len(data) < checkLen {
		checkLen = len(data)
	}
	for i := 0; i < checkLen; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}

// isTemplateFile checks if a file is a template based on its extension.
func isTemplateFile(path string, extensions []string) bool {
	for _, ext := range extensions {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

// getTemplateExtensions returns the template extensions for the selected template.
func (m TemplateExplorerModel) getTemplateExtensions() []string {
	if m.loadedTemplate != nil {
		return m.loadedTemplate.GetTemplateExtensions()
	}
	return []string{".tmpl"}
}

// getPreviewVariables returns variables for template preview.
func (m TemplateExplorerModel) getPreviewVariables() map[string]string {
	vars := make(map[string]string)

	// Add any user-provided values from Create tab
	for k, v := range m.createVars {
		vars[k] = v
	}

	// Add placeholder builtins
	owner := strings.TrimSpace(m.ownerInput.Value())
	if owner == "" {
		owner = "<owner>"
	}
	project := strings.TrimSpace(m.projectInput.Value())
	if project == "" {
		project = "<project>"
	}

	vars["OWNER"] = owner
	vars["PROJECT"] = project
	vars["SLUG"] = owner + "--" + project
	vars["CODE_ROOT"] = m.cfg.CodeRoot
	vars["WORKSPACE_PATH"] = filepath.Join(m.cfg.CodeRoot, owner+"--"+project)
	vars["CREATED_DATE"] = "<date>"
	vars["CREATED_DATETIME"] = "<datetime>"
	vars["YEAR"] = "<year>"

	if home, err := os.UserHomeDir(); err == nil {
		vars["HOME"] = home
	}

	return vars
}

// RunTemplateExplorer runs the template explorer TUI.
func RunTemplateExplorer(cfg *config.Config) error {
	// Load templates from all directories
	listings, globalPaths, err := template.ListTemplateListingsMulti(cfg.AllTemplatesDirs())
	if err != nil {
		return fmt.Errorf("loading templates: %w", err)
	}

	m := NewTemplateExplorer(cfg, listings, globalPaths)
	p := tea.NewProgram(m, tea.WithAltScreen())

	_, err = p.Run()
	return err
}

// loadFileDiagnostics loads file pattern diagnostics for the selected template.
func (m TemplateExplorerModel) loadFileDiagnostics() tea.Cmd {
	return func() tea.Msg {
		if m.selected == nil {
			return diagFileDiagsMsg{err: fmt.Errorf("no template selected")}
		}

		// Load the template to get include/exclude patterns
		tmpl, err := template.LoadTemplate(m.selected.SourceDir, m.selected.Info.Name)
		if err != nil {
			return diagFileDiagsMsg{err: err}
		}

		diags, err := template.DiagnoseTemplateFiles(tmpl, m.selected.SourceDir)
		if err != nil {
			return diagFileDiagsMsg{err: err}
		}

		return diagFileDiagsMsg{diags: diags}
	}
}

// loadPlaceholderDiagnostics loads placeholder diagnostics for the selected template.
func (m TemplateExplorerModel) loadPlaceholderDiagnostics() tea.Cmd {
	return func() tea.Msg {
		if m.selected == nil {
			return diagPlaceholdersMsg{err: fmt.Errorf("no template selected")}
		}

		// Build available variables (builtins + template defaults)
		availableVars := m.getPreviewVariables()

		// Load template to get declared variables
		tmpl, err := template.LoadTemplate(m.selected.SourceDir, m.selected.Info.Name)
		if err == nil {
			for _, v := range tmpl.Variables {
				if v.Default != nil {
					if s, ok := v.Default.(string); ok {
						availableVars[v.Name] = s
					} else {
						availableVars[v.Name] = fmt.Sprintf("%v", v.Default)
					}
				} else {
					// Mark required vars as available (user would provide)
					availableVars[v.Name] = "<user-provided>"
				}
			}
		}

		report, err := template.ScanForPlaceholders(m.selected.SourceDir, m.selected.Info.Name, availableVars)
		if err != nil {
			return diagPlaceholdersMsg{err: err}
		}

		return diagPlaceholdersMsg{report: report}
	}
}

// updateDiagnosticsOverlay handles key events when the diagnostics overlay is showing.
func (m TemplateExplorerModel) updateDiagnosticsOverlay(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		m.diagMode = false
		return m, nil

	case "j", "down":
		maxIdx := m.getDiagnosticsCount() - 1
		if m.diagSelected < maxIdx {
			m.diagSelected++
			m.diagViewport.SetContent(m.formatDiagnosticsContent())
		}
		return m, nil

	case "k", "up":
		if m.diagSelected > 0 {
			m.diagSelected--
			m.diagViewport.SetContent(m.formatDiagnosticsContent())
		}
		return m, nil

	case "g":
		m.diagSelected = 0
		m.diagViewport.SetContent(m.formatDiagnosticsContent())
		return m, nil

	case "G":
		maxIdx := m.getDiagnosticsCount() - 1
		if maxIdx >= 0 {
			m.diagSelected = maxIdx
			m.diagViewport.SetContent(m.formatDiagnosticsContent())
		}
		return m, nil

	case "p":
		// Toggle between patterns and placeholders mode
		if m.selected != nil {
			m.diagShowPatterns = !m.diagShowPatterns
			m.diagSelected = 0
			if m.diagShowPatterns {
				return m, m.loadFileDiagnostics()
			}
			return m, m.loadPlaceholderDiagnostics()
		}
		return m, nil
	}

	return m, nil
}

// getDiagnosticsCount returns the number of items in the current diagnostics view.
func (m TemplateExplorerModel) getDiagnosticsCount() int {
	if m.diagShowPatterns {
		return len(m.diagFileDiags)
	}
	if m.diagReport != nil {
		return len(m.diagReport.Placeholders)
	}
	return 0
}

// formatDiagnosticsContent formats the diagnostics content for the viewport.
func (m TemplateExplorerModel) formatDiagnosticsContent() string {
	var sb strings.Builder

	if m.diagShowPatterns {
		sb.WriteString(headerStyle.Render("File Pattern Diagnostics") + "\n\n")

		if m.selected != nil {
			sb.WriteString(fmt.Sprintf("Template: %s\n\n", m.selected.Info.Name))
		}

		if len(m.diagFileDiags) == 0 {
			sb.WriteString("No files found in template.\n")
			return sb.String()
		}

		for i, diag := range m.diagFileDiags {
			prefix := "  "
			style := lipgloss.NewStyle()
			if i == m.diagSelected {
				prefix = "> "
				style = style.Bold(true).Foreground(lipgloss.Color("212"))
			}

			icon := "✓"
			iconStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
			if !diag.MatchResult.Included {
				icon = "✗"
				iconStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
			}

			tmplIcon := ""
			if diag.IsTemplate {
				tmplIcon = " " + lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Render("[tmpl]")
			}

			line := fmt.Sprintf("%s%s %s%s", prefix, iconStyle.Render(icon), diag.FileRel, tmplIcon)
			sb.WriteString(style.Render(line) + "\n")

			// Show details for selected item
			if i == m.diagSelected {
				reasonStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).PaddingLeft(4)
				sb.WriteString(reasonStyle.Render(diag.MatchResult.Reason) + "\n")
				if diag.MatchResult.MatchedPattern != "" {
					sb.WriteString(reasonStyle.Render("Pattern: "+diag.MatchResult.MatchedPattern) + "\n")
				}
			}
		}

	} else {
		sb.WriteString(headerStyle.Render("Placeholder Diagnostics") + "\n\n")

		if m.diagReport == nil {
			sb.WriteString("No diagnostics report available.\n")
			return sb.String()
		}

		sb.WriteString(fmt.Sprintf("Template: %s\n", m.diagReport.TemplateName))
		sb.WriteString(fmt.Sprintf("Files scanned: %d of %d\n\n", m.diagReport.TotalScanned, m.diagReport.TotalFiles))

		if len(m.diagReport.Placeholders) == 0 {
			sb.WriteString("No placeholders found in template files.\n")
			return sb.String()
		}

		unresolved := m.diagReport.GetUnresolvedPlaceholders()
		if len(unresolved) > 0 {
			warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
			sb.WriteString(warningStyle.Render(fmt.Sprintf("⚠ %d unresolved placeholder(s)", len(unresolved))) + "\n\n")
		}

		for i, p := range m.diagReport.Placeholders {
			prefix := "  "
			style := lipgloss.NewStyle()
			if i == m.diagSelected {
				prefix = "> "
				style = style.Bold(true).Foreground(lipgloss.Color("212"))
			}

			icon := "✓"
			iconStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
			if !p.IsAvailable {
				icon = "⚠"
				iconStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
			}

			varName := lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Render("{{" + p.VarName + "}}")
			loc := fmt.Sprintf("%s:%d:%d", p.FileRel, p.Line, p.Column)

			line := fmt.Sprintf("%s%s %s at %s", prefix, iconStyle.Render(icon), varName, loc)
			sb.WriteString(style.Render(line) + "\n")

			// Show context for selected item
			if i == m.diagSelected {
				contextStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).PaddingLeft(4)
				sb.WriteString(contextStyle.Render("Context: "+p.Context) + "\n")
				if p.IsAvailable {
					sb.WriteString(contextStyle.Render("Status: Variable is available") + "\n")
				} else {
					sb.WriteString(contextStyle.Render("Status: Variable may be unresolved") + "\n")
				}
			}
		}
	}

	return sb.String()
}

// renderDiagnosticsOverlay renders the diagnostics overlay view.
func (m TemplateExplorerModel) renderDiagnosticsOverlay() string {
	var sb strings.Builder

	// Title
	title := "Diagnostics"
	if m.diagShowPatterns {
		title = "Pattern Diagnostics"
	} else {
		title = "Placeholder Diagnostics"
	}
	titleBar := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("212")).
		Padding(0, 1).
		Render(title)

	sb.WriteString(titleBar + "\n\n")

	// Content
	contentHeight := m.height - 8
	if contentHeight < 10 {
		contentHeight = 10
	}
	contentWidth := m.width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	m.diagViewport.Width = contentWidth
	m.diagViewport.Height = contentHeight

	content := m.formatDiagnosticsContent()
	m.diagViewport.SetContent(content)

	contentBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1).
		Width(contentWidth).
		Height(contentHeight).
		Render(content)

	sb.WriteString(contentBox + "\n")

	// Help
	help := "j/k: navigate • g/G: top/bottom • p: toggle patterns/placeholders • esc: close"
	helpLine := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(help)
	sb.WriteString("\n" + helpLine)

	return sb.String()
}

// compareTemplates compares the marked template with the currently selected one.
func (m TemplateExplorerModel) compareTemplates() tea.Cmd {
	return func() tea.Msg {
		if m.compareMarked == nil || m.selected == nil {
			return compareResultMsg{err: fmt.Errorf("no templates selected for comparison")}
		}

		// Load both templates
		tmplA, err := template.LoadTemplate(m.compareMarked.SourceDir, m.compareMarked.Info.Name)
		if err != nil {
			return compareResultMsg{err: fmt.Errorf("failed to load %s: %w", m.compareMarked.Info.Name, err)}
		}

		tmplB, err := template.LoadTemplate(m.selected.SourceDir, m.selected.Info.Name)
		if err != nil {
			return compareResultMsg{err: fmt.Errorf("failed to load %s: %w", m.selected.Info.Name, err)}
		}

		// Compare templates
		result, err := template.CompareTemplates(tmplA, tmplB, m.compareMarked.SourceDir, m.selected.SourceDir)
		if err != nil {
			return compareResultMsg{err: err}
		}

		return compareResultMsg{result: result}
	}
}

// updateCompareOverlay handles key events in compare overlay mode.
func (m TemplateExplorerModel) updateCompareOverlay(msg tea.KeyMsg) (TemplateExplorerModel, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.compareMode = false
		m.compareMarked = nil
		m.compareResult = nil
		return m, nil

	case "j", "down":
		m.compareSelected++
		total := m.getCompareItemCount()
		if m.compareSelected >= total {
			m.compareSelected = total - 1
		}
		if m.compareSelected < 0 {
			m.compareSelected = 0
		}

	case "k", "up":
		m.compareSelected--
		if m.compareSelected < 0 {
			m.compareSelected = 0
		}

	case "tab", "l", "right":
		// Next section
		m.compareSection = (m.compareSection + 1) % 4
		m.compareSelected = 0

	case "shift+tab", "h", "left":
		// Previous section
		m.compareSection = (m.compareSection + 3) % 4 // +3 is same as -1 mod 4
		m.compareSelected = 0

	case "g":
		m.compareSelected = 0

	case "G":
		total := m.getCompareItemCount()
		m.compareSelected = total - 1
		if m.compareSelected < 0 {
			m.compareSelected = 0
		}
	}

	return m, nil
}

// getCompareItemCount returns the number of items in the current compare section.
func (m TemplateExplorerModel) getCompareItemCount() int {
	if m.compareResult == nil {
		return 0
	}

	switch m.compareSection {
	case 0:
		return len(m.compareResult.Vars)
	case 1:
		return len(m.compareResult.Repos)
	case 2:
		return len(m.compareResult.Hooks)
	case 3:
		return len(m.compareResult.Files)
	default:
		return 0
	}
}

// formatCompareContent formats the comparison result for display.
func (m TemplateExplorerModel) formatCompareContent() string {
	if m.compareResult == nil {
		return "No comparison data available."
	}

	var sb strings.Builder

	// Section headers
	sections := []struct {
		name  string
		index int
		count int
	}{
		{"Variables", 0, len(m.compareResult.Vars)},
		{"Repos", 1, len(m.compareResult.Repos)},
		{"Hooks", 2, len(m.compareResult.Hooks)},
		{"Files", 3, len(m.compareResult.Files)},
	}

	// Show section tabs
	var tabs []string
	for _, s := range sections {
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		if s.index == m.compareSection {
			style = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
		}
		tabs = append(tabs, style.Render(fmt.Sprintf("%s (%d)", s.name, s.count)))
	}
	sb.WriteString(strings.Join(tabs, " │ ") + "\n\n")

	// Show content for current section
	switch m.compareSection {
	case 0:
		m.formatVarsDiff(&sb)
	case 1:
		m.formatReposDiff(&sb)
	case 2:
		m.formatHooksDiff(&sb)
	case 3:
		m.formatFilesDiff(&sb)
	}

	return sb.String()
}

// formatVarsDiff formats variable differences.
func (m TemplateExplorerModel) formatVarsDiff(sb *strings.Builder) {
	if len(m.compareResult.Vars) == 0 {
		sb.WriteString("No differences in variables.\n")
		return
	}

	for i, v := range m.compareResult.Vars {
		prefix := "  "
		style := lipgloss.NewStyle()
		if i == m.compareSelected {
			prefix = "▶ "
			style = style.Bold(true).Foreground(lipgloss.Color("212"))
		}

		icon, iconStyle := getDiffIcon(v.DiffType)

		line := fmt.Sprintf("%s%s %s", prefix, iconStyle.Render(icon), v.Name)
		sb.WriteString(style.Render(line) + "\n")

		// Show details for selected item
		if i == m.compareSelected {
			detailStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).PaddingLeft(4)
			switch v.DiffType {
			case template.DiffAdded:
				sb.WriteString(detailStyle.Render("Added in B: "+v.ValueB) + "\n")
			case template.DiffRemoved:
				sb.WriteString(detailStyle.Render("Removed from A: "+v.ValueA) + "\n")
			case template.DiffChanged:
				sb.WriteString(detailStyle.Render("In A: "+v.ValueA) + "\n")
				sb.WriteString(detailStyle.Render("In B: "+v.ValueB) + "\n")
			}
		}
	}
}

// formatReposDiff formats repository differences.
func (m TemplateExplorerModel) formatReposDiff(sb *strings.Builder) {
	if len(m.compareResult.Repos) == 0 {
		sb.WriteString("No differences in repositories.\n")
		return
	}

	for i, r := range m.compareResult.Repos {
		prefix := "  "
		style := lipgloss.NewStyle()
		if i == m.compareSelected {
			prefix = "▶ "
			style = style.Bold(true).Foreground(lipgloss.Color("212"))
		}

		icon, iconStyle := getDiffIcon(r.DiffType)

		line := fmt.Sprintf("%s%s %s", prefix, iconStyle.Render(icon), r.Name)
		sb.WriteString(style.Render(line) + "\n")

		if i == m.compareSelected {
			detailStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).PaddingLeft(4)
			switch r.DiffType {
			case template.DiffAdded:
				sb.WriteString(detailStyle.Render("Added in B: "+r.CloneB) + "\n")
			case template.DiffRemoved:
				sb.WriteString(detailStyle.Render("Removed from A: "+r.CloneA) + "\n")
			case template.DiffChanged:
				sb.WriteString(detailStyle.Render("In A: "+r.CloneA) + "\n")
				sb.WriteString(detailStyle.Render("In B: "+r.CloneB) + "\n")
			}
		}
	}
}

// formatHooksDiff formats hook differences.
func (m TemplateExplorerModel) formatHooksDiff(sb *strings.Builder) {
	if len(m.compareResult.Hooks) == 0 {
		sb.WriteString("No differences in hooks.\n")
		return
	}

	for i, h := range m.compareResult.Hooks {
		prefix := "  "
		style := lipgloss.NewStyle()
		if i == m.compareSelected {
			prefix = "▶ "
			style = style.Bold(true).Foreground(lipgloss.Color("212"))
		}

		icon, iconStyle := getDiffIcon(h.DiffType)

		line := fmt.Sprintf("%s%s %s", prefix, iconStyle.Render(icon), h.Name)
		sb.WriteString(style.Render(line) + "\n")

		if i == m.compareSelected {
			detailStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).PaddingLeft(4)
			switch h.DiffType {
			case template.DiffAdded:
				sb.WriteString(detailStyle.Render("Added in B: "+h.ScriptB) + "\n")
			case template.DiffRemoved:
				sb.WriteString(detailStyle.Render("Removed from A: "+h.ScriptA) + "\n")
			case template.DiffChanged:
				sb.WriteString(detailStyle.Render("In A: "+h.ScriptA) + "\n")
				sb.WriteString(detailStyle.Render("In B: "+h.ScriptB) + "\n")
			}
		}
	}
}

// formatFilesDiff formats file differences.
func (m TemplateExplorerModel) formatFilesDiff(sb *strings.Builder) {
	if len(m.compareResult.Files) == 0 {
		sb.WriteString("No differences in files.\n")
		return
	}

	for i, f := range m.compareResult.Files {
		prefix := "  "
		style := lipgloss.NewStyle()
		if i == m.compareSelected {
			prefix = "▶ "
			style = style.Bold(true).Foreground(lipgloss.Color("212"))
		}

		icon, iconStyle := getDiffIcon(f.DiffType)

		line := fmt.Sprintf("%s%s %s", prefix, iconStyle.Render(icon), f.OutputPath)
		sb.WriteString(style.Render(line) + "\n")

		if i == m.compareSelected {
			detailStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).PaddingLeft(4)
			switch f.DiffType {
			case template.DiffAdded:
				sb.WriteString(detailStyle.Render("Only in B") + "\n")
			case template.DiffRemoved:
				sb.WriteString(detailStyle.Render("Only in A") + "\n")
			}
		}
	}
}

// getDiffIcon returns the icon and style for a diff type.
func getDiffIcon(dt template.DiffType) (string, lipgloss.Style) {
	switch dt {
	case template.DiffAdded:
		return "+", lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	case template.DiffRemoved:
		return "-", lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	case template.DiffChanged:
		return "~", lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	default:
		return "?", lipgloss.NewStyle()
	}
}

// renderCompareOverlay renders the compare overlay view.
func (m TemplateExplorerModel) renderCompareOverlay() string {
	var sb strings.Builder

	// Title
	title := fmt.Sprintf("Comparing: %s ↔ %s", m.compareResult.TemplateA, m.compareResult.TemplateB)
	titleBar := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("212")).
		Padding(0, 1).
		Render(title)

	sb.WriteString(titleBar + "\n\n")

	// Summary
	totalDiffs := m.compareResult.TotalDiffs()
	if totalDiffs == 0 {
		summaryStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
		sb.WriteString(summaryStyle.Render("✓ Templates are identical") + "\n\n")
	} else {
		summaryStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
		sb.WriteString(summaryStyle.Render(fmt.Sprintf("Found %d difference(s)", totalDiffs)) + "\n\n")
	}

	// Content
	contentHeight := m.height - 12
	if contentHeight < 10 {
		contentHeight = 10
	}
	contentWidth := m.width - 4
	if contentWidth < 40 {
		contentWidth = 40
	}

	content := m.formatCompareContent()

	contentBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1).
		Width(contentWidth).
		Height(contentHeight).
		Render(content)

	sb.WriteString(contentBox + "\n")

	// Help
	help := "j/k: navigate • tab/h/l: switch section • g/G: top/bottom • esc: close"
	helpLine := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(help)
	sb.WriteString("\n" + helpLine)

	return sb.String()
}
