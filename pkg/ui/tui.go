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
	"github.com/lithammer/fuzzysearch/fuzzy"
	"sprout/pkg/config"
	"sprout/pkg/git"
	"sprout/pkg/linear"
)

type model struct {
	TextInput          textinput.Model
	SubtaskInput       textinput.Model
	Spinner            spinner.Model
	Submitted          bool
	Creating           bool
	Done               bool
	Success            bool
	Cancelled          bool
	ErrorMsg           string
	Result             string
	WorktreePath       string
	WorktreeManager    git.WorktreeManagerInterface
	LinearClient       linear.LinearClientInterface
	LinearIssues       []linear.Issue
	LinearLoading      bool
	LinearError        string
	SelectedIssue      *linear.Issue  // nil for custom input mode
	InputMode          bool           // true when in custom input mode, false when selecting tickets
	CreatingSubtask    bool           // true while creating subtask
	SubtaskInputMode   bool           // true when editing subtask inline
	SubtaskParentID    string         // ID of parent issue when creating subtask
	AddSubtaskSelected string         // ID of parent issue whose "Add subtask" is selected
	DefaultPlaceholder string         // The default placeholder text for the input
	SearchMode         bool           // true when in fuzzy search mode (triggered by /)
	SearchQuery        string         // current search query in search mode
	FilteredIssues     []linear.Issue // filtered list of issues based on search
	Width              int            // terminal width
	Height             int            // terminal height
	MaxIdentifierWidth int            // maximum width of issue identifiers for alignment
	MaxStatusWidth     int            // maximum width of issue statuses for alignment
	CreationMode       creationMode   // user-selected creation mode
	ActiveCreationMode creationMode   // creation mode currently executing
}

type creationMode int

