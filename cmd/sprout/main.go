package main

import (
	"fmt"
	"os"
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
		fmt.Println("Creating worktree...")
		// TODO: Handle worktree creation
	case "help", "--help", "-h":
		printHelp()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println("Sprout - Git Worktree Terminal UI")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  sprout                    Start in interactive mode")
	fmt.Println("  sprout create [branch]    Create new worktree")
	fmt.Println("  sprout help               Show this help")
}