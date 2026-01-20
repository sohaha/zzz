package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/sohaha/zlsgo/zshell"
)

type GitHubProvider struct{}

func (p *GitHubProvider) Name() string {
	return "github"
}

func (p *GitHubProvider) Validate(ctx *Context) error {
	// 检查 gh CLI 是否安装
	code, _, _, _ := zshell.ExecCommand(runContext(ctx),
		[]string{"gh", "--version"}, nil, nil, nil)
	if code != 0 {
		return fmt.Errorf("gh CLI 未安装，请安装: https://cli.github.com/")
	}

	// 检查 gh 认证状态
	code, _, _, _ = zshell.ExecCommand(runContext(ctx),
		[]string{"gh", "auth", "status"}, nil, nil, nil)
	if code != 0 {
		return fmt.Errorf("gh CLI 未认证，请运行: gh auth login")
	}

	return nil
}

func (p *GitHubProvider) DetectFromGit() (*RepositoryInfo, error) {
	code, stdout, _, err := zshell.ExecCommand(context.Background(),
		[]string{"git", "remote", "get-url", "origin"}, nil, nil, nil)
	if err != nil || code != 0 {
		return nil, fmt.Errorf("获取 git remote 失败")
	}

	remoteURL := strings.TrimSpace(stdout)

	// 支持 HTTPS 和 SSH URL
	httpsRe := regexp.MustCompile(`https://github\.com/([^/]+)/([^/]+?)(?:\.git)?$`)
	sshRe := regexp.MustCompile(`git@github\.com:([^/]+)/([^/]+?)(?:\.git)?$`)

	if matches := httpsRe.FindStringSubmatch(remoteURL); len(matches) == 3 {
		return &RepositoryInfo{
			Provider: "github",
			Owner:    matches[1],
			Repo:     matches[2],
		}, nil
	}
	if matches := sshRe.FindStringSubmatch(remoteURL); len(matches) == 3 {
		return &RepositoryInfo{
			Provider: "github",
			Owner:    matches[1],
			Repo:     matches[2],
		}, nil
	}

	return nil, fmt.Errorf("无法解析 GitHub URL: %s", remoteURL)
}

func (p *GitHubProvider) CreatePullRequest(ctx *Context, opts *PRCreateOptions) (*PullRequestInfo, error) {
	code, prOutput, stderr, _ := zshell.ExecCommand(runContext(ctx), []string{
		"gh", "pr", "create",
		"--repo", fmt.Sprintf("%s/%s", ctx.RepoInfo.Owner, ctx.RepoInfo.Repo),
		"--title", opts.Title,
		"--body", opts.Body,
		"--base", opts.TargetBranch,
	}, nil, nil, nil)

	if code != 0 {
		return nil, fmt.Errorf("创建 PR 失败: %s", stderr)
	}

	prNumberRe := regexp.MustCompile(`(?:pull/|#)(\d+)`)
	matches := prNumberRe.FindStringSubmatch(prOutput)
	if len(matches) < 2 {
		return nil, fmt.Errorf("无法解析 PR 编号")
	}

	prNumber := matches[1]
	prURL := strings.TrimSpace(prOutput)

	prNumberInt, err := strconv.Atoi(prNumber)
	if err != nil {
		return nil, fmt.Errorf("无效的 PR 编号: %v", err)
	}

	return &PullRequestInfo{
		ID:     prNumber,
		Number: prNumberInt,
		URL:    prURL,
	}, nil
}

func (p *GitHubProvider) MergePullRequest(ctx *Context, prID string, strategy MergeStrategy) error {
	mergeFlag := "--squash"
	switch strategy {
	case MergeStrategyMerge:
		mergeFlag = "--merge"
	case MergeStrategyRebase:
		mergeFlag = "--rebase"
	}

	code, _, stderr, _ := zshell.ExecCommand(runContext(ctx), []string{
		"gh", "pr", "merge", prID,
		"--repo", fmt.Sprintf("%s/%s", ctx.RepoInfo.Owner, ctx.RepoInfo.Repo),
		mergeFlag,
		"--delete-branch",
	}, nil, nil, nil)

	if code != 0 {
		return fmt.Errorf("合并 PR 失败: %s", stderr)
	}

	return nil
}

func (p *GitHubProvider) GetPRStatus(ctx *Context, prID string) (*PRStatus, error) {
	code, output, _, _ := zshell.ExecCommand(runContext(ctx), []string{
		"gh", "pr", "view", prID,
		"--repo", fmt.Sprintf("%s/%s", ctx.RepoInfo.Owner, ctx.RepoInfo.Repo),
		"--json", "state,statusCheckRollup",
	}, nil, nil, nil)

	if code != 0 {
		return nil, fmt.Errorf("获取 PR 状态失败")
	}

	var result struct {
		State             string
		StatusCheckRollup []struct {
			Status     string
			Conclusion string
			WorkflowID int `json:"workflowRunId"`
		}
	}

	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return nil, err
	}

	status := &PRStatus{
		State: result.State,
	}

	// 检查 CI 状态
	if len(result.StatusCheckRollup) > 0 {
		allSuccess := true
		var failedRunID string

		for _, check := range result.StatusCheckRollup {
			if check.Conclusion == "failure" {
				allSuccess = false
				if check.WorkflowID != 0 {
					failedRunID = fmt.Sprintf("%d", check.WorkflowID)
				}
			}
		}

		if allSuccess {
			status.CIStatus = "success"
		} else {
			status.CIStatus = "failure"
			status.CIRunID = failedRunID
		}
	}

	return status, nil
}

