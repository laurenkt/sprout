package issue

import (
	"context"
	"errors"
)

var (
	ErrIssueNotFound = errors.New("issue not found")
	ErrInvalidIssue  = errors.New("invalid issue data")
	ErrNoProvider    = errors.New("no issue provider configured")
)

// Repository defines the interface for issue persistence and retrieval
type Repository interface {
	// GetAssignedIssues returns issues assigned to the current user
	GetAssignedIssues(ctx context.Context) ([]*Issue, error)
	
	// GetIssueByID retrieves a specific issue by its ID
	GetIssueByID(ctx context.Context, id string) (*Issue, error)
	
	// GetIssueChildren retrieves child issues for a parent issue
	GetIssueChildren(ctx context.Context, parentID string) ([]*Issue, error)
	
	// CreateSubtask creates a new subtask under a parent issue
	CreateSubtask(ctx context.Context, parentID, title string) (*Issue, error)
	
	// SearchIssues searches for issues matching a query
	SearchIssues(ctx context.Context, query string) ([]*Issue, error)
	
	// TestConnection validates the connection to the issue provider
	TestConnection(ctx context.Context) error
}

// Service defines the business logic interface for issue operations
type Service interface {
	// GetAssignedIssues returns issues assigned to the current user with UI state
	GetAssignedIssues(ctx context.Context) ([]*Issue, error)
	
	// ExpandIssue loads children for an issue and marks it as expanded
	ExpandIssue(ctx context.Context, issueID string) ([]*Issue, error)
	
	// CreateSubtask creates a new subtask under a parent issue
	CreateSubtask(ctx context.Context, parentID, title string) (*Issue, error)
	
	// SearchIssues performs fuzzy search on issues
	SearchIssues(ctx context.Context, query string) ([]*Issue, error)
	
	// GetCurrentUser returns information about the authenticated user
	GetCurrentUser(ctx context.Context) (*User, error)
	
	// IsConfigured returns true if an issue provider is configured
	IsConfigured() bool
}