package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/tree"
	"sprout/pkg/config"
	"sprout/pkg/git"
	"sprout/pkg/linear"
)

type model struct {
	TextInput        textinput.Model
	SubtaskInput     textinput.Model
	Spinner          spinner.Model
	Submitted        bool
	Creating         bool
	Done             bool
	Success          bool
	Cancelled        bool
	ErrorMsg         string
	Result           string
	WorktreePath     string
	WorktreeManager  git.WorktreeManagerInterface
	LinearClient     linear.LinearClientInterface
	LinearIssues     []linear.Issue
	FlattenedIssues  []linear.Issue // flattened view for navigation
	LinearLoading    bool
	LinearError      string
	SelectedIndex    int    // -1 for custom input, 0+ for Linear ticket index
	InputMode        bool   // true when in custom input mode, false when selecting tickets
	CreatingSubtask  bool   // true while creating subtask
	SubtaskInputMode bool   // true when editing subtask inline
	SubtaskParentID  string // ID of parent issue when creating subtask
}

var (
	// Base colors - subtle and minimalist
	primaryColor   = lipgloss.Color("69")  // Blue
	secondaryColor = lipgloss.Color("243") // Gray
	accentColor    = lipgloss.Color("108") // Green
	errorColor     = lipgloss.Color("204") // Red
	warningColor   = lipgloss.Color("221") // Yellow

	// Header style
	headerStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			MarginBottom(1)

	// Selected item style - subtle highlight
	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("237")).
			Bold(true)

	// Normal item style
	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	// Tree expansion indicators
	expandedStyle = lipgloss.NewStyle().
			Foreground(secondaryColor)

	// Issue identifier style
	identifierStyle = lipgloss.NewStyle().
			Foreground(primaryColor)

	// Issue title style
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	// Add subtask style
	addSubtaskStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Italic(true)

	// Input cursor style
	cursorStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Background(lipgloss.Color("237"))

	// Status messages
	successStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	// Loading style
	loadingStyle = lipgloss.NewStyle().
			Foreground(warningColor).
			Italic(true)

	// Help text style
	helpStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Italic(true).
			MarginTop(1)
)

func NewTUI() (model, error) {
	wm, err := git.NewWorktreeManager()
	if err != nil {
		return model{}, err
	}
	return NewTUIWithManager(wm)
}

func NewTUIWithManager(wm git.WorktreeManagerInterface) (model, error) {
	// Load config to check for Linear API key
	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	var linearClient linear.LinearClientInterface
	if cfg.LinearAPIKey != "" {
		linearClient = linear.NewClient(cfg.LinearAPIKey)
	}

	return NewTUIWithDependencies(wm, linearClient)
}

func NewTUIWithDependencies(wm git.WorktreeManagerInterface, linearClient linear.LinearClientInterface) (model, error) {

	// Initialize main text input
	ti := textinput.New()
	ti.Placeholder = "enter branch name or select suggestion below"
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 80
	ti.Prompt = "> "

	// Style the text input
	ti.PromptStyle = selectedStyle // Use selected style when focused
	ti.TextStyle = titleStyle
	ti.PlaceholderStyle = helpStyle
	ti.CursorStyle = cursorStyle

	// Initialize subtask text input
	si := textinput.New()
	si.Placeholder = "enter subtask title"
	si.CharLimit = 100
	si.Width = 50
	si.Prompt = "" // No prompt for inline editing

	// Style the subtask input
	si.TextStyle = titleStyle
	si.PlaceholderStyle = helpStyle
	si.CursorStyle = cursorStyle

	// Initialize spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(warningColor)

	return model{
		TextInput:        ti,
		SubtaskInput:     si,
		Spinner:          s,
		Submitted:        false,
		Creating:         false,
		Done:             false,
		Success:          false,
		Cancelled:        false,
		ErrorMsg:         "",
		Result:           "",
		WorktreePath:     "",
		WorktreeManager:  wm,
		LinearClient:     linearClient,
		LinearIssues:     nil,
		FlattenedIssues:  nil,
		LinearLoading:    linearClient != nil, // Start loading if we have a client
		LinearError:      "",
		SelectedIndex:    -1, // Start with custom input selected
		InputMode:        true,
		CreatingSubtask:  false,
		SubtaskInputMode: false,
		SubtaskParentID:  "",
	}, nil
}

