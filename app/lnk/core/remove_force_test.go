package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestRemoveForceMultipleCommitsOnce(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test")

	lnk := NewLnk(WithRepoPath(repoDir))
	workDir := t.TempDir()
	targetA := filepath.Join(workDir, "a.conf")
	targetB := filepath.Join(workDir, "b.conf")
	repoA := filepath.Join(repoDir, filepath.Base(targetA))
	repoB := filepath.Join(repoDir, filepath.Base(targetB))

	writeFile(t, repoA, "a")
	writeFile(t, repoB, "b")
	createSymlink(t, repoA, targetA)
	createSymlink(t, repoB, targetB)

	tracking := fmt.Sprintf("%s|%s\n%s|%s\n", targetA, LinkTypeSoft, targetB, LinkTypeSoft)
	writeFile(t, filepath.Join(repoDir, TrackFilename), tracking)

	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "init")
	before := gitCommitCount(t, repoDir)

	if err := lnk.RemoveForceMultiple([]string{targetA, targetB}); err != nil {
		t.Fatalf("RemoveForceMultiple failed: %v", err)
	}

	after := gitCommitCount(t, repoDir)
	if after-before != 1 {
		t.Fatalf("expected exactly one commit, got delta=%d", after-before)
	}

	if msg := runGit(t, repoDir, "log", "-1", "--format=%s"); msg != "lnk: 强制移除 2 个文件" {
		t.Fatalf("unexpected commit message: %q", msg)
	}
	assertNotExists(t, targetA)
	assertNotExists(t, targetB)
	assertNotExists(t, repoA)
	assertNotExists(t, repoB)

	trackingContent, err := os.ReadFile(filepath.Join(repoDir, TrackFilename))
	if err != nil {
		t.Fatalf("read tracking file failed: %v", err)
	}
	if strings.TrimSpace(string(trackingContent)) != "" {
		t.Fatalf("expected tracking file to be empty, got %q", strings.TrimSpace(string(trackingContent)))
	}
}

func TestRemoveForceMultipleRollsBackOnFailure(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test")

	lnk := NewLnk(WithRepoPath(repoDir))
	workDir := t.TempDir()
	targetA := filepath.Join(workDir, "a.conf")
	targetB := filepath.Join(workDir, "b.conf")
	repoA := filepath.Join(repoDir, filepath.Base(targetA))
	repoB := filepath.Join(repoDir, filepath.Base(targetB))

	writeFile(t, repoA, "a")
	if err := os.MkdirAll(repoB, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	createSymlink(t, repoA, targetA)
	createSymlink(t, repoB, targetB)

	tracking := fmt.Sprintf("%s|%s\n%s|%s\n", targetA, LinkTypeSoft, targetB, LinkTypeSoft)
	writeFile(t, filepath.Join(repoDir, TrackFilename), tracking)

	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "init")
	before := gitCommitCount(t, repoDir)

	err := lnk.RemoveForceMultiple([]string{targetA, targetB})
	if err == nil || !strings.Contains(err.Error(), "目录") {
		t.Fatalf("expected directory error, got %v", err)
	}

	after := gitCommitCount(t, repoDir)
	if after != before {
		t.Fatalf("expected no new commit after rollback, got before=%d after=%d", before, after)
	}

	assertExists(t, repoA)
	assertExists(t, repoB)
	assertSymlinkTarget(t, targetA, repoA)
	assertSymlinkTarget(t, targetB, repoB)

	trackingContent, readErr := os.ReadFile(filepath.Join(repoDir, TrackFilename))
	if readErr != nil {
		t.Fatalf("read tracking file failed: %v", readErr)
	}
	text := strings.TrimSpace(string(trackingContent))
	if !strings.Contains(text, targetA+"|"+LinkTypeSoft) || !strings.Contains(text, targetB+"|"+LinkTypeSoft) {
		t.Fatalf("expected tracking file restored, got %q", text)
	}

	if status := runGit(t, repoDir, "status", "--porcelain"); status != "" {
		t.Fatalf("expected clean repo after rollback, got %q", status)
	}
}

