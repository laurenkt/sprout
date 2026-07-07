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
		cache:    NewPRStatusCache(repoIdentity(repoRoot)),
	}
}

func NewClientWithRunner(repoRoot string, runner commandRunner) *Client {
	if runner == nil {
		runner = runCommandOutput
	}
	return &Client{
		repoRoot: repoRoot,
		runner:   runner,
		cache:    NewPRStatusCache(repoIdentity(repoRoot)),
	}
}

func NewClientWithRunnerAndCachePath(repoRoot string, runner commandRunner, cachePath string) *Client {
	return NewClientWithRunnerCachePathTTL(repoRoot, runner, cachePath, defaultPRStatusCacheTTL, defaultPRStatusCacheJitter)
}

func NewClientWithRunnerCachePathTTL(repoRoot string, runner commandRunner, cachePath string, ttl, jitter time.Duration) *Client {
	client := NewClientWithRunner(repoRoot, runner)
	client.cache = NewPRStatusCacheWithOptions(repoIdentity(repoRoot), cachePath, ttl, jitter)
	return client
}

func runCommandOutput(dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd.Output()
}

// repoIdentity returns a stable cache key for a repository that is the same for
// its main checkout and every linked worktree. The git common directory (e.g.
// /path/to/repo/.git) is shared across all worktrees, whereas the working-tree
// root differs per worktree — keying on the latter would give each worktree its
// own cache. Falls back to repoRoot when the git dir cannot be resolved.
func repoIdentity(repoRoot string) string {
	cmd := exec.Command("git", "rev-parse", "--git-common-dir")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return repoRoot
	}
	dir := strings.TrimSpace(string(output))
	if dir == "" {
		return repoRoot
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(repoRoot, dir)
	}
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return dir
}

// GetPRStatus resolves a branch's PR status via GitHub. GitHub is the single
// source of truth for merge state: git-only heuristics (merge-base ancestry)
// miss squash-merges and PRs whose remote branch was deleted, which GitHub
// reports correctly.
func (c *Client) GetPRStatus(branchName string) string {
	if branchName == "" || branchName == "master" || branchName == "main" {
		return "-"
	}
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
