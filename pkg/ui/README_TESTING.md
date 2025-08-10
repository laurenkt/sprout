# Teatest Testing Framework for Sprout TUI

This directory contains a comprehensive testing framework using [teatest](https://github.com/charmbracelet/x/tree/main/exp/teatest) for systematic testing of the Sprout Bubbletea TUI.

## Overview

Teatest enables testing of Terminal User Interfaces built with [Bubbletea](https://github.com/charmbracelet/bubbletea). It allows us to:

- Test keyboard interactions and model updates
- Verify UI state changes and view rendering
- Catch regressions in complex navigation flows
- Test program output and final model state

## Test Structure

### Core Tests (`tui_test.go`)

1. **TestBasicFunctionality** - Unit tests for model initialization and basic state
2. **TestNavigation** - Unit tests for arrow key navigation logic  
3. **TestTeatestMinimal** - Minimal teatest integration test
4. **TestTeatestSimple** - Full teatest integration test with mock dependencies

## Running Tests

### Unit Tests (Always Available)
```bash
go test -v                    # Run all tests
go test -v -run TestBasic     # Run basic functionality tests
go test -v -run TestNav       # Run navigation tests
```

### Teatest Tests
```bash
# Run teatest integration tests
go test -v -run TestTeatest

# Run all tests including teatest
go test -v
```

## Teatest API Usage

The teatest framework provides several key functions:

### Creating Test Models
```go
tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(80, 24))
```

### Sending Messages
```go
// Send keyboard input
tm.Send(tea.KeyMsg{Type: tea.KeyDown})
tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})

// Send custom messages
tm.Send(customMessage{})
```

### Testing Output and State
```go
// Get final program output
output := tm.FinalOutput(t)

// Get final model state  
finalModel := tm.FinalModel(t)
```

## Mock Testing

The test framework includes a simplified model creation function that:

- Provides predictable Linear ticket data for testing
- Avoids external dependencies (git, Linear API)
- Enables systematic testing of UI logic

```go
func CreateTestModel() (model, error) {
    // Creates model with mock Linear tickets
    // Suitable for unit tests and navigation testing
}
```

## Integration with Real Dependencies

For full integration testing, the teatest tests can use the real `NewTUI()` function when:

1. Running in a valid git repository
2. Linear API token is configured  
3. Network connectivity is available

This enables testing the complete user workflow including:
- Real git worktree creation
- Live Linear API integration
- Actual command execution

## Writing New Tests

1. **Create test function** using teatest patterns
2. **Initialize model** with `teatest.NewTestModel()`
3. **Send interactions** using `tm.Send()`
4. **Assert results** using `tm.FinalOutput()` or `tm.FinalModel()`

Example:
```go
func TestNewFeature(t *testing.T) {
    model := createTestModel()
    tm := teatest.NewTestModel(t, model)
    
    // Send some interactions
    tm.Send(tea.KeyMsg{Type: tea.KeyDown})
    tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
    
    // Quit the program
    tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
    
    // Test final state
    finalModel := tm.FinalModel(t)
    // Assert expectations on finalModel
}
```

## Benefits

This testing framework provides:

- **Programmatic testing** of TUI interactions and state
- **Regression prevention** through automated UI testing
- **Model state verification** at any point in the program lifecycle
- **Output validation** for visual correctness
- **Confidence** when refactoring UI logic
- **Reproducible testing** across different environments

The combination of unit tests for logic and teatest for UI behavior ensures comprehensive coverage of the Sprout TUI functionality.