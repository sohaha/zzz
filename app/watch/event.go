package watch

import (
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sohaha/zlsgo/zfile"
	"github.com/sohaha/zlsgo/zlog"
)

func eventDispatcher(event fsnotify.Event) {
	ext := path.Ext(event.Name)
	types := v.GetStringSlice("monitor.types")
	if len(types) > 0 && types[0] != ".*" && !inStringArray(ext, types) {
		if zfile.DirExist(event.Name) {
			if event.Op == fsnotify.Create {
				addNewWatcher(event.Name)
				// } else if event.Op == fsnotify.Remove {
				// 	removeWatcher(event.Name)
				// } else {
				// 	otherWatcher(event.Name, event.Op)
			}
		}
		return
	}
	fileChange(event)
}

func fileChange(event fsnotify.Event) {
	switch event.Op {
	case fsnotify.Write, fsnotify.Remove, fsnotify.Rename:
		if strings.HasSuffix(event.Name, "____tmp.go") {
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

		zlog.Printf("Change: %v (%v)\n", relativeFilePath, opType)
	}
}
