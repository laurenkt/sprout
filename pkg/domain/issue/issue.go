package issue

import (
	"fmt"
	"strings"
	"time"
)

// Issue represents a ticket or task from an external system (Linear, GitHub, etc.)
type Issue struct {
	ID          string
	Identifier  string // Human-readable ID like "SPR-123"
	Title       string
	Description string
	Status      Status
	Priority    Priority
	Assignee    *User
	Parent      *Issue
	Children    []*Issue
	Labels      []string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	URL         string
	
	// UI state (should probably be moved to presentation layer)
	HasChildren bool
	Expanded    bool
	Depth       int
}

// Status represents the current state of an issue
type Status struct {
	ID   string
	Name string
	Type StatusType
}

// StatusType categorizes issue statuses
type StatusType string

const (
	StatusTypeBacklog    StatusType = "backlog"
	StatusTypeActive     StatusType = "active"
	StatusTypeCompleted  StatusType = "completed"
	StatusTypeCancelled  StatusType = "cancelled"
)

// Priority represents the urgency/importance of an issue
type Priority int

const (
	PriorityNone Priority = iota
	PriorityLow
	PriorityMedium
	PriorityHigh
	PriorityUrgent
)

// User represents a person associated with an issue
type User struct {
	ID          string
	Name        string
	DisplayName string
	Email       string
	AvatarURL   string
}

// NewIssue creates a new issue instance
func NewIssue(id, identifier, title string) *Issue {
	return &Issue{
		ID:         id,
		Identifier: identifier,
		Title:      title,
		Priority:   PriorityNone,
		Children:   make([]*Issue, 0),
		Labels:     make([]string, 0),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

// IsSubtask returns true if this issue has a parent
func (i *Issue) IsSubtask() bool {
	return i.Parent != nil
}

// AddChild adds a child issue (subtask)
func (i *Issue) AddChild(child *Issue) {
	child.Parent = i
	child.Depth = i.Depth + 1
	i.Children = append(i.Children, child)
	i.HasChildren = true
}

// GetBranchName generates a git branch name from the issue
func (i *Issue) GetBranchName() string {
	if i.Identifier == "" {
		return "invalid-issue"
	}
	
	// Convert title to kebab-case
	title := strings.ToLower(i.Title)
	title = strings.ReplaceAll(title, " ", "-")
	title = strings.ReplaceAll(title, "_", "-")
	
	// Remove special characters
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
	
	// Trim and limit length
	title = strings.Trim(title, "-")
	if len(title) > 50 {
		title = title[:50]
		title = strings.Trim(title, "-")
	}
	
	return fmt.Sprintf("%s-%s", strings.ToLower(i.Identifier), title)
}

// String returns a string representation of the issue
func (i *Issue) String() string {
	if i.Identifier != "" {
		return fmt.Sprintf("%s: %s", i.Identifier, i.Title)
	}
	return i.Title
}