package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateRepositoryRelativeSymlinkTarget(t *testing.T) {
	skipOnWindows(t)

	base := t.TempDir()
	repoDir := filepath.Join(base, "repo")
	linkDir := filepath.Join(base, "links")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(linkDir, 0o755); err != nil {
		t.Fatal(err)
	}

	lnk := NewLnk(WithRepoPath(repoDir))

	// 链接路径（临时目录在 HOME 下，会被镜像映射进仓库）
	link := filepath.Join(linkDir, "env.sh")
	repoFilePath := lnk.getRepoFilePath(link)
	if err := os.MkdirAll(filepath.Dir(repoFilePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(repoFilePath, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 相对符号链接：从链接所在目录解析等于仓库内镜像文件
	rel, err := filepath.Rel(linkDir, repoFilePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(rel, link); err != nil {
		t.Fatal(err)
	}

	// 写入跟踪文件
	if err := lnk.writeTrackingEntries([]TrackedEntry{{Path: link, Type: LinkTypeSoft}}); err != nil {
		t.Fatal(err)
	}

	// 修复前：相对目标被按进程工作目录解析，校验必然失败
	if err := lnk.ValidateRepository(); err != nil {
		t.Errorf("ValidateRepository 对有效相对符号链接不应报错: %v", err)
	}
}
