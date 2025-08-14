package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/tree"
	"github.com/lithammer/fuzzysearch/fuzzy"
	
	"sprout/pkg/application/services"
	"sprout/pkg/domain/issue"
	"sprout/pkg/presentation/tui/models"
	"sprout/pkg/shared/logging"
)

// InputComponent handles text input for branch names and search
type InputComponent struct {
	textInput textinput.Model
	focused   bool
	prompt    string
}

// NewInputComponent creates a new input component
func NewInputComponent(projectName string) *InputComponent {
	ti := textinput.New()
	ti.CharLimit = 156
	ti.Width = 80
	
	// Set prompt based on project name
	var prompt string
	if projectName != "" {
		prompt = "> " + projectName + "/"
	} else {
		prompt = "> "
	}
	ti.Prompt = prompt

	// Apply styling
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("69")).Bold(true)
	ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Italic(true)
	ti.CursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("69")).Background(lipgloss.Color("237"))

	return &InputComponent{
		textInput: ti,
		focused:   true,
		prompt:    prompt,
	}
}

// Update handles input updates
func (c *InputComponent) Update(msg tea.Msg, state *models.AppState) (tea.Cmd, error) {
	if !c.shouldHandleInput(state) {
		return nil, nil
	}

	c.textInput.Placeholder = state.GetPlaceholderText()
	
	if c.focused && state.InputFocused {
		c.textInput.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("237")).Bold(true)
	} else {
		c.textInput.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))
	}

	var cmd tea.Cmd
	c.textInput, cmd = c.textInput.Update(msg)
	
	switch state.Mode {
	case models.ModeSearch:
		if c.textInput.Value() != "" && c.textInput.Value()[0:1] == "/" {
			state.SearchQuery = c.textInput.Value()[1:]
		} else {
			state.SearchQuery = c.textInput.Value()
		}
	default:
		state.CustomInput = c.textInput.Value()
	}

	return cmd, nil
}

// View renders the input component
func (c *InputComponent) View(state *models.AppState) string {
	switch state.Mode {
	case models.ModeSearch:
		value := c.textInput.Value()
		if value == "/" && state.SearchQuery == "" {
			style := lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("237"))
			return style.Render("/type to fuzzy search")
		}
		
		searchDisplay := "/" + state.SearchQuery
		if state.SelectedIssue != nil && !state.InputFocused {
			fullDisplay := searchDisplay + " → " + state.SelectedIssue.GetBranchName()
			style := lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("237"))
			return style.Render(fullDisplay)
		}
		
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("237"))
		return style.Render(searchDisplay)
		
	default:
		return c.textInput.View()
	}
}

func (c *InputComponent) Init() tea.Cmd                     { return textinput.Blink }
func (c *InputComponent) Focus()                           { c.focused = true; c.textInput.Focus() }
func (c *InputComponent) Blur()                            { c.focused = false; c.textInput.Blur() }
func (c *InputComponent) IsFocused() bool                   { return c.focused }
func (c *InputComponent) SetValue(value string)            { c.textInput.SetValue(value) }
func (c *InputComponent) GetValue() string                 { return c.textInput.Value() }

func (c *InputComponent) SetWidth(width int) {
	promptWidth := lipgloss.Width(c.textInput.Prompt)
	c.textInput.Width = width - promptWidth - 4
	if c.textInput.Width < 20 {
		c.textInput.Width = 20
	}
}

func (c *InputComponent) shouldHandleInput(state *models.AppState) bool {
	switch state.Mode {
	case models.ModeInput, models.ModeSearch:
		return state.InputFocused
	default:
		return false
	}
}

func (c *InputComponent) Reset() {
	c.textInput.SetValue("")
	c.Focus()
}

// SpinnerComponent displays loading spinners
type SpinnerComponent struct {
	spinner spinner.Model
	focused bool
}

func NewSpinnerComponent() *SpinnerComponent {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("221"))
	
	return &SpinnerComponent{
		spinner: s,
		focused: false,
	}
}

func (c *SpinnerComponent) Update(msg tea.Msg, state *models.AppState) (tea.Cmd, error) {
	if !state.IsLoading() {
		return nil, nil
	}
	
	var cmd tea.Cmd
	c.spinner, cmd = c.spinner.Update(msg)
	return cmd, nil
}

func (c *SpinnerComponent) View(state *models.AppState) string {
	if !state.IsLoading() {
		return ""
	}
	
	var message string
	switch {
	case state.LoadingIssues:
		message = "Loading issues..."
	case state.CreatingWorktree:
		message = "Creating worktree..."
	case state.CreatingSubtask:
		message = "Creating subtask..."
	default:
		message = "Loading..."
	}
	
	return c.spinner.View() + " " + message
}

