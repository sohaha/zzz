package agent

import (
	"fmt"
)

func ValidateRequirements(ctx *Context) error {
	if err := ctx.Backend.Validate(); err != nil {
		return err
	}

	if ctx.MergeStrategy != "squash" && ctx.MergeStrategy != "merge" && ctx.MergeStrategy != "rebase" {
		return fmt.Errorf("无效的合并策略: %s (必须是 squash、merge 或 rebase)", ctx.MergeStrategy)
	}

	if ctx.CompletionThreshold < 1 {
		return fmt.Errorf("完成阈值必须 >= 1")
	}

	if ctx.CIRetryMax < 1 {
		ctx.CIRetryMax = 1
	}

	if ctx.EnableCommits && ctx.EnableBranches && !ctx.DryRun {
		if ctx.RepoProvider == nil || ctx.RepoInfo == nil {
			return fmt.Errorf("未初始化仓库提供商，请使用 --provider/--repo-id 或配置 git remote")
		}
		if err := ctx.RepoProvider.Validate(ctx); err != nil {
			return err
		}
	}

	return nil
}
