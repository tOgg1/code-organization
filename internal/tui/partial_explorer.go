package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tormodhaugland/co/internal/config"
	"github.com/tormodhaugland/co/internal/partial"
	"github.com/tormodhaugland/co/internal/template"
)

// PartialTab represents the currently active tab.
type PartialTab int

const (
	PartialTabBrowse PartialTab = iota
	PartialTabApply
	PartialTabValidate
)

func (t PartialTab) String() string {
	switch t {
	case PartialTabBrowse:
		return "Browse"
	case PartialTabApply:
		return "Apply"
	case PartialTabValidate:
		return "Validate"
	default:
		return "Unknown"
	}
}

// ApplyFocus represents the focused element in the Apply tab.
type ApplyFocus int

const (
	ApplyFocusTarget ApplyFocus = iota
	ApplyFocusConflict
	ApplyFocusDryRun
	ApplyFocusNoHooks
	ApplyFocusVars
	ApplyFocusApply
)

type partialExplorerKeyMap struct {
	NextTab    key.Binding
	PrevTab    key.Binding
	SwitchPane key.Binding
	Open       key.Binding
	Quit       key.Binding
}

var partialExplorerKeys = partialExplorerKeyMap{
	NextTab:    key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next tab")),
	PrevTab:    key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev tab")),
	SwitchPane: key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("l/right", "switch pane")),
	Open:       key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open in editor")),
	Quit:       key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}

type partialItem struct {
	info partial.PartialInfo
}

func (i partialItem) Title() string { return i.info.Name }
func (i partialItem) Description() string {
	desc := i.info.Description
	if len(desc) > 40 {
		desc = desc[:37] + "..."
	}
	source := filepath.Base(i.info.SourceDir)
	return fmt.Sprintf("%s (%d vars, %d files) | %s", desc, i.info.VarCount, i.info.FileCount, source)
}
func (i partialItem) FilterValue() string {
	return i.info.Name + " " + i.info.Description + " " + strings.Join(i.info.Tags, " ")
}

type dirPickerItem struct {
	name     string
	path     string
	isParent bool
}

func (i dirPickerItem) Title() string       { return i.name }
func (i dirPickerItem) Description() string { return "" }
func (i dirPickerItem) FilterValue() string { return i.name }

type applyResultMsg struct {
	result *partial.ApplyResult
	err    error
}

type varsPromptMsg struct {
	vars  map[string]string
	abort bool
	err   error
}

type validationMsg struct {
	results map[string]error
	err     error
}

type openDirMsg struct {
	path string
	err  error
}

// PartialExplorerModel is the main model for the partial explorer TUI.
type PartialExplorerModel struct {
	cfg        *config.Config
	partials   []partial.PartialInfo
	list       list.Model
	activeTab  PartialTab
	activePane Pane

	selectedInfo    *partial.PartialInfo
	selectedPartial *partial.Partial
	selectedFiles   []partial.FileInfo

	width          int
	height         int
	message        string
	messageIsError bool

	// Apply tab state
	targetInput   textinput.Model
	targetEditing bool
	applyFocus    ApplyFocus
	conflictIdx   int
	dryRun        bool
	noHooks       bool
	applyVars     map[string]string
	applyResult   *partial.ApplyResult
	applyError    string
	applying      bool

	// Validate tab state
	validating       bool
	validationResult map[string]error

	// Directory picker overlay
	dirPickerActive bool
	dirPickerPath   string
	dirPickerList   list.Model
}

// NewPartialExplorer creates a new partial explorer model.
func NewPartialExplorer(cfg *config.Config, partials []partial.PartialInfo) PartialExplorerModel {
	items := make([]list.Item, len(partials))
	for i, p := range partials {
		items[i] = partialItem{info: p}
	}

	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, 40, 20)
	l.Title = "Partials"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)

	ti := textinput.New()
	ti.Placeholder = "."
	ti.CharLimit = 512
	ti.Width = 50
	if wd, err := os.Getwd(); err == nil {
		ti.SetValue(wd)
	}

	m := PartialExplorerModel{
		cfg:              cfg,
		partials:         partials,
		list:             l,
		activeTab:        PartialTabBrowse,
		activePane:       PaneList,
		targetInput:      ti,
		applyFocus:       ApplyFocusTarget,
		applyVars:        map[string]string{},
		validationResult: map[string]error{},
	}

	if len(partials) > 0 {
		m.setSelected(partials[0])
	}

	return m
}

