package core

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// 路径辅助函数在 Windows 上依赖 HOME 相关环境且 NewLnk 有管理员权限检查，
// 相关测试仅在类 Unix 平台运行。
func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("路径辅助测试仅在类 Unix 平台运行")
	}
}

func TestExpandPath(t *testing.T) {
	skipOnWindows(t)
	lnk := NewLnk()

	// 环境变量展开（目标路径不存在，避免符号链接干扰）
	dir := t.TempDir()
	nonExist := filepath.Join(dir, "not-exist-file")
	t.Setenv("ZZZ_TEST_PATH", nonExist)
	if got := lnk.expandPath("$ZZZ_TEST_PATH"); got != nonExist {
		t.Errorf("expandPath 环境变量展开 = %q, want %q", got, nonExist)
	}

	// ~/ 前缀展开到主目录
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("无法获取用户主目录")
	}
	wantHome := filepath.Join(home, "zzz-expand-not-exist")
	if got := lnk.expandPath("~/zzz-expand-not-exist"); got != wantHome {
		t.Errorf("expandPath ~ 展开 = %q, want %q", got, wantHome)
	}
}

func TestNormalizeFilePath(t *testing.T) {
	skipOnWindows(t)
	lnk := NewLnk()

	// 相对路径转为绝对路径（保留末段）
	got := lnk.normalizeFilePath("some/relative/file.txt")
	if !filepath.IsAbs(got) {
		t.Errorf("normalizeFilePath 相对路径应返回绝对路径，得到 %q", got)
	}
	if filepath.Base(got) != "file.txt" {
		t.Errorf("normalizeFilePath 应保留文件名，得到 %q", got)
	}

	// 已是绝对路径则保持（经 Clean）
	dir := t.TempDir()
	abs := filepath.Join(dir, "file.txt")
	if got := lnk.normalizeFilePath(abs); got != filepath.Clean(abs) {
		t.Errorf("normalizeFilePath 绝对路径 = %q, want %q", got, filepath.Clean(abs))
	}
}

func TestToTrackingPath(t *testing.T) {
	skipOnWindows(t)
	lnk := NewLnk()

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("无法获取用户主目录")
	}

	// 主目录下的路径转为主目录相对的跟踪键
	inHome := filepath.Join(home, "docs", "a.md")
	wantRel := filepath.Join("docs", "a.md")
	if got := lnk.toTrackingPath(inHome); got != wantRel {
		t.Errorf("toTrackingPath 主目录内 = %q, want %q", got, wantRel)
	}

	// 主目录外的路径保持绝对
	outside := filepath.Join(string(filepath.Separator), "opt", "etc", "b.conf")
	if got := lnk.toTrackingPath(outside); got != outside {
		t.Errorf("toTrackingPath 主目录外 = %q, want %q", got, outside)
	}
}

func TestGetRepoFilePath(t *testing.T) {
	skipOnWindows(t)
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("无法获取用户主目录")
	}

	repoDir := t.TempDir()

	t.Run("无主机", func(t *testing.T) {
		lnk := NewLnk(WithRepoPath(repoDir))
		if lnk.host != "" {
			t.Fatalf("默认 host 应为空，得到 %q", lnk.host)
		}
		// 主目录内文件映射到仓库同相对路径
		inHome := filepath.Join(home, "docs", "a.md")
		want := filepath.Join(repoDir, "docs", "a.md")
		if got := lnk.getRepoFilePath(inHome); got != want {
			t.Errorf("getRepoFilePath = %q, want %q", got, want)
		}
		// 主目录外文件落到仓库根的文件名
		outside := filepath.Join(string(filepath.Separator), "opt", "b.conf")
		want = filepath.Join(repoDir, "b.conf")
		if got := lnk.getRepoFilePath(outside); got != want {
			t.Errorf("getRepoFilePath 主目录外 = %q, want %q", got, want)
		}
	})

	t.Run("有主机", func(t *testing.T) {
		lnk := NewLnk(WithRepoPath(repoDir), WithHost("mymac"))
		inHome := filepath.Join(home, "docs", "a.md")
		want := filepath.Join(repoDir, "mymac.lnk", "docs", "a.md")
		if got := lnk.getRepoFilePath(inHome); got != want {
			t.Errorf("getRepoFilePath(带主机) = %q, want %q", got, want)
		}
	})
}

func TestGetRelativePathInRepo(t *testing.T) {
	skipOnWindows(t)
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("无法获取用户主目录")
	}
	repoDir := t.TempDir()

	lnk := NewLnk(WithRepoPath(repoDir))
	inHome := filepath.Join(home, "docs", "a.md")
	want := filepath.Join("docs", "a.md")
	if got := lnk.getRelativePathInRepo(inHome); got != want {
		t.Errorf("getRelativePathInRepo = %q, want %q", got, want)
	}

	lnkHosted := NewLnk(WithRepoPath(repoDir), WithHost("mymac"))
	wantHosted := filepath.Join("mymac.lnk", "docs", "a.md")
	if got := lnkHosted.getRelativePathInRepo(inHome); got != wantHosted {
		t.Errorf("getRelativePathInRepo(带主机) = %q, want %q", got, wantHosted)
	}
}

func TestGetHostDir(t *testing.T) {
	repoDir := t.TempDir()
	lnk := NewLnk(WithRepoPath(repoDir))
	if got := lnk.getHostDir(); got != repoDir {
		t.Errorf("getHostDir 无主机 = %q, want %q", got, repoDir)
	}
	lnkHosted := NewLnk(WithRepoPath(repoDir), WithHost("mymac"))
	want := filepath.Join(repoDir, "mymac.lnk")
	if got := lnkHosted.getHostDir(); got != want {
		t.Errorf("getHostDir 带主机 = %q, want %q", got, want)
	}
}
