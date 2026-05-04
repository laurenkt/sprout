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
      [worktree <tab>] [u unassign] [d done] [z undo]
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
      [worktree <tab>] [u unassign] [d done] [z undo]
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
      [worktree <tab>] [u unassign] [d done] [z undo]
      """

  Scenario: Recently updated assigned subtasks keep their parent discoverable
    Given the following Linear issues exist:
      | identifier | title             | parent_id | status      | updated_at           |
      | TICK-1     | Parent Task       |           | In Progress | 2026-05-01T08:00:00Z |
      | TICK-2     | New Child Task    | TICK-1    | Todo        | 2026-05-02T12:00:00Z |
      | TICK-3     | Other Recent Task |           | Todo        | 2026-05-02T10:00:00Z |
    When I start the Sprout TUI
    Then the UI should display:
      """
      🌱 sprout

      > sprout/enter branch name or select suggestion below
      ├──TICK-1  In Progress  Parent Task
      └──TICK-3  Todo         Other Recent Task
      [worktree <tab>] [u unassign] [d done] [z undo]
      """
    When I press "down"
    And I press "right"
    Then the UI should display:
      """
      🌱 sprout

      > sprout/tick-1-parent-task
      ├──TICK-1  In Progress  Parent Task
      │  ├──TICK-2  Todo         New Child Task
      │  └──+ Add subtask
      └──TICK-3  Todo         Other Recent Task
      [worktree <tab>] [u unassign] [d done] [z undo]
      """

  Scenario: Parent can still disclose add-subtask row when child loading fails
    Given the following Linear issues exist:
      | identifier | title       | parent_id | status      |
      | TICK-1     | Parent Task |           | In Progress |
      | TICK-2     | Child Task  | TICK-1    | Todo        |
    And fetching children for "TICK-1" fails
    And my terminal width is 120 characters
    When I start the Sprout TUI
    And I press "down"
    And I press "right"
    Then the UI should display:
      """
      🌱 sprout

      > sprout/tick-1-parent-task
      └──TICK-1  In Progress  Parent Task
         └──+ Add subtask
      [worktree <tab>] [u unassign] [d done] [z undo]                                      failed to fetch children for TICK-1
      """
