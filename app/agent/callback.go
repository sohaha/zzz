package agent

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sohaha/zlsgo/zshell"
	"github.com/sohaha/zzz/util"
)

func ExecuteCallbacks(agentCtx *Context, mainErr error) {
	if mainErr == nil && agentCtx.OnComplete != "" {
		executeCallback(agentCtx, mainErr, agentCtx.OnComplete, "on-complete")
	} else if mainErr != nil && agentCtx.OnError != "" {
		executeCallback(agentCtx, mainErr, agentCtx.OnError, "on-error")
	}

	if agentCtx.OnFinish != "" {
		executeCallback(agentCtx, mainErr, agentCtx.OnFinish, "on-finish")
	}
}

func executeCallback(agentCtx *Context, mainErr error, command string, hookType string) {
	envVars := buildCallbackEnv(agentCtx, mainErr)

	envSlice := os.Environ()
	for k, v := range envVars {
		envSlice = append(envSlice, fmt.Sprintf("%s=%s", k, v))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	code, stdout, stderr, err := zshell.RunContext(ctx, command, func(o *zshell.Options) {
		o.Env = envSlice
	})
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			util.Log.Warnf("回调超时（5分钟）\n")
		} else {
			util.Log.Warnf("回调执行失败: %v\n", err)
		}
		return
	}

	if code != 0 {
		util.Log.Warnf("回调执行失败 (退出码 %d): %s\n", code, strings.TrimSpace(stdout+stderr))
		return
	}

	if trimmed := strings.TrimSpace(stdout); trimmed != "" {
		util.Log.Printf("回调输出: %s\n", trimmed)
	}
}

func buildCallbackEnv(agentCtx *Context, mainErr error) map[string]string {
	env := make(map[string]string)

	env["AGENT_PROMPT"] = agentCtx.Prompt
	env["AGENT_ITERATIONS"] = fmt.Sprintf("%d", agentCtx.SuccessfulIterations)
	env["AGENT_TOTAL_COST"] = fmt.Sprintf("%.3f", agentCtx.TotalCost)

	if !agentCtx.StartTime.IsZero() {
		env["AGENT_DURATION"] = FormatDuration(time.Since(agentCtx.StartTime))
	}

	if mainErr == nil {
		env["AGENT_STATUS"] = "success"
		env["AGENT_EXIT_CODE"] = "0"
	} else {
		env["AGENT_STATUS"] = "error"
		env["AGENT_EXIT_CODE"] = "1"

		errMsg := strings.ReplaceAll(mainErr.Error(), "\n", " ")
		if len(errMsg) > 1024 {
			errMsg = errMsg[:1024] + "... (truncated)"
		}
		env["AGENT_ERROR"] = errMsg
	}

	if agentCtx.Model != "" {
		env["AGENT_MODEL"] = agentCtx.Model
	}
	if agentCtx.WorktreeName != "" {
		env["AGENT_WORKTREE"] = agentCtx.WorktreeName
	}

	cwd, err := os.Getwd()
	if err == nil {
		env["AGENT_CWD"] = cwd
	}

	return env
}