const (
	creationModeWorktree creationMode = iota
	creationModeBranchOnly
)

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
			Bold(true)

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

	// Issue status styles - color-coded by status type
	statusBacklogStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")) // Dark gray for backlog/unstarted
	statusTodoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")) // Light gray for todo
	statusInProgressStyle = lipgloss.NewStyle().
				Foreground(warningColor) // Yellow for in progress
	statusInReviewStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("177")) // Purple for in review
	statusDoneStyle = lipgloss.NewStyle().
			Foreground(accentColor) // Green for done/completed
	statusCancelledStyle = lipgloss.NewStyle().
				Foreground(errorColor) // Red for cancelled

	// Issue title style
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	// Issue status style
	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("242"))

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
			Italic(true)
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
	// Get repository name for the prompt
	repoName, err := git.GetRepositoryName()
	if err != nil {
		// Fallback to a generic prompt if we can't get the repo name
		repoName = ""
	}

	var prompt string
	var placeholder string
	if repoName != "" {
		prompt = "> " + repoName + "/"
		placeholder = "enter branch name or select suggestion below"
	} else {
		prompt = "> "
		placeholder = "enter branch name or select suggestion below"
	}

	// Initialize main text input
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 80 // Will be updated in Update() when we get window size
	ti.Prompt = prompt

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
		TextInput:          ti,
		SubtaskInput:       si,
		Spinner:            s,
		Submitted:          false,
		Creating:           false,
		Done:               false,
		Success:            false,
		Cancelled:          false,
		ErrorMsg:           "",
		Result:             "",
		WorktreePath:       "",
		WorktreeManager:    wm,
		LinearClient:       linearClient,
		LinearIssues:       nil,
		LinearLoading:      linearClient != nil, // Start loading if we have a client
		LinearError:        "",
		SelectedIssue:      nil, // Start with custom input selected
		InputMode:          true,
		CreatingSubtask:    false,
		SubtaskInputMode:   false,
		SubtaskParentID:    "",
		AddSubtaskSelected: "",
		DefaultPlaceholder: "enter branch name or select suggestion below",
		SearchMode:         false,
		SearchQuery:        "",
		FilteredIssues:     nil,
		Width:              80, // Default, will be updated when we get window size
		Height:             24, // Default, will be updated when we get window size
		CreationMode:       creationModeWorktree,
		ActiveCreationMode: creationModeWorktree,
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
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height

		// Update text input width to use most of the terminal width
		// Leave some space for the prompt and margins
		promptWidth := lipgloss.Width(m.TextInput.Prompt)
		m.TextInput.Width = m.Width - promptWidth - 4 // 4 chars for margins
		if m.TextInput.Width < 20 {
			m.TextInput.Width = 20 // Minimum width
		}

		return m, nil

	case tea.KeyMsg:
		if m.Done {
			return m, tea.Quit
		}

		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			// Check if we're in search mode and exit that
			if m.SearchMode {
				m.SearchMode = false
				m.SearchQuery = ""
				m.FilteredIssues = nil
				m.TextInput.Placeholder = m.DefaultPlaceholder
				m.TextInput.SetValue("") // Clear the search input
				m.InputMode = true
				m.TextInput.Focus()
				m.SelectedIssue = nil
				return m, nil
			}

			// Check if we're in subtask input mode and exit that
			if m.SubtaskInputMode {
				m.SubtaskInputMode = false
				m.setSubtaskEntryMode(m.SubtaskParentID, false)
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
				if m.SelectedIssue == nil {
					// Check if we're on "Add subtask" selection (which shouldn't create worktree)
					if m.AddSubtaskSelected != "" {
						return m, nil
					}
					// Using custom input
					if strings.TrimSpace(m.TextInput.Value()) == "" {
						return m, nil // Don't submit empty input
					}
					branchName = strings.TrimSpace(m.TextInput.Value())
				} else {
					// Using selected Linear ticket
					branchName = m.SelectedIssue.GetBranchName()
				}

				m.Submitted = true
				m.Creating = true
				m.ActiveCreationMode = m.CreationMode
				m.TextInput.SetValue(branchName) // Set the input to the selected branch name

				var creationCmd tea.Cmd
				if m.CreationMode == creationModeBranchOnly {
					creationCmd = m.createBranch()
				} else {
					creationCmd = m.createWorktree()
				}

				return m, tea.Batch(creationCmd, m.Spinner.Tick)
			}
		case tea.KeyTab:
			if !m.Submitted && !m.SubtaskInputMode {
				if m.CreationMode == creationModeWorktree {
					m.CreationMode = creationModeBranchOnly
				} else {
					m.CreationMode = creationModeWorktree
				}
			}
			return m, nil

		case tea.KeyUp:
			if !m.Submitted {
				if m.SearchMode {
					// In search mode, navigate through filtered results
					if len(m.FilteredIssues) > 0 {
						if m.SelectedIssue == nil {
							// Nothing selected, go to last filtered issue
							m.SelectedIssue = &m.FilteredIssues[len(m.FilteredIssues)-1]
							m.InputMode = false
							// Don't update placeholder in search mode
						} else {
							// Find current issue in filtered list and go to previous
							for i, issue := range m.FilteredIssues {
								if issue.ID == m.SelectedIssue.ID {
									if i > 0 {
										m.SelectedIssue = &m.FilteredIssues[i-1]
									} else {
										// At first issue, go back to search input
										m.SelectedIssue = nil
										m.InputMode = true
									}
									break
								}
							}
						}
					}
				} else if m.SelectedIssue == nil && m.AddSubtaskSelected == "" {
					// Currently in custom input mode, try to go to last visible issue
					if len(m.LinearIssues) > 0 {
						m.SelectedIssue = m.getLastVisibleIssue()
						m.InputMode = false
						m.TextInput.Blur()
						if !m.SearchMode {
							m.TextInput.Placeholder = m.SelectedIssue.GetBranchName()
						}
					}
				} else if m.AddSubtaskSelected != "" {
					// From "Add subtask" selection, go to parent issue
					if parent := m.findIssueByID(m.AddSubtaskSelected); parent != nil {
						m.SelectedIssue = parent
						m.AddSubtaskSelected = ""
						if !m.SearchMode {
							m.TextInput.Placeholder = parent.GetBranchName()
						}
					}
				} else if m.SelectedIssue != nil {
					// Try to go to previous issue
					if prevIssue := m.SelectedIssue.PrevVisible(m.LinearIssues); prevIssue != nil {
						m.SelectedIssue = prevIssue
						if !m.SearchMode {
							m.TextInput.Placeholder = prevIssue.GetBranchName()
						}
					} else {
						// Go back to custom input mode
						m.SelectedIssue = nil
						m.InputMode = true
						m.TextInput.Focus()
						m.TextInput.Placeholder = m.DefaultPlaceholder
						m.CreationMode = creationModeBranchOnly
					}
				}
			}
			return m, nil

		case tea.KeyDown:
			if !m.Submitted {
				if m.SearchMode {
					// In search mode, navigate through filtered results
					if len(m.FilteredIssues) > 0 {
						if m.SelectedIssue == nil {
							// Nothing selected, go to first filtered issue
							m.SelectedIssue = &m.FilteredIssues[0]
							m.InputMode = false
							// Don't update placeholder in search mode
						} else {
							// Find current issue in filtered list and go to next
							for i, issue := range m.FilteredIssues {
								if issue.ID == m.SelectedIssue.ID {
									if i < len(m.FilteredIssues)-1 {
										m.SelectedIssue = &m.FilteredIssues[i+1]
									}
									// else stay on last issue
									break
								}
							}
						}
					}
				} else if m.AddSubtaskSelected != "" {
					// From "Add subtask" selection, go to next sibling of parent
					parent := m.findIssueByID(m.AddSubtaskSelected)
					if parent != nil {
						if nextSib := parent.NextSibling(m.LinearIssues); nextSib != nil {
							m.SelectedIssue = nextSib
							m.AddSubtaskSelected = ""
							m.TextInput.Placeholder = nextSib.GetBranchName()
						} else {
							// No next sibling, wrap to custom input
							m.SelectedIssue = nil
							m.AddSubtaskSelected = ""
							m.InputMode = true
							m.TextInput.Focus()
							m.TextInput.Placeholder = m.DefaultPlaceholder
						}
					}
				} else if m.SelectedIssue == nil && len(m.LinearIssues) > 0 {
					// Move from custom input to first visible issue
					m.SelectedIssue = m.getFirstVisibleIssue()
					m.InputMode = false
					if !m.SearchMode {
						m.TextInput.Blur()
						m.TextInput.Placeholder = m.SelectedIssue.GetBranchName()
					}
				} else if m.SelectedIssue != nil {
					// Handle down navigation based on current selection
					if m.SelectedIssue.Expanded && len(m.SelectedIssue.Children) > 0 {
						// From expanded issue with children, go to first child
						m.SelectedIssue = &m.SelectedIssue.Children[0]
						m.TextInput.Placeholder = m.SelectedIssue.GetBranchName()
					} else if m.SelectedIssue.Expanded {
						// From expanded issue with no children, go to "Add subtask" selection
						m.AddSubtaskSelected = m.SelectedIssue.ID
						m.SelectedIssue = nil
					} else {
						// From non-expanded issue, go to next sibling or up the tree
						if nextSib := m.SelectedIssue.NextSibling(m.LinearIssues); nextSib != nil {
							m.SelectedIssue = nextSib
							m.TextInput.Placeholder = nextSib.GetBranchName()
						} else {
							// No next sibling, check if parent is expanded
							if m.SelectedIssue.Parent != nil && m.SelectedIssue.Parent.Expanded {
								// Go to "Add subtask" selection for the parent
								m.AddSubtaskSelected = m.SelectedIssue.Parent.ID
								m.SelectedIssue = nil
							} else {
								// Go up and try parent's next sibling
								current := m.SelectedIssue.Parent
								for current != nil {
									if nextSib := current.NextSibling(m.LinearIssues); nextSib != nil {
										m.SelectedIssue = nextSib
										m.TextInput.Placeholder = nextSib.GetBranchName()
										break
									}
									current = current.Parent
								}
								if current == nil {
									// End of tree, wrap to custom input
									m.SelectedIssue = nil
									m.InputMode = true
									m.TextInput.Focus()
									m.TextInput.Placeholder = m.DefaultPlaceholder
								}
							}
						}
					}
				}
			}
			return m, nil

		case tea.KeyRight:
			if !m.InputMode && !m.Submitted && !m.SearchMode {
				if m.AddSubtaskSelected != "" {
					// Start subtask input mode
					m.SubtaskInputMode = true
					m.SubtaskParentID = m.AddSubtaskSelected
					m.setSubtaskEntryMode(m.AddSubtaskSelected, true)
					m.SubtaskInput.SetValue("")
					m.SubtaskInput.Focus()
				} else if m.SelectedIssue != nil {
					// Always expand - either to show children or the "add subtask" option
					if !m.SelectedIssue.Expanded {
						if m.SelectedIssue.HasChildren && len(m.SelectedIssue.Children) == 0 {
							// Fetch children and expand
							return m, m.fetchChildren(m.SelectedIssue.ID)
						} else {
							// Expand immediately (either shows existing children or just the "add subtask" option)
							m.updateIssueExpansion(m.SelectedIssue.ID, true)
						}
					}
					// If already expanded, do nothing (already showing children/add subtask option)
				}
			}
			return m, nil

		case tea.KeyLeft:
			if !m.InputMode && !m.Submitted && !m.SearchMode {
				if m.AddSubtaskSelected != "" {
					// For add subtask selection, collapse the parent and select it
					m.updateIssueExpansion(m.AddSubtaskSelected, false)
					// Find and select the parent
					if parent := m.findIssueByID(m.AddSubtaskSelected); parent != nil {
						m.SelectedIssue = parent
						m.AddSubtaskSelected = ""
					}
				} else if m.SelectedIssue != nil && m.SelectedIssue.Expanded {
					// Always collapse when left arrow is pressed on an expanded issue
					m.updateIssueExpansion(m.SelectedIssue.ID, false)
				}
			}
			return m, nil

		case tea.KeyBackspace:
			// Handle backspace in search mode
			if m.SearchMode && !m.Submitted && !m.SubtaskInputMode {
				// Let the text input handle the backspace
				m.TextInput, cmd = m.TextInput.Update(msg)
				// Extract search query (remove the leading "/")
				value := m.TextInput.Value()
				if strings.HasPrefix(value, "/") {
					m.SearchQuery = value[1:]
				} else {
					m.SearchQuery = value
				}
				// Update filtered issues
				m.FilteredIssues = m.filterIssuesBySearch(m.SearchQuery)
				return m, cmd
			}

		case tea.KeyRunes:
			// Handle "/" key to enter search mode
			if !m.Submitted && !m.SubtaskInputMode && len(msg.Runes) == 1 && msg.Runes[0] == '/' {
				if !m.SearchMode {
					// Enter search mode
					m.SearchMode = true
					m.SearchQuery = ""
					m.InputMode = true
					m.SelectedIssue = nil
					m.AddSubtaskSelected = ""
					m.TextInput.Placeholder = "type to fuzzy search"
					m.TextInput.SetValue("/")
					m.TextInput.Focus()
					// Initialize filtered issues to show all
					m.FilteredIssues = m.LinearIssues
					return m, nil
				}
			}

			// In search mode, handle typing
			if m.SearchMode && !m.Submitted {
				// Let the text input handle the typing
				m.TextInput, cmd = m.TextInput.Update(msg)
				// Extract search query (remove the leading "/")
				value := m.TextInput.Value()
				if strings.HasPrefix(value, "/") {
					m.SearchQuery = value[1:]
				} else {
					m.SearchQuery = value
				}
				// Update filtered issues
				m.FilteredIssues = m.filterIssuesBySearch(m.SearchQuery)
				return m, cmd
			}
		}

	case worktreeCreatedMsg:
		m.Creating = false
		m.Done = true
		m.Success = true
		m.Result = fmt.Sprintf("Worktree created at: %s", msg.path)
		// Store the path for later execution and quit the TUI
		m.WorktreePath = msg.path
		return m, tea.Quit

	case branchCreatedMsg:
		m.Creating = false
		m.Done = true
		m.Success = true
		m.Result = fmt.Sprintf("Branch created: %s", msg.branch)
		m.WorktreePath = ""
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
		// Update placeholder if a Linear ticket is currently selected (but not in search mode)
		if m.SelectedIssue != nil && !m.SearchMode {
			m.TextInput.Placeholder = m.SelectedIssue.GetBranchName()
		}

	case linearErrorMsg:
		m.LinearLoading = false
		m.LinearError = msg.err.Error()

	case childrenLoadedMsg:
		m.setIssueChildren(msg.parentID, msg.children)
		// Update placeholder if a Linear ticket is currently selected (but not in search mode)
		if m.SelectedIssue != nil && !m.SearchMode {
			m.TextInput.Placeholder = m.SelectedIssue.GetBranchName()
		}

	case childrenErrorMsg:
		// Could show error message or silently fail

	case subtaskCreatedMsg:
		m.CreatingSubtask = false

		// Clear subtask input
		m.SubtaskInput.SetValue("")
		m.SubtaskInput.Blur()
		m.SubtaskInputMode = false
		m.setSubtaskEntryMode(m.SubtaskParentID, false)
		m.SubtaskParentID = ""
		m.AddSubtaskSelected = ""

		// Add the newly created subtask to the parent's children and expand
		m.addSubtaskToParent(msg.parentID, msg.subtask)
		m.updateIssueExpansion(msg.parentID, true)

		// Find and select the newly created subtask
		if createdSubtask := m.findIssueByID(msg.subtask.ID); createdSubtask != nil {
			m.SelectedIssue = createdSubtask
			m.InputMode = false
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
	if m.InputMode && !m.SearchMode {
		m.TextInput, cmd = m.TextInput.Update(msg)
	} else if m.SubtaskInputMode {
		m.SubtaskInput, cmd = m.SubtaskInput.Update(msg)
	}

	return m, cmd
}

