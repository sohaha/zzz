package agent

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/sohaha/zzz/util"
)

type AIBackend interface {
	Validate() error
	RunIteration(ctx *Context, prompt string, display func() string) (*BackendResult, error)
	RunCommit(ctx *Context, prompt string) error
	Name() string
}

func NewBackend(name string) (AIBackend, error) {
	switch name {
	case "claude-code", "claude", "":
		return &ClaudeCodeBackend{}, nil
	case "codex":
		return &CodexBackend{}, nil
	case "opencode":
		return &OpencodeBackend{}, nil
	default:
		return nil, fmt.Errorf("未知的 agent 后端: %s (支持: claude-code, codex, opencode)", name)
	}
}

type streamBuffers struct {
	stdout bytes.Buffer
	stderr bytes.Buffer
}

func formatExitCodeError(command string, exitCode int, stderr *bytes.Buffer) error {
	errMsg := strings.TrimSpace(stderr.String())
	if errMsg == "" {
		errMsg = fmt.Sprintf("%s 以非零退出码退出但没有错误输出", command)
	}
	return fmt.Errorf("%s 退出码 %d: %s", command, exitCode, errMsg)
}

func handleBuffers() (*streamBuffers, func(line string, isStdout bool) (string, bool)) {
	buffers := &streamBuffers{}

	writeString := func(line string, isStdout bool) (string, bool) {
		if !isStdout {
			buffers.stderr.WriteString(line)
			buffers.stderr.WriteString("\n")
			return "", false
		}

		buffers.stdout.WriteString(line)
		buffers.stdout.WriteString("\n")

		line = strings.TrimSpace(line)
		if line == "" {
			return "", false
		}
		return line, true
	}

	return buffers, writeString
}

func RunReviewerIteration(ctx *Context, display func() string) error {
	util.Log.Printf("%s 正在运行审查流程...\n", display())

	reviewPrompt := fmt.Sprintf(`%s

## USER REVIEW INSTRUCTIONS
<REVIEW_INSTRUCTIONS>
%s
</REVIEW_INSTRUCTIONS>
`, PromptReviewerContext, ctx.ReviewPrompt)

	result, err := ctx.Backend.RunIteration(ctx, reviewPrompt, display)
	if err != nil {
		return err
	}

	if result.IsError {
		return fmt.Errorf("审查返回错误: %s", result.Result)
	}

	if result.TotalCostUSD > 0 {
		ctx.TotalCost += result.TotalCostUSD
		util.Log.Printf("%s 审查成本: $%.3f\n", display(), result.TotalCostUSD)
	}

	util.Log.Printf("%s 审查通过\n", display())
	return nil
}

func RunCIFixIteration(ctx *Context, prNumber string, display func() string, attempt int) error {
	util.Log.Printf("%s 正在尝试修复 CI 失败 (尝试 %d/%d)...\n", display(), attempt, ctx.CIRetryMax)

	failedRunID, _ := GetFailedRunID(ctx, prNumber)

	ciFixPrompt := fmt.Sprintf(`%s

## CURRENT CONTEXT

- Repository: %s/%s
- PR Number: #%s
- Branch: (current branch)`, PromptCIFixContext, ctx.Owner, ctx.Repo, prNumber)

	if failedRunID != "" {
		ciFixPrompt += fmt.Sprintf("\n- 失败的运行 ID: %s (使用 'gh run view %s --log-failed' 查看)", failedRunID, failedRunID)
	}

	ciFixPrompt += `

## INSTRUCTIONS

1. Start by running 'gh run list --status failure --limit 3' to see recent failures
2. Then use 'gh run view <RUN_ID> --log-failed' to see the error details
3. Analyze what went wrong and fix it
4. After making changes, stage and commit them with a clear commit message describing the fix`

	result, err := ctx.Backend.RunIteration(ctx, ciFixPrompt, display)
	if err != nil {
		return err
	}

	if result.IsError {
		return fmt.Errorf("CI 修复返回错误: %s", result.Result)
	}

	if result.TotalCostUSD > 0 {
		ctx.TotalCost += result.TotalCostUSD
		util.Log.Printf("%s CI 修复成本: $%.3f\n", display(), result.TotalCostUSD)
	}

	hasChanges, _ := CheckHasChanges()
	if !hasChanges {
		return fmt.Errorf("CI 修复未做任何更改")
	}

	hasUncommitted, _ := CheckHasChanges()
	if hasUncommitted {
		util.Log.Printf("%s 正在提交 CI 修复...\n", display())
		if err := ctx.Backend.RunCommit(ctx, PromptCommitMessage); err != nil {
			return fmt.Errorf("提交 CI 修复失败: %v", err)
		}
	}

	return nil
}
