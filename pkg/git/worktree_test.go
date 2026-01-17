package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"sprout/pkg/config"
)

func TestGetBaseBranch(t *testing.T) {
	// Create a temporary git repository for testing
	tempDir, err := os.MkdirTemp("", "sprout-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Configure git user for commits
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to configure git email: %v", err)
	}

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to configure git name: %v", err)
	}

	// Create initial commit
	testFile := filepath.Join(tempDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to add files: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	wm := &WorktreeManager{repoRoot: tempDir}

	t.Run("main branch exists", func(t *testing.T) {
		// Rename branch to main
		cmd := exec.Command("git", "branch", "-m", "main")
		cmd.Dir = tempDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to rename branch: %v", err)
		}

		branch, err := wm.getBaseBranch()
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if branch != "main" {
			t.Errorf("Expected 'main', got '%s'", branch)
		}
	})

	t.Run("master branch exists", func(t *testing.T) {
		// Rename branch to master
		cmd := exec.Command("git", "branch", "-m", "master")
		cmd.Dir = tempDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to rename branch: %v", err)
		}

		branch, err := wm.getBaseBranch()
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if branch != "master" {
			t.Errorf("Expected 'master', got '%s'", branch)
		}
	})

	t.Run("remote main exists", func(t *testing.T) {
		// Create a bare repo to act as remote
		remoteDir, err := os.MkdirTemp("", "sprout-remote-*")
		if err != nil {
			t.Fatalf("Failed to create remote dir: %v", err)
		}
		defer os.RemoveAll(remoteDir)

		cmd := exec.Command("git", "init", "--bare")
		cmd.Dir = remoteDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to init bare repo: %v", err)
		}

		// Add remote
		cmd = exec.Command("git", "remote", "add", "origin", remoteDir)
		cmd.Dir = tempDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to add remote: %v", err)
		}

		// Push current branch as main
		cmd = exec.Command("git", "push", "-u", "origin", "master:main")
		cmd.Dir = tempDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to push: %v", err)
		}

		// Delete local branch
		cmd = exec.Command("git", "checkout", "--detach")
		cmd.Dir = tempDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to detach HEAD: %v", err)
		}

		cmd = exec.Command("git", "branch", "-D", "master")
		cmd.Dir = tempDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to delete branch: %v", err)
		}

		branch, err := wm.getBaseBranch()
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if branch != "origin/main" {
			t.Errorf("Expected 'origin/main', got '%s'", branch)
		}
	})
}

func TestCreateWorktreeFromBase(t *testing.T) {
	// Create a temporary git repository for testing
	tempDir, err := os.MkdirTemp("", "sprout-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		// Clean up all temp directories
		os.RemoveAll(tempDir)
		worktreesDir := filepath.Join(filepath.Dir(tempDir), ".worktrees")
		os.RemoveAll(worktreesDir)
	}()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Configure git user for commits
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to configure git email: %v", err)
	}

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to configure git name: %v", err)
	}

	// Create initial commit on master
	testFile := filepath.Join(tempDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to add files: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Get the master commit hash
	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = tempDir
	masterCommitBytes, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to get master commit: %v", err)
	}
	masterCommit := strings.TrimSpace(string(masterCommitBytes))

	// Create another branch with a different commit
	cmd = exec.Command("git", "checkout", "-b", "feature-branch")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create feature branch: %v", err)
	}

	// Make another commit on feature branch
	if err := os.WriteFile(testFile, []byte("# Test\n\nFeature content"), 0644); err != nil {
		t.Fatalf("Failed to update test file: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to add files: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Feature commit")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Now create a worktree - it should be based on master, not feature-branch
	// Create a custom WorktreeManager that uses a test-specific worktree path
	testWorktreeDir := filepath.Join(tempDir, "test-worktrees")

	// Temporarily override the CreateWorktree method logic by testing the low-level function
	// We'll call createNormalWorktree directly with a test path
	wm := &WorktreeManager{repoRoot: tempDir}
	testWorktreePath := filepath.Join(testWorktreeDir, "test-worktree")

	// Create the test worktree directory
	if err := os.MkdirAll(testWorktreeDir, 0755); err != nil {
		t.Fatalf("Failed to create test worktree dir: %v", err)
	}

	worktreePath, err := wm.createNormalWorktree(testWorktreePath, "test-worktree")
	if err != nil {
		t.Fatalf("Failed to create worktree: %v", err)
	}

	// Check that the worktree is based on master commit
	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = worktreePath
	worktreeCommitBytes, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to get worktree commit: %v", err)
	}
	worktreeCommit := strings.TrimSpace(string(worktreeCommitBytes))

	if worktreeCommit != masterCommit {
		t.Errorf("Expected worktree to be based on master commit %s, but got %s", masterCommit, worktreeCommit)
	}

	// Verify the branch exists
	cmd = exec.Command("git", "branch", "--show-current")
	cmd.Dir = worktreePath
	branchBytes, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to get current branch: %v", err)
	}
	currentBranch := strings.TrimSpace(string(branchBytes))

	if currentBranch != "test-worktree" {
		t.Errorf("Expected branch to be 'test-worktree', got '%s'", currentBranch)
	}
}

