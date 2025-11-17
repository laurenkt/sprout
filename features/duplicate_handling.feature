Feature: Sprout duplicate issue handling
  As a developer using Sprout
  I want to avoid seeing duplicate issues
  So that each ticket appears only once in the most appropriate location

  Scenario: Hide child issues when parent is also assigned
    Given the following Linear issues exist:
      | identifier | title                    | parent_id | status      |
      | TICK-1     | Parent Task              |           | In Progress |
      | TICK-2     | Child Task               | TICK-1    | Todo        |
    When I start the Sprout TUI
    Then the UI should display:
      """
      🌱 sprout

      > sprout/enter branch name or select suggestion below
      └──TICK-1  In Progress  Parent Task
      [worktree <tab>]
      """
    When I press "down"
    And I press "right"
    Then the UI should display:
      """
      🌱 sprout

      > sprout/tick-1-parent-task
      └──TICK-1  In Progress  Parent Task
         ├──TICK-2  Todo         Child Task
         └──+ Add subtask
      [worktree <tab>]
      """

  Scenario: Multiple nested levels only show top-level parents
    Given the following Linear issues exist:
      | identifier | title         | parent_id | status      |
      | TICK-1     | Parent Task   |           | In Progress |
      | TICK-2     | Child Task    | TICK-1    | Todo        |
      | TICK-3     | Grandchild    | TICK-2    | Done        |
      | TICK-4     | Solo Task     |           | In Review   |
    When I start the Sprout TUI
    Then the UI should display:
      """
      🌱 sprout

      > sprout/enter branch name or select suggestion below
      ├──TICK-1  In Progress  Parent Task
      └──TICK-4  In Review    Solo Task
      [worktree <tab>]
      """