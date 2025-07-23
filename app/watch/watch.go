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
		util.Log.Println("Watcher: ", _dir)
		err := watcher.Add(dir)
		if err != nil {
			util.Log.Fatal(err)
		}
	}
	util.Log.Println("Watching...")
}

func addNewWatcher(dir string) {
	fullDir := filepath.ToSlash(dir)

	// 使用改进的isExcept函数检查是否应该排除
	if isExcept(exceptDirs, fullDir) {
		util.Log.Debugf("Excluding new directory: %s", fullDir)
		return
	}

	// 检查是否是忽略的目录类型
	if isIgnoreDirectory(fullDir) {
		util.Log.Debugf("Ignoring directory type: %s", fullDir)
		return
	}

	if !inStringArray(fullDir, watchDirs) {
		watchDirs = append(watchDirs, fullDir)
		util.Log.Printf("Adding new watcher: %s", fullDir)
		err := watcher.Add(fullDir)
		if err != nil {
			util.Log.Errorf("Failed to add new watcher for %s: %v", fullDir, err)
		} else {
			util.Log.Infof("Successfully added new watcher: %s", fullDir)
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
			util.Log.Fatal("Error listening for file path: ", includeDirs[i])
		}
		if strings.HasPrefix(arr[0], "/") {
			util.Log.Fatal("watchDirs must be relative paths: ", includeDirs[i])
		}
		isAll := len(arr) == 2 && arr[1] == "*"

		addFiles := func(dir string) {
			dir = zfile.RealPath(dir)

			if isExcept(exceptDirs, dir) {
				util.Log.Debugf("Excluding directory from watch: %s", dir)
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
				util.Log.Errorf("Invalid glob pattern %s: %v", arr[0], err)
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
