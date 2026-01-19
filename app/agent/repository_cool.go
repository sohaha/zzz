package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/sohaha/zlsgo/zshell"
	"github.com/sohaha/zzz/util"
)

type CoolProvider struct {
	apiKey   string
	Endpoint string
}

func (p *CoolProvider) Name() string {
	return "cool"
}

func (p *CoolProvider) Validate(ctx *Context) error {
	if p.apiKey == "" {
		return fmt.Errorf("CNB_TOKEN 未设置")
	}

	util.Log.Debugf("验证 Cool API 连接: %s", p.Endpoint)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(p.Endpoint + "/swagger.json")
	if err != nil {
		return fmt.Errorf("无法连接到 Cool API: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("Cool API 返回错误状态: %d", resp.StatusCode)
	}

	return nil
}

func (p *CoolProvider) DetectFromGit() (*RepositoryInfo, error) {
	code, stdout, _, err := zshell.ExecCommand(context.Background(),
		[]string{"git", "remote", "get-url", "origin"}, nil, nil, nil)
	if err != nil || code != 0 {
		return nil, fmt.Errorf("获取 git remote 失败")
	}

	remoteURL := strings.TrimSpace(stdout)

	// CNB Cool 只支持 HTTPS: https://cnb.cool/owner/repo
	httpsRe := regexp.MustCompile(`https://cnb\.cool/([^/]+)/([^/]+?)(?:\.git)?$`)

	if matches := httpsRe.FindStringSubmatch(remoteURL); len(matches) == 3 {
		return &RepositoryInfo{
			Provider: "cool",
			Owner:    matches[1],
			Repo:     matches[2], // 只取仓库名
		}, nil
	}

	return nil, fmt.Errorf("无法解析 Cool URL (格式: https://cnb.cool/owner/repo): %s", remoteURL)
}

func (p *CoolProvider) CreatePullRequest(ctx *Context, opts *PRCreateOptions) (*PullRequestInfo, error) {
	reqBody := map[string]any{
		"title": opts.Title,
		"body":  opts.Body,
		"head":  opts.SourceBranch,
		"base":  opts.TargetBranch,
	}

	jsonData, _ := json.Marshal(reqBody)

	repoPath := fmt.Sprintf("%s/%s", ctx.RepoInfo.Owner, ctx.RepoInfo.Repo)
	url := fmt.Sprintf("%s/%s/-/pulls", p.Endpoint, repoPath)

	util.Log.Debugf("创建 PR: POST %s", url)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("创建 Pull Request 失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API 错误 %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Number int    `json:"number"`
		URL    string `json:"html_url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	util.Log.Printf("PR 创建成功: #%d %s", result.Number, result.URL)

	return &PullRequestInfo{
		ID:     fmt.Sprintf("%d", result.Number),
		Number: result.Number,
		URL:    result.URL,
	}, nil
}

func (p *CoolProvider) MergePullRequest(ctx *Context, prID string, strategy MergeStrategy) error {
	reqBody := map[string]any{
		"merge_method": string(strategy),
	}

	jsonData, _ := json.Marshal(reqBody)
	repoPath := fmt.Sprintf("%s/%s", ctx.RepoInfo.Owner, ctx.RepoInfo.Repo)
	url := fmt.Sprintf("%s/%s/-/pulls/%s/merge", p.Endpoint, repoPath, prID)

	util.Log.Debugf("合并 PR: PUT %s", url)

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("合并失败: %s", string(body))
	}

	util.Log.Printf("PR #%s 合并成功", prID)
	return nil
}

func (p *CoolProvider) GetPRStatus(ctx *Context, prID string) (*PRStatus, error) {
	repoPath := fmt.Sprintf("%s/%s", ctx.RepoInfo.Owner, ctx.RepoInfo.Repo)
	url := fmt.Sprintf("%s/%s/-/pulls/%s", p.Endpoint, repoPath, prID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		State  string `json:"state"`
		Merged bool   `json:"merged"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	status := &PRStatus{
		State: result.State,
	}

	// 获取 CI 状态
	ciURL := fmt.Sprintf("%s/%s/-/pulls/%s/commit-statuses", p.Endpoint, repoPath, prID)
	ciReq, _ := http.NewRequest("GET", ciURL, nil)
	ciReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	ciResp, err := client.Do(ciReq)
	if err == nil {
		defer ciResp.Body.Close()

		var statuses []struct {
			State   string `json:"state"`
			Context string `json:"context"`
		}

		if ciResp.StatusCode == 200 {
			json.NewDecoder(ciResp.Body).Decode(&statuses)

			if len(statuses) == 0 {
				status.CIStatus = "pending"
			} else {
				allSuccess := true
				hasPending := false

				for _, s := range statuses {
					if s.State == "failure" || s.State == "error" {
						allSuccess = false
						break
					}
					if s.State == "pending" {
						hasPending = true
					}
				}

				if !allSuccess {
					status.CIStatus = "failure"
				} else if hasPending {
					status.CIStatus = "pending"
				} else {
					status.CIStatus = "success"
				}
			}
		}
	}

	return status, nil
}

func (p *CoolProvider) SupportsBranches() bool {
	return true
}

func (p *CoolProvider) SupportsCI() bool {
	return true // 支持通过 commit-statuses 端点
}

func (p *CoolProvider) GetPRChecks(ctx *Context, prID string) ([]PRCheckResult, error) {
	repoPath := fmt.Sprintf("%s/%s", ctx.RepoInfo.Owner, ctx.RepoInfo.Repo)
	url := fmt.Sprintf("%s/%s/-/pulls/%s/commit-statuses", p.Endpoint, repoPath, prID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("no checks configured")
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("获取检查失败: %s", string(body))
	}

	var statuses []struct {
		State   string `json:"state"`
		Context string `json:"context"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&statuses); err != nil {
		return nil, err
	}

	checks := make([]PRCheckResult, len(statuses))
	for i, s := range statuses {
		bucket := "pending"
		switch s.State {
		case "success":
			bucket = "pass"
		case "failure", "error":
			bucket = "fail"
		case "pending":
			bucket = "pending"
		}

		checks[i] = PRCheckResult{
			State:  s.State,
			Bucket: bucket,
		}
	}

	return checks, nil
}

func (p *CoolProvider) GetPRReviewStatus(ctx *Context, prID string) (*PRReviewStatus, error) {
	repoPath := fmt.Sprintf("%s/%s", ctx.RepoInfo.Owner, ctx.RepoInfo.Repo)
	url := fmt.Sprintf("%s/%s/-/pulls/%s/reviews", p.Endpoint, repoPath, prID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return &PRReviewStatus{}, nil
	}

	if resp.StatusCode != 200 {
		return &PRReviewStatus{}, nil
	}

	var reviews []struct {
		State string `json:"state"`
		User  struct {
			Login string `json:"login"`
		} `json:"user"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&reviews); err != nil {
		return &PRReviewStatus{}, nil
	}

	reviewStatus := &PRReviewStatus{
		ReviewRequests: []string{},
	}

	for _, r := range reviews {
		if r.State == "APPROVED" {
			reviewStatus.ReviewDecision = "APPROVED"
			break
		} else if r.State == "CHANGES_REQUESTED" {
			reviewStatus.ReviewDecision = "CHANGES_REQUESTED"
		}
	}

	return reviewStatus, nil
}

func (p *CoolProvider) UpdatePRBranch(ctx *Context, prID string) error {
	repoPath := fmt.Sprintf("%s/%s", ctx.RepoInfo.Owner, ctx.RepoInfo.Repo)
	url := fmt.Sprintf("%s/%s/-/pulls/%s/update-branch", p.Endpoint, repoPath, prID)

	req, err := http.NewRequest("PUT", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 422 {
		body, _ := io.ReadAll(resp.Body)
		if strings.Contains(string(body), "already up-to-date") || strings.Contains(string(body), "is up to date") {
			return nil
		}
	}

	if resp.StatusCode != 200 && resp.StatusCode != 202 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("更新分支失败: %s", string(body))
	}

	return nil
}

func (p *CoolProvider) ClosePullRequest(ctx *Context, prID string, deleteBranch bool) error {
	reqBody := map[string]any{
		"state": "closed",
	}

	jsonData, _ := json.Marshal(reqBody)
	repoPath := fmt.Sprintf("%s/%s", ctx.RepoInfo.Owner, ctx.RepoInfo.Repo)
	url := fmt.Sprintf("%s/%s/-/pulls/%s", p.Endpoint, repoPath, prID)

	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("关闭 PR 失败: %s", string(body))
	}

	if deleteBranch {
		var prInfo struct {
			Head struct {
				Ref string `json:"ref"`
			} `json:"head"`
		}

		resp2, err := http.Get(url)
		if err == nil {
			defer resp2.Body.Close()
			if resp2.StatusCode == 200 {
				json.NewDecoder(resp2.Body).Decode(&prInfo)

				if prInfo.Head.Ref != "" {
					branchURL := fmt.Sprintf("%s/%s/-/branches/%s", p.Endpoint, repoPath, prInfo.Head.Ref)
					delReq, _ := http.NewRequest("DELETE", branchURL, nil)
					delReq.Header.Set("Authorization", "Bearer "+p.apiKey)
					client.Do(delReq)
				}
			}
		}
	}

	util.Log.Printf("PR #%s 已关闭", prID)
	return nil
}

func (p *CoolProvider) GetFailedWorkflowRun(ctx *Context, prID string) (string, error) {
	repoPath := fmt.Sprintf("%s/%s", ctx.RepoInfo.Owner, ctx.RepoInfo.Repo)
	url := fmt.Sprintf("%s/%s/-/pulls/%s", p.Endpoint, repoPath, prID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var prInfo struct {
		Head struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&prInfo); err != nil {
		return "", err
	}

	headSHA := prInfo.Head.SHA
	if headSHA == "" {
		return "", fmt.Errorf("未找到 head SHA")
	}

	runsURL := fmt.Sprintf("%s/%s/-/actions/runs?commit=%s&status=failure&per_page=1", p.Endpoint, repoPath, headSHA)
	runsReq, err := http.NewRequest("GET", runsURL, nil)
	if err != nil {
		return "", err
	}

	runsReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	runsResp, err := client.Do(runsReq)
	if err != nil {
		return "", err
	}
	defer runsResp.Body.Close()

	if runsResp.StatusCode != 200 {
		return "", fmt.Errorf("获取 workflow runs 失败")
	}

	var runs struct {
		WorkflowRuns []struct {
			ID int `json:"id"`
		} `json:"workflow_runs"`
	}

	if err := json.NewDecoder(runsResp.Body).Decode(&runs); err != nil {
		return "", err
	}

	if len(runs.WorkflowRuns) == 0 {
		return "", fmt.Errorf("未找到失败的运行记录")
	}

	return fmt.Sprintf("%d", runs.WorkflowRuns[0].ID), nil
}
