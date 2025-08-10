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
3. **TestTeatestMinimalGolden** - Minimal teatest with golden file comparison
4. **TestTeatestGoldenNavigation** - Full model test with golden file output
5. **TestTeatestGoldenInteraction** - User interaction test with golden files

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

# Generate/update golden files
go test -v -run TestTeatestGolden -update
```

## Teatest API Usage

The teatest framework provides several key functions for golden file testing:

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

// Compare output with golden files
out, err := io.ReadAll(tm.FinalOutput(t))
if err != nil {
    t.Fatal(err)
}
teatest.RequireEqualOutput(t, out)
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
2. **Set consistent color profile** with `lipgloss.SetColorProfile(termenv.Ascii)`
3. **Initialize model** with `teatest.NewTestModel()`
4. **Send interactions** using `tm.Send()`
5. **Compare with golden files** using `teatest.RequireEqualOutput()`

Example:
```go
func TestNewFeatureGolden(t *testing.T) {
    // Set consistent color profile for testing
    lipgloss.SetColorProfile(termenv.Ascii)
    
    model := createTestModel()
    tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(80, 24))
    
    // Send some interactions
    tm.Send(tea.KeyMsg{Type: tea.KeyDown})
    tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
    
    // Quit the program
    tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
    
    // Compare output with golden file
    out, err := io.ReadAll(tm.FinalOutput(t))
    if err != nil {
        t.Fatal(err)
    }
    teatest.RequireEqualOutput(t, out)
}
```

### Golden File Workflow

1. **Write test function** following the pattern above
2. **Generate golden file** by running test with `-update` flag:
   ```bash
   go test -v -run TestNewFeatureGolden -update
   ```
3. **Review generated golden file** in `testdata/TestNewFeatureGolden.golden`
4. **Run test normally** to verify it passes:
   ```bash
   go test -v -run TestNewFeatureGolden
   ```

## Golden Files

Golden files in `testdata/` capture expected terminal output:

- **Consistent output**: Set `lipgloss.SetColorProfile(termenv.Ascii)` for reproducible results
- **Line endings**: `.gitattributes` file ensures consistent line endings across platforms
- **Regeneration**: Use `-update` flag to regenerate expected output when UI changes
- **Visual testing**: Golden files capture the exact terminal output users see

## Benefits

This testing framework provides:

- **Visual regression testing** through golden file comparison
- **Programmatic testing** of TUI interactions and state
- **Regression prevention** through automated UI testing
- **Output validation** for exact visual correctness
- **Confidence** when refactoring UI logic
- **Reproducible testing** across different environments
- **Documentation** of expected UI behavior through saved outputs

The combination of unit tests for logic and teatest with golden files for UI behavior ensures comprehensive coverage of the Sprout TUI functionality.