func (c *SpinnerComponent) Init() tea.Cmd   { return c.spinner.Tick }
func (c *SpinnerComponent) Focus()          { c.focused = true }
func (c *SpinnerComponent) Blur()           { c.focused = false }
func (c *SpinnerComponent) IsFocused() bool { return false }

// StatusComponent displays status messages and results
type StatusComponent struct {
	focused bool
}

func NewStatusComponent() *StatusComponent {
	return &StatusComponent{focused: false}
}

func (c *StatusComponent) Update(msg tea.Msg, state *models.AppState) (tea.Cmd, error) {
	return nil, nil
}

func (c *StatusComponent) View(state *models.AppState) string {
	if state.IsInResultMode() {
		return c.renderResult(state)
	}
	return ""
}

func (c *StatusComponent) Init() tea.Cmd   { return nil }
func (c *StatusComponent) Focus()          { c.focused = true }
func (c *StatusComponent) Blur()           { c.focused = false }
func (c *StatusComponent) IsFocused() bool { return false }

func (c *StatusComponent) renderResult(state *models.AppState) string {
	if state.SuccessMessage != "" {
		successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("108")).Bold(true)
		helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Italic(true)
		result := successStyle.Render("✓ " + state.SuccessMessage)
		result += "\n\n" + helpStyle.Render("Press any key to exit.")
		return result
	}
	
	if state.ErrorMessage != "" {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("204")).Bold(true)
		helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Italic(true)
		result := errorStyle.Render("✗ Error: " + state.ErrorMessage)
		result += "\n\n" + helpStyle.Render("Press any key to exit.")
		return result
	}
	
	return ""
}

// IssueListComponent displays and manages the list of issues
type IssueListComponent struct {
	issueService *services.IssueService
	logger       logging.Logger
	
	selectedIndex      int
	expandedIssues     map[string]bool
	maxIdentifierWidth int
	maxStatusWidth     int
}

func NewIssueListComponent(issueService *services.IssueService, logger logging.Logger) *IssueListComponent {
	return &IssueListComponent{
		issueService:   issueService,
		logger:         logger,
		selectedIndex:  -1,
		expandedIssues: make(map[string]bool),
	}
}

func (c *IssueListComponent) Update(msg tea.Msg, state *models.AppState) (tea.Cmd, error) {
	switch msg := msg.(type) {
	case IssuesLoadedMsg:
		state.SetIssues(msg.Issues)
		c.calculateMaxIdentifierWidth(msg.Issues)
		
	case IssueChildrenLoadedMsg:
		c.addChildrenToIssue(state, msg.ParentID, msg.Children)
		c.expandedIssues[msg.ParentID] = true
		
	case SubtaskCreatedMsg:
		c.addSubtaskToParent(state, msg.ParentID, msg.Subtask)
		
	case tea.KeyMsg:
		return c.handleKeyPress(msg, state)
	}
	
	return nil, nil
}

func (c *IssueListComponent) View(state *models.AppState) string {
	if state.LoadingIssues {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("221")).Italic(true).Render("⚡ Loading issues...")
	}
	
	if !state.HasLinearIntegration() {
		return ""
	}
	
	if !state.HasIssues() {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Italic(true).Render("No assigned issues found")
	}
	
	return c.buildIssueTree(state)
}

func (c *IssueListComponent) Init() tea.Cmd {
	if c.issueService == nil {
		return nil
	}
	
	return func() tea.Msg {
		issues, err := c.issueService.GetAssignedIssues(nil)
		if err != nil {
			return IssueErrorMsg{Error: err}
		}
		return IssuesLoadedMsg{Issues: issues}
	}
}

func (c *IssueListComponent) Focus() {}
func (c *IssueListComponent) Blur()  {}
func (c *IssueListComponent) IsFocused() bool { return false }

func (c *IssueListComponent) handleKeyPress(msg tea.KeyMsg, state *models.AppState) (tea.Cmd, error) {
	if state.Mode != models.ModeIssueSelection && state.Mode != models.ModeSearch {
		return nil, nil
	}
	
	switch msg.Type {
	case tea.KeyUp:
		c.navigateUp(state)
	case tea.KeyDown:
		c.navigateDown(state)
	case tea.KeyRight:
		return c.expandIssue(state)
	case tea.KeyLeft:
		c.collapseIssue(state)
	}
	
	return nil, nil
}

func (c *IssueListComponent) navigateUp(state *models.AppState) {
	issues := state.GetCurrentIssues()
	if len(issues) == 0 {
		return
	}
	
	if state.SelectedIssue == nil {
		state.SelectedIssue = issues[len(issues)-1]
		return
	}
	
	prevIssue := c.findPreviousVisibleIssue(state.SelectedIssue, issues)
	if prevIssue != nil {
		state.SelectedIssue = prevIssue
	} else {
		state.SelectedIssue = nil
		state.SetMode(models.ModeInput)
	}
}

