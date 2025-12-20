package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Styles for extra files picker
var (
	efPickerTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	efPickerHelpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	efPickerSelectedStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("236")).
				Foreground(lipgloss.Color("212")).
				Bold(true)
	efPickerCheckedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("108")) // muted sage (included)
	efPickerUncheckedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245")) // gray (not included)
	efPickerDirStyle       = lipgloss.NewStyle().Bold(true)
)

// ExtraFilesResult holds the result of the extra files picker.
type ExtraFilesResult struct {
	SelectedPaths []string // paths relative to source folder
	DestSubfolder string   // destination subfolder (empty = project root)
	Confirmed     bool     // true if user confirmed
	Aborted       bool     // true if user cancelled
}

// extraFileItem represents a file or folder that can be selected.
type extraFileItem struct {
	Name    string // file/directory name
	RelPath string // path relative to source folder
	IsDir   bool   // true if directory
	Checked bool   // true if selected for inclusion
}

// extraFilesPickerModel is the Bubble Tea model for selecting extra files.
type extraFilesPickerModel struct {
	sourcePath   string           // absolute path to source folder
	items        []extraFileItem  // list of non-git files/folders
	selected     int              // currently selected index
	width        int              // terminal width
	height       int              // terminal height
	scrollOffset int              // for scrolling long lists
	done         bool             // true when picker is complete
	result       ExtraFilesResult // final result

	// Destination prompt state
	showDestPrompt bool            // true when prompting for destination
	destInput      textinput.Model // text input for destination subfolder
}

// FindNonGitItems finds files and folders in sourcePath that are not inside any git repository.
// gitRoots is the list of git repository roots found in the source path.
func FindNonGitItems(sourcePath string, gitRoots []string) ([]extraFileItem, error) {
	var items []extraFileItem

	// Build a set of git root paths for quick lookup
	gitRootSet := make(map[string]bool)
	for _, root := range gitRoots {
		gitRootSet[root] = true
	}

	// Read the top-level directory
	entries, err := os.ReadDir(sourcePath)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		name := entry.Name()
		fullPath := filepath.Join(sourcePath, name)

		// Skip hidden files that are typically not useful
		if strings.HasPrefix(name, ".") && name != ".env" && name != ".gitignore" {
			continue
		}

		// Check if this path is a git root or inside a git root
		isGitManaged := false
		for gitRoot := range gitRootSet {
			if fullPath == gitRoot || strings.HasPrefix(fullPath, gitRoot+string(filepath.Separator)) {
				isGitManaged = true
				break
			}
		}

		if !isGitManaged {
			items = append(items, extraFileItem{
				Name:    name,
				RelPath: name,
				IsDir:   entry.IsDir(),
				Checked: false,
			})
		}
	}

	return items, nil
}

// newExtraFilesPickerModel creates a new extra files picker model.
func newExtraFilesPickerModel(sourcePath string, items []extraFileItem) extraFilesPickerModel {
	destInput := textinput.New()
	destInput.Placeholder = "subfolder (leave empty for project root)"
	destInput.CharLimit = 128
	destInput.Width = 50

	return extraFilesPickerModel{
		sourcePath:     sourcePath,
		items:          items,
		selected:       0,
		width:          80,
		height:         24,
		showDestPrompt: false,
		destInput:      destInput,
	}
}

// Init implements tea.Model.
func (m extraFilesPickerModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m extraFilesPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle destination prompt mode
	if m.showDestPrompt {
		return m.updateDestPrompt(msg)
	}

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
			// If nothing is selected, skip the extra files step
			selected := m.getSelectedPaths()
			if len(selected) == 0 {
				m.result.Confirmed = true
				m.done = true
				return m, tea.Quit
			}
			// Move to destination prompt
			m.showDestPrompt = true
			return m, m.destInput.Focus()

		case "j", "down":
			if m.selected < len(m.items)-1 {
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
			if len(m.items) > 0 {
				m.selected = len(m.items) - 1
				m.ensureVisible()
			}
			return m, nil

		case " ":
			// Toggle selection
			if m.selected < len(m.items) {
				m.items[m.selected].Checked = !m.items[m.selected].Checked
			}
			return m, nil

		case "a":
			// Select all
			for i := range m.items {
				m.items[i].Checked = true
			}
			return m, nil

		case "n":
			// Select none
			for i := range m.items {
				m.items[i].Checked = false
			}
			return m, nil
		}
	}

	return m, nil
}

