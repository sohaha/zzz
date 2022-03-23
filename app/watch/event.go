package watch

import (
	"github.com/sohaha/zzz/util"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sohaha/zlsgo/zfile"
)

func eventDispatcher(event fsnotify.Event) {
	ext := path.Ext(event.Name)
	event.Name = zfile.RealPath(event.Name)
	isDir := zfile.DirExist(event.Name)
	switch event.Op {
	case fsnotify.Create:
		if isDir {
			addNewWatcher(event.Name)
		}
	case fsnotify.Remove:
		removeWatcher(event.Name)
		return
	case fsnotify.Rename:
	case fsnotify.Write:
		if len(types) > 0 && types[0] != ".*" && !inStringArray(ext, types) {
			return
		}
		fileChange(event)
	default:
		otherWatcher(event.Name, event.Op)
	}
}

func fileChange(event fsnotify.Event) {
	switch event.Op {
	case fsnotify.Write, fsnotify.Remove, fsnotify.Rename:
		if strings.HasSuffix(event.Name, "_static_resources.go") {
			// ignore zzz build temporary files
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
		push := func() {
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

		util.Log.Printf("Change: %v (%v)", relativeFilePath, opType)
	}
}
