package watch

import (
	"github.com/sohaha/zlsgo/zfile"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/sohaha/zlsgo/zlog"
)

func addWatcher() {
	arrIncludeDirs()
	_ = arrExceptDirs()
	for _, dir := range watchDirs {
		_dir := dir
		if _dir == "." {
			_dir = projectFolder
		}
		zlog.Println("Watcher: ", _dir)
		err := watcher.Add(dir)
		if err != nil {
			zlog.Fatal(err)
		}
	}
	zlog.Println("Watching...")
}

func addNewWatcher(dir string) {
	fullDir := filepath.ToSlash(dir)
	for i := 0; i < len(exceptDirs); i++ {
		if dir == exceptDirs[i] {
			return
		}
	}
	if !inStringArray(fullDir, watchDirs) {
		watchDirs = append(watchDirs, fullDir)
		//isExceptDirs := arrExceptDirs()
		//if isExceptDirs {
		//	return
		//}
		zlog.Println("Watcher: ", fullDir)
		err := watcher.Add(fullDir)
		if err != nil {
			zlog.Fatal(err)
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
	// zlog.Debug("otherWatcher", name, event)
}

func arrIncludeDirs() {
	for i := 0; i < len(includeDirs); i++ {
		arr := dirParse2Array(includeDirs[i])
		isD := strings.Index(arr[0], ".") == 0
		if len(arr) < 1 || len(arr) > 2 {
			zlog.Fatal("Error listening for file path: ", includeDirs[i])
		}
		if strings.HasPrefix(arr[0], "/") {
			zlog.Fatal("watchDirs must be relative paths: ", includeDirs[i])
		}
		isAll := len(arr) == 2 && arr[1] == "*"
		addFiles := func(dir string) {
			dir = zfile.RealPath(dir)
			if isAll {
				watchDirs = append(watchDirs, dir)
				listFile(dir, func(d string) {
					//path, _ := filepath.Abs(d)
					watchDirs = arrayUniqueAdd(watchDirs, d)
				})
			} else if !isIgnoreDirectory(dir) {
				//path, _ := filepath.Abs(dir)
				watchDirs = arrayUniqueAdd(watchDirs, dir)
			}
		}

		if arr[0] == "." {
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

func arrExceptDirs() (update bool) {
	for i := 0; i < len(exceptDirs); i++ {
		p := exceptDirs[i]
		watchDirs = arrayRemoveElement(watchDirs, p)
	}
	return update
}
