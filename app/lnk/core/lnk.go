package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sohaha/zlsgo/zfile"
	"github.com/sohaha/zlsgo/zcli"
	"github.com/sohaha/zzz/util"
	"github.com/sohaha/zzz/app/lnk/fs"
	"github.com/sohaha/zzz/app/lnk/git"
)

const (
	TrackFilename = ".lnk"

	BootstrapScript = "bootstrap.sh"
)

const (
	LinkTypeSoft = "soft"
	LinkTypeHard = "hard"
)

type Lnk struct {
	repoPath     string
	host         string
	git          *git.Git
	fs           *fs.FileSystem
	errorHandler *ErrorHandler
	resources    *ResourceManager
	cache        *TrackingCache
	linkType     string
}

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

func (l *Lnk) consolidateDirectory(dirPath string) error {
	trackDirKey := l.toTrackingPath(dirPath)
	repoDir := l.getRepoFilePath(trackDirKey)
	if err := l.ensureHostDir(); err != nil {
		return fmt.Errorf("创建主机目录失败: %w", err)
	}
	if err := l.fs.EnsureDir(repoDir); err != nil {
		return fmt.Errorf("创建仓库目录失败: %w", err)
	}

	var allFiles []string
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
		allFiles = append(allFiles, path)
		return nil
	})
	if err != nil {
		return fmt.Errorf("遍历目录失败: %w", err)
	}

	var rollbackActions []func() error
	rollback := func() {
		for i := len(rollbackActions) - 1; i >= 0; i-- {
			_ = rollbackActions[i]()
		}
	}

	for _, child := range allFiles {
		rel, _ := filepath.Rel(dirPath, child)
		dst := filepath.Join(repoDir, rel)
		if err := l.fs.EnsureDir(filepath.Dir(dst)); err != nil {
			rollback()
			return fmt.Errorf("创建目标子目录失败: %w", err)
		}

		isManaged, err := l.isFileManaged(l.toTrackingPath(child))
		if err != nil {
			rollback()
			return fmt.Errorf("检查文件管理状态失败: %w", err)
		}

		if isManaged {

			oldRepo := l.getRepoFilePath(l.toTrackingPath(child))
			info, err := l.fs.GetFileInfo(oldRepo)
			if err != nil {
				rollback()
				return fmt.Errorf("获取仓库文件信息失败: %w", err)
			}

			if filepath.Clean(oldRepo) != filepath.Clean(dst) {
				if err := l.fs.Move(oldRepo, dst, info); err != nil {
					rollback()
					return WrapError(err, ErrCodeFileOperation, "移动仓库中文件失败", SeverityError).
						WithContext("source", oldRepo).WithContext("destination", dst)
				}

				rollbackActions = append(rollbackActions, func(src, d string, fi os.FileInfo) func() error {
					return func() error { return l.fs.Move(d, src, fi) }
				}(oldRepo, dst, info))
			}

			if err := l.removeFromTrackingFile(l.toTrackingPath(child)); err != nil {
				rollback()
				return fmt.Errorf("移除旧跟踪条目失败: %w", err)
			}

			rollbackActions = append(rollbackActions, func(p string) func() error {
				return func() error { return l.addToTrackingFile(p) }
			}(l.toTrackingPath(child)))
		} else {

			fi, err := l.fs.GetFileInfo(child)
			if err != nil {
				rollback()
				return fmt.Errorf("获取文件信息失败: %w", err)
			}
			if err := l.fs.Move(child, dst, fi); err != nil {
				rollback()
				return WrapError(err, ErrCodeFileOperation, "移动文件到仓库失败", SeverityError).
					WithContext("source", child).WithContext("destination", dst)
			}

			rollbackActions = append(rollbackActions, func(src, d string, fi os.FileInfo) func() error {
				return func() error { return l.fs.Move(d, src, fi) }
			}(child, dst, fi))
		}
	}

	if err := os.RemoveAll(dirPath); err != nil {
		rollback()
		return fmt.Errorf("删除原目录失败: %w", err)
	}

	// 目录不支持硬链接，统一创建符号链接
	if err := l.fs.CreateSymlink(repoDir, dirPath); err != nil {
		rollback()
		return WrapError(err, ErrCodeFileOperation, "创建符号链接失败", SeverityError).
			WithContext("target", repoDir).WithContext("link", dirPath)
	}
	rollbackActions = append(rollbackActions, func(p string) func() error {
		return func() error { return l.fs.RemoveFile(p) }
	}(dirPath))

	if err := l.addToTrackingFileWithType(trackDirKey, LinkTypeSoft); err != nil {
		rollback()
		return fmt.Errorf("添加目录到跟踪文件失败: %w", err)
	}
	rollbackActions = append(rollbackActions, func(p string) func() error {
		return func() error { return l.removeFromTrackingFile(p) }
	}(trackDirKey))

	relRepoDir := l.getRelativePathInRepo(trackDirKey)
	if err := l.git.Add(relRepoDir); err != nil {
		rollback()
		return WrapError(err, ErrCodeGitCommand, "添加目录到 Git 失败", SeverityError).
			WithContext("dir", relRepoDir)
	}
	trackingFile := l.getTrackingFilePath()
	trackingRelPath, err := filepath.Rel(l.repoPath, trackingFile)
	if err != nil {
		trackingRelPath = filepath.Base(trackingFile)
	}
	if err := l.git.Add(trackingRelPath); err != nil {
		rollback()
		return WrapError(err, ErrCodeGitCommand, "添加跟踪文件到 Git 失败", SeverityError).
			WithContext("tracking_file", trackingRelPath)
	}

	commitMsg := fmt.Sprintf("lnk: 目录整合 %s (%d 项)", filepath.Base(dirPath), len(allFiles))
	if err := l.git.Commit(commitMsg); err != nil {
		rollback()
		return WrapError(err, ErrCodeGitCommand, "提交变更失败", SeverityError).
			WithContext("commit_message", commitMsg)
	}

	rollbackActions = nil
	return nil
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

