package git

import (
	"fmt"
	"path/filepath"
)

// MockWorktreeManager is a mock implementation for testing
type MockWorktreeManager struct {
	repoRoot  string
	worktrees []Worktree
}

// NewMockWorktreeManager creates a new mock worktree manager
func NewMockWorktreeManager(repoRoot string) *MockWorktreeManager {
	return &MockWorktreeManager{
		repoRoot: repoRoot,
		worktrees: []Worktree{
			{
				Path:     filepath.Join(repoRoot, ".worktrees", "main"),
				Branch:   "main",
				Commit:   "abc123",
				PRStatus: "",
			},
			{
				Path:     filepath.Join(repoRoot, ".worktrees", "feature-branch"),
				Branch:   "feature-branch",
				Commit:   "def456",
				PRStatus: "open",
			},
		},
	}
}

// CreateWorktree creates a mock worktree
func (m *MockWorktreeManager) CreateWorktree(branchName string) (string, error) {
	sanitizedBranchName := sanitizeBranchName(branchName)
	if sanitizedBranchName == "" {
		return "", fmt.Errorf("branch name results in empty string after sanitization")
	}

	worktreePath := filepath.Join(filepath.Dir(m.repoRoot), ".worktrees", sanitizedBranchName)

	// Check if worktree already exists
	for _, wt := range m.worktrees {
		if wt.Path == worktreePath {
			return worktreePath, nil
		}
	}

	// Add new worktree to mock list
	newWorktree := Worktree{
		Path:     worktreePath,
		Branch:   sanitizedBranchName,
		Commit:   "mock123",
		PRStatus: "",
	}
	m.worktrees = append(m.worktrees, newWorktree)

	return worktreePath, nil
}

// CreateBranch is a no-op mock that tracks the branch creation request
func (m *MockWorktreeManager) CreateBranch(branchName string) error {
	if sanitizeBranchName(branchName) == "" {
		return fmt.Errorf("branch name results in empty string after sanitization")
	}
	return nil
}

// ListWorktrees returns the mock worktree list
func (m *MockWorktreeManager) ListWorktrees() ([]Worktree, error) {
	return m.worktrees, nil
}

// PruneWorktree removes a worktree from the mock list by branch name
func (m *MockWorktreeManager) PruneWorktree(branchName string) error {
	for i, wt := range m.worktrees {
		if wt.Branch == branchName {
			m.worktrees = append(m.worktrees[:i], m.worktrees[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("worktree not found for branch: %s", branchName)
}

// PruneAllMerged removes all merged worktrees (mock implementation)
func (m *MockWorktreeManager) PruneAllMerged() error {
	// In a real implementation, this would check if branches are merged
	// For the mock, we'll just remove any worktrees marked as merged
	var remaining []Worktree
	for _, wt := range m.worktrees {
		if wt.Branch != "main" && wt.PRStatus != "merged" {
			remaining = append(remaining, wt)
		}
	}
	m.worktrees = remaining
	return nil
}
