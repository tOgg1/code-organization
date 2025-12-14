package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tormodhaugland/co/internal/template"
)

// TemplateSelectResult holds the result of template selection.
type TemplateSelectResult struct {
	Selected     string // Empty string means "No template"
	Abort        bool
	TemplateInfo *template.TemplateInfo // nil if no template selected
}

// templateItem is a list item for template selection.
type templateItem struct {
	name        string
	description string
	varCount    int
	repoCount   int
	isNone      bool // true for "No template" option
}

func (i templateItem) Title() string { return i.name }
func (i templateItem) Description() string {
	if i.isNone {
		return i.description
	}
	if i.repoCount > 0 {
		return fmt.Sprintf("%s (%d vars, %d repos)", i.description, i.varCount, i.repoCount)
	}
	if i.varCount > 0 {
		return fmt.Sprintf("%s (%d vars)", i.description, i.varCount)
	}
	return i.description
}
func (i templateItem) FilterValue() string { return i.name + " " + i.description }

type templateSelectModel struct {
	list     list.Model
	selected string
	done     bool
	abort    bool
	result   TemplateSelectResult
}

func newTemplateSelectModel(templates []template.TemplateInfo) templateSelectModel {
	// Build items list with "No template" as first option
	items := make([]list.Item, 0, len(templates)+1)
	items = append(items, templateItem{
		name:        "No template",
		description: "Create an empty workspace",
		isNone:      true,
	})

	for _, t := range templates {
		items = append(items, templateItem{
			name:        t.Name,
			description: t.Description,
			varCount:    t.VarCount,
			repoCount:   t.RepoCount,
		})
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(lipgloss.Color("212"))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(lipgloss.Color("241"))

	l := list.New(items, delegate, 60, 15)
	l.Title = "Select Template"
	l.Styles.Title = headerStyle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)

	return templateSelectModel{
		list: l,
	}
}

func (m templateSelectModel) Init() tea.Cmd {
	return nil
}

func (m templateSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width-4, msg.Height-4)
		return m, nil

	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch msg.String() {
		case "ctrl+c", "esc":
			m.abort = true
			m.result.Abort = true
			m.done = true
			return m, tea.Quit

		case "enter":
			if item, ok := m.list.SelectedItem().(templateItem); ok {
				if item.isNone {
					m.result.Selected = ""
				} else {
					m.result.Selected = item.name
					m.result.TemplateInfo = &template.TemplateInfo{
						Name:        item.name,
						Description: item.description,
						VarCount:    item.varCount,
						RepoCount:   item.repoCount,
					}
				}
				m.done = true
				return m, tea.Quit
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m templateSelectModel) View() string {
	return m.list.View() + "\n" + promptHintStyle.Render("enter: select • /: search • esc: cancel")
}

// RunTemplateSelect runs the template selection TUI and returns the selected template name.
func RunTemplateSelect(templates []template.TemplateInfo) (TemplateSelectResult, error) {
	m := newTemplateSelectModel(templates)
	p := tea.NewProgram(m)

	finalModel, err := p.Run()
	if err != nil {
		return TemplateSelectResult{Abort: true}, err
	}

	result := finalModel.(templateSelectModel).result
	return result, nil
}
