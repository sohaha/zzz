package watch

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fsnotify/fsnotify"
)

func TestReadDirEntriesSkipsSymlinkDirectories(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}

	link := filepath.Join(root, "linked")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink linked: %v", err)
	}

	entries, err := readDirEntries(root)
	if err != nil {
		t.Fatalf("readDirEntries: %v", err)
	}

	if _, ok := entries["linked"]; ok {
		t.Fatalf("expected symlink directory to be skipped, got entries=%v", entries)
	}
}

func TestPollingWatcherAddSkipsSymlinkDirectories(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "child.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatalf("write child file: %v", err)
	}

	link := filepath.Join(root, "linked")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink linked: %v", err)
	}

	w := &filePoller{
		events: make(chan fsnotify.Event),
		errors: make(chan error),
	}

	if err := w.Add(root); err != nil {
		t.Fatalf("Add root: %v", err)
	}
	defer func() {
		_ = w.Close()
	}()

	if _, ok := w.watches[link]; ok {
		t.Fatalf("expected symlink directory %q to be skipped", link)
	}
	if _, ok := w.watches[filepath.Join(link, "child.txt")]; ok {
		t.Fatalf("expected files under symlink directory %q to be skipped", link)
	}
}

func TestArrIncludeDirsSkipsConfiguredSymlinkDirectories(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	root, err := os.MkdirTemp(wd, "watch-symlink-*")
	if err != nil {
		t.Fatalf("mkdirtemp: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(root)
	})

	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}

	link := filepath.Join(root, "linked")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink linked: %v", err)
	}

	oldProjectFolder := projectFolder
	oldIncludeDirs := includeDirs
	oldWatchDirs := watchDirs
	oldExceptDirs := exceptDirs
	t.Cleanup(func() {
		projectFolder = oldProjectFolder
		includeDirs = oldIncludeDirs
		watchDirs = oldWatchDirs
		exceptDirs = oldExceptDirs
	})

	projectFolder = root
	includeDirs = []string{filepath.Base(root) + "/linked,*"}
	watchDirs = nil
	exceptDirs = nil

	arrIncludeDirs()

	if len(watchDirs) != 0 {
		t.Fatalf("expected configured symlink directory to be skipped, got watchDirs=%v", watchDirs)
	}
}
