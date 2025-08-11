package git

import "fmt"

type GitCommandError struct {
	Command string
	Output  string
	Err     error
}

func (e *GitCommandError) Error() string {
	if e.Output != "" {
		return fmt.Sprintf("Git 命令执行失败 (%s): %s\n输出: %s", e.Command, e.Err, e.Output)
	}
	return fmt.Sprintf("Git 命令执行失败 (%s): %s", e.Command, e.Err)
}

func (e *GitCommandError) Unwrap() error {
	return e.Err
}

type RepoNotFoundError struct {
	Path string
}

func (e *RepoNotFoundError) Error() string {
	return fmt.Sprintf("Git 仓库未找到: %s", e.Path)
}

type RemoteNotFoundError struct {
	RemoteName string
}

func (e *RemoteNotFoundError) Error() string {
	return fmt.Sprintf("远程仓库未找到: %s", e.RemoteName)
}

type InvalidRemoteURLError struct {
	URL string
}

func (e *InvalidRemoteURLError) Error() string {
	return fmt.Sprintf("无效的远程仓库 URL: %s", e.URL)
}

type MergeConflictError struct {
	Files []string
}

func (e *MergeConflictError) Error() string {
	if len(e.Files) == 0 {
		return "检测到合并冲突"
	}
	return fmt.Sprintf("检测到合并冲突，涉及文件: %v", e.Files)
}

type BranchNotFoundError struct {
	BranchName string
}

func (e *BranchNotFoundError) Error() string {
	return fmt.Sprintf("分支未找到: %s", e.BranchName)
}

type DirtyWorkingTreeError struct {
	Files []string
}

func (e *DirtyWorkingTreeError) Error() string {
	if len(e.Files) == 0 {
		return "工作树有未提交的变更"
	}
	return fmt.Sprintf("工作树有未提交的变更，涉及文件: %v", e.Files)
}

type NetworkError struct {
	Operation string
	URL       string
	Err       error
}

func (e *NetworkError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("网络操作失败 (%s): %s (%v)", e.Operation, e.URL, e.Err)
	}
	return fmt.Sprintf("网络操作失败 (%s): %s", e.Operation, e.URL)
}

func (e *NetworkError) Unwrap() error {
	return e.Err
}

type AuthenticationError struct {
	URL string
	Err error
}

func (e *AuthenticationError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("Git 认证失败: %s (%v)", e.URL, e.Err)
	}
	return fmt.Sprintf("Git 认证失败: %s", e.URL)
}

func (e *AuthenticationError) Unwrap() error {
	return e.Err
}
