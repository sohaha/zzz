package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (l *Lnk) Add(filePath string) error {
	normalizedPath := l.normalizeFilePath(filePath)

	if err := l.fs.ValidateFileForAdd(normalizedPath); err != nil {
		return fmt.Errorf("文件验证失败: %w", err)
	}

	trackKey := l.toTrackingPath(normalizedPath)

	if l.fs.IsDir(normalizedPath) {
		if managed, _ := l.isFileManaged(trackKey); managed {
			return &FileAlreadyManagedError{FilePath: normalizedPath}
		}
		return l.consolidateDirectory(normalizedPath)
	}

	managed, err := l.isFileManaged(trackKey)
	if err != nil {
		return fmt.Errorf("检查文件管理状态失败: %w", err)
	}
	if managed {
		return &FileAlreadyManagedError{FilePath: normalizedPath}
	}

	fileInfo, err := l.fs.GetFileInfo(normalizedPath)
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %w", err)
	}

	repoFilePath := l.getRepoFilePath(trackKey)

	if err := l.ensureHostDir(); err != nil {
		return fmt.Errorf("创建主机目录失败: %w", err)
	}

	l.errorHandler.ClearRollback()

	if err := l.fs.Move(normalizedPath, repoFilePath, fileInfo); err != nil {
		return WrapError(err, ErrCodeFileOperation, "移动文件到仓库失败", SeverityError).
			WithContext("source", normalizedPath).
			WithContext("destination", repoFilePath).
			WithSuggestion("请检查文件权限和磁盘空间").
			WithRecoverable(true)
	}
	l.errorHandler.AddRollbackAction(func() error {
		return l.fs.Move(repoFilePath, normalizedPath, fileInfo)
	})

	if l.linkType == LinkTypeHard {
		if err := l.fs.CreateHardlink(repoFilePath, normalizedPath); err != nil {
			if rollbackErr := l.errorHandler.HandleError(err); rollbackErr != nil {
				return rollbackErr
			}
			return WrapError(err, ErrCodeFileOperation, "创建硬链接失败", SeverityError).
				WithContext("target", repoFilePath).
				WithContext("link", normalizedPath).
				WithSuggestion("请检查目标目录权限和硬链接支持").
				WithRecoverable(true)
		}
	} else if err := l.fs.CreateSymlink(repoFilePath, normalizedPath); err != nil {
		if rollbackErr := l.errorHandler.HandleError(err); rollbackErr != nil {
			return rollbackErr
		}
		return WrapError(err, ErrCodeFileOperation, "创建符号链接失败", SeverityError).
			WithContext("target", repoFilePath).
			WithContext("link", normalizedPath).
			WithSuggestion("请检查目标目录权限和符号链接支持").
			WithRecoverable(true)
	}
	l.errorHandler.AddRollbackAction(func() error {
		return l.fs.RemoveFile(normalizedPath)
	})

	if err := l.addToTrackingFile(trackKey); err != nil {
		if rollbackErr := l.errorHandler.HandleError(err); rollbackErr != nil {
			return rollbackErr
		}
		return WrapError(err, ErrCodeFileOperation, "添加到跟踪文件失败", SeverityError).
			WithContext("file", trackKey).
			WithContext("tracking_file", l.getTrackingFilePath()).
			WithSuggestion("请检查跟踪文件权限").
			WithRecoverable(true)
	}
	l.errorHandler.AddRollbackAction(func() error {
		return l.removeFromTrackingFile(trackKey)
	})

	relPath := l.getRelativePathInRepo(trackKey)
	if err := l.git.Add(relPath); err != nil {
		if rollbackErr := l.errorHandler.HandleError(err); rollbackErr != nil {
			return rollbackErr
		}
		return WrapError(err, ErrCodeGitCommand, "添加文件到 Git 失败", SeverityError).
			WithContext("file", relPath).
			WithSuggestion("请检查 Git 仓库状态和文件权限").
			WithRecoverable(true)
	}

	trackingFile := l.getTrackingFilePath()
	trackingRelPath, err := filepath.Rel(l.repoPath, trackingFile)
	if err != nil {
		trackingRelPath = filepath.Base(trackingFile)
	}
	if err := l.git.Add(trackingRelPath); err != nil {
		if rollbackErr := l.errorHandler.HandleError(err); rollbackErr != nil {
			return rollbackErr
		}
		return WrapError(err, ErrCodeGitCommand, "添加跟踪文件到 Git 失败", SeverityError).
			WithContext("tracking_file", trackingRelPath).
			WithSuggestion("请检查 Git 仓库状态").
			WithRecoverable(true)
	}

	commitMsg := fmt.Sprintf("lnk: 添加文件 %s", filepath.Base(normalizedPath))
	if err := l.git.Commit(commitMsg); err != nil {
		if rollbackErr := l.errorHandler.HandleError(err); rollbackErr != nil {
			return rollbackErr
		}
		return WrapError(err, ErrCodeGitCommand, "提交变更失败", SeverityError).
			WithContext("commit_message", commitMsg).
			WithSuggestion("请检查 Git 配置和仓库状态").
			WithRecoverable(true)
	}

	l.errorHandler.ClearRollback()

	return nil
}

