Feature: Sprout TUI Interaction
  As a developer using Sprout
  I want to interact with the TUI
  So that I can navigate and select issues efficiently

  Background:
    Given the following Linear issues exist:
      | identifier | title                                           | parent_id | status      |
      | SPR-123    | Add user authentication                         |           | Todo        |
      | SPR-124    | Implement dashboard with analytics and reporting |           | In Progress |
      | SPR-125    | Create analytics component                      | SPR-124   | Todo        |
      | SPR-126    | Add reporting metrics                           | SPR-124   | In Review   |
      | SPR-127    | Fix critical bug in payment processing          |           | Done        |

  Scenario: Navigate down and back up
    Given I start the Sprout TUI
    When I press "down"
    Then the UI should display:
      """
      🌱 sprout

      > sprout/spr-123-add-user-authentication█
      ├──SPR-123  Todo         Add user authentication
      ├──SPR-124  In Progress  Implement dashboard with analytics and re...
      └──SPR-127  Done         Fix critical bug in payment processing
      """
    When I press "up"
    Then the UI should display:
      """
      🌱 sprout

      > sprout/█enter branch name or select suggestion below
      ├──SPR-123  Todo         Add user authentication
      ├──SPR-124  In Progress  Implement dashboard with analytics and re...
      └──SPR-127  Done         Fix critical bug in payment processing
      """