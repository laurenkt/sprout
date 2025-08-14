package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"sprout/pkg/domain/project"
	"sprout/pkg/domain/worktree"
	"sprout/pkg/infrastructure/config"
	"sprout/pkg/shared/errors"
	"sprout/pkg/shared/logging"
)

// WorktreeRepository implements worktree.Repository using git commands
type WorktreeRepository struct {
	project        *project.Project
	configRepo     config.Repository
	statusProvider StatusProvider
	logger         logging.Logger
}

// StatusProvider defines the interface for getting worktree status
type StatusProvider interface {
	GetStatus(ctx context.Context, branchName string) (worktree.Status, error)
}

// NewWorktreeRepository creates a new git-based worktree repository
func NewWorktreeRepository(project *project.Project, configRepo config.Repository, statusProvider StatusProvider, logger logging.Logger) *WorktreeRepository {
	return &WorktreeRepository{
		project:        project,
		configRepo:     configRepo,
		statusProvider: statusProvider,
		logger:         logger,
	}
}

// Create creates a new worktree with the specified branch name
func (r *WorktreeRepository) Create(ctx context.Context, branchName string) (*worktree.Worktree, error) {
	sanitizedBranch := sanitizeBranchName(branchName)
	if sanitizedBranch == "" {
		return nil, errors.ValidationError("branch name results in empty string after sanitization").
			WithDetail("original_name", branchName)
	}

	worktreePath := r.project.GetWorktreePath(sanitizedBranch)
	
	// Create .worktrees directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(worktreePath), 0755); err != nil {
		return nil, errors.InternalError("failed to create .worktrees directory", err).
			WithDetail("path", filepath.Dir(worktreePath))
	}

	// Check if worktree already exists
	if _, err := os.Stat(worktreePath); err == nil {
		if r.isValidWorktree(worktreePath) {
			// Return existing worktree
			return r.loadExistingWorktree(worktreePath, sanitizedBranch)
		}
		return nil, errors.ConflictError("directory exists but is not a valid worktree").
			WithDetail("path", worktreePath)
	}

	// Create the worktree
	worktreeObj, err := r.createWorktree(ctx, worktreePath, sanitizedBranch)
	if err != nil {
		return nil, err
	}

	return worktreeObj, nil
}

// GetByID retrieves a worktree by its ID
func (r *WorktreeRepository) GetByID(ctx context.Context, id string) (*worktree.Worktree, error) {
	worktrees, err := r.List(ctx, worktree.ListFilter{})
	if err != nil {
		return nil, err
	}
	
	for _, wt := range worktrees {
		if wt.ID == id {
			return wt, nil
		}
	}
	
	return nil, errors.NotFoundError("worktree not found").WithDetail("id", id)
}

// GetByBranch retrieves a worktree by its branch name
func (r *WorktreeRepository) GetByBranch(ctx context.Context, branch string) (*worktree.Worktree, error) {
	worktrees, err := r.List(ctx, worktree.ListFilter{})
	if err != nil {
		return nil, err
	}
	
	for _, wt := range worktrees {
		if wt.Branch == branch {
			return wt, nil
		}
	}
	
	return nil, errors.NotFoundError("worktree not found").WithDetail("branch", branch)
}

// List returns all worktrees, optionally filtered
func (r *WorktreeRepository) List(ctx context.Context, filter worktree.ListFilter) ([]*worktree.Worktree, error) {
	cmd := exec.CommandContext(ctx, "git", "worktree", "list", "--porcelain")
	cmd.Dir = r.project.Path
	
	output, err := cmd.Output()
	if err != nil {
		return nil, errors.ExternalError("failed to list worktrees", err)
	}
	
	worktrees := r.parseWorktreeList(string(output))
	
	// Apply filters
	var filtered []*worktree.Worktree
	for _, wt := range worktrees {
		if r.shouldIncludeWorktree(wt, filter) {
			// Get status if available
			if r.statusProvider != nil {
				if status, err := r.statusProvider.GetStatus(ctx, wt.Branch); err == nil {
					wt.Status = status
				}
			}
			filtered = append(filtered, wt)
		}
	}
	
	return filtered, nil
}

// Update updates an existing worktree
func (r *WorktreeRepository) Update(ctx context.Context, wt *worktree.Worktree) error {
	// For now, we don't need to update git worktrees directly
	// This could be extended to update metadata or move worktrees
	return nil
}