type ResourceManager struct {
	openFiles []*os.File
	tempDirs  []string
}

func NewResourceManager() *ResourceManager {
	return &ResourceManager{
		openFiles: make([]*os.File, 0),
		tempDirs:  make([]string, 0),
	}
}

func (rm *ResourceManager) AddFile(file *os.File) {
	rm.openFiles = append(rm.openFiles, file)
}

func (rm *ResourceManager) AddTempDir(dir string) {
	rm.tempDirs = append(rm.tempDirs, dir)
}

func (rm *ResourceManager) Cleanup() {
	for _, file := range rm.openFiles {
		if file != nil {
			file.Close()
		}
	}
	rm.openFiles = rm.openFiles[:0]

	for _, dir := range rm.tempDirs {
		os.RemoveAll(dir)
	}
	rm.tempDirs = rm.tempDirs[:0]
}

type Option func(*Lnk)

func NewLnk(opts ...Option) *Lnk {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	defaultRepoPath := filepath.Join(homeDir, ".config", "lnk")

	lnk := &Lnk{
		repoPath:     defaultRepoPath,
		host:         "",
		fs:           fs.New(),
		errorHandler: NewErrorHandler(),
		resources:    NewResourceManager(),
		cache:        NewTrackingCache(),
		linkType:     LinkTypeSoft,
	}

	for _, opt := range opts {
		opt(lnk)
	}

	lnk.git = git.New(lnk.repoPath)

	if (runtime.GOOS == "windows"&&!zcli.IsSudo()) {
		util.Log.Warn("Windows 需要以管理员权限运行 lnk")
		os.Exit(0)
	}
	return lnk
}

func WithRepoPath(path string) Option {
	return func(l *Lnk) {
		realPath := zfile.RealPath(path)
		if realPath != "" {
			l.repoPath = realPath
		} else {
			l.repoPath = path
		}
	}
}

func WithHost(host string) Option {
	return func(l *Lnk) {
		l.host = host
	}
}

func (l *Lnk) GetRepoPath() string {
	return l.repoPath
}

func (l *Lnk) GetHost() string {
	return l.host
}

func (l *Lnk) IsInitialized() bool {
	return l.git.IsRepo()
}

func (l *Lnk) getTrackingFilePath() string {
	if l.host == "" || l.host == "localhost" {
		return filepath.Join(l.repoPath, TrackFilename)
	}
	return filepath.Join(l.repoPath, fmt.Sprintf("%s.%s", TrackFilename, l.host))
}

func (l *Lnk) getHostDir() string {
	if l.host == "" || l.host == "localhost" {
		return l.repoPath
	}
	return filepath.Join(l.repoPath, fmt.Sprintf("%s.lnk", l.host))
}

