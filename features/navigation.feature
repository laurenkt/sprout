Feature: Sprout TUI Navigation
  As a developer using Sprout
  I want to navigate through Linear issues
  So that I can select and create worktrees for specific tasks

  Background:
    Given the following Linear issues exist:
      | identifier | title                                           | parent_id | status      |
      | SPR-2      | Add user authentication                         |           | Todo        |
      | SPR-124    | Implement dashboard with analytics and reporting |           | In Progress |
      | SPR-125    | Create analytics component                      | SPR-124   | Todo        |
      | SPR-126    | Add reporting metrics                           | SPR-124   | Done        |
      | SPR-1234   | Fix critical bug in payment processing          |           | In Review   |

  Scenario: Initial TUI display
    When I start the Sprout TUI
    Then the UI should display:
      """
      🌱 sprout
      Mode: create worktree (Tab to toggle)

      > sprout/█enter branch name or select suggestion below
      ├──SPR-2     Todo         Add user authentication
      ├──SPR-124   In Progress  Implement dashboard with analytics and r...
      └──SPR-1234  In Review    Fix critical bug in payment processing
      """

  Scenario: Navigate down from input field
    Given I start the Sprout TUI
    When I press "down"
    Then the UI should display:
      """
      🌱 sprout
      Mode: create worktree (Tab to toggle)

      > sprout/spr-2-add-user-authentication
      ├──SPR-2     Todo         Add user authentication
      ├──SPR-124   In Progress  Implement dashboard with analytics and r...
      └──SPR-1234  In Review    Fix critical bug in payment processing
      """

  Scenario: Navigate back up to input field
    Given I start the Sprout TUI
    And I press "down"
    When I press "up"
    Then the UI should display:
      """
      🌱 sprout
      Mode: create worktree (Tab to toggle)

      > sprout/█enter branch name or select suggestion below
      ├──SPR-2     Todo         Add user authentication
      ├──SPR-124   In Progress  Implement dashboard with analytics and r...
      └──SPR-1234  In Review    Fix critical bug in payment processing
      """