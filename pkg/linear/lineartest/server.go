package lineartest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"sprout/pkg/linear"
)

type Server struct {
	t              testing.TB
	server         *httptest.Server
	schema         *ast.Schema
	issues         map[string]linear.Issue
	issueOrder     []string
	childrenMap    map[string][]string
	childFetchErrs map[string]error
	currentUser    *linear.User
	nextIssue      int
	Requests       []linear.GraphQLRequest
}

func NewServer(t testing.TB) *Server {
	t.Helper()

	s := &Server{
		t:              t,
		schema:         loadSchema(t),
		issues:         make(map[string]linear.Issue),
		issueOrder:     []string{},
		childrenMap:    make(map[string][]string),
		childFetchErrs: make(map[string]error),
		currentUser: &linear.User{
			ID:          "fake-user-id",
			Name:        "Test User",
			DisplayName: "Test User",
			Email:       "test@example.com",
		},
		nextIssue: 1000,
	}
	s.server = httptest.NewServer(http.HandlerFunc(s.handleGraphQL))
	t.Cleanup(s.server.Close)
	return s
}

func loadSchema(t testing.TB) *ast.Schema {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("locate lineartest server source")
	}
	schemaPath := filepath.Join(filepath.Dir(file), "..", "testdata", "linear_schema.graphqls")
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read test schema: %v", err)
	}
	return gqlparser.MustLoadSchema(&ast.Source{
		Name:  "linear_schema.graphqls",
		Input: string(data),
	})
}

func (s *Server) Client() *linear.Client {
	return linear.NewClientWithEndpoint("test-api-key", s.server.URL, s.server.Client())
}

func (s *Server) Endpoint() string {
	return s.server.URL
}

func (s *Server) AddIssue(issue linear.Issue, parentID string) {
	if issue.ID == "" {
		issue.ID = issue.Identifier
	}
	if issue.Identifier == "" {
		issue.Identifier = issue.ID
	}
	if issue.Assignee == nil {
		issue.Assignee = s.currentUser
	}
	if issue.URL == "" {
		issue.URL = "https://linear.local/" + issue.Identifier
	}
	if issue.Children == nil {
		issue.Children = []linear.Issue{}
	}

	if parentID != "" {
		parent := s.issues[parentID]
		issue.Parent = &linear.Issue{ID: parentID}
		if parent.ID != "" {
			issue.Parent.Identifier = parent.Identifier
		}
		s.childrenMap[parentID] = append(s.childrenMap[parentID], issue.ID)
		parent.HasChildren = true
		s.issues[parentID] = parent
	}

	s.issues[issue.ID] = issue
	s.issueOrder = append(s.issueOrder, issue.ID)
}

func (s *Server) FailChildFetch(issueID string, err error) {
	s.childFetchErrs[issueID] = err
}

func (s *Server) handleGraphQL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req linear.GraphQLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.Requests = append(s.Requests, req)

	if _, err := gqlparser.LoadQuery(s.schema, req.Query); err != nil {
		s.writeGraphQLError(w, err)
		return
	}

	if err := s.requestError(req); err != nil {
		s.writeGraphQLError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(linear.GraphQLResponse{
		Data: s.responseData(req),
	})
}

func (s *Server) requestError(req linear.GraphQLRequest) error {
	if !strings.Contains(req.Query, "children") || !strings.Contains(req.Query, "issue(id:") {
		return nil
	}
	issueID, _ := stringVariable(req, "issueId")
	return s.childFetchErrs[issueID]
}

func (s *Server) writeGraphQLError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(linear.GraphQLResponse{
		Errors: []linear.GraphQLError{{Message: err.Error()}},
	})
}

func (s *Server) responseData(req linear.GraphQLRequest) json.RawMessage {
	query := req.Query
	switch {
	case strings.Contains(query, "issues("):
		return rawJSON(`{"issues":{"nodes":` + mustJSON(s.assignedIssueNodes()) + `}}`)
	case strings.Contains(query, "issueCreate"):
		return rawJSON(`{"issueCreate":{"success":true,"issue":` + mustJSON(s.createIssue(req)) + `}}`)
	case strings.Contains(query, "issueUpdate"):
		s.updateIssue(req)
		return rawJSON(`{"issueUpdate":{"success":true}}`)
	case strings.Contains(query, "states("):
		return rawJSON(`{"issue":{"team":{"states":{"nodes":[{"id":"state-completed"}]}}}}`)
	case strings.Contains(query, "team") && strings.Contains(query, "viewer"):
		return rawJSON(`{"issue":{"id":` + quote(stringVarOrDefault(req, "issueId", "issue-1")) + `,"team":{"id":"team-1"}},"viewer":` + mustJSON(s.currentUser) + `}`)
	case strings.Contains(query, "children") && strings.Contains(query, "issue(id:"):
		issueID, _ := stringVariable(req, "issueId")
		return rawJSON(`{"issue":{"children":{"nodes":` + mustJSON(s.childNodes(issueID)) + `}}}`)
	case strings.Contains(query, "viewer"):
		return rawJSON(`{"viewer":` + mustJSON(s.currentUser) + `}`)
	default:
		s.t.Fatalf("unhandled GraphQL request:\n%s", query)
		return rawJSON(`{}`)
	}
}

