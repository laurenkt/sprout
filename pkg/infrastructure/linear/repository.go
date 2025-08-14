package linear

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"sprout/pkg/domain/issue"
	"sprout/pkg/shared/errors"
	"sprout/pkg/shared/logging"
)

const (
	APIEndpoint = "https://api.linear.app/graphql"
)

// Repository implements issue.Repository using the Linear GraphQL API
type Repository struct {
	apiKey     string
	httpClient *http.Client
	logger     logging.Logger
}

// NewRepository creates a new Linear repository
func NewRepository(apiKey string, logger logging.Logger) *Repository {
	return &Repository{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// GetAssignedIssues returns issues assigned to the current user
func (r *Repository) GetAssignedIssues(ctx context.Context) ([]*issue.Issue, error) {
	query := `
		query {
			issues(
				filter: {
					assignee: { isMe: { eq: true } }
					state: { type: { neq: "completed" } }
				}
				orderBy: updatedAt
			) {
				nodes {
					id
					title
					description
					identifier
					url
					priority
					createdAt
					updatedAt
					state {
						id
						name
						type
					}
					assignee {
						id
						name
						displayName
						email
					}
					children {
						nodes {
							id
						}
					}
				}
			}
		}
	`

	resp, err := r.makeRequest(ctx, query, nil)
	if err != nil {
		return nil, errors.ExternalError("failed to fetch assigned issues", err)
	}

	var result struct {
		Issues struct {
			Nodes []linearIssueResponse `json:"nodes"`
		} `json:"issues"`
	}

	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, errors.ExternalError("failed to unmarshal issues data", err)
	}

	issues := make([]*issue.Issue, len(result.Issues.Nodes))
	for i, node := range result.Issues.Nodes {
		issues[i] = r.convertIssueFromResponse(node)
		issues[i].HasChildren = len(node.Children.Nodes) > 0
		issues[i].Depth = 0
		issues[i].Expanded = false
		issues[i].Parent = nil
	}

	return issues, nil
}

// GetIssueByID retrieves a specific issue by its ID
func (r *Repository) GetIssueByID(ctx context.Context, id string) (*issue.Issue, error) {
	query := `
		query($issueId: String!) {
			issue(id: $issueId) {
				id
				title
				description
				identifier
				url
				priority
				createdAt
				updatedAt
				state {
					id
					name
					type
				}
				assignee {
					id
					name
					displayName
					email
				}
				children {
					nodes {
						id
					}
				}
			}
		}
	`

	variables := map[string]interface{}{
		"issueId": id,
	}

	resp, err := r.makeRequest(ctx, query, variables)
	if err != nil {
		return nil, errors.ExternalError("failed to fetch issue", err).WithDetail("issue_id", id)
	}

	var result struct {
		Issue *linearIssueResponse `json:"issue"`
	}

	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, errors.ExternalError("failed to unmarshal issue data", err)
	}

	if result.Issue == nil {
		return nil, errors.NotFoundError("issue not found").WithDetail("issue_id", id)
	}

	iss := r.convertIssueFromResponse(*result.Issue)
	iss.HasChildren = len(result.Issue.Children.Nodes) > 0

	return iss, nil
}

// GetIssueChildren retrieves child issues for a parent issue
func (r *Repository) GetIssueChildren(ctx context.Context, parentID string) ([]*issue.Issue, error) {
	query := `
		query($issueId: String!) {
			issue(id: $issueId) {
				children {
					nodes {
						id
						title
						description
						identifier
						url
						priority
						createdAt
						updatedAt
						state {
							id
							name
							type
						}
						assignee {
							id
							name
							displayName
							email
						}
						children {
							nodes {
								id
							}
						}
					}
				}
			}
		}
	`

	variables := map[string]interface{}{
		"issueId": parentID,
	}

	resp, err := r.makeRequest(ctx, query, variables)
	if err != nil {
		return nil, errors.ExternalError("failed to fetch issue children", err).WithDetail("parent_id", parentID)
	}

	var result struct {
		Issue struct {
			Children struct {
				Nodes []linearIssueResponse `json:"nodes"`
			} `json:"children"`
		} `json:"issue"`
	}

	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, errors.ExternalError("failed to unmarshal children data", err)
	}

	children := make([]*issue.Issue, len(result.Issue.Children.Nodes))
	for i, node := range result.Issue.Children.Nodes {
		children[i] = r.convertIssueFromResponse(node)
		children[i].HasChildren = len(node.Children.Nodes) > 0
		children[i].Expanded = false
	}

	return children, nil
}

