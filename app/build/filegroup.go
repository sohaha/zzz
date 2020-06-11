package build

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"

	"github.com/pkg/errors"
	"github.com/sohaha/zlsgo/zstring"
	// "runtime"
)

// FileGroup holds a collection of files
type FileGroup struct {
	baseDirectory  string
	assetDirectory map[string][]byte
}

// AddAsset to the FileGroup
func (f *FileGroup) AddAsset(name, data string) error {
	_, exists := f.assetDirectory[name]
	if exists {
		return fmt.Errorf("asset '%s' already registered in FileGroup '%s'", name, f.baseDirectory)
	}
	f.assetDirectory[name] = zstring.String2Bytes(data)
	return nil
}

// AddAsset to the FileGroup
func (f *FileGroup) AddByteAsset(name string, data []byte) error {
	_, exists := f.assetDirectory[name]
	if exists {
		return fmt.Errorf("asset '%s' already registered in FileGroup '%s'", name, f.baseDirectory)
	}
	f.assetDirectory[name] = data
	return nil
}

// String returns the asset as a string
// Failure is indicated by a blank string.
// If you need hard failures, use MustString.
func (f *FileGroup) String(filename string) string {
	contents, _ := f.loadAsset(filename)
	return zstring.Bytes2String(contents)
}

// Bytes returns the asset as a Byte slice.
// Failure is indicated by a blank slice.
// If you need hard failures, use MustBytes.
func (f *FileGroup) Bytes(filename string) []byte {
	contents, _ := f.loadAsset(filename)
	return contents
}

// MustString returns the asset as a string.
// If the asset doesn't exist, it hard fails
func (f *FileGroup) MustString(filename string) (string, error) {
	contents, err := f.loadAsset(filename)
	return zstring.Bytes2String(contents), err
}

// MustBytes returns the asset as a string.
// If the asset doesn't exist, it hard fails
func (f *FileGroup) MustBytes(filename string) (contents []byte, err error) {
	contents, err = f.loadAsset(filename)
	return
}

// Entries returns a slice of filenames in the FileGroup
func (f *FileGroup) Entries() []string {
	keys := reflect.ValueOf(f.assetDirectory).MapKeys()
	result := []string{}
	for _, key := range keys {
		result = append(result, key.String())
	}
	return result
}

// Reset the FileGroup
func (f *FileGroup) Reset() {
	f.assetDirectory = make(map[string][]byte)
}

// All All
func (f *FileGroup) All() map[string][]byte {
	return f.assetDirectory
}

// loadAsset loads the asset for the given filename
func (f *FileGroup) loadAsset(filename string) (contents []byte, err error) {
	// Check internal
	storedAsset, ok := f.assetDirectory[filename]
	if ok {
		return DecompressHex(storedAsset)
	}

	// Get caller directory
	// fix ---
	// _, file, _, _ := runtime.Caller(3)
	// callerDir := filepath.Dir(file)
	// Calculate full path
	fullFilePath := filepath.Join(f.baseDirectory, filename)
	// fullFilePath := filepath.Join(callerDir, f.baseDirectory, filename)

	contents, err = ioutil.ReadFile(fullFilePath)
	if err != nil {
		err = errors.Errorf("The asset '%s' was not found!", filename)
	}
	return
}

func (f *FileGroup) GetBaseDir() string {
	return f.baseDirectory
}
