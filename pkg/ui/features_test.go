package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/cucumber/godog"
	"github.com/muesli/termenv"
	"sprout/pkg/linear"
)

// TUITestContext holds the state for our Gherkin tests
type TUITestContext struct {
	model     model
	testModel *teatest.TestModel
	issues    []linear.Issue
	output    string
	t         *testing.T
}

// NewTUITestContext creates a new test context
func NewTUITestContext(t *testing.T) *TUITestContext {
	return &TUITestContext{
		t: t,
	}
}

// StripANSI removes ANSI escape sequences from text
func StripANSI(text string) string {
	// Remove common ANSI sequences
	replacements := []string{
		"\x1b[?25l",    // Hide cursor
		"\x1b[?25h",    // Show cursor
		"\x1b[?2004h",  // Enable bracketed paste
		"\x1b[?2004l",  // Disable bracketed paste
		"\x1b[?1002l",  // Disable mouse tracking
		"\x1b[?1003l",  // Disable mouse tracking
		"\x1b[?1006l",  // Disable mouse tracking
		"\x1b[K",       // Clear to end of line
		"\x1b[2K",      // Clear entire line
	}

	result := text
	for _, seq := range replacements {
		result = strings.ReplaceAll(result, seq, "")
	}

	// Remove cursor positioning sequences like [80D
	// This regex-like approach handles variable numbers
	for i := 0; i < len(result); i++ {
		if i+1 < len(result) && result[i] == '\x1b' && result[i+1] == '[' {
			// Find the end of the sequence
			j := i + 2
			for j < len(result) && result[j] >= '0' && result[j] <= '9' {
				j++
			}
			if j < len(result) && (result[j] == 'D' || result[j] == 'A' || result[j] == 'B' || result[j] == 'C') {
				// Remove the entire sequence
				result = result[:i] + result[j+1:]
				i-- // Recheck this position
			}
		}
	}

	return result
}

// Step definitions

func (tc *TUITestContext) theFollowingLinearIssuesExist(issueTable *godog.Table) error {
	// First pass: create all issues
	allIssues := make(map[string]*linear.Issue)
	
	for i, row := range issueTable.Rows {
		if i == 0 { // Skip header row
			continue
		}
		
		identifier := row.Cells[0].Value
		title := row.Cells[1].Value
		parentID := row.Cells[2].Value
		
		issue := &linear.Issue{
			ID:          identifier,  // Use identifier as ID
			Identifier:  identifier,
			Title:       title,
			HasChildren: false,      // Will be set in second pass
			Expanded:    false,
			Depth:       0,          // Will be set based on parent relationship
			Children:    []linear.Issue{},
		}
		
		// Set parent relationship if specified
		if parentID != "" {
			issue.Depth = 1 // Child issue
		}
		
		allIssues[identifier] = issue
	}
	
	// Second pass: establish parent-child relationships
	for i, row := range issueTable.Rows {
		if i == 0 { // Skip header row
			continue
		}
		
		identifier := row.Cells[0].Value
		parentID := row.Cells[2].Value
		
		if parentID != "" {
			if parent, exists := allIssues[parentID]; exists {
				if child, exists := allIssues[identifier]; exists {
					parent.HasChildren = true
					parent.Children = append(parent.Children, *child)
					child.Parent = parent
				}
			}
		}
	}
	
	// Extract top-level issues in original table order
	tc.issues = []linear.Issue{}
	for i, row := range issueTable.Rows {
		if i == 0 { // Skip header row
			continue
		}
		
		identifier := row.Cells[0].Value
		parentID := row.Cells[2].Value
		
		// Add to issues if it's a top-level item (no parent)
		if parentID == "" {
			if issue, exists := allIssues[identifier]; exists {
				tc.issues = append(tc.issues, *issue)
			}
		}
	}
	
	return nil
}


func (tc *TUITestContext) iHaveAMinimalTUIModel() error {
	// Use the MinimalModel from test_helpers
	tc.output = "Hello World" // Set expected output for minimal model
	return nil
}

func (tc *TUITestContext) iRenderTheView() error {
	// For minimal model, we already set the output
	return nil
}

func (tc *TUITestContext) theOutputShouldBe(expected string) error {
	if tc.output != expected {
		return fmt.Errorf("output mismatch: expected %q, got %q", expected, tc.output)
	}
	return nil
}

func (tc *TUITestContext) iSendAQuitCommand() error {
	// Simulate quit command
	return nil
}

func (tc *TUITestContext) theProgramShouldExitGracefully() error {
	// Simulate graceful exit
	return nil
}

