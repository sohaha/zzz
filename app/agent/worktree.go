package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sohaha/zlsgo/zfile"
	"github.com/sohaha/zlsgo/zshell"
	"github.com/sohaha/zzz/util"
)

func SetupWorktree(ctx *Context) error {
	if ctx.WorktreeName == "" {
		return nil
	}

	if code, _, _, _ := zshell.ExecCommand(context.Background(), []string{"git", "rev-parse", "--git-dir"}, nil, nil, nil); code != 0 {
		return fmt.Errorf("不在 git 仓库中。Worktree 需要 git 仓库")
	}

	_, mainRepoDir, _, _ := zshell.ExecCommand(context.Background(), []string{"git", "rev-parse", "--show-toplevel"}, nil, nil, nil)
	mainRepoDir = strings.TrimSpace(mainRepoDir)

	worktreePath := filepath.Join(ctx.WorktreeBaseDir, ctx.WorktreeName)
	if !filepath.IsAbs(worktreePath) {
		worktreePath = filepath.Join(mainRepoDir, worktreePath)
	}

	currentBranch := GetCurrentBranch()

	if zfile.DirExist(worktreePath) {
		util.Log.Printf("Worktree '%s' 已存在于: %s\n", ctx.WorktreeName, worktreePath)
		util.Log.Printf("正在切换到 worktree 目录...\n")

		if err := os.Chdir(worktreePath); err != nil {
			return fmt.Errorf("切换到 worktree 目录失败: %v", err)
		}

		util.Log.Printf("正在从 %s 拉取最新更改...\n", currentBranch)
		code, _, _, _ := zshell.ExecCommand(context.Background(), []string{"git", "pull", "origin", currentBranch}, nil, nil, nil)
		if code != 0 {
			util.Log.Warnf("警告：拉取最新变更失败（继续执行）\n")
		}
	} else {
		util.Log.Printf("创建新 worktree '%s' 于: %s\n", ctx.WorktreeName, worktreePath)

		baseDir := filepath.Dir(worktreePath)
		if !zfile.DirExist(baseDir) {
			if err := os.MkdirAll(baseDir, 0o755); err != nil {
				return fmt.Errorf("创建 worktree 基础目录失败: %v", err)
			}
		}

		code, _, stderr, _ := zshell.ExecCommand(context.Background(), []string{"git", "worktree", "add", worktreePath, currentBranch}, nil, nil, nil)
		if code != 0 {
			return fmt.Errorf("创建 worktree 失败: %s", stderr)
		}

		util.Log.Printf("正在切换到 worktree 目录...\n")
		if err := os.Chdir(worktreePath); err != nil {
			return fmt.Errorf("切换到 worktree 目录失败: %v", err)
		}
	}

	util.Log.Printf("Worktree '%s' 已就绪于: %s\n", ctx.WorktreeName, worktreePath)
	return nil
}

func CleanupWorktree(ctx *Context) {
	if ctx.WorktreeName == "" || !ctx.CleanupWorktree {
		return
	}

	if code, _, _, _ := zshell.ExecCommand(context.Background(), []string{"git", "rev-parse", "--git-dir"}, nil, nil, nil); code != 0 {
		return
	}

	_, mainRepoDir, _, _ := zshell.ExecCommand(context.Background(), []string{"git", "rev-parse", "--show-toplevel"}, nil, nil, nil)
	mainRepoDir = strings.TrimSpace(mainRepoDir)

	worktreePath := filepath.Join(ctx.WorktreeBaseDir, ctx.WorktreeName)
	if !filepath.IsAbs(worktreePath) {
		worktreePath = filepath.Join(mainRepoDir, worktreePath)
	}

	util.Log.Printf("正在清理 worktree '%s'...\n", ctx.WorktreeName)

	code, gitCommonDir, _, _ := zshell.ExecCommand(context.Background(), []string{"git", "rev-parse", "--git-common-dir"}, nil, nil, nil)
	gitCommonDir = strings.TrimSpace(gitCommonDir)

	if gitCommonDir != "" {
		mainRepo := filepath.Dir(gitCommonDir)
		if zfile.DirExist(mainRepo) {
			os.Chdir(mainRepo)
		}
	}

	code, _, _, _ = zshell.ExecCommand(context.Background(), []string{"git", "worktree", "remove", worktreePath, "--force"}, nil, nil, nil)
	if code == 0 {
		util.Log.Printf("Worktree 移除成功\n")
	} else {
		util.Log.Warnf("警告：移除 worktree 失败（可能需要手动清理）\n")
		util.Log.Printf("   您可以手动移除: git worktree remove %s --force\n", worktreePath)
	}
}

func ListWorktrees() error {
	code, _, _, _ := zshell.ExecCommand(context.Background(), []string{"git", "rev-parse", "--git-dir"}, nil, nil, nil)
	if code != 0 {
		return fmt.Errorf("不在 git 仓库中")
	}

	util.Log.Info("活跃的 Git Worktrees:")
	fmt.Println()

	code, output, stderr, _ := zshell.ExecCommand(context.Background(), []string{"git", "worktree", "list"}, nil, nil, nil)
	if code != 0 {
		return fmt.Errorf("列出 worktree 失败: %s", stderr)
	}

	fmt.Println(output)
	return nil
}