func (m model) Init() tea.Cmd {
	var cmds []tea.Cmd

	// Start with text input focus
	cmds = append(cmds, textinput.Blink)

	// Fetch Linear issues if client is available
	if m.LinearClient != nil {
		cmds = append(cmds, m.fetchLinearIssues())
	}

	// Start spinner if we have any loading states
	if m.LinearLoading || m.Creating || m.CreatingSubtask {
		cmds = append(cmds, m.Spinner.Tick)
	}

	if len(cmds) > 0 {
		return tea.Batch(cmds...)
	}
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.Done {
			return m, tea.Quit
		}

		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			// Check if we're in subtask input mode and exit that
			if m.SubtaskInputMode {
				m.SubtaskInputMode = false
				m.SubtaskParentID = ""
				m.SubtaskInput.SetValue("")
				m.SubtaskInput.Blur()
				return m, nil
			}

			m.Cancelled = true
			return m, tea.Quit

		case tea.KeyEnter:
			if !m.Submitted {
				// Check if we're in subtask input mode
				if m.SubtaskInputMode {
					// Creating a new subtask
					title := strings.TrimSpace(m.SubtaskInput.Value())
					if title == "" {
						return m, nil // Don't submit empty subtask title
					}
					m.CreatingSubtask = true
					m.SubtaskInputMode = false
					m.SubtaskInput.Blur()
					return m, tea.Batch(m.createSubtaskInline(m.SubtaskParentID, title), m.Spinner.Tick)
				}

				// Regular worktree creation logic
				var branchName string
				if m.SelectedIndex == -1 {
					// Using custom input
					if strings.TrimSpace(m.TextInput.Value()) == "" {
						return m, nil // Don't submit empty input
					}
					branchName = strings.TrimSpace(m.TextInput.Value())
				} else {
					// Using selected Linear ticket
					if m.SelectedIndex < len(m.FlattenedIssues) {
						selectedIssue := m.FlattenedIssues[m.SelectedIndex]

						// Don't create worktree for "Add subtask" placeholders that aren't being edited
						if selectedIssue.IsAddSubtask {
							return m, nil
						}

						branchName = selectedIssue.GetBranchName()
					} else {
						return m, nil // Invalid selection
					}
				}

				m.Submitted = true
				m.Creating = true
				m.TextInput.SetValue(branchName) // Set the input to the selected branch name
				return m, tea.Batch(m.createWorktree(), m.Spinner.Tick)
			}

		case tea.KeyUp:
			if !m.Submitted {
				if m.SelectedIndex == -1 {
					// Already at custom input, do nothing or go to last ticket
					if len(m.FlattenedIssues) > 0 {
						m.SelectedIndex = len(m.FlattenedIssues) - 1
						m.InputMode = false
						m.TextInput.Blur()
						// Update placeholder with selected issue's branch name
						if m.SelectedIndex < len(m.FlattenedIssues) && !m.FlattenedIssues[m.SelectedIndex].IsAddSubtask {
							m.TextInput.Placeholder = m.FlattenedIssues[m.SelectedIndex].GetBranchName()
						}
					}
				} else if m.SelectedIndex > 0 {
					m.SelectedIndex--
					// Update placeholder with selected issue's branch name
					if m.SelectedIndex < len(m.FlattenedIssues) && !m.FlattenedIssues[m.SelectedIndex].IsAddSubtask {
						m.TextInput.Placeholder = m.FlattenedIssues[m.SelectedIndex].GetBranchName()
					}
				} else {
					// Go back to custom input
					m.SelectedIndex = -1
					m.InputMode = true
					m.TextInput.Focus()
					// Reset placeholder to default
					m.TextInput.Placeholder = "enter branch name or select suggestion below"
				}
			}
			return m, nil

		case tea.KeyDown:
			if !m.Submitted {
				if m.SelectedIndex == -1 && len(m.FlattenedIssues) > 0 {
					// Move from custom input to first ticket
					m.SelectedIndex = 0
					m.InputMode = false
					m.TextInput.Blur()
					// Update placeholder with selected issue's branch name
					if m.SelectedIndex < len(m.FlattenedIssues) && !m.FlattenedIssues[m.SelectedIndex].IsAddSubtask {
						m.TextInput.Placeholder = m.FlattenedIssues[m.SelectedIndex].GetBranchName()
					}
				} else if m.SelectedIndex >= 0 && m.SelectedIndex < len(m.FlattenedIssues)-1 {
					m.SelectedIndex++
					// Update placeholder with selected issue's branch name
					if m.SelectedIndex < len(m.FlattenedIssues) && !m.FlattenedIssues[m.SelectedIndex].IsAddSubtask {
						m.TextInput.Placeholder = m.FlattenedIssues[m.SelectedIndex].GetBranchName()
					}
				} else if m.SelectedIndex == len(m.FlattenedIssues)-1 {
					// Go back to custom input
					m.SelectedIndex = -1
					m.InputMode = true
					m.TextInput.Focus()
					// Reset placeholder to default
					m.TextInput.Placeholder = "enter branch name or select suggestion below"
				}
			}
			return m, nil

		case tea.KeyRight:
			if !m.InputMode && !m.Submitted && m.SelectedIndex >= 0 && m.SelectedIndex < len(m.FlattenedIssues) {
				selectedIssue := &m.FlattenedIssues[m.SelectedIndex]

				if selectedIssue.IsAddSubtask {
					// Start subtask input mode
					m.SubtaskInputMode = true
					m.SubtaskParentID = selectedIssue.SubtaskParentID
					m.SubtaskInput.SetValue("")
					m.SubtaskInput.Focus()
				} else {
					// Always expand - either to show children or the "add subtask" option
					if !selectedIssue.Expanded {
						if selectedIssue.HasChildren && len(selectedIssue.Children) == 0 {
							// Fetch children and expand
							return m, m.fetchChildren(selectedIssue.ID)
						} else {
							// Expand immediately (either shows existing children or just the "add subtask" option)
							m.updateIssueExpansion(selectedIssue.ID, true)
							m.flattenIssues()
						}
					}
					// If already expanded, do nothing (already showing children/add subtask option)
				}
			}
			return m, nil

		case tea.KeyLeft:
			if !m.InputMode && !m.Submitted && m.SelectedIndex >= 0 && m.SelectedIndex < len(m.FlattenedIssues) {
				selectedIssue := &m.FlattenedIssues[m.SelectedIndex]

				if selectedIssue.IsAddSubtask {
					// For add subtask items, collapse their parent instead
					m.updateIssueExpansion(selectedIssue.SubtaskParentID, false)
					m.flattenIssues()
					// Move selection to the parent
					for i, issue := range m.FlattenedIssues {
						if issue.ID == selectedIssue.SubtaskParentID {
							m.SelectedIndex = i
							break
						}
					}
				} else if selectedIssue.Expanded {
					// Always collapse when left arrow is pressed on an expanded issue
					m.updateIssueExpansion(selectedIssue.ID, false)
					m.flattenIssues()
				}
			}
			return m, nil
		}

	case worktreeCreatedMsg:
		m.Creating = false
		m.Done = true
		m.Success = true
		m.Result = fmt.Sprintf("Worktree created at: %s", msg.path)
		// Store the path for later execution and quit the TUI
		m.WorktreePath = msg.path
		return m, tea.Quit

	case errMsg:
		m.Creating = false
		m.Done = true
		m.Success = false
		m.ErrorMsg = msg.err.Error()
		return m, tea.Quit

	case linearIssuesLoadedMsg:
		m.LinearLoading = false
		m.LinearIssues = msg.issues
		m.LinearError = ""
		m.flattenIssues()
		// Update placeholder if a Linear ticket is currently selected
		if m.SelectedIndex >= 0 && m.SelectedIndex < len(m.FlattenedIssues) && !m.FlattenedIssues[m.SelectedIndex].IsAddSubtask {
			m.TextInput.Placeholder = m.FlattenedIssues[m.SelectedIndex].GetBranchName()
		}

	case linearErrorMsg:
		m.LinearLoading = false
		m.LinearError = msg.err.Error()

	case childrenLoadedMsg:
		m.setIssueChildren(msg.parentID, msg.children)
		m.flattenIssues()
		// Update placeholder if a Linear ticket is currently selected
		if m.SelectedIndex >= 0 && m.SelectedIndex < len(m.FlattenedIssues) && !m.FlattenedIssues[m.SelectedIndex].IsAddSubtask {
			m.TextInput.Placeholder = m.FlattenedIssues[m.SelectedIndex].GetBranchName()
		}

	case childrenErrorMsg:
		// Could show error message or silently fail

	case subtaskCreatedMsg:
		m.CreatingSubtask = false

		// Clear subtask input
		m.SubtaskInput.SetValue("")
		m.SubtaskInput.Blur()
		m.SubtaskInputMode = false
		m.SubtaskParentID = ""

		// Add the newly created subtask to the parent's children and expand
		m.addSubtaskToParent(msg.parentID, msg.subtask)
		m.updateIssueExpansion(msg.parentID, true)
		m.flattenIssues()

		// Find and select the newly created subtask
		for i, issue := range m.FlattenedIssues {
			if issue.ID == msg.subtask.ID {
				m.SelectedIndex = i
				m.InputMode = false
				break
			}
		}

	case subtaskErrorMsg:
		m.CreatingSubtask = false
		m.Done = true
		m.Success = false
		m.ErrorMsg = fmt.Sprintf("Failed to create subtask: %s", msg.err.Error())
		return m, tea.Quit
	}

	// Update spinner if any loading state is active
	if m.LinearLoading || m.Creating || m.CreatingSubtask {
		var spinnerCmd tea.Cmd
		m.Spinner, spinnerCmd = m.Spinner.Update(msg)
		if cmd != nil {
			return m, tea.Batch(cmd, spinnerCmd)
		}
		cmd = spinnerCmd
	}

	// Update text inputs based on current mode
	if m.InputMode {
		m.TextInput, cmd = m.TextInput.Update(msg)
	} else if m.SubtaskInputMode {
		m.SubtaskInput, cmd = m.SubtaskInput.Update(msg)
	}

	return m, cmd
}