func TestRemoveForceMultipleRestoresMissingFileCacheState(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test")

	lnk := NewLnk(WithRepoPath(repoDir))
	workDir := t.TempDir()
	targetA := filepath.Join(workDir, "a.conf")
	targetB := filepath.Join(workDir, "b.conf")
	targetC := filepath.Join(workDir, "c.conf")
	repoA := filepath.Join(repoDir, filepath.Base(targetA))
	repoB := filepath.Join(repoDir, filepath.Base(targetB))
	repoC := filepath.Join(repoDir, filepath.Base(targetC))

	writeFile(t, repoA, "a")
	writeFile(t, repoB, "b")
	if err := os.MkdirAll(repoC, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	createSymlink(t, repoA, targetA)
	createSymlink(t, repoB, targetB)
	createSymlink(t, repoC, targetC)

	tracking := fmt.Sprintf(
		"%s|%s\n%s|%s\n%s|%s\n",
		targetA, LinkTypeSoft,
		targetB, LinkTypeSoft,
		targetC, LinkTypeSoft,
	)
	writeFile(t, filepath.Join(repoDir, TrackFilename), tracking)

	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "init")
	before := gitCommitCount(t, repoDir)

	if err := os.Remove(repoB); err != nil {
		t.Fatalf("remove repo file failed: %v", err)
	}
	statusBefore := runGitRaw(t, repoDir, "status", "--porcelain")
	if !strings.Contains(statusBefore, " D "+filepath.Base(repoB)) {
		t.Fatalf("expected unstaged delete before remove-force, got %q", statusBefore)
	}

	err := lnk.RemoveForceMultiple([]string{targetA, targetB, targetC})
	if err == nil || !strings.Contains(err.Error(), "目录") {
		t.Fatalf("expected directory error, got %v", err)
	}

	if after := gitCommitCount(t, repoDir); after != before {
		t.Fatalf("expected no new commit after rollback, got before=%d after=%d", before, after)
	}

	statusAfter := runGitRaw(t, repoDir, "status", "--porcelain")
	if !strings.Contains(statusAfter, " D "+filepath.Base(repoB)) {
		t.Fatalf("expected unstaged delete restored, got %q", statusAfter)
	}
	if strings.Contains(statusAfter, "D  "+filepath.Base(repoB)) {
		t.Fatalf("expected no staged delete for missing file, got %q", statusAfter)
	}
	if cached := runGit(t, repoDir, "diff", "--cached", "--name-only"); cached != "" {
		t.Fatalf("expected empty staged diff after rollback, got %q", cached)
	}
}

func TestRemoveForceMultipleRejectsSymlinkEscape(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test")

	lnk := NewLnk(WithRepoPath(repoDir))
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "evil.conf")
	writeFile(t, outsideFile, "evil")

	linkDir := filepath.Join(repoDir, "escape")
	if err := os.Symlink(outsideDir, linkDir); err != nil {
		t.Fatalf("create repo symlink dir failed: %v", err)
	}

	trackKey := filepath.Join("escape", "evil.conf")
	tracking := fmt.Sprintf("%s|%s\n", trackKey, LinkTypeSoft)
	writeFile(t, filepath.Join(repoDir, TrackFilename), tracking)

	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "init")

	err := lnk.RemoveForceMultiple([]string{trackKey})
	if err == nil || !strings.Contains(err.Error(), "越界路径") {
		t.Fatalf("expected escape path error, got %v", err)
	}

	assertExists(t, outsideFile)
	if cached := runGit(t, repoDir, "diff", "--cached", "--name-only"); cached != "" {
		t.Fatalf("expected no staged changes on reject, got %q", cached)
	}
}

func gitCommitCount(t *testing.T, repoDir string) int {
	t.Helper()
	output := runGit(t, repoDir, "rev-list", "--count", "HEAD")
	count, err := strconv.Atoi(strings.TrimSpace(output))
	if err != nil {
		t.Fatalf("parse commit count failed: %v", err)
	}
	return count
}

func runGit(t *testing.T, repoDir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repoDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %s", args, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output))
}

func runGitRaw(t *testing.T, repoDir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repoDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %s", args, strings.TrimSpace(string(output)))
	}
	return strings.TrimRight(string(output), "\n")
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
}

func createSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("create symlink failed: %v", err)
	}
}

func assertNotExists(t *testing.T, path string) {
	t.Helper()
	_, err := os.Lstat(path)
	if !os.IsNotExist(err) {
		t.Fatalf("expected %s to be removed, err=%v", path, err)
	}
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); err != nil {
		t.Fatalf("expected %s to exist, err=%v", path, err)
	}
}

func assertSymlinkTarget(t *testing.T, linkPath, target string) {
	t.Helper()
	got, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("read symlink failed: %v", err)
	}
	if got != target {
		t.Fatalf("symlink target mismatch: got=%s want=%s", got, target)
	}
}
