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
	for _, dir := range watchDirs {
		_dir := dir
		if _dir == "." {
			_dir = projectFolder
		}
		util.Log.Println("监控:", _dir)
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

	if !inStringArray(fullDir, watchDirs) {
		watchDirs = append(watchDirs, fullDir)
		util.Log.Println("监控:", fullDir)
		err := watcher.Add(fullDir)
		if err != nil {
			util.Log.Errorf("添加监控失败 %s: %v\n", fullDir, err)
		}
	}
}

func removeWatcher(dir string) {
	if inStringArray(dir, watchDirs) {
		if len(watchDirs) > 0 {
			for i, v := range watchDirs {
				if v == dir {
					watchDirs = append(watchDirs[:i], watchDirs[i+1:]...)
					break
				}
			}
		}
	}
}

func otherWatcher(name string, event fsnotify.Op) {
	// util.Log.Debug("otherWatcher", name, event)
}

func arrIncludeDirs() {
	for i := 0; i < len(includeDirs); i++ {
		arr := dirParse2Array(includeDirs[i])
		isD := strings.Index(arr[0], ".") == 0
		if len(arr) < 1 || len(arr) > 2 {
			util.Log.Fatal("监听文件路径错误:", includeDirs[i])
		}
		if strings.HasPrefix(arr[0], "/") {
			util.Log.Fatal("监控目录必须是相对路径:", includeDirs[i])
		}
		isAll := len(arr) == 2 && arr[1] == "*"

		addFiles := func(dir string) {
			dir = zfile.RealPath(dir)

			if isExcept(exceptDirs, dir) {
				util.Log.Debugf("从监控中排除目录: %s", dir)
				return
			}

			if isAll {
				watchDirs = append(watchDirs, dir)
				listFile(dir, func(d string) {
					watchDirs = arrayUniqueAdd(watchDirs, zfile.RealPath(d, true))
				})
			} else if !isIgnoreDirectory(dir) {
				watchDirs = arrayUniqueAdd(watchDirs, zfile.RealPath(dir, true))
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
			md = zfile.RealPath(md)
			if len(arr) == 2 && arr[1] == "*" {
				watchDirs = arrayUniqueAdd(watchDirs, md)
				listFile(md, func(d string) {
					path, _ := filepath.Abs(d)
					watchDirs = arrayUniqueAdd(watchDirs, path)
				})
			} else {
				watchDirs = arrayUniqueAdd(watchDirs, md)
			}
		}
	}
}
