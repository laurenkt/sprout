package config

import (
	"path/filepath"
	"testing"
)

func TestGetDefaultCommand(t *testing.T) {
	tests := []struct {
		name           string
		defaultCommand string
		expected       []string
	}{
		{
			name:           "empty command",
			defaultCommand: "",
			expected:       nil,
		},
		{
			name:           "simple command",
			defaultCommand: "bash",
			expected:       []string{"bash"},
		},
		{
			name:           "command with args",
			defaultCommand: "npm run dev",
			expected:       []string{"npm", "run", "dev"},
		},
		{
			name:           "command with quoted string argument",
			defaultCommand: `run_thing "with a string arg"`,
			expected:       []string{"run_thing", "with a string arg"},
		},
		{
			name:           "command with single quotes",
			defaultCommand: `echo 'hello world'`,
			expected:       []string{"echo", "hello world"},
		},
		{
			name:           "command with multiple quoted arguments",
			defaultCommand: `script "first arg" "second arg" third`,
			expected:       []string{"script", "first arg", "second arg", "third"},
		},
		{
			name:           "command with escaped quotes",
			defaultCommand: `echo "hello \"world\""`,
			expected:       []string{"echo", `hello "world"`},
		},
		{
			name:           "complex command with flags and quoted args",
			defaultCommand: `docker run -it --name "my container" ubuntu:latest`,
			expected:       []string{"docker", "run", "-it", "--name", "my container", "ubuntu:latest"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				DefaultCommand: tt.defaultCommand,
			}

			result := cfg.GetDefaultCommand()

			if len(result) != len(tt.expected) {
				t.Errorf("GetDefaultCommand() returned %d args, expected %d", len(result), len(tt.expected))
				t.Errorf("Got: %v", result)
				t.Errorf("Expected: %v", tt.expected)
				return
			}

			for i, arg := range result {
				if arg != tt.expected[i] {
					t.Errorf("GetDefaultCommand() arg[%d] = %q, expected %q", i, arg, tt.expected[i])
				}
			}
		})
	}
}

func TestGetWorktreeBasePath(t *testing.T) {
	repoRoot := "/Users/test/sprout"
	repoName := "sprout"
	repoBasePath := filepath.Dir(repoRoot)

	cfg := &Config{
		WorktreeBasePath: "$REPO_BASEPATH/.worktrees/$REPO_NAME/$BRANCH_NAME",
	}

	path, includesBranch, ok := cfg.GetWorktreeBasePath(repoName, repoRoot, "feature-branch")
	if !ok {
		t.Fatalf("expected worktree base path to be found for global config")
	}
	if !includesBranch {
		t.Fatalf("expected global config to include branch variable")
	}

	expectedPath := filepath.Clean(filepath.Join(repoBasePath, ".worktrees", repoName, "feature-branch"))
	if path != expectedPath {
		t.Fatalf("expected path %s, got %s", expectedPath, path)
	}

	cfg = &Config{WorktreeBasePath: "/Users/test/.worktrees/global"}

	path, includesBranch, ok = cfg.GetWorktreeBasePath(repoName, repoRoot, "feature-branch")
	if !ok {
		t.Fatalf("expected worktree base path to be found for global config")
	}
	if includesBranch {
		t.Fatalf("did not expect global config to include branch variable")
	}

	if path != "/Users/test/.worktrees/global" {
		t.Fatalf("expected path /Users/test/.worktrees/global, got %s", path)
	}

	emptyCfg := &Config{}
	if _, _, ok := emptyCfg.GetWorktreeBasePath(repoName, repoRoot, "feature-branch"); ok {
		t.Fatalf("expected no base path for empty config")
	}
}

func TestGetWorktreeBasePathFallsBackToLegacyMap(t *testing.T) {
	cfg := &Config{
		WorktreeBasePaths: map[string]string{
			"sprout": "/Users/test/.worktrees/sprout-worktrees/",
		},
	}

	path, includesBranch, ok := cfg.GetWorktreeBasePath("sprout", "/Users/test/sprout", "feature-branch")
	if !ok {
		t.Fatalf("expected worktree base path to be found for repo name")
	}
	if includesBranch {
		t.Fatalf("did not expect legacy config to include branch variable")
	}

	expectedPath := filepath.Clean("/Users/test/.worktrees/sprout-worktrees/")
	if path != expectedPath {
		t.Fatalf("expected path %s, got %s", expectedPath, path)
	}
}
