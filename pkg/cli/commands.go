package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"sprout/pkg/config"
	"sprout/pkg/git"
	"sprout/pkg/linear"
	"sprout/pkg/ui"
)

// Dependencies represents the external dependencies for CLI commands
type Dependencies struct {
	WorktreeManager git.WorktreeManagerInterface
	ConfigLoader    config.LoaderInterface
	LinearClient    linear.LinearClientInterface
	Output          io.Writer
	ErrorOutput     io.Writer
}

// NewDependencies creates production dependencies
func NewDependencies() (*Dependencies, error) {
	wm, err := git.NewWorktreeManager()
	if err != nil {
		return nil, err
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	var linearClient linear.LinearClientInterface
	if cfg.LinearAPIKey != "" {
		linearClient = linear.NewClient(cfg.LinearAPIKey)
	}

	return &Dependencies{
		WorktreeManager: wm,
		ConfigLoader:    &config.DefaultLoader{Config: cfg},
		LinearClient:    linearClient,
		Output:          os.Stdout,
		ErrorOutput:     os.Stderr,
	}, nil
}

// HandleListCommand handles the list command
func HandleListCommand(deps *Dependencies) error {
	worktrees, err := deps.WorktreeManager.ListWorktrees()
	if err != nil {
		return err
	}

	var filteredWorktrees []git.Worktree
	for _, wt := range worktrees {
		if wt.Branch != "master" && wt.Branch != "main" && wt.Branch != "" {
			filteredWorktrees = append(filteredWorktrees, wt)
		}
	}

	if len(filteredWorktrees) == 0 {
		fmt.Fprintln(deps.Output, "No worktrees found")
		return nil
	}

	// Create table with consistent styling
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("69")).
		Bold(true)

	branchStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("108"))

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("243"))).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == 0 {
				return headerStyle
			}
			if col == 0 {
				return branchStyle
			}
			return normalStyle
		}).
		Headers("BRANCH", "PR STATUS", "COMMIT")

	for _, wt := range filteredWorktrees {
		commit := wt.Commit
		if len(commit) > 8 {
			commit = commit[:8]
		}
		t.Row(wt.Branch, wt.PRStatus, commit)
	}

	fmt.Fprintln(deps.Output, headerStyle.Render("🌱 Active Worktrees"))
	fmt.Fprintln(deps.Output)
	fmt.Fprintln(deps.Output, t)

	return nil
}

// HandleDoctorCommand handles the doctor command
func HandleDoctorCommand(deps *Dependencies) error {
	cfg, err := deps.ConfigLoader.GetConfig()
	if err != nil {
		return err
	}

	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("69")).
		Bold(true)

	accentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("108"))

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	warningStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("221"))

	fmt.Fprintln(deps.Output, headerStyle.Render("🌱 Sprout Configuration"))
	fmt.Fprintln(deps.Output)

	defaultCmd := cfg.DefaultCommand
	if defaultCmd == "" {
		defaultCmd = "not configured"
	}
	fmt.Fprintf(deps.Output, "  %s: %s\n", accentStyle.Render("Default Command"), normalStyle.Render(defaultCmd))

	if cfg.GetLinearAPIKey() != "" {
		fmt.Fprintf(deps.Output, "  %s: %s\n", accentStyle.Render("Linear API Key"), normalStyle.Render("configured"))
	} else {
		fmt.Fprintf(deps.Output, "  %s: %s\n", accentStyle.Render("Linear API Key"), warningStyle.Render("not configured"))
	}

	configPath, err := getConfigPath()
	if err != nil {
		fmt.Fprintf(deps.Output, "  %s: %s\n", accentStyle.Render("Config Path"), warningStyle.Render(fmt.Sprintf("<error: %v>", err)))
	} else {
		fmt.Fprintf(deps.Output, "  %s: %s\n", accentStyle.Render("Config Path"), normalStyle.Render(configPath))

		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			fmt.Fprintf(deps.Output, "  %s: %s\n", accentStyle.Render("Config File"), warningStyle.Render("not found (using defaults)"))
		} else {
			fmt.Fprintf(deps.Output, "  %s: %s\n", accentStyle.Render("Config File"), normalStyle.Render("exists"))
		}
	}

	fmt.Fprintln(deps.Output)
	fmt.Fprintln(deps.Output, headerStyle.Render("Linear Integration"))
	fmt.Fprintln(deps.Output)

	if cfg.LinearAPIKey == "" {
		fmt.Fprintf(deps.Output, "  %s: %s\n", accentStyle.Render("API Key"), warningStyle.Render("not configured"))
		fmt.Fprintf(deps.Output, "  %s: %s\n", accentStyle.Render("Status"), warningStyle.Render("disabled"))
	} else {
		// Mask the key for security
		maskedKey := cfg.LinearAPIKey
		if len(maskedKey) > 8 {
			maskedKey = maskedKey[:8] + "..." + maskedKey[len(maskedKey)-4:]
		}
		fmt.Fprintf(deps.Output, "  %s: %s\n", accentStyle.Render("API Key"), normalStyle.Render(maskedKey))

		fmt.Fprintf(deps.Output, "  %s: ", accentStyle.Render("Status"))
		testLinearConnection(deps.LinearClient, deps.Output)
	}

	return nil
}