func (m model) createWorktree() tea.Cmd {
	return func() tea.Msg {
		branchName := strings.TrimSpace(m.TextInput.Value())
		worktreePath, err := m.WorktreeManager.CreateWorktree(branchName)
		if err != nil {
			return errMsg{err}
		}
		return worktreeCreatedMsg{branchName, worktreePath}
	}
}

// flattenIssues creates a flat list of issues for navigation, respecting expanded state
// and including "add subtask" placeholders where appropriate
func (m *model) flattenIssues() {
	m.FlattenedIssues = []linear.Issue{}

	var flatten func(issues []linear.Issue, depth int)
	flatten = func(issues []linear.Issue, depth int) {
		for _, issue := range issues {
			issue.Depth = depth
			m.FlattenedIssues = append(m.FlattenedIssues, issue)

			if issue.Expanded {
				// Add children if they exist
				if len(issue.Children) > 0 {
					flatten(issue.Children, depth+1)
				}

				// Always add an "add subtask" placeholder when expanded
				addSubtaskPlaceholder := linear.Issue{
					ID:              "", // Empty ID indicates this is a placeholder
					Title:           "+ Add subtask",
					Identifier:      "",
					IsAddSubtask:    true,
					SubtaskParentID: issue.ID,
					Depth:           depth + 1,
					EditingTitle:    false,
					TitleInput:      "",
					TitleCursor:     0,
				}
				m.FlattenedIssues = append(m.FlattenedIssues, addSubtaskPlaceholder)
			}
		}
	}

	flatten(m.LinearIssues, 0)
}

