package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"sprout/pkg/application/services"
	"sprout/pkg/domain/project"
	"sprout/pkg/infrastructure/config"
	"sprout/pkg/presentation/tui/components"
	"sprout/pkg/presentation/tui/models"
	"sprout/pkg/shared/errors"
	"sprout/pkg/shared/logging"
)

// App represents the TUI application
type App struct {
	worktreeService *services.WorktreeService
	issueService    *services.IssueService
	project         *project.Project
	config          *config.Config
	logger          logging.Logger
	model           tea.Model
}

// NewApp creates a new TUI application
func NewApp(
	worktreeService *services.WorktreeService,
	issueService *services.IssueService,
	proj *project.Project,
	cfg *config.Config,
	logger logging.Logger,
) (*App, error) {
	app := &App{
		worktreeService: worktreeService,
		issueService:    issueService,
		project:         proj,
		config:          cfg,
		logger:          logger,
	}

	// Initialize the main model
	mainModel, err := app.createMainModel()
	if err != nil {
		return nil, err
	}

	app.model = mainModel
	return app, nil
}

// Run starts the TUI application
func (a *App) Run(ctx context.Context) error {
	a.logger.Info("starting TUI application")

	program := tea.NewProgram(a.model, tea.WithAltScreen())
	
	if _, err := program.Run(); err != nil {
		a.logger.Error("TUI application failed", "error", err)
		return errors.InternalError("TUI execution failed", err)
	}

	return nil
}

// createMainModel creates the main TUI model
func (a *App) createMainModel() (tea.Model, error) {
	// Create state
	state := models.NewAppState(a.project, a.config)

	// Create components
	inputComponent := components.NewInputComponent(a.project.Name)
	
	var issueListComponent models.Component
	if a.issueService != nil {
		issueListComponent = components.NewIssueListComponent(a.issueService, a.logger)
	}

	spinnerComponent := components.NewSpinnerComponent()
	statusComponent := components.NewStatusComponent()

	// Create the main model
	return models.NewMainModel(
		state,
		inputComponent,
		issueListComponent,
		spinnerComponent,
		statusComponent,
		a.worktreeService,
		a.issueService,
		a.logger,
	), nil
}