func (m PartialExplorerModel) Init() tea.Cmd {
	return nil
}

func (m PartialExplorerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width/2-4, msg.Height-6)
		if m.dirPickerActive {
			m.dirPickerList.SetSize(msg.Width-6, msg.Height-8)
		}

	case applyResultMsg:
		m.applying = false
		if msg.err != nil {
			m.applyError = msg.err.Error()
			m.applyResult = nil
		} else {
			m.applyError = ""
			m.applyResult = msg.result
		}

	case varsPromptMsg:
		if msg.err != nil {
			m.message = "Variable prompt failed: " + msg.err.Error()
			m.messageIsError = true
			break
		}
		if msg.abort {
			m.message = "Variable prompt cancelled"
			m.messageIsError = false
			break
		}
		m.applyVars = msg.vars
		m.message = fmt.Sprintf("Captured %d variable(s)", len(msg.vars))
		m.messageIsError = false

	case validationMsg:
		m.validating = false
		if msg.err != nil {
			m.message = msg.err.Error()
			m.messageIsError = true
			break
		}
		m.validationResult = msg.results
		m.message = "Validation complete"
		m.messageIsError = false

	case openDirMsg:
		if msg.err != nil {
			m.message = "Open failed: " + msg.err.Error()
			m.messageIsError = true
		} else {
			m.message = "Opened: " + msg.path
			m.messageIsError = false
		}
	}

	if m.dirPickerActive {
		return m.updateDirPicker(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.list.FilterState() != list.Filtering {
			switch {
			case key.Matches(msg, partialExplorerKeys.Quit):
				return m, tea.Quit
			case key.Matches(msg, partialExplorerKeys.NextTab):
				m.activeTab = (m.activeTab + 1) % 3
				m.targetEditing = false
				return m, nil
			case key.Matches(msg, partialExplorerKeys.PrevTab):
				m.activeTab = (m.activeTab + 2) % 3
				m.targetEditing = false
				return m, nil
			case msg.String() == "1":
				m.activeTab = PartialTabBrowse
				m.targetEditing = false
				return m, nil
			case msg.String() == "2":
				m.activeTab = PartialTabApply
				m.targetEditing = false
				return m, nil
			case msg.String() == "3":
				m.activeTab = PartialTabValidate
				m.targetEditing = false
				return m, nil
			}
		}

		if m.activeTab == PartialTabBrowse {
			return m.updateBrowse(msg)
		}

		if m.activeTab == PartialTabApply {
			return m.updateApply(msg)
		}

		if m.activeTab == PartialTabValidate {
			return m.updateValidate(msg)
		}
	}

	return m, nil
}

func (m PartialExplorerModel) updateBrowse(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "l", "right":
		if m.activePane == PaneList {
			m.activePane = PaneDetails
		}
		return m, nil
	case "h", "left":
		if m.activePane == PaneDetails {
			m.activePane = PaneList
		}
		return m, nil
	case "o":
		return m, m.openSelected()
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	m.syncSelectedFromList()
	return m, cmd
}

func (m PartialExplorerModel) updateApply(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.targetEditing {
		switch msg.String() {
		case "enter", "esc":
			m.targetEditing = false
			m.targetInput.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		m.targetInput, cmd = m.targetInput.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "up", "k":
		if m.applyFocus > ApplyFocusTarget {
			m.applyFocus--
		}
		return m, nil
	case "down", "j":
		if m.applyFocus < ApplyFocusApply {
			m.applyFocus++
		}
		return m, nil
	case "enter":
		return m.handleApplyFocusEnter()
	case "t":
		if m.applyFocus == ApplyFocusTarget {
			m.targetEditing = true
			m.targetInput.Focus()
		}
		return m, nil
	case "c":
		if m.applyFocus == ApplyFocusConflict {
			m.cycleConflict()
		}
		return m, nil
	case "d":
		if m.applyFocus == ApplyFocusDryRun {
			m.dryRun = !m.dryRun
		}
		return m, nil
	case "h":
		if m.applyFocus == ApplyFocusNoHooks {
			m.noHooks = !m.noHooks
		}
		return m, nil
	case "v":
		if m.applyFocus == ApplyFocusVars {
			return m, m.promptVariables()
		}
		return m, nil
	case "a":
		if m.applyFocus == ApplyFocusApply {
			return m, m.applySelected()
		}
		return m, nil
	case "b":
		m.openDirPicker()
		return m, nil
	}

	return m, nil
}

func (m PartialExplorerModel) updateValidate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "v":
		return m, m.validateSelected()
	case "V":
		return m, m.validateAll()
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	m.syncSelectedFromList()
	return m, cmd
}