// HandleHelpCommand handles the help command
func HandleHelpCommand(deps *Dependencies) {
	fmt.Fprintln(deps.Output, "Sprout - Git Worktree Terminal UI")
	fmt.Fprintln(deps.Output)
	fmt.Fprintln(deps.Output, "Usage:")
	fmt.Fprintln(deps.Output, "  sprout                              Start in interactive mode")
	fmt.Fprintln(deps.Output, "  sprout list                         List all worktrees")
	fmt.Fprintln(deps.Output, "  sprout create <branch>              Create worktree and output path")
	fmt.Fprintln(deps.Output, "  sprout create <branch> <command>    Create worktree and run command in it")
	fmt.Fprintln(deps.Output, "  sprout prune [branch]               Remove worktree(s) - all merged if no branch specified")
	fmt.Fprintln(deps.Output, "  sprout doctor                       Show configuration values")
	fmt.Fprintln(deps.Output, "  sprout help                         Show this help")
	fmt.Fprintln(deps.Output)
	fmt.Fprintln(deps.Output, "Examples:")
	fmt.Fprintln(deps.Output, "  sprout list                          # Show all worktrees")
	fmt.Fprintln(deps.Output, "  cd \"$(sprout create mybranch)\"       # Change to worktree directory")
	fmt.Fprintln(deps.Output, "  sprout create mybranch bash          # Create worktree and start bash")
	fmt.Fprintln(deps.Output, "  sprout create mybranch code .        # Create worktree and open in VS Code")
	fmt.Fprintln(deps.Output, "  sprout create mybranch git status    # Create worktree and run git status")
	fmt.Fprintln(deps.Output, "  sprout prune                         # Remove all merged worktrees")
	fmt.Fprintln(deps.Output, "  sprout prune mybranch                # Remove specific worktree and directory")
}

func getConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/.sprout.json5", homeDir), nil
}

func testLinearConnection(client linear.LinearClientInterface, output io.Writer) {
	if client == nil {
		return
	}

	successStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("108")).
		Bold(true)

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("204")).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	user, err := client.GetCurrentUser()
	if err != nil {
		fmt.Fprintf(output, "%s\n", errorStyle.Render("✗ Failed"))
		fmt.Fprintf(output, "  %s\n", errorStyle.Render(fmt.Sprintf("Error: %v", err)))
		return
	}

	fmt.Fprintf(output, "%s\n", successStyle.Render("✓ Connected"))
	fmt.Fprintf(output, "  %s: %s\n", normalStyle.Render("User"), normalStyle.Render(fmt.Sprintf("%s (%s)", user.Name, user.Email)))

	// Try to fetch assigned issues
	issues, err := client.GetAssignedIssues()
	if err != nil {
		fmt.Fprintf(output, "  %s: %s\n", normalStyle.Render("Assigned Issues"), errorStyle.Render(fmt.Sprintf("<error fetching: %v>", err)))
	} else {
		fmt.Fprintf(output, "  %s: %s\n", normalStyle.Render("Assigned Issues"), normalStyle.Render(fmt.Sprintf("%d active tickets", len(issues))))
	}
}