func (l *Lnk) readTrackingFile() ([]string, error) {
	trackingFile := l.getTrackingFilePath()

	if !l.fs.FileExists(trackingFile) {
		return []string{}, nil
	}

	if l.cache.IsValid(trackingFile) {
		return l.cache.Get(), nil
	}

	file, err := os.Open(trackingFile)
	if err != nil {
		return nil, fmt.Errorf("打开跟踪文件失败: %w", err)
	}
	defer file.Close()

	content, err := os.ReadFile(trackingFile)
	if err != nil {
		return nil, fmt.Errorf("读取跟踪文件失败: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	entries := l.parseTrackingLines(lines)
	var files []string
	for _, e := range entries {
		files = append(files, e.Path)
	}

	l.cache.Update(trackingFile, files)

	return files, nil
}

func (l *Lnk) writeTrackingFile(files []string) error {
	entries := make([]TrackedEntry, 0, len(files))
	for _, p := range files {
		if p == "" {
			continue
		}
		entries = append(entries, TrackedEntry{Path: p, Type: LinkTypeSoft})
	}
	return l.writeTrackingEntries(entries)
}

func (l *Lnk) addToTrackingFile(filePath string) error {
	entries, err := l.readTrackingEntries()
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Path == filePath {
			return nil
		}
	}
	t := l.linkType
	if t == "" {
		t = LinkTypeSoft
	}
	entries = append(entries, TrackedEntry{Path: filePath, Type: t})
	return l.writeTrackingEntries(entries)
}

func (l *Lnk) removeFromTrackingFile(filePath string) error {
	entries, err := l.readTrackingEntries()
	if err != nil {
		return err
	}
	var out []TrackedEntry
	for _, e := range entries {
		if e.Path != filePath {
			out = append(out, e)
		}
	}
	return l.writeTrackingEntries(out)
}

func (l *Lnk) isFileManaged(filePath string) (bool, error) {
	files, err := l.readTrackingFile()
	if err != nil {
		return false, err
	}

	for _, f := range files {
		if f == filePath {
			return true, nil
		}
	}

	return false, nil
}

func (l *Lnk) getRepoFilePath(originalPath string) string {
	realPath := zfile.RealPath(originalPath)

	isRel := !filepath.IsAbs(originalPath)
	if realPath == "" {
		realPath = originalPath
	}

	if isRel {
		rel := strings.TrimPrefix(filepath.Clean(originalPath), "./")
		if l.host != "" {
			return filepath.Join(l.getHostDir(), rel)
		}
		return filepath.Join(l.repoPath, rel)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		fileName := filepath.Base(realPath)
		if l.host != "" {
			hostDir := l.getHostDir()
			return filepath.Join(hostDir, fileName)
		}
		return filepath.Join(l.repoPath, fileName)
	}

	relPath, err := filepath.Rel(homeDir, realPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		fileName := filepath.Base(realPath)
		if l.host != "" {
			hostDir := l.getHostDir()
			return filepath.Join(hostDir, fileName)
		}
		return filepath.Join(l.repoPath, fileName)
	}

	if l.host != "" {
		return filepath.Join(l.getHostDir(), relPath)
	}
	return filepath.Join(l.repoPath, relPath)
}

func (l *Lnk) getOriginalFilePath(repoPath string) string {
	fileName := filepath.Base(repoPath)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fileName
	}

	commonPaths := []string{
		filepath.Join(homeDir, fileName),
		filepath.Join(homeDir, ".config", fileName),
		filepath.Join(homeDir, "."+fileName),
	}

	for _, path := range commonPaths {
		if l.fs.FileExists(path) {
			return path
		}
	}

	return filepath.Join(homeDir, fileName)
}

func (l *Lnk) validateRepoPath() error {
	realPath := zfile.RealPath(l.repoPath)
	if realPath == "" {
		return fmt.Errorf("无法解析仓库路径: %s", l.repoPath)
	}

	l.repoPath = realPath
	return nil
}

func (l *Lnk) ensureRepoDir() error {
	return l.fs.EnsureDir(l.repoPath)
}

func (l *Lnk) ensureHostDir() error {
	if l.host == "" || l.host == "localhost" {
		return nil
	}

	hostDir := l.getHostDir()
	return l.fs.EnsureDir(hostDir)
}

func (l *Lnk) getRelativePathInRepo(filePath string) string {
	repoFilePath := l.getRepoFilePath(filePath)
	relPath, err := filepath.Rel(l.repoPath, repoFilePath)
	if err != nil {
		return filepath.Base(repoFilePath)
	}
	return relPath
}

func (l *Lnk) expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(homeDir, path[2:])
		}
	}

	path = os.ExpandEnv(path)

	realPath := zfile.RealPath(path)
	if realPath != "" {
		return realPath
	}

	return path
}

func (l *Lnk) normalizeFilePath(path string) string {
	expanded := l.expandPath(path)

	absPath, err := filepath.Abs(expanded)
	if err != nil {
		return expanded
	}

	return absPath
}

