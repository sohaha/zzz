package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sohaha/zlsgo/zshell"
	"github.com/sohaha/zzz/app/agent"
)

var (
	agentPrompt              string
	agentModel               string
	agentMaxRuns             int
	agentMaxCost             float64
	agentMaxDuration         string
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
	agentBackendName         string

	// 仓库提供商参数
	agentProvider     string
	agentRepoID       string
	agentRepoAPIKey   string
	agentRepoEndpoint string

	// 回调命令
	agentOnComplete string
	agentOnError    string
	agentOnFinish   string
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "持续迭代运行 Agent",
	Long:  `持续 Agent 代理 - 循环执行 Agent 并集成 git/PR 工作流`,
	Example: fmt.Sprintf(`  %s agent -p "修复所有 linter 错误" -m 5
  %[1]s agent -p "添加测试" --max-cost 10.00
  %[1]s agent -p "添加文档" --max-duration 2h
  %[1]s agent -p "重构模块" --max-duration 30m 
  %[1]s agent -p "添加单元测试" -m 5 --owner myuser --repo myproject --worktree instance-1
  %[1]s agent --agent codex -p "代码优化" -m 3 --owner myuser --repo myproject
  %[1]s agent --list-worktrees`, use),
	RunE: runAgentCommand,
}

func init() {
	rootCmd.AddCommand(agentCmd)

	agentCmd.Flags().StringVarP(&agentPrompt, "prompt", "p", "", "执行的提示词/目标 (必需)")
	agentCmd.Flags().StringVar(&agentModel, "model", "", "手动指定模型")
	agentCmd.Flags().IntVarP(&agentMaxRuns, "max-runs", "m", 0, "成功迭代的最大次数 (0 为无限)")
	agentCmd.Flags().Float64Var(&agentMaxCost, "max-cost", 0, "最大花费成本 (美元) (0 为无限)")
	agentCmd.Flags().StringVar(&agentMaxDuration, "max-duration", "", "最大运行时长 (例如: 2h, 30m, 1h30m)")
	agentCmd.Flags().BoolVar(&agentEnableCommits, "enable-commits", false, "启用自动提交和 PR 创建")
	agentCmd.Flags().BoolVar(&agentEnableBranches, "enable-branches", false, "启用分支和 PR 创建")
	agentCmd.Flags().StringVar(&agentBranchPrefix, "branch-prefix", "continuous/", "迭代分支名前缀")
	agentCmd.Flags().StringVar(&agentMergeStrategy, "merge-strategy", "squash", "PR 合并策略: squash, merge 或 rebase")
	agentCmd.Flags().StringVar(&agentNotesFile, "notes-file", "agent_notes.md", "迭代上下文共享笔记文件")
	agentCmd.Flags().BoolVar(&agentDryRun, "dry-run", false, "模拟执行不实际修改")
	agentCmd.Flags().IntVar(&agentCompletionThreshold, "completion-threshold", 3, "提前停止所需的连续完成信号数")
	agentCmd.Flags().StringVarP(&agentReviewPrompt, "review-prompt", "r", "", "每次迭代后运行审查以验证变更")
	agentCmd.Flags().BoolVar(&agentDisableCIRetry, "disable-ci-retry", false, "禁用自动 CI 失败重试 (默认启用)")
	agentCmd.Flags().IntVar(&agentCIRetryMax, "ci-retry-max", 1, "每个 PR 的最大 CI 修复尝试次数")
	agentCmd.Flags().StringVar(&agentWorktreeName, "worktree", "", "在 git worktree 中运行以支持并行执行 (需要时创建)")
	agentCmd.Flags().StringVar(&agentWorktreeBaseDir, "worktree-base-dir", "../continuous-worktrees", "worktree 基础目录")
	agentCmd.Flags().BoolVar(&agentCleanupWorktree, "cleanup-worktree", false, "完成后移除 worktree")
	agentCmd.Flags().BoolVar(&agentListWorktrees, "list-worktrees", false, "列出所有活动的 git worktree 并退出")
	agentCmd.Flags().StringVar(&agentBackendName, "agent", "claude-code", "AI 后端 [claude-code, codex]")

	// 仓库提供商参数
	agentCmd.Flags().StringVar(&agentProvider, "provider", "", "仓库提供商 [github, cool]，不指定时从 git remote 自动检测")
	agentCmd.Flags().StringVar(&agentRepoID, "repo-id", "", "仓库 ID (格式: owner/repo)，不指定时从 git remote 自动检测")
	agentCmd.Flags().StringVar(&agentRepoAPIKey, "repo-api-key", "", "API Key (或使用环境变量 COOL_API_KEY)")
	agentCmd.Flags().StringVar(&agentRepoEndpoint, "repo-endpoint", "", "API 端点 (仅 API-based providers，如 https://api.cnb.cool)")

	// 回调命令
	agentCmd.Flags().StringVar(&agentOnComplete, "on-complete", "", "成功完成时执行的 shell 命令")
	agentCmd.Flags().StringVar(&agentOnError, "on-error", "", "失败时执行的 shell 命令")
	agentCmd.Flags().StringVar(&agentOnFinish, "on-finish", "", "完成时执行的命令 (无论成功/失败)")
}

