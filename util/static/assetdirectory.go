package static

import (
	"github.com/pkg/errors"
)

// AssetDirectory is a collection of file groups
type AssetDirectory struct {
	FileGroups map[string]*FileGroup
}

// NewAssetDirectory creates a new asset directory
func NewAssetDirectory() *AssetDirectory {
	return &AssetDirectory{
		FileGroups: make(map[string]*FileGroup),
	}
}

// NewFileGroup creates a new file group
func (a *AssetDirectory) NewFileGroup(baseDirectory string) (*FileGroup, error) {
	_, exists := a.FileGroups[baseDirectory]
	if exists {
		return nil, errors.Errorf("FileGroup '%s' already registered", baseDirectory)
	}
	result := &FileGroup{
		baseDirectory:  baseDirectory,
		assetDirectory: make(map[string]string),
	}
	a.FileGroups[baseDirectory] = result

	return result, nil
}

// GetGroup gets a filegroup by name. Returns nil if not found
func (a *AssetDirectory) GetGroup(groupName string) *FileGroup {
	return a.FileGroups[groupName]
}
