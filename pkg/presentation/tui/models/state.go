package models

import (
	"sprout/pkg/domain/issue"
	"sprout/pkg/domain/project"
	"sprout/pkg/infrastructure/config"
)

// AppState represents the global application state
type AppState struct {
	// Project information
	Project *project.Project
	Config  *config.Config
	
	// UI state
	Mode            AppMode
	Width           int
	Height          int
	
	// Content state
	Issues          []*issue.Issue
	FilteredIssues  []*issue.Issue
	SelectedIssue   *issue.Issue
	
	// Input state
	CustomInput     string
	SearchQuery     string
	SubtaskInput    string
	
	// Navigation state
	InputFocused    bool
	SearchMode      bool
	SubtaskMode     bool
	
	// Loading states
	LoadingIssues   bool
	CreatingWorktree bool
	CreatingSubtask bool
	
	// Result state
	WorktreePath    string
	ErrorMessage    string
	SuccessMessage  string
	
	// UI preferences
	ShowMainBranches bool
}

// AppMode represents the current mode of the application
type AppMode int

const (
	ModeInput AppMode = iota
	ModeIssueSelection
	ModeSearch
	ModeSubtaskInput
	ModeLoading
	ModeResult
)

// NewAppState creates a new application state
func NewAppState(project *project.Project, config *config.Config) *AppState {
	return &AppState{
		Project:          project,
		Config:           config,
		Mode:             ModeInput,
		Width:            80,
		Height:           24,
		Issues:           make([]*issue.Issue, 0),
		FilteredIssues:   make([]*issue.Issue, 0),
		InputFocused:     true,
		ShowMainBranches: false,
	}
}

// Reset resets the state for a new operation
func (s *AppState) Reset() {
	s.Mode = ModeInput
	s.SelectedIssue = nil
	s.CustomInput = ""
	s.SearchQuery = ""
	s.SubtaskInput = ""
	s.InputFocused = true
	s.SearchMode = false
	s.SubtaskMode = false
	s.LoadingIssues = false
	s.CreatingWorktree = false
	s.CreatingSubtask = false
	s.WorktreePath = ""
	s.ErrorMessage = ""
	s.SuccessMessage = ""
}

// SetMode changes the current application mode
func (s *AppState) SetMode(mode AppMode) {
	s.Mode = mode
	
	switch mode {
	case ModeInput:
		s.InputFocused = true
		s.SearchMode = false
		s.SubtaskMode = false
	case ModeIssueSelection:
		s.InputFocused = false
		s.SearchMode = false
		s.SubtaskMode = false
	case ModeSearch:
		s.InputFocused = true
		s.SearchMode = true
		s.SubtaskMode = false
	case ModeSubtaskInput:
		s.InputFocused = false
		s.SearchMode = false
		s.SubtaskMode = true
	case ModeLoading:
		s.InputFocused = false
		s.SearchMode = false
		s.SubtaskMode = false
	case ModeResult:
		s.InputFocused = false
		s.SearchMode = false
		s.SubtaskMode = false
	}
}

// SetWindowSize updates the window dimensions
func (s *AppState) SetWindowSize(width, height int) {
	s.Width = width
	s.Height = height
}

// SetIssues updates the issue list and resets filtered issues
func (s *AppState) SetIssues(issues []*issue.Issue) {
	s.Issues = issues
	s.FilteredIssues = issues
	s.LoadingIssues = false
}

// SetFilteredIssues updates the filtered issue list (for search)
func (s *AppState) SetFilteredIssues(issues []*issue.Issue) {
	s.FilteredIssues = issues
}

// GetCurrentIssues returns the appropriate issue list based on current mode
func (s *AppState) GetCurrentIssues() []*issue.Issue {
	if s.SearchMode {
		return s.FilteredIssues
	}
	return s.Issues
}

// SetError sets an error message and switches to result mode
func (s *AppState) SetError(message string) {
	s.ErrorMessage = message
	s.SuccessMessage = ""
	s.SetMode(ModeResult)
}

// SetSuccess sets a success message and switches to result mode
func (s *AppState) SetSuccess(message string) {
	s.SuccessMessage = message
	s.ErrorMessage = ""
	s.SetMode(ModeResult)
}

// ClearMessages clears both error and success messages
func (s *AppState) ClearMessages() {
	s.ErrorMessage = ""
	s.SuccessMessage = ""
}

// HasIssues returns true if there are issues available
func (s *AppState) HasIssues() bool {
	return len(s.Issues) > 0
}

// HasLinearIntegration returns true if Linear integration is configured
func (s *AppState) HasLinearIntegration() bool {
	return s.Config != nil && s.Config.IsLinearConfigured()
}

// GetBranchName returns the branch name to create based on current selection
func (s *AppState) GetBranchName() string {
	if s.SelectedIssue != nil {
		return s.SelectedIssue.GetBranchName()
	}
	return s.CustomInput
}

// IsInResultMode returns true if we're showing a result (success or error)
func (s *AppState) IsInResultMode() bool {
	return s.Mode == ModeResult
}

// IsLoading returns true if any loading state is active
func (s *AppState) IsLoading() bool {
	return s.LoadingIssues || s.CreatingWorktree || s.CreatingSubtask
}

// GetPlaceholderText returns appropriate placeholder text for the input
func (s *AppState) GetPlaceholderText() string {
	switch {
	case s.SearchMode:
		return "type to fuzzy search"
	case s.SelectedIssue != nil:
		return s.SelectedIssue.GetBranchName()
	default:
		return "enter branch name or select suggestion below"
	}
}