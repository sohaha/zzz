package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/sohaha/zlsgo/zshell"
	"github.com/sohaha/zzz/util"
)

func CommitAndCreatePR(ctx *Context, branchName, mainBranch string, display func() string) error {
	hasChanges, err := CheckHasChanges()
	if err != nil {
		return err
	}
	if !hasChanges {
		util.Log.Printf("%s 未检测到更改，正在清理分支...\n", display())
		CleanupBranch(branchName, mainBranch)
		return nil
	}

	if ctx.DryRun {
		util.Log.Printf("%s (演习模式) 将提交更改并创建 PR\n", display())
		util.Log.Printf("%s (演习模式) 已在分支提交更改: %s\n", display(), branchName)
		util.Log.Printf("%s (演习模式) 将推送分支...\n", display())
		util.Log.Printf("%s (演习模式) 将创建 Pull Request...\n", display())
		util.Log.Printf("%s (演习模式) PR 已合并: <提交标题将在此显示>\n", display())
		return nil
	}

	util.Log.Printf("%s 正在提交更改...\n", display())

	if err := ctx.Backend.RunCommit(ctx, PromptCommitMessage); err != nil {
		CleanupBranch(branchName, mainBranch)
		return fmt.Errorf("提交更改失败: %v", err)
	}

	hasChangesAfter, _ := CheckHasChanges()
	if hasChangesAfter {
		CleanupBranch(branchName, mainBranch)
		return fmt.Errorf("提交命令已执行但更改仍存在 (存在未提交或未跟踪的文件)")
	}

	util.Log.Printf("%s 已在分支提交更改: %s\n", display(), branchName)

	_, commitMsg, _, _ := zshell.ExecCommand(context.Background(), []string{"git", "log", "-1", "--format=%B", branchName}, nil, nil, nil)
	commitMsg = strings.TrimSpace(commitMsg)
	lines := strings.Split(commitMsg, "\n")
	commitTitle := lines[0]
	commitBody := ""
	if len(lines) > 3 {
		commitBody = strings.Join(lines[3:], "\n")
	}

	util.Log.Printf("%s 正在推送分支...\n", display())
	code, _, stderr, _ := zshell.ExecCommand(context.Background(), []string{"git", "push", "-u", "origin", branchName}, nil, nil, nil)
	if code != 0 {
		CleanupBranch(branchName, mainBranch)
		return fmt.Errorf("推送分支失败: %s", stderr)
	}

	util.Log.Printf("%s 正在创建 Pull Request...\n", display())
	code, prOutput, stderr, _ := zshell.ExecCommand(context.Background(), []string{
		"gh", "pr", "create",
		"--repo", fmt.Sprintf("%s/%s", ctx.Owner, ctx.Repo),
		"--title", commitTitle,
		"--body", commitBody,
		"--base", mainBranch,
	}, nil, nil, nil)

	if code != 0 {
		CleanupBranch(branchName, mainBranch)
		return fmt.Errorf("创建 PR 失败: %s", stderr)
	}

	prNumberRe := regexp.MustCompile(`(?:pull/|#)(\d+)`)
	matches := prNumberRe.FindStringSubmatch(prOutput)
	if len(matches) < 2 {
		CleanupBranch(branchName, mainBranch)
		return fmt.Errorf("从输出中提取 PR 编号失败: %s", prOutput)
	}
	prNumber := matches[1]

	util.Log.Printf("%s PR #%s 已创建，等待 5 秒让 GitHub 准备...\n", display(), prNumber)
	time.Sleep(5 * time.Second)

	if !WaitForPRChecks(ctx, prNumber, display) {
		if ctx.CIRetryEnabled {
			util.Log.Printf("%s CI 检查失败，正在尝试自动修复...\n", display())
			if AttemptCIFixAndRecheck(ctx, prNumber, branchName, display) {
				util.Log.Printf("%s CI 修复成功!\n", display())
			} else {
				util.Log.Warnf("%s CI 修复失败，正在关闭 PR 并删除远程分支...", display())
				zshell.ExecCommand(context.Background(), []string{"gh", "pr", "close", prNumber, "--repo", fmt.Sprintf("%s/%s", ctx.Owner, ctx.Repo), "--delete-branch"}, nil, nil, nil)
				CleanupBranch(branchName, mainBranch)
				return fmt.Errorf("CI 修复尝试后 PR 检查仍失败")
			}
		} else {
			util.Log.Warnf("%s PR 检查失败或超时，正在关闭 PR 并删除远程分支...", display())
			if code, _, _, _ := zshell.ExecCommand(context.Background(), []string{"gh", "pr", "close", prNumber, "--repo", fmt.Sprintf("%s/%s", ctx.Owner, ctx.Repo), "--delete-branch"}, nil, nil, nil); code != 0 {
				util.Log.Warnf("关闭 PR 失败")
			}
			CleanupBranch(branchName, mainBranch)
			return fmt.Errorf("PR 检查失败")
		}
	}

	if err := MergePRAndCleanup(ctx, prNumber, branchName, mainBranch, display); err != nil {
		return err
	}

	util.Log.Printf("%s PR #%s 已合并: %s\n", display(), prNumber, commitTitle)
	return nil
}