func (m *PartialExplorerModel) syncSelectedFromList() {
	item := m.list.SelectedItem()
	if item == nil {
		return
	}
	info, ok := item.(partialItem)
	if !ok {
		return
	}
	if m.selectedInfo == nil || m.selectedInfo.Name != info.info.Name {
		m.setSelected(info.info)
	}
}

func (m *PartialExplorerModel) setSelected(info partial.PartialInfo) {
	m.selectedInfo = &info
	p, err := partial.LoadPartial(info.Path)
	if err != nil {
		m.message = "Failed to load partial: " + err.Error()
		m.messageIsError = true
		m.selectedPartial = nil
		m.selectedFiles = nil
		return
	}
	m.selectedPartial = p

	files, err := partial.ListPartialFilesWithInfo(info.Path, p.Files, p.GetTemplateExtensions())
	if err != nil {
		m.message = "Failed to list files: " + err.Error()
		m.messageIsError = true
		m.selectedFiles = nil
	} else {
		m.selectedFiles = files
	}

	m.applyVars = map[string]string{}
	m.applyResult = nil
	m.applyError = ""
	m.setDefaultConflict(p)
}

func (m *PartialExplorerModel) setDefaultConflict(p *partial.Partial) {
	defaultStrategy := p.GetConflictStrategy()
	m.conflictIdx = 0
	for i, s := range partial.ValidConflictStrategies {
		if string(s) == defaultStrategy {
			m.conflictIdx = i
			break
		}
	}
}

func (m PartialExplorerModel) handleApplyFocusEnter() (tea.Model, tea.Cmd) {
	switch m.applyFocus {
	case ApplyFocusTarget:
		m.targetEditing = true
		m.targetInput.Focus()
		return m, nil
	case ApplyFocusConflict:
		m.cycleConflict()
		return m, nil
	case ApplyFocusDryRun:
		m.dryRun = !m.dryRun
		return m, nil
	case ApplyFocusNoHooks:
		m.noHooks = !m.noHooks
		return m, nil
	case ApplyFocusVars:
		return m, m.promptVariables()
	case ApplyFocusApply:
		return m, m.applySelected()
	default:
		return m, nil
	}
}

func (m *PartialExplorerModel) cycleConflict() {
	m.conflictIdx++
	if m.conflictIdx >= len(partial.ValidConflictStrategies) {
		m.conflictIdx = 0
	}
}

func (m PartialExplorerModel) currentConflict() string {
	if len(partial.ValidConflictStrategies) == 0 {
		return string(partial.DefaultConflictStrategy)
	}
	return string(partial.ValidConflictStrategies[m.conflictIdx])
}

func (m PartialExplorerModel) promptVariables() tea.Cmd {
	if m.selectedPartial == nil {
		return nil
	}
	targetPath := strings.TrimSpace(m.targetInput.Value())
	if targetPath == "" {
		targetPath = "."
	}

	return func() tea.Msg {
		builtins, err := partial.GetPartialBuiltins(targetPath)
		if err != nil {
			return varsPromptMsg{err: err}
		}
		tmplVars := partialVarsToTemplate(m.selectedPartial.Variables)
		res, err := RunVariablePrompt(tmplVars, builtins)
		if err != nil {
			return varsPromptMsg{err: err}
		}
		if res.Abort {
			return varsPromptMsg{abort: true}
		}
		return varsPromptMsg{vars: filterPartialVars(m.selectedPartial.Variables, res.Variables)}
	}
}

