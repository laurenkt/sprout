Feature: Window Width Responsive Layout
  As a developer using Sprout
  I want the TUI to use the full width of my terminal
  So that long ticket names and branch names are not unnecessarily truncated

  Background:
    Given the following Linear issues exist:
      | identifier | title                                                              | parent_id |
      | SPR-123    | Add user authentication                                            |           |
      | SPR-124    | Implement comprehensive dashboard with advanced analytics and detailed reporting capabilities |           |
      | SPR-125    | Create analytics component with real-time data visualization      | SPR-124   |

  Scenario: Wide terminal shows full titles
    Given my terminal width is 120 characters
    When I start the Sprout TUI
    Then the UI should display:
      """
      🌱 sprout

      > sprout/enter branch name or select suggestion below
      ├──SPR-123  Add user authentication
      └──SPR-124  Implement comprehensive dashboard with advanced analytics and detailed reporting capabilities
      """

  Scenario: Narrow terminal truncates appropriately
    Given my terminal width is 60 characters
    When I start the Sprout TUI
    Then the UI should display titles truncated to fit the available width