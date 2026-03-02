package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (l *Lnk) RemoveForce(filePath string) error {
	return l.RemoveForceMultiple([]string{filePath})
}

type forceRemoveTarget struct {
	absPath     string
	trackKey    string
	repoFile    string
	relativeKey string
}

type movedRepoFile struct {
	original string
	backup   string
	relPath  string
}

type removedSymlink struct {
	path   string
	target string
}

type forceRemoveRollback struct {
	entries      []TrackedEntry
	movedFiles   []movedRepoFile
	removedLinks []removedSymlink
	removedCache []string
	backupDir    string
}

func (l *Lnk) RemoveForceMultiple(filePaths []string) error {
	if !l.IsInitialized() {
		return &RepoNotInitializedError{RepoPath: l.repoPath}
	}
	if len(filePaths) == 0 {
		return fmt.Errorf("文件路径列表不能为空")
	}

	targets, err := l.collectForceRemoveTargets(filePaths)
	if err != nil {
		return err
	}
	rollback, err := l.newForceRemoveRollback()
	if err != nil {
		return err
	}

	if err := l.applyForceRemoveTargets(targets, rollback); err != nil {
		return l.rollbackForceRemove(err, rollback)
	}
	if err := l.stageTrackingFile(); err != nil {
		return l.rollbackForceRemove(err, rollback)
	}
	if err := l.git.Commit(buildForceRemoveCommitMessage(targets)); err != nil {
		return l.rollbackForceRemove(fmt.Errorf("提交变更失败: %w", err), rollback)
	}

	l.cleanupForceRemoveBackups(rollback.backupDir)
	return nil
}

func (l *Lnk) newForceRemoveRollback() (*forceRemoveRollback, error) {
	entries, err := l.readTrackingEntries()
	if err != nil {
		return nil, fmt.Errorf("读取跟踪文件失败: %w", err)
	}
	backupDir, err := os.MkdirTemp(l.repoPath, ".lnk_tmp_remove_force_*")
	if err != nil {
		return nil, fmt.Errorf("创建回滚目录失败: %w", err)
	}
	return &forceRemoveRollback{
		entries:   entries,
		backupDir: backupDir,
	}, nil
}

func (l *Lnk) applyForceRemoveTargets(targets []forceRemoveTarget, rollback *forceRemoveRollback) error {
	for _, target := range targets {
		if err := l.removeSymlinkForForce(target, rollback); err != nil {
			return err
		}
		if err := l.moveRepoFileForForce(target, rollback); err != nil {
			return err
		}
		if err := l.git.RemoveCached(target.relativeKey); err != nil {
			return fmt.Errorf("从 Git 索引移除失败: %w", err)
		}
		rollback.removedCache = append(rollback.removedCache, target.relativeKey)
	}
	return l.rewriteTrackingWithoutTargets(rollback.entries, targets)
}

func (l *Lnk) rewriteTrackingWithoutTargets(entries []TrackedEntry, targets []forceRemoveTarget) error {
	removeSet := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		removeSet[target.trackKey] = struct{}{}
	}

	filtered := make([]TrackedEntry, 0, len(entries))
	for _, entry := range entries {
		if _, remove := removeSet[entry.Path]; remove {
			continue
		}
		filtered = append(filtered, entry)
	}

	if err := l.writeTrackingEntries(filtered); err != nil {
		return fmt.Errorf("更新跟踪文件失败: %w", err)
	}
	return nil
}

func (l *Lnk) removeSymlinkForForce(target forceRemoveTarget, rollback *forceRemoveRollback) error {
	if !l.fs.FileExists(target.absPath) || !l.fs.IsSymlink(target.absPath) {
		return nil
	}
	if err := l.fs.RemoveFile(target.absPath); err != nil {
		return fmt.Errorf("删除符号链接失败: %w", err)
	}
	rollback.removedLinks = append(rollback.removedLinks, removedSymlink{
		path:   target.absPath,
		target: target.repoFile,
	})
	return nil
}

