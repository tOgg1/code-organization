package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tormodhaugland/co/internal/model"
)

var (
	syncPickerTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	syncPickerHelpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

type SyncPickerResult struct {
	Slugs   []string
	Aborted bool
}

type syncPickerItem struct {
	record   *model.IndexRecord
	selected bool
}

func (i syncPickerItem) Title() string {
	if i.selected {
		return "[x] " + i.record.Slug
	}
	return "[ ] " + i.record.Slug
}

func (i syncPickerItem) Description() string {
	dirty := ""
	if i.record.DirtyRepos > 0 {
		dirty = fmt.Sprintf(" • %d dirty", i.record.DirtyRepos)
	}
	return fmt.Sprintf("%d repos%s", i.record.RepoCount, dirty)
}

func (i syncPickerItem) FilterValue() string { return i.record.Slug }

type syncPickerModel struct {
	list     list.Model
	selected map[string]bool
	width    int
	height   int
	done     bool
	result   SyncPickerResult
	message  string
}

type syncPickerKeyMap struct {
	Toggle  key.Binding
	Confirm key.Binding
	Quit    key.Binding
}

var syncPickerKeys = syncPickerKeyMap{
	Toggle:  key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle")),
	Confirm: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "sync")),
	Quit:    key.NewBinding(key.WithKeys("q", "esc", "ctrl+c"), key.WithHelp("q/esc", "cancel")),
}

func newSyncPickerModel(records []*model.IndexRecord) syncPickerModel {
	items := make([]list.Item, len(records))
	for i, r := range records {
		items[i] = syncPickerItem{record: r}
	}

	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, 40, 20)
	l.Title = "Select workspaces to sync"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)

	return syncPickerModel{
		list:     l,
		selected: make(map[string]bool),
		width:    80,
		height:   24,
	}
}

func (m syncPickerModel) Init() tea.Cmd { return nil }

func (m syncPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width-4, msg.Height-6)

	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch {
		case key.Matches(msg, syncPickerKeys.Quit):
			m.done = true
			m.result.Aborted = true
			return m, tea.Quit

		case key.Matches(msg, syncPickerKeys.Toggle):
			if item, ok := m.list.SelectedItem().(syncPickerItem); ok {
				item.selected = !item.selected
				m.selected[item.record.Slug] = item.selected
				m.list.SetItem(m.list.Index(), item)
			}
			return m, nil

		case key.Matches(msg, syncPickerKeys.Confirm):
			m.result.Slugs = m.collectSelections()
			if len(m.result.Slugs) == 0 {
				if item, ok := m.list.SelectedItem().(syncPickerItem); ok {
					m.result.Slugs = []string{item.record.Slug}
				}
			}
			if len(m.result.Slugs) == 0 {
				m.message = "No workspaces selected"
				return m, nil
			}
			m.done = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m syncPickerModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	header := syncPickerTitleStyle.Render(m.list.Title)
	body := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1).
		Width(m.width - 2).
		Height(m.height - 6).
		Render(m.list.View())

	help := syncPickerHelpStyle.Render("space: toggle • enter: sync • /: search • q: cancel")
	if m.message != "" {
		help = syncPickerHelpStyle.Render(m.message)
	}

	return strings.Join([]string{header, body, help}, "\n")
}

func (m syncPickerModel) collectSelections() []string {
	slugs := make([]string, 0, len(m.selected))
	for _, item := range m.list.Items() {
		if s, ok := item.(syncPickerItem); ok && m.selected[s.record.Slug] {
			slugs = append(slugs, s.record.Slug)
		}
	}
	return slugs
}

func RunSyncPicker(records []*model.IndexRecord) (SyncPickerResult, error) {
	sorted := make([]*model.IndexRecord, len(records))
	copy(sorted, records)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Slug < sorted[j].Slug
	})

	m := newSyncPickerModel(sorted)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return SyncPickerResult{}, err
	}

	if model, ok := finalModel.(syncPickerModel); ok {
		return model.result, nil
	}

	return SyncPickerResult{}, fmt.Errorf("unexpected model type")
}
