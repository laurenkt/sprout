package worktree

import (
	"context"
	"errors"
)

var (
	ErrWorktreeNotFound    = errors.New("worktree not found")
	ErrWorktreeExists      = errors.New("worktree already exists")
	ErrInvalidBranchName   = errors.New("invalid branch name")
	ErrMainBranchRequired  = errors.New("main branch not found")
)

// Repository defines the interface for worktree persistence and retrieval
type Repository interface {
	// Create creates a new worktree with the specified branch name
	Create(ctx context.Context, branchName string) (*Worktree, error)
	
	// GetByID retrieves a worktree by its ID
	GetByID(ctx context.Context, id string) (*Worktree, error)
	
	// GetByBranch retrieves a worktree by its branch name
	GetByBranch(ctx context.Context, branch string) (*Worktree, error)
	
	// List returns all worktrees, optionally filtered
	List(ctx context.Context, filter ListFilter) ([]*Worktree, error)
	
	// Update updates an existing worktree
	Update(ctx context.Context, worktree *Worktree) error
	
	// Delete removes a worktree
	Delete(ctx context.Context, id string) error
	
	// DeleteMerged removes all worktrees with merged status
	DeleteMerged(ctx context.Context) ([]string, error)
}

// ListFilter provides filtering options for listing worktrees
type ListFilter struct {
	ExcludeMainBranches bool
	Status              *Status
	BranchPattern       string
}

// Service defines the business logic interface for worktree operations
type Service interface {
	// CreateWorktree creates a new worktree and returns its path
	CreateWorktree(ctx context.Context, branchName string) (string, error)
	
	// ListWorktrees returns all worktrees with their current status
	ListWorktrees(ctx context.Context, includeMain bool) ([]*Worktree, error)
	
	// PruneWorktree removes a specific worktree
	PruneWorktree(ctx context.Context, branchName string) error
	
	// PruneAllMerged removes all merged worktrees
	PruneAllMerged(ctx context.Context) ([]string, error)
	
	// GetWorktreeStatus gets the current status of a worktree
	GetWorktreeStatus(ctx context.Context, branchName string) (Status, error)
}