func (l *Lnk) moveRepoFileForForce(target forceRemoveTarget, rollback *forceRemoveRollback) error {
	if !l.fs.FileExists(target.repoFile) {
		return nil
	}

	info, err := os.Lstat(target.repoFile)
	if err != nil {
		return fmt.Errorf("读取仓库文件失败: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("仓库路径是目录，拒绝强制移除: %s", target.relativeKey)
	}

	backupPath := buildForceRemoveBackupPath(rollback.backupDir, target.relativeKey, len(rollback.movedFiles))
	if err := os.Rename(target.repoFile, backupPath); err != nil {
		return fmt.Errorf("准备删除仓库文件失败: %w", err)
	}

	rollback.movedFiles = append(rollback.movedFiles, movedRepoFile{
		original: target.repoFile,
		backup:   backupPath,
		relPath:  target.relativeKey,
	})
	return nil
}

func buildForceRemoveBackupPath(backupDir, relPath string, index int) string {
	replacer := strings.NewReplacer(string(os.PathSeparator), "_", ":", "_")
	cleanRelPath := replacer.Replace(relPath)
	if strings.TrimSpace(cleanRelPath) == "" {
		cleanRelPath = "tracked"
	}
	name := fmt.Sprintf("%03d_%s.bak", index, cleanRelPath)
	return filepath.Join(backupDir, name)
}

func (l *Lnk) rollbackForceRemove(cause error, rollback *forceRemoveRollback) error {
	defer l.cleanupForceRemoveBackups(rollback.backupDir)

	failures := make([]string, 0)
	if err := l.restoreMovedFiles(rollback); err != nil {
		failures = append(failures, err.Error())
	}
	if err := l.restoreRemovedCache(rollback); err != nil {
		failures = append(failures, err.Error())
	}
	if err := l.restoreRemovedSymlinks(rollback); err != nil {
		failures = append(failures, err.Error())
	}
	if err := l.writeTrackingEntries(rollback.entries); err != nil {
		failures = append(failures, fmt.Sprintf("恢复跟踪文件失败: %v", err))
	}
	if err := l.stageTrackingFile(); err != nil {
		failures = append(failures, fmt.Sprintf("恢复跟踪文件暂存失败: %v", err))
	}

	if len(failures) == 0 {
		return cause
	}
	return fmt.Errorf("%w; 回滚失败: %s", cause, strings.Join(failures, "; "))
}

func (l *Lnk) restoreRemovedCache(rollback *forceRemoveRollback) error {
	moved := make(map[string]struct{}, len(rollback.movedFiles))
	for _, item := range rollback.movedFiles {
		moved[item.relPath] = struct{}{}
	}

	failures := make([]string, 0)
	for i := len(rollback.removedCache) - 1; i >= 0; i-- {
		relPath := rollback.removedCache[i]
		if _, ok := moved[relPath]; ok {
			continue
		}
		if err := l.git.RestoreStaged(relPath); err != nil {
			failures = append(failures, fmt.Sprintf("恢复 Git 暂存失败(%s): %v", relPath, err))
		}
	}
	if len(failures) == 0 {
		return nil
	}
	return errors.New(strings.Join(failures, "; "))
}

func (l *Lnk) restoreMovedFiles(rollback *forceRemoveRollback) error {
	failures := make([]string, 0)
	for i := len(rollback.movedFiles) - 1; i >= 0; i-- {
		item := rollback.movedFiles[i]
		if !l.fs.FileExists(item.backup) {
			continue
		}
		if err := l.fs.EnsureDir(filepath.Dir(item.original)); err != nil {
			failures = append(failures, fmt.Sprintf("恢复目录失败(%s): %v", item.relPath, err))
			continue
		}
		if err := os.Rename(item.backup, item.original); err != nil {
			failures = append(failures, fmt.Sprintf("恢复文件失败(%s): %v", item.relPath, err))
			continue
		}
		if err := l.git.Add(item.relPath); err != nil {
			failures = append(failures, fmt.Sprintf("恢复 Git 索引失败(%s): %v", item.relPath, err))
		}
	}
	if len(failures) == 0 {
		return nil
	}
	return errors.New(strings.Join(failures, "; "))
}

func (l *Lnk) restoreRemovedSymlinks(rollback *forceRemoveRollback) error {
	failures := make([]string, 0)
	for i := len(rollback.removedLinks) - 1; i >= 0; i-- {
		item := rollback.removedLinks[i]
		if l.fs.FileExists(item.path) {
			continue
		}
		if err := l.fs.EnsureDir(filepath.Dir(item.path)); err != nil {
			failures = append(failures, fmt.Sprintf("恢复链接目录失败(%s): %v", item.path, err))
			continue
		}
		if err := os.Symlink(item.target, item.path); err != nil {
			failures = append(failures, fmt.Sprintf("恢复链接失败(%s): %v", item.path, err))
		}
	}
	if len(failures) == 0 {
		return nil
	}
	return errors.New(strings.Join(failures, "; "))
}

func (l *Lnk) cleanupForceRemoveBackups(backupDir string) {
	if strings.TrimSpace(backupDir) == "" {
		return
	}
	_ = os.RemoveAll(backupDir)
}

func (l *Lnk) collectForceRemoveTargets(filePaths []string) ([]forceRemoveTarget, error) {
	targets := make([]forceRemoveTarget, 0, len(filePaths))
	seen := make(map[string]struct{}, len(filePaths))
	for _, filePath := range filePaths {
		absPath, trackKey := resolveRemovePath(l, filePath)
		if _, ok := seen[trackKey]; ok {
			continue
		}
		seen[trackKey] = struct{}{}

		managed, err := l.isFileManaged(trackKey)
		if err != nil {
			return nil, fmt.Errorf("检查文件管理状态失败: %w", err)
		}
		if !managed {
			return nil, &FileNotManagedError{FilePath: absPath}
		}
		repoFile := l.getRepoFilePath(trackKey)
		if err := validateForceRemoveRepoPath(l.repoPath, repoFile); err != nil {
			return nil, err
		}
		targets = append(targets, forceRemoveTarget{
			absPath:     absPath,
			trackKey:    trackKey,
			repoFile:    repoFile,
			relativeKey: l.getRelativePathInRepo(trackKey),
		})
	}
	return targets, nil
}

func validateForceRemoveRepoPath(repoPath, repoFile string) error {
	repoReal, err := filepath.EvalSymlinks(filepath.Clean(repoPath))
	if err != nil {
		return fmt.Errorf("解析仓库真实路径失败: %w", err)
	}
	fileReal, err := resolvePathWithSymlinks(repoFile)
	if err != nil {
		return fmt.Errorf("解析仓库文件真实路径失败: %w", err)
	}
	return ensurePathInRepo(repoReal, fileReal)
}

func resolvePathWithSymlinks(path string) (string, error) {
	cleaned := filepath.Clean(path)
	current := cleaned
	missing := make([]string, 0)

	for {
		_, err := os.Lstat(current)
		if err == nil {
			resolved, resolveErr := filepath.EvalSymlinks(current)
			if resolveErr != nil {
				return "", resolveErr
			}
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return filepath.Clean(resolved), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}

		dir := filepath.Dir(current)
		if dir == current {
			return "", fmt.Errorf("路径不存在: %s", path)
		}
		missing = append(missing, filepath.Base(current))
		current = dir
	}
}

func ensurePathInRepo(repoPath, repoFile string) error {
	rel, err := filepath.Rel(repoPath, repoFile)
	if err != nil {
		return fmt.Errorf("计算仓库相对路径失败: %w", err)
	}
	if rel == "." {
		return fmt.Errorf("拒绝移除仓库根目录: %s", repoFile)
	}
	parent := ".."
	prefix := parent + string(os.PathSeparator)
	if rel == parent || strings.HasPrefix(rel, prefix) {
		return fmt.Errorf("检测到越界路径，拒绝移除: %s", repoFile)
	}
	return nil
}

func buildForceRemoveCommitMessage(targets []forceRemoveTarget) string {
	if len(targets) == 1 {
		return fmt.Sprintf("lnk: 强制移除文件 %s", filepath.Base(targets[0].absPath))
	}
	return fmt.Sprintf("lnk: 强制移除 %d 个文件", len(targets))
}

func resolveRemovePath(l *Lnk, filePath string) (string, string) {
	if !filepath.IsAbs(filePath) && !strings.HasPrefix(filePath, "~/") {
		trackKey := strings.TrimPrefix(filepath.Clean(filePath), "./")
		homeDir, _ := os.UserHomeDir()
		return filepath.Join(homeDir, trackKey), trackKey
	}

	normalizedPath := l.normalizeFilePath(filePath)
	return normalizedPath, l.toTrackingPath(normalizedPath)
}

func (l *Lnk) stageTrackingFile() error {
	trackingFile := l.getTrackingFilePath()
	trackingRelPath, err := filepath.Rel(l.repoPath, trackingFile)
	if err != nil {
		trackingRelPath = filepath.Base(trackingFile)
	}
	if err := l.git.Add(trackingRelPath); err != nil {
		return fmt.Errorf("添加跟踪文件到 Git 失败: %w", err)
	}
	return nil
}
