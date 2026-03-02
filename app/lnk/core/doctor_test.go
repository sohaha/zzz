package core

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestIsPathTraversal(t *testing.T) {
	childPath := filepath.Join("..", "evil")
	tests := []struct {
		path string
		want bool
	}{
		{path: "..", want: true},
		{path: childPath, want: true},
		{path: "..vimrc", want: false},
		{path: filepath.Join("config", "..vimrc"), want: false},
	}

	for _, tt := range tests {
		if got := isPathTraversal(tt.path); got != tt.want {
			t.Fatalf("isPathTraversal(%q)=%v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestFindInvalidEntriesAllowsDoubleDotFilename(t *testing.T) {
	repoDir := t.TempDir()
	lnk := NewLnk(WithRepoPath(repoDir))

	validPath := "..vimrc"
	repoFile := filepath.Join(repoDir, validPath)
	if err := os.WriteFile(repoFile, []byte("ok"), 0o644); err != nil {
		t.Fatalf("write repo file failed: %v", err)
	}

	entries := []TrackedEntry{
		{Path: validPath, Type: LinkTypeSoft},
		{Path: filepath.Join("..", "evil"), Type: LinkTypeSoft},
	}

	invalid := lnk.findInvalidEntries(entries)
	if slices.Contains(invalid, validPath) {
		t.Fatalf("expected %q to be treated as valid filename", validPath)
	}
	if !slices.Contains(invalid, filepath.Join("..", "evil")) {
		t.Fatalf("expected traversal entry to be invalid")
	}
}
