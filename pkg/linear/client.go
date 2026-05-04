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
	IsAddSubtask        bool   `json:"-"` // true if this is an "add subtask" placeholder
	SubtaskParentID     string `json:"-"` // ID of parent for new subtask
	EditingTitle        bool   `json:"-"` // true when editing this item's title
	TitleInput          string `json:"-"` // input buffer for title editing
	TitleCursor         int    `json:"-"` // cursor position in title input
	ShowingSubtaskEntry bool   `json:"-"` // true when showing inline subtask entry for this issue
	SubtaskEntryText    string `json:"-"` // text being entered for new subtask
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

// LinearClientInterface defines the methods needed for Linear API interaction
type LinearClientInterface interface {
	GetCurrentUser() (*User, error)
	GetAssignedIssues() ([]Issue, error)
	GetIssueChildren(issueID string) ([]Issue, error)
	CreateSubtask(parentID, title string) (*Issue, error)
	UnassignIssue(issueID string) error
	AssignIssueToMe(issueID string) error
	MarkIssueDone(issueID string) error
	TestConnection() error
}

// Client is a Linear API client
type Client struct {
	apiKey     string
	endpoint   string
	httpClient *http.Client
}

// NewClient creates a new Linear API client
func NewClient(apiKey string) *Client {
	return NewClientWithEndpoint(apiKey, APIEndpoint, &http.Client{
		Timeout: 30 * time.Second,
	})
}

// NewClientWithEndpoint creates a Linear API client for a specific GraphQL endpoint.
func NewClientWithEndpoint(apiKey, endpoint string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}
	return &Client{
		apiKey:     apiKey,
		endpoint:   endpoint,
		httpClient: httpClient,
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

	httpReq, err := http.NewRequest("POST", c.endpoint, bytes.NewBuffer(reqBody))
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
		messages := make([]string, 0, len(gqlResp.Errors))
		for _, gqlErr := range gqlResp.Errors {
			messages = append(messages, gqlErr.Message)
		}
		return nil, fmt.Errorf("%s", strings.Join(messages, "; "))
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
					parent {
						id
					}
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
				Parent *struct {
					ID string `json:"id"`
				} `json:"parent"`
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

	// First pass: collect all issues and build a map by ID, preserving order
	allIssues := make(map[string]Issue)
	issueParents := make(map[string]string) // childID -> parentID
	var issueOrder []string                 // preserve the order from API response

	for _, node := range result.Issues.Nodes {
		issue := node.Issue
		issue.HasChildren = len(node.Children.Nodes) > 0
		issue.Depth = 0
		issue.Expanded = false
		issue.Parent = nil

		allIssues[issue.ID] = issue
		issueOrder = append(issueOrder, issue.ID)

		// Track parent relationship if this issue has a parent
		if node.Parent != nil {
			issueParents[issue.ID] = node.Parent.ID
		}
	}

	// If an assigned child is folded under an assigned parent, keep the parent
	// discoverable and sort it by the freshest hidden child activity.
	for childID, parentID := range issueParents {
		child := allIssues[childID]
		parent := allIssues[parentID]
		if parent.ID == "" {
			continue
		}
		parent.HasChildren = true
		if child.UpdatedAt.After(parent.UpdatedAt) {
			parent.UpdatedAt = child.UpdatedAt
		}
		allIssues[parentID] = parent
	}

	// Second pass: filter out issues whose parents are also in the assigned list
	// Process in order to maintain consistent results
	var filteredIssues []Issue

	for _, issueID := range issueOrder {
		issue := allIssues[issueID]
		parentID, hasParent := issueParents[issueID]

		// If this issue has no parent, or its parent is not in our assigned issues,
		// then include it as a top-level issue
		if !hasParent || allIssues[parentID].ID == "" {
			filteredIssues = append(filteredIssues, issue)
		}
		// Otherwise, skip this issue as it will appear under its parent when expanded
	}

	return filteredIssues, nil
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

// UnassignIssue removes the assignee from an issue.
func (c *Client) UnassignIssue(issueID string) error {
	query := `
		mutation($issueId: String!) {
			issueUpdate(
				id: $issueId
				input: {
					assigneeId: null
				}
			) {
				success
			}
		}
	`

	variables := map[string]interface{}{
		"issueId": issueID,
	}

	resp, err := c.makeRequest(query, variables)
	if err != nil {
		return err
	}

	var result struct {
		IssueUpdate struct {
			Success bool `json:"success"`
		} `json:"issueUpdate"`
	}

	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return fmt.Errorf("failed to unmarshal issue unassign response: %w", err)
	}

	if !result.IssueUpdate.Success {
		return fmt.Errorf("failed to unassign issue")
	}

	return nil
}

// AssignIssueToMe assigns the issue to the current user.
func (c *Client) AssignIssueToMe(issueID string) error {
	viewer, err := c.GetCurrentUser()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}
	if viewer == nil || viewer.ID == "" {
		return fmt.Errorf("failed to get current user id")
	}

	query := `
		mutation($issueId: String!, $assigneeId: String!) {
			issueUpdate(
				id: $issueId
				input: {
					assigneeId: $assigneeId
				}
			) {
				success
			}
		}
	`

	variables := map[string]interface{}{
		"issueId":    issueID,
		"assigneeId": viewer.ID,
	}

	resp, err := c.makeRequest(query, variables)
	if err != nil {
		return err
	}

	var result struct {
		IssueUpdate struct {
			Success bool `json:"success"`
		} `json:"issueUpdate"`
	}

	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return fmt.Errorf("failed to unmarshal issue assign response: %w", err)
	}

	if !result.IssueUpdate.Success {
		return fmt.Errorf("failed to assign issue")
	}

	return nil
}

