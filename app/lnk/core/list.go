package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sohaha/zzz/app/lnk/git"
)

func (l *Lnk) List() ([]string, error) {
	if !l.IsInitialized() {
		return nil, &RepoNotInitializedError{
			RepoPath: l.repoPath,
		}
	}

	files, err := l.readTrackingFile()
	if err != nil {
		return nil, fmt.Errorf("读取跟踪文件失败: %w", err)
	}

	return files, nil
}

func (l *Lnk) ListAll() (map[string][]string, error) {
	if !l.IsInitialized() {
		return nil, &RepoNotInitializedError{
			RepoPath: l.repoPath,
		}
	}

	result := make(map[string][]string)

	entries, err := os.ReadDir(l.repoPath)
	if err != nil {
		return nil, fmt.Errorf("读取仓库目录失败: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if name == ".lnk" {

			files, err := l.readTrackingFileByPath(filepath.Join(l.repoPath, name))
			if err != nil {
				continue
			}
			result["general"] = files
		} else if strings.HasPrefix(name, ".lnk.") {

			hostName := strings.TrimPrefix(name, ".lnk.")
			files, err := l.readTrackingFileByPath(filepath.Join(l.repoPath, name))
			if err != nil {
				continue
			}
			result[hostName] = files
		}
	}

	return result, nil
}

func (l *Lnk) ListByHost(hostName string) ([]string, error) {
	if !l.IsInitialized() {
		return nil, &RepoNotInitializedError{
			RepoPath: l.repoPath,
		}
	}

	originalHost := l.host
	l.host = hostName
	defer func() {
		l.host = originalHost
	}()

	files, err := l.readTrackingFile()
	if err != nil {
		return nil, fmt.Errorf("读取主机 %s 的跟踪文件失败: %w", hostName, err)
	}

	return files, nil
}

func (l *Lnk) readTrackingFileByPath(filePath string) ([]string, error) {
	if !l.fs.FileExists(filePath) {
		return []string{}, nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开跟踪文件失败: %w", err)
	}
	defer file.Close()

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取跟踪文件失败: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	entries := l.parseTrackingLines(lines)
	var files []string
	for _, e := range entries {
		files = append(files, e.Path)
	}

	return files, nil
}

func (l *Lnk) Status() (*StatusInfo, error) {
	if !l.IsInitialized() {
		return nil, &RepoNotInitializedError{
			RepoPath: l.repoPath,
		}
	}

	gitStatus, err := l.git.GetStatus()
	if err != nil {
		return nil, fmt.Errorf("获取 Git 状态失败: %w", err)
	}

	entries, err := l.readTrackingEntries()
	if err != nil {
		return nil, fmt.Errorf("获取管理文件列表失败: %w", err)
	}

	brokenLinks := l.checkSymlinkStatus(entries)

	return &StatusInfo{
		RepoPath:     l.repoPath,
		Host:         l.host,
		GitStatus:    gitStatus,
		ManagedFiles: len(entries),
		BrokenLinks:  brokenLinks,
	}, nil
}

type StatusInfo struct {
	RepoPath     string
	Host         string
	GitStatus    *git.StatusInfo
	ManagedFiles int
	BrokenLinks  []string
}

func (l *Lnk) checkSymlinkStatus(entries []TrackedEntry) []string {
	var brokenLinks []string

	for _, ent := range entries {
		trackKey := ent.Path

		absPath := trackKey
		if !filepath.IsAbs(trackKey) {
			if homeDir, err := os.UserHomeDir(); err == nil {
				absPath = filepath.Join(homeDir, strings.TrimPrefix(filepath.Clean(trackKey), "./"))
			}
		}

		if !l.fs.FileExists(absPath) {
			brokenLinks = append(brokenLinks, absPath)
			continue
		}
		if ent.Type == LinkTypeHard {
			// 对硬链接：不做符号链接校验，只要存在即认为正常
			continue
		}
		// 软链接校验
		if !l.fs.IsSymlink(absPath) {
			brokenLinks = append(brokenLinks, absPath)
			continue
		}
		target, err := l.fs.ReadResolvedSymlink(absPath)
		if err != nil {
			brokenLinks = append(brokenLinks, absPath)
			continue
		}
		if !l.fs.FileExists(target) {
			brokenLinks = append(brokenLinks, absPath)
			continue
		}
	}

	return brokenLinks
}

func (l *Lnk) GetManagedFileCount() (int, error) {
	files, err := l.List()
	if err != nil {
		return 0, err
	}
	return len(files), nil
}

func (l *Lnk) GetAllHostsFileCount() (map[string]int, error) {
	allFiles, err := l.ListAll()
	if err != nil {
		return nil, err
	}

	result := make(map[string]int)
	for host, files := range allFiles {
		result[host] = len(files)
	}

	return result, nil
}

func (l *Lnk) IsFileInRepo(filePath string) bool {
	repoFilePath := l.getRepoFilePath(filePath)
	return l.fs.FileExists(repoFilePath)
}

func (l *Lnk) GetRepoFileInfo(filePath string) (os.FileInfo, error) {
	repoFilePath := l.getRepoFilePath(filePath)
	return l.fs.GetFileInfo(repoFilePath)
}
