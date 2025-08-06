package linear

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	APIEndpoint = "https://api.linear.app/graphql"
)

// Issue represents a Linear issue/ticket
type Issue struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	State       State     `json:"state"`
	Assignee    *User     `json:"assignee"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	URL         string    `json:"url"`
	Identifier  string    `json:"identifier"`
	Priority    int       `json:"priority"`
	Children    []Issue   `json:"children,omitempty"`
	Parent      *Issue    `json:"parent,omitempty"`
	HasChildren bool      `json:"hasChildren"`
	Expanded    bool      `json:"expanded"`
	Depth       int       `json:"depth"`
	
	// UI state for inline subtask creation
	IsAddSubtask     bool   `json:"-"`        // true if this is an "add subtask" placeholder
	SubtaskParentID  string `json:"-"`        // ID of parent for new subtask
	EditingTitle     bool   `json:"-"`        // true when editing this item's title
	TitleInput       string `json:"-"`        // input buffer for title editing
	TitleCursor      int    `json:"-"`        // cursor position in title input
}

// State represents the state of an issue
type State struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// User represents a Linear user
type User struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
}

// Client is a Linear API client
type Client struct {
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new Linear API client
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GraphQLRequest represents a GraphQL request
type GraphQLRequest struct {
	Query     string      `json:"query"`
	Variables interface{} `json:"variables,omitempty"`
}

// GraphQLResponse represents a GraphQL response
type GraphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []GraphQLError  `json:"errors,omitempty"`
}

// GraphQLError represents a GraphQL error
type GraphQLError struct {
	Message string `json:"message"`
	Path    []any  `json:"path,omitempty"`
}

// makeRequest makes a GraphQL request to the Linear API
func (c *Client) makeRequest(query string, variables interface{}) (*GraphQLResponse, error) {
	req := GraphQLRequest{
		Query:     query,
		Variables: variables,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", APIEndpoint, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var gqlResp GraphQLResponse
	if err := json.Unmarshal(body, &gqlResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(gqlResp.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL errors: %v", gqlResp.Errors)
	}

	return &gqlResp, nil
}

// GetCurrentUser returns information about the current user
func (c *Client) GetCurrentUser() (*User, error) {
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

	resp, err := c.makeRequest(query, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Viewer User `json:"viewer"`
	}

	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user data: %w", err)
	}

	return &result.Viewer, nil
}

// GetAssignedIssues returns issues assigned to the current user
func (c *Client) GetAssignedIssues() ([]Issue, error) {
	query := `
		query {
			issues(
				filter: {
					assignee: { isMe: { eq: true } }
					state: { type: { neq: "completed" } }
				}
				orderBy: updatedAt
				first: 10
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

	resp, err := c.makeRequest(query, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Issues struct {
			Nodes []struct {
				Issue
				Children struct {
					Nodes []struct {
						ID string `json:"id"`
					} `json:"nodes"`
				} `json:"children"`
			} `json:"nodes"`
		} `json:"issues"`
	}

	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal issues data: %w", err)
	}

	issues := make([]Issue, len(result.Issues.Nodes))
	for i, node := range result.Issues.Nodes {
		issues[i] = node.Issue
		issues[i].HasChildren = len(node.Children.Nodes) > 0
		issues[i].Depth = 0
		issues[i].Expanded = false
	}

	return issues, nil
}

// GetIssueChildren fetches children/sub-issues for a given issue ID
func (c *Client) GetIssueChildren(issueID string) ([]Issue, error) {
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
		"issueId": issueID,
	}

	resp, err := c.makeRequest(query, variables)
	if err != nil {
		return nil, err
	}

	var result struct {
		Issue struct {
			Children struct {
				Nodes []struct {
					Issue
					Children struct {
						Nodes []struct {
							ID string `json:"id"`
						} `json:"nodes"`
					} `json:"children"`
				} `json:"nodes"`
			} `json:"children"`
		} `json:"issue"`
	}

	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal children data: %w", err)
	}

	children := make([]Issue, len(result.Issue.Children.Nodes))
	for i, node := range result.Issue.Children.Nodes {
		children[i] = node.Issue
		children[i].HasChildren = len(node.Children.Nodes) > 0
		children[i].Expanded = false
	}

	return children, nil
}

