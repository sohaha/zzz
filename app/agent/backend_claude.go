package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sohaha/zlsgo/zshell"
	"github.com/sohaha/zzz/util"
)

const (
	claudeCLICommand = "claude"
)

type ClaudeCodeBackend struct{}

func (b *ClaudeCodeBackend) Name() string {
	return "claude-code"
}

func (b *ClaudeCodeBackend) Validate() error {
	if code, _, _, _ := zshell.ExecCommand(context.Background(),
		[]string{claudeCLICommand, "--version"}, nil, nil, nil); code != 0 {
		return fmt.Errorf("未安装 Claude Code: https://claude.ai/code")
	}
	return nil
}

func (b *ClaudeCodeBackend) RunCommit(ctx *Context, prompt string, display func() string) error {
	if ctx.DryRun {
		return nil
	}

	toolName := fmt.Sprintf("%s commit", b.Name())
	commandCtx, cancel := commandContext(ctx)
	defer cancel()
	started := logToolStart(display, toolName, formatToolDetail(ctx, prompt))
	code, _, _, err := zshell.ExecCommand(commandCtx, []string{
		claudeCLICommand, "-p", prompt,
		"--allowedTools", "Bash(git)",
		"--dangerously-skip-permissions",
	}, nil, nil, nil)
	logToolFinish(display, toolName, started, code, err, false)

	if err != nil || code != 0 {
		return fmt.Errorf("提交失败")
	}

	return nil
}

func (b *ClaudeCodeBackend) RunIteration(ctx *Context, prompt string, display func() string) (*BackendResult, error) {
	if ctx.DryRun {
		util.Log.Println("(演习模式) 将运行 Agent")
		return &BackendResult{
			Result: "Agent 的模拟响应",
		}, nil
	}

	command := []string{claudeCLICommand, "-p", prompt, "--dangerously-skip-permissions", "--output-format", "stream-json", "--verbose"}
	if ctx.Model != "" {
		command = append(command, "--model", ctx.Model)
	}

	toolName := b.Name()
	buffers, writeString := handleBuffers(ctx, display, toolName)
	var lastResult BackendResult
	hadError := false
	commandCtx, cancel := commandContext(ctx)
	defer cancel()
	started := logToolStart(display, toolName, formatToolDetail(ctx, prompt))
	code, err := runStreamCommand(commandCtx, command, func(line string, isStdout bool) {
		if line, isStdout = writeString(line, isStdout); !isStdout {
			return
		}

		var result BackendResult
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			return
		}

		lastResult = result
		if result.IsError {
			hadError = true
		}

		if result.Type == "assistant" {
			var msg streamMessage
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				return
			}

			for _, content := range msg.Message.Content {
				if content.Type != "text" || content.Text == "" {
					continue
				}

				logToolOutputLines(display, content.Text)
			}
		}
	})
	logToolFinish(display, toolName, started, code, err, hadError)
	if err != nil {
		return nil, fmt.Errorf("执行 Agent 失败: %v", err)
	}

	if code != 0 {
		return nil, formatExitCodeError("Agent", code, &buffers.stderr)
	}

	return &lastResult, nil
}

type streamMessage struct {
	Type    string `json:"type"`
	Message struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message"`
}