func (m PartialExplorerModel) applySelected() tea.Cmd {
	if m.selectedPartial == nil || m.selectedInfo == nil {
		m.applyError = "No partial selected"
		return nil
	}

	targetPath := strings.TrimSpace(m.targetInput.Value())
	if targetPath == "" {
		targetPath = "."
	}

	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		m.applyError = "Invalid target path: " + err.Error()
		return nil
	}

	opts := partial.ApplyOptions{
		PartialName:      m.selectedInfo.Name,
		TargetPath:       absTarget,
		Variables:        m.applyVars,
		ConflictStrategy: m.currentConflict(),
		DryRun:           m.dryRun,
		NoHooks:          m.noHooks,
	}

	m.applying = true
	m.applyError = ""
	m.applyResult = nil

	return func() tea.Msg {
		result, err := partial.Apply(opts, m.cfg.AllPartialsDirs())
		return applyResultMsg{result: result, err: err}
	}
}

func (m PartialExplorerModel) validateSelected() tea.Cmd {
	if m.selectedInfo == nil {
		return func() tea.Msg {
			return validationMsg{err: fmt.Errorf("no partial selected")}
		}
	}
	m.validating = true
	return func() tea.Msg {
		err := partial.ValidatePartialDir(m.selectedInfo.Path)
		results := map[string]error{m.selectedInfo.Name: err}
		return validationMsg{results: results}
	}
}

func (m PartialExplorerModel) validateAll() tea.Cmd {
	m.validating = true
	return func() tea.Msg {
		results := make(map[string]error, len(m.partials))
		for _, info := range m.partials {
			results[info.Name] = partial.ValidatePartialDir(info.Path)
		}
		return validationMsg{results: results}
	}
}

func (m *PartialExplorerModel) openDirPicker() {
	start := strings.TrimSpace(m.targetInput.Value())
	if start == "" {
		start = "."
	}
	path, err := filepath.Abs(start)
	if err != nil {
		m.message = "Invalid path: " + err.Error()
		m.messageIsError = true
		return
	}

	l, err := buildDirPickerList(path, m.width-6, m.height-8)
	if err != nil {
		m.message = "Failed to open picker: " + err.Error()
		m.messageIsError = true
		return
	}

	m.dirPickerActive = true
	m.dirPickerPath = path
	m.dirPickerList = l
}

func (m PartialExplorerModel) updateDirPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			m.dirPickerActive = false
			return m, nil
		case "s":
			m.targetInput.SetValue(m.dirPickerPath)
			m.dirPickerActive = false
			return m, nil
		case "backspace", "h":
			parent := filepath.Dir(m.dirPickerPath)
			l, err := buildDirPickerList(parent, m.width-6, m.height-8)
			if err != nil {
				m.message = "Failed to open parent: " + err.Error()
				m.messageIsError = true
				return m, nil
			}
			m.dirPickerPath = parent
			m.dirPickerList = l
			return m, nil
		case "enter":
			item := m.dirPickerList.SelectedItem()
			if item == nil {
				return m, nil
			}
			pick := item.(dirPickerItem)
			if pick.isParent {
				l, err := buildDirPickerList(pick.path, m.width-6, m.height-8)
				if err != nil {
					m.message = "Failed to open parent: " + err.Error()
					m.messageIsError = true
					return m, nil
				}
				m.dirPickerPath = pick.path
				m.dirPickerList = l
				return m, nil
			}
			m.targetInput.SetValue(pick.path)
			m.dirPickerActive = false
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.dirPickerList, cmd = m.dirPickerList.Update(msg)
	return m, cmd
}

func (m PartialExplorerModel) openSelected() tea.Cmd {
	if m.selectedInfo == nil {
		return nil
	}
	partialPath := m.selectedInfo.Path

	return func() tea.Msg {
		var cmd *exec.Cmd
		if m.cfg.Editor != "" {
			cmd = exec.Command(m.cfg.Editor, partialPath)
		} else if runtime.GOOS == "darwin" {
			cmd = exec.Command("open", partialPath)
		} else {
			cmd = exec.Command("xdg-open", partialPath)
		}
		err := cmd.Start()
		return openDirMsg{path: partialPath, err: err}
	}
}

