package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"sprout/pkg/config"
	"sprout/pkg/github"
)

// WorktreeManagerInterface defines the interface for worktree operations
type WorktreeManagerInterface interface {
	CreateWorktree(branchName string) (string, error)
	CreateBranch(branchName string) error
	ListWorktrees() ([]Worktree, error)
	PruneWorktree(branchName string) error
	PruneAllMerged() error
}

type WorktreeManager struct {
	repoRoot     string
	repoName     string
	configLoader config.LoaderInterface
	githubClient *github.Client
}

func NewWorktreeManager() (*WorktreeManager, error) {
	repoRoot, err := getRepositoryRoot()
	if err != nil {
		return nil, fmt.Errorf("not in a git repository: %w", err)
	}

	repoName, err := GetRepositoryName()
	if err != nil {
		return nil, fmt.Errorf("failed to determine repository name: %w", err)
	}

	return &WorktreeManager{
		repoRoot:     repoRoot,
		repoName:     repoName,
		configLoader: &config.FileLoader{},
		githubClient: github.NewClient(repoRoot),
	}, nil
}

func (wm *WorktreeManager) CreateWorktree(branchName string) (string, error) {
	sanitizedBranchName := sanitizeBranchName(branchName)
	if sanitizedBranchName == "" {
		return "", fmt.Errorf("branch name results in empty string after sanitization")
	}

	cfg, cfgErr := wm.loadConfig()
	worktreePath := wm.resolveWorktreePath(cfg, sanitizedBranchName)

	if err := os.MkdirAll(filepath.Dir(worktreePath), 0755); err != nil {
		return "", fmt.Errorf("failed to create worktree base directory: %w", err)
	}

	if _, err := os.Stat(worktreePath); err == nil {
		if isValidWorktree(worktreePath) {
			return worktreePath, nil
		}
		return "", fmt.Errorf("directory exists but is not a valid worktree: %s", worktreePath)
	}

	if cfgErr != nil {
		// Log warning but continue with normal worktree creation
		fmt.Printf("Warning: failed to load config, using normal checkout: %v\n", cfgErr)
		return wm.createNormalWorktree(worktreePath, sanitizedBranchName)
	}

	directories, hasSparseCheckout := cfg.GetSparseCheckoutDirectories(wm.repoRoot)
	if hasSparseCheckout {
		return wm.createSparseWorktree(worktreePath, sanitizedBranchName, directories)
	}

	return wm.createNormalWorktree(worktreePath, sanitizedBranchName)
}

func (wm *WorktreeManager) loadConfig() (*config.Config, error) {
	if wm.configLoader != nil {
		return wm.configLoader.GetConfig()
	}

	return config.Load()
}

func (wm *WorktreeManager) getWorktreeBasePath(cfg *config.Config, branchName string) (string, bool) {
	if cfg != nil {
		if basePath, includesBranch, ok := cfg.GetWorktreeBasePath(wm.repoName, wm.repoRoot, branchName); ok {
			return basePath, includesBranch
		}
	}

	return filepath.Join(filepath.Dir(wm.repoRoot), ".worktrees"), false
}

func (wm *WorktreeManager) resolveWorktreePath(cfg *config.Config, branchName string) string {
	basePath, includesBranch := wm.getWorktreeBasePath(cfg, branchName)
	if includesBranch {
		return basePath
	}
	return filepath.Join(basePath, branchName)
}

func (wm *WorktreeManager) createNormalWorktree(worktreePath, branchName string) (string, error) {
	// Determine the base branch (master or main)
	baseBranch, err := wm.getBaseBranch()
	if err != nil {
		return "", fmt.Errorf("failed to determine base branch: %w", err)
	}

	cmd := exec.Command("git", "worktree", "add", worktreePath, "-b", branchName, baseBranch)
	cmd.Dir = wm.repoRoot

	if output, err := cmd.CombinedOutput(); err != nil {
		if strings.Contains(string(output), "already exists") {
			return worktreePath, nil
		}
		return "", fmt.Errorf("failed to create worktree: %w\nOutput: %s", err, string(output))
	}

	return worktreePath, nil
}