// CreateSubtask creates a new subtask under the given parent issue
func (c *Client) CreateSubtask(parentID, title string) (*Issue, error) {
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
	
	parentResp, err := c.makeRequest(parentQuery, parentVars)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent issue: %w", err)
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
		return nil, fmt.Errorf("failed to unmarshal parent issue data: %w", err)
	}

	// Now create the subtask with the correct teamId and assignee
	query := `
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

	variables := map[string]interface{}{
		"parentId":   parentID,
		"title":      title,
		"teamId":     parentResult.Issue.Team.ID,
		"assigneeId": parentResult.Viewer.ID,
	}

	resp, err := c.makeRequest(query, variables)
	if err != nil {
		return nil, err
	}

	var result struct {
		IssueCreate struct {
			Success bool `json:"success"`
			Issue   struct {
				ID          string `json:"id"`
				Title       string `json:"title"`
				Description string `json:"description,omitempty"`
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
				Assignee *struct {
					ID          string `json:"id"`
					Name        string `json:"name"`
					DisplayName string `json:"displayName"`
					Email       string `json:"email"`
				} `json:"assignee,omitempty"`
			} `json:"issue"`
		} `json:"issueCreate"`
	}

	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal subtask creation response: %w", err)
	}

	if !result.IssueCreate.Success {
		return nil, fmt.Errorf("failed to create subtask")
	}

	// Convert the response to our Issue struct
	createdAt, _ := time.Parse(time.RFC3339, result.IssueCreate.Issue.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339, result.IssueCreate.Issue.UpdatedAt)
	
	issue := &Issue{
		ID:          result.IssueCreate.Issue.ID,
		Title:       result.IssueCreate.Issue.Title,
		Description: result.IssueCreate.Issue.Description,
		Identifier:  result.IssueCreate.Issue.Identifier,
		URL:         result.IssueCreate.Issue.URL,
		Priority:    result.IssueCreate.Issue.Priority,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
		State: State{
			ID:   result.IssueCreate.Issue.State.ID,
			Name: result.IssueCreate.Issue.State.Name,
			Type: result.IssueCreate.Issue.State.Type,
		},
		HasChildren: false, // New subtask won't have children initially
		Expanded:    false,
		Depth:       0, // Will be set by the UI
	}
	
	// Convert assignee if present
	if result.IssueCreate.Issue.Assignee != nil {
		issue.Assignee = &User{
			ID:          result.IssueCreate.Issue.Assignee.ID,
			Name:        result.IssueCreate.Issue.Assignee.Name,
			DisplayName: result.IssueCreate.Issue.Assignee.DisplayName,
			Email:       result.IssueCreate.Issue.Assignee.Email,
		}
	}
	
	return issue, nil
}

// TestConnection tests the connection to Linear API and returns basic info
func (c *Client) TestConnection() error {
	_, err := c.GetCurrentUser()
	return err
}

// GetBranchName generates a branch name from an issue
func (i *Issue) GetBranchName() string {
	// Safety check for placeholder issues
	if i.Identifier == "" || i.IsAddSubtask {
		return "invalid-issue"
	}
	
	// Convert title to kebab-case, limit to reasonable length
	title := strings.ToLower(i.Title)
	title = strings.ReplaceAll(title, " ", "-")
	title = strings.ReplaceAll(title, "_", "-")
	
	// Remove special characters except hyphens
	var cleaned strings.Builder
	for _, r := range title {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			cleaned.WriteRune(r)
		}
	}
	title = cleaned.String()
	
	// Remove consecutive hyphens
	for strings.Contains(title, "--") {
		title = strings.ReplaceAll(title, "--", "-")
	}
	
	// Trim hyphens from start/end
	title = strings.Trim(title, "-")
	
	// Limit length (keeping identifier + reasonable title length)
	if len(title) > 50 {
		title = title[:50]
		title = strings.Trim(title, "-")
	}
	
	return fmt.Sprintf("%s-%s", strings.ToLower(i.Identifier), title)
}