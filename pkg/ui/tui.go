package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/tree"
	"sprout/pkg/config"
	"sprout/pkg/git"
	"sprout/pkg/linear"
)

type model struct {
	textInput         textinput.Model
	subtaskInput      textinput.Model
	submitted         bool
	creating          bool
	done              bool
	success           bool
	cancelled         bool
	errorMsg          string
	result            string
	worktreePath      string
	worktreeManager   git.WorktreeManagerInterface
	linearClient      *linear.Client
	linearIssues      []linear.Issue
	flattenedIssues   []linear.Issue // flattened view for navigation
	linearLoading     bool
	linearError       string
	selectedIndex     int  // -1 for custom input, 0+ for Linear ticket index
	inputMode         bool // true when in custom input mode, false when selecting tickets
	creatingSubtask   bool // true while creating subtask
	subtaskInputMode  bool // true when editing subtask inline
	subtaskParentID   string // ID of parent issue when creating subtask
}

var (
	// Base colors - subtle and minimalist
	primaryColor   = lipgloss.Color("69")   // Blue
	secondaryColor = lipgloss.Color("243")  // Gray
	accentColor    = lipgloss.Color("108")  // Green
	errorColor     = lipgloss.Color("204")  // Red
	warningColor   = lipgloss.Color("221")  // Yellow
	
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

	var linearClient *linear.Client
	linearLoading := false
	if cfg.LinearAPIKey != "" {
		linearClient = linear.NewClient(cfg.LinearAPIKey)
		linearLoading = true // We'll start loading immediately in Init
	}

	// Initialize main text input
	ti := textinput.New()
	ti.Placeholder = "enter branch name or select suggestion below"
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 80
	ti.Prompt = "> "
	
	// Style the text input
	ti.PromptStyle = selectedStyle  // Use selected style when focused
	ti.TextStyle = titleStyle
	ti.PlaceholderStyle = helpStyle
	ti.CursorStyle = cursorStyle

	// Initialize subtask text input
	si := textinput.New()
	si.Placeholder = "enter subtask title"
	si.CharLimit = 100
	si.Width = 50
	si.Prompt = ""  // No prompt for inline editing
	
	// Style the subtask input
	si.TextStyle = titleStyle
	si.PlaceholderStyle = helpStyle
	si.CursorStyle = cursorStyle

	return model{
		textInput:         ti,
		subtaskInput:      si,
		submitted:         false,
		creating:          false,
		done:              false,
		success:           false,
		cancelled:         false,
		errorMsg:          "",
		result:            "",
		worktreePath:      "",
		worktreeManager:   wm,
		linearClient:      linearClient,
		linearIssues:      nil,
		flattenedIssues:   nil,
		linearLoading:     linearLoading,
		linearError:       "",
		selectedIndex:     -1, // Start with custom input selected
		inputMode:         true,
		creatingSubtask:   false,
		subtaskInputMode:  false,
		subtaskParentID:   "",
	}, nil
}