// Run handles the main CLI logic and returns an exit code
func Run(args []string) int {
	// Create dependencies for CLI commands
	deps, err := NewDependencies()
	if err != nil {
		fmt.Printf("Error: Failed to initialize dependencies: %v\n", err)
		return 1
	}
	
	return RunWithDependencies(args, deps)
}

// RunWithDependencies handles CLI logic with injected dependencies for testing
func RunWithDependencies(args []string, deps *Dependencies) int {
	if len(args) < 2 {
		// Interactive mode
		if err := ui.RunInteractive(); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
		return 0
	}

	// One-shot mode
	command := args[1]
	switch command {
	case "create":
		if err := handleCreateCommandWithDeps(args[2:], deps); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
	case "list":
		if err := HandleListCommand(deps); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
	case "prune":
		if err := handlePruneCommandWithDeps(args[2:], deps); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
	case "doctor":
		if err := HandleDoctorCommand(deps); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
	case "help", "--help", "-h":
		HandleHelpCommand(deps)
		return 0
	default:
		fmt.Fprintf(deps.ErrorOutput, "Unknown command: %s\n", command)
		HandleHelpCommand(deps)
		return 1
	}
	return 0
}

// Legacy functions for backward compatibility
func handleCreateCommand(args []string) error {
	deps, err := NewDependencies()
	if err != nil {
		return err
	}
	return handleCreateCommandWithDeps(args, deps)
}

func handlePruneCommand(args []string) error {
	deps, err := NewDependencies()
	if err != nil {
		return err
	}
	return handlePruneCommandWithDeps(args, deps)
}

func handleCreateCommandWithDeps(args []string, deps *Dependencies) error {
	if len(args) == 0 {
		return fmt.Errorf("branch name is required. Usage: sprout create <branch-name> [command...]")
	}
	
	branchName := args[0]
	
	worktreePath, err := deps.WorktreeManager.CreateWorktree(branchName)
	if err != nil {
		return err
	}
	
	fmt.Fprintf(deps.ErrorOutput, "Worktree ready at: %s\n", worktreePath)
	
	// If no command provided, check for default command
	if len(args) == 1 {
		cfg, err := deps.ConfigLoader.GetConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		
		defaultCmd := cfg.GetDefaultCommand()
		if len(defaultCmd) > 0 {
			// Execute the default command in the worktree directory
			cmd := exec.Command(defaultCmd[0], defaultCmd[1:]...)
			cmd.Dir = worktreePath
			cmd.Stdin = os.Stdin
			cmd.Stdout = deps.Output
			cmd.Stderr = deps.ErrorOutput
			
			if err := cmd.Run(); err != nil {
				if exitError, ok := err.(*exec.ExitError); ok {
					if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
						fmt.Fprintf(deps.ErrorOutput, "\nWorktree directory: %s\n", worktreePath)
						os.Exit(status.ExitStatus())
					}
				}
				return fmt.Errorf("default command failed: %w", err)
			}
			fmt.Fprintf(deps.ErrorOutput, "\nWorktree directory: %s\n", worktreePath)
			return nil
		}
		
		// No default command, output path for shell evaluation
		fmt.Fprint(deps.Output, worktreePath)
		return nil
	}
	
	// Execute the provided command in the worktree directory
	command := args[1]
	commandArgs := args[2:]
	
	cmd := exec.Command(command, commandArgs...)
	cmd.Dir = worktreePath
	cmd.Stdin = os.Stdin
	cmd.Stdout = deps.Output
	cmd.Stderr = deps.ErrorOutput
	
	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				fmt.Fprintf(deps.ErrorOutput, "\nWorktree directory: %s\n", worktreePath)
				os.Exit(status.ExitStatus())
			}
		}
		return fmt.Errorf("command failed: %w", err)
	}
	
	fmt.Fprintf(deps.ErrorOutput, "\nWorktree directory: %s\n", worktreePath)
	return nil
}

func handlePruneCommandWithDeps(args []string, deps *Dependencies) error {
	if len(args) == 0 {
		// Prune all merged branches
		return deps.WorktreeManager.PruneAllMerged()
	}
	
	branchName := args[0]
	return deps.WorktreeManager.PruneWorktree(branchName)
}