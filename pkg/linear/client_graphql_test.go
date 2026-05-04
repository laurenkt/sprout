package linear_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"sprout/pkg/linear"
	"sprout/pkg/linear/lineartest"
)

func TestGetIssueChildrenUsesValidGraphQL(t *testing.T) {
	api := lineartest.NewServer(t)
	addParentAndChild(api)
	client := api.Client()

	children, err := client.GetIssueChildren("TICK-1")
	if err != nil {
		t.Fatalf("GetIssueChildren returned error: %v", err)
	}

	if len(children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(children))
	}
	child := children[0]
	if child.ID != "TICK-2" || child.Identifier != "TICK-2" || child.Title != "Child Task" {
		t.Fatalf("unexpected child: %+v", child)
	}
	if child.HasChildren {
		t.Fatalf("expected child HasChildren=false")
	}
}

func TestLinearGraphQLHarnessRejectsInvalidSyntax(t *testing.T) {
	api := lineartest.NewServer(t)

	resp, err := http.Post(api.Endpoint(), "application/json", strings.NewReader(`{"query":"query { issue(id: \"TICK-1\") { children() { nodes { id } } } }"}`))
	if err != nil {
		t.Fatalf("post invalid query: %v", err)
	}
	defer resp.Body.Close()

	var gqlResp linear.GraphQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(gqlResp.Errors) == 0 {
		t.Fatalf("expected validation errors for invalid query")
	}
	if !strings.Contains(gqlResp.Errors[0].Message, "expected") {
		t.Fatalf("expected syntax error, got %q", gqlResp.Errors[0].Message)
	}
}

func TestLinearClientOperationsUseValidGraphQL(t *testing.T) {
	tests := []struct {
		name string
		run  func(*linear.Client) error
	}{
		{
			name: "GetCurrentUser",
			run: func(client *linear.Client) error {
				_, err := client.GetCurrentUser()
				return err
			},
		},
		{
			name: "GetAssignedIssues",
			run: func(client *linear.Client) error {
				_, err := client.GetAssignedIssues()
				return err
			},
		},
		{
			name: "GetIssueChildren",
			run: func(client *linear.Client) error {
				_, err := client.GetIssueChildren("TICK-1")
				return err
			},
		},
		{
			name: "CreateSubtask",
			run: func(client *linear.Client) error {
				_, err := client.CreateSubtask("TICK-1", "Created Subtask")
				return err
			},
		},
		{
			name: "UnassignIssue",
			run: func(client *linear.Client) error {
				return client.UnassignIssue("TICK-1")
			},
		},
		{
			name: "AssignIssueToMe",
			run: func(client *linear.Client) error {
				return client.AssignIssueToMe("TICK-1")
			},
		},
		{
			name: "MarkIssueDone",
			run: func(client *linear.Client) error {
				return client.MarkIssueDone("TICK-1")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			api := lineartest.NewServer(t)
			addParentAndChild(api)
			client := api.Client()
			if err := tc.run(client); err != nil {
				t.Fatalf("%s returned error: %v", tc.name, err)
			}
			if len(api.Requests) == 0 {
				t.Fatalf("expected at least one GraphQL request")
			}
		})
	}
}

func addParentAndChild(api *lineartest.Server) {
	api.AddIssue(linear.Issue{
		ID:         "TICK-1",
		Identifier: "TICK-1",
		Title:      "Parent Task",
		State:      linear.State{ID: "state-started", Name: "In Progress", Type: "started"},
	}, "")
	api.AddIssue(linear.Issue{
		ID:         "TICK-2",
		Identifier: "TICK-2",
		Title:      "Child Task",
		State:      linear.State{ID: "state-todo", Name: "Todo", Type: "unstarted"},
	}, "TICK-1")
}
