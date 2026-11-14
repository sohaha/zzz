package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (l *Lnk) Remove(filePath string) error {
	// Accept either a real filesystem path or a HOME-relative tracking key (e.g., ".codeium/.../xx.md")
	var normalizedPath string
	var trackKey string
	if !filepath.IsAbs(filePath) && !strings.HasPrefix(filePath, "~/") {
		// Treat as tracking key relative to HOME
		tk := strings.TrimPrefix(filepath.Clean(filePath), "./")
		homeDir, _ := os.UserHomeDir()
		normalizedPath = filepath.Join(homeDir, tk)
		trackKey = tk
	} else {
		normalizedPath = l.normalizeFilePath(filePath)
		trackKey = l.toTrackingPath(normalizedPath)
	}

	managed, err := l.isFileManaged(trackKey)
	if err != nil {
		return fmt.Errorf("检查文件管理状态失败: %w", err)
	}
	if !managed {
		return &FileNotManagedError{FilePath: normalizedPath}
	}

	if !l.fs.IsSymlink(normalizedPath) {
		return fmt.Errorf("不是符号链接: %s", normalizedPath)
	}

	repoFilePath := l.getRepoFilePath(trackKey)

	if !l.fs.FileExists(repoFilePath) {
		return fmt.Errorf("仓库中的文件不存在: %s", repoFilePath)
	}

	fileInfo, err := l.fs.GetFileInfo(repoFilePath)
	if err != nil {
		return fmt.Errorf("获取仓库文件信息失败: %w", err)
	}

	var rollbackActions []func() error
	rollback := func() {
		for i := len(rollbackActions) - 1; i >= 0; i-- {
			if err := rollbackActions[i](); err != nil {
				fmt.Printf("回滚操作失败: %v\n", err)
			}
		}
	}

	if err := l.fs.RemoveFile(normalizedPath); err != nil {
		return fmt.Errorf("删除符号链接失败: %w", err)
	}
	rollbackActions = append(rollbackActions, func() error {
		return l.fs.CreateSymlink(repoFilePath, normalizedPath)
	})

	if err := l.fs.Move(repoFilePath, normalizedPath, fileInfo); err != nil {
		rollback()
		return fmt.Errorf("恢复原始文件失败: %w", err)
	}
	rollbackActions = append(rollbackActions, func() error {
		return l.fs.Move(normalizedPath, repoFilePath, fileInfo)
	})

	if err := l.removeFromTrackingFile(trackKey); err != nil {
		rollback()
		return fmt.Errorf("从跟踪文件中移除失败: %w", err)
	}
	rollbackActions = append(rollbackActions, func() error {
		return l.addToTrackingFile(trackKey)
	})

	relPath := l.getRelativePathInRepo(trackKey)
	if err := l.git.Remove(relPath); err != nil {
		rollback()
		return fmt.Errorf("从 Git 中移除文件失败: %w", err)
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

	commitMsg := fmt.Sprintf("lnk: 移除文件 %s", filepath.Base(normalizedPath))
	if err := l.git.Commit(commitMsg); err != nil {
		rollback()
		return fmt.Errorf("提交变更失败: %w", err)
	}

	return nil
}

func (l *Lnk) RemoveMultiple(filePaths []string) error {
	if len(filePaths) == 0 {
		return fmt.Errorf("文件路径列表不能为空")
	}

	type rmItem struct {
		abs string
		tk  string
	}
	var items []rmItem
	for _, inPath := range filePaths {
		var abs string
		var tk string
		if !filepath.IsAbs(inPath) && !strings.HasPrefix(inPath, "~/") {
			// treat as tracking key
			tk = strings.TrimPrefix(filepath.Clean(inPath), "./")
			homeDir, _ := os.UserHomeDir()
			abs = filepath.Join(homeDir, tk)
		} else {
			abs = l.normalizeFilePath(inPath)
			tk = l.toTrackingPath(abs)
		}

		managed, err := l.isFileManaged(tk)
		if err != nil {
			return fmt.Errorf("检查文件 %s 管理状态失败: %w", inPath, err)
		}
		if !managed {
			return &FileNotManagedError{FilePath: abs}
		}

		// Tolerant check
		if !l.fs.IsSymlink(abs) {
			return fmt.Errorf("文件 %s 不是符号链接", inPath)
		}

		items = append(items, rmItem{abs: abs, tk: tk})
	}

	var processedFiles []string
	var rollbackActions []func() error
	rollback := func() {
		for i := len(rollbackActions) - 1; i >= 0; i-- {
			if err := rollbackActions[i](); err != nil {
				fmt.Printf("回滚操作失败: %v\n", err)
			}
		}
	}

	for _, it := range items {
		filePath := it.abs
		trackKey := it.tk

		repoFilePath := l.getRepoFilePath(trackKey)

		if !l.fs.FileExists(repoFilePath) {
			rollback()
			return fmt.Errorf("仓库中的文件不存在: %s", repoFilePath)
		}

		fileInfo, err := l.fs.GetFileInfo(repoFilePath)
		if err != nil {
			rollback()
			return fmt.Errorf("获取仓库文件 %s 信息失败: %w", filePath, err)
		}

		if err := l.fs.RemoveFile(filePath); err != nil {
			rollback()
			return fmt.Errorf("删除符号链接 %s 失败: %w", filePath, err)
		}
		rollbackActions = append(rollbackActions, func(path, repoPath string) func() error {
			return func() error {
				return l.fs.CreateSymlink(repoPath, path)
			}
		}(filePath, repoFilePath))

		if err := l.fs.Move(repoFilePath, filePath, fileInfo); err != nil {
			rollback()
			return fmt.Errorf("恢复原始文件 %s 失败: %w", filePath, err)
		}
		rollbackActions = append(rollbackActions, func(path, repoPath string, info os.FileInfo) func() error {
			return func() error {
				return l.fs.Move(path, repoPath, info)
			}
		}(filePath, repoFilePath, fileInfo))

		if err := l.removeFromTrackingFile(trackKey); err != nil {
			rollback()
			return fmt.Errorf("从跟踪文件中移除 %s 失败: %w", trackKey, err)
		}
		rollbackActions = append(rollbackActions, func(path string) func() error {
			return func() error {
				return l.addToTrackingFile(path)
			}
		}(trackKey))

		relPath := l.getRelativePathInRepo(trackKey)
		if err := l.git.Remove(relPath); err != nil {
			rollback()
			return fmt.Errorf("从 Git 中移除文件 %s 失败: %w", filePath, err)
		}

		processedFiles = append(processedFiles, filePath)
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

	commitMsg := fmt.Sprintf("lnk: 批量移除 %d 个文件", len(processedFiles))
	if err := l.git.Commit(commitMsg); err != nil {
		rollback()
		return fmt.Errorf("提交变更失败: %w", err)
	}

	return nil
}
