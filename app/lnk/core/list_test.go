package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckSymlinkStatusRelativeTarget(t *testing.T) {
	skipOnWindows(t)

	base := t.TempDir()
	repoDir := filepath.Join(base, "repo")
	linkDir := filepath.Join(base, "links")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(linkDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// 仓库内真实文件
	targetFile := filepath.Join(repoDir, "env.sh")
	if err := os.WriteFile(targetFile, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	lnk := NewLnk(WithRepoPath(repoDir))

	// 相对符号链接（../repo/env.sh），从链接所在目录解析有效
	validLink := filepath.Join(linkDir, "valid.sh")
	if err := os.Symlink(filepath.Join("..", "repo", "env.sh"), validLink); err != nil {
		t.Fatal(err)
	}
	// 真实损坏的符号链接
	brokenLink := filepath.Join(linkDir, "broken.sh")
	if err := os.Symlink(filepath.Join("..", "repo", "not-exist"), brokenLink); err != nil {
		t.Fatal(err)
	}

	entries := []TrackedEntry{
		{Path: validLink, Type: LinkTypeSoft},
		{Path: brokenLink, Type: LinkTypeSoft},
	}

	broken := lnk.checkSymlinkStatus(entries)
	if len(broken) != 1 {
		t.Fatalf("应仅报告 1 个损坏链接，得到 %d 个: %v", len(broken), broken)
	}
	if broken[0] != brokenLink {
		t.Errorf("损坏链接应为 %q，得到 %q", brokenLink, broken[0])
	}
}