func (l *Lnk) toTrackingPath(path string) string {
	// Ensure absolute for normalization
	abs := l.normalizeFilePath(path)
	abs = filepath.Clean(abs)

	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		return abs
	}
	homeDir = filepath.Clean(homeDir)

	sep := string(os.PathSeparator)
	prefix := homeDir + sep
	if strings.HasPrefix(abs, prefix) {
		rel := strings.TrimPrefix(abs, prefix)
		rel = strings.TrimPrefix(rel, "./")
		return rel
	}
	return abs
}

func (l *Lnk) isValidFileName(name string) bool {
	invalidChars := []string{"\x00", "/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range invalidChars {
		if strings.Contains(name, char) {
			return false
		}
	}

	reservedNames := []string{".", "..", "CON", "PRN", "AUX", "NUL"}
	upperName := strings.ToUpper(name)
	for _, reserved := range reservedNames {
		if upperName == reserved {
			return false
		}
	}

	return true
}

func (l *Lnk) createBackup(filePath string) (string, error) {
	backupPath := filePath + ".lnk.backup"

	if l.fs.FileExists(backupPath) {
		backupPath = fmt.Sprintf("%s.lnk.backup.%d", filePath, os.Getpid())
	}

	info, err := l.fs.GetFileInfo(filePath)
	if err != nil {
		return "", err
	}

	if err := l.fs.Move(filePath, backupPath, info); err != nil {
		return "", fmt.Errorf("创建备份失败: %w", err)
	}

	return backupPath, nil
}

func (l *Lnk) restoreBackup(backupPath, originalPath string) error {
	info, err := l.fs.GetFileInfo(backupPath)
	if err != nil {
		return err
	}

	return l.fs.Move(backupPath, originalPath, info)
}

func (l *Lnk) Init() error {
	if l.IsInitialized() {
		return &RepoAlreadyExistsError{
			RepoPath: l.repoPath,
		}
	}

	if err := l.validateRepoPath(); err != nil {
		return err
	}

	if err := l.ensureRepoDir(); err != nil {
		return fmt.Errorf("创建仓库目录失败: %w", err)
	}

	if err := l.git.Init(); err != nil {
		return fmt.Errorf("初始化 Git 仓库失败: %w", err)
	}

	if err := l.writeTrackingEntries([]TrackedEntry{}); err != nil {
		return fmt.Errorf("创建跟踪文件失败: %w", err)
	}

	trackingFile := l.getTrackingFilePath()
	relPath, err := filepath.Rel(l.repoPath, trackingFile)
	if err != nil {
		relPath = filepath.Base(trackingFile)
	}

	if err := l.git.Add(relPath); err != nil {
		return fmt.Errorf("添加跟踪文件到 Git 失败: %w", err)
	}

	if err := l.git.Commit("lnk: 初始化仓库"); err != nil {
		return fmt.Errorf("初始提交失败: %w", err)
	}

	return nil
}

func (l *Lnk) InitWithRemote(remoteURL string) error {
	return l.initWithRemote(remoteURL, false, true)
}

func (l *Lnk) InitWithRemoteForce(remoteURL string, noBootstrap bool) error {
	return l.initWithRemote(remoteURL, true, !noBootstrap)
}

func (l *Lnk) initWithRemote(remoteURL string, force bool, runBootstrap bool) error {
	if l.IsInitialized() && !force {
		return &RepoAlreadyExistsError{
			RepoPath: l.repoPath,
		}
	}

	if force && l.fs.IsDir(l.repoPath) {
		if err := os.RemoveAll(l.repoPath); err != nil {
			return fmt.Errorf("清理现有目录失败: %w", err)
		}
	}

	if err := l.validateRepoPath(); err != nil {
		return err
	}

	if err := l.git.Clone(remoteURL); err != nil {
		return fmt.Errorf("克隆远程仓库失败: %w", err)
	}

	if err := l.validateLnkRepo(); err != nil {

		os.RemoveAll(l.repoPath)
		return fmt.Errorf("无效的 lnk 仓库: %w", err)
	}

	if err := l.ensureHostDir(); err != nil {
		return fmt.Errorf("创建主机目录失败: %w", err)
	}

	if runBootstrap {
		if err := l.runBootstrapIfExists(); err != nil {
			fmt.Printf("警告: bootstrap 脚本执行失败: %v\n", err)
		}
	}

	return nil
}