// updateIssueExpansion updates the expanded state of an issue recursively
func (m *model) updateIssueExpansion(issueID string, expanded bool) {
	var update func(issues *[]linear.Issue)
	update = func(issues *[]linear.Issue) {
		for i := range *issues {
			if (*issues)[i].ID == issueID {
				(*issues)[i].Expanded = expanded
				return
			}
			if len((*issues)[i].Children) > 0 {
				update(&(*issues)[i].Children)
			}
		}
	}
	update(&m.LinearIssues)
}

// setIssueChildren sets the children for a specific issue
func (m *model) setIssueChildren(parentID string, children []linear.Issue) {
	var setChildren func(issues *[]linear.Issue)
	setChildren = func(issues *[]linear.Issue) {
		for i := range *issues {
			if (*issues)[i].ID == parentID {
				(*issues)[i].Children = children
				(*issues)[i].Expanded = true
				// Set depth for children
				for j := range (*issues)[i].Children {
					(*issues)[i].Children[j].Depth = (*issues)[i].Depth + 1
				}
				return
			}
			if len((*issues)[i].Children) > 0 {
				setChildren(&(*issues)[i].Children)
			}
		}
	}
	setChildren(&m.LinearIssues)
}

func (m model) fetchLinearIssues() tea.Cmd {
	return func() tea.Msg {
		issues, err := m.LinearClient.GetAssignedIssues()
		if err != nil {
			return linearErrorMsg{err}
		}
		return linearIssuesLoadedMsg{issues}
	}
}

