Feature: Sprout CLI Commands
  As a developer using Sprout
  I want to use non-TUI commands like help, list, and doctor
  So that I can manage worktrees and configuration from the command line

  Scenario: Display help information
    When I run "sprout help"
    Then the output should contain:
      """
      Sprout - Git Worktree Terminal UI

      Usage:
        sprout                              Start in interactive mode
        sprout list                         List all worktrees
        sprout create <branch>              Create worktree and output path
        sprout create <branch> <command>    Create worktree and run command in it
        sprout prune [branch]               Remove worktree(s) - all merged if no branch specified
        sprout doctor                       Show configuration values
        sprout help                         Show this help
      """

  Scenario: Show help with --help flag
    When I run "sprout --help"
    Then the output should contain:
      """
      Sprout - Git Worktree Terminal UI
      """

  Scenario: List worktrees when none exist
    Given no worktrees exist
    When I run "sprout list"
    Then the output should be:
      """
      No worktrees found
      """

  Scenario: List existing worktrees
    Given the following worktrees exist:
      | branch      | commit   | pr_status |
      | feature-123 | abc12345 | Open      |
      | bugfix-456  | def67890 | Merged    |
    When I run "sprout list"
    Then the output should contain:
      """
      🌱 Active Worktrees
      """
    And the output should contain a table with:
      | BRANCH      | PR STATUS | COMMIT   |
      | feature-123 | Open      | abc12345 |
      | bugfix-456  | Merged    | def67890 |

  Scenario: Doctor command shows configuration
    Given a config with:
      | key             | value        |
      | default_command | code .       |
      | linear_api_key  | <not_set>    |
    When I run "sprout doctor"
    Then the output should contain:
      """
      🌱 Sprout Configuration

        Default Command: code .
        Linear API Key: not configured
      """
    And the output should contain:
      """
      Linear Integration

        API Key: not configured
        Status: disabled
      """

  Scenario: Doctor command with Linear API key configured
    Given a config with:
      | key             | value                      |
      | default_command | code .                     |
      | linear_api_key  | lin_api_test123456789abc   |
    When I run "sprout doctor"
    Then the output should contain:
      """
      🌱 Sprout Configuration

        Default Command: code .
        Linear API Key: configured
      """
    And the output should contain:
      """
      Linear Integration

        API Key: lin_api_...9abc
      """

  Scenario: Unknown command shows error and help
    When I run "sprout unknown"
    Then the command should fail
    And the output should contain:
      """
      Unknown command: unknown
      """
    And the output should contain:
      """
      Sprout - Git Worktree Terminal UI
      """