package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
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

// TestCatwalkSimple tests using catwalk with a simple test file
func TestCatwalkSimple(t *testing.T) {
	// Create a more complete model for catwalk testing
	// This will use real NewTUI but will fail gracefully without git repo
	t.Skip("Catwalk test requires full git repository setup - skipping for now")
	
	// Uncomment and adjust when running in proper git repo:
	// model, err := NewTUI()
	// if err != nil {
	// 	t.Skip("NewTUI failed - likely not in git repo:", err)
	// }
	// 
	// catwalk.RunModel(t, "testdata/simple.txt", model, nil)
}