package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func setupRepoWithRemote(t *testing.T) (*Git, string) {
	t.Helper()
	base := t.TempDir()
	repoPath := filepath.Join(base, "repo")
	remotePath := filepath.Join(base, "remote.git")

	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "file.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoPath
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v 失败: %v\n%s", args, err, out)
		}
	}

	run("init")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "test")
	run("add", ".")
	run("commit", "-m", "init")

	if err := exec.Command("git", "init", "--bare", remotePath).Run(); err != nil {
		t.Fatal(err)
	}
	run("remote", "add", "origin", remotePath)

	return New(repoPath), remotePath
}

func TestPushSetsUpstreamOnFirstPush(t *testing.T) {
	g, remotePath := setupRepoWithRemote(t)

	// 首次推送：分支无 upstream，Push 应自动带上 -u 成功
	if err := g.Push(); err != nil {
		t.Fatalf("首次 Push 应自动设置 upstream 并成功: %v", err)
	}

	// 远端应有引用
	out, err := exec.Command("git", "--git-dir", remotePath, "rev-parse", "HEAD").CombinedOutput()
	if err != nil {
		t.Fatalf("远端应收到推送: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("远端 HEAD 为空")
	}

	// 再次推送（已设 upstream）应成功
	if err := g.Push(); err != nil {
		t.Fatalf("二次 Push 应成功: %v", err)
	}
}
