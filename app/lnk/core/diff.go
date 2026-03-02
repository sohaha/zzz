package core

import "fmt"

func (l *Lnk) Diff(color bool) (string, error) {
	if !l.IsInitialized() {
		return "", &RepoNotInitializedError{RepoPath: l.repoPath}
	}

	output, err := l.git.Diff(color)
	if err != nil {
		return "", fmt.Errorf("获取仓库差异失败: %w", err)
	}

	return output, nil
}
