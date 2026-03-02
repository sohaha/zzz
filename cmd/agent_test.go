package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestStdinSizeLimit(t *testing.T) {
	tests := []struct {
		name      string
		inputSize int
		wantErr   bool
	}{
		{"small input", 100, false},
		{"1MB exactly", 1024 * 1024, false},
		{"over 1MB", 1024*1024 + 1, true},
		{"2MB", 2 * 1024 * 1024, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldStdin := os.Stdin
			defer func() { os.Stdin = oldStdin }()

			r, w, err := os.Pipe()
			if err != nil {
				t.Fatalf("failed to create pipe: %v", err)
			}
			os.Stdin = r

			input := strings.Repeat("a", tt.inputSize)
			go func() {
				w.Write([]byte(input))
				w.Close()
			}()

			agentPrompt = "-"

			const maxStdinSize = 1 * 1024 * 1024
			limitedReader := io.LimitReader(os.Stdin, maxStdinSize+1)
			stdinBytes, err := io.ReadAll(limitedReader)
			if err != nil {
				t.Errorf("ReadAll failed: %v", err)
				return
			}

			hasError := len(stdinBytes) > maxStdinSize

			if hasError != tt.wantErr {
				t.Errorf("stdin size %d: hasError = %v, want %v", tt.inputSize, hasError, tt.wantErr)
			}
		})
	}
}

func TestStdinEmpty(t *testing.T) {
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdin = r

	w.Close()

	const maxStdinSize = 1 * 1024 * 1024
	limitedReader := io.LimitReader(os.Stdin, maxStdinSize+1)
	stdinBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	prompt := strings.TrimSpace(string(stdinBytes))
	if prompt != "" {
		t.Errorf("expected empty prompt, got %q", prompt)
	}
}

func TestStdinBoundary(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantLen int
	}{
		{"whitespace only", "   \n\t  ", 0},
		{"with newlines", "hello\nworld\n", 11},
		{"exactly 1MB minus 1", strings.Repeat("x", 1024*1024-1), 1024*1024 - 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldStdin := os.Stdin
			defer func() { os.Stdin = oldStdin }()

			r, w, err := os.Pipe()
			if err != nil {
				t.Fatalf("failed to create pipe: %v", err)
			}
			os.Stdin = r

			go func() {
				w.Write([]byte(tt.content))
				w.Close()
			}()

			var buf bytes.Buffer
			io.Copy(&buf, os.Stdin)

			result := strings.TrimSpace(buf.String())
			if len(result) != tt.wantLen {
				t.Errorf("content length = %d, want %d", len(result), tt.wantLen)
			}
		})
	}
}

func TestAgentEnableBranchesRequiresCommits(t *testing.T) {
	restore := setAgentDefaults()
	defer restore()

	agentEnableBranches = true
	agentEnableCommits = false

	err := runAgentCommand(agentCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "--enable-branches 需要同时启用 --enable-commits") {
		t.Fatalf("expected enable-branches error, got %v", err)
	}
}

func TestAgentRepoIDConflict(t *testing.T) {
	restore := setAgentDefaults()
	defer restore()

	agentRepoID = "owner/repo"
	agentRepoOwner = "owner"
	agentRepoName = "repo"

	err := runAgentCommand(agentCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "请不要同时使用 --repo-id 与 --owner/--repo") {
		t.Fatalf("expected repo-id conflict error, got %v", err)
	}
}

func TestAgentOwnerRepoPairRequired(t *testing.T) {
	restore := setAgentDefaults()
	defer restore()

	agentRepoOwner = "owner"
	agentRepoName = ""

	err := runAgentCommand(agentCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "需要同时指定 --owner 与 --repo") {
		t.Fatalf("expected owner/repo pair error, got %v", err)
	}
}

func setAgentDefaults() func() {
	oldPrompt := agentPrompt
	oldMaxRuns := agentMaxRuns
	oldMaxCost := agentMaxCost
	oldMaxDuration := agentMaxDuration
	oldEnableCommits := agentEnableCommits
	oldEnableBranches := agentEnableBranches
	oldRepoID := agentRepoID
	oldRepoOwner := agentRepoOwner
	oldRepoName := agentRepoName
	oldListWorktrees := agentListWorktrees
	oldCompletionSignal := agentCompletionSignal

	agentPrompt = "test"
	agentMaxRuns = 1
	agentMaxCost = 0
	agentMaxDuration = ""
	agentEnableCommits = false
	agentEnableBranches = false
	agentRepoID = ""
	agentRepoOwner = ""
	agentRepoName = ""
	agentListWorktrees = false
	agentCompletionSignal = ""

	return func() {
		agentPrompt = oldPrompt
		agentMaxRuns = oldMaxRuns
		agentMaxCost = oldMaxCost
		agentMaxDuration = oldMaxDuration
		agentEnableCommits = oldEnableCommits
		agentEnableBranches = oldEnableBranches
		agentRepoID = oldRepoID
		agentRepoOwner = oldRepoOwner
		agentRepoName = oldRepoName
		agentListWorktrees = oldListWorktrees
		agentCompletionSignal = oldCompletionSignal
	}
}