func (m model) fetchChildren(issueID string) tea.Cmd {
	return func() tea.Msg {
		children, err := m.LinearClient.GetIssueChildren(issueID)
		if err != nil {
			return childrenErrorMsg{err}
		}
		return childrenLoadedMsg{issueID, children}
	}
}

func (m model) createSubtaskInline(parentID, title string) tea.Cmd {
	return func() tea.Msg {
		subtask, err := m.LinearClient.CreateSubtask(parentID, title)
		if err != nil {
			return subtaskErrorMsg{err}
		}
		return subtaskCreatedMsg{parentID, *subtask}
	}
}

// addSubtaskToParent adds a newly created subtask to its parent's children
func (m *model) addSubtaskToParent(parentID string, subtask linear.Issue) {
	var addToParent func(issues *[]linear.Issue)
	addToParent = func(issues *[]linear.Issue) {
		for i := range *issues {
			if (*issues)[i].ID == parentID {
				// Add the new subtask as a child
				subtask.Depth = (*issues)[i].Depth + 1
				if (*issues)[i].Children == nil {
					(*issues)[i].Children = []linear.Issue{}
				}
				(*issues)[i].Children = append((*issues)[i].Children, subtask)
				(*issues)[i].HasChildren = true
				return
			}
			if len((*issues)[i].Children) > 0 {
				addToParent(&(*issues)[i].Children)
			}
		}
	}
	addToParent(&m.LinearIssues)
}

type errMsg struct {
	err error
}

type worktreeCreatedMsg struct {
	branch string
	path   string
}

type linearIssuesLoadedMsg struct {
	issues []linear.Issue
}

type linearErrorMsg struct {
	err error
}

type childrenLoadedMsg struct {
	parentID string
	children []linear.Issue
}

