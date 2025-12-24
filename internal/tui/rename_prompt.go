package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/model"
)

// RenamePromptResult holds the result of the rename prompt.
type RenamePromptResult struct {
	CurrentSlug string
	NewOwner    string
	NewProject  string
	Cancelled   bool
}

type renameState int

const (
	renameStateSelect renameState = iota
	renameStateOwner
	renameStateProject
)

type renameModel struct {
	cfg          *config.Config
	workspaces   []*model.IndexRecord
	selected     int
	scrollOffset int
	height       int
	width        int

	state        renameState
	ownerInput   textinput.Model
	projectInput textinput.Model
	error        string

	result RenamePromptResult
}

var (
	renameTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("212")).
				MarginBottom(1)

	renameHelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	renameSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("212")).
				Bold(true)

	renameErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196"))
)

func newRenameModel(cfg *config.Config, workspaces []*model.IndexRecord) *renameModel {
	ownerInput := textinput.New()
	ownerInput.Placeholder = "owner"
	ownerInput.CharLimit = 64
	ownerInput.Width = 30

	projectInput := textinput.New()
	projectInput.Placeholder = "project"
	projectInput.CharLimit = 64
	projectInput.Width = 30

	return &renameModel{
		cfg:          cfg,
		workspaces:   workspaces,
		state:        renameStateSelect,
		ownerInput:   ownerInput,
		projectInput: projectInput,
		height:       20,
	}
}

func (m renameModel) Init() tea.Cmd {
	return nil
}

func (m renameModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height - 6
		if m.height < 5 {
			m.height = 5
		}
		return m, nil

	case tea.KeyMsg:
		switch m.state {
		case renameStateSelect:
			return m.handleSelectKeys(msg)
		case renameStateOwner:
			return m.handleOwnerKeys(msg)
		case renameStateProject:
			return m.handleProjectKeys(msg)
		}
	}

	return m, nil
}

func (m renameModel) handleSelectKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c", "esc":
		m.result.Cancelled = true
		return m, tea.Quit

	case "j", "down":
		if m.selected < len(m.workspaces)-1 {
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
		m.selected = 0
		m.scrollOffset = 0
		return m, nil

	case "G":
		m.selected = len(m.workspaces) - 1
		m.ensureVisible()
		return m, nil

	case "enter":
		if len(m.workspaces) == 0 {
			return m, nil
		}
		ws := m.workspaces[m.selected]
		m.result.CurrentSlug = ws.Slug

		// Pre-fill with current values
		parts := strings.SplitN(ws.Slug, "--", 2)
		if len(parts) == 2 {
			m.ownerInput.SetValue(parts[0])
			m.projectInput.SetValue(parts[1])
		}

		m.state = renameStateOwner
		m.ownerInput.Focus()
		return m, textinput.Blink
	}

	return m, nil
}

func (m renameModel) handleOwnerKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.result.Cancelled = true
		return m, tea.Quit

	case "esc":
		m.state = renameStateSelect
		m.ownerInput.Blur()
		m.error = ""
		return m, nil

	case "enter", "tab":
		owner := strings.TrimSpace(m.ownerInput.Value())
		if owner == "" {
			m.error = "Owner is required"
			return m, nil
		}
		m.error = ""
		m.state = renameStateProject
		m.ownerInput.Blur()
		m.projectInput.Focus()
		return m, textinput.Blink
	}

	var cmd tea.Cmd
	m.ownerInput, cmd = m.ownerInput.Update(msg)
	return m, cmd
}

