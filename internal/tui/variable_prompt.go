package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tormodhaugland/co/internal/template"
)

// VariablePromptResult holds the result of variable prompting.
type VariablePromptResult struct {
	Variables map[string]string
	Abort     bool
}

// choiceItem is a list item for choice selection.
type choiceItem struct {
	value string
}

func (i choiceItem) Title() string       { return i.value }
func (i choiceItem) Description() string { return "" }
func (i choiceItem) FilterValue() string { return i.value }

// inputMode represents the current input mode.
type inputMode int

const (
	modeText inputMode = iota
	modeBoolean
	modeChoice
)

type variablePromptModel struct {
	variables    []template.TemplateVar
	currentIndex int
	values       map[string]string
	textInput    textinput.Model
	choiceList   list.Model
	boolValue    bool
	mode         inputMode
	err          string
	done         bool
	abort        bool
}

func newVariablePromptModel(vars []template.TemplateVar, builtins map[string]string) variablePromptModel {
	values := make(map[string]string)

	// Pre-populate with builtins
	for k, v := range builtins {
		values[k] = v
	}

	ti := textinput.New()
	ti.Placeholder = "value"
	ti.CharLimit = 256
	ti.Width = 40

	m := variablePromptModel{
		variables: vars,
		values:    values,
		textInput: ti,
	}

	if len(vars) > 0 {
		m.setupCurrentVar()
	} else {
		m.done = true
	}

	return m
}

func (m *variablePromptModel) setupCurrentVar() {
	if m.currentIndex >= len(m.variables) {
		m.done = true
		return
	}

	v := m.variables[m.currentIndex]

	// Get default value
	defaultVal := ""
	if v.Default != nil {
		defaultVal = fmt.Sprintf("%v", v.Default)
		// Substitute any variable references in default
		if substituted, err := template.SubstituteVariables(defaultVal, m.values); err == nil {
			defaultVal = substituted
		}
	}

	switch v.Type {
	case template.VarTypeBoolean:
		m.mode = modeBoolean
		m.boolValue = defaultVal == "true" || defaultVal == "yes" || defaultVal == "1"

	case template.VarTypeChoice:
		m.mode = modeChoice
		items := make([]list.Item, len(v.Choices))
		selectedIdx := 0
		for i, choice := range v.Choices {
			items[i] = choiceItem{value: choice}
			if choice == defaultVal {
				selectedIdx = i
			}
		}
		delegate := list.NewDefaultDelegate()
		delegate.ShowDescription = false
		delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(lipgloss.Color("212"))

		l := list.New(items, delegate, 40, min(len(v.Choices)+4, 12))
		l.Title = v.Name
		l.Styles.Title = promptLabelStyle
		l.SetShowStatusBar(false)
		l.SetFilteringEnabled(false)
		l.SetShowHelp(false)
		l.Select(selectedIdx)
		m.choiceList = l

	default: // string or integer
		m.mode = modeText
		m.textInput.SetValue(defaultVal)
		m.textInput.Focus()
	}
}

