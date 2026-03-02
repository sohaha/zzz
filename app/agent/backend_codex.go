package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sohaha/zlsgo/zshell"
	"github.com/sohaha/zzz/util"
)

const (
	codexCLICommand         = "codex"
	codexEventItemCompleted = "item.completed"
	codexEventError         = "error"
)

type CodexBackend struct{}

func (b *CodexBackend) Name() string {
	return "codex"
}

func (b *CodexBackend) Validate() error {
	if code, _, _, _ := zshell.ExecCommand(context.Background(),
		[]string{codexCLICommand, "--version"}, nil, nil, nil); code != 0 {
		return fmt.Errorf("未安装 Codex CLI: https://openai.com/codex")
	}

	return nil
}

func (b *CodexBackend) RunCommit(ctx *Context, prompt string, display func() string) error {
	if ctx.DryRun {
		return nil
	}

	toolName := fmt.Sprintf("%s commit", b.Name())
	command := []string{
		codexCLICommand, "exec",
		"--json",
		"--dangerously-bypass-approvals-and-sandbox",
		"--ask-for-approval",
		"never",
		prompt,
	}

	commandCtx, cancel := commandContext(ctx)
	defer cancel()
	started := logToolStart(display, toolName, formatToolDetail(ctx, prompt))
	code, _, _, err := zshell.ExecCommand(commandCtx, command, nil, nil, nil)
	logToolFinish(display, toolName, started, code, err, false)

	if err != nil || code != 0 {
		return fmt.Errorf("提交失败")
	}

	return nil
}

func (b *CodexBackend) RunIteration(ctx *Context, prompt string, display func() string) (*BackendResult, error) {
	if ctx.DryRun {
		util.Log.Println("(演习模式) 将调用 Codex exec")
		return &BackendResult{Result: "模拟响应"}, nil
	}

	command := []string{
		codexCLICommand, "exec",
		"--json",
		"--dangerously-bypass-approvals-and-sandbox",
		prompt,
	}
	if ctx.Model != "" {
		command = append(command, "--model", ctx.Model)
	}

	toolName := b.Name()
	buffers, writeString := handleBuffers(ctx, display, toolName)
	var result BackendResult
	hadError := false
	commandCtx, cancel := commandContext(ctx)
	defer cancel()
	started := logToolStart(display, toolName, formatToolDetail(ctx, prompt))
	code, err := runStreamCommand(commandCtx, command, func(line string, isStdout bool) {
		if line, isStdout = writeString(line, isStdout); !isStdout {
			return
		}

		var event codexEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return
		}

		switch event.Type {
		case codexEventItemCompleted:
			if event.Item != nil && event.Item.Text != "" {
				result.Result = event.Item.Text
				logToolOutputLines(display, event.Item.Text)
			}
		case codexEventError:
			if event.Error != nil {
				result.IsError = true
				result.Result = fmt.Sprintf("[%s] %s", event.Error.Type, event.Error.Message)
				hadError = true
			}
		}
	})
	logToolFinish(display, toolName, started, code, err, hadError)
	if err != nil {
		return nil, fmt.Errorf("执行 codex 失败: %v", err)
	}

	if code != 0 {
		return nil, formatExitCodeError("codex", code, &buffers.stderr)
	}

	return &result, nil
}

type codexEvent struct {
	Type  string      `json:"type"`
	Item  *codexItem  `json:"item,omitempty"`
	Error *codexError `json:"error,omitempty"`
}

type codexItem struct {
	Text string `json:"text"`
}

type codexError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