// updateDestPrompt handles input when showing the destination prompt.
func (m extraFilesPickerModel) updateDestPrompt(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.result.Aborted = true
			m.done = true
			return m, tea.Quit

		case "esc":
			// Go back to file selection
			m.showDestPrompt = false
			m.destInput.Blur()
			return m, nil

		case "enter":
			// Confirm destination
			dest := strings.TrimSpace(m.destInput.Value())
			// Sanitize: remove leading/trailing slashes
			dest = strings.Trim(dest, "/\\")

			m.result.SelectedPaths = m.getSelectedPaths()
			m.result.DestSubfolder = dest
			m.result.Confirmed = true
			m.done = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.destInput, cmd = m.destInput.Update(msg)
	return m, cmd
}

// getSelectedPaths returns the relative paths of all checked items.
func (m *extraFilesPickerModel) getSelectedPaths() []string {
	var paths []string
	for _, item := range m.items {
		if item.Checked {
			paths = append(paths, item.RelPath)
		}
	}
	return paths
}

// ensureVisible ensures the selected item is visible in the viewport.
func (m *extraFilesPickerModel) ensureVisible() {
	visibleLines := m.height - 10 // account for header/footer
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
func (m extraFilesPickerModel) View() string {
	if m.showDestPrompt {
		return m.viewDestPrompt()
	}

	var sb strings.Builder

	// Title
	sb.WriteString(efPickerTitleStyle.Render("Include Extra Files") + "\n")
	sb.WriteString(efPickerHelpStyle.Render("Found files/folders not managed by git. Select which to include in the workspace.") + "\n\n")

	// Calculate visible area
	visibleLines := m.height - 10
	if visibleLines < 5 {
		visibleLines = 5
	}

	// Render items
	startIdx := m.scrollOffset
	endIdx := startIdx + visibleLines
	if endIdx > len(m.items) {
		endIdx = len(m.items)
	}

	for i := startIdx; i < endIdx; i++ {
		item := m.items[i]
		line := m.renderItem(item, i == m.selected)
		sb.WriteString(line + "\n")
	}

	// Scroll indicator
	if len(m.items) > visibleLines {
		sb.WriteString(fmt.Sprintf("\n(%d/%d)", m.selected+1, len(m.items)))
	}

	// Summary
	selectedCount := 0
	for _, item := range m.items {
		if item.Checked {
			selectedCount++
		}
	}
	sb.WriteString(fmt.Sprintf("\n\n%d of %d selected", selectedCount, len(m.items)))

	// Help
	sb.WriteString("\n\n" + efPickerHelpStyle.Render("j/k: navigate • space: toggle • a: all • n: none"))
	sb.WriteString("\n" + efPickerHelpStyle.Render("enter: continue • q/esc: skip extra files"))

	return sb.String()
}

// viewDestPrompt renders the destination folder prompt.
func (m extraFilesPickerModel) viewDestPrompt() string {
	var sb strings.Builder

	sb.WriteString(efPickerTitleStyle.Render("Destination Folder") + "\n\n")

	selectedCount := 0
	for _, item := range m.items {
		if item.Checked {
			selectedCount++
		}
	}
	sb.WriteString(fmt.Sprintf("Copying %d file(s)/folder(s) to workspace.\n\n", selectedCount))

	sb.WriteString("Enter destination subfolder:\n")
	sb.WriteString("(leave empty to place at project root)\n\n")
	sb.WriteString(m.destInput.View() + "\n")

	sb.WriteString("\n" + efPickerHelpStyle.Render("enter: confirm • esc: back to selection"))

	return sb.String()
}

// renderItem renders a single item row.
func (m extraFilesPickerModel) renderItem(item extraFileItem, isSelected bool) string {
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
		name = efPickerDirStyle.Render(name + "/")
	}

	line := checkbox + name

	// Apply styling
	if isSelected {
		line = efPickerSelectedStyle.Render(line)
	} else if item.Checked {
		line = efPickerCheckedStyle.Render(line)
	} else {
		line = efPickerUncheckedStyle.Render(line)
	}

	return line
}

// RunExtraFilesPicker runs the interactive extra files picker TUI.
func RunExtraFilesPicker(sourcePath string, items []extraFileItem) (ExtraFilesResult, error) {
	if len(items) == 0 {
		return ExtraFilesResult{Confirmed: true}, nil
	}

	m := newExtraFilesPickerModel(sourcePath, items)
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return ExtraFilesResult{Aborted: true}, err
	}

	result := finalModel.(extraFilesPickerModel).result
	return result, nil
}
