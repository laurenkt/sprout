package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	
	"sprout/pkg/github"
)

type WorktreeManager struct {
	repoRoot     string
	githubClient *github.Client
}

func NewWorktreeManager() (*WorktreeManager, error) {
	repoRoot, err := getRepositoryRoot()
	if err != nil {
		return nil, fmt.Errorf("not in a git repository: %w", err)
	}
	
	return &WorktreeManager{
		repoRoot:     repoRoot,
		githubClient: github.NewClient(repoRoot),
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

func (wm *WorktreeManager) ListWorktrees() ([]Worktree, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = wm.repoRoot
	
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}
	
	worktrees := parseWorktreeList(string(output))
	
	for i := range worktrees {
		worktrees[i].PRStatus = wm.githubClient.GetPRStatus(worktrees[i].Branch)
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

func (wm *WorktreeManager) PruneWorktree(branchName string) error {
	if err := validateBranchName(branchName); err != nil {
		return err
	}

	worktreePath := filepath.Join(filepath.Dir(wm.repoRoot), ".worktrees", branchName)
	
	// Check if worktree exists
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		return fmt.Errorf("worktree does not exist: %s", branchName)
	}

	// Remove worktree from git
	cmd := exec.Command("git", "worktree", "remove", worktreePath, "--force")
	cmd.Dir = wm.repoRoot
	
	if output, err := cmd.CombinedOutput(); err != nil {
		// If git worktree remove fails, we still want to try to remove the directory
		fmt.Printf("Warning: git worktree remove failed: %v\nOutput: %s\n", err, string(output))
		fmt.Println("Attempting to remove directory manually...")
	}

	// Remove the directory and all its contents
	if err := os.RemoveAll(worktreePath); err != nil {
		return fmt.Errorf("failed to remove worktree directory: %w", err)
	}

	// Delete the branch if it exists and has no commits beyond the base
	cmd = exec.Command("git", "branch", "-D", branchName)
	cmd.Dir = wm.repoRoot
	
	if output, err := cmd.CombinedOutput(); err != nil {
		// Branch deletion might fail if it doesn't exist or has unmerged changes
		// This is not necessarily an error, so we just warn
		fmt.Printf("Warning: failed to delete branch '%s': %v\nOutput: %s\n", branchName, err, string(output))
	}

	fmt.Printf("Worktree '%s' has been pruned successfully\n", branchName)
	return nil
}

func (wm *WorktreeManager) PruneAllMerged() error {
	worktrees, err := wm.ListWorktrees()
	if err != nil {
		return err
	}

	var mergedWorktrees []Worktree
	for _, wt := range worktrees {
		// Skip main/master branches and only include merged PRs
		if (wt.Branch == "master" || wt.Branch == "main" || wt.Branch == "") {
			continue
		}
		if wt.PRStatus == "Merged" {
			// Check if worktree directory actually exists
			worktreePath := filepath.Join(filepath.Dir(wm.repoRoot), ".worktrees", wt.Branch)
			if _, err := os.Stat(worktreePath); err == nil {
				mergedWorktrees = append(mergedWorktrees, wt)
			}
		}
	}

	if len(mergedWorktrees) == 0 {
		fmt.Println("No merged worktrees found to prune")
		return nil
	}

	fmt.Printf("Found %d merged worktree(s) to prune:\n", len(mergedWorktrees))
	for _, wt := range mergedWorktrees {
		fmt.Printf("  - %s\n", wt.Branch)
	}
	fmt.Println()

	var failed []string
	for _, wt := range mergedWorktrees {
		fmt.Printf("Pruning %s...\n", wt.Branch)
		if err := wm.PruneWorktree(wt.Branch); err != nil {
			fmt.Printf("Failed to prune %s: %v\n", wt.Branch, err)
			failed = append(failed, wt.Branch)
		}
	}

	if len(failed) > 0 {
		fmt.Printf("\nFailed to prune %d worktree(s): %s\n", len(failed), strings.Join(failed, ", "))
		return fmt.Errorf("some worktrees could not be pruned")
	}

	fmt.Printf("\nSuccessfully pruned %d merged worktree(s)\n", len(mergedWorktrees))
	return nil
}