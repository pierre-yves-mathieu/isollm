package barerepo

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGetMountPath(t *testing.T) {
	path, err := GetMountPath("my-project")
	if err != nil {
		t.Fatalf("GetMountPath failed: %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".isollm", "my-project.git")
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}

func TestCreateAndExists(t *testing.T) {
	// Create a temporary git repo
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	bareDir := filepath.Join(tmpDir, "project.git")

	// Initialize a git repo
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	cmd := exec.Command("git", "init")
	cmd.Dir = projectDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Create initial commit
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = projectDir
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = projectDir
	cmd.Run()

	testFile := filepath.Join(projectDir, "README.md")
	os.WriteFile(testFile, []byte("# Test"), 0644)

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = projectDir
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = projectDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Test that bare repo doesn't exist yet
	if Exists(bareDir) {
		t.Error("bare repo should not exist yet")
	}

	// Create bare repo
	repo, err := Create(projectDir, bareDir)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Test that it exists now
	if !Exists(bareDir) {
		t.Error("bare repo should exist after Create")
	}

	if repo.Path() != bareDir {
		t.Errorf("expected path %s, got %s", bareDir, repo.Path())
	}

	// Verify gc.auto is disabled
	cmd = exec.Command("git", "config", "gc.auto")
	cmd.Dir = bareDir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to check gc.auto: %v", err)
	}
	if string(output) != "0\n" {
		t.Errorf("gc.auto should be 0, got %q", output)
	}
}

func TestIsHostAhead(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	bareDir := filepath.Join(tmpDir, "project.git")

	// Set up test repo
	setupTestRepo(t, projectDir)

	// Create bare repo
	repo, err := Create(projectDir, bareDir)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Initially should be 0
	ahead, err := repo.IsHostAhead(projectDir, "master")
	if err != nil {
		t.Fatalf("IsHostAhead failed: %v", err)
	}
	if ahead != 0 {
		t.Errorf("expected 0 commits ahead, got %d", ahead)
	}

	// Make a commit in the project
	testFile := filepath.Join(projectDir, "new-file.txt")
	os.WriteFile(testFile, []byte("new content"), 0644)

	cmd := exec.Command("git", "add", ".")
	cmd.Dir = projectDir
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "new commit")
	cmd.Dir = projectDir
	cmd.Run()

	// Now should be 1 ahead
	ahead, err = repo.IsHostAhead(projectDir, "master")
	if err != nil {
		t.Fatalf("IsHostAhead failed: %v", err)
	}
	if ahead != 1 {
		t.Errorf("expected 1 commit ahead, got %d", ahead)
	}
}

func TestPushAndPull(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	bareDir := filepath.Join(tmpDir, "project.git")

	setupTestRepo(t, projectDir)

	repo, err := Create(projectDir, bareDir)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Make a commit in the project
	testFile := filepath.Join(projectDir, "new-file.txt")
	os.WriteFile(testFile, []byte("new content"), 0644)

	cmd := exec.Command("git", "add", ".")
	cmd.Dir = projectDir
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "new commit")
	cmd.Dir = projectDir
	cmd.Run()

	// Push to bare repo
	if err := repo.PushToBare(projectDir, "master"); err != nil {
		t.Fatalf("PushToBare failed: %v", err)
	}

	// Now should be 0 ahead
	ahead, err := repo.IsHostAhead(projectDir, "master")
	if err != nil {
		t.Fatalf("IsHostAhead failed: %v", err)
	}
	if ahead != 0 {
		t.Errorf("expected 0 commits ahead after push, got %d", ahead)
	}
}

func TestListTaskBranches(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	bareDir := filepath.Join(tmpDir, "project.git")

	setupTestRepo(t, projectDir)

	repo, err := Create(projectDir, bareDir)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Initially no task branches
	branches, err := repo.ListTaskBranches()
	if err != nil {
		t.Fatalf("ListTaskBranches failed: %v", err)
	}
	if len(branches) != 0 {
		t.Errorf("expected 0 branches, got %d", len(branches))
	}

	// Create a task branch in bare repo
	cmd := exec.Command("git", "branch", "isollm/ar-a1b2", "master")
	cmd.Dir = bareDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}

	// Now should have 1 task branch
	branches, err = repo.ListTaskBranches()
	if err != nil {
		t.Fatalf("ListTaskBranches failed: %v", err)
	}
	if len(branches) != 1 {
		t.Fatalf("expected 1 branch, got %d", len(branches))
	}

	if branches[0].Name != "isollm/ar-a1b2" {
		t.Errorf("expected branch name isollm/ar-a1b2, got %s", branches[0].Name)
	}
	if branches[0].TaskID != "ar-a1b2" {
		t.Errorf("expected task ID ar-a1b2, got %s", branches[0].TaskID)
	}
}

func TestDeleteBranch(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	bareDir := filepath.Join(tmpDir, "project.git")

	setupTestRepo(t, projectDir)

	repo, err := Create(projectDir, bareDir)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Create a task branch
	cmd := exec.Command("git", "branch", "isollm/ar-test", "master")
	cmd.Dir = bareDir
	cmd.Run()

	// Verify it exists
	branches, _ := repo.ListTaskBranches()
	if len(branches) != 1 {
		t.Fatalf("expected 1 branch, got %d", len(branches))
	}

	// Delete it
	if err := repo.DeleteBranch("isollm/ar-test"); err != nil {
		t.Fatalf("DeleteBranch failed: %v", err)
	}

	// Verify it's gone
	branches, _ = repo.ListTaskBranches()
	if len(branches) != 0 {
		t.Errorf("expected 0 branches after delete, got %d", len(branches))
	}
}

func setupTestRepo(t *testing.T, projectDir string) {
	t.Helper()

	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	cmd := exec.Command("git", "init")
	cmd.Dir = projectDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = projectDir
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = projectDir
	cmd.Run()

	testFile := filepath.Join(projectDir, "README.md")
	os.WriteFile(testFile, []byte("# Test"), 0644)

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = projectDir
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = projectDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}
}
