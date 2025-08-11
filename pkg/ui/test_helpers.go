package ui

import (
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
	
	return createTestModelWithIssues(testIssues)
}

// CreateTestModelTwoExpandedTrees creates a model with two expandable issues for testing expansion bugs
func CreateTestModelTwoExpandedTrees() (model, error) {
	// Create mock Linear issues with two different expandable trees
	testIssues := []linear.Issue{
		{
			ID:          "issue-1",
			Title:       "Feature A: User management system",
			Identifier:  "SPR-100",
			HasChildren: true,
			Expanded:    false,
			Children: []linear.Issue{
				{
					ID:         "issue-1-1",
					Title:      "Add user registration",
					Identifier: "SPR-101",
					Depth:      1,
				},
				{
					ID:         "issue-1-2",
					Title:      "Implement user authentication",
					Identifier: "SPR-102",
					Depth:      1,
				},
			},
		},
		{
			ID:          "issue-2",
			Title:       "Feature B: Dashboard and analytics",
			Identifier:  "SPR-200",
			HasChildren: true,
			Expanded:    false,
			Children: []linear.Issue{
				{
					ID:         "issue-2-1",
					Title:      "Create dashboard layout",
					Identifier: "SPR-201",
					Depth:      1,
				},
				{
					ID:         "issue-2-2",
					Title:      "Add analytics widgets",
					Identifier: "SPR-202",
					Depth:      1,
				},
				{
					ID:         "issue-2-3",
					Title:      "Implement data visualization",
					Identifier: "SPR-203",
					Depth:      1,
				},
			},
		},
		{
			ID:          "issue-3",
			Title:       "Bug fix: Payment processing errors",
			Identifier:  "SPR-300",
			HasChildren: false,
			Expanded:    false,
		},
	}
	
	return createTestModelWithIssues(testIssues)
}

// CreateTestModelWithIssues creates a test model with specific issues
func CreateTestModelWithIssues(issues []linear.Issue) (model, error) {
	return createTestModelWithIssues(issues)
}

// CreateTestModelWithFakeClient creates a test model with a fake Linear client
func CreateTestModelWithFakeClient(fakeClient *linear.FakeLinearClient) (model, error) {
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
		TextInput:         ti,
		SubtaskInput:      si,
		Submitted:         false,
		Creating:          false,
		Done:              false,
		Success:           false,
		Cancelled:         false,
		ErrorMsg:          "",
		Result:            "",
		WorktreePath:      "",
		WorktreeManager:   nil, // Skip for testing
		LinearClient:      fakeClient, // Use fake client
		LinearIssues:      nil, // Will be loaded via GetAssignedIssues()
		FlattenedIssues:   nil,
		LinearLoading:     true, // Start loading to simulate real behavior
		LinearError:       "",
		SelectedIndex:     -1, // Start with custom input selected
		InputMode:         true,
		CreatingSubtask:   false,
		SubtaskInputMode:  false,
		SubtaskParentID:   "",
	}
	
	// Initialize by loading issues from fake client to match real behavior
	issues, err := fakeClient.GetAssignedIssues()
	if err == nil {
		m.LinearIssues = issues
		m.LinearLoading = false
		m.flattenIssues()
	}
	
	return m, nil
}

// createTestModelWithIssues is the internal implementation
func createTestModelWithIssues(issues []linear.Issue) (model, error) {
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
	
	// Create a mock linear client for testing
	mockClient := linear.NewClient("test-key")
	
	// Create a basic model structure for testing
	m := model{
		TextInput:         ti,
		SubtaskInput:      si,
		Submitted:         false,
		Creating:          false,
		Done:              false,
		Success:           false,
		Cancelled:         false,
		ErrorMsg:          "",
		Result:            "",
		WorktreePath:      "",
		WorktreeManager:   nil, // Skip for testing
		LinearClient:      mockClient, // Use mock client so View() renders issues
		LinearIssues:      issues,
		FlattenedIssues:   nil,
		LinearLoading:     false,
		LinearError:       "",
		SelectedIndex:     -1, // Start with custom input selected
		InputMode:         true,
		CreatingSubtask:   false,
		SubtaskInputMode:  false,
		SubtaskParentID:   "",
	}
	
	// Flatten issues for navigation
	m.flattenIssues()
	
	return m, nil
}

// Minimal test model to isolate teatest usage
type MinimalModel struct{}

func (m MinimalModel) Init() tea.Cmd { return nil }
func (m MinimalModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { 
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC || msg.Type == tea.KeyEsc {
			return m, tea.Quit
		}
	}
	return m, nil 
}
func (m MinimalModel) View() string { return "Hello World" }

// SendKey helper function for cleaner test code
func SendKey(m model, keyType tea.KeyType) model {
	updatedModel, _ := m.Update(tea.KeyMsg{Type: keyType})
	return updatedModel.(model)
}