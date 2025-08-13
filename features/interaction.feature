Feature: Sprout TUI Interaction
  As a developer using Sprout
  I want to interact with the TUI
  So that I can navigate and select issues efficiently

  Background:
    Given the following Linear issues exist:
      | identifier | title                                           | parent_id |
      | SPR-123    | Add user authentication                         |           |
      | SPR-124    | Implement dashboard with analytics and reporting |           |
      | SPR-125    | Create analytics component                      | SPR-124   |
      | SPR-126    | Add reporting metrics                           | SPR-124   |
      | SPR-127    | Fix critical bug in payment processing          |           |

  Scenario: Navigate down and back up
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
    When I press "up"
    Then the UI should display:
      """
      🌱 sprout

      > show-repo-name-at-front-of-prompt/enter branch name or select suggestion below
      ├──SPR-123  Add user authentication
      ├──SPR-124  Implement dashboard with analytics and reporting
      └──SPR-127  Fix critical bug in payment processing
      """