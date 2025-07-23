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
				util.Log.Printf("Change: %v (%v)\n", relativeFilePath, opType)
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
	util.Log.Printf("Change: %v (%v)\n", cf.Name, cf.Type)
	task.Put(cf)
	sendChang(cf)
}
