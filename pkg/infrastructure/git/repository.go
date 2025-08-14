package git

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"

	"sprout/pkg/domain/project"
	"sprout/pkg/shared/errors"
	"sprout/pkg/shared/logging"
)

// ProjectRepository implements project.Repository using git commands
type ProjectRepository struct {
	logger logging.Logger
}

// NewProjectRepository creates a new git-based project repository
func NewProjectRepository(logger logging.Logger) *ProjectRepository {
	return &ProjectRepository{
		logger: logger,
	}
}

// GetCurrent returns information about the current project
func (r *ProjectRepository) GetCurrent(ctx context.Context) (*project.Project, error) {
	repoRoot, err := r.getRepositoryRoot()
	if err != nil {
		return nil, errors.NotFoundError("not in a git repository").WithCause(err)
	}

	name := filepath.Base(repoRoot)
	remoteURL := r.getRemoteURL()
	
	proj := project.NewProject(name, repoRoot, remoteURL)
	
	// Set main branch
	mainBranch, err := r.GetMainBranch(ctx)
	if err != nil {
		r.logger.Warn("failed to determine main branch", "error", err)
		mainBranch = "main" // Default fallback
	}
	proj.MainBranch = mainBranch

	return proj, nil
}

// GetMainBranch determines the main branch name (main/master)
func (r *ProjectRepository) GetMainBranch(ctx context.Context) (string, error) {
	// Try to get default branch from remote
	cmd := exec.CommandContext(ctx, "git", "symbolic-ref", "refs/remotes/origin/HEAD")
	if output, err := cmd.Output(); err == nil {
		// Output format: refs/remotes/origin/main
		parts := strings.Split(strings.TrimSpace(string(output)), "/")
		if len(parts) > 0 {
			return parts[len(parts)-1], nil
		}
	}

	// Check if 'main' branch exists
	cmd = exec.CommandContext(ctx, "git", "show-ref", "--verify", "--quiet", "refs/heads/main")
	if err := cmd.Run(); err == nil {
		return "main", nil
	}

	// Check if 'master' branch exists
	cmd = exec.CommandContext(ctx, "git", "show-ref", "--verify", "--quiet", "refs/heads/master")
	if err := cmd.Run(); err == nil {
		return "master", nil
	}

	// Check remote branches
	cmd = exec.CommandContext(ctx, "git", "show-ref", "--verify", "--quiet", "refs/remotes/origin/main")
	if err := cmd.Run(); err == nil {
		return "origin/main", nil
	}

	cmd = exec.CommandContext(ctx, "git", "show-ref", "--verify", "--quiet", "refs/remotes/origin/master")
	if err := cmd.Run(); err == nil {
		return "origin/master", nil
	}

	return "", errors.NotFoundError("neither 'main' nor 'master' branch found")
}

// HasRemoteOrigin checks if the project has a remote origin
func (r *ProjectRepository) HasRemoteOrigin(ctx context.Context) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "remote", "get-url", "origin")
	err := cmd.Run()
	return err == nil, nil
}

// IsClean returns true if the working directory has no uncommitted changes
func (r *ProjectRepository) IsClean(ctx context.Context) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return false, errors.ExternalError("failed to check git status", err)
	}
	
	return len(strings.TrimSpace(string(output))) == 0, nil
}

// getRepositoryRoot returns the root directory of the current git repository
func (r *ProjectRepository) getRepositoryRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	
	return strings.TrimSpace(string(output)), nil
}

// getRemoteURL returns the remote URL for origin
func (r *ProjectRepository) getRemoteURL() string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	
	return strings.TrimSpace(string(output))
}

// GetRepositoryName returns the name of the repository
func GetRepositoryName() (string, error) {
	// Try to get repo name from remote URL first (works in worktrees)
	cmd := exec.Command("git", "remote", "get-url", "origin")
	output, err := cmd.Output()
	if err == nil {
		remoteURL := strings.TrimSpace(string(output))
		if repoName := extractRepoNameFromURL(remoteURL); repoName != "" {
			return repoName, nil
		}
	}
	
	// Fallback to directory name method
	cmd = exec.Command("git", "rev-parse", "--show-toplevel")
	output, err = cmd.Output()
	if err != nil {
		return "", err
	}
	
	repoRoot := strings.TrimSpace(string(output))
	return filepath.Base(repoRoot), nil
}

// extractRepoNameFromURL extracts repository name from various URL formats
func extractRepoNameFromURL(url string) string {
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