package ui

import (
	"io"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/muesli/termenv"
	"sprout/pkg/git"
)


// TestBasicFunctionality runs a simple test to verify the model works
func TestBasicFunctionality(t *testing.T) {
	m, err := CreateTestModel()
	if err != nil {
		t.Fatal(err)
	}

	// Test that the model initializes correctly
	if m.SelectedIndex != -1 {
		t.Errorf("Expected SelectedIndex to be -1, got %d", m.SelectedIndex)
	}

	if !m.InputMode {
		t.Error("Expected InputMode to be true")
	}

	if len(m.LinearIssues) != 3 {
		t.Errorf("Expected 3 linear issues, got %d", len(m.LinearIssues))
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

	if m.SelectedIndex != 0 {
		t.Errorf("Expected SelectedIndex to be 0, got %d", m.SelectedIndex)
	}

	if m.InputMode {
		t.Error("Expected InputMode to be false after navigation")
	}

	// Test moving back up to input
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = newModel.(model)

	if m.SelectedIndex != -1 {
		t.Errorf("Expected SelectedIndex to be -1, got %d", m.SelectedIndex)
	}

	if !m.InputMode {
		t.Error("Expected InputMode to be true after navigation back to input")
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
	if model.WorktreeManager == nil {
		t.Fatal("WorktreeManager is nil")
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


// TestTeatestMinimalGolden tests teatest with golden file output
func TestTeatestMinimalGolden(t *testing.T) {
	// Set consistent color profile for testing
	lipgloss.SetColorProfile(termenv.Ascii)

	model := MinimalModel{}
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(80, 24))

	// Send a quit message to make the program exit
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})

	// Capture output and compare with golden file
	out, err := io.ReadAll(tm.FinalOutput(t))
	if err != nil {
		t.Fatal(err)
	}
	teatest.RequireEqualOutput(t, out)
}

// TestTeatestGoldenNavigation tests navigation with deterministic test model
func TestTeatestGoldenNavigation(t *testing.T) {
	// Set consistent color profile for testing
	lipgloss.SetColorProfile(termenv.Ascii)

	// Use CreateTestModel for deterministic behavior
	model, err := CreateTestModel()
	if err != nil {
		t.Fatalf("CreateTestModel failed: %v", err)
	}

	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(80, 24))

	// Test navigation: down arrow to select first ticket
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})

	// Send quit message to exit
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})

	// Capture output and compare with golden file
	out, err := io.ReadAll(tm.FinalOutput(t))
	if err != nil {
		t.Fatal(err)
	}
	teatest.RequireEqualOutput(t, out)
}

// TestTeatestGoldenInteraction tests user interactions with deterministic model
func TestTeatestGoldenInteraction(t *testing.T) {
	// Set consistent color profile for testing
	lipgloss.SetColorProfile(termenv.Ascii)

	// Use CreateTestModel for deterministic behavior
	model, err := CreateTestModel()
	if err != nil {
		t.Fatalf("CreateTestModel failed: %v", err)
	}

	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(80, 24))

	// Test navigation: down arrow to select first ticket, then up to go back
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyUp})

	// Send quit message to exit
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})

	// Capture output and compare with golden file
	out, err := io.ReadAll(tm.FinalOutput(t))
	if err != nil {
		t.Fatal(err)
	}
	teatest.RequireEqualOutput(t, out)
}


