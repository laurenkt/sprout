package handlers

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"

	"sprout/pkg/application/services"
	"sprout/pkg/domain/worktree"
	"sprout/pkg/infrastructure/config"
	"sprout/pkg/shared/errors"
	"sprout/pkg/shared/logging"
)

// CommandHandler handles CLI command execution
type CommandHandler struct {
	worktreeService *services.WorktreeService
	issueService    *services.IssueService
	projectService  *services.ProjectService
	configRepo      config.Repository
	logger          logging.Logger
}

// NewCommandHandler creates a new command handler
func NewCommandHandler(
	worktreeService *services.WorktreeService,
	issueService *services.IssueService,
	projectService *services.ProjectService,
	configRepo config.Repository,
	logger logging.Logger,
) *CommandHandler {
	return &CommandHandler{
		worktreeService: worktreeService,
		issueService:    issueService,
		projectService:  projectService,
		configRepo:      configRepo,
		logger:          logger,
	}
}

// CreateWorktreeCommand represents a command to create a worktree
type CreateWorktreeCommand struct {
	BranchName string
	Command    []string
	Output     io.Writer
	ErrorOutput io.Writer
}

// HandleCreateWorktree handles the create worktree command
func (h *CommandHandler) HandleCreateWorktree(ctx context.Context, cmd CreateWorktreeCommand) error {
	if cmd.BranchName == "" {
		return errors.ValidationError("branch name is required")
	}

	h.logger.Info("handling create worktree command", "branch", cmd.BranchName)

	// Validate worktree creation
	if err := h.worktreeService.ValidateWorktreeCreation(ctx, cmd.BranchName); err != nil {
		return err
	}

	// Create the worktree
	worktreePath, err := h.worktreeService.CreateWorktree(ctx, cmd.BranchName)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.ErrorOutput, "Worktree ready at: %s\n", worktreePath)

	// Handle command execution
	if len(cmd.Command) == 0 {
		// No command provided, check for default command
		return h.executeDefaultCommand(ctx, worktreePath, cmd)
	}

	// Execute the provided command
	return h.executeCommand(ctx, worktreePath, cmd.Command, cmd)
}

// ListWorktreesCommand represents a command to list worktrees
type ListWorktreesCommand struct {
	IncludeMain bool
	Output      io.Writer
}

// HandleListWorktrees handles the list worktrees command
func (h *CommandHandler) HandleListWorktrees(ctx context.Context, cmd ListWorktreesCommand) error {
	h.logger.Info("handling list worktrees command", "include_main", cmd.IncludeMain)

	worktrees, err := h.worktreeService.ListWorktrees(ctx, cmd.IncludeMain)
	if err != nil {
		return err
	}

	if len(worktrees) == 0 {
		fmt.Fprintln(cmd.Output, "No worktrees found")
		return nil
	}

	// Format and display the worktrees
	return h.formatWorktreeList(worktrees, cmd.Output)
}

// PruneWorktreeCommand represents a command to prune worktrees
type PruneWorktreeCommand struct {
	BranchName string // Empty means prune all merged
	Output     io.Writer
}

// HandlePruneWorktree handles the prune worktree command
func (h *CommandHandler) HandlePruneWorktree(ctx context.Context, cmd PruneWorktreeCommand) error {
	if cmd.BranchName == "" {
		// Prune all merged worktrees
		h.logger.Info("handling prune all merged worktrees command")
		
		deletedBranches, err := h.worktreeService.PruneAllMerged(ctx)
		if err != nil {
			return err
		}

		if len(deletedBranches) == 0 {
			fmt.Fprintln(cmd.Output, "No merged worktrees found to prune")
		} else {
			fmt.Fprintf(cmd.Output, "Pruned %d merged worktree(s): %v\n", len(deletedBranches), deletedBranches)
		}
		
		return nil
	}

	// Prune specific worktree
	h.logger.Info("handling prune specific worktree command", "branch", cmd.BranchName)
	
	err := h.worktreeService.PruneWorktree(ctx, cmd.BranchName)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.Output, "Worktree '%s' has been pruned successfully\n", cmd.BranchName)
	return nil
}

// DoctorCommand represents a command to show configuration and status
type DoctorCommand struct {
	Output io.Writer
}