func (m PartialExplorerModel) View() string {
	if len(m.partials) == 0 {
		var sb strings.Builder
		sb.WriteString(titleStyle.Render("Partial Explorer") + "\n\n")
		sb.WriteString("No partials were found in the configured directories.\n\n")
		sb.WriteString(helpStyle.Render("Add partials under ~/Code/_system/partials or ~/.config/co/partials"))
		return sb.String()
	}

	var sb strings.Builder
	sb.WriteString(m.renderTabBar())
	sb.WriteString("\n")

	switch m.activeTab {
	case PartialTabBrowse:
		sb.WriteString(m.renderBrowseTab())
	case PartialTabApply:
		sb.WriteString(m.renderApplyTab())
	case PartialTabValidate:
		sb.WriteString(m.renderValidateTab())
	}

	sb.WriteString("\n")
	sb.WriteString(m.renderStatus())
	sb.WriteString("\n")
	sb.WriteString(m.renderHelp())

	if m.dirPickerActive {
		sb.WriteString("\n\n")
		sb.WriteString(m.renderDirPicker())
	}

	return sb.String()
}

func (m PartialExplorerModel) renderTabBar() string {
	tabs := []PartialTab{PartialTabBrowse, PartialTabApply, PartialTabValidate}
	var rendered []string

	for _, tab := range tabs {
		style := tabStyle
		if tab == m.activeTab {
			style = activeTabStyle
		}
		rendered = append(rendered, style.Render(tab.String()))
	}

	return tabBarStyle.Render(strings.Join(rendered, ""))
}

