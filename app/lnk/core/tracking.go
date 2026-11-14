package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (l *Lnk) readTrackingEntries() ([]TrackedEntry, error) {
	trackingFile := l.getTrackingFilePath()
	if !l.fs.FileExists(trackingFile) {
		return []TrackedEntry{}, nil
	}
	content, err := os.ReadFile(trackingFile)
	if err != nil {
		return nil, fmt.Errorf("读取跟踪文件失败: %w", err)
	}
	lines := strings.Split(string(content), "\n")
	return l.parseTrackingLines(lines), nil
}

func (l *Lnk) writeTrackingEntries(entries []TrackedEntry) error {
	trackingFile := l.getTrackingFilePath()

	if err := l.fs.EnsureDir(filepath.Dir(trackingFile)); err != nil {
		return fmt.Errorf("创建跟踪文件目录失败: %w", err)
	}

	lines := l.serializeTrackingEntries(entries)
	content := strings.Join(lines, "\n")
	if content != "" {
		content += "\n"
	}

	if err := os.WriteFile(trackingFile, []byte(content), 0o644); err != nil {
		return fmt.Errorf("写入跟踪文件失败: %w", err)
	}

	var files []string
	for _, e := range entries {
		files = append(files, e.Path)
	}
	l.cache.Update(trackingFile, files)
	return nil
}

func (l *Lnk) addToTrackingFileWithType(filePath, typ string) error {
	entries, err := l.readTrackingEntries()
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Path == filePath {
			return nil
		}
	}
	t := LinkTypeSoft
	if typ == LinkTypeHard {
		t = LinkTypeHard
	}
	entries = append(entries, TrackedEntry{Path: filePath, Type: t})
	return l.writeTrackingEntries(entries)
}

type TrackedEntry struct {
	Path string
	Type string
}

func WithLinkType(t string) Option {
	return func(l *Lnk) {
		if t != LinkTypeHard {
			l.linkType = LinkTypeSoft
		} else {
			l.linkType = LinkTypeHard
		}
	}
}

func (l *Lnk) SetLinkType(t string) {
	if t != LinkTypeHard {
		l.linkType = LinkTypeSoft
	} else {
		l.linkType = LinkTypeHard
	}
}

func (l *Lnk) parseTrackingLines(lines []string) []TrackedEntry {
	var entries []TrackedEntry
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		p := strings.TrimSpace(parts[0])
		t := LinkTypeSoft
		if len(parts) == 2 {
			tt := strings.TrimSpace(parts[1])
			if tt == LinkTypeHard {
				t = LinkTypeHard
			} else {
				t = LinkTypeSoft
			}
		}
		entries = append(entries, TrackedEntry{Path: p, Type: t})
	}
	return entries
}

func (l *Lnk) serializeTrackingEntries(entries []TrackedEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.Path == "" {
			continue
		}
		lt := e.Type
		if lt != LinkTypeHard {
			lt = LinkTypeSoft
		}
		out = append(out, fmt.Sprintf("%s|%s", e.Path, lt))
	}
	return out
}

type TrackingCache struct {
	files    []string
	lastMod  int64
	filePath string
}

func NewTrackingCache() *TrackingCache {
	return &TrackingCache{
		files:   make([]string, 0),
		lastMod: 0,
	}
}

func (tc *TrackingCache) IsValid(filePath string) bool {
	if tc.filePath != filePath {
		return false
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return false
	}

	return info.ModTime().Unix() == tc.lastMod
}

func (tc *TrackingCache) Update(filePath string, files []string) {
	tc.filePath = filePath
	tc.files = make([]string, len(files))
	copy(tc.files, files)

	if info, err := os.Stat(filePath); err == nil {
		tc.lastMod = info.ModTime().Unix()
	}
}

func (tc *TrackingCache) Get() []string {
	result := make([]string, len(tc.files))
	copy(result, tc.files)
	return result
}

func (tc *TrackingCache) Clear() {
	tc.files = nil
	tc.lastMod = 0
	tc.filePath = ""
}