func (c *IssueListComponent) navigateDown(state *models.AppState) {
	issues := state.GetCurrentIssues()
	if len(issues) == 0 {
		return
	}
	
	if state.SelectedIssue == nil {
		state.SelectedIssue = issues[0]
		state.SetMode(models.ModeIssueSelection)
		return
	}
	
	nextIssue := c.findNextVisibleIssue(state.SelectedIssue, issues)
	if nextIssue != nil {
		state.SelectedIssue = nextIssue
	}
}

func (c *IssueListComponent) expandIssue(state *models.AppState) (tea.Cmd, error) {
	if state.SelectedIssue == nil || !state.SelectedIssue.HasChildren {
		return nil, nil
	}
	
	if c.expandedIssues[state.SelectedIssue.ID] {
		return nil, nil
	}
	
	if len(state.SelectedIssue.Children) > 0 {
		c.expandedIssues[state.SelectedIssue.ID] = true
		return nil, nil
	}
	
	return func() tea.Msg {
		children, err := c.issueService.ExpandIssue(nil, state.SelectedIssue.ID)
		if err != nil {
			return IssueErrorMsg{Error: err}
		}
		return IssueChildrenLoadedMsg{
			ParentID: state.SelectedIssue.ID,
			Children: children,
		}
	}, nil
}

func (c *IssueListComponent) collapseIssue(state *models.AppState) {
	if state.SelectedIssue == nil {
		return
	}
	
	if c.expandedIssues[state.SelectedIssue.ID] {
		c.expandedIssues[state.SelectedIssue.ID] = false
	}
}

func (c *IssueListComponent) buildIssueTree(state *models.AppState) string {
	issues := state.GetCurrentIssues()
	if len(issues) == 0 {
		return ""
	}
	
	root := tree.Root("").
		ItemStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("252"))).
		EnumeratorStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("243")))
	
	for _, iss := range issues {
		c.addIssueToTree(root, iss, state)
	}
	
	return root.String()
}

func (c *IssueListComponent) addIssueToTree(parent *tree.Tree, iss *issue.Issue, state *models.AppState) {
	content := c.formatIssueContent(iss, state)
	
	if c.expandedIssues[iss.ID] && len(iss.Children) > 0 {
		issueNode := tree.New().Root(content).
			ItemStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("252"))).
			EnumeratorStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("243")))
		
		for _, child := range iss.Children {
			c.addIssueToTree(issueNode, child, state)
		}
		
		addSubtaskContent := lipgloss.NewStyle().Foreground(lipgloss.Color("108")).Italic(true).Render("+ Add subtask")
		issueNode.Child(addSubtaskContent)
		parent.Child(issueNode)
	} else {
		parent.Child(content)
	}
}

func (c *IssueListComponent) formatIssueContent(iss *issue.Issue, state *models.AppState) string {
	// Format identifier with fixed width
	identifier := lipgloss.NewStyle().Foreground(lipgloss.Color("69")).Render(iss.Identifier)
	identifierWidth := lipgloss.Width(identifier)
	idPadding := c.maxIdentifierWidth - identifierWidth
	paddedIdentifier := identifier + strings.Repeat(" ", idPadding)
	
	// Format status with color and fixed width
	statusStyle := c.getStatusStyle(iss.Status.Name)
	styledStatus := statusStyle.Render(iss.Status.Name)
	statusWidth := lipgloss.Width(styledStatus)
	statusPadding := c.maxStatusWidth - statusWidth
	paddedStatus := styledStatus + strings.Repeat(" ", statusPadding)
	
	// Calculate available width for title
	// Account for: identifier + 2 spaces + status + 2 spaces + some margin
	usedWidth := c.maxIdentifierWidth + 2 + c.maxStatusWidth + 2 + 10
	availableWidth := state.Width - usedWidth
	
	// Format title with truncation if needed
	title := iss.Title
	if availableWidth > 20 && len(title) > availableWidth {
		title = title[:availableWidth-3] + "..."
	}
	titleText := lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Render(title)
	
	// Combine all parts: identifier + status + title
	content := fmt.Sprintf("%s  %s  %s", paddedIdentifier, paddedStatus, titleText)
	
	// Apply selection highlighting
	if state.SelectedIssue != nil && state.SelectedIssue.ID == iss.ID {
		content = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("237")).Bold(true).Render(content)
	}
	
	return content
}

func (c *IssueListComponent) findPreviousVisibleIssue(current *issue.Issue, issues []*issue.Issue) *issue.Issue {
	for i, iss := range issues {
		if iss.ID == current.ID && i > 0 {
			return issues[i-1]
		}
	}
	return nil
}

