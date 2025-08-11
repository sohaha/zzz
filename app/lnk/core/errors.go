package core

import "fmt"

type LnkError struct {
	Message    string
	Path       string
	Suggestion string
}

func (e *LnkError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("%s (路径: %s)", e.Message, e.Path)
	}
	return e.Message
}

func NewLnkError(message, path, suggestion string) *LnkError {
	return &LnkError{
		Message:    message,
		Path:       path,
		Suggestion: suggestion,
	}
}

type RepoNotInitializedError struct {
	RepoPath string
}

func (e *RepoNotInitializedError) Error() string {
	return fmt.Sprintf("lnk 仓库未初始化: %s", e.RepoPath)
}

func (e *RepoNotInitializedError) Code() ErrorCode {
	return ErrCodeRepoNotInitialized
}

func (e *RepoNotInitializedError) Severity() ErrorSeverity {
	return SeverityError
}

func (e *RepoNotInitializedError) Context() map[string]interface{} {
	return map[string]interface{}{
		"repo_path": e.RepoPath,
	}
}

func (e *RepoNotInitializedError) Suggestion() string {
	return "请先运行 'zzz lnk init' 初始化仓库"
}

func (e *RepoNotInitializedError) Recoverable() bool {
	return false
}

type RepoAlreadyExistsError struct {
	RepoPath string
}

func (e *RepoAlreadyExistsError) Error() string {
	return fmt.Sprintf("lnk 仓库已存在: %s", e.RepoPath)
}

func (e *RepoAlreadyExistsError) Code() ErrorCode {
	return ErrCodeRepoAlreadyExists
}

func (e *RepoAlreadyExistsError) Severity() ErrorSeverity {
	return SeverityWarning
}

func (e *RepoAlreadyExistsError) Context() map[string]interface{} {
	return map[string]interface{}{
		"repo_path": e.RepoPath,
	}
}

func (e *RepoAlreadyExistsError) Suggestion() string {
	return "如需重新初始化，请使用 --force 参数"
}

func (e *RepoAlreadyExistsError) Recoverable() bool {
	return false
}

type FileAlreadyManagedError struct {
	FilePath string
}

func (e *FileAlreadyManagedError) Error() string {
	return fmt.Sprintf("文件已被 lnk 管理: %s", e.FilePath)
}

func (e *FileAlreadyManagedError) Code() ErrorCode {
	return ErrCodeFileAlreadyManaged
}

func (e *FileAlreadyManagedError) Severity() ErrorSeverity {
	return SeverityWarning
}

func (e *FileAlreadyManagedError) Context() map[string]interface{} {
	return map[string]interface{}{
		"file_path": e.FilePath,
	}
}

func (e *FileAlreadyManagedError) Suggestion() string {
	return "文件已在管理中，无需重复添加"
}

func (e *FileAlreadyManagedError) Recoverable() bool {
	return false
}

type FileNotManagedError struct {
	FilePath string
}

func (e *FileNotManagedError) Error() string {
	return fmt.Sprintf("文件未被 lnk 管理: %s", e.FilePath)
}

func (e *FileNotManagedError) Code() ErrorCode {
	return ErrCodeFileNotManaged
}

func (e *FileNotManagedError) Severity() ErrorSeverity {
	return SeverityError
}

func (e *FileNotManagedError) Context() map[string]interface{} {
	return map[string]interface{}{
		"file_path": e.FilePath,
	}
}

func (e *FileNotManagedError) Suggestion() string {
	return "请先使用 'zzz lnk add' 将文件添加到管理"
}

func (e *FileNotManagedError) Recoverable() bool {
	return false
}

type HostNotFoundError struct {
	HostName string
}

func (e *HostNotFoundError) Error() string {
	return fmt.Sprintf("主机配置未找到: %s", e.HostName)
}

func (e *HostNotFoundError) Code() ErrorCode {
	return ErrCodeHostNotFound
}

func (e *HostNotFoundError) Severity() ErrorSeverity {
	return SeverityError
}

func (e *HostNotFoundError) Context() map[string]interface{} {
	return map[string]interface{}{
		"host_name": e.HostName,
	}
}

func (e *HostNotFoundError) Suggestion() string {
	return "请检查主机名是否正确，或先为该主机添加配置文件"
}

func (e *HostNotFoundError) Recoverable() bool {
	return false
}

type BootstrapScriptNotFoundError struct {
	RepoPath string
}

func (e *BootstrapScriptNotFoundError) Error() string {
	return fmt.Sprintf("引导脚本未找到: %s/bootstrap.sh", e.RepoPath)
}

func (e *BootstrapScriptNotFoundError) Code() ErrorCode {
	return ErrCodeBootstrapNotFound
}

func (e *BootstrapScriptNotFoundError) Severity() ErrorSeverity {
	return SeverityWarning
}

func (e *BootstrapScriptNotFoundError) Context() map[string]interface{} {
	return map[string]interface{}{
		"repo_path":   e.RepoPath,
		"script_path": e.RepoPath + "/bootstrap.sh",
	}
}

func (e *BootstrapScriptNotFoundError) Suggestion() string {
	return "如需自动配置环境，请在仓库根目录创建 bootstrap.sh 脚本"
}

func (e *BootstrapScriptNotFoundError) Recoverable() bool {
	return false
}
