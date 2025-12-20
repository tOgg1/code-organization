package tui

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// RepoSelectResult holds the result of repo selection.
type RepoSelectResult struct {
	Selected string // The selected repo name
	Path     string // Full path to the selected repo
	Abort    bool
}

// repoItem is a list item for repo selection.
type repoItem struct {
	name string
	path string
}

func (i repoItem) Title() string       { return i.name }
func (i repoItem) Description() string { return i.path }
func (i repoItem) FilterValue() string { return i.name }

type repoSelectModel struct {
	list          list.Model
	workspacePath string
	done          bool
	result        RepoSelectResult
}

func newRepoSelectModel(repos []string, workspacePath string) repoSelectModel {
	items := make([]list.Item, 0, len(repos))
	for _, repo := range repos {
		repoPath := filepath.Join(workspacePath, "repos", repo)
		items = append(items, repoItem{
			name: repo,
			path: repoPath,
		})
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(lipgloss.Color("212"))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(lipgloss.Color("241"))

	l := list.New(items, delegate, 60, 15)
	l.Title = "Select Repository"
	l.Styles.Title = headerStyle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)

	return repoSelectModel{
		list:          l,
		workspacePath: workspacePath,
	}
}

func (m repoSelectModel) Init() tea.Cmd {
	return nil
}

func (m repoSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			m.result.Abort = true
			m.done = true
			return m, tea.Quit

		case "enter":
			if item, ok := m.list.SelectedItem().(repoItem); ok {
				m.result.Selected = item.name
				m.result.Path = item.path
				m.done = true
				return m, tea.Quit
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m repoSelectModel) View() string {
	return m.list.View() + "\n" + promptHintStyle.Render("enter: select • /: search • esc: cancel")
}

// RunRepoSelect runs the repo selection TUI and returns the selected repo.
func RunRepoSelect(repos []string, workspacePath string) (RepoSelectResult, error) {
	if len(repos) == 0 {
		return RepoSelectResult{Abort: true}, fmt.Errorf("no repositories found in workspace")
	}

	// Use stderr for rendering so stdout stays clean for path output in $(co cd -r)
	// Also configure lipgloss to detect colors from stderr, not stdout
	lipgloss.SetDefaultRenderer(lipgloss.NewRenderer(os.Stderr, termenv.WithColorCache(true)))

	m := newRepoSelectModel(repos, workspacePath)
	p := tea.NewProgram(m, tea.WithOutput(os.Stderr))

	finalModel, err := p.Run()
	if err != nil {
		return RepoSelectResult{Abort: true}, err
	}

	result := finalModel.(repoSelectModel).result
	return result, nil
}
