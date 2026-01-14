package agent

import (
	"context"
	"fmt"

	"github.com/sohaha/zlsgo/zshell"
)

func ValidateRequirements(ctx *Context) error {
	if err := ctx.Backend.Validate(); err != nil {
		return err
	}

	if ctx.EnableCommits {
		if code, _, _, _ := zshell.ExecCommand(context.Background(), []string{"gh", "--version"}, nil, nil, nil); code != 0 {
			return fmt.Errorf("未安装 GitHub CLI (gh): https://cli.github.com")
		}
		if code, _, _, _ := zshell.ExecCommand(context.Background(), []string{"gh", "auth", "status"}, nil, nil, nil); code != 0 {
			return fmt.Errorf("GitHub CLI 未认证，请运行 'gh auth login'")
		}
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

	return nil
}
