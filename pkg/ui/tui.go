package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"sprout/pkg/config"
	"sprout/pkg/git"
	"sprout/pkg/linear"
)

type model struct {
	input           string
	cursor          int
	submitted       bool
	creating        bool
	done            bool
	success         bool
	cancelled       bool
	errorMsg        string
	result          string
	worktreePath    string
	worktreeManager *git.WorktreeManager
	linearClient    *linear.Client
	linearIssues    []linear.Issue
	flattenedIssues []linear.Issue // flattened view for navigation
	linearLoading   bool
	linearError     string
	selectedIndex   int  // -1 for custom input, 0+ for Linear ticket index
	inputMode       bool // true when in custom input mode, false when selecting tickets
}

var (
	selectedStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("240")).
		Foreground(lipgloss.Color("15"))
	
	normalStyle = lipgloss.NewStyle()
)

func NewTUI() (model, error) {
	wm, err := git.NewWorktreeManager()
	if err != nil {
		return model{}, err
	}

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

	return model{
		input:           "",
		cursor:          0,
		submitted:       false,
		creating:        false,
		done:            false,
		success:         false,
		cancelled:       false,
		errorMsg:        "",
		result:          "",
		worktreePath:    "",
		worktreeManager: wm,
		linearClient:    linearClient,
		linearIssues:    nil,
		flattenedIssues: nil,
		linearLoading:   linearLoading,
		linearError:     "",
		selectedIndex:   -1, // Start with custom input selected
		inputMode:       true,
	}, nil
}

func (m model) Init() tea.Cmd {
	if m.linearClient != nil {
		return m.fetchLinearIssues()
	}
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.done {
			return m, tea.Quit
		}

		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.cancelled = true
			return m, tea.Quit

		case tea.KeyEnter:
			if !m.submitted {
				var branchName string
				if m.selectedIndex == -1 {
					// Using custom input
					if strings.TrimSpace(m.input) == "" {
						return m, nil // Don't submit empty input
					}
					branchName = strings.TrimSpace(m.input)
				} else {
					// Using selected Linear ticket
					if m.selectedIndex < len(m.flattenedIssues) {
						branchName = m.flattenedIssues[m.selectedIndex].GetBranchName()
					} else {
						return m, nil // Invalid selection
					}
				}
				
				m.submitted = true
				m.creating = true
				m.input = branchName // Set the input to the selected branch name
				return m, m.createWorktree()
			}

		case tea.KeyBackspace:
			if m.cursor > 0 && !m.submitted {
				m.input = m.input[:m.cursor-1] + m.input[m.cursor:]
				m.cursor--
			}

		case tea.KeyLeft:
			if m.cursor > 0 && !m.submitted && m.inputMode {
				m.cursor--
			}

		case tea.KeyRight:
			if m.inputMode && m.cursor < len(m.input) && !m.submitted {
				// Normal cursor movement when in input mode
				m.cursor++
			} else if !m.inputMode && !m.submitted && m.selectedIndex >= 0 && m.selectedIndex < len(m.flattenedIssues) {
				// Expand/collapse sub-issues when in selection mode
				selectedIssue := &m.flattenedIssues[m.selectedIndex]
				if selectedIssue.HasChildren {
					if !selectedIssue.Expanded {
						// Expand: fetch children if not already loaded
						if len(selectedIssue.Children) == 0 {
							return m, m.fetchChildren(selectedIssue.ID)
						} else {
							// Already loaded, just expand
							m.updateIssueExpansion(selectedIssue.ID, true)
							m.flattenIssues()
						}
					} else {
						// Collapse
						m.updateIssueExpansion(selectedIssue.ID, false)
						m.flattenIssues()
					}
				}
			}
			
		case tea.KeyUp:
			if !m.submitted {
				if m.selectedIndex == -1 {
					// Already at custom input, do nothing or go to last ticket
					if len(m.flattenedIssues) > 0 {
						m.selectedIndex = len(m.flattenedIssues) - 1
						m.inputMode = false
					}
				} else if m.selectedIndex > 0 {
					m.selectedIndex--
				} else {
					// Go back to custom input
					m.selectedIndex = -1
					m.inputMode = true
				}
			}

		case tea.KeyDown:
			if !m.submitted {
				if m.selectedIndex == -1 && len(m.flattenedIssues) > 0 {
					// Move from custom input to first ticket
					m.selectedIndex = 0
					m.inputMode = false
				} else if m.selectedIndex >= 0 && m.selectedIndex < len(m.flattenedIssues)-1 {
					m.selectedIndex++
				} else if m.selectedIndex == len(m.flattenedIssues)-1 {
					// Go back to custom input
					m.selectedIndex = -1
					m.inputMode = true
				}
			}

		case tea.KeyRunes:
			if !m.submitted && m.inputMode {
				m.input = m.input[:m.cursor] + string(msg.Runes) + m.input[m.cursor:]
				m.cursor += len(msg.Runes)
			}
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
		return m, nil

	case linearErrorMsg:
		m.linearLoading = false
		m.linearError = msg.err.Error()
		return m, nil

	case childrenLoadedMsg:
		m.setIssueChildren(msg.parentID, msg.children)
		m.flattenIssues()
		return m, nil

	case childrenErrorMsg:
		// Could show error message or silently fail
		return m, nil
	}

	return m, nil
}

