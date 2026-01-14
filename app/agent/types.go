package agent

import (
	"time"
)

const (
	CompletionSignal      = "CONTINUOUS_PROJECT_COMPLETE"
	MaxConsecutiveErrors  = 3
	IterationDelaySeconds = 1
)

type Context struct {
	Prompt              string
	Model               string
	MaxRuns             int
	MaxCost             float64
	MaxDuration         time.Duration
	Owner               string
	Repo                string
	EnableCommits       bool
	EnableBranches      bool
	BranchPrefix        string
	MergeStrategy       string
	NotesFile           string
	DryRun              bool
	CompletionSignal    string
	CompletionThreshold int
	ReviewPrompt        string
	CIRetryEnabled      bool
	CIRetryMax          int
	WorktreeName        string
	WorktreeBaseDir     string
	CleanupWorktree     bool
	Backend             AIBackend

	SuccessfulIterations int
	ErrorCount           int
	ExtraIterations      int
	TotalCost            float64
	CompletionCount      int
	StartTime            time.Time
}

type BackendResult struct {
	Type         string  `json:"type"`
	Result       string  `json:"result"`
	IsError      bool    `json:"is_error"`
	TotalCostUSD float64 `json:"total_cost_usd"`
}

type PRCheckResult struct {
	State  string `json:"state"`
	Bucket string `json:"bucket"`
}

type PRInfo struct {
	ReviewDecision string `json:"reviewDecision"`
	ReviewRequests []struct {
		Login string `json:"login"`
	} `json:"reviewRequests"`
	State      string `json:"state"`
	HeadRefOid string `json:"headRefOid"`
}

type WorkflowRun struct {
	DatabaseId int `json:"databaseId"`
}
