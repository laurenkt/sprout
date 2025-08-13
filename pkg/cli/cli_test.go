package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/cucumber/godog"
	"sprout/pkg/config"
	"sprout/pkg/git"
	"sprout/pkg/linear"
)

// CLITestContext holds the state for CLI Gherkin tests
type CLITestContext struct {
	lastCommand    []string
	lastOutput     string
	lastExitCode   int
	originalStdout *os.File
	originalStderr *os.File
	outputBuffer   *bytes.Buffer
	errorBuffer    *bytes.Buffer
	deps           *Dependencies
	t              *testing.T
}

// NewCLITestContext creates a new CLI test context
func NewCLITestContext(t *testing.T) *CLITestContext {
	outputBuffer := &bytes.Buffer{}
	errorBuffer := &bytes.Buffer{}
	
	return &CLITestContext{
		t:              t,
		outputBuffer:   outputBuffer,
		errorBuffer:    errorBuffer,
		originalStdout: os.Stdout,
		originalStderr: os.Stderr,
		deps: &Dependencies{
			WorktreeManager:    &MockWorktreeManager{Worktrees: []git.Worktree{}},
			ConfigLoader:       &MockConfigLoader{Config: &config.Config{}},
			LinearClient:       nil,
			ConfigPathProvider: &MockConfigPathProvider{
				ConfigPath: "/Users/laurenkt/.sprout.json5",
				FileExists: true,
			},
			Output:      outputBuffer,
			ErrorOutput: errorBuffer,
		},
	}
}

// Step definitions

func (tc *CLITestContext) iRun(command string) error {
	// Parse command into parts to create mock os.Args
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}
	
	tc.lastCommand = parts
	
	// Clear output buffers
	tc.outputBuffer.Reset()
	tc.errorBuffer.Reset()
	
	// Run the CLI with mocked dependencies
	tc.lastExitCode = RunWithDependencies(parts, tc.deps)
	
	// Capture output from buffers
	tc.lastOutput = tc.outputBuffer.String() + tc.errorBuffer.String()
	
	return nil
}


func (tc *CLITestContext) noWorktreesExist() error {
	tc.deps.WorktreeManager.(*MockWorktreeManager).Worktrees = []git.Worktree{}
	return nil
}

func (tc *CLITestContext) theFollowingWorktreesExist(worktreeTable *godog.Table) error {
	var worktrees []git.Worktree
	
	for i, row := range worktreeTable.Rows {
		if i == 0 { // Skip header row
			continue
		}
		
		branch := row.Cells[0].Value
		commit := row.Cells[1].Value
		prStatus := row.Cells[2].Value
		
		worktrees = append(worktrees, git.Worktree{
			Branch:   branch,
			Commit:   commit,
			PRStatus: prStatus,
		})
	}
	
	tc.deps.WorktreeManager.(*MockWorktreeManager).Worktrees = worktrees
	return nil
}

func (tc *CLITestContext) aConfigWith(configTable *godog.Table) error {
	cfg := &config.Config{}
	
	for i, row := range configTable.Rows {
		if i == 0 { // Skip header row
			continue
		}
		
		key := row.Cells[0].Value
		value := row.Cells[1].Value
		
		switch key {
		case "default_command":
			if value != "<not_set>" {
				cfg.DefaultCommand = value
			}
		case "linear_api_key":
			if value != "<not_set>" {
				cfg.LinearAPIKey = value
				// Set up mock Linear client if API key is provided
				tc.deps.LinearClient = &MockLinearClient{
					CurrentUser: &linear.User{
						Name:  "Test User",
						Email: "test@example.com",
					},
					AssignedIssues: []linear.Issue{},
				}
			}
		}
	}
	
	tc.deps.ConfigLoader.(*MockConfigLoader).Config = cfg
	return nil
}

func (tc *CLITestContext) theOutputShouldBe(expected *godog.DocString) error {
	expectedContent := strings.TrimSpace(expected.Content)
	actualContent := strings.TrimSpace(tc.lastOutput)
	
	if actualContent != expectedContent {
		return fmt.Errorf("output mismatch:\nExpected:\n%s\n\nActual:\n%s", expectedContent, actualContent)
	}
	
	return nil
}

func (tc *CLITestContext) theCommandShouldFail() error {
	if tc.lastExitCode == 0 {
		return fmt.Errorf("expected command to fail but it succeeded")
	}
	return nil
}

// InitializeCLIScenario initializes godog with CLI step definitions
func InitializeCLIScenario(ctx *godog.ScenarioContext, t *testing.T) {
	var tc *CLITestContext
	
	// Setup a test context for each scenario
	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		// Create fresh test context for each scenario
		tc = NewCLITestContext(t)
		return ctx, nil
	})
	
	// Step definitions
	ctx.Step(`^I run "([^"]*)"$`, func(command string) error {
		return tc.iRun(command)
	})
	ctx.Step(`^no worktrees exist$`, func() error {
		return tc.noWorktreesExist()
	})
	ctx.Step(`^the following worktrees exist:$`, func(table *godog.Table) error {
		return tc.theFollowingWorktreesExist(table)
	})
	ctx.Step(`^a config with:$`, func(table *godog.Table) error {
		return tc.aConfigWith(table)
	})
	ctx.Step(`^the output should be:$`, func(expected *godog.DocString) error {
		return tc.theOutputShouldBe(expected)
	})
	ctx.Step(`^the command should fail$`, func() error {
		return tc.theCommandShouldFail()
	})
}

// TestCLIFeatures runs the CLI Gherkin tests
func TestCLIFeatures(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: func(ctx *godog.ScenarioContext) {
			InitializeCLIScenario(ctx, t)
		},
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"../../features/cli_commands.feature"},
			TestingT: t,
		},
	}
	
	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run CLI feature tests")
	}
}