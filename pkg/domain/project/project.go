package project

import (
	"path/filepath"
	"strings"
)

// Project represents a git repository/project
type Project struct {
	ID          string
	Name        string
	Path        string
	RemoteURL   string
	MainBranch  string
	WorktreeDir string
}

// NewProject creates a new project instance
func NewProject(name, path, remoteURL string) *Project {
	return &Project{
		ID:          generateProjectID(path),
		Name:        name,
		Path:        path,
		RemoteURL:   remoteURL,
		WorktreeDir: filepath.Join(filepath.Dir(path), ".worktrees"),
	}
}

// GetWorktreePath returns the path for a specific worktree
func (p *Project) GetWorktreePath(branchName string) string {
	sanitizedBranch := sanitizeBranchName(branchName)
	return filepath.Join(p.WorktreeDir, sanitizedBranch)
}

// IsGitHubProject returns true if this is a GitHub-hosted project
func (p *Project) IsGitHubProject() bool {
	return strings.Contains(p.RemoteURL, "github.com")
}

// GetRepositoryOwnerAndName extracts owner and repository name from GitHub URL
func (p *Project) GetRepositoryOwnerAndName() (string, string) {
	if !p.IsGitHubProject() {
		return "", ""
	}
	
	// Handle different URL formats
	url := p.RemoteURL
	if strings.HasPrefix(url, "https://") {
		// https://github.com/owner/repo.git -> owner/repo
		parts := strings.Split(url, "/")
		if len(parts) >= 5 {
			owner := parts[3]
			repo := strings.TrimSuffix(parts[4], ".git")
			return owner, repo
		}
	} else if strings.HasPrefix(url, "git@") {
		// git@github.com:owner/repo.git -> owner/repo
		if idx := strings.Index(url, ":"); idx != -1 {
			pathPart := url[idx+1:]
			pathPart = strings.TrimSuffix(pathPart, ".git")
			parts := strings.Split(pathPart, "/")
			if len(parts) == 2 {
				return parts[0], parts[1]
			}
		}
	}
	
	return "", ""
}

// generateProjectID creates a unique ID for a project
func generateProjectID(path string) string {
	return filepath.Base(path)
}

// sanitizeBranchName ensures branch names are safe for filesystem use
func sanitizeBranchName(name string) string {
	if name == "" {
		return "unknown"
	}
	
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.ReplaceAll(name, "/", "-")
	
	var result strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '.' {
			result.WriteRune(r)
		}
	}
	
	cleaned := result.String()
	
	// Remove consecutive hyphens
	for strings.Contains(cleaned, "--") {
		cleaned = strings.ReplaceAll(cleaned, "--", "-")
	}
	
	// Trim and limit length
	cleaned = strings.Trim(cleaned, "-.")
	if len(cleaned) > 100 {
		cleaned = cleaned[:100]
		cleaned = strings.TrimSuffix(cleaned, "-")
	}
	
	return cleaned
}