func (l *Lnk) AddMultiple(filePaths []string) error {
	if len(filePaths) == 0 {
		return fmt.Errorf("文件路径列表不能为空")
	}

	var validPaths []string
	for _, filePath := range filePaths {
		normalizedPath := l.normalizeFilePath(filePath)
		trackKey := l.toTrackingPath(normalizedPath)

		if err := l.fs.ValidateFileForAdd(normalizedPath); err != nil {
			return fmt.Errorf("文件 %s 验证失败: %w", filePath, err)
		}

		managed, err := l.isFileManaged(trackKey)
		if err != nil {
			return fmt.Errorf("检查文件 %s 管理状态失败: %w", filePath, err)
		}
		if managed {
			continue
		}

		validPaths = append(validPaths, normalizedPath)
	}

	if len(validPaths) == 0 {
		return fmt.Errorf("没有需要添加的文件（所有文件都已被管理）")
	}

	if err := l.ensureHostDir(); err != nil {
		return fmt.Errorf("创建主机目录失败: %w", err)
	}

	var processedFiles []string
	var gitFilesToAdd []string
	var rollbackActions []func() error
	rollback := func() {
		for i := len(rollbackActions) - 1; i >= 0; i-- {
			if err := rollbackActions[i](); err != nil {
				fmt.Printf("回滚操作失败: %v\n", err)
			}
		}
	}

	for _, filePath := range validPaths {

		fileInfo, err := l.fs.GetFileInfo(filePath)
		if err != nil {
			rollback()
			return fmt.Errorf("获取文件 %s 信息失败: %w", filePath, err)
		}

		trackKey := l.toTrackingPath(filePath)
		repoFilePath := l.getRepoFilePath(trackKey)

		if err := l.fs.Move(filePath, repoFilePath, fileInfo); err != nil {
			rollback()
			return fmt.Errorf("移动文件 %s 到仓库失败: %w", filePath, err)
		}
		rollbackActions = append(rollbackActions, func(path, repoPath string, info os.FileInfo) func() error {
			return func() error {
				return l.fs.Move(repoPath, path, info)
			}
		}(filePath, repoFilePath, fileInfo))

		if l.linkType == LinkTypeHard {
			if err := l.fs.CreateHardlink(repoFilePath, filePath); err != nil {
				rollback()
				return fmt.Errorf("创建硬链接 %s 失败: %w", filePath, err)
			}
		} else if err := l.fs.CreateSymlink(repoFilePath, filePath); err != nil {
			rollback()
			return fmt.Errorf("创建符号链接 %s 失败: %w", filePath, err)
		}
		rollbackActions = append(rollbackActions, func(path string) func() error {
			return func() error {
				return l.fs.RemoveFile(path)
			}
		}(filePath))

		if err := l.addToTrackingFile(trackKey); err != nil {
			rollback()
			return fmt.Errorf("添加文件 %s 到跟踪文件失败: %w", trackKey, err)
		}
		rollbackActions = append(rollbackActions, func(path string) func() error {
			return func() error {
				return l.removeFromTrackingFile(path)
			}
		}(trackKey))

		relPath := l.getRelativePathInRepo(trackKey)
		gitFilesToAdd = append(gitFilesToAdd, relPath)

		processedFiles = append(processedFiles, filePath)
	}

	if len(gitFilesToAdd) > 0 {
		if err := l.git.AddMultiple(gitFilesToAdd); err != nil {
			rollback()
			return fmt.Errorf("批量添加文件到 Git 失败: %w", err)
		}
	}

	trackingFile := l.getTrackingFilePath()
	trackingRelPath, err := filepath.Rel(l.repoPath, trackingFile)
	if err != nil {
		trackingRelPath = filepath.Base(trackingFile)
	}
	if err := l.git.Add(trackingRelPath); err != nil {
		rollback()
		return fmt.Errorf("添加跟踪文件到 Git 失败: %w", err)
	}

	commitMsg := fmt.Sprintf("lnk: 批量添加 %d 个文件", len(processedFiles))
	if err := l.git.Commit(commitMsg); err != nil {
		rollback()
		return fmt.Errorf("提交变更失败: %w", err)
	}

	return nil
}

