Feature: Window Width Responsive Layout
  As a developer using Sprout
  I want the TUI to use the full width of my terminal
  So that long ticket names and branch names are not unnecessarily truncated

  Background:
    Given the following Linear issues exist:
      | identifier | title                                                              | parent_id | status      |
      | SPR-123    | Add user authentication                                            |           | Todo        |
      | SPR-124    | Implement comprehensive dashboard with advanced analytics and detailed reporting capabilities |           | In Progress |
      | SPR-125    | Create analytics component with real-time data visualization      | SPR-124   | Todo        |

  Scenario: Wide terminal shows full titles
    Given my terminal width is 120 characters
    When I start the Sprout TUI
    Then the UI should display:
      """
      🌱 sprout [worktree]

      > sprout/enter branch name or select suggestion below
      ├──SPR-123  Todo         Add user authentication
      └──SPR-124  In Progress  Implement comprehensive dashboard with advanced analytics and detailed reporting ...
      [worktree <tab>]
      """

  Scenario: Narrow terminal truncates appropriately
    Given my terminal width is 60 characters
    When I start the Sprout TUI
    Then the UI should display titles truncated to fit the available width