func (m PartialExplorerModel) renderBrowseTab() string {
	paneHeight := m.height - 7
	left := m.list.View()
	leftPane := paneStyle.Width(m.width/2 - 2).Height(paneHeight).Render(left)
	if m.activePane == PaneList {
		leftPane = activePaneStyle.Width(m.width/2 - 2).Height(paneHeight).Render(left)
	}

	rightPane := paneStyle.Width(m.width/2 - 2).Height(paneHeight).Render(m.partialDetailsView())
	if m.activePane == PaneDetails {
		rightPane = activePaneStyle.Width(m.width/2 - 2).Height(paneHeight).Render(m.partialDetailsView())
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
}

func (m PartialExplorerModel) renderApplyTab() string {
	var sb strings.Builder

	if m.selectedPartial == nil {
		sb.WriteString("No partial selected.\n")
		sb.WriteString("Select a partial in the Browse tab.\n")
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("Partial: %s\n\n", m.selectedPartial.Name))

	sb.WriteString(m.renderApplyRow("Target", m.targetInput.View(), m.applyFocus == ApplyFocusTarget))
	sb.WriteString(m.renderApplyRow("Conflict", m.currentConflict(), m.applyFocus == ApplyFocusConflict))
	sb.WriteString(m.renderApplyRow("Dry-run", checkboxValue(m.dryRun), m.applyFocus == ApplyFocusDryRun))
	sb.WriteString(m.renderApplyRow("No-hooks", checkboxValue(m.noHooks), m.applyFocus == ApplyFocusNoHooks))
	sb.WriteString(m.renderApplyRow("Variables", fmt.Sprintf("%d set", len(m.applyVars)), m.applyFocus == ApplyFocusVars))
	sb.WriteString(m.renderApplyRow("Apply", "Press enter to run", m.applyFocus == ApplyFocusApply))

	if m.applying {
		sb.WriteString("\nApplying partial...\n")
	}

	if m.applyError != "" {
		sb.WriteString("\n" + promptErrorStyle.Render("Error: "+m.applyError) + "\n")
	}

	if m.applyResult != nil {
		sb.WriteString("\n" + m.formatApplyResult(m.applyResult) + "\n")
	}

	return sb.String()
}

func (m PartialExplorerModel) renderValidateTab() string {
	var sb strings.Builder

	if m.validating {
		sb.WriteString("Validating partials...\n\n")
	}

	if len(m.validationResult) == 0 {
		sb.WriteString("No validation results yet.\n")
		sb.WriteString("Press 'v' to validate selected or 'V' for all.\n")
		return sb.String()
	}

	names := make([]string, 0, len(m.validationResult))
	for name := range m.validationResult {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		err := m.validationResult[name]
		status := "OK"
		if err != nil {
			status = "ERR"
		}
		line := fmt.Sprintf("%s %s", status, name)
		if err != nil {
			line = fmt.Sprintf("%s: %v", line, err)
		}
		sb.WriteString(line + "\n")
	}

	return sb.String()
}

func (m PartialExplorerModel) partialDetailsView() string {
	if m.selectedPartial == nil {
		return "No partial selected"
	}

	p := m.selectedPartial
	var sb strings.Builder

	sb.WriteString(headerStyle.Render(p.Name) + "\n")
	sb.WriteString(p.Description + "\n")
	if p.Version != "" {
		sb.WriteString("Version: " + p.Version + "\n")
	}
	sb.WriteString("\n")

	if len(p.Variables) > 0 {
		sb.WriteString("Variables:\n")
		for _, v := range p.Variables {
			required := ""
			if v.Required {
				required = " (required)"
			}
			sb.WriteString(fmt.Sprintf("  - %s%s: %s [%s]\n", v.Name, required, v.Description, v.Type))
			if v.Default != nil {
				sb.WriteString(fmt.Sprintf("    Default: %v\n", v.Default))
			}
			if len(v.Choices) > 0 {
				sb.WriteString(fmt.Sprintf("    Choices: %v\n", v.Choices))
			}
			if v.Validation != "" {
				sb.WriteString(fmt.Sprintf("    Validation: %s\n", v.Validation))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Files:\n")
	if len(m.selectedFiles) == 0 {
		sb.WriteString("  (none)\n")
	} else {
		max := 8
		if len(m.selectedFiles) < max {
			max = len(m.selectedFiles)
		}
		for i := 0; i < max; i++ {
			sb.WriteString("  - " + m.selectedFiles[i].RelPath + "\n")
		}
		if len(m.selectedFiles) > max {
			sb.WriteString(fmt.Sprintf("  ... (%d more)\n", len(m.selectedFiles)-max))
		}
	}
	sb.WriteString("\n")

	if p.Hooks.PreApply.Script != "" || p.Hooks.PostApply.Script != "" {
		sb.WriteString("Hooks:\n")
		if p.Hooks.PreApply.Script != "" {
			timeout := p.Hooks.PreApply.Timeout
			if timeout == "" {
				timeout = partial.DefaultHookTimeout
			}
			sb.WriteString(fmt.Sprintf("  - pre_apply: %s (timeout: %s)\n", p.Hooks.PreApply.Script, timeout))
		}
		if p.Hooks.PostApply.Script != "" {
			timeout := p.Hooks.PostApply.Timeout
			if timeout == "" {
				timeout = partial.DefaultHookTimeout
			}
			sb.WriteString(fmt.Sprintf("  - post_apply: %s (timeout: %s)\n", p.Hooks.PostApply.Script, timeout))
		}
		sb.WriteString("\n")
	}

	if len(p.Tags) > 0 {
		sb.WriteString("Tags: " + strings.Join(p.Tags, ", ") + "\n")
	}

	return sb.String()
}

func (m PartialExplorerModel) renderApplyRow(label, value string, focused bool) string {
	style := inputLabelStyle
	valueStyle := lipgloss.NewStyle()
	if focused {
		style = inputFocusedStyle
		valueStyle = selectedStyle
	}

	return fmt.Sprintf("%s %s\n", style.Render(label+":"), valueStyle.Render(value))
}

func (m PartialExplorerModel) formatApplyResult(result *partial.ApplyResult) string {
	var sb strings.Builder
	sb.WriteString("Apply Result\n")
	sb.WriteString(fmt.Sprintf("Target: %s\n", result.TargetPath))

	if len(result.FilesCreated) > 0 {
		sb.WriteString("Created:\n")
		for _, f := range result.FilesCreated {
			sb.WriteString("  + " + f + "\n")
		}
	}

	if len(result.FilesOverwritten) > 0 {
		sb.WriteString("Overwritten:\n")
		for _, f := range result.FilesOverwritten {
			sb.WriteString("  ~ " + f + "\n")
		}
	}

	if len(result.FilesBackedUp) > 0 {
		sb.WriteString("Backed up:\n")
		for _, f := range result.FilesBackedUp {
			sb.WriteString("  ! " + f + "\n")
		}
	}

	if len(result.FilesMerged) > 0 {
		sb.WriteString("Merged:\n")
		for _, f := range result.FilesMerged {
			sb.WriteString("  M " + f + "\n")
		}
	}

	if len(result.FilesSkipped) > 0 {
		sb.WriteString("Skipped:\n")
		for _, f := range result.FilesSkipped {
			sb.WriteString("  - " + f + "\n")
		}
	}

	if len(result.HooksRun) > 0 {
		sb.WriteString("Hooks run: " + strings.Join(result.HooksRun, ", ") + "\n")
	}
	if len(result.HooksSkipped) > 0 {
		sb.WriteString("Hooks skipped: " + strings.Join(result.HooksSkipped, ", ") + "\n")
	}

	if len(result.Warnings) > 0 {
		sb.WriteString("Warnings:\n")
		for _, w := range result.Warnings {
			sb.WriteString("  - " + w + "\n")
		}
	}

	return sb.String()
}

func (m PartialExplorerModel) renderStatus() string {
	if m.message == "" {
		return ""
	}
	if m.messageIsError {
		return promptErrorStyle.Render(m.message)
	}
	return helpStyle.Render(m.message)
}

func (m PartialExplorerModel) renderHelp() string {
	switch m.activeTab {
	case PartialTabBrowse:
		return helpStyle.Render("j/k: navigate | /: search | l/h: switch pane | o: open | tab: next tab | q: quit")
	case PartialTabApply:
		return helpStyle.Render("j/k: focus | enter: edit/confirm | t: edit target | c/d/h/v/a: conflict/dry/no-hooks/vars/apply | b: browse dir | tab: next tab | q: quit")
	case PartialTabValidate:
		return helpStyle.Render("j/k: navigate | v: validate selected | V: validate all | tab: next tab | q: quit")
	default:
		return ""
	}
}

func (m PartialExplorerModel) renderDirPicker() string {
	var sb strings.Builder
	sb.WriteString(promptLabelStyle.Render("Select target directory") + "\n\n")
	sb.WriteString("Current: " + m.dirPickerPath + "\n\n")
	sb.WriteString(m.dirPickerList.View())
	sb.WriteString("\n" + helpStyle.Render("enter: select | s: select current | h/backspace: up | esc: cancel"))
	return sb.String()
}

func checkboxValue(checked bool) string {
	if checked {
		return "[x]"
	}
	return "[ ]"
}

func buildDirPickerList(path string, width, height int) (list.Model, error) {
	if width <= 0 {
		width = 40
	}
	if height <= 0 {
		height = 15
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return list.Model{}, err
	}

	items := make([]list.Item, 0, len(entries)+1)
	parent := filepath.Dir(path)
	if parent != path {
		items = append(items, dirPickerItem{name: "..", path: parent, isParent: true})
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		items = append(items, dirPickerItem{
			name: entry.Name(),
			path: filepath.Join(path, entry.Name()),
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		return strings.ToLower(items[i].(dirPickerItem).name) < strings.ToLower(items[j].(dirPickerItem).name)
	})

	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, width, height)
	l.Title = path
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)
	return l, nil
}

func partialVarsToTemplate(vars []partial.PartialVar) []template.TemplateVar {
	out := make([]template.TemplateVar, 0, len(vars))
	for _, v := range vars {
		out = append(out, template.TemplateVar{
			Name:        v.Name,
			Description: v.Description,
			Type:        v.Type,
			Required:    v.Required,
			Default:     v.Default,
			Validation:  v.Validation,
			Choices:     v.Choices,
		})
	}
	return out
}

func filterPartialVars(vars []partial.PartialVar, values map[string]string) map[string]string {
	filtered := make(map[string]string)
	for _, v := range vars {
		if val, ok := values[v.Name]; ok {
			filtered[v.Name] = val
		}
	}
	return filtered
}

// RunPartialExplorer runs the partial explorer TUI.
func RunPartialExplorer(cfg *config.Config) error {
	partials, err := partial.ListPartials(cfg.AllPartialsDirs())
	if err != nil {
		return fmt.Errorf("loading partials: %w", err)
	}

	m := NewPartialExplorer(cfg, partials)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}