// Delete removes a worktree
func (r *WorktreeRepository) Delete(ctx context.Context, id string) error {
	wt, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}
	
	return r.deleteWorktree(ctx, wt)
}

// DeleteMerged removes all worktrees with merged status
func (r *WorktreeRepository) DeleteMerged(ctx context.Context) ([]string, error) {
	worktrees, err := r.List(ctx, worktree.ListFilter{
		ExcludeMainBranches: true,
	})
	if err != nil {
		return nil, err
	}
	
	var deletedBranches []string
	var failures []string
	
	for _, wt := range worktrees {
		if wt.Status == worktree.StatusMerged {
			if err := r.deleteWorktree(ctx, wt); err != nil {
				r.logger.Warn("failed to delete merged worktree", "branch", wt.Branch, "error", err)
				failures = append(failures, wt.Branch)
			} else {
				deletedBranches = append(deletedBranches, wt.Branch)
			}
		}
	}
	
	if len(failures) > 0 {
		return deletedBranches, errors.InternalError("some worktrees could not be deleted", nil).
			WithDetail("failures", failures)
	}
	
	return deletedBranches, nil
}

// createWorktree creates a new git worktree
func (r *WorktreeRepository) createWorktree(ctx context.Context, worktreePath, branchName string) (*worktree.Worktree, error) {
	// Check if sparse checkout is configured
	cfg, err := r.configRepo.Load()
	if err != nil {
		r.logger.Warn("failed to load config, using normal checkout", "error", err)
		return r.createNormalWorktree(ctx, worktreePath, branchName)
	}

	directories, hasSparseCheckout := cfg.GetSparseCheckoutDirectories(r.project.Path)
	if hasSparseCheckout {
		return r.createSparseWorktree(ctx, worktreePath, branchName, directories)
	}

	return r.createNormalWorktree(ctx, worktreePath, branchName)
}

// createNormalWorktree creates a regular worktree
func (r *WorktreeRepository) createNormalWorktree(ctx context.Context, worktreePath, branchName string) (*worktree.Worktree, error) {
	cmd := exec.CommandContext(ctx, "git", "worktree", "add", worktreePath, "-b", branchName, r.project.MainBranch)
	cmd.Dir = r.project.Path
	
	if output, err := cmd.CombinedOutput(); err != nil {
		if strings.Contains(string(output), "already exists") {
			return r.loadExistingWorktree(worktreePath, branchName)
		}
		return nil, errors.ExternalError("failed to create worktree", err).
			WithDetail("output", string(output)).
			WithDetail("path", worktreePath).
			WithDetail("branch", branchName)
	}

	return worktree.NewWorktree(worktreePath, branchName, ""), nil
}

// createSparseWorktree creates a sparse-checkout worktree
func (r *WorktreeRepository) createSparseWorktree(ctx context.Context, worktreePath, branchName string, directories []string) (*worktree.Worktree, error) {
	// Create worktree without checkout
	cmd := exec.CommandContext(ctx, "git", "worktree", "add", "--no-checkout", worktreePath, "-b", branchName, r.project.MainBranch)
	cmd.Dir = r.project.Path
	
	if output, err := cmd.CombinedOutput(); err != nil {
		if strings.Contains(string(output), "already exists") {
			return r.loadExistingWorktree(worktreePath, branchName)
		}
		return nil, errors.ExternalError("failed to create worktree", err).
			WithDetail("output", string(output))
	}

	// Initialize sparse checkout
	if err := r.initializeSparseCheckout(ctx, worktreePath, directories); err != nil {
		r.logger.Warn("failed to initialize sparse checkout, falling back to normal checkout", "error", err)
		return r.checkoutAll(ctx, worktreePath, branchName)
	}

	r.logger.Info("created sparse worktree", "directories", directories)
	return worktree.NewWorktree(worktreePath, branchName, ""), nil
}

