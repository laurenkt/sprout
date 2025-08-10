package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"sprout/pkg/git"
	"sprout/pkg/linear"
)

// CreateTestModel creates a simplified model for testing
func CreateTestModel() (model, error) {
	// Create mock Linear issues for testing
	testIssues := []linear.Issue{
		{
			ID:          "issue-1",
			Title:       "Add user authentication",
			Identifier:  "SPR-123",
			HasChildren: false,
			Expanded:    false,
		},
		{
			ID:          "issue-2",
			Title:       "Implement dashboard with analytics and reporting features",
			Identifier:  "SPR-124",
			HasChildren: true,
			Expanded:    false,
			Children: []linear.Issue{
				{
					ID:         "issue-2-1",
					Title:      "Create analytics component",
					Identifier: "SPR-125",
					Depth:      1,
				},
				{
					ID:         "issue-2-2",
					Title:      "Add reporting metrics",
					Identifier: "SPR-126",
					Depth:      1,
				},
			},
		},
		{
			ID:          "issue-3",
			Title:       "Fix critical bug in payment processing",
			Identifier:  "SPR-127",
			HasChildren: false,
			Expanded:    false,
		},
	}
	
	// Initialize text inputs
	ti := textinput.New()
	ti.Placeholder = "enter branch name or select suggestion below"
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 80
	ti.Prompt = "> "
	
	si := textinput.New()
	si.Placeholder = "enter subtask title"
	si.CharLimit = 100
	si.Width = 50
	si.Prompt = ""
	
	// Create a basic model structure for testing
	m := model{
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
		worktreeManager:   nil, // Skip for testing
		linearClient:      nil, // Skip for testing
		linearIssues:      testIssues,
		flattenedIssues:   nil,
		linearLoading:     false,
		linearError:       "",
		selectedIndex:     -1, // Start with custom input selected
		inputMode:         true,
		creatingSubtask:   false,
		subtaskInputMode:  false,
		subtaskParentID:   "",
	}
	
	// Flatten issues for navigation
	m.flattenIssues()
	
	return m, nil
}

// TestBasicFunctionality runs a simple test to verify the model works
func TestBasicFunctionality(t *testing.T) {
	m, err := CreateTestModel()
	if err != nil {
		t.Fatal(err)
	}
	
	// Test that the model initializes correctly
	if m.selectedIndex != -1 {
		t.Errorf("Expected selectedIndex to be -1, got %d", m.selectedIndex)
	}
	
	if !m.inputMode {
		t.Error("Expected inputMode to be true")
	}
	
	if len(m.linearIssues) != 3 {
		t.Errorf("Expected 3 linear issues, got %d", len(m.linearIssues))
	}
	
	// Test view rendering
	view := m.View()
	if view == "" {
		t.Error("View should not be empty")
	}
}

// TestNavigation tests basic navigation functionality
func TestNavigation(t *testing.T) {
	m, err := CreateTestModel()
	if err != nil {
		t.Fatal(err)
	}
	
	// Test moving down from input to first ticket
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(model)
	
	if m.selectedIndex != 0 {
		t.Errorf("Expected selectedIndex to be 0, got %d", m.selectedIndex)
	}
	
	if m.inputMode {
		t.Error("Expected inputMode to be false after navigation")
	}
	
	// Test moving back up to input
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = newModel.(model)
	
	if m.selectedIndex != -1 {
		t.Errorf("Expected selectedIndex to be -1, got %d", m.selectedIndex)
	}
	
	if !m.inputMode {
		t.Error("Expected inputMode to be true after navigation back to input")
	}
}

// TestMockWorktreeManager tests the mock worktree manager functionality
func TestMockWorktreeManager(t *testing.T) {
	// Create a mock worktree manager for testing
	mockWM := git.NewMockWorktreeManager("/tmp/test-repo")
	
	// Test worktree creation
	path, err := mockWM.CreateWorktree("test-branch")
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}
	if path == "" {
		t.Error("CreateWorktree returned empty path")
	}
	
	// Test worktree listing
	worktrees, err := mockWM.ListWorktrees()
	if err != nil {
		t.Fatalf("ListWorktrees failed: %v", err)
	}
	if len(worktrees) < 3 { // Should have 2 initial + 1 created
		t.Errorf("Expected at least 3 worktrees, got %d", len(worktrees))
	}
}

