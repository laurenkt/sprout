package services

import (
	"context"

	"sprout/pkg/domain/project"
	"sprout/pkg/shared/errors"
	"sprout/pkg/shared/logging"
)

// ProjectService implements project.Service
type ProjectService struct {
	repo   project.Repository
	logger logging.Logger
}

// NewProjectService creates a new project service
func NewProjectService(repo project.Repository, logger logging.Logger) *ProjectService {
	return &ProjectService{
		repo:   repo,
		logger: logger,
	}
}

// GetCurrentProject returns the current project with all metadata
func (s *ProjectService) GetCurrentProject(ctx context.Context) (*project.Project, error) {
	proj, err := s.repo.GetCurrent(ctx)
	if err != nil {
		s.logger.Error("failed to get current project", "error", err)
		return nil, err
	}

	s.logger.Debug("retrieved current project", "name", proj.Name, "path", proj.Path)
	return proj, nil
}

// ValidateProject ensures the current directory is a valid git project
func (s *ProjectService) ValidateProject(ctx context.Context) error {
	// Try to get the current project
	_, err := s.repo.GetCurrent(ctx)
	if err != nil {
		if errors.IsType(err, errors.ErrorTypeNotFound) {
			return errors.ValidationError("current directory is not a git repository")
		}
		return err
	}

	// Check if it has a remote origin (optional but recommended)
	hasRemote, err := s.repo.HasRemoteOrigin(ctx)
	if err != nil {
		s.logger.Warn("failed to check remote origin", "error", err)
		// Don't fail validation for this
	} else if !hasRemote {
		s.logger.Warn("project has no remote origin configured")
	}

	return nil
}

// GetProjectInfo returns formatted project information for display
func (s *ProjectService) GetProjectInfo(ctx context.Context) (project.ProjectInfo, error) {
	proj, err := s.repo.GetCurrent(ctx)
	if err != nil {
		return project.ProjectInfo{}, err
	}

	// Get main branch
	mainBranch, err := s.repo.GetMainBranch(ctx)
	if err != nil {
		s.logger.Warn("failed to determine main branch", "error", err)
		mainBranch = "unknown"
	}

	// Check if working directory is clean
	isClean, err := s.repo.IsClean(ctx)
	if err != nil {
		s.logger.Warn("failed to check if working directory is clean", "error", err)
		isClean = false
	}

	info := project.ProjectInfo{
		Name:        proj.Name,
		Path:        proj.Path,
		MainBranch:  mainBranch,
		RemoteURL:   proj.RemoteURL,
		IsGitHub:    proj.IsGitHubProject(),
		WorktreeDir: proj.WorktreeDir,
		HasChanges:  !isClean,
	}

	// Extract GitHub owner and repo if it's a GitHub project
	if proj.IsGitHubProject() {
		owner, repo := proj.GetRepositoryOwnerAndName()
		info.Owner = owner
		info.Repository = repo
	}

	return info, nil
}

// IsInGitRepository checks if the current directory is inside a git repository
func (s *ProjectService) IsInGitRepository(ctx context.Context) bool {
	_, err := s.repo.GetCurrent(ctx)
	return err == nil
}

// GetWorkingDirectoryStatus returns information about the current working directory
func (s *ProjectService) GetWorkingDirectoryStatus(ctx context.Context) (WorkingDirectoryStatus, error) {
	isClean, err := s.repo.IsClean(ctx)
	if err != nil {
		return WorkingDirectoryStatus{}, err
	}

	hasRemote, err := s.repo.HasRemoteOrigin(ctx)
	if err != nil {
		s.logger.Warn("failed to check remote origin", "error", err)
		hasRemote = false
	}

	return WorkingDirectoryStatus{
		IsClean:   isClean,
		HasRemote: hasRemote,
	}, nil
}

// WorkingDirectoryStatus contains information about the working directory
type WorkingDirectoryStatus struct {
	IsClean   bool
	HasRemote bool
}