func (wm *WorktreeManager) createSparseWorktree(worktreePath, branchName string, directories []string) (string, error) {
	// Determine the base branch (master or main)
	baseBranch, err := wm.getBaseBranch()
	if err != nil {
		return "", fmt.Errorf("failed to determine base branch: %w", err)
	}

	// Create worktree without checkout
	cmd := exec.Command("git", "worktree", "add", "--no-checkout", worktreePath, "-b", branchName, baseBranch)
	cmd.Dir = wm.repoRoot

	if output, err := cmd.CombinedOutput(); err != nil {
		if strings.Contains(string(output), "already exists") {
			return worktreePath, nil
		}
		return "", fmt.Errorf("failed to create worktree: %w\nOutput: %s", err, string(output))
	}

	// Initialize sparse checkout with cone mode
	cmd = exec.Command("git", "sparse-checkout", "init", "--cone")
	cmd.Dir = worktreePath

	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("Warning: failed to initialize sparse checkout, falling back to normal checkout: %v\nOutput: %s\n", err, string(output))
		// Fallback: checkout everything
		return wm.checkoutAll(worktreePath)
	}

	// Set sparse checkout directories
	args := append([]string{"sparse-checkout", "set"}, directories...)
	cmd = exec.Command("git", args...)
	cmd.Dir = worktreePath

	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("Warning: failed to set sparse checkout patterns, falling back to normal checkout: %v\nOutput: %s\n", err, string(output))
		// Fallback: checkout everything
		return wm.checkoutAll(worktreePath)
	}

	// Checkout with sparse patterns applied
	cmd = exec.Command("git", "checkout")
	cmd.Dir = worktreePath

	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("Warning: failed to checkout with sparse patterns, falling back to normal checkout: %v\nOutput: %s\n", err, string(output))
		// Fallback: checkout everything
		return wm.checkoutAll(worktreePath)
	}

	fmt.Printf("Created sparse worktree with directories: %s\n", strings.Join(directories, ", "))
	return worktreePath, nil
}