type childrenErrorMsg struct {
	err error
}

type subtaskCreatedMsg struct {
	parentID string
	subtask  linear.Issue
}

type subtaskErrorMsg struct {
	err error
}

func (m model) View() string {
	if m.Done {
		if m.Success {
			return successStyle.Render("✓ "+m.Result) + "\n\n" + helpStyle.Render("Press any key to exit.")
		} else {
			return errorStyle.Render("✗ Error: "+m.ErrorMsg) + "\n\n" + helpStyle.Render("Press any key to exit.")
		}
	}

	if m.Creating {
		return fmt.Sprintf("%s Creating worktree...", m.Spinner.View())
	}

	if m.CreatingSubtask {
		return fmt.Sprintf("%s Creating subtask...", m.Spinner.View())
	}

	s := strings.Builder{}
	s.WriteString(headerStyle.Render("🌱 sprout"))
	s.WriteString("\n")

	// Input using textinput component - adjust prompt style based on selection
	if m.SelectedIndex == -1 {
		// When input is selected, use selected style for prompt
		m.TextInput.PromptStyle = selectedStyle
	} else {
		// When input is not selected, use normal style
		m.TextInput.PromptStyle = lipgloss.NewStyle().Foreground(primaryColor)
	}

	s.WriteString(m.TextInput.View())
	s.WriteString("\n")

	// Display Linear tickets tree if available
	if m.LinearClient != nil {
		if m.LinearLoading {
			s.WriteString(fmt.Sprintf("%s Loading tickets...", m.Spinner.View()))
		} else if m.LinearError != "" {
			s.WriteString(errorStyle.Render("Error: " + m.LinearError))
		} else if len(m.LinearIssues) == 0 {
			s.WriteString(helpStyle.Render("No assigned tickets found"))
		} else {
			treeView := m.buildSimpleLinearTree()
			s.WriteString(treeView)
		}
	}

	return s.String()
}

func (m model) renderTreeLine(identifier, title string, maxLen int, isTree, isAddSubtask bool) string {
	var styledIdentifier string
	if isAddSubtask {
		styledIdentifier = addSubtaskStyle.Render(identifier)
	} else if isTree {
		styledIdentifier = identifierStyle.Render(identifier)
	} else {
		styledIdentifier = identifierStyle.Render(identifier)
	}

	paddedIdentifier := fmt.Sprintf("%-*s", maxLen+10, styledIdentifier) // Extra padding for color codes
	return fmt.Sprintf(" %s  %s", paddedIdentifier, title)
}

func (m model) renderIssueTreeLine(issue linear.Issue, index int, maxLen int) string {
	var displayTitle string

	// Handle inline editing for add subtask placeholders
	if issue.IsAddSubtask && issue.EditingTitle {
		displayTitle = ""
		for j, r := range issue.TitleInput {
			if j == issue.TitleCursor {
				displayTitle += cursorStyle.Render("│")
			}
			displayTitle += titleStyle.Render(string(r))
		}
		if issue.TitleCursor == len(issue.TitleInput) {
			displayTitle += cursorStyle.Render("│")
		}
	} else {
		title := issue.Title
		if len(title) > 50 {
			title = title[:47] + "..."
		}
		if issue.IsAddSubtask {
			displayTitle = addSubtaskStyle.Render(title)
		} else {
			displayTitle = titleStyle.Render(title)
		}
	}

	// Create tree structure
	treePrefix := strings.Repeat("  ", issue.Depth)
	expandIndicator := getTreeIndicator(issue)
	styledIndicator := expandedStyle.Render(expandIndicator)

	identifier := issue.Identifier
	if issue.IsAddSubtask {
		identifier = ""
	}

	identifierWithTree := treePrefix + styledIndicator + identifierStyle.Render(identifier)

	return m.renderTreeLine(identifierWithTree, displayTitle, maxLen, true, issue.IsAddSubtask)
}

