package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sohaha/zlsgo/zshell"
	"github.com/sohaha/zzz/util"
)

type streamMessage struct {
	Type    string `json:"type"`
	Message struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message"`
}

func RunClaudeIteration(ctx *Context, prompt string, display func() string) (*ClaudeResult, error) {
	if ctx.DryRun {
		util.Log.Println("(演习模式) 将运行 Agent")
		return &ClaudeResult{
			Result: "Agent 的模拟响应",
		}, nil
	}

	command := []string{"claude", "-p", prompt, "--dangerously-skip-permissions", "--output-format", "stream-json", "--verbose"}

	var stdout, stderr bytes.Buffer
	var lastResult ClaudeResult
	exitCodeChan, _, err := zshell.CallbackRunContext(context.Background(), command,
		func(line string, isStdout bool) {
			if !isStdout {
				stderr.WriteString(line)
				stderr.WriteString("\n")
				return
			}

			stdout.WriteString(line)
			stdout.WriteString("\n")

			line = strings.TrimSpace(line)
			if line == "" {
				return
			}

			var result ClaudeResult
			if err := json.Unmarshal([]byte(line), &result); err == nil {
				lastResult = result
			}

			if result.Type == "assistant" {
				var msg streamMessage
				if err := json.Unmarshal([]byte(line), &msg); err == nil {
					for _, content := range msg.Message.Content {
						if content.Type == "text" && content.Text != "" {
							lines := strings.Split(content.Text, "\n")
							for _, textLine := range lines {
								if strings.TrimSpace(textLine) != "" {
									util.Log.Printf("%s %s\n", display(), textLine)
								}
							}
						}
					}
				}
			}
		}, func(o *zshell.Options) {
			o.CloseStdin = true
		})
	if err != nil {
		return nil, fmt.Errorf("执行 Agent 失败: %v", err)
	}

	code := <-exitCodeChan

	if code != 0 {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = "Agent 以非零退出码退出但没有错误输出"
		}
		return nil, fmt.Errorf("Agent 退出码 %d: %s", code, errMsg)
	}

	return &lastResult, nil
}

func RunReviewerIteration(ctx *Context, display func() string) error {
	util.Log.Printf("%s 正在运行审查流程...\n", display())

	reviewPrompt := fmt.Sprintf(`%s

## USER REVIEW INSTRUCTIONS
<REVIEW_INSTRUCTIONS>
%s
</REVIEW_INSTRUCTIONS>
`, PromptReviewerContext, ctx.ReviewPrompt)

	result, err := RunClaudeIteration(ctx, reviewPrompt, display)
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

	result, err := RunClaudeIteration(ctx, ciFixPrompt, display)
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
		code, _, _, _ := zshell.ExecCommand(context.Background(), []string{
			"claude", "-p", PromptCommitMessage,
			"--allowedTools", "Bash(git)",
			"--dangerously-skip-permissions",
		}, nil, nil, nil)

		if code != 0 {
			return fmt.Errorf("提交 CI 修复失败")
		}
	}

	return nil
}
