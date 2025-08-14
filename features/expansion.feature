Feature: Sprout TUI Tree Expansion
  As a developer using Sprout
  I want to expand and collapse issue trees
  So that I can view and navigate through sub-issues

  Scenario: Expand multiple issue trees
    Given the following Linear issues exist:
      | identifier | title                         | parent_id | status      |
      | SPR-100    | Feature A: User management system    |           | In Progress |
      | SPR-101    | Add user registration         | SPR-100   | Done        |
      | SPR-102    | Implement user authentication | SPR-100   | Todo        |
      | SPR-200    | Feature B: Dashboard and analytics   |           | Todo        |
      | SPR-201    | Create dashboard layout       | SPR-200   | In Progress |
      | SPR-202    | Add analytics widgets         | SPR-200   | Todo        |
      | SPR-203    | Implement data visualization  | SPR-200   | Backlog     |
      | SPR-300    | Bug fix: Payment processing errors   |           | In Review   |
    When I start the Sprout TUI
    Then the UI should display:
      """
      🌱 sprout

      > sprout/enter branch name or select suggestion below
      ├──SPR-100  In Progress  Feature A: User management system
      ├──SPR-200  Todo         Feature B: Dashboard and analytics
      └──SPR-300  In Review    Bug fix: Payment processing errors
      """
    When I press "down"
    Then the UI should display:
      """
      🌱 sprout

      > sprout/spr-100-feature-a-user-management-system
      ├──SPR-100  In Progress  Feature A: User management system
      ├──SPR-200  Todo         Feature B: Dashboard and analytics
      └──SPR-300  In Review    Bug fix: Payment processing errors
      """
    When I press "right"
    Then the UI should display:
      """
      🌱 sprout

      > sprout/spr-100-feature-a-user-management-system
      ├──SPR-100  In Progress  Feature A: User management system
      │  ├──SPR-101  Done         Add user registration
      │  ├──SPR-102  Todo         Implement user authentication
      │  └──+ Add subtask
      ├──SPR-200  Todo         Feature B: Dashboard and analytics
      └──SPR-300  In Review    Bug fix: Payment processing errors
      """
    When I press "down"
    And I press "down"
    And I press "down"
    And I press "down"
    Then the UI should display:
      """
      🌱 sprout

      > sprout/spr-200-feature-b-dashboard-and-analytics
      ├──SPR-100  In Progress  Feature A: User management system
      │  ├──SPR-101  Done         Add user registration
      │  ├──SPR-102  Todo         Implement user authentication
      │  └──+ Add subtask
      ├──SPR-200  Todo         Feature B: Dashboard and analytics
      └──SPR-300  In Review    Bug fix: Payment processing errors
      """
    When I press "right"
    Then the UI should display:
      """
      🌱 sprout

      > sprout/spr-200-feature-b-dashboard-and-analytics
      ├──SPR-100  In Progress  Feature A: User management system
      │  ├──SPR-101  Done         Add user registration
      │  ├──SPR-102  Todo         Implement user authentication
      │  └──+ Add subtask
      ├──SPR-200  Todo         Feature B: Dashboard and analytics
      │  ├──SPR-201  In Progress  Create dashboard layout
      │  ├──SPR-202  Todo         Add analytics widgets
      │  ├──SPR-203  Backlog      Implement data visualization
      │  └──+ Add subtask
      └──SPR-300  In Review    Bug fix: Payment processing errors
      """