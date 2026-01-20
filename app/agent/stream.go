package agent

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/sohaha/zlsgo/zshell"
)

func runStreamCommand(ctx context.Context, command []string, onLine func(line string, isStdout bool)) (int, error) {
	if len(command) == 0 || command[0] == "" {
		return -1, fmt.Errorf("no such command")
	}

	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	if zshell.Env != nil {
		cmd.Env = zshell.Env
	} else {
		cmd.Env = os.Environ()
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return -1, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return -1, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return -1, err
	}

	if err := cmd.Start(); err != nil {
		return -1, err
	}

	_ = stdin.Close()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		readStreamLines(stdout, true, onLine)
	}()
	go func() {
		defer wg.Done()
		readStreamLines(stderr, false, onLine)
	}()

	waitErr := cmd.Wait()
	wg.Wait()

	exitCode := -1
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	if waitErr != nil {
		if ctx != nil && ctx.Err() != nil {
			return exitCode, ctx.Err()
		}
	}

	return exitCode, nil
}

func commandContext(ctx *Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		return context.Background(), func() {}
	}
	if ctx.RunContext != nil {
		return context.WithCancel(ctx.RunContext)
	}
	if ctx.MaxDuration <= 0 {
		return context.Background(), func() {}
	}
	start := ctx.StartTime
	if start.IsZero() {
		start = time.Now()
	}
	remaining := ctx.MaxDuration - time.Since(start)
	if remaining < 0 {
		remaining = 0
	}
	return context.WithTimeout(context.Background(), remaining)
}

func readStreamLines(pipe io.ReadCloser, isStdout bool, onLine func(line string, isStdout bool)) {
	reader := bufio.NewReader(pipe)
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			line = strings.TrimRight(line, "\r\n")
			onLine(line, isStdout)
		}
		if err != nil {
			return
		}
	}
}