func runAgentCommand(cmd *cobra.Command, args []string) error {
	if agentListWorktrees {
		return agent.ListWorktrees()
	}

	if agentPrompt == "" {
		return fmt.Errorf("需要 --prompt 参数，使用 -p 提供提示词")
	}

	if agentPrompt == "-" {
		const maxStdinSize = 1 * 1024 * 1024
		limitedReader := io.LimitReader(os.Stdin, maxStdinSize+1)
		stdinBytes, err := io.ReadAll(limitedReader)
		if err != nil {
			return fmt.Errorf("从 stdin 读取失败: %v", err)
		}
		if len(stdinBytes) > maxStdinSize {
			return fmt.Errorf("stdin 超过最大限制 1MB")
		}
		agentPrompt = strings.TrimSpace(string(stdinBytes))
		if agentPrompt == "" {
			return fmt.Errorf("stdin 为空，无法执行")
		}
	}

	if agentMaxRuns == 0 && agentMaxCost == 0 && agentMaxDuration == "" {
		return fmt.Errorf("需要 --max-runs、--max-cost 或 --max-duration 中的至少一个参数")
	}

	backend, err := agent.NewBackend(agentBackendName)
	if err != nil {
		return err
	}

	ctx := &agent.Context{
		Prompt:              agentPrompt,
		Model:               agentModel,
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
		Backend:             backend,
		StartTime:           time.Now(),
		OnComplete:          agentOnComplete,
		OnError:             agentOnError,
		OnFinish:            agentOnFinish,
	}

	if agentMaxDuration != "" {
		duration, err := agent.ParseDuration(agentMaxDuration)
		if err != nil {
			return fmt.Errorf("无效的 --max-duration 参数: %v", err)
		}
		ctx.MaxDuration = duration
	}

	if ctx.EnableCommits {
		// 检测或解析仓库信息
		var repoInfo *agent.RepositoryInfo
		var providerName string

		if agentRepoID != "" {
			// 手动指定仓库 ID
			parts := strings.Split(agentRepoID, "/")
			if len(parts) != 2 {
				return fmt.Errorf("--repo-id 格式错误，应为 'owner/repo'")
			}

			// 优先使用 --provider 参数，否则自动检测
			if agentProvider != "" {
				providerName = agentProvider
			} else {
				// 尝试从 git remote 检测
				code, stdout, _, err := zshell.ExecCommand(nil,
					[]string{"git", "remote", "get-url", "origin"}, nil, nil, nil)
				if err == nil && code == 0 {
					providerName = agent.DetectProviderFromRemote(stdout)
				}
				if providerName == "" {
					providerName = "github" // 默认
				}
			}

			repoInfo = &agent.RepositoryInfo{
				Provider: providerName,
				Owner:    parts[0],
				Repo:     parts[1], // 只取仓库名，不包含 owner
			}
		} else {
			// 完全自动检测
			code, stdout, _, err := zshell.ExecCommand(nil,
				[]string{"git", "remote", "get-url", "origin"}, nil, nil, nil)
			if err != nil || code != 0 {
				return fmt.Errorf("无法检测仓库类型，请使用 --repo-id 或 --provider 手动指定")
			}

			providerName = agent.DetectProviderFromRemote(stdout)
			if providerName == "" {
				return fmt.Errorf("无法检测仓库类型，请使用 --repo-id 或 --provider 手动指定")
			}

			providerOpts := agent.ProviderOptions{
				Endpoint: agentRepoEndpoint,
			}
			tempProvider, err := agent.NewRepositoryProvider(providerName, providerOpts)
			if err != nil {
				return err
			}

			repoInfo, err = tempProvider.DetectFromGit()
			if err != nil {
				return fmt.Errorf("检测仓库失败: %v\n请使用 --repo-id 手动指定", err)
			}
		}

		providerOpts := agent.ProviderOptions{
			Endpoint: agentRepoEndpoint,
		}
		provider, err := agent.NewRepositoryProvider(providerName, providerOpts)
		if err != nil {
			return err
		}

		ctx.RepoProvider = provider
		ctx.RepoInfo = repoInfo

		// 验证 Provider
		if err := provider.Validate(ctx); err != nil {
			return fmt.Errorf("仓库提供商验证失败: %v", err)
		}
	}

	return agent.Run(ctx)
}