func WaitForPRChecks(ctx *Context, prNumber string, display func() string) bool {
	maxIterations := 180
	prevState := ""

	for i := 0; i < maxIterations; i++ {
		time.Sleep(10 * time.Second)

		code, checksJSON, _, _ := zshell.ExecCommand(context.Background(), []string{
			"gh", "pr", "checks", prNumber,
			"--repo", fmt.Sprintf("%s/%s", ctx.Owner, ctx.Repo),
			"--json", "state,bucket",
		}, nil, nil, nil)

		var checks []PRCheckResult
		noChecksConfigured := false

		if code != 0 || checksJSON == "" {
			if strings.Contains(checksJSON, "no checks") {
				noChecksConfigured = true
			} else {
				continue
			}
		}

		if !noChecksConfigured && checksJSON != "" {
			if err := json.Unmarshal([]byte(strings.TrimSpace(checksJSON)), &checks); err != nil {
				continue
			}
		}

		if noChecksConfigured || len(checks) == 0 {
			if i < 18 {
				continue
			}
			util.Log.Warnf("   等待后未找到检查项，继续执行（无检查）")
			return true
		}

		allCompleted := true
		allSuccess := true
		successCount := 0
		pendingCount := 0
		failedCount := 0

		for _, check := range checks {
			bucket := check.Bucket
			if bucket == "" {
				bucket = "pending"
			}
			if bucket == "pending" || bucket == "null" {
				allCompleted = false
				pendingCount++
			} else if bucket == "fail" {
				allSuccess = false
				failedCount++
			} else {
				successCount++
			}
		}

		code, prInfoJSON, _, _ := zshell.ExecCommand(context.Background(), []string{
			"gh", "pr", "view", prNumber,
			"--repo", fmt.Sprintf("%s/%s", ctx.Owner, ctx.Repo),
			"--json", "reviewDecision,reviewRequests",
		}, nil, nil, nil)

		var prInfo PRInfo
		if code == 0 && prInfoJSON != "" {
			if err := json.Unmarshal([]byte(strings.TrimSpace(prInfoJSON)), &prInfo); err != nil {
				util.Log.Warnf("解析 PR 信息失败: %v", err)
			}
		}

		reviewsPending := false
		if prInfo.ReviewDecision == "REVIEW_REQUIRED" || len(prInfo.ReviewRequests) > 0 {
			reviewsPending = true
		}

		currentState := fmt.Sprintf("checks:%d,success:%d,pending:%d,failed:%d,review:%s",
			len(checks), successCount, pendingCount, failedCount, prInfo.ReviewDecision)

		if currentState != prevState {
			util.Log.Printf("%s 正在检查 PR 状态 (迭代 %d/%d)...\n", display(), i+1, maxIterations)
			util.Log.Printf("找到 %d 个检查项\n", len(checks))
			if len(checks) > 0 {
				util.Log.Printf("   %d    %d    %d\n", successCount, pendingCount, failedCount)
			}
			reviewStatus := "None"
			if prInfo.ReviewDecision != "" {
				reviewStatus = prInfo.ReviewDecision
			} else if len(prInfo.ReviewRequests) > 0 {
				reviewStatus = fmt.Sprintf("已请求 %d 个审查", len(prInfo.ReviewRequests))
			}
			util.Log.Printf("审查状态: %s\n", reviewStatus)
			prevState = currentState
		}

		if allCompleted && allSuccess {
			if prInfo.ReviewDecision == "APPROVED" ||
				(prInfo.ReviewDecision == "" && len(prInfo.ReviewRequests) == 0) {
				util.Log.Printf("%s 所有 PR 检查和审查已通过\n", display())
				return true
			} else if reviewsPending {
				util.Log.Printf("所有检查已通过，等待审查中...\n")
			}
		}

		if allCompleted && !allSuccess {
			util.Log.Errorf("%s PR 检查失败", display())
			return false
		}

		if prInfo.ReviewDecision == "CHANGES_REQUESTED" {
			util.Log.Errorf("%s PR 审查要求修改", display())
			return false
		}
	}

	util.Log.Warnf("%s 等待 PR 检查和审查超时 (30 分钟)", display())
	return false
}