func (l *Lnk) validateLnkRepo() error {
	if !l.git.IsRepo() {
		return fmt.Errorf("不是有效的 Git 仓库")
	}

	trackingFile := l.getTrackingFilePath()
	generalTrackingFile := filepath.Join(l.repoPath, ".lnk")

	if !l.fs.FileExists(trackingFile) && !l.fs.FileExists(generalTrackingFile) {
		if !l.hasAnyLnkFiles() {
			return fmt.Errorf("未找到 lnk 跟踪文件")
		}
	}

	return nil
}

func (l *Lnk) hasAnyLnkFiles() bool {
	entries, err := os.ReadDir(l.repoPath)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if name == ".lnk" || strings.HasPrefix(name, ".lnk.") {
			return true
		}
	}

	return false
}

func (l *Lnk) runBootstrapIfExists() error {
	bootstrapScript := filepath.Join(l.repoPath, "bootstrap.sh")

	if !l.fs.FileExists(bootstrapScript) {
		return nil
	}

	return l.RunBootstrapScript()
}

func (l *Lnk) RunBootstrapScript() error {
	bootstrapScript := filepath.Join(l.repoPath, "bootstrap.sh")

	if !l.fs.FileExists(bootstrapScript) {
		return &BootstrapScriptNotFoundError{
			RepoPath: l.repoPath,
		}
	}

	if runtime.GOOS == "windows" {
		util.Log.Warn("Windows 不支持执行 bootstrap.sh 脚本: %s\n", bootstrapScript)
		return nil
	}

	if err := os.Chmod(bootstrapScript, 0o755); err != nil {
		return fmt.Errorf("设置脚本权限失败: %w", err)
	}

	cmd := exec.Command("/bin/bash", bootstrapScript)
	cmd.Dir = l.repoPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("执行 bootstrap 脚本失败: %w", err)
	}

	return nil
}

func (l *Lnk) FindBootstrapScript() (string, error) {
	bootstrapScript := filepath.Join(l.repoPath, "bootstrap.sh")

	if !l.fs.FileExists(bootstrapScript) {
		return "", &BootstrapScriptNotFoundError{
			RepoPath: l.repoPath,
		}
	}

	return bootstrapScript, nil
}

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

	// commitMsg := fmt.Sprintf("lnk: 递归添加 %d 个文件", len(processedFiles))
	// if err := l.git.Commit(commitMsg); err != nil {
	// 	rollback()
	// 	return fmt.Errorf("提交变更失败: %w", err)
	// }

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
		target, err := l.fs.ReadSymlink(absPath)
		if err != nil || !l.fs.FileExists(target) {
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

func (l *Lnk) ValidateRepository() error {
	if !l.IsInitialized() {
		return &RepoNotInitializedError{
			RepoPath: l.repoPath,
		}
	}

	trackingFile := l.getTrackingFilePath()
	if !l.fs.FileExists(trackingFile) {
		return fmt.Errorf("跟踪文件不存在: %s", trackingFile)
	}

	entries, err := l.readTrackingEntries()
	if err != nil {
		return fmt.Errorf("读取管理文件列表失败: %w", err)
	}

	var errors []string
	for _, ent := range entries {
		filePath := ent.Path
		repoFilePath := l.getRepoFilePath(filePath)

		if !l.fs.FileExists(repoFilePath) {
			errors = append(errors, fmt.Sprintf("仓库文件不存在: %s", repoFilePath))
		}

		absPath := filePath
		if !filepath.IsAbs(filePath) {
			if homeDir, err := os.UserHomeDir(); err == nil {
				absPath = filepath.Join(homeDir, strings.TrimPrefix(filepath.Clean(filePath), "./"))
			}
		}

		if ent.Type == LinkTypeHard {
			if !l.fs.FileExists(absPath) {
				errors = append(errors, fmt.Sprintf("硬链接不存在: %s", absPath))
			}
			continue
		}
		if !l.fs.FileExists(absPath) {
			errors = append(errors, fmt.Sprintf("符号链接不存在: %s", absPath))
			continue
		}
		if !l.fs.IsSymlink(absPath) {
			errors = append(errors, fmt.Sprintf("不是符号链接: %s", absPath))
			continue
		}
		target, err := l.fs.ReadSymlink(absPath)
		if err != nil || !l.fs.FileExists(target) {
			errors = append(errors, fmt.Sprintf("符号链接目标不存在或无效: %s", absPath))
			continue
		}
		if target != repoFilePath {
			errors = append(errors, fmt.Sprintf("符号链接目标不正确: %s", absPath))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("仓库验证失败:\n%s", strings.Join(errors, "\n"))
	}

	return nil
}

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