// TestTeatestGoldenTwoExpandedTrees tests the bug with two different sub-trees expanded
func TestTeatestGoldenTwoExpandedTrees(t *testing.T) {
	// Set consistent color profile for testing
	lipgloss.SetColorProfile(termenv.Ascii)

	// Use CreateTestModelTwoExpandedTrees for testing multiple expanded trees
	model, err := CreateTestModelTwoExpandedTrees()
	if err != nil {
		t.Fatalf("CreateTestModelTwoExpandedTrees failed: %v", err)
	}

	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(80, 30))

	// Navigate to first expandable issue (Feature A)
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})  // Move to first issue
	tm.Send(tea.KeyMsg{Type: tea.KeyRight}) // Expand first issue

	// Navigate to second expandable issue (Feature B)
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})  // Move past first subtask
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})  // Move past second subtask
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})  // Move past "+ Add subtask"
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})  // Move to Feature B
	tm.Send(tea.KeyMsg{Type: tea.KeyRight}) // Expand second issue

	// Force a redraw to ensure screen is updated
	tm.Send(tea.WindowSizeMsg{Width: 80, Height: 30})

	// Now quit the program - this should capture the final screen state
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})

	// Capture output and compare with golden file
	out, err := io.ReadAll(tm.FinalOutput(t))
	if err != nil {
		t.Fatal(err)
	}
	teatest.RequireEqualOutput(t, out)
}


// TestTwoExpandedTreesModelState tests the internal state when two sub-trees are expanded
func TestTwoExpandedTreesModelState(t *testing.T) {
	// Use CreateTestModelTwoExpandedTrees for testing multiple expanded trees
	model, err := CreateTestModelTwoExpandedTrees()
	if err != nil {
		t.Fatalf("CreateTestModelTwoExpandedTrees failed: %v", err)
	}

	// Navigate to first expandable issue (Feature A) and expand it
	model = SendKey(model, tea.KeyDown)  // Move to first issue
	model = SendKey(model, tea.KeyRight) // Expand first issue

	// Check first issue is expanded
	if !model.LinearIssues[0].Expanded {
		t.Error("First issue should be expanded")
	}

	// Navigate to second expandable issue (Feature B)
	model = SendKey(model, tea.KeyDown)  // Move past first subtask
	model = SendKey(model, tea.KeyDown)  // Move past second subtask
	model = SendKey(model, tea.KeyDown)  // Move past "+ Add subtask"
	model = SendKey(model, tea.KeyDown)  // Move to Feature B
	model = SendKey(model, tea.KeyRight) // Expand second issue

	// Check both issues are expanded
	if !model.LinearIssues[0].Expanded {
		t.Error("First issue should still be expanded after expanding second")
	}
	if !model.LinearIssues[1].Expanded {
		t.Error("Second issue should be expanded")
	}

	// Check flattened issues include children from both trees
	// Expected: 3 main issues + 2 children from first + 1 placeholder + 3 children from second + 1 placeholder = 10
	expectedCount := 10
	if len(model.FlattenedIssues) != expectedCount {
		t.Errorf("Expected %d flattened issues when both trees expanded, got %d", expectedCount, len(model.FlattenedIssues))

		// Debug: print the flattened issues
		t.Log("Flattened issues:")
		for i, issue := range model.FlattenedIssues {
			t.Logf("  [%d] %s (ID: %s, Depth: %d, IsAddSubtask: %v)", i, issue.Title, issue.ID, issue.Depth, issue.IsAddSubtask)
		}
	}

	// Generate view to capture the visual state in the test output
	view := model.View()

	// Check that both expanded trees are visible in the view
	if !strings.Contains(view, "Feature A: User management system") {
		t.Error("View should contain Feature A")
	}
	if !strings.Contains(view, "Add user registration") {
		t.Error("View should contain Feature A's first child")
	}
	if !strings.Contains(view, "Implement user authentication") {
		t.Error("View should contain Feature A's second child")
	}
	if !strings.Contains(view, "Feature B: Dashboard and analytics") {
		t.Error("View should contain Feature B")
	}
	if !strings.Contains(view, "Create dashboard layout") {
		t.Error("View should contain Feature B's first child")
	}
	if !strings.Contains(view, "Add analytics widgets") {
		t.Error("View should contain Feature B's second child")
	}
	if !strings.Contains(view, "Implement data visualization") {
		t.Error("View should contain Feature B's third child")
	}

	// Log the view for debugging
	t.Log("Final view state with both trees expanded:")
	t.Log(view)
}
