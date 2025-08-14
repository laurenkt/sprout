package project

import (
	"context"
	"errors"
)

var (
	ErrNotInRepository = errors.New("not in a git repository")
	ErrInvalidProject  = errors.New("invalid project configuration")
)

// Repository defines the interface for project information retrieval
type Repository interface {
	// GetCurrent returns information about the current project
	GetCurrent(ctx context.Context) (*Project, error)
	
	// GetMainBranch determines the main branch name (main/master)
	GetMainBranch(ctx context.Context) (string, error)
	
	// HasRemoteOrigin checks if the project has a remote origin
	HasRemoteOrigin(ctx context.Context) (bool, error)
	
	// IsClean returns true if the working directory has no uncommitted changes
	IsClean(ctx context.Context) (bool, error)
}

// Service defines the business logic interface for project operations
type Service interface {
	// GetCurrentProject returns the current project with all metadata
	GetCurrentProject(ctx context.Context) (*Project, error)
	
	// ValidateProject ensures the current directory is a valid git project
	ValidateProject(ctx context.Context) error
	
	// GetProjectInfo returns formatted project information for display
	GetProjectInfo(ctx context.Context) (ProjectInfo, error)
}

// ProjectInfo contains formatted project information for display
type ProjectInfo struct {
	Name         string
	Path         string
	MainBranch   string
	RemoteURL    string
	IsGitHub     bool
	Owner        string
	Repository   string
	WorktreeDir  string
	HasChanges   bool
}