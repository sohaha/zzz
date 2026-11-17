package watch

import (
	"path/filepath"
	"strings"

	"github.com/sohaha/zlsgo/zfile"
	"github.com/sohaha/zzz/util"

	"github.com/fsnotify/fsnotify"
)

func addWatcher() {
	arrIncludeDirs()
	for _, dir := range watchedDirsSnapshot() {
		if dir == "." {
			dir = projectFolder
		}
		util.Log.Println("监控:", dir)
		err := watcher.Add(dir)
		if err != nil {
			util.Log.Fatal(err)
		}
	}
	util.Log.Println("监控中...")
}

func addNewWatcher(dir string) {
	fullDir := filepath.ToSlash(dir)
	if isExcept(exceptDirs, fullDir) {
		// util.Log.Debugf("Excluding new directory: %s\n", fullDir)
		return
	}
	if isIgnoreDirectory(fullDir) {
		// util.Log.Debugf("Ignoring directory type: %s\n", fullDir)
		return
	}

	if !addWatchDir(fullDir) {
		return
	}
	util.Log.Println("监控:", fullDir)
	if err := watcher.Add(fullDir); err != nil {
		util.Log.Errorf("添加监控失败 %s: %v\n", fullDir, err)
	}
}

func removeWatcher(dir string) {
	removeWatchDir(dir)
}

// syncWatchDirSet rebuilds the lookup index from watchDirs. Callers may replace
// watchDirs directly (tests do), so the index is rebuilt before it is used.
func syncWatchDirSet() {
	watchDirsMu.Lock()
	defer watchDirsMu.Unlock()
	set := make(map[string]struct{}, len(watchDirs))
	for _, dir := range watchDirs {
		set[dir] = struct{}{}
	}
	watchDirSet = set
}

// watchedDirsSnapshot returns a copy so callers can iterate without holding
// the lock while calling into the watcher.
func watchedDirsSnapshot() []string {
	watchDirsMu.RLock()
	defer watchDirsMu.RUnlock()
	return append([]string(nil), watchDirs...)
}

// addWatchDir appends dir once, keeping the slice and the lookup index in sync.
func addWatchDir(dir string) bool {
	watchDirsMu.Lock()
	defer watchDirsMu.Unlock()
	if _, exists := watchDirSet[dir]; exists {
		return false
	}
	watchDirSet[dir] = struct{}{}
	watchDirs = append(watchDirs, dir)
	return true
}

func removeWatchDir(dir string) bool {
	watchDirsMu.Lock()
	defer watchDirsMu.Unlock()
	if _, exists := watchDirSet[dir]; !exists {
		return false
	}
	delete(watchDirSet, dir)
	for i, v := range watchDirs {
		if v == dir {
			watchDirs = append(watchDirs[:i], watchDirs[i+1:]...)
			break
		}
	}
	return true
}

func otherWatcher(name string, event fsnotify.Op) {
	// util.Log.Debug("otherWatcher", name, event)
}

func arrIncludeDirs() {
	syncWatchDirSet()
	for i := 0; i < len(includeDirs); i++ {
		arr := dirParse2Array(includeDirs[i])
		if len(arr) < 1 {
			util.Log.Warnf("忽略空的监听目录配置: %q\n", includeDirs[i])
			continue
		}
		if len(arr) > 2 {
			util.Log.Fatal("监听文件路径错误:", includeDirs[i])
		}
		isD := strings.Index(arr[0], ".") == 0
		if strings.HasPrefix(arr[0], "/") {
			util.Log.Fatal("监控目录必须是相对路径:", includeDirs[i])
		}
		isAll := len(arr) == 2 && arr[1] == "*"

		addFiles := func(dir string) {
			if !filepath.IsAbs(dir) {
				dir = filepath.Join(projectFolder, dir)
			}
			dir = filepath.Clean(dir)
			isLinkDir, err := isSymlinkDirectory(dir)
			if err != nil {
				util.Log.Warnf("跳过无法解析的目录 %s: %v\n", dir, err)
				return
			}
			if isLinkDir {
				util.Log.Warnf("跳过软链接目录: %s\n", dir)
				return
			}

			dir = zfile.RealPath(dir)

			if isExcept(exceptDirs, dir) {
				util.Log.Debugf("从监控中排除目录: %s", dir)
				return
			}

			if isAll {
				addWatchDir(dir)
				listFile(dir, func(d string) {
					d = zfile.RealPath(d, true)
					if isExcept(exceptDirs, d) {
						return
					}
					addWatchDir(d)
				})
			} else if !isIgnoreDirectory(dir) {
				self := zfile.RealPath(dir, true)
				if !isExcept(exceptDirs, self) {
					addWatchDir(self)
				}
			}
		}

		if strings.Contains(arr[0], "*") {
			matches, err := filepath.Glob(arr[0])
			if err != nil {
				util.Log.Errorf("无效的 glob 模式 %s: %v", arr[0], err)
				continue
			}
			for _, match := range matches {
				if zfile.DirExist(match) {
					addFiles(match)
				}
			}
		} else if arr[0] == "." {
			addFiles(projectFolder)
		} else if isD {
			addFiles(arr[0])
		} else {
			md := arr[0]
			if !filepath.IsAbs(md) {
				md = filepath.Join(projectFolder, md)
			}
			md = filepath.Clean(md)
			isLinkDir, err := isSymlinkDirectory(md)
			if err != nil {
				util.Log.Warnf("跳过无法解析的目录 %s: %v\n", md, err)
				continue
			}
			if isLinkDir {
				util.Log.Warnf("跳过软链接目录: %s\n", md)
				continue
			}
			md = zfile.RealPath(md)
			if len(arr) == 2 && arr[1] == "*" {
				addWatchDir(md)
				listFile(md, func(d string) {
					path, _ := filepath.Abs(d)
					if !isExcept(exceptDirs, path) {
						addWatchDir(path)
					}
				})
			} else {
				addWatchDir(md)
			}
		}
	}
}
