package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sohaha/zlsgo/zshell"
	"github.com/sohaha/zlsgo/ztime"
	"github.com/sohaha/zzz/util"
)

func CreateIterationBranch(ctx *Context, iteration int, display func() string) (mainBranch, branchName string, err error) {
	mainBranch = GetCurrentBranch()

	if strings.HasPrefix(mainBranch, ctx.BranchPrefix) {
		util.Log.Warnf("%s 已在迭代分支上: %s", display(), mainBranch)
		zshell.ExecCommand(context.Background(), []string{"git", "checkout", "main"}, nil, nil, nil)
		mainBranch = "main"
	}

	dateStr := ztime.FormatTime(time.Now(), "2006-01-02")
	randomHash := GenerateRandomHash()
	branchName = fmt.Sprintf("%siteration-%d/%s-%s", ctx.BranchPrefix, iteration, dateStr, randomHash)

	util.Log.Printf("%s 正在创建分支: %s\n", display(), branchName)

	if ctx.DryRun {
		util.Log.Printf("   (演习模式) 将创建分支 %s\n", branchName)
		return mainBranch, branchName, nil
	}

	code, _, _, _ := zshell.ExecCommand(context.Background(), []string{"git", "checkout", "-b", branchName}, nil, nil, nil)
	if code != 0 {
		return mainBranch, "", fmt.Errorf("创建分支失败")
	}

	return mainBranch, branchName, nil
}

func CommitOnCurrentBranch(ctx *Context, display func() string) error {
	hasChanges, err := CheckHasChanges()
	if err != nil {
		return err
	}
	if !hasChanges {
		util.Log.Printf("%s 无需提交的更改\n", display())
		return nil
	}

	if ctx.DryRun {
		util.Log.Printf("%s (演习模式) 将在当前分支提交更改\n", display())
		return nil
	}

	util.Log.Printf("%s 正在当前分支提交更改...\n", display())

	code, _, _, _ := zshell.ExecCommand(context.Background(), []string{
		"claude", "-p", PromptCommitMessage,
		"--allowedTools", "Bash(git)",
		"--dangerously-skip-permissions",
	}, nil, nil, nil)

	if code != 0 {
		return fmt.Errorf("提交更改失败")
	}

	hasChangesAfter, _ := CheckHasChanges()
	if hasChangesAfter {
		return fmt.Errorf("提交命令已执行但更改仍存在")
	}

	code, commitTitle, _, _ := zshell.ExecCommand(context.Background(), []string{"git", "log", "-1", "--format=%s"}, nil, nil, nil)
	if code == 0 {
		util.Log.Printf("%s 已提交: %s\n", display(), strings.TrimSpace(commitTitle))
	}

	return nil
}
