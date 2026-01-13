package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/sohaha/zzz/app/agent"
)

var (
	agentPrompt              string
	agentMaxRuns             int
	agentMaxCost             float64
	agentMaxDuration         string
	agentOwner               string
	agentRepo                string
	agentEnableCommits       bool
	agentEnableBranches      bool
	agentBranchPrefix        string
	agentMergeStrategy       string
	agentNotesFile           string
	agentDryRun              bool
	agentCompletionThreshold int
	agentReviewPrompt        string
	agentDisableCIRetry      bool
	agentCIRetryMax          int
	agentWorktreeName        string
	agentWorktreeBaseDir     string
	agentCleanupWorktree     bool
	agentListWorktrees       bool
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "持续迭代运行 Agent",
	Long:  `持续 Agent 代理 - 循环执行 Agent 并集成 git/PR 工作流`,
	Example: fmt.Sprintf(`  %s agent -p "修复所有 linter 错误" -m 5 --owner myuser --repo myproject
  %[1]s agent -p "添加测试" --max-cost 10.00 --owner myuser --repo myproject
  %[1]s agent -p "添加文档" --max-duration 2h --owner myuser --repo myproject
  %[1]s agent -p "重构模块" --max-duration 30m --owner myuser --repo myproject
  %[1]s agent -p "添加单元测试" -m 5 --owner myuser --repo myproject --worktree instance-1
  %[1]s agent --list-worktrees`, use),
	RunE: runAgentCommand,
}

func init() {
	rootCmd.AddCommand(agentCmd)

	agentCmd.Flags().StringVarP(&agentPrompt, "prompt", "p", "", "执行的提示词/目标 (必需)")
	agentCmd.Flags().IntVarP(&agentMaxRuns, "max-runs", "m", 0, "成功迭代的最大次数 (0 为无限)")
	agentCmd.Flags().Float64Var(&agentMaxCost, "max-cost", 0, "最大花费成本 (美元) (0 为无限)")
	agentCmd.Flags().StringVar(&agentMaxDuration, "max-duration", "", "最大运行时长 (例如: 2h, 30m, 1h30m)")
	agentCmd.Flags().StringVar(&agentOwner, "owner", "", "GitHub 仓库所有者 (未提供时从 git remote 自动检测)")
	agentCmd.Flags().StringVar(&agentRepo, "repo", "", "GitHub 仓库名称 (未提供时从 git remote 自动检测)")
	agentCmd.Flags().BoolVar(&agentEnableCommits, "enable-commits", false, "启用自动提交和 PR 创建")
	agentCmd.Flags().BoolVar(&agentEnableBranches, "enable-branches", false, "启用分支和 PR 创建")
	agentCmd.Flags().StringVar(&agentBranchPrefix, "branch-prefix", "continuous/", "迭代分支名前缀")
	agentCmd.Flags().StringVar(&agentMergeStrategy, "merge-strategy", "squash", "PR 合并策略: squash, merge 或 rebase")
	agentCmd.Flags().StringVar(&agentNotesFile, "notes-file", "", "迭代上下文共享笔记文件")
	agentCmd.Flags().BoolVar(&agentDryRun, "dry-run", false, "模拟执行不实际修改")
	agentCmd.Flags().IntVar(&agentCompletionThreshold, "completion-threshold", 3, "提前停止所需的连续完成信号数")
	agentCmd.Flags().StringVarP(&agentReviewPrompt, "review-prompt", "r", "", "每次迭代后运行审查以验证变更")
	agentCmd.Flags().BoolVar(&agentDisableCIRetry, "disable-ci-retry", false, "禁用自动 CI 失败重试 (默认启用)")
	agentCmd.Flags().IntVar(&agentCIRetryMax, "ci-retry-max", 1, "每个 PR 的最大 CI 修复尝试次数")
	agentCmd.Flags().StringVar(&agentWorktreeName, "worktree", "", "在 git worktree 中运行以支持并行执行 (需要时创建)")
	agentCmd.Flags().StringVar(&agentWorktreeBaseDir, "worktree-base-dir", "../continuous-worktrees", "worktree 基础目录")
	agentCmd.Flags().BoolVar(&agentCleanupWorktree, "cleanup-worktree", false, "完成后移除 worktree")
	agentCmd.Flags().BoolVar(&agentListWorktrees, "list-worktrees", false, "列出所有活动的 git worktree 并退出")
}

func runAgentCommand(cmd *cobra.Command, args []string) error {
	if agentListWorktrees {
		return agent.ListWorktrees()
	}

	if agentPrompt == "" {
		return fmt.Errorf("需要 --prompt 参数，使用 -p 提供提示词")
	}

	if agentMaxRuns == 0 && agentMaxCost == 0 && agentMaxDuration == "" {
		return fmt.Errorf("需要 --max-runs、--max-cost 或 --max-duration 中的至少一个参数")
	}

	if agentNotesFile == "" {
		agentNotesFile = ".agent_notes_" + time.Now().Format("2006-01-02_15-04-05") + ".md"
	}

	ctx := &agent.Context{
		Prompt:              agentPrompt,
		MaxRuns:             agentMaxRuns,
		MaxCost:             agentMaxCost,
		EnableCommits:       agentEnableCommits,
		EnableBranches:      agentEnableBranches,
		BranchPrefix:        agentBranchPrefix,
		MergeStrategy:       agentMergeStrategy,
		NotesFile:           agentNotesFile,
		DryRun:              agentDryRun,
		CompletionSignal:    agent.CompletionSignal,
		CompletionThreshold: agentCompletionThreshold,
		ReviewPrompt:        agentReviewPrompt,
		CIRetryEnabled:      !agentDisableCIRetry,
		CIRetryMax:          agentCIRetryMax,
		WorktreeName:        agentWorktreeName,
		WorktreeBaseDir:     agentWorktreeBaseDir,
		CleanupWorktree:     agentCleanupWorktree,
		StartTime:           time.Now(),
	}

	if agentMaxDuration != "" {
		duration, err := agent.ParseDuration(agentMaxDuration)
		if err != nil {
			return fmt.Errorf("无效的 --max-duration 参数: %v", err)
		}
		ctx.MaxDuration = duration
	}

	if ctx.EnableCommits {
		owner, repo, err := agent.DetectGitHubRepo()
		if err != nil && agentOwner == "" && agentRepo == "" {
			return fmt.Errorf("检测 GitHub 仓库失败: %v\n请使用 --owner 和 --repo 参数，或从带有 GitHub remote 的 Git 仓库运行", err)
		}
		if agentOwner != "" {
			ctx.Owner = agentOwner
		} else {
			ctx.Owner = owner
		}
		if agentRepo != "" {
			ctx.Repo = agentRepo
		} else {
			ctx.Repo = repo
		}

		if ctx.Owner == "" || ctx.Repo == "" {
			return fmt.Errorf("需要 GitHub owner 和 repo，请使用 --owner 和 --repo 参数")
		}
	}

	return agent.Run(ctx)
}