func (l *Lnk) AddRecursive(paths []string) error {
	return l.AddRecursiveWithProgress(paths, nil)
}

type ProgressCallback func(current, total int, currentFile string)

func (l *Lnk) AddRecursiveWithProgress(paths []string, progressCallback ProgressCallback) error {
	if len(paths) == 0 {
		return fmt.Errorf("路径列表不能为空")
	}

	var allFiles []string
	for _, path := range paths {
		normalizedPath := l.normalizeFilePath(path)

		if l.fs.IsDir(normalizedPath) {

			files, err := l.collectFilesRecursively(normalizedPath)
			if err != nil {
				return fmt.Errorf("收集目录 %s 中的文件失败: %w", path, err)
			}
			allFiles = append(allFiles, files...)
		} else {
			allFiles = append(allFiles, normalizedPath)
		}
	}

	if len(allFiles) == 0 {
		return fmt.Errorf("没有找到需要添加的文件")
	}

	var validFiles []string
	for _, filePath := range allFiles {

		if err := l.fs.ValidateFileForAdd(filePath); err != nil {
			continue
		}

		managed, err := l.isFileManaged(l.toTrackingPath(filePath))
		if err != nil {
			continue
		}
		if managed {
			continue
		}

		validFiles = append(validFiles, filePath)
	}

	if len(validFiles) == 0 {
		return fmt.Errorf("没有需要添加的文件（所有文件都已被管理或无效）")
	}

	if err := l.ensureHostDir(); err != nil {
		return fmt.Errorf("创建主机目录失败: %w", err)
	}

	var gitFilesToAdd []string
	var rollbackActions []func() error
	rollback := func() {
		for i := len(rollbackActions) - 1; i >= 0; i-- {
			if err := rollbackActions[i](); err != nil {
				fmt.Printf("回滚操作失败: %v\n", err)
			}
		}
	}

	total := len(validFiles)
	for i, filePath := range validFiles {

		if progressCallback != nil {
			progressCallback(i+1, total, filePath)
		}

		fileInfo, err := l.fs.GetFileInfo(filePath)
		if err != nil {
			rollback()
			return fmt.Errorf("获取文件 %s 信息失败: %w", filePath, err)
		}

		repoFilePath := l.getRepoFilePath(l.toTrackingPath(filePath))

		if err := l.fs.Move(filePath, repoFilePath, fileInfo); err != nil {
			rollback()
			return fmt.Errorf("移动文件 %s 到仓库失败: %w", filePath, err)
		}
		rollbackActions = append(rollbackActions, func(path, repoPath string, info os.FileInfo) func() error {
			return func() error {
				return l.fs.Move(repoPath, path, info)
			}
		}(filePath, repoFilePath, fileInfo))

		if l.linkType == LinkTypeHard {
			if err := l.fs.CreateHardlink(repoFilePath, filePath); err != nil {
				rollback()
				return fmt.Errorf("创建硬链接 %s 失败: %w", filePath, err)
			}
		} else if err := l.fs.CreateSymlink(repoFilePath, filePath); err != nil {
			rollback()
			return fmt.Errorf("创建符号链接 %s 失败: %w", filePath, err)
		}
		rollbackActions = append(rollbackActions, func(path string) func() error {
			return func() error {
				return l.fs.RemoveFile(path)
			}
		}(filePath))

		if err := l.addToTrackingFile(l.toTrackingPath(filePath)); err != nil {
			rollback()
			return fmt.Errorf("添加文件 %s 到跟踪文件失败: %w", l.toTrackingPath(filePath), err)
		}
		rollbackActions = append(rollbackActions, func(path string) func() error {
			return func() error {
				return l.removeFromTrackingFile(path)
			}
		}(l.toTrackingPath(filePath)))

		relPath := l.getRelativePathInRepo(l.toTrackingPath(filePath))
		gitFilesToAdd = append(gitFilesToAdd, relPath)
	}

	if len(gitFilesToAdd) > 0 {
		if err := l.git.AddMultiple(gitFilesToAdd); err != nil {
			rollback()
			return fmt.Errorf("批量添加文件到 Git 失败: %w", err)
		}
	}

	trackingFile := l.getTrackingFilePath()
	trackingRelPath, err := filepath.Rel(l.repoPath, trackingFile)
	if err != nil {
		trackingRelPath = filepath.Base(trackingFile)
	}
	if err := l.git.Add(trackingRelPath); err != nil {
		rollback()
		return fmt.Errorf("添加跟踪文件到 Git 失败: %w", err)
	}

	return nil
}

func (l *Lnk) collectFilesRecursively(dirPath string) ([]string, error) {
	var files []string

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			return nil
		}

		if strings.HasPrefix(filepath.Base(path), ".") {
			return nil
		}

		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return files, nil
}
