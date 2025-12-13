package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	confirmLabelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	confirmHintStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

type ConfirmResult struct {
	Confirmed bool
	Aborted   bool
}

type confirmModel struct {
	message  string
	selected bool // true = Yes, false = No
	done     bool
	result   ConfirmResult
}

func newConfirmModel(message string) confirmModel {
	return confirmModel{
		message:  message,
		selected: true, // default to Yes
	}
}

func (m confirmModel) Init() tea.Cmd {
	return nil
}

func (m confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.result.Aborted = true
			m.done = true
			return m, tea.Quit

		case "left", "right", "tab", "h", "l":
			m.selected = !m.selected
			return m, nil

		case "y", "Y":
			m.selected = true
			m.result.Confirmed = true
			m.done = true
			return m, tea.Quit

		case "n", "N":
			m.selected = false
			m.result.Confirmed = false
			m.done = true
			return m, tea.Quit

		case "enter":
			m.result.Confirmed = m.selected
			m.done = true
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m confirmModel) View() string {
	var sb strings.Builder

	sb.WriteString(confirmLabelStyle.Render(m.message) + "\n\n")

	yesStyle := lipgloss.NewStyle().Padding(0, 2)
	noStyle := lipgloss.NewStyle().Padding(0, 2)

	if m.selected {
		yesStyle = yesStyle.Background(lipgloss.Color("212")).Foreground(lipgloss.Color("0"))
	} else {
		noStyle = noStyle.Background(lipgloss.Color("212")).Foreground(lipgloss.Color("0"))
	}

	sb.WriteString(fmt.Sprintf("  %s  %s\n", yesStyle.Render("Yes"), noStyle.Render("No")))
	sb.WriteString("\n" + confirmHintStyle.Render("←/→: select • enter: confirm • y/n: quick select • esc: cancel"))

	return sb.String()
}

func RunConfirm(message string) (ConfirmResult, error) {
	m := newConfirmModel(message)
	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return ConfirmResult{Aborted: true}, err
	}

	result := finalModel.(confirmModel).result
	return result, nil
}