func (s *Server) assignedIssueNodes() []map[string]any {
	issues := make([]linear.Issue, 0, len(s.issues))
	for _, issueID := range s.issueOrder {
		issue := s.issues[issueID]
		if issue.Assignee == nil || issue.Assignee.ID != s.currentUser.ID {
			continue
		}
		issues = append(issues, issue)
	}
	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].UpdatedAt.IsZero() && issues[j].UpdatedAt.IsZero() {
			return false
		}
		return issues[i].UpdatedAt.After(issues[j].UpdatedAt)
	})
	nodes := make([]map[string]any, 0, len(issues))
	for _, issue := range issues {
		nodes = append(nodes, s.issueNode(issue, true))
	}
	return nodes
}

func (s *Server) childNodes(parentID string) []map[string]any {
	childIDs := s.childrenMap[parentID]
	nodes := make([]map[string]any, 0, len(childIDs))
	for _, childID := range childIDs {
		nodes = append(nodes, s.issueNode(s.issues[childID], false))
	}
	return nodes
}

func (s *Server) issueNode(issue linear.Issue, includeParent bool) map[string]any {
	node := map[string]any{
		"id":          issue.ID,
		"title":       issue.Title,
		"description": issue.Description,
		"identifier":  issue.Identifier,
		"url":         issue.URL,
		"priority":    issue.Priority,
		"createdAt":   graphTime(issue.CreatedAt),
		"updatedAt":   graphTime(issue.UpdatedAt),
		"state":       issue.State,
		"assignee":    issue.Assignee,
		"children": map[string]any{
			"nodes": s.childIDNodes(issue.ID),
		},
	}
	if includeParent {
		if issue.Parent != nil && issue.Parent.ID != "" {
			node["parent"] = map[string]any{"id": issue.Parent.ID}
		} else {
			node["parent"] = nil
		}
	}
	return node
}

func (s *Server) childIDNodes(parentID string) []map[string]string {
	childIDs := s.childrenMap[parentID]
	nodes := make([]map[string]string, 0, len(childIDs))
	for _, childID := range childIDs {
		nodes = append(nodes, map[string]string{"id": childID})
	}
	return nodes
}

func (s *Server) createIssue(req linear.GraphQLRequest) map[string]any {
	parentID, _ := stringVariable(req, "parentId")
	title, _ := stringVariable(req, "title")
	s.nextIssue++
	identifier := fmt.Sprintf("TICK-%d", s.nextIssue)
	if parent := s.issues[parentID]; parent.Identifier != "" {
		prefix := strings.Split(parent.Identifier, "-")[0]
		identifier = fmt.Sprintf("%s-%d", prefix, s.nextIssue)
	}
	issue := linear.Issue{
		ID:          fmt.Sprintf("fake-subtask-%d", s.nextIssue),
		Identifier:  identifier,
		Title:       title,
		State:       linear.State{ID: "state-todo", Name: "Todo", Type: "unstarted"},
		Assignee:    s.currentUser,
		CreatedAt:   time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC),
		URL:         "https://linear.local/" + identifier,
		Children:    []linear.Issue{},
		HasChildren: false,
	}
	s.AddIssue(issue, parentID)
	return s.issueNode(issue, false)
}

func (s *Server) updateIssue(req linear.GraphQLRequest) {
	issueID, _ := stringVariable(req, "issueId")
	issue := s.issues[issueID]
	if strings.Contains(req.Query, "assigneeId: null") {
		issue.Assignee = nil
	} else if _, ok := stringVariable(req, "assigneeId"); ok {
		issue.Assignee = s.currentUser
	} else if _, ok := stringVariable(req, "stateId"); ok {
		issue.State = linear.State{ID: "state-completed", Name: "Done", Type: "completed"}
	}
	s.issues[issueID] = issue
}

func stringVarOrDefault(req linear.GraphQLRequest, key, fallback string) string {
	if value, ok := stringVariable(req, key); ok {
		return value
	}
	return fallback
}

func stringVariable(req linear.GraphQLRequest, key string) (string, bool) {
	vars, ok := req.Variables.(map[string]any)
	if !ok {
		return "", false
	}
	value, ok := vars[key].(string)
	return value, ok
}

func mustJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func graphTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value.Format(time.RFC3339)
}

func rawJSON(value string) json.RawMessage {
	return json.RawMessage(value)
}

func quote(value string) string {
	data, _ := json.Marshal(value)
	return string(data)
}
