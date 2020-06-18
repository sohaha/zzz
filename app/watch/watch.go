package watch

import (
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
	exceptDirs := v.GetStringSlice("monitor.ExceptDirs")
	fullDir := filepath.ToSlash(dir)
	for i := 0; i < len(exceptDirs); i++ {
		if dir == exceptDirs[i] {
			return
		}
	}

	if !inStringArray(fullDir, watchDirs) {
		watchDirs = append(watchDirs, fullDir)
		isExceptDirs := arrExceptDirs()
		if isExceptDirs {
			return
		}
		zlog.Println("Watcher: ", fullDir)
		err := watcher.Add(fullDir)
		if err != nil {
			zlog.Fatal(err)
		}
	}
}

func removeWatcher(dir string) {
	fullDir := filepath.ToSlash(dir)
	err := watcher.Remove(fullDir)
	if err == nil && inStringArray(fullDir, watchDirs) {
		if len(watchDirs) > 0 {
			for i, v := range watchDirs {
				if v == fullDir {
					watchDirs = append(watchDirs[:i], watchDirs[i+1:]...)
					break
				}
			}
		}

	}
	zlog.Println("RemoveWatcher: ", fullDir)
}

func otherWatcher(name string, event fsnotify.Op) {
	// zlog.Debug("otherWatcher", name, event)
}

func arrIncludeDirs() {
	includeDirs := v.GetStringSlice("monitor.includeDirs")
	for i := 0; i < len(includeDirs); i++ {
		darr := dirParse2Array(includeDirs[i])

		isD := strings.Index(darr[0], ".") == 0

		if len(darr) < 1 || len(darr) > 2 {
			zlog.Fatal("Error listening for file path: ", includeDirs[i])
		}
		if strings.HasPrefix(darr[0], "/") {
			zlog.Fatal("watchDirs must be relative paths: ", includeDirs[i])
		}
		isAll := len(darr) == 2 && darr[1] == "*"
		addFiles := func(dir string) {
			if isAll {
				watchDirs = append(watchDirs, dir)
				listFile(dir, func(d string) {
					path, _ := filepath.Abs(d)
					watchDirs = arrayUniqueAdd(watchDirs, path)
				})
			} else {
				path, _ := filepath.Abs(dir)
				watchDirs = arrayUniqueAdd(watchDirs, path)
			}
		}

		if darr[0] == "." {
			addFiles(projectFolder)
		} else if isD {
			path, _ := filepath.Abs(darr[0])
			addFiles(path)
		} else {
			md := darr[0]
			if !filepath.IsAbs(md) {
				md = projectFolder + "/" + darr[0]
			}
			if len(darr) == 2 && darr[1] == "*" {
				path, _ := filepath.Abs(md)
				watchDirs = arrayUniqueAdd(watchDirs, path)
				listFile(md, func(d string) {
					path, _ := filepath.Abs(d)
					watchDirs = arrayUniqueAdd(watchDirs, path)
				})
			} else {
				path, _ := filepath.Abs(md)
				watchDirs = arrayUniqueAdd(watchDirs, path)
			}
		}
	}
}

func arrExceptDirs() (update bool) {
	exceptDirs := v.GetStringSlice("monitor.ExceptDirs")
	for i := 0; i < len(exceptDirs); i++ {
		p := exceptDirs[i]
		if !filepath.IsAbs(p) {
			p = projectFolder + "/" + exceptDirs[i]
		}
		path, _ := filepath.Abs(p)
		update = true
		watchDirs = arrayRemoveElement(watchDirs, path)
		listFile(p, func(d string) {
			path, _ := filepath.Abs(d)
			watchDirs = arrayRemoveElement(watchDirs, path)
		})
	}

	return update
}