func (wm *WorktreeManager) checkoutAll(worktreePath string) (string, error) {
	cmd := exec.Command("git", "checkout")
	cmd.Dir = worktreePath

	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to checkout: %w\nOutput: %s", err, string(output))
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

func GetRepositoryName() (string, error) {
	// Try to get repo name from remote URL first (works in worktrees)
	cmd := exec.Command("git", "remote", "get-url", "origin")
	output, err := cmd.Output()
	if err == nil {
		remoteURL := strings.TrimSpace(string(output))
		// Extract repo name from URL like "https://github.com/user/repo.git" or "git@github.com:user/repo.git"
		if repoName := extractRepoNameFromURL(remoteURL); repoName != "" {
			return repoName, nil
		}
	}

	// Fallback to directory name method
	repoRoot, err := getRepositoryRoot()
	if err != nil {
		return "", err
	}

	return filepath.Base(repoRoot), nil
}

func extractRepoNameFromURL(url string) string {
	// Handle different URL formats
	if strings.HasPrefix(url, "https://") {
		// https://github.com/user/repo.git -> repo
		parts := strings.Split(url, "/")
		if len(parts) > 0 {
			repoWithExt := parts[len(parts)-1]
			return strings.TrimSuffix(repoWithExt, ".git")
		}
	} else if strings.HasPrefix(url, "git@") {
		// git@github.com:user/repo.git -> repo
		if idx := strings.LastIndex(url, "/"); idx != -1 {
			repoWithExt := url[idx+1:]
			return strings.TrimSuffix(repoWithExt, ".git")
		} else if idx := strings.LastIndex(url, ":"); idx != -1 {
			pathPart := url[idx+1:]
			if slashIdx := strings.LastIndex(pathPart, "/"); slashIdx != -1 {
				repoWithExt := pathPart[slashIdx+1:]
				return strings.TrimSuffix(repoWithExt, ".git")
			} else {
				return strings.TrimSuffix(pathPart, ".git")
			}
		}
	}

	return ""
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

func (wm *WorktreeManager) getBaseBranch() (string, error) {
	// Check if 'main' branch exists
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/main")
	cmd.Dir = wm.repoRoot
	if err := cmd.Run(); err == nil {
		return "main", nil
	}

	// Check if 'master' branch exists
	cmd = exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/master")
	cmd.Dir = wm.repoRoot
	if err := cmd.Run(); err == nil {
		return "master", nil
	}

	// Also check remote branches in case we haven't fetched yet
	cmd = exec.Command("git", "show-ref", "--verify", "--quiet", "refs/remotes/origin/main")
	cmd.Dir = wm.repoRoot
	if err := cmd.Run(); err == nil {
		return "origin/main", nil
	}

	cmd = exec.Command("git", "show-ref", "--verify", "--quiet", "refs/remotes/origin/master")
	cmd.Dir = wm.repoRoot
	if err := cmd.Run(); err == nil {
		return "origin/master", nil
	}

	return "", fmt.Errorf("neither 'main' nor 'master' branch found")
}

func sanitizeBranchName(name string) string {
	if name == "" {
		return ""
	}

	// Convert to lowercase for consistency
	name = strings.ToLower(name)

	// Replace spaces and other problematic characters with hyphens
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")

	// Remove special characters that aren't allowed in git branch names
	var result strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '.' || r == '/' {
			result.WriteRune(r)
		}
	}
	name = result.String()

	// Remove consecutive hyphens
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}

	// Remove leading/trailing hyphens and dots
	name = strings.Trim(name, "-.")

	// Ensure it doesn't start with a slash
	name = strings.TrimPrefix(name, "/")

	// Limit length to reasonable size
	if len(name) > 100 {
		name = name[:100]
		name = strings.TrimSuffix(name, "-")
	}

	return name
}

func (wm *WorktreeManager) PruneWorktree(branchName string) error {
	// For pruning, we should use the branch name as-is since it comes from git worktree list
	// But we still need to check it's not empty
	if branchName == "" {
		return fmt.Errorf("branch name cannot be empty")
	}

	cfg, err := wm.loadConfig()
	if err != nil {
		fmt.Printf("Warning: failed to load config, using default worktree path: %v\n", err)
	}

	worktreePath := wm.resolveWorktreePath(cfg, branchName)

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

	cfg, cfgErr := wm.loadConfig()
	if cfgErr != nil {
		fmt.Printf("Warning: failed to load config, using default worktree path: %v\n", cfgErr)
	}

	var mergedWorktrees []Worktree
	for _, wt := range worktrees {
		// Skip main/master branches and only include merged PRs
		if wt.Branch == "master" || wt.Branch == "main" || wt.Branch == "" {
			continue
		}
		if wt.PRStatus == "Merged" {
			// Check if worktree directory actually exists
			worktreePath := wm.resolveWorktreePath(cfg, wt.Branch)
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

// CreateBranch creates a git branch without making a worktree
func (wm *WorktreeManager) CreateBranch(branchName string) error {
	sanitizedBranchName := sanitizeBranchName(branchName)
	if sanitizedBranchName == "" {
		return fmt.Errorf("branch name results in empty string after sanitization")
	}

	// Determine the base branch to branch from
	baseBranch, err := wm.getBaseBranch()
	if err != nil {
		return fmt.Errorf("failed to determine base branch: %w", err)
	}

	// If the branch already exists, treat it as success
	checkCmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+sanitizedBranchName)
	checkCmd.Dir = wm.repoRoot
	if err := checkCmd.Run(); err == nil {
		return nil
	}

	cmd := exec.Command("git", "branch", sanitizedBranchName, baseBranch)
	cmd.Dir = wm.repoRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create branch: %w\nOutput: %s", err, string(output))
	}

	return nil
}