// getFirstVisibleIssue returns the first visible issue in the tree
func (m *model) getFirstVisibleIssue() *linear.Issue {
	if len(m.LinearIssues) > 0 {
		return &m.LinearIssues[0]
	}
	return nil
}

// getLastVisibleIssue returns the last visible issue in the tree
func (m *model) getLastVisibleIssue() *linear.Issue {
	if len(m.LinearIssues) == 0 {
		return nil
	}
	// Start with the last root issue and find its last visible descendant
	return m.LinearIssues[len(m.LinearIssues)-1].LastVisible()
}

// findIssueByID finds an issue by ID in the tree
func (m *model) findIssueByID(id string) *linear.Issue {
	var find func(issues []linear.Issue) *linear.Issue
	find = func(issues []linear.Issue) *linear.Issue {
		for i := range issues {
			if issues[i].ID == id {
				return &issues[i]
			}
			if found := find(issues[i].Children); found != nil {
				return found
			}
		}
		return nil
	}
	return find(m.LinearIssues)
}

// setSubtaskEntryMode sets the subtask entry mode for an issue
func (m *model) setSubtaskEntryMode(issueID string, enabled bool) {
	var update func(issues *[]linear.Issue)
	update = func(issues *[]linear.Issue) {
		for i := range *issues {
			if (*issues)[i].ID == issueID {
				(*issues)[i].ShowingSubtaskEntry = enabled
				if enabled {
					(*issues)[i].SubtaskEntryText = ""
				}
				return
			}
			if len((*issues)[i].Children) > 0 {
				update(&(*issues)[i].Children)
			}
		}
	}
	update(&m.LinearIssues)
}

