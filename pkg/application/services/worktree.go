package services

import (
	"context"
	"fmt"

	"sprout/pkg/domain/worktree"
	"sprout/pkg/shared/errors"
	"sprout/pkg/shared/events"
	"sprout/pkg/shared/logging"
)

// WorktreeService implements worktree.Service
type WorktreeService struct {
	repo     worktree.Repository
	eventBus events.Bus
	logger   logging.Logger
}

// NewWorktreeService creates a new worktree service
func NewWorktreeService(repo worktree.Repository, eventBus events.Bus, logger logging.Logger) *WorktreeService {
	return &WorktreeService{
		repo:     repo,
		eventBus: eventBus,
		logger:   logger,
	}
}

// CreateWorktree creates a new worktree and returns its path
func (s *WorktreeService) CreateWorktree(ctx context.Context, branchName string) (string, error) {
	if branchName == "" {
		return "", errors.ValidationError("branch name cannot be empty")
	}

	s.logger.Info("creating worktree", "branch", branchName)

	wt, err := s.repo.Create(ctx, branchName)
	if err != nil {
		s.logger.Error("failed to create worktree", "branch", branchName, "error", err)
		return "", err
	}

	s.logger.Info("worktree created successfully", "branch", wt.Branch, "path", wt.Path)

	// Publish event
	s.eventBus.Publish(ctx, events.NewEvent(events.WorktreeCreated, map[string]interface{}{
		"worktree_id": wt.ID,
		"branch":      wt.Branch,
		"path":        wt.Path,
	}))

	return wt.Path, nil
}

// ListWorktrees returns all worktrees with their current status
func (s *WorktreeService) ListWorktrees(ctx context.Context, includeMain bool) ([]*worktree.Worktree, error) {
	filter := worktree.ListFilter{
		ExcludeMainBranches: !includeMain,
	}

	worktrees, err := s.repo.List(ctx, filter)
	if err != nil {
		s.logger.Error("failed to list worktrees", "error", err)
		return nil, err
	}

	s.logger.Debug("listed worktrees", "count", len(worktrees), "include_main", includeMain)

	return worktrees, nil
}

// PruneWorktree removes a specific worktree
func (s *WorktreeService) PruneWorktree(ctx context.Context, branchName string) error {
	if branchName == "" {
		return errors.ValidationError("branch name cannot be empty")
	}

	s.logger.Info("pruning worktree", "branch", branchName)

	// Get the worktree first to have its ID for the event
	wt, err := s.repo.GetByBranch(ctx, branchName)
	if err != nil {
		return err
	}

	err = s.repo.Delete(ctx, wt.ID)
	if err != nil {
		s.logger.Error("failed to prune worktree", "branch", branchName, "error", err)
		return err
	}

	s.logger.Info("worktree pruned successfully", "branch", branchName)

	// Publish event
	s.eventBus.Publish(ctx, events.NewEvent(events.WorktreeDeleted, map[string]interface{}{
		"worktree_id": wt.ID,
		"branch":      wt.Branch,
		"path":        wt.Path,
	}))

	return nil
}

// PruneAllMerged removes all merged worktrees
func (s *WorktreeService) PruneAllMerged(ctx context.Context) ([]string, error) {
	s.logger.Info("pruning all merged worktrees")

	deletedBranches, err := s.repo.DeleteMerged(ctx)
	if err != nil {
		s.logger.Error("failed to prune merged worktrees", "error", err)
		return deletedBranches, err
	}

	s.logger.Info("pruned merged worktrees", "count", len(deletedBranches), "branches", deletedBranches)

	// Publish events for each deleted worktree
	for _, branch := range deletedBranches {
		s.eventBus.Publish(ctx, events.NewEvent(events.WorktreeDeleted, map[string]interface{}{
			"branch": branch,
			"reason": "merged",
		}))
	}

	return deletedBranches, nil
}

// GetWorktreeStatus gets the current status of a worktree
func (s *WorktreeService) GetWorktreeStatus(ctx context.Context, branchName string) (worktree.Status, error) {
	wt, err := s.repo.GetByBranch(ctx, branchName)
	if err != nil {
		return worktree.StatusUnknown, err
	}

	return wt.Status, nil
}

// ValidateWorktreeCreation validates that a worktree can be created with the given name
func (s *WorktreeService) ValidateWorktreeCreation(ctx context.Context, branchName string) error {
	if branchName == "" {
		return errors.ValidationError("branch name cannot be empty")
	}

	// Check for reserved names
	reservedNames := []string{"main", "master", "HEAD", "origin"}
	for _, reserved := range reservedNames {
		if branchName == reserved {
			return errors.ValidationError(fmt.Sprintf("branch name '%s' is reserved", branchName))
		}
	}

	// Check if worktree already exists
	_, err := s.repo.GetByBranch(ctx, branchName)
	if err == nil {
		return errors.ConflictError(fmt.Sprintf("worktree with branch '%s' already exists", branchName))
	}
	if !errors.IsType(err, errors.ErrorTypeNotFound) {
		return err // Some other error occurred
	}

	return nil
}