func TestCreateBranch(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sprout-branch-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to configure git email: %v", err)
	}

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to configure git name: %v", err)
	}

	testFile := filepath.Join(tempDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Branch Test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to add files: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	wm := &WorktreeManager{repoRoot: tempDir}

	if err := wm.CreateBranch("Feature Branch!"); err != nil {
		t.Fatalf("CreateBranch returned error: %v", err)
	}

	cmd = exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/feature-branch")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Expected branch 'feature-branch' to exist: %v", err)
	}

	baseBranch, err := wm.getBaseBranch()
	if err != nil {
		t.Fatalf("Failed to determine base branch: %v", err)
	}

	cmd = exec.Command("git", "rev-parse", baseBranch)
	cmd.Dir = tempDir
	baseCommit, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to get base branch commit: %v", err)
	}

	cmd = exec.Command("git", "rev-parse", "feature-branch")
	cmd.Dir = tempDir
	featureCommit, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to get feature branch commit: %v", err)
	}

	if strings.TrimSpace(string(baseCommit)) != strings.TrimSpace(string(featureCommit)) {
		t.Fatalf("Expected feature branch to point to base branch commit")
	}

	if err := wm.CreateBranch("Feature Branch!"); err != nil {
		t.Fatalf("Expected CreateBranch to be idempotent, got error: %v", err)
	}
}

func TestCreateWorktreeUsesConfiguredBasePath(t *testing.T) {
	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "template excludes branch variable",
			template: filepath.Join("custom-worktrees", "$REPO_NAME"),
			expected: filepath.Join("custom-worktrees", "sprout", "feature-branch"),
		},
		{
			name:     "template includes branch variable",
			template: filepath.Join("custom-worktrees", "$REPO_NAME", "${BRANCH_NAME}-worktree"),
			expected: filepath.Join("custom-worktrees", "sprout", "feature-branch-worktree"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoRoot := initTestRepo(t)
			repoName := filepath.Base(repoRoot)

			customBase := t.TempDir()
			cfg := &config.Config{
				WorktreeBasePath: filepath.Join(customBase, tt.template),
			}

			wm := &WorktreeManager{
				repoRoot:     repoRoot,
				repoName:     repoName,
				configLoader: &config.DefaultLoader{Config: cfg},
			}

			worktreePath, err := wm.CreateWorktree("Feature Branch")
			if err != nil {
				t.Fatalf("Failed to create worktree: %v", err)
			}

			expectedPath := filepath.Join(customBase, strings.ReplaceAll(tt.expected, "sprout", repoName))
			if worktreePath != expectedPath {
				t.Fatalf("Expected worktree path %s, got %s", expectedPath, worktreePath)
			}

			if _, err := os.Stat(expectedPath); err != nil {
				t.Fatalf("Expected worktree directory to exist at %s: %v", expectedPath, err)
			}
		})
	}
}

func initTestRepo(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "sprout-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	t.Cleanup(func() {
		os.RemoveAll(tempDir)
		os.RemoveAll(filepath.Join(filepath.Dir(tempDir), ".worktrees"))
	})

	runGitCommand(t, tempDir, "init")
	runGitCommand(t, tempDir, "config", "user.email", "test@example.com")
	runGitCommand(t, tempDir, "config", "user.name", "Test User")

	testFile := filepath.Join(tempDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	runGitCommand(t, tempDir, "add", ".")
	runGitCommand(t, tempDir, "commit", "-m", "Initial commit")

	return tempDir
}

func runGitCommand(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir

	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to run git %v in %s: %v", strings.Join(args, " "), dir, err)
	}
}
