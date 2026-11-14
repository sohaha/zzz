package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sohaha/zlsgo/zcli"
	"github.com/sohaha/zlsgo/zfile"
	"github.com/sohaha/zzz/app/lnk/fs"
	"github.com/sohaha/zzz/app/lnk/git"
	"github.com/sohaha/zzz/util"
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

	if runtime.GOOS == "windows" && !zcli.IsSudo() {
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
		util.Log.Warnf("Windows 不支持执行 bootstrap.sh 脚本: %s", bootstrapScript)
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
		target, err := l.fs.ReadResolvedSymlink(absPath)
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
