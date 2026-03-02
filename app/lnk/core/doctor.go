package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type DoctorResult struct {
	InvalidEntries []string
	BrokenSymlinks []string
}

func (r *DoctorResult) HasIssues() bool {
	return len(r.InvalidEntries) > 0 || len(r.BrokenSymlinks) > 0
}

func (r *DoctorResult) TotalIssues() int {
	return len(r.InvalidEntries) + len(r.BrokenSymlinks)
}

func (l *Lnk) PreviewDoctor() (*DoctorResult, error) {
	if !l.IsInitialized() {
		return nil, &RepoNotInitializedError{RepoPath: l.repoPath}
	}

	entries, err := l.readTrackingEntries()
	if err != nil {
		return nil, fmt.Errorf("读取跟踪文件失败: %w", err)
	}

	return &DoctorResult{
		InvalidEntries: l.findInvalidEntries(entries),
		BrokenSymlinks: l.findBrokenSymlinks(entries),
	}, nil
}

func (l *Lnk) Doctor() (*DoctorResult, error) {
	result, err := l.PreviewDoctor()
	if err != nil {
		return nil, err
	}
	if !result.HasIssues() {
		return result, nil
	}

	if err := l.removeInvalidTrackingEntries(result.InvalidEntries); err != nil {
		return nil, err
	}

	if len(result.BrokenSymlinks) > 0 {
		if err := l.RestoreSymlinks(); err != nil {
			return nil, fmt.Errorf("恢复符号链接失败: %w", err)
		}
	}

	return result, nil
}

func (l *Lnk) findInvalidEntries(entries []TrackedEntry) []string {
	invalid := make([]string, 0)
	for _, ent := range entries {
		if strings.TrimSpace(ent.Path) == "" {
			invalid = append(invalid, ent.Path)
			continue
		}

		cleaned := filepath.Clean(ent.Path)
		if isPathTraversal(cleaned) {
			invalid = append(invalid, ent.Path)
			continue
		}
		if !l.fs.FileExists(l.getRepoFilePath(ent.Path)) {
			invalid = append(invalid, ent.Path)
		}
	}
	return invalid
}

func isPathTraversal(cleaned string) bool {
	parent := ".."
	prefix := parent + string(os.PathSeparator)
	return cleaned == parent || strings.HasPrefix(cleaned, prefix)
}

func (l *Lnk) findBrokenSymlinks(entries []TrackedEntry) []string {
	broken := make([]string, 0)
	for _, ent := range entries {
		if strings.TrimSpace(ent.Path) == "" {
			continue
		}

		repoFilePath := l.getRepoFilePath(ent.Path)
		if !l.fs.FileExists(repoFilePath) {
			continue
		}
		absPath := trackedToAbsPath(ent.Path)
		if ent.Type == LinkTypeHard {
			if !l.fs.FileExists(absPath) || !l.fs.IsHardlinkTo(absPath, repoFilePath) {
				broken = append(broken, ent.Path)
			}
			continue
		}
		if !isValidSoftlink(l, absPath, repoFilePath) {
			broken = append(broken, ent.Path)
		}
	}
	return broken
}

func trackedToAbsPath(trackKey string) string {
	if strings.TrimSpace(trackKey) == "" {
		return ""
	}

	if filepath.IsAbs(trackKey) {
		return trackKey
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return trackKey
	}
	cleaned := strings.TrimPrefix(filepath.Clean(trackKey), "./")
	return filepath.Join(homeDir, cleaned)
}

func isValidSoftlink(l *Lnk, absPath, repoFilePath string) bool {
	if !l.fs.FileExists(absPath) || !l.fs.IsSymlink(absPath) {
		return false
	}

	target, err := l.fs.ReadSymlink(absPath)
	if err != nil {
		return false
	}

	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(absPath), target)
	}

	target = filepath.Clean(target)
	repoFilePath = filepath.Clean(repoFilePath)
	return target == repoFilePath
}

func (l *Lnk) removeInvalidTrackingEntries(invalidEntries []string) error {
	if len(invalidEntries) == 0 {
		return nil
	}

	entries, err := l.readTrackingEntries()
	if err != nil {
		return fmt.Errorf("读取跟踪文件失败: %w", err)
	}

	invalidSet := make(map[string]struct{}, len(invalidEntries))
	for _, item := range invalidEntries {
		invalidSet[item] = struct{}{}
	}

	valid := make([]TrackedEntry, 0, len(entries))
	for _, ent := range entries {
		if _, ok := invalidSet[ent.Path]; ok {
			continue
		}
		valid = append(valid, ent)
	}

	if err := l.writeTrackingEntries(valid); err != nil {
		return fmt.Errorf("更新跟踪文件失败: %w", err)
	}
	l.cache.Clear()
	return nil
}
