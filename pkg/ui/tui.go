package ui

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
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
	PromptInput        textarea.Model
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
	Worktrees          []git.Worktree
	WorktreesLoading   bool
	WorktreesError     string
	ShowAllWorkItems   bool
	SelectedWorktree   string
	ResumeBranch       string
	ResumeCommandArgs  []string
	Resumed            bool
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
	LastUnassigned     *unassignedIssueSnapshot
	DefaultCommandArgs []string
	NeedsPromptCapture bool
	PromptCaptureMode  bool
	PromptSubmitted    bool
	CreationFinished   bool
	CapturedPrompt     string
}

type unassignedIssueSnapshot struct {
	Issue    linear.Issue
	ParentID string
	Index    int
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

	return NewTUIWithDependenciesAndConfig(wm, linearClient, cfg)
}

func NewTUIWithDependencies(wm git.WorktreeManagerInterface, linearClient linear.LinearClientInterface) (model, error) {
	return NewTUIWithDependenciesAndConfig(wm, linearClient, config.DefaultConfig())
}

func NewTUIWithDependenciesAndConfig(wm git.WorktreeManagerInterface, linearClient linear.LinearClientInterface, cfg *config.Config) (model, error) {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	defaultCommandArgs := cfg.GetDefaultCommand()
	resumeCommandArgs := cfg.GetResumeCommand()

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

	// Initialize multiline prompt input for async queued prompts
	pi := textarea.New()
	pi.Prompt = "prompt> "
	pi.Placeholder = "type prompt (Enter submits, Alt+Enter/Shift+Enter newline)"
	pi.ShowLineNumbers = false
	pi.CharLimit = 0
	pi.SetHeight(5)
	pi.SetWidth(80)
	pi.KeyMap.InsertNewline = key.NewBinding(key.WithKeys("alt+enter", "shift+enter", "ctrl+j"))
	pi.FocusedStyle.Prompt = selectedStyle
	pi.FocusedStyle.Text = titleStyle
	pi.FocusedStyle.Placeholder = helpStyle
	pi.BlurredStyle.Prompt = selectedStyle
	pi.BlurredStyle.Text = titleStyle
	pi.BlurredStyle.Placeholder = helpStyle

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
		PromptInput:        pi,
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
		Worktrees:          nil,
		WorktreesLoading:   wm != nil,
		WorktreesError:     "",
		ShowAllWorkItems:   false,
		SelectedWorktree:   "",
		ResumeBranch:       "",
		ResumeCommandArgs:  resumeCommandArgs,
		Resumed:            false,
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
		LastUnassigned:     nil,
		DefaultCommandArgs: defaultCommandArgs,
		NeedsPromptCapture: config.NeedsPromptCapture(defaultCommandArgs),
		PromptCaptureMode:  false,
		PromptSubmitted:    false,
		CreationFinished:   false,
		CapturedPrompt:     "",
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
	if m.WorktreeManager != nil {
		cmds = append(cmds, m.fetchWorktrees())
	}

	// Start spinner if we have any loading states
	if m.LinearLoading || m.WorktreesLoading || m.Creating || m.CreatingSubtask {
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
		promptInputWidth := m.Width - 4
		if promptInputWidth < 20 {
			promptInputWidth = 20
		}
		m.PromptInput.SetWidth(promptInputWidth)
		if m.Height > 12 {
			m.PromptInput.SetHeight(5)
		} else {
			m.PromptInput.SetHeight(3)
		}

		return m, nil

	case tea.KeyMsg:
		if m.Done {
			return m, tea.Quit
		}

		if m.PromptCaptureMode && m.ActiveCreationMode == creationModeWorktree {
			switch msg.Type {
			case tea.KeyCtrlC, tea.KeyEsc:
				m.Cancelled = true
				return m, tea.Quit
			case tea.KeyEnter:
				if msg.Alt {
					m.PromptInput.InsertRune('\n')
					return m, nil
				}
				if m.PromptSubmitted {
					return m, nil
				}

				m.CapturedPrompt = m.PromptInput.Value()
				m.PromptSubmitted = true

				if m.CreationFinished {
					m.PromptCaptureMode = false
					m.Done = true
					m.Success = true
					m.Result = fmt.Sprintf("Worktree created at: %s", m.WorktreePath)
					return m, tea.Quit
				}

				return m, nil
			}
			if msg.String() == "shift+enter" || msg.Type == tea.KeyCtrlJ {
				m.PromptInput.InsertRune('\n')
				return m, nil
			}

			m.PromptInput, cmd = m.PromptInput.Update(msg)
			return m, cmd
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

				if selected := m.selectedRow(); selected != nil && selected.Worktree != nil && selected.Kind != workQueueRowAddSubtask {
					m.Submitted = true
					m.Creating = false
					m.Done = true
					m.Success = true
					m.Resumed = true
					m.WorktreePath = selected.Worktree.Path
					m.ResumeBranch = selected.Worktree.Branch
					m.Result = fmt.Sprintf("Worktree resumed at: %s", selected.Worktree.Path)
					return m, tea.Quit
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
				m.CreationFinished = false
				m.PromptSubmitted = false
				m.CapturedPrompt = ""
				m.PromptInput.Reset()
				m.PromptInput.Blur()

				if m.CreationMode == creationModeWorktree && m.NeedsPromptCapture {
					m.PromptCaptureMode = true
					m.SearchMode = false
					m.SearchQuery = ""
					m.FilteredIssues = nil
					m.SelectedIssue = nil
					m.AddSubtaskSelected = ""
					m.InputMode = false
					m.TextInput.Blur()
					m.PromptInput.Focus()
				} else {
					m.PromptCaptureMode = false
					m.TextInput.SetValue(branchName) // Set the input to the selected branch name
				}

				var creationCmd tea.Cmd
				if m.CreationMode == creationModeBranchOnly {
					creationCmd = m.createBranch(branchName)
				} else {
					creationCmd = m.createWorktree(branchName)
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
				m.moveSelection(-1)
			}
			return m, nil

		case tea.KeyDown:
			if !m.Submitted {
				m.moveSelection(1)
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
			if !m.Submitted && !m.SubtaskInputMode && !m.SearchMode && len(msg.Runes) == 1 {
				switch msg.Runes[0] {
				case 'a', 'A':
					if m.InputMode && m.TextInput.Value() != "" {
						break
					}
					if len(m.Worktrees) == 0 {
						break
					}
					m.ShowAllWorkItems = !m.ShowAllWorkItems
					m.selectInput()
					return m, nil
				case 'u', 'U':
					if m.SelectedIssue != nil && m.LinearClient != nil {
						return m, m.unassignIssue(m.SelectedIssue.ID)
					}
				case 'd', 'D':
					if m.SelectedIssue != nil && m.LinearClient != nil {
						return m, m.markIssueDone(m.SelectedIssue.ID)
					}
				case 'z', 'Z':
					if m.LastUnassigned != nil && m.LinearClient != nil {
						return m, m.assignIssueToMe(m.LastUnassigned.Issue.ID)
					}
				}
			}

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
		m.WorktreePath = msg.path
		m.CreationFinished = true

		if m.PromptCaptureMode {
			if m.PromptSubmitted {
				m.PromptCaptureMode = false
				m.Done = true
				m.Success = true
				m.Result = fmt.Sprintf("Worktree created at: %s", msg.path)
				return m, tea.Quit
			}
			return m, nil
		}

		m.Done = true
		m.Success = true
		m.Result = fmt.Sprintf("Worktree created at: %s", msg.path)
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

	case worktreesLoadedMsg:
		m.WorktreesLoading = false
		m.Worktrees = msg.worktrees
		m.WorktreesError = ""

	case worktreesErrorMsg:
		m.WorktreesLoading = false
		m.WorktreesError = msg.err.Error()

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

	case issueUnassignedMsg:
		snapshot, ok := m.removeIssueByID(msg.issueID)
		if ok {
			m.LastUnassigned = &snapshot
			m.selectAfterIssueRemoval(snapshot)
		}

	case issueUnassignErrorMsg:
		m.LinearError = msg.err.Error()

	case issueReassignedMsg:
		if m.LastUnassigned != nil && m.LastUnassigned.Issue.ID == msg.issueID {
			restoredIssueID := m.LastUnassigned.Issue.ID
			m.restoreIssue(*m.LastUnassigned)
			m.LastUnassigned = nil
			if restored := m.findIssueByID(restoredIssueID); restored != nil {
				m.SelectedIssue = restored
				m.InputMode = false
				m.TextInput.Blur()
				if !m.SearchMode {
					m.TextInput.Placeholder = restored.GetBranchName()
				}
			}
		}

	case issueReassignErrorMsg:
		m.LinearError = msg.err.Error()

	case issueDoneMsg:
		snapshot, ok := m.removeIssueByID(msg.issueID)
		if ok {
			m.selectAfterIssueRemoval(snapshot)
		}

	case issueDoneErrorMsg:
		m.LinearError = msg.err.Error()
	}

	// Update spinner if any loading state is active
	if m.LinearLoading || m.WorktreesLoading || m.Creating || m.CreatingSubtask {
		var spinnerCmd tea.Cmd
		m.Spinner, spinnerCmd = m.Spinner.Update(msg)
		if cmd != nil {
			return m, tea.Batch(cmd, spinnerCmd)
		}
		cmd = spinnerCmd
	}

	// Update text inputs based on current mode
	if m.PromptCaptureMode {
		m.PromptInput, cmd = m.PromptInput.Update(msg)
	} else if m.InputMode && !m.SearchMode {
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

func (m model) createWorktree(branchName string) tea.Cmd {
	return func() tea.Msg {
		if m.WorktreeManager == nil {
			return errMsg{fmt.Errorf("worktree manager not configured")}
		}

		branchName = strings.TrimSpace(branchName)
		if branchName == "" {
			return errMsg{fmt.Errorf("branch name cannot be empty")}
		}

		worktreePath, err := m.WorktreeManager.CreateWorktree(branchName)
		if err != nil {
			return errMsg{err}
		}
		return worktreeCreatedMsg{branchName, worktreePath}
	}
}

func (m model) createBranch(branchName string) tea.Cmd {
	return func() tea.Msg {
		if m.WorktreeManager == nil {
			return errMsg{fmt.Errorf("worktree manager not configured")}
		}

		branchName = strings.TrimSpace(branchName)
		if branchName == "" {
			return errMsg{fmt.Errorf("branch name cannot be empty")}
		}

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

func (m model) fetchWorktrees() tea.Cmd {
	return func() tea.Msg {
		worktrees, err := m.WorktreeManager.ListWorktreesForTUI()
		if err != nil {
			return worktreesErrorMsg{err}
		}
		return worktreesLoadedMsg{worktrees}
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

func (m model) unassignIssue(issueID string) tea.Cmd {
	return func() tea.Msg {
		if m.LinearClient == nil {
			return issueUnassignErrorMsg{err: fmt.Errorf("linear client not configured")}
		}
		if err := m.LinearClient.UnassignIssue(issueID); err != nil {
			return issueUnassignErrorMsg{err: err}
		}
		return issueUnassignedMsg{issueID: issueID}
	}
}

func (m model) assignIssueToMe(issueID string) tea.Cmd {
	return func() tea.Msg {
		if m.LinearClient == nil {
			return issueReassignErrorMsg{err: fmt.Errorf("linear client not configured")}
		}
		if err := m.LinearClient.AssignIssueToMe(issueID); err != nil {
			return issueReassignErrorMsg{err: err}
		}
		return issueReassignedMsg{issueID: issueID}
	}
}

func (m model) markIssueDone(issueID string) tea.Cmd {
	return func() tea.Msg {
		if m.LinearClient == nil {
			return issueDoneErrorMsg{err: fmt.Errorf("linear client not configured")}
		}
		if err := m.LinearClient.MarkIssueDone(issueID); err != nil {
			return issueDoneErrorMsg{err: err}
		}
		return issueDoneMsg{issueID: issueID}
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

func (m *model) removeIssueByID(issueID string) (unassignedIssueSnapshot, bool) {
	var zero unassignedIssueSnapshot

	for i := range m.LinearIssues {
		if m.LinearIssues[i].ID == issueID {
			removed := m.LinearIssues[i]
			m.LinearIssues = append(m.LinearIssues[:i], m.LinearIssues[i+1:]...)
			m.normalizeIssueTree()
			return unassignedIssueSnapshot{
				Issue:    removed,
				ParentID: "",
				Index:    i,
			}, true
		}
	}

	var removeFromChildren func(parent *linear.Issue) (unassignedIssueSnapshot, bool)
	removeFromChildren = func(parent *linear.Issue) (unassignedIssueSnapshot, bool) {
		for i := range parent.Children {
			if parent.Children[i].ID == issueID {
				removed := parent.Children[i]
				parent.Children = append(parent.Children[:i], parent.Children[i+1:]...)
				parent.HasChildren = len(parent.Children) > 0
				m.normalizeIssueTree()
				return unassignedIssueSnapshot{
					Issue:    removed,
					ParentID: parent.ID,
					Index:    i,
				}, true
			}

			if snapshot, ok := removeFromChildren(&parent.Children[i]); ok {
				return snapshot, true
			}
		}
		return zero, false
	}

	for i := range m.LinearIssues {
		if snapshot, ok := removeFromChildren(&m.LinearIssues[i]); ok {
			return snapshot, true
		}
	}

	return zero, false
}

func (m *model) restoreIssue(snapshot unassignedIssueSnapshot) {
	if snapshot.ParentID == "" {
		index := snapshot.Index
		if index < 0 {
			index = 0
		}
		if index > len(m.LinearIssues) {
			index = len(m.LinearIssues)
		}
		m.LinearIssues = append(m.LinearIssues[:index], append([]linear.Issue{snapshot.Issue}, m.LinearIssues[index:]...)...)
		m.normalizeIssueTree()
		return
	}

	parent := m.findIssueByID(snapshot.ParentID)
	if parent == nil {
		m.LinearIssues = append(m.LinearIssues, snapshot.Issue)
		m.normalizeIssueTree()
		return
	}

	index := snapshot.Index
	if index < 0 {
		index = 0
	}
	if index > len(parent.Children) {
		index = len(parent.Children)
	}
	parent.Children = append(parent.Children[:index], append([]linear.Issue{snapshot.Issue}, parent.Children[index:]...)...)
	parent.HasChildren = true
	parent.Expanded = true
	m.normalizeIssueTree()
}

func (m *model) normalizeIssueTree() {
	var normalize func(parent *linear.Issue, issues *[]linear.Issue, depth int)
	normalize = func(parent *linear.Issue, issues *[]linear.Issue, depth int) {
		for i := range *issues {
			(*issues)[i].Depth = depth
			(*issues)[i].Parent = parent
			(*issues)[i].HasChildren = len((*issues)[i].Children) > 0
			if len((*issues)[i].Children) > 0 {
				normalize(&(*issues)[i], &(*issues)[i].Children, depth+1)
			}
		}
	}

	normalize(nil, &m.LinearIssues, 0)
}

func (m *model) selectAfterIssueRemoval(snapshot unassignedIssueSnapshot) {
	if len(m.LinearIssues) == 0 {
		m.SelectedIssue = nil
		m.InputMode = true
		m.TextInput.Focus()
		m.TextInput.Placeholder = m.DefaultPlaceholder
		return
	}

	if snapshot.ParentID == "" {
		index := snapshot.Index
		if index >= len(m.LinearIssues) {
			index = len(m.LinearIssues) - 1
		}
		if index >= 0 {
			m.SelectedIssue = &m.LinearIssues[index]
		}
	} else {
		parent := m.findIssueByID(snapshot.ParentID)
		if parent != nil && parent.Expanded && len(parent.Children) > 0 {
			index := snapshot.Index
			if index >= len(parent.Children) {
				index = len(parent.Children) - 1
			}
			if index >= 0 {
				m.SelectedIssue = &parent.Children[index]
			}
		} else if parent != nil {
			m.SelectedIssue = parent
		}
	}

	if m.SelectedIssue == nil {
		m.InputMode = true
		m.TextInput.Focus()
		m.TextInput.Placeholder = m.DefaultPlaceholder
		return
	}

	m.InputMode = false
	m.TextInput.Blur()
	if !m.SearchMode {
		m.TextInput.Placeholder = m.SelectedIssue.GetBranchName()
	}
}

func (m *model) visibleWorkQueueRows() []workQueueRow {
	allRows := m.buildWorkQueueRows()
	if m.SearchMode {
		return m.filterWorkQueueRows(allRows, m.SearchQuery)
	}
	return allRows
}

func (m *model) buildWorkQueueRows() []workQueueRow {
	matchedBranches := make(map[string]bool)
	worktreesByIssue := m.matchWorktreesToIssues(&matchedBranches)

	var activeRows []workQueueRow
	var closedRows []workQueueRow
	for i := range m.LinearIssues {
		row := m.issueRow(&m.LinearIssues[i], worktreesByIssue)
		if row.Closed && len(m.Worktrees) > 0 {
			closedRows = append(closedRows, row)
		} else {
			activeRows = append(activeRows, row)
		}
	}

	for i := range m.Worktrees {
		wt := m.Worktrees[i]
		if !m.shouldConsiderWorktree(wt) || matchedBranches[wt.Branch] {
			continue
		}
		row := workQueueRow{
			Kind:     workQueueRowWorktree,
			Worktree: &m.Worktrees[i],
			Closed:   wt.Merged,
			Updated:  wt.UpdatedAt,
		}
		if row.Closed {
			closedRows = append(closedRows, row)
		} else {
			activeRows = append(activeRows, row)
		}
	}

	sortRows(activeRows)
	sortRows(closedRows)

	var rows []workQueueRow
	for _, row := range activeRows {
		rows = append(rows, m.expandRow(row, worktreesByIssue)...)
		if !m.ShowAllWorkItems && len(rows) >= maxVisibleActiveRows {
			return rows[:maxVisibleActiveRows]
		}
	}
	if m.ShowAllWorkItems {
		for _, row := range closedRows {
			rows = append(rows, m.expandRow(row, worktreesByIssue)...)
		}
	}
	return rows
}

func sortRows(rows []workQueueRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Updated.IsZero() && rows[j].Updated.IsZero() {
			return false
		}
		if !rows[i].Updated.Equal(rows[j].Updated) {
			return rows[i].Updated.After(rows[j].Updated)
		}
		return rowSortLabel(rows[i]) < rowSortLabel(rows[j])
	})
}

func rowSortLabel(row workQueueRow) string {
	if row.Issue != nil {
		return strings.ToLower(row.Issue.Identifier)
	}
	if row.Worktree != nil {
		return strings.ToLower(row.Worktree.Branch)
	}
	return ""
}

func (m *model) issueRow(issue *linear.Issue, worktreesByIssue map[string]*git.Worktree) workQueueRow {
	wt := worktreesByIssue[strings.ToUpper(issue.Identifier)]
	updated := issue.UpdatedAt
	if updated.IsZero() && wt != nil && wt.UpdatedAt.After(updated) {
		updated = wt.UpdatedAt
	}
	return workQueueRow{
		Kind:     workQueueRowIssue,
		Issue:    issue,
		Worktree: wt,
		Closed:   isClosedIssue(*issue) || (wt != nil && wt.Merged),
		Updated:  updated,
	}
}

func (m *model) expandRow(row workQueueRow, worktreesByIssue map[string]*git.Worktree) []workQueueRow {
	rows := []workQueueRow{row}
	if row.Kind != workQueueRowIssue || row.Issue == nil || !row.Issue.Expanded {
		return rows
	}
	for i := range row.Issue.Children {
		childRow := m.issueRow(&row.Issue.Children[i], worktreesByIssue)
		if childRow.Closed && len(m.Worktrees) > 0 && !m.ShowAllWorkItems {
			continue
		}
		rows = append(rows, m.expandRow(childRow, worktreesByIssue)...)
	}
	rows = append(rows, workQueueRow{Kind: workQueueRowAddSubtask, ParentID: row.Issue.ID})
	return rows
}

func (m *model) matchWorktreesToIssues(matchedBranches *map[string]bool) map[string]*git.Worktree {
	result := make(map[string]*git.Worktree)
	var walk func([]linear.Issue)
	walk = func(issues []linear.Issue) {
		for i := range issues {
			identifier := strings.ToUpper(issues[i].Identifier)
			for j := range m.Worktrees {
				wt := &m.Worktrees[j]
				if !m.shouldConsiderWorktree(*wt) {
					continue
				}
				if branchMatchesIdentifier(wt.Branch, identifier) {
					if existing := result[identifier]; existing == nil || wt.UpdatedAt.After(existing.UpdatedAt) {
						result[identifier] = wt
					}
					(*matchedBranches)[wt.Branch] = true
				}
			}
			if len(issues[i].Children) > 0 {
				walk(issues[i].Children)
			}
		}
	}
	walk(m.LinearIssues)
	return result
}

func branchMatchesIdentifier(branch, identifier string) bool {
	branch = strings.ToLower(branch)
	identifier = strings.ToLower(identifier)
	return branch == identifier || strings.HasPrefix(branch, identifier+"-") || strings.HasPrefix(branch, identifier+"/")
}

func (m *model) shouldConsiderWorktree(wt git.Worktree) bool {
	if wt.Prunable || wt.Branch == "" {
		return false
	}
	branch := strings.ToLower(wt.Branch)
	return branch != "main" && branch != "master"
}

func isClosedIssue(issue linear.Issue) bool {
	stateType := strings.ToLower(issue.State.Type)
	return stateType == "completed" || stateType == "done" || stateType == "canceled" || stateType == "cancelled"
}

func (m *model) filterWorkQueueRows(rows []workQueueRow, query string) []workQueueRow {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return rows
	}
	var filtered []workQueueRow
	for _, row := range rows {
		target := ""
		if row.Issue != nil {
			target = row.Issue.Identifier + " " + row.Issue.Title
		} else if row.Worktree != nil {
			target = row.Worktree.Branch + " " + row.Worktree.Path
		}
		if fuzzy.MatchNormalized(query, strings.ToLower(target)) {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

func (m *model) selectedRow() *workQueueRow {
	rows := m.visibleWorkQueueRows()
	for i := range rows {
		row := rows[i]
		switch row.Kind {
		case workQueueRowIssue:
			if m.SelectedIssue != nil && row.Issue != nil && row.Issue.ID == m.SelectedIssue.ID {
				return &row
			}
		case workQueueRowWorktree:
			if row.Worktree != nil && row.Worktree.Branch == m.SelectedWorktree {
				return &row
			}
		case workQueueRowAddSubtask:
			if row.ParentID == m.AddSubtaskSelected {
				return &row
			}
		}
	}
	return nil
}

func (m *model) selectRow(row workQueueRow) {
	m.SelectedIssue = nil
	m.SelectedWorktree = ""
	m.AddSubtaskSelected = ""
	m.InputMode = false
	m.TextInput.Blur()

	switch row.Kind {
	case workQueueRowIssue:
		m.SelectedIssue = row.Issue
		if row.Worktree != nil {
			m.TextInput.Placeholder = row.Worktree.Branch
		} else if row.Issue != nil {
			m.TextInput.Placeholder = row.Issue.GetBranchName()
		}
	case workQueueRowWorktree:
		if row.Worktree != nil {
			m.SelectedWorktree = row.Worktree.Branch
			m.TextInput.Placeholder = row.Worktree.Branch
		}
	case workQueueRowAddSubtask:
		m.AddSubtaskSelected = row.ParentID
	}
}

func (m *model) selectInput() {
	m.SelectedIssue = nil
	m.SelectedWorktree = ""
	m.AddSubtaskSelected = ""
	m.InputMode = true
	m.TextInput.Focus()
	m.TextInput.Placeholder = m.DefaultPlaceholder
}

func (m *model) moveSelection(delta int) {
	rows := m.visibleWorkQueueRows()
	if len(rows) == 0 {
		m.selectInput()
		return
	}
	if m.SelectedIssue == nil && m.SelectedWorktree == "" && m.AddSubtaskSelected == "" {
		if delta > 0 {
			m.selectRow(rows[0])
		} else {
			m.selectRow(rows[len(rows)-1])
		}
		return
	}
	current := -1
	for i := range rows {
		row := rows[i]
		if (row.Kind == workQueueRowIssue && m.SelectedIssue != nil && row.Issue != nil && row.Issue.ID == m.SelectedIssue.ID) ||
			(row.Kind == workQueueRowWorktree && row.Worktree != nil && row.Worktree.Branch == m.SelectedWorktree) ||
			(row.Kind == workQueueRowAddSubtask && row.ParentID == m.AddSubtaskSelected) {
			current = i
			break
		}
	}
	next := current + delta
	if next < 0 || next >= len(rows) {
		m.selectInput()
		if delta < 0 {
			m.CreationMode = creationModeBranchOnly
		}
		return
	}
	m.selectRow(rows[next])
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

type worktreesLoadedMsg struct {
	worktrees []git.Worktree
}

type worktreesErrorMsg struct {
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

type issueUnassignedMsg struct {
	issueID string
}

type issueUnassignErrorMsg struct {
	err error
}

type issueReassignedMsg struct {
	issueID string
}

type issueReassignErrorMsg struct {
	err error
}

type issueDoneMsg struct {
	issueID string
}

type issueDoneErrorMsg struct {
	err error
}

type workQueueRowKind int

const (
	workQueueRowIssue workQueueRowKind = iota
	workQueueRowWorktree
	workQueueRowAddSubtask
)

type workQueueRow struct {
	Kind     workQueueRowKind
	Issue    *linear.Issue
	Worktree *git.Worktree
	ParentID string
	Closed   bool
	Updated  time.Time
}

const maxVisibleActiveRows = 20

func (m model) View() string {
	if m.Done {
		if m.Success {
			return successStyle.Render("✓ "+m.Result) + "\n\n" + helpStyle.Render("Press any key to exit.")
		} else {
			return errorStyle.Render("✗ Error: "+m.ErrorMsg) + "\n\n" + helpStyle.Render("Press any key to exit.")
		}
	}

	if m.PromptCaptureMode {
		return m.renderPromptCaptureView()
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
		if m.SelectedIssue == nil && m.SelectedWorktree == "" && m.AddSubtaskSelected == "" {
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
	if m.LinearLoading || m.WorktreesLoading {
		s.WriteString(fmt.Sprintf("%s Loading work queue...", m.Spinner.View()))
	} else if m.LinearError != "" {
		s.WriteString(errorStyle.Render("Error: " + m.LinearError))
	} else if m.WorktreesError != "" {
		s.WriteString(errorStyle.Render("Error: " + m.WorktreesError))
	} else {
		treeView := m.buildWorkQueueTree()
		if treeView != "" {
			trimmedTree := strings.TrimRight(treeView, "\n")
			s.WriteString(trimmedTree)
			s.WriteString("\n")
		} else if m.LinearClient != nil && !m.SearchMode {
			s.WriteString(helpStyle.Render("No assigned tickets found"))
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
	allLabel := ""
	if len(m.Worktrees) > 0 {
		allLabel = " [a all]"
		if m.ShowAllWorkItems {
			allLabel = " [a active]"
		}
	}
	hotkeys := modeLabel + allLabel + " [u unassign] [d done] [z undo]"
	s.WriteString(helpStyle.Render(hotkeys))

	return s.String()
}

func (m model) renderPromptCaptureView() string {
	status := "Creating worktree..."
	if m.PromptSubmitted && !m.CreationFinished {
		status = "Prompt queued, waiting for git..."
	} else if !m.PromptSubmitted && m.CreationFinished {
		status = "Worktree ready, press Enter to launch"
	}

	s := strings.Builder{}
	s.WriteString(headerStyle.Render("🌱 sprout"))
	s.WriteString("\n\n")
	if m.Creating {
		s.WriteString(fmt.Sprintf("%s %s", m.Spinner.View(), status))
	} else {
		s.WriteString(loadingStyle.Render(status))
	}
	s.WriteString("\n")

	s.WriteString(m.PromptInput.View())
	s.WriteString("\n")
	s.WriteString(helpStyle.Render("[enter submit] [alt+enter or shift+enter newline] [esc cancel]"))
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
	if len(m.Worktrees) > 0 && maxIdentifierWidth > 0 && maxIdentifierWidth < 8 {
		maxIdentifierWidth = 8
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

func (m model) buildWorkQueueTree() string {
	rows := m.visibleWorkQueueRows()
	if len(rows) == 0 {
		return ""
	}

	maxIdentifierWidth := 0
	maxStatusWidth := 0
	for _, row := range rows {
		if row.Issue == nil {
			continue
		}
		if width := lipgloss.Width(row.Issue.Identifier); width > maxIdentifierWidth {
			maxIdentifierWidth = width
		}
		if width := lipgloss.Width(row.Issue.State.Name); width > maxStatusWidth {
			maxStatusWidth = width
		}
	}
	if len(m.Worktrees) > 0 && maxIdentifierWidth > 0 && maxIdentifierWidth < 8 {
		maxIdentifierWidth = 8
	}

	var s strings.Builder
	for i, row := range rows {
		depth := rowDepth(row)
		s.WriteString(m.treePrefix(rows, i, depth))
		s.WriteString(m.renderWorkQueueRow(row, maxIdentifierWidth, maxStatusWidth))
		if i < len(rows)-1 {
			s.WriteString("\n")
		}
	}
	return s.String()
}

func rowDepth(row workQueueRow) int {
	if row.Issue != nil {
		return row.Issue.Depth
	}
	if row.Kind == workQueueRowAddSubtask {
		return 1
	}
	return 0
}

func (m model) treePrefix(rows []workQueueRow, index, depth int) string {
	var prefix strings.Builder
	for level := 0; level < depth; level++ {
		if hasLaterAtDepth(rows, index, level) {
			prefix.WriteString("│  ")
		} else {
			prefix.WriteString("   ")
		}
	}
	if hasLaterAtDepth(rows, index, depth) {
		prefix.WriteString("├──")
	} else {
		prefix.WriteString("└──")
	}
	return expandedStyle.Render(prefix.String())
}

func hasLaterAtDepth(rows []workQueueRow, index, depth int) bool {
	for i := index + 1; i < len(rows); i++ {
		nextDepth := rowDepth(rows[i])
		if nextDepth < depth {
			return false
		}
		if nextDepth == depth {
			return true
		}
	}
	return false
}

func (m model) renderWorkQueueRow(row workQueueRow, maxIdentifierWidth, maxStatusWidth int) string {
	var content string
	switch row.Kind {
	case workQueueRowWorktree:
		if row.Worktree != nil {
			content = titleStyle.Render(row.Worktree.Branch)
		}
	case workQueueRowAddSubtask:
		if parent := m.findIssueByID(row.ParentID); parent != nil && parent.ShowingSubtaskEntry {
			if m.SubtaskInputMode && m.SubtaskParentID == row.ParentID {
				content = m.SubtaskInput.View()
			} else {
				content = addSubtaskStyle.Render("+ " + parent.SubtaskEntryText)
			}
		} else {
			content = addSubtaskStyle.Render("+ Add subtask")
		}
	case workQueueRowIssue:
		if row.Issue != nil {
			content = m.renderIssueContent(*row.Issue, maxIdentifierWidth, maxStatusWidth)
		}
	}

	selected := false
	switch row.Kind {
	case workQueueRowIssue:
		selected = m.SelectedIssue != nil && row.Issue != nil && row.Issue.ID == m.SelectedIssue.ID
	case workQueueRowWorktree:
		selected = row.Worktree != nil && row.Worktree.Branch == m.SelectedWorktree
	case workQueueRowAddSubtask:
		selected = row.ParentID == m.AddSubtaskSelected
	}
	if selected {
		return selectedStyle.Render(content)
	}
	return normalStyle.Render(content)
}

func (m model) renderIssueContent(issue linear.Issue, maxIdentifierWidth, maxStatusWidth int) string {
	title := issue.Title
	statusText := issue.State.Name
	statusStyle := m.getStatusStyle(issue.State)
	styledStatus := statusStyle.Render(statusText)

	statusWidth := lipgloss.Width(statusText)
	treePrefixWidth := (issue.Depth + 1) * 3
	marginWidth := 15
	availableWidth := m.Width - maxIdentifierWidth - maxStatusWidth - treePrefixWidth - marginWidth
	if availableWidth < 20 {
		availableWidth = 20
	}
	if len(title) > availableWidth && availableWidth > 3 {
		title = title[:availableWidth-3] + "..."
	}

	identifier := identifierStyle.Render(issue.Identifier)
	titleText := titleStyle.Render(title)
	identifierPadding := maxIdentifierWidth - lipgloss.Width(issue.Identifier)
	statusPadding := maxStatusWidth - statusWidth
	return fmt.Sprintf("%s%s  %s%s  %s", identifier, strings.Repeat(" ", identifierPadding), styledStatus, strings.Repeat(" ", statusPadding), titleText)
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
	if resultModel, ok := finalModel.(model); ok && resultModel.Success && resultModel.WorktreePath != "" && resultModel.Resumed {
		repoName, _ := git.GetRepositoryName()
		resolvedCmd := config.ResolveResumeCommand(resultModel.ResumeCommandArgs, resultModel.DefaultCommandArgs, config.ResumeContext{
			WorktreePath: resultModel.WorktreePath,
			BranchName:   resultModel.ResumeBranch,
			RepoName:     repoName,
		})
		if len(resolvedCmd) > 0 {
			cmd := exec.Command(resolvedCmd[0], resolvedCmd[1:]...)
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
	} else if resultModel, ok := finalModel.(model); ok && resultModel.Success && resultModel.WorktreePath != "" {
		resolvedCmd := config.ResolveDefaultCommand(resultModel.DefaultCommandArgs, resultModel.CapturedPrompt)
		if len(resolvedCmd) > 0 {
			// Execute the default command in the worktree directory
			cmd := exec.Command(resolvedCmd[0], resolvedCmd[1:]...)
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
