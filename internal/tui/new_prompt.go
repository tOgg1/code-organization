package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/tormodhaugland/co/internal/template"
)

type NewPromptResult struct {
	Owner   string
	Project string
	Abort   bool
}

// NewWorkspacePromptResult holds the result of the full new workspace prompt flow.
type NewWorkspacePromptResult struct {
	Owner        string
	Project      string
	TemplateName string            // Empty string means no template
	Variables    map[string]string // Template variables (includes builtins)
	Abort        bool
}

type newPromptModel struct {
	ownerInput   textinput.Model
	projectInput textinput.Model
	focusIndex   int
	err          string
	done         bool
	result       NewPromptResult
}

func newNewPromptModel() newPromptModel {
	oi := textinput.New()
	oi.Placeholder = "owner"
	oi.CharLimit = 64
	oi.Width = 30
	oi.Focus()

	pi := textinput.New()
	pi.Placeholder = "project"
	pi.CharLimit = 64
	pi.Width = 30

	return newPromptModel{
		ownerInput:   oi,
		projectInput: pi,
		focusIndex:   0,
	}
}

func (m newPromptModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m newPromptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m newPromptModel) View() string {
	var sb strings.Builder

	sb.WriteString(promptLabelStyle.Render("Create new workspace") + "\n\n")

	sb.WriteString(fmt.Sprintf("%s %s\n", promptLabelStyle.Render("Owner:  "), m.ownerInput.View()))
	sb.WriteString(fmt.Sprintf("%s %s\n", promptLabelStyle.Render("Project:"), m.projectInput.View()))

	if m.err != "" {
		sb.WriteString("\n" + promptErrorStyle.Render("Error: "+m.err) + "\n")
	}

	sb.WriteString("\n" + promptHintStyle.Render("tab: next field • enter: confirm • esc: cancel"))

	return sb.String()
}

func RunNewPrompt() (NewPromptResult, error) {
	m := newNewPromptModel()
	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return NewPromptResult{Abort: true}, err
	}

	result := finalModel.(newPromptModel).result
	return result, nil
}

// RunNewWorkspacePrompt runs the full new workspace prompt flow:
// 1. Template selection
// 2. Owner/project input
// 3. Variable prompting (if template has variables)
//
// If templates is empty, skips template selection.
// If codeRoot is provided, used for builtin variable resolution.
func RunNewWorkspacePrompt(templates []template.TemplateInfo, templatesDir, codeRoot string) (NewWorkspacePromptResult, error) {
	result := NewWorkspacePromptResult{
		Variables: make(map[string]string),
	}

	// Step 1: Template selection (if templates available)
	if len(templates) > 0 {
		tmplResult, err := RunTemplateSelect(templates)
		if err != nil {
			return NewWorkspacePromptResult{Abort: true}, err
		}
		if tmplResult.Abort {
			return NewWorkspacePromptResult{Abort: true}, nil
		}
		result.TemplateName = tmplResult.Selected
	}

	// Step 2: Owner/project prompt
	ownerProjectResult, err := RunNewPrompt()
	if err != nil {
		return NewWorkspacePromptResult{Abort: true}, err
	}
	if ownerProjectResult.Abort {
		return NewWorkspacePromptResult{Abort: true}, nil
	}
	result.Owner = ownerProjectResult.Owner
	result.Project = ownerProjectResult.Project

	// Step 3: Variable prompting (if template selected and has variables)
	if result.TemplateName != "" && templatesDir != "" {
		tmpl, err := template.LoadTemplate(templatesDir, result.TemplateName)
		if err != nil {
			return NewWorkspacePromptResult{Abort: true}, fmt.Errorf("failed to load template: %w", err)
		}

		// Get builtin variables
		workspacePath := ""
		if codeRoot != "" {
			slug := result.Owner + "--" + result.Project
			workspacePath = codeRoot + "/" + slug
		}
		builtins := template.GetBuiltinVariables(result.Owner, result.Project, workspacePath, codeRoot)

		// Only prompt for variables that need input
		if len(tmpl.Variables) > 0 {
			varResult, err := RunVariablePrompt(tmpl.Variables, builtins)
			if err != nil {
				return NewWorkspacePromptResult{Abort: true}, err
			}
			if varResult.Abort {
				return NewWorkspacePromptResult{Abort: true}, nil
			}
			result.Variables = varResult.Variables
		} else {
			result.Variables = builtins
		}
	}

	return result, nil
}
