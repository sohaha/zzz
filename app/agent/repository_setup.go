package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/sohaha/zlsgo/zshell"
)

func SetupRepoProvider(ctx *Context, providerName, repoID string, opts ProviderOptions) error {
	if ctx == nil {
		return fmt.Errorf("上下文为空")
	}

	name := strings.TrimSpace(providerName)
	if name == "" {
		remoteURL, err := getGitRemoteURL()
		if err != nil {
			return err
		}
		name = DetectProviderFromRemote(remoteURL)
		if name == "" {
			return fmt.Errorf("无法从 git remote 识别仓库提供商，请使用 --provider 指定")
		}
	}

	provider, err := NewRepositoryProvider(name, opts)
	if err != nil {
		return err
	}

	repoInfo, err := resolveRepoInfo(provider, repoID)
	if err != nil {
		return err
	}

	ctx.RepoProvider = provider
	ctx.RepoInfo = repoInfo
	return nil
}

func getGitRemoteURL() (string, error) {
	code, stdout, _, err := zshell.ExecCommand(context.Background(),
		[]string{"git", "remote", "get-url", "origin"}, nil, nil, nil)
	if err != nil || code != 0 {
		return "", fmt.Errorf("获取 git remote 失败")
	}
	return strings.TrimSpace(stdout), nil
}

func resolveRepoInfo(provider RepositoryProvider, repoID string) (*RepositoryInfo, error) {
	repoID = strings.TrimSpace(repoID)
	if repoID == "" {
		return provider.DetectFromGit()
	}

	owner, repo, err := parseRepoID(repoID)
	if err != nil {
		return nil, err
	}

	return &RepositoryInfo{
		Provider: provider.Name(),
		Owner:    owner,
		Repo:     repo,
	}, nil
}

func parseRepoID(repoID string) (string, string, error) {
	parts := strings.Split(repoID, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("无效的 repo-id，必须为 owner/repo")
	}
	owner := strings.TrimSpace(parts[0])
	repo := strings.TrimSpace(parts[1])
	if owner == "" || repo == "" {
		return "", "", fmt.Errorf("无效的 repo-id，必须为 owner/repo")
	}
	return owner, repo, nil
}
