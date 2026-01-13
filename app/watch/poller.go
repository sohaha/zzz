package watch

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

var (
	errPollerClosed = errors.New("轮询器已关闭")
	errNoSuchWatch  = errors.New("监听不存在")
)

const watchWaitTime = 200 * time.Millisecond

type filePoller struct {
	watches map[string]chan struct{}
	events  chan fsnotify.Event
	errors  chan error
	mu      sync.Mutex
	closed  bool
}

func (w *filePoller) Add(name string) error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return errPollerClosed
	}

	f, err := os.Open(name)
	if err != nil {
		w.mu.Unlock()
		return err
	}
	fi, err := os.Stat(name)
	if err != nil {
		w.mu.Unlock()
		f.Close()
		return err
	}

	if w.watches == nil {
		w.watches = make(map[string]chan struct{})
	}
	if _, exists := w.watches[name]; exists {
		w.mu.Unlock()
		f.Close()
		return nil
		// return fmt.Errorf("watch exists")
	}
	chClose := make(chan struct{})
	w.watches[name] = chClose
	w.mu.Unlock()
	if fi.IsDir() {
		return filepath.Walk(f.Name(), func(path string, info os.FileInfo, err error) error {
			if isIgnoreDirectory(info.Name()) {
				return filepath.SkipDir
			}
			return w.Add(path)
		})
	}

	go w.watch(f, fi, chClose)
	return nil
}

func (w *filePoller) Remove(name string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.remove(name)
}

func (w *filePoller) remove(name string) error {
	if w.closed {
		return errPollerClosed
	}
	chClose, exists := w.watches[name]
	if !exists {
		return errNoSuchWatch
	}
	close(chClose)
	delete(w.watches, name)
	return nil
}

func (w *filePoller) Events() <-chan fsnotify.Event {
	return w.events
}

func (w *filePoller) Errors() <-chan error {
	return w.errors
}

func (w *filePoller) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}

	for name := range w.watches {
		_ = w.remove(name)
	}
	w.closed = true
	return nil
}

func (w *filePoller) sendEvent(e fsnotify.Event, chClose <-chan struct{}) error {
	select {
	case w.events <- e:
	case <-chClose:
		return fmt.Errorf("已关闭")
	}
	return nil
}

func (w *filePoller) sendErr(e error, chClose <-chan struct{}) error {
	select {
	case w.errors <- e:
	case <-chClose:
		return fmt.Errorf("已关闭")
	}
	return nil
}

func (w *filePoller) watch(f *os.File, lastFi os.FileInfo, chClose chan struct{}) {
	defer f.Close()

	timer := time.NewTimer(watchWaitTime)
	if !timer.Stop() {
		<-timer.C
	}
	defer timer.Stop()

	for {
		timer.Reset(watchWaitTime)

		select {
		case <-timer.C:
		case <-chClose:
			// util.Log.Debugf("watch for %s closed", f.Name())
			return
		}

		fi, err := os.Stat(f.Name())
		if err != nil {
			if lastFi == nil {
				continue
			}
			if os.IsNotExist(err) {
				if err := w.sendEvent(fsnotify.Event{Op: fsnotify.Remove, Name: f.Name()}, chClose); err != nil {
					return
				}
				lastFi = nil
				continue
			}
			if err := w.sendErr(err, chClose); err != nil {
				return
			}
			continue
		}

		if lastFi == nil {
			if err := w.sendEvent(fsnotify.Event{Op: fsnotify.Create, Name: fi.Name()}, chClose); err != nil {
				return
			}
			lastFi = fi
			continue
		}

		if fi.Mode() != lastFi.Mode() {
			if err := w.sendEvent(fsnotify.Event{Op: fsnotify.Chmod, Name: fi.Name()}, chClose); err != nil {
				return
			}
			lastFi = fi
			continue
		}

		if fi.ModTime() != lastFi.ModTime() || fi.Size() != lastFi.Size() {
			if err := w.sendEvent(fsnotify.Event{Op: fsnotify.Write, Name: fi.Name()}, chClose); err != nil {
				return
			}
			lastFi = fi
			continue
		}
	}
}
