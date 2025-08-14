package worktree

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// Worktree represents a git worktree with associated metadata
type Worktree struct {
	ID           string
	Path         string
	Branch       string
	Commit       string
	Status       Status
	CreatedAt    time.Time
	LastAccessed time.Time
}

// Status represents the current state of a worktree
type Status string

const (
	StatusActive  Status = "active"
	StatusMerged  Status = "merged"
	StatusClosed  Status = "closed"
	StatusUnknown Status = "unknown"
)

// NewWorktree creates a new worktree instance
func NewWorktree(path, branch, commit string) *Worktree {
	return &Worktree{
		ID:           generateID(path, branch),
		Path:         path,
		Branch:       branch,
		Commit:       commit,
		Status:       StatusActive,
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
	}
}

// IsMainBranch returns true if this worktree represents a main branch
func (w *Worktree) IsMainBranch() bool {
	return w.Branch == "main" || w.Branch == "master"
}

// ShortCommit returns the first 8 characters of the commit hash
func (w *Worktree) ShortCommit() string {
	if len(w.Commit) > 8 {
		return w.Commit[:8]
	}
	return w.Commit
}

// DirectoryName returns the directory name of the worktree path
func (w *Worktree) DirectoryName() string {
	return filepath.Base(w.Path)
}

// generateID creates a unique ID for a worktree based on path and branch
func generateID(path, branch string) string {
	return fmt.Sprintf("%s-%s", filepath.Base(path), sanitizeBranchName(branch))
}

// sanitizeBranchName ensures branch names are safe for use as identifiers
func sanitizeBranchName(name string) string {
	if name == "" {
		return "unknown"
	}
	
	// Convert to lowercase and replace problematic characters
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")
	
	// Remove special characters
	var result strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	
	return strings.Trim(result.String(), "-")
}