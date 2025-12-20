package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	promptLabelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	promptHintStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	promptErrorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

type ImportPromptResult struct {
	Owner   string
	Project string
	Abort   bool
}

type importPromptModel struct {
	ownerInput   textinput.Model
	projectInput textinput.Model
	focusIndex   int
	sourceFolder string
	gitRoots     []string
	err          string
	done         bool
	result       ImportPromptResult
}

func newImportPromptModel(sourceFolder string, gitRoots []string, suggestedOwner, suggestedProject string) importPromptModel {
	oi := textinput.New()
	oi.Placeholder = "owner"
	oi.CharLimit = 64
	oi.Width = 30
	oi.SetValue(suggestedOwner)
	oi.Focus()

	pi := textinput.New()
	pi.Placeholder = "project"
	pi.CharLimit = 64
	pi.Width = 30
	pi.SetValue(suggestedProject)

	return importPromptModel{
		ownerInput:   oi,
		projectInput: pi,
		focusIndex:   0,
		sourceFolder: sourceFolder,
		gitRoots:     gitRoots,
	}
}

func (m importPromptModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m importPromptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.result.Abort = true
			m.done = true
			return m, tea.Quit

		case "tab", "down", "enter":
			if msg.String() == "enter" && m.focusIndex == 1 {
				owner := strings.ToLower(strings.TrimSpace(m.ownerInput.Value()))
				project := strings.ToLower(strings.TrimSpace(m.projectInput.Value()))

				if owner == "" {
					m.err = "owner is required"
					return m, nil
				}
				if project == "" {
					m.err = "project is required"
					return m, nil
				}

				if !isValidSlugPart(owner) {
					m.err = "owner must be lowercase alphanumeric with hyphens"
					return m, nil
				}
				if !isValidSlugPart(project) {
					m.err = "project must be lowercase alphanumeric with hyphens"
					return m, nil
				}

				m.result.Owner = owner
				m.result.Project = project
				m.done = true
				return m, tea.Quit
			}
			m.focusIndex = (m.focusIndex + 1) % 2
			m.ownerInput.Blur()
			m.projectInput.Blur()
			if m.focusIndex == 0 {
				return m, m.ownerInput.Focus()
			}
			return m, m.projectInput.Focus()

		case "shift+tab", "up":
			m.focusIndex = (m.focusIndex + 1) % 2
			m.ownerInput.Blur()
			m.projectInput.Blur()
			if m.focusIndex == 0 {
				return m, m.ownerInput.Focus()
			}
			return m, m.projectInput.Focus()
		}
	}

	var cmd tea.Cmd
	if m.focusIndex == 0 {
		m.ownerInput, cmd = m.ownerInput.Update(msg)
	} else {
		m.projectInput, cmd = m.projectInput.Update(msg)
	}

	return m, cmd
}

func (m importPromptModel) View() string {
	var sb strings.Builder

	sb.WriteString(promptLabelStyle.Render("Import folder to workspace") + "\n\n")
	sb.WriteString(fmt.Sprintf("Source: %s\n", m.sourceFolder))

	switch len(m.gitRoots) {
	case 0:
		sb.WriteString("Found:  no git repositories (files only)\n\n")
	case 1:
		sb.WriteString("Found:  1 git repository\n\n")
	default:
		sb.WriteString(fmt.Sprintf("Found:  %d git repositories\n\n", len(m.gitRoots)))
	}

	sb.WriteString(fmt.Sprintf("%s %s\n", promptLabelStyle.Render("Owner:  "), m.ownerInput.View()))
	sb.WriteString(fmt.Sprintf("%s %s\n", promptLabelStyle.Render("Project:"), m.projectInput.View()))

	if m.err != "" {
		sb.WriteString("\n" + promptErrorStyle.Render("Error: "+m.err) + "\n")
	}

	sb.WriteString("\n" + promptHintStyle.Render("tab: next field • enter: confirm • esc: cancel"))

	return sb.String()
}

func isValidSlugPart(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return false
		}
	}
	return true
}

func RunImportPrompt(sourceFolder string, gitRoots []string, suggestedOwner, suggestedProject string) (ImportPromptResult, error) {
	m := newImportPromptModel(sourceFolder, gitRoots, suggestedOwner, suggestedProject)
	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return ImportPromptResult{Abort: true}, err
	}

	result := finalModel.(importPromptModel).result
	return result, nil
}
