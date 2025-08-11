package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"sprout/pkg/config"
	"sprout/pkg/git"
	"sprout/pkg/linear"
	"sprout/pkg/ui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error: Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if len(os.Args) < 2 {
		// Interactive mode
		if err := ui.RunInteractive(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// One-shot mode
	command := os.Args[1]
	switch command {
	case "create":
		if err := handleCreateCommand(os.Args[2:]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	case "list":
		if err := handleListCommand(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	case "prune":
		if err := handlePruneCommand(os.Args[2:]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	case "doctor":
		handleDoctorCommand(cfg)
	case "help", "--help", "-h":
		printHelp()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printHelp()
		os.Exit(1)
	}
}

func handleCreateCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("branch name is required. Usage: sprout create <branch-name> [command...]")
	}
	
	branchName := args[0]
	
	wm, err := git.NewWorktreeManager()
	if err != nil {
		return err
	}
	
	worktreePath, err := wm.CreateWorktree(branchName)
	if err != nil {
		return err
	}
	
	fmt.Fprintf(os.Stderr, "Worktree ready at: %s\n", worktreePath)
	
	// If no command provided, check for default command
	if len(args) == 1 {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		
		defaultCmd := cfg.GetDefaultCommand()
		if len(defaultCmd) > 0 {
			// Execute the default command in the worktree directory
			cmd := exec.Command(defaultCmd[0], defaultCmd[1:]...)
			cmd.Dir = worktreePath
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			
			if err := cmd.Run(); err != nil {
				if exitError, ok := err.(*exec.ExitError); ok {
					if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
						fmt.Fprintf(os.Stderr, "\nWorktree directory: %s\n", worktreePath)
						os.Exit(status.ExitStatus())
					}
				}
				return fmt.Errorf("default command failed: %w", err)
			}
			fmt.Fprintf(os.Stderr, "\nWorktree directory: %s\n", worktreePath)
			return nil
		}
		
		// No default command, output path for shell evaluation
		fmt.Print(worktreePath)
		return nil
	}
	
	// Execute the provided command in the worktree directory
	command := args[1]
	commandArgs := args[2:]
	
	cmd := exec.Command(command, commandArgs...)
	cmd.Dir = worktreePath
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				fmt.Fprintf(os.Stderr, "\nWorktree directory: %s\n", worktreePath)
				os.Exit(status.ExitStatus())
			}
		}
		return fmt.Errorf("command failed: %w", err)
	}
	
	fmt.Fprintf(os.Stderr, "\nWorktree directory: %s\n", worktreePath)
	return nil
}

func handleListCommand() error {
	wm, err := git.NewWorktreeManager()
	if err != nil {
		return err
	}
	
	worktrees, err := wm.ListWorktrees()
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
		fmt.Println("No worktrees found")
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
	
	fmt.Println(headerStyle.Render("🌱 Active Worktrees"))
	fmt.Println()
	fmt.Println(t)
	
	return nil
}

func handlePruneCommand(args []string) error {
	wm, err := git.NewWorktreeManager()
	if err != nil {
		return err
	}
	
	if len(args) == 0 {
		// Prune all merged branches
		return wm.PruneAllMerged()
	}
	
	branchName := args[0]
	return wm.PruneWorktree(branchName)
}

