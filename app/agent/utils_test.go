package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromptNoCommitPush(t *testing.T) {
	required := "Do NOT commit or push changes"
	if !strings.Contains(PromptWorkflowContext, required) {
		t.Fatalf("PromptWorkflowContext missing required notice")
	}
	if !strings.Contains(PromptReviewerContext, required) {
		t.Fatalf("PromptReviewerContext missing required notice")
	}
}

func TestCheckHasChangesIgnoresDirtySubmodules(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	rootDir := t.TempDir()
	mainRepo := filepath.Join(rootDir, "main")
	subRepo := filepath.Join(rootDir, "sub")

	makeDir(t, mainRepo)
	makeDir(t, subRepo)

	runCmd(t, subRepo, "git", "init")
	runCmd(t, subRepo, "git", "config", "user.email", "test@example.com")
	runCmd(t, subRepo, "git", "config", "user.name", "Test")
	writeFile(t, filepath.Join(subRepo, "sub.txt"), "ok")
	runCmd(t, subRepo, "git", "add", ".")
	runCmd(t, subRepo, "git", "commit", "-m", "init")

	runCmd(t, mainRepo, "git", "init")
	runCmd(t, mainRepo, "git", "config", "user.email", "test@example.com")
	runCmd(t, mainRepo, "git", "config", "user.name", "Test")
	runCmd(t, mainRepo, "git", "-c", "protocol.file.allow=always", "submodule", "add", subRepo, "submodule")
	runCmd(t, mainRepo, "git", "commit", "-m", "add submodule")

	writeFile(t, filepath.Join(mainRepo, "submodule", "sub.txt"), "dirty")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})
	if err := os.Chdir(mainRepo); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}

	hasChanges, err := CheckHasChanges()
	if err != nil {
		t.Fatalf("CheckHasChanges failed: %v", err)
	}
	if hasChanges {
		t.Fatalf("expected no changes when only submodule is dirty")
	}

	writeFile(t, filepath.Join(mainRepo, "main.txt"), "changed")
	hasChanges, err = CheckHasChanges()
	if err != nil {
		t.Fatalf("CheckHasChanges failed: %v", err)
	}
	if !hasChanges {
		t.Fatalf("expected changes when main repo has untracked files")
	}
}

func TestCheckHasChangesDetectsUnstagedAndStaged(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	t.Run("unstaged", func(t *testing.T) {
		repo := setupRepo(t)
		writeFile(t, filepath.Join(repo, "file.txt"), "changed")

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatalf("getwd failed: %v", err)
		}
		t.Cleanup(func() {
			_ = os.Chdir(cwd)
		})
		if err := os.Chdir(repo); err != nil {
			t.Fatalf("chdir failed: %v", err)
		}

		hasChanges, err := CheckHasChanges()
		if err != nil {
			t.Fatalf("CheckHasChanges failed: %v", err)
		}
		if !hasChanges {
			t.Fatalf("expected changes for unstaged modifications")
		}
	})

	t.Run("staged", func(t *testing.T) {
		repo := setupRepo(t)
		writeFile(t, filepath.Join(repo, "file.txt"), "changed")
		runCmd(t, repo, "git", "add", "file.txt")

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatalf("getwd failed: %v", err)
		}
		t.Cleanup(func() {
			_ = os.Chdir(cwd)
		})
		if err := os.Chdir(repo); err != nil {
			t.Fatalf("chdir failed: %v", err)
		}

		hasChanges, err := CheckHasChanges()
		if err != nil {
			t.Fatalf("CheckHasChanges failed: %v", err)
		}
		if !hasChanges {
			t.Fatalf("expected changes for staged modifications")
		}
	})
}

func setupRepo(t *testing.T) string {
	rootDir := t.TempDir()
	repo := filepath.Join(rootDir, "repo")
	makeDir(t, repo)
	runCmd(t, repo, "git", "init")
	runCmd(t, repo, "git", "config", "user.email", "test@example.com")
	runCmd(t, repo, "git", "config", "user.name", "Test")
	writeFile(t, filepath.Join(repo, "file.txt"), "base")
	runCmd(t, repo, "git", "add", ".")
	runCmd(t, repo, "git", "commit", "-m", "init")
	return repo
}

func runCmd(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command failed: %s %v: %s", name, args, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output))
}

func makeDir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
}