func (m renameModel) handleProjectKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.result.Cancelled = true
		return m, tea.Quit

	case "esc":
		m.state = renameStateOwner
		m.projectInput.Blur()
		m.ownerInput.Focus()
		m.error = ""
		return m, textinput.Blink

	case "enter":
		project := strings.TrimSpace(m.projectInput.Value())
		if project == "" {
			m.error = "Project name is required"
			return m, nil
		}

		owner := strings.TrimSpace(m.ownerInput.Value())
		newSlug := owner + "--" + project

		// Check if same as current
		if newSlug == m.result.CurrentSlug {
			m.error = "New name is the same as current"
			return m, nil
		}

		m.result.NewOwner = owner
		m.result.NewProject = project
		return m, tea.Quit

	case "shift+tab":
		m.state = renameStateOwner
		m.projectInput.Blur()
		m.ownerInput.Focus()
		return m, textinput.Blink
	}

	var cmd tea.Cmd
	m.projectInput, cmd = m.projectInput.Update(msg)
	return m, cmd
}

func (m *renameModel) ensureVisible() {
	if m.selected < m.scrollOffset {
		m.scrollOffset = m.selected
	} else if m.selected >= m.scrollOffset+m.height {
		m.scrollOffset = m.selected - m.height + 1
	}
}

func (m renameModel) View() string {
	var sb strings.Builder

	switch m.state {
	case renameStateSelect:
		sb.WriteString(renameTitleStyle.Render("Select Workspace to Rename") + "\n\n")

		if len(m.workspaces) == 0 {
			sb.WriteString("No workspaces found. Run 'co index' first.\n")
			sb.WriteString("\n" + renameHelpStyle.Render("q: quit"))
			return sb.String()
		}

		// Render visible workspaces
		visibleEnd := m.scrollOffset + m.height
		if visibleEnd > len(m.workspaces) {
			visibleEnd = len(m.workspaces)
		}

		for i := m.scrollOffset; i < visibleEnd; i++ {
			ws := m.workspaces[i]
			prefix := "  "
			line := ws.Slug
			if i == m.selected {
				prefix = "> "
				line = renameSelectedStyle.Render(line)
			}
			sb.WriteString(prefix + line + "\n")
		}

		// Scroll indicator
		if len(m.workspaces) > m.height {
			sb.WriteString(fmt.Sprintf("\n(%d/%d)", m.selected+1, len(m.workspaces)))
		}

		sb.WriteString("\n\n" + renameHelpStyle.Render("j/k: navigate • enter: select • q: quit"))

	case renameStateOwner, renameStateProject:
		sb.WriteString(renameTitleStyle.Render("Rename Workspace") + "\n\n")
		sb.WriteString(fmt.Sprintf("Current: %s\n\n", m.result.CurrentSlug))

		// Owner field
		ownerLabel := "Owner:   "
		if m.state == renameStateOwner {
			ownerLabel = renameSelectedStyle.Render(ownerLabel)
		}
		sb.WriteString(ownerLabel + m.ownerInput.View() + "\n")

		// Project field
		projectLabel := "Project: "
		if m.state == renameStateProject {
			projectLabel = renameSelectedStyle.Render(projectLabel)
		}
		sb.WriteString(projectLabel + m.projectInput.View() + "\n")

		// Preview new slug
		newSlug := strings.TrimSpace(m.ownerInput.Value()) + "--" + strings.TrimSpace(m.projectInput.Value())
		sb.WriteString(fmt.Sprintf("\nNew slug: %s\n", newSlug))

		// Error
		if m.error != "" {
			sb.WriteString("\n" + renameErrorStyle.Render(m.error) + "\n")
		}

		sb.WriteString("\n" + renameHelpStyle.Render("tab: next field • enter: confirm • esc: back"))
	}

	return sb.String()
}

// RunRenamePrompt runs the interactive rename prompt.
func RunRenamePrompt(cfg *config.Config) (RenamePromptResult, error) {
	// Load index
	idx, err := model.LoadIndex(cfg.IndexPath())
	if err != nil {
		return RenamePromptResult{}, fmt.Errorf("failed to load index: %w", err)
	}

	// Sort workspaces by slug
	workspaces := idx.Records

	m := newRenameModel(cfg, workspaces)
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return RenamePromptResult{}, err
	}

	return finalModel.(renameModel).result, nil
}
