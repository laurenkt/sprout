package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	
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
		if err := handlePruneCommand(); err != nil {
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
	
	fmt.Printf("%-20s %-10s %s\n", "BRANCH", "PR STATUS", "COMMIT")
	fmt.Println(strings.Repeat("-", 40))
	
	for _, wt := range filteredWorktrees {
		commit := wt.Commit
		if len(commit) > 8 {
			commit = commit[:8]
		}
		fmt.Printf("%-20s %-10s %s\n", wt.Branch, wt.PRStatus, commit)
	}
	
	return nil
}

func handlePruneCommand() error {
	wm, err := git.NewWorktreeManager()
	if err != nil {
		return err
	}
	
	worktrees, err := wm.ListWorktrees()
	if err != nil {
		return err
	}
	
	var mergedWorktrees []git.Worktree
	for _, wt := range worktrees {
		// Filter out main/master branches and only include merged PRs
		if wt.Branch != "master" && wt.Branch != "main" && wt.Branch != "" && wt.PRStatus == "Merged" {
			mergedWorktrees = append(mergedWorktrees, wt)
		}
	}
	
	if len(mergedWorktrees) == 0 {
		fmt.Println("No worktrees with merged PRs found")
		return nil
	}
	
	fmt.Printf("%-20s %-10s %s\n", "BRANCH", "PR STATUS", "COMMIT")
	fmt.Println(strings.Repeat("-", 40))
	
	for _, wt := range mergedWorktrees {
		commit := wt.Commit
		if len(commit) > 8 {
			commit = commit[:8]
		}
		fmt.Printf("%-20s %-10s %s\n", wt.Branch, wt.PRStatus, commit)
	}
	
	fmt.Printf("\n%d worktree(s) ready to be pruned\n", len(mergedWorktrees))
	
	return nil
}

func handleDoctorCommand(cfg *config.Config) {
	fmt.Println("Sprout Configuration")
	fmt.Println("===================")
	fmt.Printf("Default Command: %s\n", cfg.DefaultCommand)
	
	if cfg.GetLinearAPIKey() != "" {
		fmt.Printf("Linear API Key: configured\n")
	} else {
		fmt.Printf("Linear API Key: not configured\n")
	}
	
	configPath, err := getConfigPath()
	if err != nil {
		fmt.Printf("Config Path: <error: %v>\n", err)
	} else {
		fmt.Printf("Config Path: %s\n", configPath)
		
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			fmt.Println("Config File: not found (using defaults)")
		} else {
			fmt.Println("Config File: exists")
		}
	}
	
	// Linear connectivity test
	fmt.Println()
	fmt.Println("Linear Integration")
	fmt.Println("=================")
	
	if cfg.LinearAPIKey == "" {
		fmt.Println("Linear API Key: not configured")
		fmt.Println("Linear Status: disabled")
	} else {
		// Mask the key for security
		maskedKey := cfg.LinearAPIKey
		if len(maskedKey) > 8 {
			maskedKey = maskedKey[:8] + "..." + maskedKey[len(maskedKey)-4:]
		}
		fmt.Printf("Linear API Key: %s\n", maskedKey)
		
		fmt.Print("Linear Status: testing connection... ")
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
	client := linear.NewClient(apiKey)
	
	user, err := client.GetCurrentUser()
	if err != nil {
		fmt.Printf("❌ Failed\n")
		fmt.Printf("Error: %v\n", err)
		return
	}
	
	fmt.Printf("✅ Connected\n")
	fmt.Printf("Linear User: %s (%s)\n", user.Name, user.Email)
	
	// Try to fetch assigned issues
	issues, err := client.GetAssignedIssues()
	if err != nil {
		fmt.Printf("Assigned Issues: <error fetching: %v>\n", err)
	} else {
		fmt.Printf("Assigned Issues: %d active tickets\n", len(issues))
	}
}

func printHelp() {
	fmt.Println("Sprout - Git Worktree Terminal UI")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  sprout                              Start in interactive mode")
	fmt.Println("  sprout list                         List all worktrees")
	fmt.Println("  sprout prune                        List worktrees with merged PRs")
	fmt.Println("  sprout create <branch>              Create worktree and output path")
	fmt.Println("  sprout create <branch> <command>    Create worktree and run command in it")
	fmt.Println("  sprout doctor                       Show configuration values")
	fmt.Println("  sprout help                         Show this help")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  sprout list                          # Show all worktrees")
	fmt.Println("  cd \"$(sprout create mybranch)\"       # Change to worktree directory")
	fmt.Println("  sprout create mybranch bash          # Create worktree and start bash")
	fmt.Println("  sprout create mybranch code .        # Create worktree and open in VS Code")
	fmt.Println("  sprout create mybranch git status    # Create worktree and run git status")
}