package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/model"
)

var (
	titleStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	itemStyle         = lipgloss.NewStyle().PaddingLeft(2)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("212"))
	paneStyle         = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("63")).
				Padding(1)
	activePaneStyle   = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("212")).
				Padding(1)
	helpStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	headerStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).MarginBottom(1)
)

type workspaceItem struct {
	record *model.IndexRecord
}

func (i workspaceItem) Title() string       { return i.record.Slug }
func (i workspaceItem) Description() string {
	dirty := ""
	if i.record.DirtyRepos > 0 {
		dirty = fmt.Sprintf(" [%d dirty]", i.record.DirtyRepos)
	}
	return fmt.Sprintf("%s • %d repos%s", i.record.State, i.record.RepoCount, dirty)
}
func (i workspaceItem) FilterValue() string { return i.record.Slug + " " + i.record.Owner }

type keyMap struct {
	Open    key.Binding
	Shell   key.Binding
	Archive key.Binding
	Sync    key.Binding
	Reindex key.Binding
	Quit    key.Binding
}

var keys = keyMap{
	Open:    key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open in editor")),
	Shell:   key.NewBinding(key.WithKeys("enter", "c"), key.WithHelp("enter/c", "shell")),
	Archive: key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "archive")),
	Sync:    key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sync")),
	Reindex: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reindex")),
	Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}

type Model struct {
	cfg      *config.Config
	list     list.Model
	records  []*model.IndexRecord
	selected *model.IndexRecord
	width    int
	height   int
	message  string
}

func New(cfg *config.Config, records []*model.IndexRecord) Model {
	items := make([]list.Item, len(records))
	for i, r := range records {
		items[i] = workspaceItem{record: r}
	}

	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, 40, 20)
	l.Title = "Workspaces"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)

	var selected *model.IndexRecord
	if len(records) > 0 {
		selected = records[0]
	}

	return Model{
		cfg:      cfg,
		list:     l,
		records:  records,
		selected: selected,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width/2-4, msg.Height-6)

	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, keys.Shell):
			if m.selected != nil {
				return m, m.openShell()
			}

		case key.Matches(msg, keys.Open):
			if m.selected != nil {
				return m, m.openWorkspace()
			}

		case key.Matches(msg, keys.Reindex):
			return m, m.reindex()
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	if i, ok := m.list.SelectedItem().(workspaceItem); ok {
		m.selected = i.record
	}

	return m, cmd
}

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	leftPane := activePaneStyle.Width(m.width/2 - 2).Height(m.height - 6).Render(m.list.View())
	rightPane := paneStyle.Width(m.width/2 - 2).Height(m.height - 6).Render(m.detailsView())

	main := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
	help := helpStyle.Render("enter/c: shell • o: editor • a: archive • s: sync • r: reindex • /: search • q: quit")

	if m.message != "" {
		help = m.message
	}

	return lipgloss.JoinVertical(lipgloss.Left, main, help)
}

func (m Model) detailsView() string {
	if m.selected == nil {
		return "No workspace selected"
	}

	r := m.selected
	var sb strings.Builder

	sb.WriteString(titleStyle.Render(r.Slug) + "\n\n")
	sb.WriteString(fmt.Sprintf("Owner:  %s\n", r.Owner))
	sb.WriteString(fmt.Sprintf("State:  %s\n", r.State))
	sb.WriteString(fmt.Sprintf("Path:   %s\n", r.Path))
	sb.WriteString(fmt.Sprintf("Repos:  %d\n", r.RepoCount))
	sb.WriteString(fmt.Sprintf("Dirty:  %d\n", r.DirtyRepos))
	sb.WriteString(fmt.Sprintf("Size:   %s\n", formatBytes(r.SizeBytes)))

	if len(r.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("Tags:   %v\n", r.Tags))
	}

	if r.LastCommitAt != nil {
		sb.WriteString(fmt.Sprintf("\nLast commit: %s\n", r.LastCommitAt.Format("2006-01-02 15:04")))
	}

	if len(r.Repos) > 0 {
		sb.WriteString("\nRepositories:\n")
		for _, repo := range r.Repos {
			dirty := ""
			if repo.Dirty {
				dirty = " [dirty]"
			}
			sb.WriteString(fmt.Sprintf("  • %s (%s)%s\n", repo.Name, repo.Branch, dirty))
		}
	}

	return sb.String()
}

func (m Model) openShell() tea.Cmd {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	cmd := exec.Command(shell)
	cmd.Dir = m.selected.Path
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return nil
	})
}

func (m Model) openWorkspace() tea.Cmd {
	return func() tea.Msg {
		if m.cfg.Editor != "" {
			cmd := exec.Command(m.cfg.Editor, m.selected.Path)
			cmd.Start()
		} else {
			exec.Command("open", m.selected.Path).Start()
		}
		return nil
	}
}

func (m Model) reindex() tea.Cmd {
	return tea.ExecProcess(exec.Command(os.Args[0], "index"), nil)
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func Run(cfg *config.Config) error {
	idx, err := model.LoadIndex(cfg.IndexPath())
	if err != nil {
		return fmt.Errorf("failed to load index (run 'co index' first): %w", err)
	}

	m := New(cfg, idx.Records)
	p := tea.NewProgram(m, tea.WithAltScreen())

	_, err = p.Run()
	return err
}
