package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (l *Lnk) Push(message string) (bool, error) {
	if !l.IsInitialized() {
		return false, &RepoNotInitializedError{
			RepoPath: l.repoPath,
		}
	}

	if err := l.git.AddAll(); err != nil {
		return false, fmt.Errorf("暂存变更失败: %w", err)
	}

	if message == "" {
		message = "lnk: 自动同步变更"
	}
	committed := true
	if err := l.git.Commit(message); err != nil {
		em := err.Error()
		if !(strings.Contains(em, "nothing to commit") || strings.Contains(em, "无文件要提交") || strings.Contains(em, "干净的工作区")) {
			return false, fmt.Errorf("提交变更失败: %w", err)
		}
		committed = false
	}

	if !l.git.HasRemote() {
		return committed, fmt.Errorf("没有配置远程仓库，无法推送")
	}

	if err := l.git.Push(); err != nil {
		return committed, fmt.Errorf("推送到远程仓库失败: %w", err)
	}

	return committed, nil
}

func (l *Lnk) Pull() error {
	if !l.IsInitialized() {
		return &RepoNotInitializedError{
			RepoPath: l.repoPath,
		}
	}

	if !l.git.HasRemote() {
		return fmt.Errorf("没有配置远程仓库，无法拉取")
	}

	if err := l.git.Pull(); err != nil {
		return fmt.Errorf("从远程仓库拉取失败: %w", err)
	}

	if err := l.RestoreSymlinks(); err != nil {
		return fmt.Errorf("恢复符号链接失败: %w", err)
	}

	return nil
}

func (l *Lnk) RestoreSymlinks() error {
	if !l.IsInitialized() {
		return &RepoNotInitializedError{
			RepoPath: l.repoPath,
		}
	}

	entries, err := l.readTrackingEntries()
	if err != nil {
		return fmt.Errorf("获取管理文件列表失败: %w", err)
	}

	if len(entries) == 0 {
		return nil
	}

	var errors []string
	restoredCount := 0

	for _, ent := range entries {
		trackKey := ent.Path

		absPath := trackKey
		if !filepath.IsAbs(trackKey) {
			if homeDir, err := os.UserHomeDir(); err == nil {
				absPath = filepath.Join(homeDir, strings.TrimPrefix(filepath.Clean(trackKey), "./"))
			}
		}

		repoFilePath := l.getRepoFilePath(trackKey)

		if !l.fs.FileExists(repoFilePath) {
			errors = append(errors, fmt.Sprintf("仓库文件不存在: %s", repoFilePath))
			continue
		}

		if l.fs.IsSymlink(absPath) {
			target, err := l.fs.ReadSymlink(absPath)
			if err == nil && target == repoFilePath {
				restoredCount++
				continue
			}
		}

		if ent.Type == LinkTypeHard && l.fs.FileExists(absPath) {
			if l.fs.IsHardlinkTo(absPath, repoFilePath) {
				restoredCount++
				continue
			}
		}

		if l.fs.FileExists(absPath) && !l.fs.IsSymlink(absPath) {
			backupPath, err := l.createBackup(absPath)
			if err != nil {
				errors = append(errors, fmt.Sprintf("创建备份失败 %s: %v", absPath, err))
				continue
			}
			fmt.Printf("已备份现有文件: %s -> %s\n", absPath, backupPath)
		} else if l.fs.FileExists(absPath) {
			if err := l.fs.RemoveFile(absPath); err != nil {
				errors = append(errors, fmt.Sprintf("删除错误符号链接失败 %s: %v", absPath, err))
				continue
			}
		}

		targetDir := filepath.Dir(absPath)
		if err := l.fs.EnsureDir(targetDir); err != nil {
			errors = append(errors, fmt.Sprintf("创建目标目录失败 %s: %v", targetDir, err))
			continue
		}

		if ent.Type == LinkTypeHard {
			if err := l.fs.CreateHardlink(repoFilePath, absPath); err != nil {
				errors = append(errors, fmt.Sprintf("创建硬链接失败 %s: %v", absPath, err))
			} else {
				restoredCount++
			}
		} else {
			if err := l.fs.CreateSymlink(repoFilePath, absPath); err != nil {
				errors = append(errors, fmt.Sprintf("创建符号链接失败 %s: %v", absPath, err))
			} else {
				restoredCount++
			}
		}
	}
	if len(errors) > 0 {
		return fmt.Errorf("恢复符号链接时发生错误 (成功: %d, 失败: %d):\n%s",
			restoredCount, len(errors), strings.Join(errors, "\n"))
	}

	return nil
}