// HandleDoctor handles the doctor command
func (h *CommandHandler) HandleDoctor(ctx context.Context, cmd DoctorCommand) error {
	h.logger.Info("handling doctor command")

	// Load configuration
	cfg, err := h.configRepo.Load()
	if err != nil {
		return err
	}

	// Get project information
	projectInfo, err := h.projectService.GetProjectInfo(ctx)
	if err != nil {
		return err
	}

	// Display project information
	fmt.Fprintln(cmd.Output, "🌱 Sprout Configuration")
	fmt.Fprintln(cmd.Output)
	
	// Project info
	fmt.Fprintf(cmd.Output, "Project: %s\n", projectInfo.Name)
	fmt.Fprintf(cmd.Output, "Path: %s\n", projectInfo.Path)
	fmt.Fprintf(cmd.Output, "Main Branch: %s\n", projectInfo.MainBranch)
	fmt.Fprintf(cmd.Output, "Worktree Directory: %s\n", projectInfo.WorktreeDir)
	
	if projectInfo.IsGitHub {
		fmt.Fprintf(cmd.Output, "GitHub Repository: %s/%s\n", projectInfo.Owner, projectInfo.Repository)
	}
	
	fmt.Fprintln(cmd.Output)

	// Configuration
	fmt.Fprintln(cmd.Output, "Configuration:")
	
	defaultCmd := cfg.DefaultCommand
	if defaultCmd == "" {
		defaultCmd = "not configured"
	}
	fmt.Fprintf(cmd.Output, "  Default Command: %s\n", defaultCmd)
	
	// Linear integration status
	if cfg.IsLinearConfigured() {
		fmt.Fprintf(cmd.Output, "  Linear API Key: configured\n")
		
		// Test Linear connection
		if h.issueService.IsConfigured() {
			fmt.Fprintf(cmd.Output, "  Linear Status: ")
			if err := h.issueService.TestConnection(ctx); err != nil {
				fmt.Fprintf(cmd.Output, "✗ Failed (%v)\n", err)
			} else {
				fmt.Fprintf(cmd.Output, "✓ Connected\n")
				
				// Get user info
				if user, err := h.issueService.GetCurrentUser(ctx); err == nil {
					fmt.Fprintf(cmd.Output, "  Linear User: %s (%s)\n", user.Name, user.Email)
				}
			}
		}
	} else {
		fmt.Fprintf(cmd.Output, "  Linear API Key: not configured\n")
		fmt.Fprintf(cmd.Output, "  Linear Status: disabled\n")
	}

	return nil
}

// executeDefaultCommand executes the configured default command
func (h *CommandHandler) executeDefaultCommand(ctx context.Context, worktreePath string, cmd CreateWorktreeCommand) error {
	cfg, err := h.configRepo.Load()
	if err != nil {
		return errors.ConfigurationError("failed to load config").WithCause(err)
	}

	defaultCmd := cfg.GetDefaultCommand()
	if len(defaultCmd) == 0 {
		// No default command, output path for shell evaluation
		fmt.Fprint(cmd.Output, worktreePath)
		return nil
	}

	// Execute the default command
	return h.executeCommand(ctx, worktreePath, defaultCmd, cmd)
}

// executeCommand executes a command in the worktree directory
func (h *CommandHandler) executeCommand(ctx context.Context, worktreePath string, command []string, cmd CreateWorktreeCommand) error {
	if len(command) == 0 {
		return errors.ValidationError("command cannot be empty")
	}

	execCmd := exec.CommandContext(ctx, command[0], command[1:]...)
	execCmd.Dir = worktreePath
	execCmd.Stdin = os.Stdin
	execCmd.Stdout = cmd.Output
	execCmd.Stderr = cmd.ErrorOutput

	if err := execCmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				fmt.Fprintf(cmd.ErrorOutput, "\nWorktree directory: %s\n", worktreePath)
				os.Exit(status.ExitStatus())
			}
		}
		return errors.ExternalError("command execution failed", err)
	}

	fmt.Fprintf(cmd.ErrorOutput, "\nWorktree directory: %s\n", worktreePath)
	return nil
}

// formatWorktreeList formats and displays the list of worktrees
func (h *CommandHandler) formatWorktreeList(worktrees []*worktree.Worktree, output io.Writer) error {
	// For now, use a simple format. This could be enhanced with table formatting
	fmt.Fprintf(output, "%-20s %-15s %-10s\n", "BRANCH", "STATUS", "COMMIT")
	fmt.Fprintf(output, "%-20s %-15s %-10s\n", "------", "------", "------")
	
	for _, wt := range worktrees {
		commit := wt.ShortCommit()
		if commit == "" {
			commit = "unknown"
		}
		
		status := string(wt.Status)
		if status == "" {
			status = "unknown"
		}
		
		fmt.Fprintf(output, "%-20s %-15s %-10s\n", wt.Branch, status, commit)
	}
	
	return nil
}