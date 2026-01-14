package agent

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sohaha/zzz/util"
)

func Run(ctx *Context) error {
	if err := ValidateRequirements(ctx); err != nil {
		return err
	}

	if err := SetupWorktree(ctx); err != nil {
		return fmt.Errorf("设置 worktree 失败: %v", err)
	}

	defer CleanupWorktree(ctx)

	util.Log.Printf("目标: %s\n", ctx.Prompt)
	if ctx.MaxRuns > 0 {
		util.Log.Printf("最大迭代次数: %d\n", ctx.MaxRuns)
	}
	if ctx.MaxCost > 0 {
		util.Log.Printf("最大成本: $%.2f\n", ctx.MaxCost)
	}
	if ctx.MaxDuration > 0 {
		util.Log.Printf("最大时长: %s\n", FormatDuration(ctx.MaxDuration))
	}

	return runMainLoop(ctx)
}

func runMainLoop(ctx *Context) error {
	iteration := 1

	for {
		if !shouldContinue(ctx) {
			break
		}

		display := GetIterationDisplay(iteration, ctx.MaxRuns, ctx.ExtraIterations)
		util.Log.Printf("%s 开始迭代...\n", display())

		if err := executeSingleIteration(ctx, iteration, display); err != nil {
			if err := handleIterationError(ctx, display, err); err != nil {
				return err
			}
		} else {
			ctx.ErrorCount = 0
			if ctx.ExtraIterations > 0 {
				ctx.ExtraIterations--
			}
			ctx.SuccessfulIterations++
		}

		iteration++
		time.Sleep(IterationDelaySeconds * time.Second)
	}

	showCompletionSummary(ctx)
	return nil
}

func handleIterationError(ctx *Context, display func() string, err error) error {
	ctx.ErrorCount++
	ctx.ExtraIterations++
	util.Log.Printf("%s 错误: %v\n", display(), err)

	if ctx.ErrorCount >= MaxConsecutiveErrors {
		return fmt.Errorf("连续发生 %d 次错误，退出", MaxConsecutiveErrors)
	}
	return nil
}

func shouldContinue(ctx *Context) bool {
	if ctx.CompletionCount >= ctx.CompletionThreshold {
		return false
	}

	if ctx.MaxRuns > 0 && ctx.SuccessfulIterations >= ctx.MaxRuns {
		return false
	}

	if ctx.MaxCost > 0 && ctx.TotalCost >= ctx.MaxCost {
		util.Log.Warnf("已达到最大成本限制: $%.3f", ctx.TotalCost)
		return false
	}

	if ctx.MaxDuration > 0 {
		elapsed := time.Since(ctx.StartTime)
		if elapsed >= ctx.MaxDuration {
			util.Log.Warnf("已达到最大时长限制: %s", FormatDuration(elapsed))
			return false
		}
	}

	return true
}

func executeSingleIteration(ctx *Context, iteration int, display func() string) error {
	var mainBranch, branchName string

	if ctx.EnableCommits && ctx.EnableBranches {
		var err error
		mainBranch, branchName, err = CreateIterationBranch(ctx, iteration, display)
		if err != nil {
			return fmt.Errorf("创建分支失败: %v", err)
		}
	}

	enhancedPrompt := BuildEnhancedPrompt(ctx)

	util.Log.Printf("%s 正在运行 %s Agent...\n", display(), ctx.Backend.Name())
	result, err := ctx.Backend.RunIteration(ctx, enhancedPrompt, display)
	if err != nil {
		CleanupBranch(branchName, mainBranch)
		return err
	}

	if result.IsError {
		CleanupBranch(branchName, mainBranch)
		return fmt.Errorf("Agent 返回错误: %s", result.Result)
	}

	if result.TotalCostUSD > 0 {
		ctx.TotalCost += result.TotalCostUSD
		util.Log.Printf("%s 迭代成本: $%.3f (total: $%.3f)\n", display(), result.TotalCostUSD, ctx.TotalCost)
	}

	if strings.Contains(result.Result, ctx.CompletionSignal) {
		ctx.CompletionCount++
		util.Log.Printf("%s 检测到完成信号 (%d/%d)\n", display(), ctx.CompletionCount, ctx.CompletionThreshold)
	} else {
		if ctx.CompletionCount > 0 {
			util.Log.Printf("%s 未检测到完成信号，重置计数器\n", display())
		}
		ctx.CompletionCount = 0
	}

	if ctx.ReviewPrompt != "" {
		if err := RunReviewerIteration(ctx, display); err != nil {
			CleanupBranch(branchName, mainBranch)
			return fmt.Errorf("审查失败: %v", err)
		}
	}

	util.Log.Printf("%s 工作完成\n", display())

	if !ctx.EnableCommits {
		util.Log.Printf("%s 跳过提交（未开启 --enable-commits）\n", display())
		CleanupBranch(branchName, mainBranch)
		return nil
	}

	var commitErr error
	if ctx.EnableBranches {
		commitErr = CommitAndCreatePR(ctx, branchName, mainBranch, display)
	} else {
		commitErr = CommitOnCurrentBranch(ctx, display)
	}

	if commitErr != nil {
		return commitErr
	}

	return nil
}

func showCompletionSummary(ctx *Context) {
	elapsed := time.Since(ctx.StartTime)
	elapsedMsg := fmt.Sprintf(" (耗时: %s)", FormatDuration(elapsed))

	if ctx.CompletionCount >= ctx.CompletionThreshold {
		util.Log.Printf("项目完成！连续检测到 %d 次完成信号。总成本: $%.3f%s\n",
			ctx.CompletionCount, ctx.TotalCost, elapsedMsg)
	} else if ctx.TotalCost > 0 {
		util.Log.Printf("完成，总成本: $%.3f%s\n", ctx.TotalCost, elapsedMsg)
	} else {
		util.Log.Printf("完成%s\n", elapsedMsg)
	}

	os.Remove(ctx.NotesFile)
}