func (c *IssueListComponent) findNextVisibleIssue(current *issue.Issue, issues []*issue.Issue) *issue.Issue {
	for i, iss := range issues {
		if iss.ID == current.ID && i < len(issues)-1 {
			return issues[i+1]
		}
	}
	return nil
}

func (c *IssueListComponent) calculateMaxIdentifierWidth(issues []*issue.Issue) {
	maxIdWidth := 0
	maxStatusWidth := 0
	
	var calculateWidths func([]*issue.Issue)
	calculateWidths = func(issueList []*issue.Issue) {
		for _, iss := range issueList {
			idWidth := len(iss.Identifier)
			if idWidth > maxIdWidth {
				maxIdWidth = idWidth
			}
			
			statusWidth := len(iss.Status.Name)
			if statusWidth > maxStatusWidth {
				maxStatusWidth = statusWidth
			}
			
			// Check children recursively
			if len(iss.Children) > 0 {
				calculateWidths(iss.Children)
			}
		}
	}
	
	calculateWidths(issues)
	c.maxIdentifierWidth = maxIdWidth
	c.maxStatusWidth = maxStatusWidth
}

// getStatusStyle returns a styled status string based on the status name
func (c *IssueListComponent) getStatusStyle(statusName string) lipgloss.Style {
	switch strings.ToLower(statusName) {
	case "todo", "backlog":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("243")) // Gray
	case "in progress", "started", "in_progress":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("221")) // Yellow
	case "in review", "in_review", "review":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("75"))  // Blue
	case "done", "completed":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("108")) // Green
	case "cancelled", "canceled":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("204")) // Red
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("252")) // Default white
	}
}

func (c *IssueListComponent) addChildrenToIssue(state *models.AppState, parentID string, children []*issue.Issue) {
	var findAndAddChildren func([]*issue.Issue) bool
	findAndAddChildren = func(issues []*issue.Issue) bool {
		for _, iss := range issues {
			if iss.ID == parentID {
				iss.Children = children
				for _, child := range children {
					child.Parent = iss
					child.Depth = iss.Depth + 1
				}
				return true
			}
			if findAndAddChildren(iss.Children) {
				return true
			}
		}
		return false
	}
	
	findAndAddChildren(state.Issues)
}

func (c *IssueListComponent) addSubtaskToParent(state *models.AppState, parentID string, subtask *issue.Issue) {
	var findAndAddSubtask func([]*issue.Issue) bool
	findAndAddSubtask = func(issues []*issue.Issue) bool {
		for _, iss := range issues {
			if iss.ID == parentID {
				subtask.Parent = iss
				subtask.Depth = iss.Depth + 1
				iss.Children = append(iss.Children, subtask)
				iss.HasChildren = true
				c.expandedIssues[parentID] = true
				return true
			}
			if findAndAddSubtask(iss.Children) {
				return true
			}
		}
		return false
	}
	
	findAndAddSubtask(state.Issues)
}

func (c *IssueListComponent) FilterIssues(state *models.AppState, query string) {
	if query == "" {
		state.SetFilteredIssues(state.Issues)
		return
	}
	
	allIssues := c.flattenIssues(state.Issues)
	
	var targets []string
	for _, iss := range allIssues {
		targets = append(targets, strings.ToLower(iss.Identifier+" "+iss.Title))
	}
	
	matches := fuzzy.FindNormalized(strings.ToLower(query), targets)
	
	matchedTargets := make(map[string]bool)
	for _, match := range matches {
		matchedTargets[match] = true
	}
	
	var filtered []*issue.Issue
	for _, iss := range allIssues {
		target := strings.ToLower(iss.Identifier+" "+iss.Title)
		if matchedTargets[target] && iss.Depth == 0 {
			filtered = append(filtered, iss)
		}
	}
	
	state.SetFilteredIssues(filtered)
}

func (c *IssueListComponent) flattenIssues(issues []*issue.Issue) []*issue.Issue {
	var result []*issue.Issue
	
	var flatten func([]*issue.Issue)
	flatten = func(issueList []*issue.Issue) {
		for _, iss := range issueList {
			result = append(result, iss)
			if len(iss.Children) > 0 {
				flatten(iss.Children)
			}
		}
	}
	
	flatten(issues)
	return result
}

// Message types
type IssuesLoadedMsg struct {
	Issues []*issue.Issue
}

type IssueChildrenLoadedMsg struct {
	ParentID string
	Children []*issue.Issue
}

type SubtaskCreatedMsg struct {
	ParentID string
	Subtask  *issue.Issue
}

type IssueErrorMsg struct {
	Error error
}