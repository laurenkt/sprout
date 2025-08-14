package container

import (
	"context"
	"log/slog"
	"os"

	"sprout/pkg/domain/issue"
	"sprout/pkg/domain/project"
	"sprout/pkg/domain/worktree"
	"sprout/pkg/infrastructure/config"
	"sprout/pkg/infrastructure/git"
	"sprout/pkg/infrastructure/github"
	"sprout/pkg/infrastructure/linear"
	"sprout/pkg/shared/errors"
	"sprout/pkg/shared/events"
	"sprout/pkg/shared/logging"
)

// Container holds all application dependencies
type Container struct {
	// Infrastructure
	config     config.Repository
	eventBus   events.Bus
	logger     logging.Logger
	
	// Repositories
	projectRepo  project.Repository
	worktreeRepo worktree.Repository
	issueRepo    issue.Repository
	
	// Providers
	statusProvider git.StatusProvider
	
	// Cached instances
	currentProject *project.Project
	
	// Configuration
	appConfig *config.Config
}

// New creates a new dependency injection container
func New(ctx context.Context) (*Container, error) {
	container := &Container{}
	
	if err := container.initialize(ctx); err != nil {
		return nil, err
	}
	
	return container, nil
}

// initialize sets up all dependencies
func (c *Container) initialize(ctx context.Context) error {
	// Initialize event bus
	c.eventBus = events.NewInMemoryBus()
	
	// Load configuration first
	configRepo, err := config.NewFileRepository()
	if err != nil {
		return errors.ConfigurationError("failed to initialize config repository").WithCause(err)
	}
	c.config = configRepo
	
	cfg, err := c.config.Load()
	if err != nil {
		return errors.ConfigurationError("failed to load application config").WithCause(err)
	}
	c.appConfig = cfg
	
	// Initialize logger based on config
	c.logger = c.createLogger(cfg)
	
	// Initialize project repository
	c.projectRepo = git.NewProjectRepository(c.logger)
	
	// Get current project
	proj, err := c.projectRepo.GetCurrent(ctx)
	if err != nil {
		return errors.NotFoundError("failed to get current project").WithCause(err)
	}
	c.currentProject = proj
	
	// Initialize status provider (GitHub if it's a GitHub project)
	if proj.IsGitHubProject() {
		c.statusProvider = github.NewStatusProvider(proj, c.logger)
	}
	
	// Initialize worktree repository
	c.worktreeRepo = git.NewWorktreeRepository(proj, c.config, c.statusProvider, c.logger)
	
	// Initialize issue repository (Linear if configured)
	if cfg.IsLinearConfigured() {
		c.issueRepo = linear.NewRepository(cfg.GetLinearAPIKey(), c.logger)
	}
	
	c.logger.Info("dependency container initialized", 
		"project", proj.Name, 
		"linear_configured", cfg.IsLinearConfigured(),
		"github_project", proj.IsGitHubProject())
	
	return nil
}

// createLogger creates a logger instance based on configuration
func (c *Container) createLogger(cfg *config.Config) logging.Logger {
	var level slog.Level
	switch cfg.GetLogLevel() {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelWarn
	}
	
	var output *os.File
	switch cfg.GetLogOutput() {
	case "stdout":
		output = os.Stdout
	case "stderr":
		output = os.Stderr
	default:
		output = os.Stderr
	}
	
	return logging.NewLogger(output, level)
}

// Close shuts down the container and all its resources
func (c *Container) Close() error {
	if c.eventBus != nil {
		return c.eventBus.Close()
	}
	return nil
}

// Getters for dependencies

func (c *Container) Config() config.Repository {
	return c.config
}

func (c *Container) AppConfig() *config.Config {
	return c.appConfig
}

func (c *Container) EventBus() events.Bus {
	return c.eventBus
}

func (c *Container) Logger() logging.Logger {
	return c.logger
}

func (c *Container) ProjectRepo() project.Repository {
	return c.projectRepo
}

func (c *Container) WorktreeRepo() worktree.Repository {
	return c.worktreeRepo
}

func (c *Container) IssueRepo() issue.Repository {
	return c.issueRepo
}

func (c *Container) CurrentProject() *project.Project {
	return c.currentProject
}

func (c *Container) StatusProvider() git.StatusProvider {
	return c.statusProvider
}

// HasIssueProvider returns true if an issue provider is configured
func (c *Container) HasIssueProvider() bool {
	return c.issueRepo != nil
}

// HasStatusProvider returns true if a status provider is available
func (c *Container) HasStatusProvider() bool {
	return c.statusProvider != nil
}