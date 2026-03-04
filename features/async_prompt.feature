Feature: Async prompt capture while creating worktrees
  As a developer using Sprout
  I want to type prompt text while git commands are running
  So that post-create commands can launch immediately once worktree setup completes

  Background:
    Given the following Linear issues exist:
      | identifier | title                                           | parent_id | status      |
      | SPR-123    | Add user authentication                         |           | Todo        |
      | SPR-124    | Implement dashboard with analytics and reporting |           | In Progress |
      | SPR-125    | Create analytics component                      | SPR-124   | Todo        |
      | SPR-126    | Add reporting metrics                           | SPR-124   | In Review   |
      | SPR-127    | Fix critical bug in payment processing          |           | Done        |

  Scenario: Queue prompt while worktree creation is still running
    Given the default worktree command is "codex \"$PROMPT\""
    And worktree creation is delayed
    And I start the Sprout TUI
    When I press "down"
    And I press "enter"
    And I type "write tests"
    And I press "enter"
    Then the UI should contain "Prompt queued, waiting for git..."
    When worktree creation completes
    Then the following commands should be run:
      | command                                                                                                       |
      | git worktree add /mock/worktrees/spr-123-add-user-authentication -b spr-123-add-user-authentication main |
      | cd /mock/worktrees/spr-123-add-user-authentication && codex "write tests"                                |

  Scenario: Wait for prompt submission when worktree finishes first
    Given the default worktree command is "codex \"$PROMPT\""
    And worktree creation is delayed
    And I start the Sprout TUI
    When I press "down"
    And I press "enter"
    And worktree creation completes
    Then the UI should contain "Worktree ready, press Enter to launch"
    When I type "ship it"
    And I press "enter"
    Then the following commands should be run:
      | command                                                                                                       |
      | git worktree add /mock/worktrees/spr-123-add-user-authentication -b spr-123-add-user-authentication main |
      | cd /mock/worktrees/spr-123-add-user-authentication && codex "ship it"                                    |

  Scenario: No placeholder keeps legacy behavior
    Given the default worktree command is "code ."
    And worktree creation is delayed
    And I start the Sprout TUI
    When I press "down"
    And I press "enter"
    And worktree creation completes
    Then the following commands should be run:
      | command                                                                                                       |
      | git worktree add /mock/worktrees/spr-123-add-user-authentication -b spr-123-add-user-authentication main |
      | cd /mock/worktrees/spr-123-add-user-authentication && code .                                               |

  Scenario: Substitute repeated prompt placeholders
    Given the default worktree command is "tool --a \"$PROMPT\" --b \"$PROMPT\""
    And worktree creation is delayed
    And I start the Sprout TUI
    When I press "down"
    And I press "enter"
    And I type "alpha"
    And I press "enter"
    And worktree creation completes
    Then the following commands should be run:
      | command                                                                                                       |
      | git worktree add /mock/worktrees/spr-123-add-user-authentication -b spr-123-add-user-authentication main |
      | cd /mock/worktrees/spr-123-add-user-authentication && tool --a alpha --b alpha                            |

  Scenario: Preserve multiline prompt text
    Given the default worktree command is "codex \"$PROMPT\""
    And worktree creation is delayed
    And I start the Sprout TUI
    When I press "down"
    And I press "enter"
    And I type "outline tests"
    And I press "alt+enter"
    And I type "include edge-cases"
    And I press "enter"
    And worktree creation completes
    Then the following commands should be run:
      | command                                                                                                              |
      | git worktree add /mock/worktrees/spr-123-add-user-authentication -b spr-123-add-user-authentication main        |
      | cd /mock/worktrees/spr-123-add-user-authentication && codex "outline tests\ninclude edge-cases"                 |

  Scenario: Preserve multiline prompt text with shift+enter
    Given the default worktree command is "codex \"$PROMPT\""
    And worktree creation is delayed
    And I start the Sprout TUI
    When I press "down"
    And I press "enter"
    And I type "first line"
    And I press "shift+enter"
    And I type "second line"
    And I press "enter"
    And worktree creation completes
    Then the following commands should be run:
      | command                                                                                                     |
      | git worktree add /mock/worktrees/spr-123-add-user-authentication -b spr-123-add-user-authentication main |
      | cd /mock/worktrees/spr-123-add-user-authentication && codex "first line\nsecond line"                   |

  Scenario: Prompt longer than 156 characters is not truncated
    Given the default worktree command is "tool --a \"$PROMPT\""
    And worktree creation is delayed
    And I start the Sprout TUI
    When I press "down"
    And I press "enter"
    And I type "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnop"
    And I press "enter"
    And worktree creation completes
    Then the following commands should be run:
      | command                                                                                                                                                                                                    |
      | git worktree add /mock/worktrees/spr-123-add-user-authentication -b spr-123-add-user-authentication main                                                                                              |
      | cd /mock/worktrees/spr-123-add-user-authentication && tool --a abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnop |
