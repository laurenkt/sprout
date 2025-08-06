package github

import (
	"encoding/json"
	"os/exec"
	"strings"
)

type PR struct {
	State string `json:"state"`
	Title string `json:"title"`
}

type Client struct {
	repoRoot string
}

func NewClient(repoRoot string) *Client {
	return &Client{
		repoRoot: repoRoot,
	}
}

func (c *Client) GetPRStatus(branchName string) string {
	if branchName == "" || branchName == "master" || branchName == "main" {
		return "-"
	}
	
	// Fast git-based check first
	status := c.checkBranchStatusWithGit(branchName)
	if status != "" {
		return status
	}
	
	// Fallback to gh command if git checks are inconclusive
	return c.checkPRStatusWithGH(branchName)
}

func (c *Client) checkBranchStatusWithGit(branchName string) string {
	// Check if remote tracking branch exists
	cmd := exec.Command("git", "rev-parse", "--verify", "origin/"+branchName)
	cmd.Dir = c.repoRoot
	if err := cmd.Run(); err != nil {
		// Remote branch doesn't exist - could be never pushed or merged and deleted
		// Only check for "Merged" if we have evidence the branch was previously pushed
		if c.wasBranchPushed(branchName) && c.isBranchMerged(branchName) {
			return "Merged"
		}
		return "No PR"
	}
	
	// Remote branch exists, check if it's ahead/behind
	return "" // Let gh command handle this case
}

func (c *Client) isBranchMerged(branchName string) bool {
	// Get the main branch name
	mainBranch := c.getMainBranch()
	if mainBranch == "" {
		return false
	}
	
	// Check if branch commits are in main branch history
	cmd := exec.Command("git", "merge-base", "--is-ancestor", branchName, mainBranch)
	cmd.Dir = c.repoRoot
	return cmd.Run() == nil
}

func (c *Client) getMainBranch() string {
	// Try to get default branch from remote
	cmd := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	cmd.Dir = c.repoRoot
	if output, err := cmd.Output(); err == nil {
		// Output format: refs/remotes/origin/main
		parts := strings.Split(strings.TrimSpace(string(output)), "/")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}
	
	// Fallback to common names
	for _, branch := range []string{"main", "master"} {
		cmd := exec.Command("git", "rev-parse", "--verify", "origin/"+branch)
		cmd.Dir = c.repoRoot
		if err := cmd.Run(); err == nil {
			return branch
		}
	}
	
	return "main" // Default fallback
}

func (c *Client) wasBranchPushed(branchName string) bool {
	// Check git reflog for evidence the branch was pushed
	cmd := exec.Command("git", "reflog", "--grep-reflog=origin/"+branchName, "--all", "--oneline")
	cmd.Dir = c.repoRoot
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	
	// If we find any reflog entries mentioning origin/branchName, it was pushed
	return len(strings.TrimSpace(string(output))) > 0
}

func (c *Client) checkPRStatusWithGH(branchName string) string {
	cmd := exec.Command("gh", "pr", "list", "--head", branchName, "--state", "all", "--json", "state", "--limit", "1")
	cmd.Dir = c.repoRoot
	
	output, err := cmd.Output()
	if err != nil {
		return "-"
	}
	
	var prs []PR
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