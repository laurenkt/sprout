Feature: Sprout TUI Navigation
  As a developer using Sprout
  I want to navigate through Linear issues
  So that I can select and create worktrees for specific tasks

  Background:
    Given the following Linear issues exist:
      | identifier | title                                           | parent_id |
      | SPR-123    | Add user authentication                         |           |
      | SPR-124    | Implement dashboard with analytics and reporting |           |
      | SPR-125    | Create analytics component                      | SPR-124   |
      | SPR-126    | Add reporting metrics                           | SPR-124   |
      | SPR-127    | Fix critical bug in payment processing          |           |

  Scenario: Initial TUI display
    When I start the Sprout TUI
    Then the UI should display:
      """
      🌱 sprout

      > show-repo-name-at-front-of-prompt/enter branch name or select suggestion below
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

      > show-repo-name-at-front-of-prompt/enter branch name or select suggestion below
      ├──SPR-123  Add user authentication
      ├──SPR-124  Implement dashboard with analytics and reporting
      └──SPR-127  Fix critical bug in payment processing
      """