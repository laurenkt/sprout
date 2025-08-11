Feature: Sprout TUI Tree Expansion
  As a developer using Sprout
  I want to expand and collapse issue trees
  So that I can view and navigate through sub-issues

  Scenario: Expand multiple issue trees
    Given the following Linear issues exist:
      | id      | identifier | title                                | has_children | children                    |
      | issue-1 | SPR-100    | Feature A: User management system    | true         | issue-1-1,issue-1-2         |
      | issue-2 | SPR-200    | Feature B: Dashboard and analytics   | true         | issue-2-1,issue-2-2,issue-2-3 |
      | issue-3 | SPR-300    | Bug fix: Payment processing errors   | false        |                             |
    And the child issues are:
      | id        | identifier | title                         | parent_id |
      | issue-1-1 | SPR-101    | Add user registration         | issue-1   |
      | issue-1-2 | SPR-102    | Implement user authentication | issue-1   |
      | issue-2-1 | SPR-201    | Create dashboard layout       | issue-2   |
      | issue-2-2 | SPR-202    | Add analytics widgets         | issue-2   |
      | issue-2-3 | SPR-203    | Implement data visualization  | issue-2   |
    When I start the Sprout TUI
    Then the UI should display:
      """
      🌱 sprout

      > enter branch name or select suggestion below
      ├──SPR-100  Feature A: User management system
      ├──SPR-200  Feature B: Dashboard and analytics
      └──SPR-300  Bug fix: Payment processing errors
      """
    When I press "down"
    Then the UI should display:
      """
      🌱 sprout

      > spr-100-feature-a-user-management-system
      ├──SPR-100  Feature A: User management system
      ├──SPR-200  Feature B: Dashboard and analytics
      └──SPR-300  Bug fix: Payment processing errors
      """
    When I press "right"
    Then the UI should display:
      """
      🌱 sprout

      > spr-100-feature-a-user-management-system
      ├──SPR-100  Feature A: User management system
      │  ├──SPR-101  Add user registration
      │  ├──SPR-102  Implement user authentication
      │  └──+ Add subtask
      ├──SPR-200  Feature B: Dashboard and analytics
      └──SPR-300  Bug fix: Payment processing errors
      """
    When I press "down"
    And I press "down"
    And I press "down"
    And I press "down"
    Then the UI should display:
      """
      🌱 sprout

      > spr-200-feature-b-dashboard-and-analytics
      ├──SPR-100  Feature A: User management system
      │  ├──SPR-101  Add user registration
      │  ├──SPR-102  Implement user authentication
      │  └──+ Add subtask
      ├──SPR-200  Feature B: Dashboard and analytics
      └──SPR-300  Bug fix: Payment processing errors
      """
    When I press "right"
    Then the UI should display:
      """
      🌱 sprout

      > spr-200-feature-b-dashboard-and-analytics
      ├──SPR-100  Feature A: User management system
      │  ├──SPR-101  Add user registration
      │  ├──SPR-102  Implement user authentication
      │  └──+ Add subtask
      ├──SPR-200  Feature B: Dashboard and analytics
      │  ├──SPR-201  Create dashboard layout
      │  ├──SPR-202  Add analytics widgets
      │  ├──SPR-203  Implement data visualization
      │  └──+ Add subtask
      └──SPR-300  Bug fix: Payment processing errors
      """