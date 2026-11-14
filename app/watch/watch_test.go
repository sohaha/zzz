package watch

import "testing"

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