func (m model) Init() tea.Cmd {
	var cmds []tea.Cmd
	
	// Start with text input focus
	cmds = append(cmds, textinput.Blink)
	
	// Fetch Linear issues if client is available
	if m.linearClient != nil {
		cmds = append(cmds, m.fetchLinearIssues())
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
		if m.done {
			return m, tea.Quit
		}

		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			// Check if we're in subtask input mode and exit that
			if m.subtaskInputMode {
				m.subtaskInputMode = false
				m.subtaskParentID = ""
				m.subtaskInput.SetValue("")
				m.subtaskInput.Blur()
				return m, nil
			}
			
			m.cancelled = true
			return m, tea.Quit

		case tea.KeyEnter:
			if !m.submitted {
				// Check if we're in subtask input mode
				if m.subtaskInputMode {
					// Creating a new subtask
					title := strings.TrimSpace(m.subtaskInput.Value())
					if title == "" {
						return m, nil // Don't submit empty subtask title
					}
					m.creatingSubtask = true
					m.subtaskInputMode = false
					m.subtaskInput.Blur()
					return m, m.createSubtaskInline(m.subtaskParentID, title)
				}
				
				// Regular worktree creation logic
				var branchName string
				if m.selectedIndex == -1 {
					// Using custom input
					if strings.TrimSpace(m.textInput.Value()) == "" {
						return m, nil // Don't submit empty input
					}
					branchName = strings.TrimSpace(m.textInput.Value())
				} else {
					// Using selected Linear ticket
					if m.selectedIndex < len(m.flattenedIssues) {
						selectedIssue := m.flattenedIssues[m.selectedIndex]
						
						// Don't create worktree for "Add subtask" placeholders that aren't being edited
						if selectedIssue.IsAddSubtask {
							return m, nil
						}
						
						branchName = selectedIssue.GetBranchName()
					} else {
						return m, nil // Invalid selection
					}
				}
				
				m.submitted = true
				m.creating = true
				m.textInput.SetValue(branchName) // Set the input to the selected branch name
				return m, m.createWorktree()
			}

		case tea.KeyUp:
			if !m.submitted {
				if m.selectedIndex == -1 {
					// Already at custom input, do nothing or go to last ticket
					if len(m.flattenedIssues) > 0 {
						m.selectedIndex = len(m.flattenedIssues) - 1
						m.inputMode = false
						m.textInput.Blur()
					}
				} else if m.selectedIndex > 0 {
					m.selectedIndex--
				} else {
					// Go back to custom input
					m.selectedIndex = -1
					m.inputMode = true
					m.textInput.Focus()
				}
			}
			return m, nil

		case tea.KeyDown:
			if !m.submitted {
				if m.selectedIndex == -1 && len(m.flattenedIssues) > 0 {
					// Move from custom input to first ticket
					m.selectedIndex = 0
					m.inputMode = false
					m.textInput.Blur()
				} else if m.selectedIndex >= 0 && m.selectedIndex < len(m.flattenedIssues)-1 {
					m.selectedIndex++
				} else if m.selectedIndex == len(m.flattenedIssues)-1 {
					// Go back to custom input
					m.selectedIndex = -1
					m.inputMode = true
					m.textInput.Focus()
				}
			}
			return m, nil

		case tea.KeyRight:
			if !m.inputMode && !m.submitted && m.selectedIndex >= 0 && m.selectedIndex < len(m.flattenedIssues) {
				selectedIssue := &m.flattenedIssues[m.selectedIndex]
				
				if selectedIssue.IsAddSubtask {
					// Start subtask input mode
					m.subtaskInputMode = true
					m.subtaskParentID = selectedIssue.SubtaskParentID
					m.subtaskInput.SetValue("")
					m.subtaskInput.Focus()
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
			if !m.inputMode && !m.submitted && m.selectedIndex >= 0 && m.selectedIndex < len(m.flattenedIssues) {
				selectedIssue := &m.flattenedIssues[m.selectedIndex]
				
				if selectedIssue.IsAddSubtask {
					// For add subtask items, collapse their parent instead
					m.updateIssueExpansion(selectedIssue.SubtaskParentID, false)
					m.flattenIssues()
					// Move selection to the parent
					for i, issue := range m.flattenedIssues {
						if issue.ID == selectedIssue.SubtaskParentID {
							m.selectedIndex = i
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
		m.creating = false
		m.done = true
		m.success = true
		m.result = fmt.Sprintf("Worktree created at: %s", msg.path)
		// Store the path for later execution and quit the TUI
		m.worktreePath = msg.path
		return m, tea.Quit

	case errMsg:
		m.creating = false
		m.done = true
		m.success = false
		m.errorMsg = msg.err.Error()
		return m, tea.Quit

	case linearIssuesLoadedMsg:
		m.linearLoading = false
		m.linearIssues = msg.issues
		m.linearError = ""
		m.flattenIssues()

	case linearErrorMsg:
		m.linearLoading = false
		m.linearError = msg.err.Error()

	case childrenLoadedMsg:
		m.setIssueChildren(msg.parentID, msg.children)
		m.flattenIssues()

	case childrenErrorMsg:
		// Could show error message or silently fail
	
	case subtaskCreatedMsg:
		m.creatingSubtask = false
		
		// Clear subtask input
		m.subtaskInput.SetValue("")
		m.subtaskInput.Blur()
		m.subtaskInputMode = false
		m.subtaskParentID = ""
		
		// Add the newly created subtask to the parent's children and expand
		m.addSubtaskToParent(msg.parentID, msg.subtask)
		m.updateIssueExpansion(msg.parentID, true)
		m.flattenIssues()
		
		// Find and select the newly created subtask
		for i, issue := range m.flattenedIssues {
			if issue.ID == msg.subtask.ID {
				m.selectedIndex = i
				m.inputMode = false
				break
			}
		}
	
	case subtaskErrorMsg:
		m.creatingSubtask = false
		m.done = true
		m.success = false
		m.errorMsg = fmt.Sprintf("Failed to create subtask: %s", msg.err.Error())
		return m, tea.Quit
	}

	// Update text inputs based on current mode
	if m.inputMode {
		m.textInput, cmd = m.textInput.Update(msg)
	} else if m.subtaskInputMode {
		m.subtaskInput, cmd = m.subtaskInput.Update(msg)
	}

	return m, cmd
}

func (m model) createWorktree() tea.Cmd {
	return func() tea.Msg {
		branchName := strings.TrimSpace(m.textInput.Value())
		worktreePath, err := m.worktreeManager.CreateWorktree(branchName)
		if err != nil {
			return errMsg{err}
		}
		return worktreeCreatedMsg{branchName, worktreePath}
	}
}

// flattenIssues creates a flat list of issues for navigation, respecting expanded state
// and including "add subtask" placeholders where appropriate
func (m *model) flattenIssues() {
	m.flattenedIssues = []linear.Issue{}
	
	var flatten func(issues []linear.Issue, depth int)
	flatten = func(issues []linear.Issue, depth int) {
		for _, issue := range issues {
			issue.Depth = depth
			m.flattenedIssues = append(m.flattenedIssues, issue)
			
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
				m.flattenedIssues = append(m.flattenedIssues, addSubtaskPlaceholder)
			}
		}
	}
	
	flatten(m.linearIssues, 0)
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
	update(&m.linearIssues)
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
	setChildren(&m.linearIssues)
}

func (m model) fetchLinearIssues() tea.Cmd {
	return func() tea.Msg {
		issues, err := m.linearClient.GetAssignedIssues()
		if err != nil {
			return linearErrorMsg{err}
		}
		return linearIssuesLoadedMsg{issues}
	}
}

func (m model) fetchChildren(issueID string) tea.Cmd {
	return func() tea.Msg {
		children, err := m.linearClient.GetIssueChildren(issueID)
		if err != nil {
			return childrenErrorMsg{err}
		}
		return childrenLoadedMsg{issueID, children}
	}
}

func (m model) createSubtaskInline(parentID, title string) tea.Cmd {
	return func() tea.Msg {
		subtask, err := m.linearClient.CreateSubtask(parentID, title)
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
	addToParent(&m.linearIssues)
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
	if m.done {
		if m.success {
			return successStyle.Render("✓ "+m.result) + "\n\n" + helpStyle.Render("Press any key to exit.")
		} else {
			return errorStyle.Render("✗ Error: "+m.errorMsg) + "\n\n" + helpStyle.Render("Press any key to exit.")
		}
	}

	if m.creating {
		return loadingStyle.Render("Creating worktree...")
	}
	
	if m.creatingSubtask {
		return loadingStyle.Render("Creating subtask...")
	}

	s := strings.Builder{}
	s.WriteString(headerStyle.Render("🌱 sprout"))
	s.WriteString("\n")
	
	// Input using textinput component - adjust prompt style based on selection
	if m.selectedIndex == -1 {
		// When input is selected, use selected style for prompt
		m.textInput.PromptStyle = selectedStyle
	} else {
		// When input is not selected, use normal style
		m.textInput.PromptStyle = lipgloss.NewStyle().Foreground(primaryColor)
	}
	
	s.WriteString(m.textInput.View())
	s.WriteString("\n")
	
	// Display Linear tickets tree if available
	if m.linearClient != nil {
		if m.linearLoading {
			s.WriteString(loadingStyle.Render("Loading tickets..."))
		} else if m.linearError != "" {
			s.WriteString(errorStyle.Render("Error: " + m.linearError))
		} else if len(m.linearIssues) == 0 {
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
	if len(m.flattenedIssues) == 0 {
		return ""
	}
	
	// Build tree using lipgloss tree library from flattened issues
	// This properly creates nested sub-trees for child issues
	root := tree.Root("").
		ItemStyle(normalStyle).
		EnumeratorStyle(expandedStyle)
	
	// Stack to track parent nodes at each depth level
	nodeStack := []*tree.Tree{root}
	
	for i, issue := range m.flattenedIssues {
		targetDepth := issue.Depth
		
		// Adjust stack to match the target depth
		// If we're going to a deeper level, keep adding to stack
		// If we're going to a shallower level, pop from stack
		if targetDepth < len(nodeStack)-1 {
			// Going shallower - pop nodes until we're at the right level
			nodeStack = nodeStack[:targetDepth+1]
		}
		
		// Create the display content
		var content string
		if issue.IsAddSubtask {
			if m.subtaskInputMode && m.subtaskParentID == issue.SubtaskParentID {
				content = m.subtaskInput.View()
			} else {
				content = addSubtaskStyle.Render("+ Add subtask")
			}
		} else {
			title := issue.Title
			if len(title) > 50 {
				title = title[:47] + "..."
			}
			identifier := identifierStyle.Render(issue.Identifier)
			titleText := titleStyle.Render(title)
			content = fmt.Sprintf("%s  %s", identifier, titleText)
		}
		
		// Apply selection styling if this is the selected item
		if m.selectedIndex == i {
			content = selectedStyle.Render(content)
		} else {
			content = normalStyle.Render(content)
		}
		
		// Add to the appropriate parent in the tree
		currentParent := nodeStack[len(nodeStack)-1]
		newNode := currentParent.Child(content)
		
		// If this is not an "Add subtask" item, it could have children
		// So add it to the stack for potential children
		if !issue.IsAddSubtask {
			// Only add to stack if we need to go deeper
			if targetDepth+1 >= len(nodeStack) {
				nodeStack = append(nodeStack, newNode)
			} else {
				nodeStack[targetDepth+1] = newNode
			}
		}
	}
	
	return root.String()
}

func (m model) renderSubtaskInput(parentID string) string {
	if m.subtaskInputMode && m.subtaskParentID == parentID {
		return m.subtaskInput.View()
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
	if resultModel, ok := finalModel.(model); ok && resultModel.cancelled {
		// User pressed Escape/Ctrl+C, exit cleanly
		return nil
	}

	// After TUI exits, check if we need to execute a default command
	if resultModel, ok := finalModel.(model); ok && resultModel.success && resultModel.worktreePath != "" {
		cfg, err := config.Load()
		if err != nil {
			cfg = config.DefaultConfig()
		}
		
		defaultCmd := cfg.GetDefaultCommand()
		if len(defaultCmd) > 0 {
			// Execute the default command in the worktree directory
			cmd := exec.Command(defaultCmd[0], defaultCmd[1:]...)
			cmd.Dir = resultModel.worktreePath
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