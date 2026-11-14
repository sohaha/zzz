package watch

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sohaha/zlsgo/zfile"
)

func TestIsTypeWatched(t *testing.T) {
	oldTypes := types
	t.Cleanup(func() {
		types = oldTypes
	})

	cases := []struct {
		name  string
		types []string
		ext   string
		want  bool
	}{
		{"空配置全部监听", nil, ".go", true},
		{"通配符全部监听", []string{".*"}, ".tmp", true},
		{"命中类型", []string{".go", ".php"}, ".go", true},
		{"未命中类型", []string{".go", ".php"}, ".tmp", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			types = c.types
			if got := isTypeWatched(c.ext); got != c.want {
				t.Fatalf("isTypeWatched(%q) with types=%v = %v, want %v", c.ext, c.types, got, c.want)
			}
		})
	}
}

func TestEventDispatcherRemoveStopsWatchingDir(t *testing.T) {
	dir := t.TempDir()
	dir = zfile.RealPath(dir)

	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("new watcher: %v", err)
	}
	t.Cleanup(func() {
		_ = fsWatcher.Close()
	})

	watcher = &fsNotifyWatcher{fsWatcher}
	if err := watcher.Add(dir); err != nil {
		t.Fatalf("add: %v", err)
	}

	eventDispatcher(fsnotify.Event{Op: fsnotify.Remove, Name: dir})

	for _, p := range fsWatcher.WatchList() {
		if p == dir {
			t.Fatalf("expected %q to be removed from watch list, got %v", dir, fsWatcher.WatchList())
		}
	}
}

func TestEventDispatcherRenameStopsWatchingDir(t *testing.T) {
	dir := t.TempDir()
	dir = zfile.RealPath(dir)

	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("new watcher: %v", err)
	}
	t.Cleanup(func() {
		_ = fsWatcher.Close()
	})

	watcher = &fsNotifyWatcher{fsWatcher}
	if err := watcher.Add(dir); err != nil {
		t.Fatalf("add: %v", err)
	}

	eventDispatcher(fsnotify.Event{Op: fsnotify.Rename, Name: dir})

	for _, p := range fsWatcher.WatchList() {
		if p == dir {
			t.Fatalf("expected %q to be removed from watch list, got %v", dir, fsWatcher.WatchList())
		}
	}
}

func TestEventDispatcherCreateFileTriggersChange(t *testing.T) {
	root := t.TempDir()
	root = zfile.RealPath(root)

	oldTypes := types
	oldProjectFolder := projectFolder
	t.Cleanup(func() {
		types = oldTypes
		projectFolder = oldProjectFolder
	})
	types = []string{".go"}
	projectFolder = root

	triggered := make(chan string, 1)
	fileDebouncer = newDebouncer(1, func(filePath string) {
		triggered <- filePath
	})
	t.Cleanup(func() {
		fileDebouncer.stop()
		fileDebouncer = nil
		pendingFiles.Range(func(key, _ interface{}) bool {
			pendingFiles.Delete(key)
			return true
		})
	})

	file := filepath.Join(root, "main.go")
	eventDispatcher(fsnotify.Event{Op: fsnotify.Create, Name: file})

	select {
	case got := <-triggered:
		if want, _ := filepath.Rel(projectFolder, zfile.RealPath(file)); got != filepath.ToSlash(want) {
			t.Fatalf("triggered path = %q, want %q", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("expected created .go file to trigger a change")
	}
}

func TestEventDispatcherCreateFileRespectsTypeFilter(t *testing.T) {
	root := t.TempDir()
	root = zfile.RealPath(root)

	oldTypes := types
	oldProjectFolder := projectFolder
	t.Cleanup(func() {
		types = oldTypes
		projectFolder = oldProjectFolder
	})
	types = []string{".go"}
	projectFolder = root

	triggered := make(chan string, 1)
	fileDebouncer = newDebouncer(1, func(filePath string) {
		triggered <- filePath
	})
	t.Cleanup(func() {
		fileDebouncer.stop()
		fileDebouncer = nil
		pendingFiles.Range(func(key, _ interface{}) bool {
			pendingFiles.Delete(key)
			return true
		})
	})

	file := filepath.Join(root, "notes.tmp")
	eventDispatcher(fsnotify.Event{Op: fsnotify.Create, Name: file})
	eventDispatcher(fsnotify.Event{Op: fsnotify.Write, Name: file})

	select {
	case got := <-triggered:
		t.Fatalf("expected no trigger for filtered type, got %q", got)
	default:
	}
}
