Feature: Sprout TUI Navigation
  As a developer using Sprout
  I want to navigate through Linear issues
  So that I can select and create worktrees for specific tasks

  Background:
    Given the following Linear issues exist:
      | id      | identifier | title                                           | has_children | children        |
      | issue-1 | SPR-123    | Add user authentication                         | false        |                 |
      | issue-2 | SPR-124    | Implement dashboard with analytics and reporting | true         | issue-2-1,issue-2-2 |
      | issue-3 | SPR-127    | Fix critical bug in payment processing          | false        |                 |
    And the child issues are:
      | id        | identifier | title                     | parent_id |
      | issue-2-1 | SPR-125    | Create analytics component | issue-2   |
      | issue-2-2 | SPR-126    | Add reporting metrics      | issue-2   |

  Scenario: Initial TUI display
    When I start the Sprout TUI
    Then the UI should display:
      """
      🌱 sprout

      > enter branch name or select suggestion below
      ├──SPR-123  Add user authentication
      ├──SPR-124  Implement dashboard with analytics and reporting
      └──SPR-127  Fix critical bug in payment processing
      """

  Scenario: Navigate down from input field
    Given I start the Sprout TUI
    When I press "down"
    Then the UI should display:
      """
      🌱 sprout

      > spr-123-add-user-authentication
      ├──SPR-123  Add user authentication
      ├──SPR-124  Implement dashboard with analytics and reporting
      └──SPR-127  Fix critical bug in payment processing
      """

  Scenario: Navigate back up to input field
    Given I start the Sprout TUI
    And I press "down"
    When I press "up"
    Then the UI should display:
      """
      🌱 sprout

      > enter branch name or select suggestion below
      ├──SPR-123  Add user authentication
      ├──SPR-124  Implement dashboard with analytics and reporting
      └──SPR-127  Fix critical bug in payment processing
      """