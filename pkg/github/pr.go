package github

import (
	"encoding/json"
	"os/exec"
)

type PR struct {
	State string `json:"state"`
	Title string `json:"title"`
}

type Client struct {
	repoRoot string
}

func NewClient(repoRoot string) *Client {
	return &Client{
		repoRoot: repoRoot,
	}
}

func (c *Client) GetPRStatus(branchName string) string {
	if branchName == "" || branchName == "master" || branchName == "main" {
		return "-"
	}
	
	cmd := exec.Command("gh", "pr", "list", "--head", branchName, "--json", "state", "--limit", "1")
	cmd.Dir = c.repoRoot
	
	output, err := cmd.Output()
	if err != nil {
		return "-"
	}
	
	var prs []PR
	if err := json.Unmarshal(output, &prs); err != nil {
		return "-"
	}
	
	if len(prs) == 0 {
		return "No PR"
	}
	
	switch prs[0].State {
	case "OPEN":
		return "Open"
	case "MERGED":
		return "Merged"
	case "CLOSED":
		return "Closed"
	default:
		return prs[0].State
	}
}