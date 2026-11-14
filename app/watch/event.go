package watch

import (
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/sohaha/zzz/util"

	"github.com/fsnotify/fsnotify"
	"github.com/sohaha/zlsgo/zfile"
)

func eventDispatcher(event fsnotify.Event) {
	isLinkDir, err := isSymlinkDirectory(event.Name)
	if err == nil && isLinkDir {
		return
	}

	ext := path.Ext(event.Name)
	event.Name = zfile.RealPath(event.Name)
	isDir := zfile.DirExist(event.Name)
	switch event.Op {
	case fsnotify.Create:
		if isDir {
			addNewWatcher(event.Name)
			return
		}
		if !isTypeWatched(ext) {
			return
		}
		fileChange(event)
	case fsnotify.Remove:
		removeWatcher(event.Name)
		if isDir {
			_ = watcher.Remove(event.Name)
		}
		return
	case fsnotify.Rename:
		removeWatcher(event.Name)
		if isDir {
			_ = watcher.Remove(event.Name)
			return
		}
		fileChange(event)
	case fsnotify.Write:
		if !isTypeWatched(ext) {
			return
		}
		fileChange(event)
	default:
		otherWatcher(event.Name, event.Op)
	}
}

func isTypeWatched(ext string) bool {
	if len(types) == 0 || types[0] == ".*" {
		return true
	}
	return inStringArray(ext, types)
}

func fileChange(event fsnotify.Event) {
	switch event.Op {
	case fsnotify.Create, fsnotify.Write, fsnotify.Remove, fsnotify.Rename:
		if strings.HasSuffix(event.Name, "_static_resources.go") {
			return
		}
		ext := path.Ext(event.Name)
		fileName, _ := filepath.Abs(event.Name)
		fileName = filepath.ToSlash(fileName)
		opType := event.Op.String()
		relativeFilePath, _ := filepath.Rel(projectFolder, fileName)
		relativeFilePath = filepath.ToSlash(relativeFilePath)
		data := &changedFile{
			Name:    relativeFilePath,
			Path:    fileName,
			Changed: time.Now().UnixNano(),
			Ext:     ext,
			Type:    opType,
		}

		if fileDebouncer != nil {
			pendingFiles.Store(relativeFilePath, data)
			fileDebouncer.trigger(relativeFilePath)
		} else {
			push := func() {
				util.Log.Printf("变更: %v (%v)\n", relativeFilePath, opType)
				task.Put(data)
				sendChang(data)
			}

			if lashTime, ok := pushTimer.Load(relativeFilePath); ok {
				lashTime.(*time.Timer).Stop()
				pushTimer.Delete(relativeFilePath)
			}
			pushTimer.Store(relativeFilePath, time.AfterFunc(getDelay(), func() {
				pushTimer.Delete(relativeFilePath)
				push()
			}))
		}
	}
}

func handleFileChangeDebounced(cf *changedFile) {
	util.Log.Printf("变更: %v (%v)\n", cf.Name, cf.Type)
	task.Put(cf)
	sendChang(cf)
}
