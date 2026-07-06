package github

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type PR struct {
	State string `json:"state"`
	Title string `json:"title"`
}

type Client struct {
	repoRoot string
	runner   commandRunner
	cache    *PRStatusCache
}

type commandRunner func(dir string, name string, args ...string) ([]byte, error)

func NewClient(repoRoot string) *Client {
	return &Client{
		repoRoot: repoRoot,
		runner:   runCommandOutput,
		cache:    NewPRStatusCache(repoRoot),
	}
}

func NewClientWithRunner(repoRoot string, runner commandRunner) *Client {
	if runner == nil {
		runner = runCommandOutput
	}
	return &Client{
		repoRoot: repoRoot,
		runner:   runner,
		cache:    NewPRStatusCache(repoRoot),
	}
}

func NewClientWithRunnerAndCachePath(repoRoot string, runner commandRunner, cachePath string) *Client {
	return NewClientWithRunnerCachePathTTL(repoRoot, runner, cachePath, defaultPRStatusCacheTTL, defaultPRStatusCacheJitter)
}

func NewClientWithRunnerCachePathTTL(repoRoot string, runner commandRunner, cachePath string, ttl, jitter time.Duration) *Client {
	client := NewClientWithRunner(repoRoot, runner)
	client.cache = NewPRStatusCacheWithOptions(repoRoot, cachePath, ttl, jitter)
	return client
}

func runCommandOutput(dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd.Output()
}

func (c *Client) GetPRStatus(branchName string) string {
	if branchName == "" || branchName == "master" || branchName == "main" {
		return "-"
	}

	// Fast git-based check first
	status := c.checkBranchStatusWithGit(branchName)
	if status != "" {
		return status
	}

	// Fallback to gh command if git checks are inconclusive
	return c.checkPRStatusWithGH(branchName)
}

func (c *Client) checkBranchStatusWithGit(branchName string) string {
	// Check if remote tracking branch exists
	cmd := exec.Command("git", "rev-parse", "--verify", "origin/"+branchName)
	cmd.Dir = c.repoRoot
	if err := cmd.Run(); err != nil {
		// Remote branch doesn't exist - could be never pushed or merged and deleted
		// Only check for "Merged" if we have evidence the branch was previously pushed
		if c.wasBranchPushed(branchName) && c.isBranchMerged(branchName) {
			return "Merged"
		}
		return "No PR"
	}

	// Remote branch exists, check if it's ahead/behind
	return "" // Let gh command handle this case
}

func (c *Client) isBranchMerged(branchName string) bool {
	// Get the main branch name
	mainBranch := c.getMainBranch()
	if mainBranch == "" {
		return false
	}

	// Check if branch commits are in main branch history
	cmd := exec.Command("git", "merge-base", "--is-ancestor", branchName, mainBranch)
	cmd.Dir = c.repoRoot
	return cmd.Run() == nil
}

func (c *Client) getMainBranch() string {
	// Try to get default branch from remote
	cmd := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	cmd.Dir = c.repoRoot
	if output, err := cmd.Output(); err == nil {
		// Output format: refs/remotes/origin/main
		parts := strings.Split(strings.TrimSpace(string(output)), "/")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}

	// Fallback to common names
	for _, branch := range []string{"main", "master"} {
		cmd := exec.Command("git", "rev-parse", "--verify", "origin/"+branch)
		cmd.Dir = c.repoRoot
		if err := cmd.Run(); err == nil {
			return branch
		}
	}

	return "main" // Default fallback
}

func (c *Client) wasBranchPushed(branchName string) bool {
	// Check git reflog for evidence the branch was pushed
	cmd := exec.Command("git", "reflog", "--grep-reflog=origin/"+branchName, "--all", "--oneline")
	cmd.Dir = c.repoRoot
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// If we find any reflog entries mentioning origin/branchName, it was pushed
	return len(strings.TrimSpace(string(output))) > 0
}

func (c *Client) checkPRStatusWithGH(branchName string) string {
	status, err := c.GetPRStatusFromGH(branchName)
	if err != nil {
		return "-"
	}
	return status
}

func PRStatusCommand(branchName string) string {
	return fmt.Sprintf("gh pr list --head %s --state all --json state --limit 1", branchName)
}

// CachedPRStatus returns the cached PR status for a branch at a commit along
// with its freshness so callers can decide whether to serve it directly, serve
// it and refresh asynchronously, or fetch synchronously.
func (c *Client) CachedPRStatus(branchName, commit string) (string, CacheState) {
	if c.cache == nil {
		return "", CacheMiss
	}
	return c.cache.Lookup(branchName, commit)
}

// RememberPRStatus stores the result of a PR status check for later runs.
func (c *Client) RememberPRStatus(branchName, commit, status string) {
	if c.cache != nil {
		c.cache.Remember(branchName, commit, status)
	}
}

