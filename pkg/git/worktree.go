package git

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type WorktreeManager struct {
	repoRoot string
}

func NewWorktreeManager() (*WorktreeManager, error) {
	repoRoot, err := getRepositoryRoot()
	if err != nil {
		return nil, fmt.Errorf("not in a git repository: %w", err)
	}
	
	return &WorktreeManager{
		repoRoot: repoRoot,
	}, nil
}

func (wm *WorktreeManager) CreateWorktree(branchName string) (string, error) {
	if err := validateBranchName(branchName); err != nil {
		return "", err
	}

	worktreePath := filepath.Join(filepath.Dir(wm.repoRoot), ".worktrees", branchName)
	
	if err := os.MkdirAll(filepath.Dir(worktreePath), 0755); err != nil {
		return "", fmt.Errorf("failed to create .worktrees directory: %w", err)
	}

	if _, err := os.Stat(worktreePath); err == nil {
		if isValidWorktree(worktreePath) {
			return worktreePath, nil
		}
		return "", fmt.Errorf("directory exists but is not a valid worktree: %s", worktreePath)
	}

	cmd := exec.Command("git", "worktree", "add", worktreePath, "-b", branchName)
	cmd.Dir = wm.repoRoot
	
	if output, err := cmd.CombinedOutput(); err != nil {
		if strings.Contains(string(output), "already exists") {
			return worktreePath, nil
		}
		return "", fmt.Errorf("failed to create worktree: %w\nOutput: %s", err, string(output))
	}

	return worktreePath, nil
}

func isValidWorktree(path string) bool {
	gitDir := filepath.Join(path, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		return false
	}
	
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = path
	output, err := cmd.Output()
	
	return err == nil && strings.TrimSpace(string(output)) == "true"
}

func getRepositoryRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	
	return strings.TrimSpace(string(output)), nil
}

type Worktree struct {
	Path     string
	Branch   string
	Commit   string
	PRStatus string
}

type GitHubPR struct {
	State string `json:"state"`
	Title string `json:"title"`
}

func (wm *WorktreeManager) ListWorktrees() ([]Worktree, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = wm.repoRoot
	
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}
	
	worktrees := parseWorktreeList(string(output))
	
	for i := range worktrees {
		worktrees[i].PRStatus = wm.getPRStatus(worktrees[i].Branch)
	}
	
	return worktrees, nil
}

func parseWorktreeList(output string) []Worktree {
	var worktrees []Worktree
	var current Worktree
	
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if line == "" {
			if current.Path != "" {
				worktrees = append(worktrees, current)
				current = Worktree{}
			}
			continue
		}
		
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}
		
		key, value := parts[0], parts[1]
		switch key {
		case "worktree":
			current.Path = value
		case "branch":
			if strings.HasPrefix(value, "refs/heads/") {
				current.Branch = strings.TrimPrefix(value, "refs/heads/")
			}
		case "HEAD":
			current.Commit = value
		}
	}
	
	if current.Path != "" {
		worktrees = append(worktrees, current)
	}
	
	return worktrees
}

func (wm *WorktreeManager) getPRStatus(branchName string) string {
	if branchName == "" || branchName == "master" || branchName == "main" {
		return "-"
	}
	
	cmd := exec.Command("gh", "pr", "list", "--head", branchName, "--json", "state", "--limit", "1")
	cmd.Dir = wm.repoRoot
	
	output, err := cmd.Output()
	if err != nil {
		return "-"
	}
	
	var prs []GitHubPR
	if err := json.Unmarshal(output, &prs); err != nil {
		return "-"
	}
	
	if len(prs) == 0 {
		return "No PR"
	}
	
	switch prs[0].State {
	case "OPEN":
		return "Open"
	case "MERGED":
		return "Merged"
	case "CLOSED":
		return "Closed"
	default:
		return prs[0].State
	}
}

func validateBranchName(name string) error {
	if name == "" {
		return fmt.Errorf("branch name cannot be empty")
	}
	
	if strings.Contains(name, " ") {
		return fmt.Errorf("branch name cannot contain spaces")
	}
	
	if strings.HasPrefix(name, "-") {
		return fmt.Errorf("branch name cannot start with a dash")
	}
	
	return nil
}