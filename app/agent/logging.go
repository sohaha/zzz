package agent

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sohaha/zzz/util"
)

const logDetailLimit = 4096

func truncateText(text string, limit int) string {
	text = strings.TrimSpace(text)
	if limit <= 0 || text == "" {
		return text
	}
	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	runes := []rune(text)
	return string(runes[:limit]) + "... (truncated)"
}

func displayPrefix(display func() string) string {
	if display == nil {
		return ""
	}
	value := strings.TrimSpace(display())
	if value == "" {
		return ""
	}
	return value + " "
}

func formatToolDetail(ctx *Context, prompt string) string {
	parts := []string{fmt.Sprintf("prompt=%d", utf8.RuneCountInString(prompt))}
	if ctx != nil {
		if model := strings.TrimSpace(ctx.Model); model != "" {
			parts = append(parts, "model="+model)
		}
	}
	return strings.Join(parts, ",")
}

func logToolStart(display func() string, tool string, detail string) time.Time {
	prefix := displayPrefix(display)
	if detail = strings.TrimSpace(detail); detail != "" {
		util.Log.Printf("%s调用工具 %s (%s)\n", prefix, tool, truncateText(detail, logDetailLimit))
	} else {
		util.Log.Printf("%s调用工具 %s\n", prefix, tool)
	}
	return time.Now()
}

func logToolFinish(display func() string, tool string, started time.Time, exitCode int, err error, resultError bool) {
	prefix := displayPrefix(display)
	status := "成功"
	if err != nil || exitCode != 0 || resultError {
		status = "失败"
	}
	duration := FormatDuration(time.Since(started))
	util.Log.Printf("%s工具结束 %s exit=%d 耗时=%s 状态=%s\n", prefix, tool, exitCode, duration, status)
}

func logToolOutputLines(display func() string, text string) {
	if text == "" {
		return
	}
	prefix := displayPrefix(display)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		util.Log.Printf("%s%s\n", prefix, line)
	}
}