func (m model) buildSimpleLinearTree() string {
	if len(m.LinearIssues) == 0 {
		return ""
	}

	// Build tree using lipgloss tree library directly from the tree structure
	root := tree.Root("").
		ItemStyle(normalStyle).
		EnumeratorStyle(expandedStyle)

	// Recursively build the tree
	flatIndex := 0
	for _, issue := range m.LinearIssues {
		m.addIssueNode(root, issue, &flatIndex)
	}

	return root.String()
}

// addIssueNode recursively adds an issue and its children to the tree
func (m model) addIssueNode(parent *tree.Tree, issue linear.Issue, flatIndex *int) {
	// Create the display content
	title := issue.Title
	if len(title) > 50 {
		title = title[:47] + "..."
	}
	identifier := identifierStyle.Render(issue.Identifier)
	titleText := titleStyle.Render(title)
	content := fmt.Sprintf("%s  %s", identifier, titleText)

	// Apply selection styling if this is the selected item
	if m.SelectedIndex == *flatIndex {
		content = selectedStyle.Render(content)
	} else {
		content = normalStyle.Render(content)
	}
	*flatIndex++

	// If expanded and has children or needs to show "Add subtask"
	if issue.Expanded {
		// Create a new tree node with the issue as root
		issueNode := tree.New().Root(content).
			ItemStyle(normalStyle).
			EnumeratorStyle(expandedStyle)
		
		// Add actual children
		for _, child := range issue.Children {
			m.addIssueNode(issueNode, child, flatIndex)
		}

		// Add "Add subtask" placeholder
		var addSubtaskContent string
		if m.SubtaskInputMode && m.SubtaskParentID == issue.ID {
			addSubtaskContent = m.SubtaskInput.View()
		} else {
			addSubtaskContent = addSubtaskStyle.Render("+ Add subtask")
		}

		// Apply selection styling if this is the selected item
		if m.SelectedIndex == *flatIndex {
			addSubtaskContent = selectedStyle.Render(addSubtaskContent)
		} else {
			addSubtaskContent = normalStyle.Render(addSubtaskContent)
		}
		*flatIndex++

		issueNode.Child(addSubtaskContent)
		
		// Add the complete subtree to parent
		parent.Child(issueNode)
	} else {
		// Just add the issue without children
		parent.Child(content)
	}
}

func (m model) renderSubtaskInput(parentID string) string {
	if m.SubtaskInputMode && m.SubtaskParentID == parentID {
		return m.SubtaskInput.View()
	}
	return addSubtaskStyle.Render("+ Add subtask")
}

func getTreeIndicator(issue linear.Issue) string {
	if issue.IsAddSubtask {
		return "├─ "
	} else if issue.HasChildren {
		if issue.Expanded {
			return "▼ "
		} else {
			return "▶ "
		}
	} else {
		return "   "
	}
}

func RunInteractive() error {
	m, err := NewTUI()
	if err != nil {
		return err
	}

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	// Check if user cancelled
	if resultModel, ok := finalModel.(model); ok && resultModel.Cancelled {
		// User pressed Escape/Ctrl+C, exit cleanly
		return nil
	}

	// After TUI exits, check if we need to execute a default command
	if resultModel, ok := finalModel.(model); ok && resultModel.Success && resultModel.WorktreePath != "" {
		cfg, err := config.Load()
		if err != nil {
			cfg = config.DefaultConfig()
		}

		defaultCmd := cfg.GetDefaultCommand()
		if len(defaultCmd) > 0 {
			// Execute the default command in the worktree directory
			cmd := exec.Command(defaultCmd[0], defaultCmd[1:]...)
			cmd.Dir = resultModel.WorktreePath
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			if err := cmd.Run(); err != nil {
				if exitError, ok := err.(*exec.ExitError); ok {
					if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
						os.Exit(status.ExitStatus())
					}
				}
				os.Exit(1)
			}
		}
	}

	return nil
}