// CreateSubtask creates a new subtask under a parent issue
func (r *Repository) CreateSubtask(ctx context.Context, parentID, title string) (*issue.Issue, error) {
	// First, get the parent issue to extract teamId and current user
	parentQuery := `
		query($issueId: String!) {
			issue(id: $issueId) {
				id
				team {
					id
				}
			}
			viewer {
				id
			}
		}
	`
	
	parentVars := map[string]interface{}{
		"issueId": parentID,
	}
	
	parentResp, err := r.makeRequest(ctx, parentQuery, parentVars)
	if err != nil {
		return nil, errors.ExternalError("failed to get parent issue for subtask creation", err).WithDetail("parent_id", parentID)
	}
	
	var parentResult struct {
		Issue struct {
			ID   string `json:"id"`
			Team struct {
				ID string `json:"id"`
			} `json:"team"`
		} `json:"issue"`
		Viewer struct {
			ID string `json:"id"`
		} `json:"viewer"`
	}
	
	if err := json.Unmarshal(parentResp.Data, &parentResult); err != nil {
		return nil, errors.ExternalError("failed to unmarshal parent issue data", err)
	}

	// Create the subtask
	createQuery := `
		mutation($parentId: String!, $title: String!, $teamId: String!, $assigneeId: String!) {
			issueCreate(input: {
				title: $title
				parentId: $parentId
				teamId: $teamId
				assigneeId: $assigneeId
			}) {
				success
				issue {
					id
					title
					description
					identifier
					url
					priority
					createdAt
					updatedAt
					state {
						id
						name
						type
					}
					assignee {
						id
						name
						displayName
						email
					}
					children {
						nodes {
							id
						}
					}
				}
			}
		}
	`

	createVars := map[string]interface{}{
		"parentId":   parentID,
		"title":      title,
		"teamId":     parentResult.Issue.Team.ID,
		"assigneeId": parentResult.Viewer.ID,
	}

	resp, err := r.makeRequest(ctx, createQuery, createVars)
	if err != nil {
		return nil, errors.ExternalError("failed to create subtask", err).WithDetail("parent_id", parentID)
	}

	var result struct {
		IssueCreate struct {
			Success bool                 `json:"success"`
			Issue   linearIssueResponse  `json:"issue"`
		} `json:"issueCreate"`
	}

	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, errors.ExternalError("failed to unmarshal subtask creation response", err)
	}

	if !result.IssueCreate.Success {
		return nil, errors.ExternalError("subtask creation was not successful", nil)
	}

	createdIssue := r.convertIssueFromResponse(result.IssueCreate.Issue)
	createdIssue.HasChildren = false
	createdIssue.Expanded = false

	return createdIssue, nil
}

// SearchIssues searches for issues matching a query
func (r *Repository) SearchIssues(ctx context.Context, query string) ([]*issue.Issue, error) {
	// For now, we'll fetch all assigned issues and do client-side filtering
	// Linear's search API is more complex and would require additional setup
	allIssues, err := r.GetAssignedIssues(ctx)
	if err != nil {
		return nil, err
	}
	
	if query == "" {
		return allIssues, nil
	}
	
	var filtered []*issue.Issue
	lowerQuery := strings.ToLower(query)
	
	for _, iss := range allIssues {
		if strings.Contains(strings.ToLower(iss.Identifier), lowerQuery) ||
		   strings.Contains(strings.ToLower(iss.Title), lowerQuery) ||
		   strings.Contains(strings.ToLower(iss.Description), lowerQuery) {
			filtered = append(filtered, iss)
		}
	}
	
	return filtered, nil
}

// TestConnection validates the connection to the Linear API
func (r *Repository) TestConnection(ctx context.Context) error {
	query := `
		query {
			viewer {
				id
				name
			}
		}
	`

	_, err := r.makeRequest(ctx, query, nil)
	if err != nil {
		return errors.ExternalError("failed to connect to Linear API", err)
	}

	return nil
}