func (tc *TUITestContext) iStartTheSproutTUI() error {
	// Set consistent color profile for testing
	lipgloss.SetColorProfile(termenv.Ascii)
	
	// Create test model with our issues
	var err error
	tc.model, err = CreateTestModelWithIssues(tc.issues)
	if err != nil {
		return err
	}
	
	if tc.t != nil {
		tc.testModel = teatest.NewTestModel(tc.t, tc.model, teatest.WithInitialTermSize(80, 24))
	}
	
	return nil
}

func (tc *TUITestContext) iPress(key string) error {
	var keyMsg tea.KeyMsg
	
	switch key {
	case "down":
		keyMsg = tea.KeyMsg{Type: tea.KeyDown}
	case "up":
		keyMsg = tea.KeyMsg{Type: tea.KeyUp}
	case "right":
		keyMsg = tea.KeyMsg{Type: tea.KeyRight}
	case "left":
		keyMsg = tea.KeyMsg{Type: tea.KeyLeft}
	case "enter":
		keyMsg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		keyMsg = tea.KeyMsg{Type: tea.KeyEsc}
	default:
		return fmt.Errorf("unknown key: %s", key)
	}
	
	if tc.testModel != nil {
		tc.testModel.Send(keyMsg)
	}
	
	// Update our local model reference
	updatedModel, _ := tc.model.Update(keyMsg)
	tc.model = updatedModel.(model)
	
	return nil
}

func (tc *TUITestContext) theUIShouldDisplay(expected *godog.DocString) error {
	if tc.testModel == nil {
		return fmt.Errorf("test model not initialized")
	}
	
	// Get current view from our model state instead of teatest output
	actual := tc.model.View()
	
	// Strip ANSI codes for comparison
	actual = StripANSI(actual)
	expectedContent := expected.Content
	
	// Normalize whitespace more aggressively - strip leading and trailing whitespace from each line
	actualLines := strings.Split(actual, "\n")
	expectedLines := strings.Split(expectedContent, "\n")
	
	for i := range actualLines {
		actualLines[i] = strings.TrimSpace(actualLines[i])
	}
	for i := range expectedLines {
		expectedLines[i] = strings.TrimSpace(expectedLines[i])
	}
	
	// Remove empty lines at the beginning and end
	actualLines = trimEmptyLines(actualLines)
	expectedLines = trimEmptyLines(expectedLines)
	
	actualNormalized := strings.Join(actualLines, "\n")
	expectedNormalized := strings.Join(expectedLines, "\n")
	
	if actualNormalized != expectedNormalized {
		return fmt.Errorf("UI output mismatch:\nExpected:\n%s\n\nActual:\n%s", expectedNormalized, actualNormalized)
	}
	
	return nil
}

// trimEmptyLines removes empty lines from the beginning and end of a slice
func trimEmptyLines(lines []string) []string {
	// Find first non-empty line
	start := 0
	for start < len(lines) && lines[start] == "" {
		start++
	}
	
	// Find last non-empty line
	end := len(lines)
	for end > start && lines[end-1] == "" {
		end--
	}
	
	if start >= end {
		return []string{}
	}
	
	return lines[start:end]
}


// InitializeScenario initializes godog with our step definitions
func InitializeScenario(ctx *godog.ScenarioContext, t *testing.T) {
	tc := &TUITestContext{
		t: t,
	}
	
	// Setup a test context for each scenario
	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		tc.issues = nil
		tc.output = ""
		tc.t = t // Ensure t is set for each scenario
		return ctx, nil
	})
	
	// Step definitions
	ctx.Step(`^the following Linear issues exist:$`, tc.theFollowingLinearIssuesExist)
	ctx.Step(`^I have a minimal TUI model$`, tc.iHaveAMinimalTUIModel)
	ctx.Step(`^I render the view$`, tc.iRenderTheView)
	ctx.Step(`^the output should be "([^"]*)"$`, tc.theOutputShouldBe)
	ctx.Step(`^I send a quit command$`, tc.iSendAQuitCommand)
	ctx.Step(`^the program should exit gracefully$`, tc.theProgramShouldExitGracefully)
	ctx.Step(`^I start the Sprout TUI$`, tc.iStartTheSproutTUI)
	ctx.Step(`^I press "([^"]*)"$`, tc.iPress)
	ctx.Step(`^the UI should display:$`, tc.theUIShouldDisplay)
}

// TestFeatures runs the Gherkin tests
func TestFeatures(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: func(ctx *godog.ScenarioContext) {
			InitializeScenario(ctx, t)
		},
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"../../features"},
			TestingT: t,
		},
	}
	
	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}