package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	
	"sprout/pkg/git"
)

func main() {
	if len(os.Args) < 2 {
		// Interactive mode
		fmt.Println("Starting Sprout in interactive mode...")
		// TODO: Launch TUI
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

func printHelp() {
	fmt.Println("Sprout - Git Worktree Terminal UI")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  sprout                              Start in interactive mode")
	fmt.Println("  sprout create <branch>              Create worktree and output path")
	fmt.Println("  sprout create <branch> <command>    Create worktree and run command in it")
	fmt.Println("  sprout help                         Show this help")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  cd \"$(sprout create mybranch)\"       # Change to worktree directory")
	fmt.Println("  sprout create mybranch bash          # Create worktree and start bash")
	fmt.Println("  sprout create mybranch code .        # Create worktree and open in VS Code")
	fmt.Println("  sprout create mybranch git status    # Create worktree and run git status")
}