// MarkIssueDone moves an issue to a completed state.
func (c *Client) MarkIssueDone(issueID string) error {
	stateID, err := c.getCompletedStateID(issueID)
	if err != nil {
		return err
	}

	query := `
		mutation($issueId: String!, $stateId: String!) {
			issueUpdate(
				id: $issueId
				input: {
					stateId: $stateId
				}
			) {
				success
			}
		}
	`

	variables := map[string]interface{}{
		"issueId": issueID,
		"stateId": stateID,
	}

	resp, err := c.makeRequest(query, variables)
	if err != nil {
		return err
	}

	var result struct {
		IssueUpdate struct {
			Success bool `json:"success"`
		} `json:"issueUpdate"`
	}

	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return fmt.Errorf("failed to unmarshal issue done response: %w", err)
	}

	if !result.IssueUpdate.Success {
		return fmt.Errorf("failed to mark issue as done")
	}

	return nil
}

func (c *Client) getCompletedStateID(issueID string) (string, error) {
	query := `
		query($issueId: String!) {
			issue(id: $issueId) {
				team {
					states(filter: { type: { eq: "completed" } }) {
						nodes {
							id
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
		return "", err
	}

	var result struct {
		Issue *struct {
			Team *struct {
				States struct {
					Nodes []struct {
						ID string `json:"id"`
					} `json:"nodes"`
				} `json:"states"`
			} `json:"team"`
		} `json:"issue"`
	}

	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return "", fmt.Errorf("failed to unmarshal completed state lookup response: %w", err)
	}

	if result.Issue == nil || result.Issue.Team == nil || len(result.Issue.Team.States.Nodes) == 0 {
		return "", fmt.Errorf("no completed state found for issue")
	}

	return result.Issue.Team.States.Nodes[0].ID, nil
}

// TestConnection tests the connection to Linear API and returns basic info
func (c *Client) TestConnection() error {
	_, err := c.GetCurrentUser()
	return err
}

// NextVisible returns the next visible issue in the tree traversal order
func (i *Issue) NextVisible(roots []Issue) *Issue {
	// For add subtask placeholders, find the parent and get its next sibling
	if i.IsAddSubtask {
		parent := i.findParent(roots)
		if parent != nil {
			// Try to find next sibling of parent
			if nextSib := parent.NextSibling(roots); nextSib != nil {
				return nextSib
			}
			// If no next sibling, go up to parent's parent and try its next sibling
			current := parent.Parent
			for current != nil {
				if nextSib := current.NextSibling(roots); nextSib != nil {
					return nextSib
				}
				current = current.Parent
			}
		}
		return nil
	}

	// If this issue is expanded and has children, go to first child
	if i.Expanded && len(i.Children) > 0 {
		return &i.Children[0]
	}

	// Try to find next sibling
	if nextSib := i.NextSibling(roots); nextSib != nil {
		return nextSib
	}

	// Go up to parent and try its next sibling
	current := i.Parent
	for current != nil {
		if nextSib := current.NextSibling(roots); nextSib != nil {
			return nextSib
		}
		current = current.Parent
	}

	return nil // End of tree
}

// PrevVisible returns the previous visible issue in the tree traversal order
func (i *Issue) PrevVisible(roots []Issue) *Issue {
	// For "Add subtask" placeholders, go to the last child of parent if any, otherwise parent
	if i.IsAddSubtask {
		parent := i.findParent(roots)
		if parent != nil {
			if parent.Expanded && len(parent.Children) > 0 {
				// Go to last visible child of parent
				return parent.Children[len(parent.Children)-1].LastVisible()
			}
			return parent
		}
		return nil
	}

	// Try to find previous sibling
	if prevSib := i.prevSibling(roots); prevSib != nil {
		// Go to the last visible item under the previous sibling
		return prevSib.LastVisible()
	}

	// Go to parent
	return i.Parent
}

// findParent finds the parent issue by ID in the tree
func (i *Issue) findParent(roots []Issue) *Issue {
	if i.SubtaskParentID == "" {
		return nil
	}

	var find func(issues []Issue) *Issue
	find = func(issues []Issue) *Issue {
		for j := range issues {
			if issues[j].ID == i.SubtaskParentID {
				return &issues[j]
			}
			if found := find(issues[j].Children); found != nil {
				return found
			}
		}
		return nil
	}

	return find(roots)
}

// NextSibling finds the next sibling of this issue
func (i *Issue) NextSibling(roots []Issue) *Issue {
	if i.Parent != nil {
		// Look in parent's children
		for j, sibling := range i.Parent.Children {
			if sibling.ID == i.ID && j < len(i.Parent.Children)-1 {
				return &i.Parent.Children[j+1]
			}
		}
	} else {
		// Look in root issues
		for j, root := range roots {
			if root.ID == i.ID && j < len(roots)-1 {
				return &roots[j+1]
			}
		}
	}
	return nil
}

// prevSibling finds the previous sibling of this issue
func (i *Issue) prevSibling(roots []Issue) *Issue {
	if i.Parent != nil {
		// Look in parent's children
		for j, sibling := range i.Parent.Children {
			if sibling.ID == i.ID && j > 0 {
				return &i.Parent.Children[j-1]
			}
		}
	} else {
		// Look in root issues
		for j, root := range roots {
			if root.ID == i.ID && j > 0 {
				return &roots[j-1]
			}
		}
	}
	return nil
}

// LastVisible returns the last visible item in this subtree
func (i *Issue) LastVisible() *Issue {
	// If expanded and has children, return the last visible of the last child
	if i.Expanded && len(i.Children) > 0 {
		return i.Children[len(i.Children)-1].LastVisible()
	}

	// If not expanded or no children, this issue is the last visible
	return i
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
