package cli

import (
	"sprout/pkg/config"
	"sprout/pkg/git"
	"sprout/pkg/linear"
)

// MockWorktreeManager implements git.WorktreeManagerInterface for testing
type MockWorktreeManager struct {
	Worktrees []git.Worktree
}

func (m *MockWorktreeManager) CreateWorktree(branchName string) (string, error) {
	// For testing purposes, just return a mock path
	return "/mock/path/" + branchName, nil
}

func (m *MockWorktreeManager) ListWorktrees() ([]git.Worktree, error) {
	return m.Worktrees, nil
}

func (m *MockWorktreeManager) PruneWorktree(branchName string) error {
	return nil
}

func (m *MockWorktreeManager) PruneAllMerged() error {
	return nil
}

// MockConfigLoader implements config.LoaderInterface for testing
type MockConfigLoader struct {
	Config *config.Config
}

func (m *MockConfigLoader) GetConfig() (*config.Config, error) {
	return m.Config, nil
}

// MockLinearClient implements linear.LinearClientInterface for testing
type MockLinearClient struct {
	CurrentUser     *linear.User
	AssignedIssues  []linear.Issue
	ConnectionError error
}

func (m *MockLinearClient) GetCurrentUser() (*linear.User, error) {
	if m.ConnectionError != nil {
		return nil, m.ConnectionError
	}
	return m.CurrentUser, nil
}

func (m *MockLinearClient) GetAssignedIssues() ([]linear.Issue, error) {
	if m.ConnectionError != nil {
		return nil, m.ConnectionError
	}
	return m.AssignedIssues, nil
}

func (m *MockLinearClient) GetIssueChildren(issueID string) ([]linear.Issue, error) {
	return []linear.Issue{}, nil
}

func (m *MockLinearClient) CreateSubtask(parentID, title string) (*linear.Issue, error) {
	return &linear.Issue{}, nil
}

func (m *MockLinearClient) TestConnection() error {
	return m.ConnectionError
}

// MockConfigPathProvider provides configurable config path and file status for testing
type MockConfigPathProvider struct {
	ConfigPath   string
	FileExists   bool
	PathError    error
}

func (p *MockConfigPathProvider) GetConfigPath() (string, error) {
	if p.PathError != nil {
		return "", p.PathError
	}
	return p.ConfigPath, nil
}

func (p *MockConfigPathProvider) ConfigFileExists() bool {
	return p.FileExists
}