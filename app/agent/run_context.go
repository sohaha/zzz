package agent

import (
	"context"
	"time"
)

func setupRunContext(ctx *Context) {
	if ctx == nil {
		return
	}
	if ctx.RunContext == nil {
		ctx.RunContext = context.Background()
	}
	if ctx.MaxDuration <= 0 {
		return
	}
	start := ctx.StartTime
	if start.IsZero() {
		start = time.Now()
		ctx.StartTime = start
	}
	deadline := start.Add(ctx.MaxDuration)
	ctx.RunContext, ctx.RunCancel = context.WithDeadline(ctx.RunContext, deadline)
}

func runContextDone(ctx *Context) bool {
	return ctx != nil && ctx.RunContext != nil && ctx.RunContext.Err() != nil
}

func runContextErr(ctx *Context) error {
	if ctx == nil || ctx.RunContext == nil {
		return nil
	}
	return ctx.RunContext.Err()
}

func runContext(ctx *Context) context.Context {
	if ctx != nil && ctx.RunContext != nil {
		return ctx.RunContext
	}
	return context.Background()
}

func sleepWithContext(ctx *Context, d time.Duration) bool {
	if ctx == nil || ctx.RunContext == nil {
		time.Sleep(d)
		return true
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.RunContext.Done():
		return false
	case <-timer.C:
		return true
	}
}