func (m model) createWorktree() tea.Cmd {
	return func() tea.Msg {
		if m.WorktreeManager == nil {
			return errMsg{fmt.Errorf("worktree manager not configured")}
		}

		branchName := strings.TrimSpace(m.TextInput.Value())
		worktreePath, err := m.WorktreeManager.CreateWorktree(branchName)
		if err != nil {
			return errMsg{err}
		}
		return worktreeCreatedMsg{branchName, worktreePath}
	}
}

func (m model) createBranch() tea.Cmd {
	return func() tea.Msg {
		if m.WorktreeManager == nil {
			return errMsg{fmt.Errorf("worktree manager not configured")}
		}

		branchName := strings.TrimSpace(m.TextInput.Value())
		if err := m.WorktreeManager.CreateBranch(branchName); err != nil {
			return errMsg{err}
		}
		return branchCreatedMsg{branch: branchName}
	}
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
				// Set depth and parent pointers for children
				for j := range (*issues)[i].Children {
					(*issues)[i].Children[j].Depth = (*issues)[i].Depth + 1
					(*issues)[i].Children[j].Parent = &(*issues)[i]
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

// filterIssuesBySearch filters issues using fuzzy search on identifier and title
func (m *model) filterIssuesBySearch(query string) []linear.Issue {
	if query == "" {
		return m.LinearIssues
	}

	var filtered []linear.Issue

	// Helper function to recursively collect all issues (including children)
	var collectAllIssues func(issues []linear.Issue) []linear.Issue
	collectAllIssues = func(issues []linear.Issue) []linear.Issue {
		var result []linear.Issue
		for _, issue := range issues {
			result = append(result, issue)
			if len(issue.Children) > 0 {
				result = append(result, collectAllIssues(issue.Children)...)
			}
		}
		return result
	}

	allIssues := collectAllIssues(m.LinearIssues)

	// Create search targets (identifier + title) for fuzzy matching
	var targets []string
	for _, issue := range allIssues {
		targets = append(targets, strings.ToLower(issue.Identifier+" "+issue.Title))
	}

	// Perform fuzzy search
	matches := fuzzy.FindNormalized(strings.ToLower(query), targets)

	// Build filtered results maintaining only top-level issues
	matchedTargets := make(map[string]bool)
	for _, match := range matches {
		matchedTargets[match] = true
	}

	for _, issue := range allIssues {
		target := strings.ToLower(issue.Identifier + " " + issue.Title)
		if matchedTargets[target] {
			// Only include top-level issues (depth 0) in filtered results
			if issue.Depth == 0 {
				filtered = append(filtered, issue)
			}
		}
	}

	return filtered
}

// getStatusStyle returns the appropriate style for a given issue status
func (m *model) getStatusStyle(state linear.State) lipgloss.Style {
	switch strings.ToLower(state.Type) {
	case "backlog", "unstarted":
		return statusBacklogStyle
	case "started", "in_progress", "in progress":
		return statusInProgressStyle
	case "in_review", "review", "in review":
		return statusInReviewStyle
	case "done", "completed":
		return statusDoneStyle
	case "cancelled", "canceled":
		return statusCancelledStyle
	default:
		// For "todo" and other unknown states
		return statusTodoStyle
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
				subtask.Parent = &(*issues)[i]
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

type branchCreatedMsg struct {
	branch string
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
		if m.ActiveCreationMode == creationModeBranchOnly {
			return fmt.Sprintf("%s Creating branch...", m.Spinner.View())
		}
		return fmt.Sprintf("%s Creating worktree...", m.Spinner.View())
	}

	if m.CreatingSubtask {
		return fmt.Sprintf("%s Creating subtask...", m.Spinner.View())
	}

	s := strings.Builder{}
	s.WriteString(headerStyle.Render("🌱 sprout"))
	s.WriteString("\n\n")

	// Input using textinput component - adjust prompt style based on selection and display search mode appropriately
	if m.SearchMode {
		// In search mode, show special search UI
		m.TextInput.PromptStyle = selectedStyle
		// Show search input with proper formatting
		value := m.TextInput.Value()
		if value == "/" && m.SearchQuery == "" {
			// Show placeholder when only "/" is entered
			s.WriteString(selectedStyle.Render("/type to fuzzy search"))
		} else {
			// Show actual search content with selected branch if any
			searchDisplay := "/" + m.SearchQuery
			if m.SelectedIssue != nil && !m.InputMode {
				// Show selected issue's branch name after the search
				fullDisplay := searchDisplay + " sprout/" + m.SelectedIssue.GetBranchName()
				s.WriteString(selectedStyle.Render(fullDisplay))
			} else {
				s.WriteString(selectedStyle.Render(searchDisplay))
			}
		}
	} else {
		// Normal mode - adjust prompt style based on selection
		if m.SelectedIssue == nil {
			// When input is selected, use selected style for prompt
			m.TextInput.PromptStyle = selectedStyle
		} else {
			// When input is not selected, use normal style
			m.TextInput.PromptStyle = lipgloss.NewStyle().Foreground(primaryColor)
		}
		s.WriteString(m.TextInput.View())
	}
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
			if treeView != "" {
				trimmedTree := strings.TrimRight(treeView, "\n")
				s.WriteString(trimmedTree)
				s.WriteString("\n")
			}
		}
	}

	// Display creation mode toggle at the bottom, ensuring we only add a newline if needed
	if !strings.HasSuffix(s.String(), "\n") {
		s.WriteString("\n")
	}
	modeLabel := "[worktree <tab>]"
	if m.CreationMode == creationModeBranchOnly {
		modeLabel = "[branch <tab>]"
	}
	s.WriteString(helpStyle.Render(modeLabel))

	return s.String()
}

func (m model) buildSimpleLinearTree() string {
	// Choose which issues to display based on search mode
	var issuesToDisplay []linear.Issue
	if m.SearchMode {
		issuesToDisplay = m.FilteredIssues
	} else {
		issuesToDisplay = m.LinearIssues
	}

	if len(issuesToDisplay) == 0 {
		return ""
	}

	// Calculate maximum identifier and status widths for alignment
	maxIdentifierWidth := 0
	maxStatusWidth := 0
	var calculateMaxWidths func([]linear.Issue)
	calculateMaxWidths = func(issues []linear.Issue) {
		for _, issue := range issues {
			identifierWidth := lipgloss.Width(issue.Identifier)
			if identifierWidth > maxIdentifierWidth {
				maxIdentifierWidth = identifierWidth
			}

			statusWidth := lipgloss.Width(issue.State.Name)
			if statusWidth > maxStatusWidth {
				maxStatusWidth = statusWidth
			}

			// Check children if not in search mode
			if !m.SearchMode && len(issue.Children) > 0 {
				calculateMaxWidths(issue.Children)
			}
		}
	}

	// Calculate max widths from all issues (including nested children)
	if m.SearchMode {
		calculateMaxWidths(m.FilteredIssues)
	} else {
		calculateMaxWidths(m.LinearIssues)
	}

	// Create a copy of the model with the calculated max widths
	mWithWidth := m
	mWithWidth.MaxIdentifierWidth = maxIdentifierWidth
	mWithWidth.MaxStatusWidth = maxStatusWidth

	// Build tree using lipgloss tree library directly from the tree structure
	root := tree.Root("").
		ItemStyle(normalStyle).
		EnumeratorStyle(expandedStyle)

	// Recursively build the tree
	for _, issue := range issuesToDisplay {
		// In search mode, pass a copy of the issue that's not expanded
		if m.SearchMode {
			issueCopy := issue
			issueCopy.Expanded = false // Don't show children in search mode
			mWithWidth.addIssueNode(root, issueCopy)
		} else {
			mWithWidth.addIssueNode(root, issue)
		}
	}

	return root.String()
}

// addIssueNode recursively adds an issue and its children to the tree
func (m model) addIssueNode(parent *tree.Tree, issue linear.Issue) {
	// Create the display content
	title := issue.Title

	// Get status display
	statusText := issue.State.Name
	statusStyle := m.getStatusStyle(issue.State)
	styledStatus := statusStyle.Render(statusText)

	// Calculate available width for title based on terminal size
	// Account for: identifier, status, spaces, tree symbols, and margins
	statusWidth := lipgloss.Width(statusText)
	treePrefixWidth := (issue.Depth + 1) * 3 // Approximate tree prefix width
	marginWidth := 15                        // Safety margin for tree symbols, spacing, and status padding
	availableWidth := m.Width - m.MaxIdentifierWidth - m.MaxStatusWidth - treePrefixWidth - marginWidth

	// Ensure minimum width and truncate if necessary
	if availableWidth < 20 {
		availableWidth = 20
	}
	if len(title) > availableWidth {
		if availableWidth > 3 {
			title = title[:availableWidth-3] + "..."
		}
	}

	identifier := identifierStyle.Render(issue.Identifier)
	titleText := titleStyle.Render(title)

	// Pad identifier to align with the longest identifier
	identifierPadding := m.MaxIdentifierWidth - lipgloss.Width(identifier)
	paddedIdentifier := identifier + strings.Repeat(" ", identifierPadding)

	// Pad status to align with the longest status
	statusPadding := m.MaxStatusWidth - statusWidth
	paddedStatus := styledStatus + strings.Repeat(" ", statusPadding)

	content := fmt.Sprintf("%s  %s  %s", paddedIdentifier, paddedStatus, titleText)

	// Apply selection styling if this is the selected item
	if m.SelectedIssue != nil && m.SelectedIssue.ID == issue.ID {
		content = selectedStyle.Render(content)
	} else {
		content = normalStyle.Render(content)
	}

	// If expanded and has children or needs to show "Add subtask"
	if issue.Expanded {
		// Create a new tree node with the issue as root
		issueNode := tree.New().Root(content).
			ItemStyle(normalStyle).
			EnumeratorStyle(expandedStyle)

		// Add actual children
		for _, child := range issue.Children {
			m.addIssueNode(issueNode, child)
		}

		// Add "Add subtask" entry - either input field or placeholder
		var addSubtaskContent string
		if issue.ShowingSubtaskEntry {
			// Show the input field inline
			if m.SubtaskInputMode && m.SubtaskParentID == issue.ID {
				addSubtaskContent = m.SubtaskInput.View()
			} else {
				// Show the text being entered (not currently in input mode)
				addSubtaskContent = addSubtaskStyle.Render("+ " + issue.SubtaskEntryText)
			}
		} else {
			addSubtaskContent = addSubtaskStyle.Render("+ Add subtask")
		}

		// Apply selection styling if this is the selected "Add subtask" item
		if m.AddSubtaskSelected == issue.ID {
			addSubtaskContent = selectedStyle.Render(addSubtaskContent)
		} else {
			addSubtaskContent = normalStyle.Render(addSubtaskContent)
		}

		issueNode.Child(addSubtaskContent)

		// Add the complete subtree to parent
		parent.Child(issueNode)
	} else {
		// Just add the issue without children
		parent.Child(content)
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
