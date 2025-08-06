# Sprout - Git Worktree Terminal UI

A user-friendly terminal UI for managing git worktrees with integrated Linear workflow support.

## Overview

Sprout simplifies git worktree management by providing both interactive and command-line interfaces, with seamless Linear integration for ticket-based development workflows.

## Features

### Git Worktree Management
- **List existing worktrees**: View all worktrees with branch names, PR status, and commit information
- **Identify merge-ready worktrees**: List worktrees with merged PRs that are ready for pruning
- **Create worktrees from any location**: Generate new worktrees from the current repository, regardless of which worktree you're currently in
- **Flexible branch naming**: Optionally specify branch names or let Linear integration handle it automatically
- **Intelligent input parsing**: Enter as much or as little information as you want - Sprout figures out the rest

### Operating Modes
- **Interactive Mode**: Full terminal UI for browsing and managing worktrees and Linear tickets
- **One-shot Mode**: Command-line interface for quick worktree operations

### Linear Integration
- **Ticket-based worktrees**: Select Linear tickets to automatically create worktrees with suggested branch names
- **Task management**: Create new subtasks on Linear issues directly from the tool
- **Flexible ticket access**: 
  - View tasks assigned to you
  - Search and browse tasks beyond your assignments
- **Seamless workflow**: Skip manual branch naming by leveraging Linear's branch name suggestions

### User Experience
- **Smart input handling**: Provide partial information and let Sprout intelligently complete the workflow
- **Context-aware**: Understands your current git state and adapts accordingly
- **Minimal friction**: Streamlined workflows for common development tasks

## Getting Started

```bash
# Interactive mode
sprout

# List all worktrees with PR status
sprout list

# List worktrees with merged PRs (ready to prune)
sprout prune

# One-shot worktree creation
sprout create [branch-name]

# Create worktree and run command in it
sprout create [branch-name] [command] [args...]

# One-shot with Linear ticket
sprout create --linear [ticket-id]
```

### Command Examples

```bash
# Create worktree and change to it
cd "$(sprout create mybranch)"

# Create worktree and open in VS Code
sprout create mybranch code .

# Create worktree and start a shell
sprout create mybranch bash

# Create worktree and run git status
sprout create mybranch git status
```

**Note**: When running commands with `sprout create`, the worktree directory is printed to stderr after command execution for easy reference.

## Requirements

- Go 1.21+
- Git 2.5+ (for worktree support)
- GitHub CLI (`gh`) for PR status information
- Linear API access (for Linear integration features)

## Configuration

Sprout supports configuration via `~/.sprout.json5` for customizing behavior:

```json5
{
  // Command to run when sprout is called without arguments
  // If not specified, defaults to interactive mode
  "defaultCommand": "echo 'Welcome to Sprout!'",
  
  // Linear API key for ticket integration
  "linearApiKey": "your-linear-api-key-here"
}
```

### Configuration Options

- **`defaultCommand`**: Command to execute when `sprout` is called without arguments. If not specified, launches interactive mode.
- **`linearApiKey`**: Your Linear API key for accessing tickets and creating branches from Linear issues.

### Linear Integration

To use Linear integration features, you'll need to:

1. Get your Linear API key from [Linear Settings > API](https://linear.app/settings/api)
2. Add it to your `~/.sprout.json5` configuration file
3. Run `sprout doctor` to verify the configuration is working

The Linear API key enables:
- Browsing and selecting tickets for worktree creation
- Automatic branch name suggestions based on ticket titles
- Creating subtasks directly from the terminal UI