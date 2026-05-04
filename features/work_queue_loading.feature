Feature: Work queue loading status
  As a developer using Sprout in a large repository
  I want loading text to show the exact work being done
  So that I can tell which command is taking time

  Scenario: Worktree loading shows the exact git worktree command
    Given worktree loading is paused at "git worktree list --porcelain"
    When I start the Sprout TUI
    Then the UI should display "git worktree list --porcelain"
    And the UI should not display "Loading work queue..."
    And the UI should not show work queue rows

  Scenario: Branch metadata loading shows the exact for-each-ref command
    Given worktree loading is paused at "git for-each-ref refs/heads/feature-search refs/heads/spr-124-dashboard-analytics --format=%(refname:short)%00%(committerdate:iso-strict)"
    When I start the Sprout TUI
    Then the UI should display "git for-each-ref refs/heads/feature-search refs/heads/spr-124-dashboard-analytics --format=%(refname:short)%00%(committerdate:iso-strict)"
    And the UI should not show work queue rows

  Scenario: GitHub PR loading shows the exact gh command
    Given worktree loading is paused at "gh pr list --head feature-search --state all --json state --limit 1"
    When I start the Sprout TUI
    Then the UI should display "gh pr list --head feature-search --state all --json state --limit 1"
    And the UI should not show work queue rows

  Scenario: Linear loading uses human-readable text
    Given Linear issue loading is paused
    When I start the Sprout TUI
    Then the UI should display "Loading Linear issues..."
    And the UI should not show work queue rows

  Scenario: Concurrent loading shows each active action
    Given Linear issue loading is paused
    And worktree loading is paused at "git worktree list --porcelain"
    When I start the Sprout TUI
    Then the UI should display "Loading Linear issues..."
    And the UI should display "git worktree list --porcelain"
    And the UI should not show work queue rows

  Scenario: Queue appears only after all loading is complete
    Given 1 active work queue rows exist
    And Linear issue loading is paused
    And worktree loading has completed
    When I start the Sprout TUI
    Then the UI should display "Loading Linear issues..."
    And the UI should not show work queue rows
    When Linear issue loading completes
    Then the UI should show 1 work queue rows

  Scenario: GitHub lookup failure blocks the queue with an error
    Given GitHub PR status lookup fails for branch "feature-search"
    When I start the Sprout TUI
    Then the UI should display "Error:"
    And the UI should display "gh pr list --head feature-search --state all --json state --limit 1"
    And the UI should not show work queue rows
