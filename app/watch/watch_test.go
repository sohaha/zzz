package watch

import (
	"path/filepath"
	"testing"

	"github.com/fsnotify/fsnotify"
)

type fakeWatcher struct {
	added []string
}

func (f *fakeWatcher) Events() <-chan fsnotify.Event { return nil }
func (f *fakeWatcher) Errors() <-chan error          { return nil }
func (f *fakeWatcher) Close() error                  { return nil }
func (f *fakeWatcher) Remove(string) error           { return nil }
func (f *fakeWatcher) Add(name string) error {
	f.added = append(f.added, name)
	return nil
}

func TestAddWatcherResolvesDotToProjectFolder(t *testing.T) {
	oldWatcher := watcher
	oldIncludeDirs := includeDirs
	oldWatchDirs := watchDirs
	oldProjectFolder := projectFolder
	t.Cleanup(func() {
		watcher = oldWatcher
		includeDirs = oldIncludeDirs
		watchDirs = oldWatchDirs
		projectFolder = oldProjectFolder
		syncWatchDirSet()
	})

	fake := &fakeWatcher{}
	watcher = fake
	includeDirs = nil
	projectFolder = filepath.ToSlash(t.TempDir())
	watchDirs = []string{"."}

	addWatcher()

	if len(fake.added) != 1 || fake.added[0] != projectFolder {
		t.Fatalf("expected %q to be watched, got %v", projectFolder, fake.added)
	}
}

func TestAddNewWatcherSkipsDuplicates(t *testing.T) {
	oldWatcher := watcher
	oldWatchDirs := watchDirs
	oldExceptDirs := exceptDirs
	t.Cleanup(func() {
		watcher = oldWatcher
		watchDirs = oldWatchDirs
		exceptDirs = oldExceptDirs
		syncWatchDirSet()
	})

	fake := &fakeWatcher{}
	watcher = fake
	exceptDirs = nil
	watchDirs = nil
	syncWatchDirSet()

	dir := filepath.ToSlash(t.TempDir())
	addNewWatcher(dir)
	addNewWatcher(dir)

	if len(fake.added) != 1 {
		t.Fatalf("expected one Add call for duplicate directories, got %v", fake.added)
	}
	if len(watchDirs) != 1 {
		t.Fatalf("expected one watched directory, got %v", watchDirs)
	}
}

func TestRemoveWatcherUpdatesIndex(t *testing.T) {
	oldWatchDirs := watchDirs
	t.Cleanup(func() {
		watchDirs = oldWatchDirs
		syncWatchDirSet()
	})

	dir := filepath.ToSlash(t.TempDir())
	watchDirs = []string{dir}
	syncWatchDirSet()

	removeWatcher(dir)

	if len(watchDirs) != 0 {
		t.Fatalf("expected directory to be removed, got %v", watchDirs)
	}
	if removeWatchDir(dir) {
		t.Fatal("expected second removal to be a no-op")
	}
}

func TestArrIncludeDirsIgnoresEmptyEntries(t *testing.T) {
	oldIncludeDirs := includeDirs
	oldWatchDirs := watchDirs
	oldProjectFolder := projectFolder
	t.Cleanup(func() {
		includeDirs = oldIncludeDirs
		watchDirs = oldWatchDirs
		projectFolder = oldProjectFolder
	})

	projectFolder = t.TempDir()
	includeDirs = []string{"", " , ", ".*"}
	watchDirs = nil

	arrIncludeDirs()

	if len(watchDirs) != 0 {
		t.Fatalf("expected empty entries to be skipped, got watchDirs=%v", watchDirs)
	}
}
