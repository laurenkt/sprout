package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	
	"sprout/pkg/config"
	"sprout/pkg/git"
	"sprout/pkg/ui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Warning: Failed to load config: %v\n", err)
		cfg = config.DefaultConfig()
	}

	if len(os.Args) < 2 {
		// Check if there's a default command configured
		defaultCmd := cfg.GetDefaultCommand()
		if len(defaultCmd) > 0 {
			// Execute the default command
			cmd := exec.Command(defaultCmd[0], defaultCmd[1:]...)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			
			if err := cmd.Run(); err != nil {
				if exitError, ok := err.(*exec.ExitError); ok {
					if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
						os.Exit(status.ExitStatus())
					}
				}
				fmt.Printf("Default command failed: %v\n", err)
				os.Exit(1)
			}
			return
		}
		
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
	
	// If no command provided, output path for shell evaluation
	if len(args) == 1 {
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
				os.Exit(status.ExitStatus())
			}
		}
		return fmt.Errorf("command failed: %w", err)
	}
	
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

func printHelp() {
	fmt.Println("Sprout - Git Worktree Terminal UI")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  sprout                              Start in interactive mode")
	fmt.Println("  sprout list                         List all worktrees")
	fmt.Println("  sprout create <branch>              Create worktree and output path")
	fmt.Println("  sprout create <branch> <command>    Create worktree and run command in it")
	fmt.Println("  sprout help                         Show this help")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  sprout list                          # Show all worktrees")
	fmt.Println("  cd \"$(sprout create mybranch)\"       # Change to worktree directory")
	fmt.Println("  sprout create mybranch bash          # Create worktree and start bash")
	fmt.Println("  sprout create mybranch code .        # Create worktree and open in VS Code")
	fmt.Println("  sprout create mybranch git status    # Create worktree and run git status")
}