func (p *GitHubProvider) SupportsBranches() bool {
	return true
}

func (p *GitHubProvider) SupportsCI() bool {
	return true
}

func (p *GitHubProvider) GetPRChecks(ctx *Context, prID string) ([]PRCheckResult, error) {
	code, checksJSON, _, _ := zshell.ExecCommand(runContext(ctx), []string{
		"gh", "pr", "checks", prID,
		"--repo", fmt.Sprintf("%s/%s", ctx.RepoInfo.Owner, ctx.RepoInfo.Repo),
		"--json", "state,bucket",
	}, nil, nil, nil)

	if code != 0 || checksJSON == "" {
		return nil, fmt.Errorf("获取 PR 检查失败")
	}

	var checks []PRCheckResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(checksJSON)), &checks); err != nil {
		return nil, err
	}

	return checks, nil
}

func (p *GitHubProvider) GetPRReviewStatus(ctx *Context, prID string) (*PRReviewStatus, error) {
	code, prInfoJSON, _, _ := zshell.ExecCommand(runContext(ctx), []string{
		"gh", "pr", "view", prID,
		"--repo", fmt.Sprintf("%s/%s", ctx.RepoInfo.Owner, ctx.RepoInfo.Repo),
		"--json", "reviewDecision,reviewRequests",
	}, nil, nil, nil)

	if code != 0 {
		return nil, fmt.Errorf("获取 PR 审查状态失败")
	}

	var prInfo struct {
		ReviewDecision string `json:"reviewDecision"`
		ReviewRequests []struct {
			Login string `json:"login"`
		} `json:"reviewRequests"`
	}

	if err := json.Unmarshal([]byte(strings.TrimSpace(prInfoJSON)), &prInfo); err != nil {
		return nil, err
	}

	reviewers := make([]string, len(prInfo.ReviewRequests))
	for i, req := range prInfo.ReviewRequests {
		reviewers[i] = req.Login
	}

	return &PRReviewStatus{
		ReviewDecision: prInfo.ReviewDecision,
		ReviewRequests: reviewers,
	}, nil
}

func (p *GitHubProvider) UpdatePRBranch(ctx *Context, prID string) error {
	code, updateOutput, _, _ := zshell.ExecCommand(runContext(ctx), []string{
		"gh", "pr", "update-branch", prID,
		"--repo", fmt.Sprintf("%s/%s", ctx.RepoInfo.Owner, ctx.RepoInfo.Repo),
	}, nil, nil, nil)

	if code != 0 {
		if strings.Contains(updateOutput, "already up-to-date") || strings.Contains(updateOutput, "is up to date") {
			return nil
		}
		return fmt.Errorf("更新 PR 分支失败: %s", updateOutput)
	}

	return nil
}

func (p *GitHubProvider) ClosePullRequest(ctx *Context, prID string, deleteBranch bool) error {
	args := []string{
		"gh", "pr", "close", prID,
		"--repo", fmt.Sprintf("%s/%s", ctx.RepoInfo.Owner, ctx.RepoInfo.Repo),
	}
	if deleteBranch {
		args = append(args, "--delete-branch")
	}

	code, _, stderr, _ := zshell.ExecCommand(runContext(ctx), args, nil, nil, nil)
	if code != 0 {
		return fmt.Errorf("关闭 PR 失败: %s", stderr)
	}

	return nil
}

func (p *GitHubProvider) GetFailedWorkflowRun(ctx *Context, prID string) (string, error) {
	code, prInfoJSON, _, _ := zshell.ExecCommand(runContext(ctx), []string{
		"gh", "pr", "view", prID,
		"--repo", fmt.Sprintf("%s/%s", ctx.RepoInfo.Owner, ctx.RepoInfo.Repo),
		"--json", "headRefOid",
	}, nil, nil, nil)

	if code != 0 {
		return "", fmt.Errorf("获取 PR 信息失败")
	}

	var prInfo struct {
		HeadRefOid string `json:"headRefOid"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(prInfoJSON)), &prInfo); err != nil {
		return "", err
	}

	headSHA := prInfo.HeadRefOid
	if headSHA == "" {
		return "", fmt.Errorf("未找到 head SHA")
	}

	code, runsJSON, _, _ := zshell.ExecCommand(runContext(ctx), []string{
		"gh", "run", "list",
		"--repo", fmt.Sprintf("%s/%s", ctx.RepoInfo.Owner, ctx.RepoInfo.Repo),
		"--commit", headSHA,
		"--status", "failure",
		"--limit", "1",
		"--json", "databaseId",
	}, nil, nil, nil)

	if code != 0 {
		return "", fmt.Errorf("列出运行记录失败")
	}

	var runs []struct {
		DatabaseId int `json:"databaseId"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(runsJSON)), &runs); err != nil {
		return "", err
	}

	if len(runs) == 0 {
		return "", fmt.Errorf("未找到失败的运行记录")
	}

	return fmt.Sprintf("%d", runs[0].DatabaseId), nil
}
