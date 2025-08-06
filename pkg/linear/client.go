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
			Nodes []Issue `json:"nodes"`
		} `json:"issues"`
	}

	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal issues data: %w", err)
	}

	return result.Issues.Nodes, nil
}

// TestConnection tests the connection to Linear API and returns basic info
func (c *Client) TestConnection() error {
	_, err := c.GetCurrentUser()
	return err
}

// GetBranchName generates a branch name from an issue
func (i *Issue) GetBranchName() string {
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