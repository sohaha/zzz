package watch

import "github.com/fsnotify/fsnotify"

type fsNotifyWatcher struct {
	*fsnotify.Watcher
}

func (w *fsNotifyWatcher) Events() <-chan fsnotify.Event {
	return w.Watcher.Events
}

func (w *fsNotifyWatcher) Errors() <-chan error {
	return w.Watcher.Errors
}
