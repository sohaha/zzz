package watch

import "github.com/fsnotify/fsnotify"

type FileWatcher interface {
	Events() <-chan fsnotify.Event
	Errors() <-chan error
	Add(name string) error
	Remove(name string) error
	Close() error
}

func NewWatcher() (FileWatcher, error) {
	if watcher, err := NewEventWatcher(); err == nil {
		return watcher, nil
	}
	return NewPollingWatcher(), nil
}

func NewPollingWatcher() FileWatcher {
	return &filePoller{
		events: make(chan fsnotify.Event),
		errors: make(chan error),
	}
}

func NewEventWatcher() (FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &fsNotifyWatcher{watcher}, nil
}
