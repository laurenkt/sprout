# Catwalk Testing Framework for Sprout TUI

This directory contains a comprehensive testing framework using [catwalk](https://github.com/knz/catwalk) for systematic testing of the Sprout Bubbletea TUI.

## Overview

Catwalk enables datadriven testing of Terminal User Interfaces built with [Bubbletea](https://github.com/charmbracelet/bubbletea). It allows us to:

- Test keyboard interactions systematically
- Verify UI state changes and view rendering
- Catch regressions in complex navigation flows
- Document expected behavior through test scenarios

## Test Structure

### Core Tests (`tui_test.go`)

1. **TestBasicFunctionality** - Unit tests for model initialization and basic state
2. **TestNavigation** - Unit tests for arrow key navigation logic  
3. **TestCatwalkSimple** - Catwalk integration test (requires git repository)

### Test Scenarios (`testdata/`)

The `testdata/` directory contains catwalk test scenarios in datadriven format:

#### Navigation Tests
- `basic_navigation.txt` - Arrow key navigation between input and Linear tickets
- `tree_expansion.txt` - Expanding/collapsing Linear ticket trees with right arrow

#### Input Handling Tests  
- `input_mode.txt` - Custom branch name entry and text input behavior
- `subtask_creation.txt` - Inline subtask creation workflow

#### Edge Cases & Error Handling
- `error_handling.txt` - Network errors, invalid inputs, API failures
- `empty_states.txt` - No Linear tickets, empty responses
- `long_content.txt` - Text truncation and long content handling
- `keyboard_shortcuts.txt` - Escape, Ctrl+C, Enter key behaviors

## Running Tests

### Unit Tests (Always Available)
```bash
go test -v                    # Run all tests
go test -v -run TestBasic     # Run basic functionality tests
go test -v -run TestNav       # Run navigation tests
```

### Catwalk Tests (Requires Git Repository)
```bash
# Enable catwalk tests by uncommenting code in TestCatwalkSimple
go test -v -run TestCatwalk

# Generate/update expected outputs with -rewrite flag
go test -v -run TestCatwalk -rewrite
```

## Test Data Format

Catwalk uses a simple datadriven format:

```
# Test description
run
key down    # Send down arrow key
----
Expected UI output here...

run  
type hello  # Type "hello"
key enter   # Press Enter
----
Expected UI output after typing...
```

### Available Commands
- `key up|down|left|right|enter|escape|ctrl+c` - Send key events
- `type <text>` - Type text into focused input
- `paste <text>` - Paste text
- `resize <width> <height>` - Resize terminal

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

For full integration testing, the catwalk tests can use the real `NewTUI()` function when:

1. Running in a valid git repository
2. Linear API token is configured  
3. Network connectivity is available

This enables testing the complete user workflow including:
- Real git worktree creation
- Live Linear API integration
- Actual command execution

## Writing New Tests

1. **Create test scenario** in `testdata/new_test.txt`
2. **Write test commands** using catwalk syntax
3. **Run with `-rewrite`** to generate expected output
4. **Review and commit** the test file

Example:
```bash
# Create new test
echo "run
key down
----" > testdata/my_new_test.txt

# Generate expected output
go test -v -run TestCatwalk -rewrite

# Review generated output in testdata/my_new_test.txt
```

## Benefits

This testing framework provides:

- **Systematic verification** of complex TUI behaviors
- **Regression prevention** through automated UI testing
- **Documentation** of expected user interactions
- **Confidence** when refactoring UI logic
- **Reproducible testing** across different environments

The combination of unit tests for logic and catwalk tests for UI behavior ensures comprehensive coverage of the Sprout TUI functionality.