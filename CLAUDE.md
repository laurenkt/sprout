# Claude Code Guide for Sprout

## Project Overview

**Sprout** is a Git worktree terminal UI tool with Linear integration, written in Go. It provides both interactive and command-line interfaces for managing git worktrees with seamless Linear workflow support for ticket-based development.

### Key Features
- Git worktree management from any location
- Interactive terminal UI and one-shot command-line modes  
- Linear integration for ticket-based development workflows
- Smart input handling and context-aware operations
- Automatic branch naming from Linear tickets

## Project Structure

```
/Users/laurenkt/Projects/sprout/
├── README.md                 # Project documentation and usage guide
├── go.mod                   # Go module definition (Go 1.24.2)
├── .gitignore              # Git ignore patterns
├── cmd/sprout/main.go      # Main entry point with basic CLI structure
└── pkg/                    # Package directory structure (currently empty)
    ├── config/             # Configuration management (planned)
    ├── git/               # Git operations (planned) 
    ├── linear/            # Linear API integration (planned)
    └── ui/                # Terminal UI components (planned)
```

## Current Development State

**EARLY DEVELOPMENT PHASE** - The project is in initial setup with:

- ✅ Basic project structure established
- ✅ Go module initialized (Go 1.24.2)
- ✅ Main entry point with command parsing skeleton
- ✅ README with comprehensive feature documentation
- ⏳ Package implementations are scaffolded but not implemented
- ⏳ Interactive TUI not yet implemented
- ⏳ Linear integration not yet implemented
- ⏳ Git worktree operations not yet implemented

## Architecture Design

### Command Structure
The application supports two primary modes:
1. **Interactive Mode**: `sprout` (launches terminal UI)
2. **One-shot Mode**: `sprout create [options]` (command-line operations)

### Package Architecture
- `cmd/sprout/`: Application entry point and CLI parsing
- `pkg/config/`: Configuration management and Linear API tokens
- `pkg/git/`: Git worktree operations and repository management
- `pkg/linear/`: Linear API integration and ticket management
- `pkg/ui/`: Terminal user interface components

## Build and Development

### Prerequisites
- Go 1.21+ (project uses Go 1.24.2)
- Git 2.5+ (for worktree support)
- Linear API access (for integration features)

### Development Commands

Prefer to run with `go run` rather than building artefacts.

```bash
# Run directly
go run ./cmd/sprout

# Run with arguments
go run ./cmd/sprout create --help

# Run tests (when implemented)
go test ./...

# Get dependencies
go mod tidy
```

### Git Integration
- Repository is tracked in Git (master branch, up to date with origin)
- Standard .gitignore includes Go build artifacts and test files
- Working tree is clean

## Implementation Priorities

Based on the README and current structure, the development order should likely be:

1. **Core Git Operations** (`pkg/git/`):
   - Repository detection and validation
   - Worktree creation, listing, and management
   - Branch operations

2. **Configuration Management** (`pkg/config/`):
   - Config file handling
   - Linear API token management
   - User preferences

3. **Linear Integration** (`pkg/linear/`):
   - API client and authentication
   - Ticket fetching and searching
   - Branch name generation from tickets
   - Subtask creation

4. **Terminal UI** (`pkg/ui/`):
   - Interactive worktree browser
   - Linear ticket selection interface
   - Input forms and validation

5. **Enhanced CLI** (extend `cmd/sprout/main.go`):
   - Complete command-line argument parsing
   - Integration with implemented packages

## Development Conventions

### Go Practices
- Follow standard Go project layout
- Use `pkg/` for library code that can be imported
- Keep `cmd/` focused on application entry points
- Implement proper error handling throughout

### Git Workflow
- Working on master branch (consider feature branches for major changes)
- Clean working tree maintained
- Standard ignore patterns for Go artifacts

### Testing Strategy
- Add `*_test.go` files alongside implementation
- Focus on unit tests for core git and linear operations
- BDD tests using Cucumber/Gherkin for TUI behavior (see BDD Testing section below)

## Key Integration Points

### Git Worktree Operations
- Must handle worktree creation from any current location
- Support for flexible branch naming conventions
- Integration with existing repository structure

### Linear API Integration
- Requires API token configuration
- Should support both assigned and searchable tickets
- Branch name suggestions from Linear metadata
- Subtask creation capabilities

### User Experience
- Smart input parsing and completion
- Context-aware operation based on current git state
- Minimal friction workflows for common operations

## Configuration Requirements

The application will need configuration for:
- Linear API tokens and team settings
- Default branch naming patterns
- Worktree directory preferences
- User interface customizations

## BDD Testing

The project uses Behavior-Driven Development (BDD) testing with Cucumber/Gherkin to test the terminal UI behavior. This provides human-readable test scenarios that describe how the TUI should behave.

### Test Structure

**Feature Files**: `/features/*.feature`
- Written in Gherkin syntax (Given/When/Then)
- Define user scenarios and expected UI behavior
- Test files:
  - `navigation.feature` - Basic TUI navigation and input
  - `expansion.feature` - Issue tree expansion/collapse behavior  
  - `interaction.feature` - User interaction patterns

**Step Definitions**: `/pkg/ui/features_test.go`
- Go code that implements the Gherkin steps
- Uses `github.com/cucumber/godog` for BDD testing
- Integrates with `github.com/charmbracelet/x/exp/teatest` for TUI testing

### Test Data Format

Linear issues are defined using a simple table format:

```gherkin
Given the following Linear issues exist:
  | identifier | title                              | parent_id |
  | SPR-123    | Add user authentication            |           |
  | SPR-124    | Implement dashboard                |           |
  | SPR-125    | Create analytics component         | SPR-124   |
  | SPR-126    | Add reporting metrics              | SPR-124   |
```

Key points:
- `identifier` serves as both ID and display identifier (e.g., SPR-123)
- `parent_id` references the parent issue's identifier for sub-issues
- Empty `parent_id` indicates a top-level issue
- Issues with children automatically get `HasChildren: true` set

### Running BDD Tests

```bash
# Run all BDD tests
go test ./pkg/ui/

# Run with verbose output to see scenario details
go test -v ./pkg/ui/

# Run tests with Gherkin output formatting
go test ./pkg/ui/ 2>&1 | less -R
```

### Making Changes to BDD Tests

1. **Adding new scenarios**: Edit the `.feature` files in `/features/` directory
2. **Adding new step definitions**: Add step functions to `features_test.go`
3. **Modifying test data**: Update the issue tables in the feature files
4. **Testing UI changes**: Run the BDD tests to verify TUI behavior matches expected output

The BDD tests are particularly useful for:
- Verifying keyboard navigation works correctly
- Ensuring UI output matches expected format
- Testing issue tree expansion/collapse behavior
- Validating user interaction workflows

### Test Context Management

The `TUITestContext` struct maintains test state:
- `issues` - Linear issues loaded from test data
- `model` - The TUI model being tested
- `testModel` - Teatest wrapper for sending key events
- Test scenarios are isolated - each gets a fresh context

# When raising PRs

- Don't include a "test plan"