func (m model) createWorktree() tea.Cmd {
	return func() tea.Msg {
		branchName := strings.TrimSpace(m.input)
		worktreePath, err := m.worktreeManager.CreateWorktree(branchName)
		if err != nil {
			return errMsg{err}
		}
		return worktreeCreatedMsg{branchName, worktreePath}
	}
}

// flattenIssues creates a flat list of issues for navigation, respecting expanded state
func (m *model) flattenIssues() {
	m.flattenedIssues = []linear.Issue{}
	
	var flatten func(issues []linear.Issue, depth int)
	flatten = func(issues []linear.Issue, depth int) {
		for _, issue := range issues {
			issue.Depth = depth
			m.flattenedIssues = append(m.flattenedIssues, issue)
			
			if issue.Expanded && len(issue.Children) > 0 {
				flatten(issue.Children, depth+1)
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

func (m model) View() string {
	if m.done {
		if m.success {
			return fmt.Sprintf("✅ %s\n\nPress any key to exit.\n", m.result)
		} else {
			return fmt.Sprintf("❌ Error: %s\n\nPress any key to exit.\n", m.errorMsg)
		}
	}

	if m.creating {
		return "Creating worktree...\n"
	}

	s := strings.Builder{}
	s.WriteString("🌱 Sprout - Create New Worktree\n\n")
	
	// Find the longest label for alignment (including both "Branch Name" and Linear identifiers)
	maxLabelLen := len("Branch Name")
	if len(m.flattenedIssues) > 0 {
		displayIssues := m.flattenedIssues
		if len(displayIssues) > 5 {
			displayIssues = displayIssues[:5]
		}
		for _, issue := range displayIssues {
			// Account for indentation in label length calculation
			indentedLen := len(issue.Identifier) + (issue.Depth * 2)
			if indentedLen > maxLabelLen {
				maxLabelLen = indentedLen
			}
		}
	}
	
	// Custom input field
	inputLabel := "Branch Name"
	inputText := ""
	if m.inputMode {
		for i, r := range m.input {
			if i == m.cursor {
				inputText += "│"
			}
			inputText += string(r)
		}
		if m.cursor == len(m.input) {
			inputText += "│"
		}
	} else {
		inputText = m.input
	}
	
	paddedLabel := fmt.Sprintf("%-*s", maxLabelLen, inputLabel)
	inputLine := fmt.Sprintf("     %s: %s", paddedLabel, inputText)
	if m.selectedIndex == -1 {
		s.WriteString(selectedStyle.Render(inputLine) + "\n\n")
	} else {
		s.WriteString(normalStyle.Render(inputLine) + "\n\n")
	}
	
	// Display Linear tickets if available
	if m.linearClient != nil {
		s.WriteString("📋 Linear Tickets (Assigned to You):\n")
		
		if m.linearLoading {
			s.WriteString("   Loading tickets...\n")
		} else if m.linearError != "" {
			s.WriteString(fmt.Sprintf("   Error: %s\n", m.linearError))
		} else if len(m.flattenedIssues) == 0 {
			s.WriteString("   No assigned tickets found\n")
		} else {
			displayIssues := m.flattenedIssues
			if len(displayIssues) > 5 {
				displayIssues = displayIssues[:5]
			}
			
			for i, issue := range displayIssues {
				truncatedTitle := issue.Title
				if len(truncatedTitle) > 60 {
					truncatedTitle = truncatedTitle[:57] + "..."
				}
				
				// Create indentation based on depth
				indent := strings.Repeat("  ", issue.Depth)
				
				// Add expansion indicator for items with children
				expandIndicator := ""
				if issue.HasChildren {
					if issue.Expanded {
						expandIndicator = "▼ "
					} else {
						expandIndicator = "▶ "
					}
				} else {
					expandIndicator = "  " // Same width as indicators
				}
				
				// Create the identifier with indentation
				identifierWithIndent := indent + expandIndicator + issue.Identifier
				paddedIdentifier := fmt.Sprintf("%-*s", maxLabelLen, identifierWithIndent)
				line := fmt.Sprintf("     %s: %s", paddedIdentifier, truncatedTitle)
				
				if m.selectedIndex == i {
					s.WriteString(selectedStyle.Render(line) + "\n")
				} else {
					s.WriteString(normalStyle.Render(line) + "\n")
				}
			}
			
			if len(m.flattenedIssues) > 5 {
				s.WriteString(fmt.Sprintf("     ... and %d more\n", len(m.flattenedIssues)-5))
			}
		}
		s.WriteString("\n")
	}
	
	s.WriteString("Use ↑/↓ to navigate, → to expand/collapse sub-issues, Enter to create worktree, Esc/Ctrl+C to quit")
	
	return s.String()
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