// initializeSparseCheckout sets up sparse checkout in a worktree
func (r *WorktreeRepository) initializeSparseCheckout(ctx context.Context, worktreePath string, directories []string) error {
	// Initialize sparse checkout with cone mode
	cmd := exec.CommandContext(ctx, "git", "sparse-checkout", "init", "--cone")
	cmd.Dir = worktreePath
	
	if err := cmd.Run(); err != nil {
		return errors.ExternalError("failed to initialize sparse checkout", err)
	}

	// Set sparse checkout directories
	args := append([]string{"sparse-checkout", "set"}, directories...)
	cmd = exec.CommandContext(ctx, "git", args...)
	cmd.Dir = worktreePath
	
	if err := cmd.Run(); err != nil {
		return errors.ExternalError("failed to set sparse checkout patterns", err)
	}

	// Checkout with sparse patterns applied
	cmd = exec.CommandContext(ctx, "git", "checkout")
	cmd.Dir = worktreePath
	
	if err := cmd.Run(); err != nil {
		return errors.ExternalError("failed to checkout with sparse patterns", err)
	}

	return nil
}

// checkoutAll performs a full checkout (fallback for sparse checkout failures)
func (r *WorktreeRepository) checkoutAll(ctx context.Context, worktreePath, branchName string) (*worktree.Worktree, error) {
	cmd := exec.CommandContext(ctx, "git", "checkout")
	cmd.Dir = worktreePath
	
	if err := cmd.Run(); err != nil {
		return nil, errors.ExternalError("failed to checkout", err)
	}

	return worktree.NewWorktree(worktreePath, branchName, ""), nil
}

// deleteWorktree removes a worktree and its branch
func (r *WorktreeRepository) deleteWorktree(ctx context.Context, wt *worktree.Worktree) error {
	// Remove worktree from git
	cmd := exec.CommandContext(ctx, "git", "worktree", "remove", wt.Path, "--force")
	cmd.Dir = r.project.Path
	
	if output, err := cmd.CombinedOutput(); err != nil {
		r.logger.Warn("git worktree remove failed, attempting manual cleanup", "error", err, "output", string(output))
	}

	// Remove the directory manually
	if err := os.RemoveAll(wt.Path); err != nil {
		return errors.InternalError("failed to remove worktree directory", err).
			WithDetail("path", wt.Path)
	}

	// Delete the branch
	cmd = exec.CommandContext(ctx, "git", "branch", "-D", wt.Branch)
	cmd.Dir = r.project.Path
	
	if err := cmd.Run(); err != nil {
		r.logger.Warn("failed to delete branch", "branch", wt.Branch, "error", err)
		// Don't fail the entire operation if branch deletion fails
	}

	return nil
}

// parseWorktreeList parses the output of 'git worktree list --porcelain'
func (r *WorktreeRepository) parseWorktreeList(output string) []*worktree.Worktree {
	var worktrees []*worktree.Worktree
	var current *worktree.Worktree
	
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if line == "" {
			if current != nil && current.Path != "" {
				worktrees = append(worktrees, current)
				current = nil
			}
			continue
		}
		
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}
		
		key, value := parts[0], parts[1]
		if current == nil {
			current = &worktree.Worktree{}
		}
		
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
	
	// Don't forget the last worktree
	if current != nil && current.Path != "" {
		worktrees = append(worktrees, current)
	}
	
	// Set additional fields
	for _, wt := range worktrees {
		wt.ID = fmt.Sprintf("%s-%s", filepath.Base(wt.Path), wt.Branch)
		wt.CreatedAt = time.Now() // We can't easily get the real creation time
		wt.LastAccessed = time.Now()
		wt.Status = worktree.StatusActive // Will be updated by status provider
	}
	
	return worktrees
}

// shouldIncludeWorktree checks if a worktree matches the filter criteria
func (r *WorktreeRepository) shouldIncludeWorktree(wt *worktree.Worktree, filter worktree.ListFilter) bool {
	if filter.ExcludeMainBranches && wt.IsMainBranch() {
		return false
	}
	
	if filter.Status != nil && wt.Status != *filter.Status {
		return false
	}
	
	if filter.BranchPattern != "" && !strings.Contains(wt.Branch, filter.BranchPattern) {
		return false
	}
	
	return true
}

// loadExistingWorktree creates a Worktree object for an existing directory
func (r *WorktreeRepository) loadExistingWorktree(worktreePath, branchName string) (*worktree.Worktree, error) {
	return worktree.NewWorktree(worktreePath, branchName, ""), nil
}

// isValidWorktree checks if a directory is a valid git worktree
func (r *WorktreeRepository) isValidWorktree(path string) bool {
	gitDir := filepath.Join(path, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		return false
	}
	
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = path
	output, err := cmd.Output()
	
	return err == nil && strings.TrimSpace(string(output)) == "true"
}

// sanitizeBranchName ensures branch names are safe for git and filesystem use
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