package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sohaha/zlsgo/zshell"
	"github.com/sohaha/zzz/util"
)

const (
	opencodeCLICommand     = "opencode"
	opencodeTypeText       = "text"
	opencodeTypeStepFinish = "step_finish"
)

type OpencodeBackend struct{}

func (b *OpencodeBackend) Name() string {
	return "opencode"
}

func (b *OpencodeBackend) Validate() error {
	if code, _, _, _ := zshell.ExecCommand(context.Background(),
		[]string{opencodeCLICommand, "--version"}, nil, nil, nil); code != 0 {
		return fmt.Errorf("未安装 Opencode CLI")
	}
	return nil
}

func (b *OpencodeBackend) RunCommit(ctx *Context, prompt string) error {
	if ctx.DryRun {
		return nil
	}

	command := []string{
		opencodeCLICommand, "run",
		"--format", "json",
		prompt,
	}

	commandCtx, cancel := commandContext(ctx)
	defer cancel()
	code, _, _, _ := zshell.ExecCommand(commandCtx, command, nil, nil, nil)

	if code != 0 {
		return fmt.Errorf("提交失败")
	}

	return nil
}

func (b *OpencodeBackend) RunIteration(ctx *Context, prompt string, display func() string) (*BackendResult, error) {
	if ctx.DryRun {
		util.Log.Println("(演习模式) 将调用 Opencode run")
		return &BackendResult{Result: "模拟响应"}, nil
	}

	command := []string{
		opencodeCLICommand, "run",
		"--format", "json",
		prompt,
	}
	if ctx.Model != "" {
		command = append(command, "--model", ctx.Model)
	}

	buffers, writeString := handleBuffers(ctx, display)
	var result BackendResult
	commandCtx, cancel := commandContext(ctx)
	defer cancel()
	code, err := runStreamCommand(commandCtx, command, func(line string, isStdout bool) {
		if line, isStdout = writeString(line, isStdout); !isStdout {
			return
		}

		var event opencodeEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return
		}

		switch event.Type {
		case opencodeTypeText:
			if event.Part.Text != "" {
				result.Result = event.Part.Text
				util.Log.Printf("%s %s\n", display(), event.Part.Text)
			}
		case opencodeTypeStepFinish:
			if event.Part.Reason == "error" {
				result.IsError = true
			}
		}
	})
	if err != nil {
		return nil, fmt.Errorf("执行 opencode 失败: %v", err)
	}

	if code != 0 {
		return nil, formatExitCodeError("opencode", code, &buffers.stderr)
	}

	return &result, nil
}

type opencodeEvent struct {
	Type      string       `json:"type"`
	Timestamp int64        `json:"timestamp"`
	SessionID string       `json:"sessionID"`
	Part      opencodePart `json:"part"`
}

type opencodePart struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionID"`
	MessageID string `json:"messageID"`
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Snapshot  string `json:"snapshot,omitempty"`
}
