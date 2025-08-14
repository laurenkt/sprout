package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/yosuke-furukawa/json5/encoding/json5"
	"sprout/pkg/shared/errors"
)

// Config represents the application configuration
type Config struct {
	DefaultCommand  string              `json:"defaultCommand,omitempty"`
	LinearAPIKey    string              `json:"linearApiKey,omitempty"`
	SparseCheckout  map[string][]string `json:"sparseCheckout,omitempty"`
	LogLevel        string              `json:"logLevel,omitempty"`
	LogOutput       string              `json:"logOutput,omitempty"`
}

// Repository defines the interface for configuration management
type Repository interface {
	// Load loads the configuration from the default location
	Load() (*Config, error)
	
	// Save saves the configuration to the default location
	Save(config *Config) error
	
	// GetPath returns the path to the configuration file
	GetPath() (string, error)
	
	// Exists returns true if the configuration file exists
	Exists() bool
}

// FileRepository provides file-based configuration storage
type FileRepository struct {
	configPath string
}

// NewFileRepository creates a new file-based configuration repository
func NewFileRepository() (*FileRepository, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, errors.ConfigurationError("failed to determine config path").WithCause(err)
	}
	
	return &FileRepository{
		configPath: configPath,
	}, nil
}

// Load loads the configuration from file
func (r *FileRepository) Load() (*Config, error) {
	config := DefaultConfig()

	if !r.Exists() {
		return config, nil
	}

	file, err := os.Open(r.configPath)
	if err != nil {
		return nil, errors.ConfigurationError("failed to open config file").
			WithCause(err).
			WithDetail("path", r.configPath)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, errors.ConfigurationError("failed to read config file").WithCause(err)
	}

	// First parse into a generic map to detect unknown keys
	var rawConfig map[string]interface{}
	if err := json5.Unmarshal(data, &rawConfig); err != nil {
		return nil, errors.ValidationError("invalid JSON5 format in config file").WithCause(err)
	}

	// Check for unknown keys
	if err := r.validateConfigKeys(rawConfig); err != nil {
		return nil, err
	}

	// Now parse into the actual config struct
	if err := json5.Unmarshal(data, config); err != nil {
		return nil, errors.ConfigurationError("failed to parse config structure").WithCause(err)
	}

	return config, nil
}

// Save saves the configuration to file
func (r *FileRepository) Save(config *Config) error {
	configDir := filepath.Dir(r.configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return errors.ConfigurationError("failed to create config directory").
			WithCause(err).
			WithDetail("directory", configDir)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return errors.InternalError("failed to marshal config", err)
	}

	if err := os.WriteFile(r.configPath, data, 0644); err != nil {
		return errors.ConfigurationError("failed to write config file").
			WithCause(err).
			WithDetail("path", r.configPath)
	}

	return nil
}

// GetPath returns the path to the configuration file
func (r *FileRepository) GetPath() (string, error) {
	return r.configPath, nil
}

// Exists returns true if the configuration file exists
func (r *FileRepository) Exists() bool {
	_, err := os.Stat(r.configPath)
	return err == nil
}

// validateConfigKeys checks for unknown configuration keys
func (r *FileRepository) validateConfigKeys(rawConfig map[string]interface{}) error {
	validKeys := map[string]bool{
		"defaultCommand": true,
		"linearApiKey":   true,
		"sparseCheckout": true,
		"logLevel":       true,
		"logOutput":      true,
	}

	var unknownKeys []string
	for key := range rawConfig {
		if !validKeys[key] {
			unknownKeys = append(unknownKeys, key)
		}
	}

	if len(unknownKeys) > 0 {
		return errors.ValidationError(fmt.Sprintf("unknown config keys found: %v", unknownKeys)).
			WithDetail("valid_keys", []string{"defaultCommand", "linearApiKey", "sparseCheckout", "logLevel", "logOutput"}).
			WithDetail("unknown_keys", unknownKeys)
	}

	return nil
}

// DefaultConfig returns a configuration with default values
func DefaultConfig() *Config {
	return &Config{
		DefaultCommand: "",
		LinearAPIKey:   "",
		SparseCheckout: make(map[string][]string),
		LogLevel:       "warn",
		LogOutput:      "stderr",
	}
}

// GetDefaultCommand parses and returns the default command as a slice
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

// GetLinearAPIKey returns the Linear API key
func (c *Config) GetLinearAPIKey() string {
	return c.LinearAPIKey
}

// GetSparseCheckoutDirectories returns sparse checkout directories for a repository
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

// IsLinearConfigured returns true if Linear integration is configured
func (c *Config) IsLinearConfigured() bool {
	return c.LinearAPIKey != ""
}

// GetLogLevel returns the configured log level
func (c *Config) GetLogLevel() string {
	if c.LogLevel == "" {
		return "warn"
	}
	return c.LogLevel
}

// GetLogOutput returns the configured log output destination
func (c *Config) GetLogOutput() string {
	if c.LogOutput == "" {
		return "stderr"
	}
	return c.LogOutput
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

// getConfigPath returns the path to the configuration file
func getConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".sprout.json5"), nil
}