func (c *Client) GetPRStatusFromGH(branchName string) (string, error) {
	if branchName == "" || branchName == "master" || branchName == "main" {
		return "-", nil
	}

	output, err := c.runner(c.repoRoot, "gh", "pr", "list", "--head", branchName, "--state", "all", "--json", "state", "--limit", "1")
	if err != nil {
		return "", fmt.Errorf("%s: %w", PRStatusCommand(branchName), err)
	}

	var prs []PR
	if err := json.Unmarshal(output, &prs); err != nil {
		return "", fmt.Errorf("%s: %w", PRStatusCommand(branchName), err)
	}

	if len(prs) == 0 {
		return "No PR", nil
	}

	switch prs[0].State {
	case "OPEN":
		return "Open", nil
	case "MERGED":
		return "Merged", nil
	case "CLOSED":
		return "Closed", nil
	default:
		return prs[0].State, nil
	}
}

// CacheState describes how usable a cached PR status is.
type CacheState int

const (
	// CacheMiss means there is no usable entry (absent, for a different
	// commit, or empty); the caller must fetch synchronously.
	CacheMiss CacheState = iota
	// CacheFresh means the entry is within its TTL and can be served without
	// any network call.
	CacheFresh
	// CacheStale means the entry has expired; it can still be shown, but the
	// caller should refresh it asynchronously so the next run is fresh.
	CacheStale
)

const (
	defaultPRStatusCacheTTL    = 10 * time.Minute
	defaultPRStatusCacheJitter = 5 * time.Minute
)

type PRStatusCache struct {
	repoRoot string
	path     string
	ttl      time.Duration
	jitter   time.Duration
	mu       sync.Mutex
}

type prStatusEntry struct {
	Status    string    `json:"status"`
	Commit    string    `json:"commit"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type prStatusCacheFile struct {
	Repos map[string]map[string]prStatusEntry `json:"repos"`
}

func NewPRStatusCache(repoRoot string) *PRStatusCache {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil
	}
	return NewPRStatusCacheWithPath(repoRoot, filepath.Join(cacheDir, "sprout", "pr-status-cache.json"))
}

func NewPRStatusCacheWithPath(repoRoot, path string) *PRStatusCache {
	return NewPRStatusCacheWithOptions(repoRoot, path, defaultPRStatusCacheTTL, defaultPRStatusCacheJitter)
}

func NewPRStatusCacheWithOptions(repoRoot, path string, ttl, jitter time.Duration) *PRStatusCache {
	if path == "" {
		return nil
	}
	return &PRStatusCache{
		repoRoot: repoRoot,
		path:     path,
		ttl:      ttl,
		jitter:   jitter,
	}
}

// Lookup returns the cached PR status for a branch at a given commit along with
// its freshness. Entries recorded for a different commit are treated as a miss
// so that pushing new work always re-checks the PR.
func (c *PRStatusCache) Lookup(branchName, commit string) (string, CacheState) {
	if c == nil || branchName == "" || commit == "" {
		return "", CacheMiss
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	cacheFile, err := c.load()
	if err != nil {
		return "", CacheMiss
	}
	entry, ok := cacheFile.Repos[c.repoRoot][branchName]
	if !ok || entry.Commit != commit || entry.Status == "" {
		return "", CacheMiss
	}
	if time.Now().After(entry.ExpiresAt) {
		return entry.Status, CacheStale
	}
	return entry.Status, CacheFresh
}

// Remember stores the PR status for a branch at a commit, stamping it with a
// jittered expiry so entries written together do not all go stale on the same
// later run.
func (c *PRStatusCache) Remember(branchName, commit, status string) {
	if c == nil || branchName == "" || commit == "" || status == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	cacheFile, err := c.load()
	if err != nil {
		cacheFile = prStatusCacheFile{Repos: make(map[string]map[string]prStatusEntry)}
	}
	if cacheFile.Repos == nil {
		cacheFile.Repos = make(map[string]map[string]prStatusEntry)
	}
	if cacheFile.Repos[c.repoRoot] == nil {
		cacheFile.Repos[c.repoRoot] = make(map[string]prStatusEntry)
	}
	cacheFile.Repos[c.repoRoot][branchName] = prStatusEntry{
		Status:    status,
		Commit:    commit,
		ExpiresAt: time.Now().Add(c.entryLifetime()),
	}
	_ = c.save(cacheFile)
}

// entryLifetime is the base TTL plus a random jitter in [0, jitter).
func (c *PRStatusCache) entryLifetime() time.Duration {
	if c.jitter <= 0 {
		return c.ttl
	}
	return c.ttl + time.Duration(rand.Int63n(int64(c.jitter)))
}

func (c *PRStatusCache) load() (prStatusCacheFile, error) {
	cacheFile := prStatusCacheFile{Repos: make(map[string]map[string]prStatusEntry)}
	data, err := os.ReadFile(c.path)
	if err != nil {
		return cacheFile, err
	}
	if err := json.Unmarshal(data, &cacheFile); err != nil {
		return cacheFile, err
	}
	if cacheFile.Repos == nil {
		cacheFile.Repos = make(map[string]map[string]prStatusEntry)
	}
	return cacheFile, nil
}

func (c *PRStatusCache) save(cacheFile prStatusCacheFile) error {
	if err := os.MkdirAll(filepath.Dir(c.path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cacheFile, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, data, 0644)
}
