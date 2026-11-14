package core

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestConsolidateDirectoryDoesNotIgnoreWalkErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permission behavior differs on Windows")
	}

	root := t.TempDir()
	repoDir := t.TempDir()
	dir := filepath.Join(root, "configs")
	if err := os.MkdirAll(filepath.Join(dir, "unreadable"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.conf"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "unreadable"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(dir, "unreadable"), 0o755) })

	lnk := NewLnk(WithRepoPath(repoDir))
	err := lnk.consolidateDirectory(dir)
	if err == nil || !strings.Contains(err.Error(), "遍历目录失败") {
		t.Fatalf("expected a walk error, got %v", err)
	}
	assertExists(t, filepath.Join(dir, "app.conf"))
}

func TestCollectFilesRecursivelyDoesNotIgnoreWalkErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permission behavior differs on Windows")
	}

	root := t.TempDir()
	dir := filepath.Join(root, "configs")
	if err := os.MkdirAll(filepath.Join(dir, "unreadable"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.conf"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "unreadable"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(dir, "unreadable"), 0o755) })

	lnk := NewLnk(WithRepoPath(t.TempDir()))
	_, err := lnk.collectFilesRecursively(dir)
	if err == nil {
		t.Fatal("expected a walk error, got nil")
	}
	assertExists(t, filepath.Join(dir, "app.conf"))
}
