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
					if m.selectedIndex < len(m.linearIssues) {
						branchName = m.linearIssues[m.selectedIndex].GetBranchName()
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
			if m.cursor < len(m.input) && !m.submitted && m.inputMode {
				m.cursor++
			}
			
		case tea.KeyUp:
			if !m.submitted {
				if m.selectedIndex == -1 {
					// Already at custom input, do nothing or go to last ticket
					if len(m.linearIssues) > 0 {
						m.selectedIndex = len(m.linearIssues) - 1
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
				if m.selectedIndex == -1 && len(m.linearIssues) > 0 {
					// Move from custom input to first ticket
					m.selectedIndex = 0
					m.inputMode = false
				} else if m.selectedIndex >= 0 && m.selectedIndex < len(m.linearIssues)-1 {
					m.selectedIndex++
				} else if m.selectedIndex == len(m.linearIssues)-1 {
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
		return m, nil

	case linearErrorMsg:
		m.linearLoading = false
		m.linearError = msg.err.Error()
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

func (m model) fetchLinearIssues() tea.Cmd {
	return func() tea.Msg {
		issues, err := m.linearClient.GetAssignedIssues()
		if err != nil {
			return linearErrorMsg{err}
		}
		return linearIssuesLoadedMsg{issues}
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
	if len(m.linearIssues) > 0 {
		displayIssues := m.linearIssues
		if len(displayIssues) > 5 {
			displayIssues = displayIssues[:5]
		}
		for _, issue := range displayIssues {
			if len(issue.Identifier) > maxLabelLen {
				maxLabelLen = len(issue.Identifier)
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
		} else if len(m.linearIssues) == 0 {
			s.WriteString("   No assigned tickets found\n")
		} else {
			displayIssues := m.linearIssues
			if len(displayIssues) > 5 {
				displayIssues = displayIssues[:5]
			}
			
			for i, issue := range displayIssues {
				truncatedTitle := issue.Title
				if len(truncatedTitle) > 60 {
					truncatedTitle = truncatedTitle[:57] + "..."
				}
				
				// Create aligned format using the same maxLabelLen as the input field
				paddedIdentifier := fmt.Sprintf("%-*s", maxLabelLen, issue.Identifier)
				line := fmt.Sprintf("     %s: %s", paddedIdentifier, truncatedTitle)
				
				if m.selectedIndex == i {
					s.WriteString(selectedStyle.Render(line) + "\n")
				} else {
					s.WriteString(normalStyle.Render(line) + "\n")
				}
			}
			
			if len(m.linearIssues) > 5 {
				s.WriteString(fmt.Sprintf("     ... and %d more\n", len(m.linearIssues)-5))
			}
		}
		s.WriteString("\n")
	}
	
	s.WriteString("Use ↑/↓ to navigate, Enter to create worktree, Esc/Ctrl+C to quit")
	
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