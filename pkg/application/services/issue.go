package services

import (
	"context"
	"strings"

	"sprout/pkg/domain/issue"
	"sprout/pkg/shared/errors"
	"sprout/pkg/shared/events"
	"sprout/pkg/shared/logging"
)

// IssueService implements issue.Service
type IssueService struct {
	repo     issue.Repository
	eventBus events.Bus
	logger   logging.Logger
}

// NewIssueService creates a new issue service
func NewIssueService(repo issue.Repository, eventBus events.Bus, logger logging.Logger) *IssueService {
	return &IssueService{
		repo:     repo,
		eventBus: eventBus,
		logger:   logger,
	}
}

// GetAssignedIssues returns issues assigned to the current user with UI state
func (s *IssueService) GetAssignedIssues(ctx context.Context) ([]*issue.Issue, error) {
	if s.repo == nil {
		return nil, errors.NoProvider
	}

	issues, err := s.repo.GetAssignedIssues(ctx)
	if err != nil {
		s.logger.Error("failed to get assigned issues", "error", err)
		return nil, err
	}

	s.logger.Debug("retrieved assigned issues", "count", len(issues))

	// Initialize UI state
	for _, iss := range issues {
		s.initializeUIState(iss)
	}

	return issues, nil
}

// ExpandIssue loads children for an issue and marks it as expanded
func (s *IssueService) ExpandIssue(ctx context.Context, issueID string) ([]*issue.Issue, error) {
	if s.repo == nil {
		return nil, errors.NoProvider
	}

	children, err := s.repo.GetIssueChildren(ctx, issueID)
	if err != nil {
		s.logger.Error("failed to expand issue", "issue_id", issueID, "error", err)
		return nil, err
	}

	s.logger.Debug("expanded issue", "issue_id", issueID, "children_count", len(children))

	// Initialize UI state for children
	for _, child := range children {
		s.initializeUIState(child)
	}

	// Publish event
	s.eventBus.Publish(ctx, events.NewEvent(events.IssueExpanded, map[string]interface{}{
		"issue_id":       issueID,
		"children_count": len(children),
	}))

	return children, nil
}

// CreateSubtask creates a new subtask under a parent issue
func (s *IssueService) CreateSubtask(ctx context.Context, parentID, title string) (*issue.Issue, error) {
	if s.repo == nil {
		return nil, errors.NoProvider
	}

	if strings.TrimSpace(title) == "" {
		return nil, errors.ValidationError("subtask title cannot be empty")
	}

	s.logger.Info("creating subtask", "parent_id", parentID, "title", title)

	subtask, err := s.repo.CreateSubtask(ctx, parentID, title)
	if err != nil {
		s.logger.Error("failed to create subtask", "parent_id", parentID, "title", title, "error", err)
		return nil, err
	}

	s.logger.Info("subtask created successfully", "subtask_id", subtask.ID, "parent_id", parentID)

	// Initialize UI state
	s.initializeUIState(subtask)

	// Publish event
	s.eventBus.Publish(ctx, events.NewEvent(events.SubtaskCreated, map[string]interface{}{
		"subtask_id": subtask.ID,
		"parent_id":  parentID,
		"title":      subtask.Title,
	}))

	return subtask, nil
}

// SearchIssues performs fuzzy search on issues
func (s *IssueService) SearchIssues(ctx context.Context, query string) ([]*issue.Issue, error) {
	if s.repo == nil {
		return nil, errors.NoProvider
	}

	if strings.TrimSpace(query) == "" {
		// Return all assigned issues for empty query
		return s.GetAssignedIssues(ctx)
	}

	results, err := s.repo.SearchIssues(ctx, query)
	if err != nil {
		s.logger.Error("failed to search issues", "query", query, "error", err)
		return nil, err
	}

	s.logger.Debug("searched issues", "query", query, "results_count", len(results))

	// Initialize UI state
	for _, iss := range results {
		s.initializeUIState(iss)
	}

	return results, nil
}

// GetCurrentUser returns information about the authenticated user
func (s *IssueService) GetCurrentUser(ctx context.Context) (*issue.User, error) {
	if s.repo == nil {
		return nil, errors.NoProvider
	}

	// For repositories that support it (like Linear), try to get user info
	if linearRepo, ok := s.repo.(interface{ GetCurrentUser(context.Context) (*issue.User, error) }); ok {
		user, err := linearRepo.GetCurrentUser(ctx)
		if err != nil {
			s.logger.Error("failed to get current user", "error", err)
			return nil, err
		}
		return user, nil
	}

	return nil, errors.InternalError("current repository does not support user information", nil)
}

// IsConfigured returns true if an issue provider is configured
func (s *IssueService) IsConfigured() bool {
	return s.repo != nil
}

// TestConnection tests the connection to the issue provider
func (s *IssueService) TestConnection(ctx context.Context) error {
	if s.repo == nil {
		return errors.NoProvider
	}

	err := s.repo.TestConnection(ctx)
	if err != nil {
		s.logger.Error("issue provider connection test failed", "error", err)
		return err
	}

	s.logger.Info("issue provider connection test successful")
	return nil
}

// initializeUIState sets up initial UI state for an issue
func (s *IssueService) initializeUIState(iss *issue.Issue) {
	iss.Expanded = false
	
	// Set depth based on parent relationship
	if iss.Parent != nil {
		iss.Depth = iss.Parent.Depth + 1
	} else {
		iss.Depth = 0
	}
}

// FlattenIssueTree flattens a tree of issues into a list for easier processing
func (s *IssueService) FlattenIssueTree(issues []*issue.Issue) []*issue.Issue {
	var flattened []*issue.Issue
	
	var flatten func([]*issue.Issue)
	flatten = func(issueList []*issue.Issue) {
		for _, iss := range issueList {
			flattened = append(flattened, iss)
			if len(iss.Children) > 0 {
				flatten(iss.Children)
			}
		}
	}
	
	flatten(issues)
	return flattened
}

// UpdateIssueUIState updates the UI state of an issue (expansion, selection, etc.)
func (s *IssueService) UpdateIssueUIState(ctx context.Context, issueID string, expanded bool) {
	s.eventBus.Publish(ctx, events.NewEvent("issue.ui_state_changed", map[string]interface{}{
		"issue_id": issueID,
		"expanded": expanded,
	}))
}