func handleDoctorCommand(cfg *config.Config) {
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("69")).
		Bold(true)
	
	accentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("108"))
	
	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))
	
	warningStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("221"))

	fmt.Println(headerStyle.Render("🌱 Sprout Configuration"))
	fmt.Println()
	
	fmt.Printf("  %s: %s\n", accentStyle.Render("Default Command"), normalStyle.Render(cfg.DefaultCommand))
	
	if cfg.GetLinearAPIKey() != "" {
		fmt.Printf("  %s: %s\n", accentStyle.Render("Linear API Key"), normalStyle.Render("configured"))
	} else {
		fmt.Printf("  %s: %s\n", accentStyle.Render("Linear API Key"), warningStyle.Render("not configured"))
	}
	
	configPath, err := getConfigPath()
	if err != nil {
		fmt.Printf("  %s: %s\n", accentStyle.Render("Config Path"), warningStyle.Render(fmt.Sprintf("<error: %v>", err)))
	} else {
		fmt.Printf("  %s: %s\n", accentStyle.Render("Config Path"), normalStyle.Render(configPath))
		
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			fmt.Printf("  %s: %s\n", accentStyle.Render("Config File"), warningStyle.Render("not found (using defaults)"))
		} else {
			fmt.Printf("  %s: %s\n", accentStyle.Render("Config File"), normalStyle.Render("exists"))
		}
	}
	
	fmt.Println()
	fmt.Println(headerStyle.Render("Sparse Checkout"))
	fmt.Println()
	
	if len(cfg.SparseCheckout) == 0 {
		fmt.Printf("  %s: %s\n", accentStyle.Render("Configured Repositories"), normalStyle.Render("none"))
	} else {
		fmt.Printf("  %s: %s\n", accentStyle.Render("Configured Repositories"), normalStyle.Render(fmt.Sprintf("%d", len(cfg.SparseCheckout))))
		for repoPath, directories := range cfg.SparseCheckout {
			fmt.Printf("    %s: %s\n", normalStyle.Render(repoPath), normalStyle.Render(fmt.Sprintf("[%s]", strings.Join(directories, ", "))))
		}
	}
	
	fmt.Println()
	fmt.Println(headerStyle.Render("Linear Integration"))
	fmt.Println()
	
	if cfg.LinearAPIKey == "" {
		fmt.Printf("  %s: %s\n", accentStyle.Render("API Key"), warningStyle.Render("not configured"))
		fmt.Printf("  %s: %s\n", accentStyle.Render("Status"), warningStyle.Render("disabled"))
	} else {
		// Mask the key for security
		maskedKey := cfg.LinearAPIKey
		if len(maskedKey) > 8 {
			maskedKey = maskedKey[:8] + "..." + maskedKey[len(maskedKey)-4:]
		}
		fmt.Printf("  %s: %s\n", accentStyle.Render("API Key"), normalStyle.Render(maskedKey))
		
		fmt.Printf("  %s: ", accentStyle.Render("Status"))
		testLinearConnection(cfg.LinearAPIKey)
	}
}

func getConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/.sprout.json5", homeDir), nil
}

func testLinearConnection(apiKey string) {
	successStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("108")).
		Bold(true)
	
	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("204")).
		Bold(true)
	
	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	client := linear.NewClient(apiKey)
	
	user, err := client.GetCurrentUser()
	if err != nil {
		fmt.Printf("%s\n", errorStyle.Render("✗ Failed"))
		fmt.Printf("  %s\n", errorStyle.Render(fmt.Sprintf("Error: %v", err)))
		return
	}
	
	fmt.Printf("%s\n", successStyle.Render("✓ Connected"))
	fmt.Printf("  %s: %s\n", normalStyle.Render("User"), normalStyle.Render(fmt.Sprintf("%s (%s)", user.Name, user.Email)))
	
	// Try to fetch assigned issues
	issues, err := client.GetAssignedIssues()
	if err != nil {
		fmt.Printf("  %s: %s\n", normalStyle.Render("Assigned Issues"), errorStyle.Render(fmt.Sprintf("<error fetching: %v>", err)))
	} else {
		fmt.Printf("  %s: %s\n", normalStyle.Render("Assigned Issues"), normalStyle.Render(fmt.Sprintf("%d active tickets", len(issues))))
	}
}

func printHelp() {
	fmt.Println("Sprout - Git Worktree Terminal UI")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  sprout                              Start in interactive mode")
	fmt.Println("  sprout list                         List all worktrees")
	fmt.Println("  sprout create <branch>              Create worktree and output path")
	fmt.Println("  sprout create <branch> <command>    Create worktree and run command in it")
	fmt.Println("  sprout prune [branch]               Remove worktree(s) - all merged if no branch specified")
	fmt.Println("  sprout doctor                       Show configuration values")
	fmt.Println("  sprout help                         Show this help")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  sprout list                          # Show all worktrees")
	fmt.Println("  cd \"$(sprout create mybranch)\"       # Change to worktree directory")
	fmt.Println("  sprout create mybranch bash          # Create worktree and start bash")
	fmt.Println("  sprout create mybranch code .        # Create worktree and open in VS Code")
	fmt.Println("  sprout create mybranch git status    # Create worktree and run git status")
	fmt.Println("  sprout prune                         # Remove all merged worktrees")
	fmt.Println("  sprout prune mybranch                # Remove specific worktree and directory")
}