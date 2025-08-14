package cli

import (
	"context"
	"fmt"
	"os"

	"sprout/pkg/application/handlers"
	"sprout/pkg/application/services"
	"sprout/pkg/infrastructure/container"
	"sprout/pkg/presentation/tui"
	"sprout/pkg/shared/errors"
)

// App represents the CLI application
type App struct {
	container      *container.Container
	commandHandler *handlers.CommandHandler
}

// New creates a new CLI application
func New(ctx context.Context) (*App, error) {
	// Initialize dependency container
	cont, err := container.New(ctx)
	if err != nil {
		return nil, errors.InternalError("failed to initialize dependency container", err)
	}

	// Create services
	var worktreeService *services.WorktreeService
	var issueService *services.IssueService
	var projectService *services.ProjectService

	worktreeService = services.NewWorktreeService(
		cont.WorktreeRepo(),
		cont.EventBus(),
		cont.Logger(),
	)

	if cont.HasIssueProvider() {
		issueService = services.NewIssueService(
			cont.IssueRepo(),
			cont.EventBus(),
			cont.Logger(),
		)
	}

	projectService = services.NewProjectService(
		cont.ProjectRepo(),
		cont.Logger(),
	)

	// Create command handler
	commandHandler := handlers.NewCommandHandler(
		worktreeService,
		issueService,
		projectService,
		cont.Config(),
		cont.Logger(),
	)

	return &App{
		container:      cont,
		commandHandler: commandHandler,
	}, nil
}

// Run executes the CLI application with the given arguments
func (a *App) Run(ctx context.Context, args []string) error {
	defer a.container.Close()

	if len(args) < 2 {
		// Interactive mode
		return a.runInteractive(ctx)
	}

	// One-shot command mode
	command := args[1]
	commandArgs := args[2:]

	switch command {
	case "create":
		return a.handleCreateCommand(ctx, commandArgs)
	case "list":
		return a.handleListCommand(ctx, commandArgs)
	case "prune":
		return a.handlePruneCommand(ctx, commandArgs)
	case "doctor":
		return a.handleDoctorCommand(ctx, commandArgs)
	case "help", "--help", "-h":
		a.showHelp()
		return nil
	default:
		return errors.ValidationError(fmt.Sprintf("unknown command: %s", command))
	}
}

// runInteractive starts the interactive TUI mode
func (a *App) runInteractive(ctx context.Context) error {
	a.container.Logger().Info("starting interactive mode")

	// Create TUI dependencies
	var issueService *services.IssueService
	if a.container.HasIssueProvider() {
		issueService = services.NewIssueService(
			a.container.IssueRepo(),
			a.container.EventBus(),
			a.container.Logger(),
		)
	}

	worktreeService := services.NewWorktreeService(
		a.container.WorktreeRepo(),
		a.container.EventBus(),
		a.container.Logger(),
	)

	// Start TUI
	tuiApp, err := tui.NewApp(
		worktreeService,
		issueService,
		a.container.CurrentProject(),
		a.container.AppConfig(),
		a.container.Logger(),
	)
	if err != nil {
		return errors.InternalError("failed to create TUI application", err)
	}

	return tuiApp.Run(ctx)
}

// handleCreateCommand handles the create worktree command
func (a *App) handleCreateCommand(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.ValidationError("branch name is required. Usage: sprout create <branch-name> [command...]")
	}

	cmd := handlers.CreateWorktreeCommand{
		BranchName:  args[0],
		Command:     args[1:],
		Output:      os.Stdout,
		ErrorOutput: os.Stderr,
	}

	return a.commandHandler.HandleCreateWorktree(ctx, cmd)
}

// handleListCommand handles the list worktrees command
func (a *App) handleListCommand(ctx context.Context, args []string) error {
	includeMain := false
	
	// Parse flags (simple implementation)
	for _, arg := range args {
		if arg == "--include-main" || arg == "-m" {
			includeMain = true
		}
	}

	cmd := handlers.ListWorktreesCommand{
		IncludeMain: includeMain,
		Output:      os.Stdout,
	}

	return a.commandHandler.HandleListWorktrees(ctx, cmd)
}

// handlePruneCommand handles the prune worktree command
func (a *App) handlePruneCommand(ctx context.Context, args []string) error {
	var branchName string
	if len(args) > 0 {
		branchName = args[0]
	}

	cmd := handlers.PruneWorktreeCommand{
		BranchName: branchName,
		Output:     os.Stdout,
	}

	return a.commandHandler.HandlePruneWorktree(ctx, cmd)
}

// handleDoctorCommand handles the doctor command
func (a *App) handleDoctorCommand(ctx context.Context, args []string) error {
	cmd := handlers.DoctorCommand{
		Output: os.Stdout,
	}

	return a.commandHandler.HandleDoctor(ctx, cmd)
}

// showHelp displays the help message
func (a *App) showHelp() {
	fmt.Println("Sprout - Git Worktree Terminal UI")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  sprout                              Start in interactive mode")
	fmt.Println("  sprout list [--include-main]        List all worktrees")
	fmt.Println("  sprout create <branch> [command]    Create worktree and optionally run command")
	fmt.Println("  sprout prune [branch]               Remove worktree(s) - all merged if no branch specified")
	fmt.Println("  sprout doctor                       Show configuration and status")
	fmt.Println("  sprout help                         Show this help")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  sprout                              # Start interactive mode")
	fmt.Println("  sprout list                         # Show all worktrees")
	fmt.Println("  cd \"$(sprout create mybranch)\"       # Change to worktree directory")
	fmt.Println("  sprout create mybranch code .       # Create worktree and open in VS Code")
	fmt.Println("  sprout prune                        # Remove all merged worktrees")
	fmt.Println("  sprout prune mybranch               # Remove specific worktree")
}

// Run is the main entry point for the CLI
func Run(args []string) int {
	ctx := context.Background()
	
	app, err := New(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to initialize application: %v\n", err)
		return 1
	}
	
	if err := app.Run(ctx, args); err != nil {
		// Check error type for appropriate exit codes
		switch {
		case errors.IsType(err, errors.ErrorTypeValidation):
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 2
		case errors.IsType(err, errors.ErrorTypeNotFound):
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 3
		case errors.IsType(err, errors.ErrorTypeConfiguration):
			fmt.Fprintf(os.Stderr, "Configuration Error: %v\n", err)
			return 4
		case errors.IsType(err, errors.ErrorTypeExternal):
			fmt.Fprintf(os.Stderr, "External Error: %v\n", err)
			return 5
		default:
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
	}
	
	return 0
}