func MergePRAndCleanup(ctx *Context, prNumber, branchName, mainBranch string, display func() string) error {
	util.Log.Printf("%s 正在使用 main 最新内容更新分支...\n", display())
	code, updateOutput, _, _ := zshell.ExecCommand(context.Background(), []string{
		"gh", "pr", "update-branch", prNumber,
		"--repo", fmt.Sprintf("%s/%s", ctx.Owner, ctx.Repo),
	}, nil, nil, nil)

	if code == 0 {
		util.Log.Printf("%s 分支已更新，重新检查 PR 状态...\n", display())
		if !WaitForPRChecks(ctx, prNumber, display) {
			return fmt.Errorf("分支更新后 PR 检查失败")
		}
	} else {
		if strings.Contains(updateOutput, "already up-to-date") || strings.Contains(updateOutput, "is up to date") {
			util.Log.Printf("%s 分支已是最新\n", display())
		} else {
			util.Log.Warnf("%s 分支更新失败: %s", display(), updateOutput)
			return fmt.Errorf("分支更新失败")
		}
	}

	mergeFlag := "--squash"
	switch ctx.MergeStrategy {
	case "merge":
		mergeFlag = "--merge"
	case "rebase":
		mergeFlag = "--rebase"
	}

	util.Log.Printf("%s 正在使用 %s 策略合并 PR #%s...\n", display(), ctx.MergeStrategy, prNumber)
	code, _, _, _ = zshell.ExecCommand(context.Background(), []string{
		"gh", "pr", "merge", prNumber,
		"--repo", fmt.Sprintf("%s/%s", ctx.Owner, ctx.Repo),
		mergeFlag,
	}, nil, nil, nil)

	if code != 0 {
		return fmt.Errorf("合并 PR 失败 (可能存在冲突或被阻止)")
	}

	util.Log.Printf("%s 正在从 main 拉取最新内容...\n", display())
	zshell.ExecCommand(context.Background(), []string{"git", "checkout", mainBranch}, nil, nil, nil)
	zshell.ExecCommand(context.Background(), []string{"git", "pull", "origin", mainBranch}, nil, nil, nil)

	util.Log.Printf(" %s 正在删除本地分支: %s\n", display(), branchName)
	zshell.ExecCommand(context.Background(), []string{"git", "branch", "-d", branchName}, nil, nil, nil)

	return nil
}

func GetFailedRunID(ctx *Context, prNumber string) (string, error) {
	code, prInfoJSON, _, _ := zshell.ExecCommand(context.Background(), []string{
		"gh", "pr", "view", prNumber,
		"--repo", fmt.Sprintf("%s/%s", ctx.Owner, ctx.Repo),
		"--json", "headRefOid",
	}, nil, nil, nil)

	if code != 0 {
		return "", fmt.Errorf("获取 PR 信息失败")
	}

	var prInfo PRInfo
	if err := json.Unmarshal([]byte(strings.TrimSpace(prInfoJSON)), &prInfo); err != nil {
		return "", err
	}

	headSHA := prInfo.HeadRefOid
	if headSHA == "" {
		return "", fmt.Errorf("未找到 head SHA")
	}

	code, runsJSON, _, _ := zshell.ExecCommand(context.Background(), []string{
		"gh", "run", "list",
		"--repo", fmt.Sprintf("%s/%s", ctx.Owner, ctx.Repo),
		"--commit", headSHA,
		"--status", "failure",
		"--limit", "1",
		"--json", "databaseId",
	}, nil, nil, nil)

	if code != 0 {
		return "", fmt.Errorf("列出运行记录失败")
	}

	var runs []WorkflowRun
	if err := json.Unmarshal([]byte(strings.TrimSpace(runsJSON)), &runs); err != nil {
		return "", err
	}

	if len(runs) == 0 {
		return "", fmt.Errorf("未找到失败的运行记录")
	}

	return fmt.Sprintf("%d", runs[0].DatabaseId), nil
}

func AttemptCIFixAndRecheck(ctx *Context, prNumber, branchName string, display func() string) bool {
	for attempt := 1; attempt <= ctx.CIRetryMax; attempt++ {
		if err := RunCIFixIteration(ctx, prNumber, display, attempt); err != nil {
			util.Log.Warnf("%s CI fix attempt %d failed: %v", display(), attempt, err)
			continue
		}

		util.Log.Printf("%s 正在推送 CI 修复到分支...\n", display())
		code, _, _, _ := zshell.ExecCommand(context.Background(), []string{"git", "push", "origin", branchName}, nil, nil, nil)
		if code != 0 {
			util.Log.Warnf("%s 推送 CI 修复失败", display())
			continue
		}

		util.Log.Printf("%s CI 修复已推送，等待新的检查...\n", display())
		time.Sleep(5 * time.Second)

		util.Log.Printf("%s 正在等待修复后的 CI 检查...\n", display())
		if WaitForPRChecks(ctx, prNumber, display) {
			util.Log.Printf("%s 修复后 CI 检查通过!\n", display())
			return true
		}

		util.Log.Warnf("%s 修复尝试 %d 后 CI 仍然失败\n", display(), attempt)
	}

	util.Log.Errorf("%s 所有 CI 修复尝试均已用尽", display())
	return false
}