func (l *Lnk) RestoreSymlinksForHost(hostName string) error {
	if !l.IsInitialized() {
		return &RepoNotInitializedError{
			RepoPath: l.repoPath,
		}
	}

	entries, err := l.readTrackingEntries()
	if err != nil {
		return fmt.Errorf("获取管理文件列表失败: %w", err)
	}

	if len(entries) == 0 {
		return nil
	}

	originalHost := l.host
	l.host = hostName
	defer func() {
		l.host = originalHost
	}()

	var errors []string
	restoredCount := 0

	for _, ent := range entries {
		trackKey := ent.Path

		absPath := trackKey
		if !filepath.IsAbs(trackKey) {
			if homeDir, err := os.UserHomeDir(); err == nil {
				absPath = filepath.Join(homeDir, strings.TrimPrefix(filepath.Clean(trackKey), "./"))
			}
		}

		repoFilePath := l.getRepoFilePath(trackKey)

		// 若目标是硬链接，并且现有文件已是指向仓库文件的硬链接，则跳过
		if ent.Type == LinkTypeHard && l.fs.FileExists(absPath) {
			if l.fs.IsHardlinkTo(absPath, repoFilePath) {
				restoredCount++
				continue
			}
		}

		if l.fs.FileExists(absPath) {
			if l.fs.IsSymlink(absPath) {
				target, err := l.fs.ReadSymlink(absPath)
				if err == nil && target == repoFilePath {
					restoredCount++
					continue
				}
				if err := l.fs.RemoveFile(absPath); err != nil {
					errors = append(errors, fmt.Sprintf("删除无效符号链接失败 %s: %v", absPath, err))
					continue
				}
			} else {
				if _, err := l.createBackup(absPath); err != nil {
					errors = append(errors, fmt.Sprintf("备份已存在文件失败 %s: %v", absPath, err))
					continue
				}
			}
		}

		targetDir := filepath.Dir(absPath)
		if err := l.fs.EnsureDir(targetDir); err != nil {
			errors = append(errors, fmt.Sprintf("创建目标目录失败 %s: %v", targetDir, err))
			continue
		}

		if ent.Type == LinkTypeHard {
			if err := l.fs.CreateHardlink(repoFilePath, absPath); err != nil {
				errors = append(errors, fmt.Sprintf("创建硬链接失败 %s: %v", absPath, err))
				continue
			}
		} else {
			if err := l.fs.CreateSymlink(repoFilePath, absPath); err != nil {
				errors = append(errors, fmt.Sprintf("创建符号链接失败 %s: %v", absPath, err))
				continue
			}
		}

		restoredCount++
	}

	if len(errors) > 0 {
		fmt.Printf("警告: 恢复主机 %s 的符号链接时发生 %d 个错误 (成功: %d):\n%s\n请检查 Windows 符号链接权限（启用开发者模式或以管理员身份运行）。\n",
			hostName, len(errors), restoredCount, strings.Join(errors, "\n"))
		return nil
	}

	return nil
}

func (l *Lnk) CleanupInvalidEntries() error {
	if !l.IsInitialized() {
		return &RepoNotInitializedError{
			RepoPath: l.repoPath,
		}
	}

	entries, err := l.readTrackingEntries()
	if err != nil {
		return fmt.Errorf("读取跟踪文件失败: %w", err)
	}

	var validEntries []TrackedEntry
	var removedCount int

	for _, ent := range entries {
		filePath := ent.Path
		repoFilePath := l.getRepoFilePath(filePath)

		if l.fs.FileExists(repoFilePath) {
			validEntries = append(validEntries, ent)
		} else {
			fmt.Printf("移除无效条目: %s (仓库文件不存在: %s)\n", filePath, repoFilePath)
			removedCount++
		}
	}

	if removedCount > 0 {
		if err := l.writeTrackingEntries(validEntries); err != nil {
			return fmt.Errorf("更新跟踪文件失败: %w", err)
		}
		l.cache.Clear()

		fmt.Printf("清理完成: 移除了 %d 个无效条目，保留了 %d 个有效条目\n", removedCount, len(validEntries))
	} else {
		fmt.Println("没有发现无效条目")
	}

	return nil
}
