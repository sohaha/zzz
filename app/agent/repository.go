package agent

// 架构说明:
//
// RepositoryProvider 接口设计用于支持多个代码托管平台 (GitHub, CNB Cool, GitLab 等)
//
// 当前实现状态 (已完成重构):
// - GitHubProvider: 完整实现，封装所有 gh CLI 调用
// - CoolProvider: 完整实现，通过 HTTP API 调用
// - pr.go: 完全通过 RepositoryProvider 接口调用，不再直接依赖 gh CLI
//
// 重构优势:
// 1. pr.go 与具体平台解耦，易于扩展新平台 (GitLab, Gitee 等)
// 2. 统一的错误处理和日志记录
// 3. 可通过 --repo-provider 参数切换平台
// 4. 代码更简洁，职责分离清晰
//
// 使用方式:
// - GitHub (默认): zzz agent -p "task"
// - Cool 平台: zzz agent -p "task" --provider cool --repo-api-key $COOL_API_KEY

import (
	"fmt"
	"strings"

	"github.com/sohaha/zlsgo/zutil"
)

// ErrUnknownProvider 返回未知提供商错误
func ErrUnknownProvider(name string) error {
	return fmt.Errorf("不支持的仓库提供商: %s (支持: github, cool)", name)
}

// ProviderOptions 提供商初始化选项
type ProviderOptions struct {
	Endpoint string // API 端点 (可选，有默认值)
}

// DetectProviderFromRemote 从 git remote URL 检测仓库提供商
func DetectProviderFromRemote(remoteURL string) string {
	remoteURL = strings.TrimSpace(remoteURL)

	// GitHub
	if strings.Contains(remoteURL, "github.com") {
		return "github"
	}

	// CNB Cool: https://cnb.cool/owner/repo
	if strings.Contains(remoteURL, "cnb.cool") {
		return "cool"
	}

	return ""
}

// RepositoryProvider 定义仓库操作的抽象接口
type RepositoryProvider interface {
	// Name 返回提供商名称 (github, cool, etc.)
	Name() string

	// Validate 验证配置和环境
	Validate(ctx *Context) error

	// DetectFromGit 从 git remote 自动检测仓库信息
	DetectFromGit() (*RepositoryInfo, error)

	// CreatePullRequest 创建 PR/MR
	CreatePullRequest(ctx *Context, opts *PRCreateOptions) (*PullRequestInfo, error)

	// MergePullRequest 合并 PR
	MergePullRequest(ctx *Context, prID string, strategy MergeStrategy) error

	// GetPRStatus 获取 PR 的 CI 状态
	GetPRStatus(ctx *Context, prID string) (*PRStatus, error)

	// GetPRChecks 获取 PR 检查项详细信息
	GetPRChecks(ctx *Context, prID string) ([]PRCheckResult, error)

	// GetPRReviewStatus 获取 PR 审查状态
	GetPRReviewStatus(ctx *Context, prID string) (*PRReviewStatus, error)

	// UpdatePRBranch 更新 PR 分支与目标分支同步
	UpdatePRBranch(ctx *Context, prID string) error

	// ClosePullRequest 关闭 PR
	ClosePullRequest(ctx *Context, prID string, deleteBranch bool) error

	// GetFailedWorkflowRun 获取 PR 的失败 workflow run ID
	GetFailedWorkflowRun(ctx *Context, prID string) (string, error)

	// SupportsBranches 是否支持分支工作流
	SupportsBranches() bool

	// SupportsCI 是否支持 CI 集成
	SupportsCI() bool
}

// RepositoryInfo 仓库信息
type RepositoryInfo struct {
	Provider string // github, cool, etc.
	Owner    string // 所有者/组织
	Repo     string // 仓库名（完整路径，如 "owner/repo"）
}

// PRCreateOptions PR 创建选项
type PRCreateOptions struct {
	Title        string
	Body         string
	SourceBranch string
	TargetBranch string
}

// PullRequestInfo PR 信息
type PullRequestInfo struct {
	ID     string
	Number int
	URL    string
}

// PRStatus PR 状态
type PRStatus struct {
	State     string // open, merged, closed
	CIStatus  string // pending, success, failure
	CIRunID   string
	ChecksURL string
}

// PRReviewStatus PR 审查状态
type PRReviewStatus struct {
	ReviewDecision string   // APPROVED, CHANGES_REQUESTED, REVIEW_REQUIRED, ""
	ReviewRequests []string // 待审查者列表
}

// MergeStrategy 合并策略
type MergeStrategy string

const (
	MergeStrategySquash MergeStrategy = "squash"
	MergeStrategyMerge  MergeStrategy = "merge"
	MergeStrategyRebase MergeStrategy = "rebase"
)

// NewRepositoryProvider 创建仓库提供商实例
func NewRepositoryProvider(providerName string, opts ProviderOptions) (RepositoryProvider, error) {
	switch providerName {
	case "github", "gh", "":
		return &GitHubProvider{}, nil
	case "cool":
		endpoint := opts.Endpoint
		if endpoint == "" {
			endpoint = "https://api.cnb.cool"
		}
		return &CoolProvider{
			apiKey:   zutil.Getenv("CNB_TOKEN"),
			Endpoint: endpoint,
		}, nil
	default:
		return nil, ErrUnknownProvider(providerName)
	}
}
