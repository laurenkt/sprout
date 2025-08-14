Feature: Sprout TUI Fuzzy Search
  As a developer using Sprout
  I want to fuzzy search through Linear issues
  So that I can quickly find and select specific tasks from a large list

  Background:
    Given the following Linear issues exist:
      | identifier | title                                           | parent_id | status      |
      | SPR-123    | Add user authentication                         |           | Todo        |
      | SPR-124    | Implement dashboard with analytics and reporting |           | In Progress |
      | SPR-125    | Create analytics component                      | SPR-124   | Todo        |
      | SPR-126    | Add reporting metrics                           | SPR-124   | Done        |
      | SPR-127    | Fix critical bug in payment processing          |           | In Review   |
      | SPR-128    | Update user profile settings                    |           | Backlog     |
      | SPR-129    | Implement notification system                   |           | Todo        |

  Scenario: Enter search mode with forward slash
    Given I start the Sprout TUI
    When I press "/"
    Then the UI should display:
      """
      🌱 sprout

      /type to fuzzy search
      ├──SPR-123  Todo         Add user authentication
      ├──SPR-124  In Progress  Implement dashboard with analytics and re...
      ├──SPR-127  In Review    Fix critical bug in payment processing
      ├──SPR-128  Backlog      Update user profile settings
      └──SPR-129  Todo         Implement notification system
      """

  Scenario: Filter issues by typing partial text
    Given I start the Sprout TUI
    And I press "/"
    When I type "auth"
    Then the UI should display:
      """
      🌱 sprout

      /auth
      └──SPR-123  Todo  Add user authentication
      """

  Scenario: Filter issues by identifier
    Given I start the Sprout TUI
    And I press "/"
    When I type "127"
    Then the UI should display:
      """
      🌱 sprout

      /127
      └──SPR-127  In Review  Fix critical bug in payment processing
      """

  Scenario: Filter shows multiple matches
    Given I start the Sprout TUI
    And I press "/"
    When I type "user"
    Then the UI should display:
      """
      🌱 sprout

      /user
      ├──SPR-123  Todo     Add user authentication
      └──SPR-128  Backlog  Update user profile settings
      """

  Scenario: No matches found
    Given I start the Sprout TUI
    And I press "/"
    When I type "xyz"
    Then the UI should display:
      """
      🌱 sprout

      /xyz
      """

  Scenario: Clear search and return to normal mode
    Given I start the Sprout TUI
    And I press "/"
    And I type "auth"
    When I press "escape"
    Then the UI should display:
      """
      🌱 sprout

      > sprout/enter branch name or select suggestion below
      ├──SPR-123  Todo         Add user authentication
      ├──SPR-124  In Progress  Implement dashboard with analytics and re...
      ├──SPR-127  In Review    Fix critical bug in payment processing
      ├──SPR-128  Backlog      Update user profile settings
      └──SPR-129  Todo         Implement notification system
      """

  Scenario: Navigate search results with arrow keys
    Given I start the Sprout TUI
    And I press "/"
    And I type "user"
    When I press "down"
    Then the UI should display:
      """
      🌱 sprout

      /user sprout/spr-123-add-user-authentication
      ├──SPR-123  Todo     Add user authentication
      └──SPR-128  Backlog  Update user profile settings
      """
    When I press "down"
    Then the UI should display:
      """
      🌱 sprout

      /user sprout/spr-128-update-user-profile-settings
      ├──SPR-123  Todo     Add user authentication
      └──SPR-128  Backlog  Update user profile settings
      """
    When I press "up"
    Then the UI should display:
      """
      🌱 sprout

      /user sprout/spr-123-add-user-authentication
      ├──SPR-123  Todo     Add user authentication
      └──SPR-128  Backlog  Update user profile settings
      """

  Scenario: Backspace works in search mode
    Given I start the Sprout TUI
    And I press "/"
    And I type "auth"
    When I press "backspace"
    Then the UI should display:
      """
      🌱 sprout

      /aut
      ├──SPR-123  Todo       Add user authentication
      ├──SPR-127  In Review  Fix critical bug in payment processing
      └──SPR-128  Backlog    Update user profile settings
      """
    When I press "backspace"
    Then the UI should display:
      """
      🌱 sprout

      /au
      ├──SPR-123  Todo       Add user authentication
      ├──SPR-127  In Review  Fix critical bug in payment processing
      └──SPR-128  Backlog    Update user profile settings
      """
    When I press "backspace"
    Then the UI should display:
      """
      🌱 sprout

      /a
      ├──SPR-123  Todo         Add user authentication
      ├──SPR-124  In Progress  Implement dashboard with analytics and re...
      ├──SPR-127  In Review    Fix critical bug in payment processing
      ├──SPR-128  Backlog      Update user profile settings
      └──SPR-129  Todo         Implement notification system
      """
    When I press "backspace"
    Then the UI should display:
      """
      🌱 sprout

      /type to fuzzy search
      ├──SPR-123  Todo         Add user authentication
      ├──SPR-124  In Progress  Implement dashboard with analytics and re...
      ├──SPR-127  In Review    Fix critical bug in payment processing
      ├──SPR-128  Backlog      Update user profile settings
      └──SPR-129  Todo         Implement notification system
      """