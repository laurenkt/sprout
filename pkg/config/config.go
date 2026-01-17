package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/yosuke-furukawa/json5/encoding/json5"
)

type Config struct {
	DefaultCommand    string              `json:"defaultCommand,omitempty"`
	LinearAPIKey      string              `json:"linearApiKey,omitempty"`
	SparseCheckout    map[string][]string `json:"sparseCheckout,omitempty"`
	WorktreeBasePath  string              `json:"worktreeBasePath,omitempty"`
	WorktreeBasePaths map[string]string   `json:"worktreeBasePaths,omitempty"`
}

// LoaderInterface defines the interface for config loading
type LoaderInterface interface {
	GetConfig() (*Config, error)
}

// DefaultLoader implements LoaderInterface
type DefaultLoader struct {
	Config *Config
}

func (dl *DefaultLoader) GetConfig() (*Config, error) {
	return dl.Config, nil
}

// FileLoader loads configuration from disk using Load().
type FileLoader struct{}

func (fl *FileLoader) GetConfig() (*Config, error) {
	return Load()
}

func DefaultConfig() *Config {
	return &Config{
		DefaultCommand:    "",
		LinearAPIKey:      "",
		SparseCheckout:    make(map[string][]string),
		WorktreeBasePath:  "",
		WorktreeBasePaths: make(map[string]string),
	}
}

func Load() (*Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get config path: %w", err)
	}

	config := DefaultConfig()

	file, err := os.Open(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// First parse into a generic map to detect unknown keys
	var rawConfig map[string]interface{}
	if err := json5.Unmarshal(data, &rawConfig); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Check for unknown keys
	validKeys := map[string]bool{
		"defaultCommand":    true,
		"linearApiKey":      true,
		"sparseCheckout":    true,
		"worktreeBasePath":  true,
		"worktreeBasePaths": true,
	}

	var unknownKeys []string
	for key := range rawConfig {
		if !validKeys[key] {
			unknownKeys = append(unknownKeys, key)
		}
	}

	if len(unknownKeys) > 0 {
		return nil, fmt.Errorf("unknown config keys found: %v\n\nValid config keys are:\n  - defaultCommand: string (command to run by default in new worktrees)\n  - linearApiKey: string (API key for Linear integration)\n  - sparseCheckout: object (map of repository paths to directory arrays)\n  - worktreeBasePath: string (base worktree directory with optional variables)\n  - worktreeBasePaths: object (deprecated: map of repository names or paths to base worktree directories)", unknownKeys)
	}

	// Now parse into the actual config struct
	if err := json5.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}

func Save(config *Config) error {
	configPath, err := getConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func getConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".sprout.json5"), nil
}

func (c *Config) GetDefaultCommand() []string {
	if c.DefaultCommand == "" {
		return nil
	}

	parts := parseCommandLine(c.DefaultCommand)
	if len(parts) == 0 {
		return nil
	}

	return parts
}

// parseCommandLine parses a command line string respecting quotes
func parseCommandLine(command string) []string {
	var args []string
	var current strings.Builder
	var inQuote rune
	var escaped bool

	for _, r := range command {
		if escaped {
			// Handle escaped characters
			switch r {
			case '"', '\'', '\\':
				current.WriteRune(r)
			default:
				// For other escapes, keep the backslash
				current.WriteRune('\\')
				current.WriteRune(r)
			}
			escaped = false
			continue
		}

		if r == '\\' {
			escaped = true
			continue
		}

		if inQuote != 0 {
			// Inside quotes
			if r == inQuote {
				inQuote = 0
			} else {
				current.WriteRune(r)
			}
		} else {
			// Outside quotes
			switch r {
			case '"', '\'':
				inQuote = r
			case ' ', '\t', '\n':
				// Whitespace separates arguments
				if current.Len() > 0 {
					args = append(args, current.String())
					current.Reset()
				}
			default:
				current.WriteRune(r)
			}
		}
	}

	// Don't forget the last argument
	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}

func (c *Config) GetLinearAPIKey() string {
	return c.LinearAPIKey
}

func (c *Config) GetSparseCheckoutDirectories(repoPath string) ([]string, bool) {
	if c.SparseCheckout == nil {
		return nil, false
	}

	directories, exists := c.SparseCheckout[repoPath]
	if !exists || len(directories) == 0 {
		return nil, false
	}

	return directories, true
}

func (c *Config) GetWorktreeBasePath(repoName, repoRoot, branchName string) (string, bool, bool) {
	if c == nil {
		return "", false, false
	}

	if strings.TrimSpace(c.WorktreeBasePath) != "" {
		expanded := expandWorktreeBasePath(c.WorktreeBasePath, repoName, repoRoot, branchName)
		return filepath.Clean(expanded), containsBranchVariable(c.WorktreeBasePath), true
	}

	if c.WorktreeBasePaths == nil {
		return "", false, false
	}

	// Prefer repo name match, but allow full repo path override too.
	if basePath, ok := c.WorktreeBasePaths[repoName]; ok && strings.TrimSpace(basePath) != "" {
		return filepath.Clean(expandWorktreeBasePath(basePath, repoName, repoRoot, branchName)), containsBranchVariable(basePath), true
	}
	if basePath, ok := c.WorktreeBasePaths[repoRoot]; ok && strings.TrimSpace(basePath) != "" {
		return filepath.Clean(expandWorktreeBasePath(basePath, repoName, repoRoot, branchName)), containsBranchVariable(basePath), true
	}

	return "", false, false
}

func expandWorktreeBasePath(value, repoName, repoRoot, branchName string) string {
	repoBasePath := filepath.Dir(repoRoot)
	return os.Expand(value, func(key string) string {
		switch key {
		case "REPO_BASEPATH":
			return repoBasePath
		case "REPO_NAME":
			return repoName
		case "BRANCH_NAME":
			return branchName
		default:
			return os.Getenv(key)
		}
	})
}

func containsBranchVariable(value string) bool {
	return strings.Contains(value, "$BRANCH_NAME") || strings.Contains(value, "${BRANCH_NAME}")
}