// GetCurrentUser returns information about the current user (for testing connection)
func (r *Repository) GetCurrentUser(ctx context.Context) (*issue.User, error) {
	query := `
		query {
			viewer {
				id
				name
				displayName
				email
			}
		}
	`

	resp, err := r.makeRequest(ctx, query, nil)
	if err != nil {
		return nil, errors.ExternalError("failed to get current user", err)
	}

	var result struct {
		Viewer linearUserResponse `json:"viewer"`
	}

	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, errors.ExternalError("failed to unmarshal user data", err)
	}

	return r.convertUserFromResponse(result.Viewer), nil
}

// makeRequest makes a GraphQL request to the Linear API
func (r *Repository) makeRequest(ctx context.Context, query string, variables interface{}) (*graphQLResponse, error) {
	req := graphQLRequest{
		Query:     query,
		Variables: variables,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, errors.InternalError("failed to marshal GraphQL request", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", APIEndpoint, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, errors.InternalError("failed to create HTTP request", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", r.apiKey)

	resp, err := r.httpClient.Do(httpReq)
	if err != nil {
		return nil, errors.ExternalError("HTTP request failed", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, errors.ExternalError("Linear API request failed", nil).
			WithDetail("status_code", resp.StatusCode).
			WithDetail("response_body", string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.ExternalError("failed to read response body", err)
	}

	var gqlResp graphQLResponse
	if err := json.Unmarshal(body, &gqlResp); err != nil {
		return nil, errors.ExternalError("failed to unmarshal GraphQL response", err)
	}

	if len(gqlResp.Errors) > 0 {
		return nil, errors.ExternalError("GraphQL errors returned", nil).
			WithDetail("errors", gqlResp.Errors)
	}

	return &gqlResp, nil
}

// convertIssueFromResponse converts a Linear API response to a domain issue
func (r *Repository) convertIssueFromResponse(resp linearIssueResponse) *issue.Issue {
	iss := issue.NewIssue(resp.ID, resp.Identifier, resp.Title)
	iss.Description = resp.Description
	iss.URL = resp.URL
	iss.Priority = issue.Priority(resp.Priority)
	
	// Parse timestamps
	if createdAt, err := time.Parse(time.RFC3339, resp.CreatedAt); err == nil {
		iss.CreatedAt = createdAt
	}
	if updatedAt, err := time.Parse(time.RFC3339, resp.UpdatedAt); err == nil {
		iss.UpdatedAt = updatedAt
	}
	
	// Convert status
	iss.Status = issue.Status{
		ID:   resp.State.ID,
		Name: resp.State.Name,
		Type: r.convertStatusType(resp.State.Type),
	}
	
	// Convert assignee
	if resp.Assignee != nil {
		iss.Assignee = r.convertUserFromResponse(*resp.Assignee)
	}
	
	return iss
}

// convertUserFromResponse converts a Linear API user response to a domain user
func (r *Repository) convertUserFromResponse(resp linearUserResponse) *issue.User {
	return &issue.User{
		ID:          resp.ID,
		Name:        resp.Name,
		DisplayName: resp.DisplayName,
		Email:       resp.Email,
	}
}

// convertStatusType converts a Linear status type to a domain status type
func (r *Repository) convertStatusType(linearType string) issue.StatusType {
	switch strings.ToLower(linearType) {
	case "backlog":
		return issue.StatusTypeBacklog
	case "unstarted", "started", "in_progress":
		return issue.StatusTypeActive
	case "completed", "done":
		return issue.StatusTypeCompleted
	case "cancelled", "canceled":
		return issue.StatusTypeCancelled
	default:
		return issue.StatusTypeBacklog
	}
}

// GraphQL types and response structures
type graphQLRequest struct {
	Query     string      `json:"query"`
	Variables interface{} `json:"variables,omitempty"`
}

type graphQLResponse struct {
	Data   json.RawMessage   `json:"data"`
	Errors []graphQLError    `json:"errors,omitempty"`
}

type graphQLError struct {
	Message string `json:"message"`
	Path    []any  `json:"path,omitempty"`
}

type linearIssueResponse struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Identifier  string `json:"identifier"`
	URL         string `json:"url"`
	Priority    int    `json:"priority"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
	State       struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"state"`
	Assignee *linearUserResponse `json:"assignee,omitempty"`
	Children struct {
		Nodes []struct {
			ID string `json:"id"`
		} `json:"nodes"`
	} `json:"children"`
}

type linearUserResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
}