func (m variablePromptModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m variablePromptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.done {
		return m, tea.Quit
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.abort = true
			m.done = true
			return m, tea.Quit
		}

		v := m.variables[m.currentIndex]

		switch m.mode {
		case modeBoolean:
			switch msg.String() {
			case "y", "Y", "t", "1":
				m.boolValue = true
			case "n", "N", "f", "0":
				m.boolValue = false
			case "left", "h":
				m.boolValue = !m.boolValue
			case "right", "l":
				m.boolValue = !m.boolValue
			case "tab", " ":
				m.boolValue = !m.boolValue
			case "enter":
				if m.boolValue {
					m.values[v.Name] = "true"
				} else {
					m.values[v.Name] = "false"
				}
				m.err = ""
				m.currentIndex++
				m.setupCurrentVar()
				if m.done {
					return m, tea.Quit
				}
			}
			return m, nil

		case modeChoice:
			switch msg.String() {
			case "enter":
				if item, ok := m.choiceList.SelectedItem().(choiceItem); ok {
					m.values[v.Name] = item.value
					m.err = ""
					m.currentIndex++
					m.setupCurrentVar()
					if m.done {
						return m, tea.Quit
					}
				}
				return m, nil
			}
			var cmd tea.Cmd
			m.choiceList, cmd = m.choiceList.Update(msg)
			return m, cmd

		default: // text mode
			switch msg.String() {
			case "enter":
				value := strings.TrimSpace(m.textInput.Value())

				// Validate
				if v.Required && value == "" {
					m.err = fmt.Sprintf("%s is required", v.Name)
					return m, nil
				}

				if v.Type == template.VarTypeInteger && value != "" {
					if _, err := strconv.Atoi(value); err != nil {
						m.err = "must be a valid integer"
						return m, nil
					}
				}

				if value != "" {
					if err := template.ValidateVarValue(v, value); err != nil {
						m.err = err.Error()
						return m, nil
					}
				}

				m.values[v.Name] = value
				m.err = ""
				m.textInput.SetValue("")
				m.currentIndex++
				m.setupCurrentVar()
				if m.done {
					return m, tea.Quit
				}
				return m, nil
			}

			var cmd tea.Cmd
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m variablePromptModel) View() string {
	if m.done || m.currentIndex >= len(m.variables) {
		return ""
	}

	v := m.variables[m.currentIndex]
	var sb strings.Builder

	sb.WriteString(promptLabelStyle.Render(fmt.Sprintf("Variable %d/%d", m.currentIndex+1, len(m.variables))) + "\n\n")

	// Show variable name and description
	sb.WriteString(promptLabelStyle.Render(v.Name))
	if v.Required {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(" *"))
	}
	sb.WriteString("\n")

	if v.Description != "" {
		sb.WriteString(promptHintStyle.Render(v.Description) + "\n")
	}
	sb.WriteString("\n")

	// Show input based on mode
	switch m.mode {
	case modeBoolean:
		yes := "  yes  "
		no := "  no  "
		if m.boolValue {
			yes = lipgloss.NewStyle().
				Background(lipgloss.Color("212")).
				Foreground(lipgloss.Color("0")).
				Bold(true).
				Render(" yes ")
		} else {
			no = lipgloss.NewStyle().
				Background(lipgloss.Color("212")).
				Foreground(lipgloss.Color("0")).
				Bold(true).
				Render("  no  ")
		}
		sb.WriteString(fmt.Sprintf("[ %s | %s ]\n", yes, no))
		sb.WriteString("\n" + promptHintStyle.Render("y/n: toggle • tab: toggle • enter: confirm"))

	case modeChoice:
		sb.WriteString(m.choiceList.View())
		sb.WriteString("\n" + promptHintStyle.Render("j/k: move • enter: select • esc: cancel"))

	default:
		sb.WriteString(m.textInput.View() + "\n")
		if v.Type == template.VarTypeInteger {
			sb.WriteString(promptHintStyle.Render("(integer)") + "\n")
		}
		sb.WriteString("\n" + promptHintStyle.Render("enter: confirm • esc: cancel"))
	}

	if m.err != "" {
		sb.WriteString("\n\n" + promptErrorStyle.Render("Error: "+m.err))
	}

	return sb.String()
}

// RunVariablePrompt runs the variable prompting TUI.
func RunVariablePrompt(vars []template.TemplateVar, builtins map[string]string) (VariablePromptResult, error) {
	// Filter out variables that already have values in builtins or have defaults
	promptVars := make([]template.TemplateVar, 0)
	for _, v := range vars {
		// Skip if already in builtins
		if _, ok := builtins[v.Name]; ok {
			continue
		}
		promptVars = append(promptVars, v)
	}

	if len(promptVars) == 0 {
		// No variables to prompt for
		return VariablePromptResult{Variables: builtins}, nil
	}

	m := newVariablePromptModel(promptVars, builtins)
	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return VariablePromptResult{Abort: true}, err
	}

	fm := finalModel.(variablePromptModel)
	if fm.abort {
		return VariablePromptResult{Abort: true}, nil
	}

	return VariablePromptResult{Variables: fm.values}, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