// TestTUIWithMock tests the TUI with mock dependencies
func TestTUIWithMock(t *testing.T) {
	// Create a mock worktree manager for testing
	mockWM := git.NewMockWorktreeManager("/tmp/test-repo")
	
	// Create model with mock dependencies
	model, err := NewTUIWithManager(mockWM)
	if err != nil {
		t.Fatalf("NewTUIWithManager failed: %v", err)
	}
	
	// Test basic model functionality
	if model.worktreeManager == nil {
		t.Fatal("worktreeManager is nil")
	}
	
	// Test that the model can be initialized
	cmd := model.Init()
	if cmd == nil {
		t.Log("Init returned nil command (this is fine)")
	}
	
	// Test that View doesn't panic
	view := model.View()
	if view == "" {
		t.Error("View returned empty string")
	}
	t.Log("TUI model works correctly with mock")
	
	// Test calling View multiple times to ensure stability
	for i := 0; i < 5; i++ {
		view := model.View()
		if view == "" {
			t.Errorf("View returned empty string on iteration %d", i)
		}
	}
	
	// Test Update with different message types
	updatedModel, updateCmd := model.Update(nil)
	if updatedModel == nil {
		t.Error("Update returned nil model")
	}
	if updateCmd != nil {
		t.Log("Update returned a command")
	}
}

// Minimal test model to isolate teatest usage
type minimalModel struct{}

func (m minimalModel) Init() tea.Cmd { return nil }
func (m minimalModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { 
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC || msg.Type == tea.KeyEsc {
			return m, tea.Quit
		}
	}
	return m, nil 
}
func (m minimalModel) View() string { return "Hello World" }

// TestTeatestMinimal tests teatest with a minimal model first
func TestTeatestMinimal(t *testing.T) {
	model := minimalModel{}
	t.Log("Testing minimal model with teatest...")
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(80, 24))
	
	// Send a quit message to make the program exit
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	
	// Test that the model can be created and produces output
	output := tm.FinalOutput(t)
	if output == nil {
		t.Error("Expected output from model")
	}
	
	// Check final model state
	finalModel := tm.FinalModel(t)
	if finalModel == nil {
		t.Error("Expected final model")
	}
}

// TestTeatestSimple tests using teatest with the full model
func TestTeatestSimple(t *testing.T) {
	// Create a mock worktree manager for testing
	mockWM := git.NewMockWorktreeManager("/tmp/test-repo")
	
	// Create model with mock dependencies
	model, err := NewTUIWithManager(mockWM)
	if err != nil {
		t.Fatalf("NewTUIWithManager failed: %v", err)
	}
	
	// Verify model implements tea.Model interface properly
	var teaModel tea.Model = model
	if teaModel == nil {
		t.Fatal("Model does not implement tea.Model interface")
	}
	
	// Test each method individually first
	t.Log("Testing Init()...")
	initCmd := model.Init()
	t.Logf("Init() returned: %v", initCmd)
	
	t.Log("Testing View()...")
	view := model.View()
	t.Logf("View() returned %d characters", len(view))
	
	t.Log("Testing Update()...")
	updatedModel, updateCmd := model.Update(nil)
	t.Logf("Update() returned model and cmd: %v", updateCmd)
	_ = updatedModel
	
	// Now test with teatest
	t.Log("Running teatest...")
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(80, 24))
	
	// Test basic navigation interaction first
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyUp})
	
	// Send quit message to exit
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	
	// Test that we can get output
	output := tm.FinalOutput(t)
	if output == nil {
		t.Error("Expected output from model")
	}
	
	// Test that we can access the final model
	finalModel := tm.FinalModel(t)
	if finalModel == nil {
		t.Error("Expected final model")
	}
	
	t.Log("Teatest completed successfully")
}