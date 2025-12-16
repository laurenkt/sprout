package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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

func TestCreateBranchCreatesAndChecksOut(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sprout-branch-*")
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
	for key, value := range map[string]string{
		"user.email": "test@example.com",
		"user.name":  "Test User",
	} {
		cmd = exec.Command("git", "config", key, value)
		cmd.Dir = tempDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to configure git %s: %v", key, err)
		}
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

	baseBranch, err := wm.getBaseBranch()
	if err != nil {
		t.Fatalf("Failed to determine base branch: %v", err)
	}

	targetBranch := "feature/create-branch"

	if err := wm.CreateBranch(targetBranch); err != nil {
		t.Fatalf("CreateBranch returned error: %v", err)
	}

	// Verify we are on the new branch
	cmd = exec.Command("git", "branch", "--show-current")
	cmd.Dir = tempDir
	currentBranchBytes, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to get current branch: %v", err)
	}
	currentBranch := strings.TrimSpace(string(currentBranchBytes))
	if currentBranch != targetBranch {
		t.Fatalf("Expected to be on branch '%s', got '%s'", targetBranch, currentBranch)
	}

	// Verify the branch exists and matches the base commit
	cmd = exec.Command("git", "rev-parse", baseBranch)
	cmd.Dir = tempDir
	baseCommitBytes, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to get base branch commit: %v", err)
	}
	baseCommit := strings.TrimSpace(string(baseCommitBytes))

	cmd = exec.Command("git", "rev-parse", targetBranch)
	cmd.Dir = tempDir
	targetCommitBytes, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to get target branch commit: %v", err)
	}
	targetCommit := strings.TrimSpace(string(targetCommitBytes))

	if baseCommit != targetCommit {
		t.Fatalf("Expected new branch to start from %s, got %s", baseCommit, targetCommit)
	}
}
