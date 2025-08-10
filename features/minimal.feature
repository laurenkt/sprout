Feature: Minimal TUI Model
  As a developer
  I want to test the basic TUI functionality
  So that I can ensure the foundation works correctly

  Scenario: Minimal model displays correctly
    Given I have a minimal TUI model
    When I render the view
    Then the output should be "Hello World"

  Scenario: Minimal model handles quit command
    Given I have a minimal TUI model
    When I send a quit command
    Then the program should exit gracefully