package models

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"sprout/pkg/application/services"
	"sprout/pkg/domain/issue"
	"sprout/pkg/shared/errors"
	"sprout/pkg/shared/logging"
)

// Component represents a UI component that can be rendered and updated
type Component interface {
	// Update handles messages and updates component state
	Update(msg tea.Msg, state *AppState) (tea.Cmd, error)
	
	// View renders the component
	View(state *AppState) string
	
	// Init returns initial commands for the component
	Init() tea.Cmd
	
	// Focus sets focus to this component
	Focus()
	
	// Blur removes focus from this component
	Blur()
	
	// IsFocused returns true if this component is focused
	IsFocused() bool
}

// MainModel is the root model that orchestrates all components
type MainModel struct {
	state            *AppState
	inputComponent   Component
	issueComponent   Component
	spinnerComponent Component
	statusComponent  Component
	
	// Services
	worktreeService *services.WorktreeService
	issueService    *services.IssueService
	logger          logging.Logger
	
	// Internal state
	quitting bool
}

// NewMainModel creates a new main model
func NewMainModel(
	state *AppState,
	inputComponent Component,
	issueComponent Component,
	spinnerComponent Component,
	statusComponent Component,
	worktreeService *services.WorktreeService,
	issueService *services.IssueService,
	logger logging.Logger,
) *MainModel {
	return &MainModel{
		state:            state,
		inputComponent:   inputComponent,
		issueComponent:   issueComponent,
		spinnerComponent: spinnerComponent,
		statusComponent:  statusComponent,
		worktreeService:  worktreeService,
		issueService:     issueService,
		logger:           logger,
	}
}

// Init initializes the model and all components
func (m *MainModel) Init() tea.Cmd {
	var cmds []tea.Cmd
	
	// Initialize all components
	if m.inputComponent != nil {
		if cmd := m.inputComponent.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	
	if m.issueComponent != nil {
		if cmd := m.issueComponent.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	
	if m.spinnerComponent != nil {
		if cmd := m.spinnerComponent.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	
	if m.statusComponent != nil {
		if cmd := m.statusComponent.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	
	if len(cmds) > 0 {
		return tea.Batch(cmds...)
	}
	
	return nil
}

// Update handles all messages and coordinates component updates
func (m *MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.quitting {
		return m, tea.Quit
	}
	
	var cmds []tea.Cmd
	
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.handleWindowResize(msg)
		
	case tea.KeyMsg:
		if cmd := m.handleKeyPress(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
		
	case WorktreeCreatedMsg:
		m.handleWorktreeCreated(msg)
		
	case WorktreeErrorMsg:
		m.handleWorktreeError(msg)
		
	default:
		// Forward message to all components
		if cmd := m.updateComponent(m.inputComponent, msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
		
		if cmd := m.updateComponent(m.issueComponent, msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
		
		if cmd := m.updateComponent(m.spinnerComponent, msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
		
		if cmd := m.updateComponent(m.statusComponent, msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
		
		// Handle search filtering if in search mode
		if m.state.SearchMode && m.issueComponent != nil {
			if issueList, ok := m.issueComponent.(interface{ FilterIssues(*AppState, string) }); ok {
				issueList.FilterIssues(m.state, m.state.SearchQuery)
			}
		}
	}
	
	if len(cmds) > 0 {
		return m, tea.Batch(cmds...)
	}
	
	return m, nil
}

// View renders the entire application
func (m *MainModel) View() string {
	if m.quitting {
		return ""
	}
	
	// If we're showing a result, only show the status component
	if m.state.IsInResultMode() {
		return m.statusComponent.View(m.state)
	}
	
	// If we're loading, show spinner
	if m.state.IsLoading() {
		return m.spinnerComponent.View(m.state)
	}
	
	// Build the main interface
	var sections []string
	
	// Header
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("69")).
		Bold(true).
		MarginBottom(1)
	sections = append(sections, headerStyle.Render("🌱 sprout"))
	
	// Input component
	if m.inputComponent != nil {
		sections = append(sections, m.inputComponent.View(m.state))
	}
	
	// Issue list component (if available)
	if m.issueComponent != nil {
		if issueView := m.issueComponent.View(m.state); issueView != "" {
			sections = append(sections, issueView)
		}
	}
	
	return strings.Join(sections, "\n")
}

// handleWindowResize handles terminal window size changes
func (m *MainModel) handleWindowResize(msg tea.WindowSizeMsg) {
	m.state.SetWindowSize(msg.Width, msg.Height)
	
	// Update input component width if available
	if inputComp, ok := m.inputComponent.(interface{ SetWidth(int) }); ok {
		inputComp.SetWidth(msg.Width)
	}
}

// handleKeyPress handles keyboard input
func (m *MainModel) handleKeyPress(msg tea.KeyMsg) tea.Cmd {
	// Global key handlers
	switch msg.Type {
	case tea.KeyCtrlC:
		m.quitting = true
		return tea.Quit
		
	case tea.KeyEsc:
		return m.handleEscape()
		
	case tea.KeyEnter:
		return m.handleEnter()
	}
	
	// Mode-specific key handlers
	switch msg.String() {
	case "/":
		if !m.state.SearchMode && !m.state.SubtaskMode {
			return m.enterSearchMode()
		}
	}
	
	// Navigation keys
	switch msg.Type {
	case tea.KeyUp, tea.KeyDown:
		return m.handleNavigation(msg)
		
	case tea.KeyLeft, tea.KeyRight:
		return m.handleHorizontalNavigation(msg)
	}
	
	// Forward to input component for text input
	if m.inputComponent != nil && (m.state.InputFocused || m.state.SearchMode) {
		if cmd := m.updateComponent(m.inputComponent, msg); cmd != nil {
			return cmd
		}
	}
	
	return nil
}

// handleEscape handles the escape key
func (m *MainModel) handleEscape() tea.Cmd {
	switch {
	case m.state.SearchMode:
		m.exitSearchMode()
	case m.state.SubtaskMode:
		m.exitSubtaskMode()
	case m.state.IsInResultMode():
		m.quitting = true
		return tea.Quit
	default:
		m.quitting = true
		return tea.Quit
	}
	
	return nil
}

// handleEnter handles the enter key
func (m *MainModel) handleEnter() tea.Cmd {
	switch m.state.Mode {
	case ModeInput, ModeIssueSelection:
		return m.createWorktree()
		
	case ModeSubtaskInput:
		return m.createSubtask()
		
	case ModeResult:
		m.quitting = true
		return tea.Quit
	}
	
	return nil
}

// handleNavigation handles up/down arrow keys
func (m *MainModel) handleNavigation(msg tea.KeyMsg) tea.Cmd {
	if m.state.SearchMode || m.state.Mode == ModeIssueSelection {
		// Forward to issue component
		if m.issueComponent != nil {
			return m.updateComponent(m.issueComponent, msg)
		}
	}
	
	return nil
}

// handleHorizontalNavigation handles left/right arrow keys
func (m *MainModel) handleHorizontalNavigation(msg tea.KeyMsg) tea.Cmd {
	if m.state.Mode == ModeIssueSelection {
		// Forward to issue component for expand/collapse
		if m.issueComponent != nil {
			return m.updateComponent(m.issueComponent, msg)
		}
	}
	
	return nil
}

// enterSearchMode switches to search mode
func (m *MainModel) enterSearchMode() tea.Cmd {
	m.state.SetMode(ModeSearch)
	
	if inputComp, ok := m.inputComponent.(interface{ SetValue(string) }); ok {
		inputComp.SetValue("/")
	}
	
	return nil
}

// exitSearchMode exits search mode
func (m *MainModel) exitSearchMode() {
	m.state.SetMode(ModeInput)
	m.state.SearchQuery = ""
	m.state.FilteredIssues = m.state.Issues
	m.state.SelectedIssue = nil
	
	if inputComp, ok := m.inputComponent.(interface{ SetValue(string); Focus() }); ok {
		inputComp.SetValue("")
		inputComp.Focus()
	}
}

// exitSubtaskMode exits subtask input mode
func (m *MainModel) exitSubtaskMode() {
	m.state.SetMode(ModeIssueSelection)
	m.state.SubtaskInput = ""
}

// createWorktree starts worktree creation
func (m *MainModel) createWorktree() tea.Cmd {
	branchName := m.state.GetBranchName()
	if strings.TrimSpace(branchName) == "" {
		return nil
	}
	
	m.state.CreatingWorktree = true
	m.state.SetMode(ModeLoading)
	
	return func() tea.Msg {
		ctx := context.Background()
		path, err := m.worktreeService.CreateWorktree(ctx, branchName)
		if err != nil {
			return WorktreeErrorMsg{Error: err}
		}
		return WorktreeCreatedMsg{
			Branch: branchName,
			Path:   path,
		}
	}
}

// createSubtask starts subtask creation
func (m *MainModel) createSubtask() tea.Cmd {
	if m.state.SelectedIssue == nil || m.issueService == nil {
		return nil
	}
	
	title := strings.TrimSpace(m.state.SubtaskInput)
	if title == "" {
		return nil
	}
	
	m.state.CreatingSubtask = true
	m.state.SetMode(ModeLoading)
	
	parentID := m.state.SelectedIssue.ID
	
	return func() tea.Msg {
		ctx := context.Background()
		subtask, err := m.issueService.CreateSubtask(ctx, parentID, title)
		if err != nil {
			return SubtaskErrorMsg{Error: err}
		}
		return SubtaskCreatedMsg{
			ParentID: parentID,
			Subtask:  subtask,
		}
	}
}

// handleWorktreeCreated handles successful worktree creation
func (m *MainModel) handleWorktreeCreated(msg WorktreeCreatedMsg) {
	m.state.CreatingWorktree = false
	m.state.WorktreePath = msg.Path
	m.state.SetSuccess(fmt.Sprintf("Worktree created at: %s", msg.Path))
}

// handleWorktreeError handles worktree creation errors
func (m *MainModel) handleWorktreeError(msg WorktreeErrorMsg) {
	m.state.CreatingWorktree = false
	
	// Extract user-friendly error message
	var errorMsg string
	if sproutErr, ok := msg.Error.(*errors.SproutError); ok {
		errorMsg = sproutErr.Message
	} else {
		errorMsg = msg.Error.Error()
	}
	
	m.state.SetError(errorMsg)
}

// updateComponent safely updates a component
func (m *MainModel) updateComponent(comp Component, msg tea.Msg) tea.Cmd {
	if comp == nil {
		return nil
	}
	
	cmd, err := comp.Update(msg, m.state)
	if err != nil {
		m.logger.Error("component update failed", "error", err)
		return nil
	}
	
	return cmd
}

// Message types for worktree operations
type WorktreeCreatedMsg struct {
	Branch string
	Path   string
}

type WorktreeErrorMsg struct {
	Error error
}

type SubtaskCreatedMsg struct {
	ParentID string
	Subtask  *issue.Issue
}

type SubtaskErrorMsg struct {
	Error error
}