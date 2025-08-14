package github

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"

	"sprout/pkg/domain/project"
	"sprout/pkg/domain/worktree"
	"sprout/pkg/shared/errors"
	"sprout/pkg/shared/logging"
)

// StatusProvider provides GitHub-based status information for worktrees
type StatusProvider struct {
	project *project.Project
	logger  logging.Logger
}

// NewStatusProvider creates a new GitHub status provider
func NewStatusProvider(project *project.Project, logger logging.Logger) *StatusProvider {
	return &StatusProvider{
		project: project,
		logger:  logger,
	}
}

// GetStatus returns the status of a worktree branch based on GitHub PR status
func (p *StatusProvider) GetStatus(ctx context.Context, branchName string) (worktree.Status, error) {
	if branchName == "" || branchName == "master" || branchName == "main" {
		return worktree.StatusActive, nil
	}
	
	// Fast git-based check first
	if status := p.checkBranchStatusWithGit(ctx, branchName); status != worktree.StatusUnknown {
		return status, nil
	}
	
	// Fallback to GitHub CLI if git checks are inconclusive
	return p.checkPRStatusWithGH(ctx, branchName), nil
}

// checkBranchStatusWithGit uses git commands to determine branch status
func (p *StatusProvider) checkBranchStatusWithGit(ctx context.Context, branchName string) worktree.Status {
	// Check if remote tracking branch exists
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--verify", "origin/"+branchName)
	cmd.Dir = p.project.Path
	
	if err := cmd.Run(); err != nil {
		// Remote branch doesn't exist - could be never pushed or merged and deleted
		if p.wasBranchPushed(ctx, branchName) && p.isBranchMerged(ctx, branchName) {
			return worktree.StatusMerged
		}
		return worktree.StatusUnknown
	}
	
	// Remote branch exists, need more sophisticated check
	return worktree.StatusUnknown
}

// isBranchMerged checks if a branch has been merged into the main branch
func (p *StatusProvider) isBranchMerged(ctx context.Context, branchName string) bool {
	mainBranch := p.project.MainBranch
	if mainBranch == "" {
		return false
	}
	
	// Check if branch commits are in main branch history
	cmd := exec.CommandContext(ctx, "git", "merge-base", "--is-ancestor", branchName, mainBranch)
	cmd.Dir = p.project.Path
	return cmd.Run() == nil
}

// wasBranchPushed checks if a branch was previously pushed to origin
func (p *StatusProvider) wasBranchPushed(ctx context.Context, branchName string) bool {
	// Check git reflog for evidence the branch was pushed
	cmd := exec.CommandContext(ctx, "git", "reflog", "--grep-reflog=origin/"+branchName, "--all", "--oneline")
	cmd.Dir = p.project.Path
	
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	
	// If we find any reflog entries mentioning origin/branchName, it was pushed
	return len(strings.TrimSpace(string(output))) > 0
}

// checkPRStatusWithGH uses GitHub CLI to check PR status
func (p *StatusProvider) checkPRStatusWithGH(ctx context.Context, branchName string) worktree.Status {
	if !p.project.IsGitHubProject() {
		return worktree.StatusUnknown
	}
	
	cmd := exec.CommandContext(ctx, "gh", "pr", "list", "--head", branchName, "--state", "all", "--json", "state", "--limit", "1")
	cmd.Dir = p.project.Path
	
	output, err := cmd.Output()
	if err != nil {
		p.logger.Debug("failed to check PR status with gh", "error", err, "branch", branchName)
		return worktree.StatusUnknown
	}
	
	var prs []PR
	if err := json.Unmarshal(output, &prs); err != nil {
		p.logger.Debug("failed to parse PR JSON", "error", err)
		return worktree.StatusUnknown
	}
	
	if len(prs) == 0 {
		return worktree.StatusActive // No PR means active development
	}
	
	switch prs[0].State {
	case "OPEN":
		return worktree.StatusActive
	case "MERGED":
		return worktree.StatusMerged
	case "CLOSED":
		return worktree.StatusClosed
	default:
		return worktree.StatusUnknown
	}
}

// GetPRInfo returns detailed PR information for a branch
func (p *StatusProvider) GetPRInfo(ctx context.Context, branchName string) (*PRInfo, error) {
	if !p.project.IsGitHubProject() {
		return nil, errors.ConfigurationError("project is not hosted on GitHub")
	}
	
	cmd := exec.CommandContext(ctx, "gh", "pr", "list", "--head", branchName, "--state", "all", "--json", "state,title,url,number", "--limit", "1")
	cmd.Dir = p.project.Path
	
	output, err := cmd.Output()
	if err != nil {
		return nil, errors.ExternalError("failed to get PR information", err)
	}
	
	var prs []PRInfo
	if err := json.Unmarshal(output, &prs); err != nil {
		return nil, errors.ExternalError("failed to parse PR information", err)
	}
	
	if len(prs) == 0 {
		return nil, errors.NotFoundError("no PR found for branch").WithDetail("branch", branchName)
	}
	
	return &prs[0], nil
}

// PR represents a GitHub pull request (minimal info)
type PR struct {
	State string `json:"state"`
	Title string `json:"title"`
}

// PRInfo represents detailed PR information
type PRInfo struct {
	Number int    `json:"number"`
	State  string `json:"state"`
	Title  string `json:"title"`
	URL    string `json:"url"`
}