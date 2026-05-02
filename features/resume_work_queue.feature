Feature: Resume work from an interleaved work queue
  As a developer using Sprout
  I want Linear tickets and existing worktrees shown together
  So that I can either start new work or resume existing work quickly

  Background:
    Given the following Linear issues exist:
      | identifier | title                 | parent_id | status      | updated_at           |
      | SPR-124    | Dashboard analytics   |           | In Progress | 2026-05-01T10:00:00Z |
      | SPR-125    | Create analytics card | SPR-124   | Todo        | 2026-05-01T09:00:00Z |
      | SPR-140    | Fix onboarding copy   |           | Todo        | 2026-04-30T12:00:00Z |
      | SPR-141    | Old auth cleanup      |           | Done        | 2026-05-02T09:00:00Z |
    And the following worktrees exist:
      | branch                      | path                                         | updated_at           | merged |
      | spr-124-dashboard-analytics | /mock/worktrees/spr-124-dashboard-analytics | 2026-05-02T08:00:00Z | false  |
      | feature-search              | /mock/worktrees/feature-search              | 2026-05-01T16:00:00Z | false  |
      | misc-cleanup                | /mock/worktrees/misc-cleanup                | 2026-04-29T10:00:00Z | false  |
      | old-merged-branch           | /mock/worktrees/old-merged-branch           | 2026-05-02T07:00:00Z | true   |

  Scenario: Initial list interleaves active tickets and worktrees
    When I start the Sprout TUI
    Then the UI should display:
      """
      🌱 sprout

      > sprout/█enter branch name or select suggestion below
      ├──feature-search
      ├──SPR-124   In Progress  Dashboard analytics
      ├──SPR-140   Todo         Fix onboarding copy
      └──misc-cleanup
      [worktree <tab>] [a all] [u unassign] [d done] [z undo]
      """

  Scenario: Matching worktree and Linear ticket render as a single Linear row
    When I start the Sprout TUI
    Then the UI should not display "spr-124-dashboard-analytics"
    And the UI should display "SPR-124   In Progress  Dashboard analytics"

  Scenario: Selecting a matched Linear row resumes its existing worktree
    Given I start the Sprout TUI
    When I press "down"
    And I press "down"
    And I press "enter"
    Then the TUI should resume worktree "/mock/worktrees/spr-124-dashboard-analytics"
    And no new worktree should be created

  Scenario: Selecting a Linear-only row creates a new worktree
    Given I start the Sprout TUI
    When I press "down"
    And I press "down"
    And I press "down"
    And I press "enter"
    Then a worktree should be created for branch "spr-140-fix-onboarding-copy"

  Scenario: Selecting a worktree-only row resumes that worktree
    Given I start the Sprout TUI
    When I press "down"
    And I press "enter"
    Then the TUI should resume worktree "/mock/worktrees/feature-search"
    And no new worktree should be created

  Scenario: Identifier matching is delimiter safe
    Given the following Linear issues exist:
      | identifier | title       | parent_id | status | updated_at           |
      | SPR-12     | Short issue |           | Todo   | 2026-05-01T10:00:00Z |
    And the following worktrees exist:
      | branch        | path                    | updated_at           | merged |
      | spr-123-other | /mock/worktrees/spr-123 | 2026-05-02T08:00:00Z | false  |
    When I start the Sprout TUI
    Then the UI should display "SPR-12"
    And the UI should display "spr-123-other"

  Scenario: Disclosures still expand Linear sub-tickets
    Given I start the Sprout TUI
    When I press "down"
    And I press "down"
    And I press "right"
    Then the UI should display:
      """
      🌱 sprout

      > sprout/spr-124-dashboard-analytics
      ├──feature-search
      ├──SPR-124   In Progress  Dashboard analytics
      │  ├──SPR-125   Todo         Create analytics card
      │  └──+ Add subtask
      ├──SPR-140   Todo         Fix onboarding copy
      └──misc-cleanup
      [worktree <tab>] [a all] [u unassign] [d done] [z undo]
      """

  Scenario: Worktree-only rows are leaves
    Given I start the Sprout TUI
    When I press "down"
    And I press "right"
    Then the UI should display:
      """
      🌱 sprout

      > sprout/feature-search
      ├──feature-search
      ├──SPR-124   In Progress  Dashboard analytics
      ├──SPR-140   Todo         Fix onboarding copy
      └──misc-cleanup
      [worktree <tab>] [a all] [u unassign] [d done] [z undo]
      """

  Scenario: Closed and merged rows are hidden by default
    When I start the Sprout TUI
    Then the UI should not display "SPR-141"
    And the UI should not display "old-merged-branch"

  Scenario: Toggle all rows shows closed and merged items after active items
    Given I start the Sprout TUI
    When I press "a"
    Then the UI should display:
      """
      🌱 sprout

      > sprout/█enter branch name or select suggestion below
      ├──feature-search
      ├──SPR-124   In Progress  Dashboard analytics
      ├──SPR-140   Todo         Fix onboarding copy
      ├──misc-cleanup
      ├──SPR-141   Done         Old auth cleanup
      └──old-merged-branch
      [worktree <tab>] [a active] [u unassign] [d done] [z undo]
      """

  Scenario: Default list is limited to twenty active rows
    Given 25 active work queue rows exist
    When I start the Sprout TUI
    Then the UI should show 20 work queue rows

  Scenario: Search filters tickets and branch names
    Given I start the Sprout TUI
    When I type "/search"
    Then the UI should display "feature-search"
    And the UI should not display "SPR-124"
