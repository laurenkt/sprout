Feature: Resume command execution
  As a developer using Sprout
  I want resumed worktrees to use a separate command
  So that continuing work can run a different tool invocation from new work

  Scenario: Resume uses resumeCommand when configured
    Given a config with:
      | key            | value           |
      | defaultCommand | claude $PROMPT  |
      | resumeCommand  | claude --resume |
    And the following worktrees exist:
      | branch         | path                           | updated_at           | merged |
      | feature-search | /mock/worktrees/feature-search | 2026-05-01T16:00:00Z | false  |
    When I start the Sprout TUI
    And I press "down"
    And I press "enter"
    Then the post-resume command should be "cd /mock/worktrees/feature-search && claude --resume"

  Scenario: Resume falls back to defaultCommand without prompt placeholder
    Given a config with:
      | key            | value  |
      | defaultCommand | code . |
    And the following worktrees exist:
      | branch         | path                           | updated_at           | merged |
      | feature-search | /mock/worktrees/feature-search | 2026-05-01T16:00:00Z | false  |
    When I start the Sprout TUI
    And I press "down"
    And I press "enter"
    Then the post-resume command should be "cd /mock/worktrees/feature-search && code ."

  Scenario: Resume does not fall back to prompt-based defaultCommand
    Given a config with:
      | key            | value          |
      | defaultCommand | claude $PROMPT |
    And the following worktrees exist:
      | branch         | path                           | updated_at           | merged |
      | feature-search | /mock/worktrees/feature-search | 2026-05-01T16:00:00Z | false  |
    When I start the Sprout TUI
    And I press "down"
    And I press "enter"
    Then no post-resume command should run
    And the TUI should resume worktree "/mock/worktrees/feature-search"
