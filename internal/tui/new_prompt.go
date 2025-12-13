package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type NewPromptResult struct {
	Owner   string
	Project string
	Abort   bool
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
