package static

import (
	"mime"
	"net/http"
	"path/filepath"

	"github.com/sohaha/zlsgo/zlog"
	"github.com/sohaha/zlsgo/znet"

	"github.com/sohaha/zstatic/build"
)

// mainAssetDirectory stores all the assets
var mainAssetDirectory = build.NewAssetDirectory()
var rootFileGroup *build.FileGroup
var err error

func init() {
	rootFileGroup, err = mainAssetDirectory.NewFileGroup(".")
	if err != nil {
		zlog.Fatal(err)
	}
}

// String gets the asset value by name
func String(name string) string {
	return rootFileGroup.String(name)
}

// MustString gets the asset value by name
func MustString(name string) (string, error) {
	return rootFileGroup.MustString(name)
}

// Bytes gets the asset value by name
func Bytes(name string) []byte {
	return rootFileGroup.Bytes(name)
}

// MustBytes gets the asset value by name
func MustBytes(name string) ([]byte, error) {
	return rootFileGroup.MustBytes(name)
}

// AddAsset adds the given asset to the root context
func AddAsset(groupName, name, value string) {
	fileGroup := mainAssetDirectory.GetGroup(groupName)
	if fileGroup == nil {
		fileGroup, err = mainAssetDirectory.NewFileGroup(groupName)
		if err != nil {
			zlog.Fatal(err)
		}
	}
	_ = fileGroup.AddAsset(name, value)
}

// AddByteAsset adds the given asset to the root context
func AddByteAsset(groupName, name string, value []byte) {
	fileGroup := mainAssetDirectory.GetGroup(groupName)
	if fileGroup == nil {
		fileGroup, err = mainAssetDirectory.NewFileGroup(groupName)
		if err != nil {
			zlog.Fatal(err)
		}
	}
	_ = fileGroup.AddByteAsset(name, value)
}

// Entries returns the file entries as a slice of filenames
func Entries() []string {
	return rootFileGroup.Entries()
}

// Reset clears the file entries
func Reset() {
	rootFileGroup.Reset()
}

// All All
func All() map[string][]byte {
	return rootFileGroup.All()
}

// Group holds a group of assets
func Group(name string) (result *build.FileGroup, err error) {
	result = mainAssetDirectory.GetGroup(name)
	if result == nil {
		result, err = mainAssetDirectory.NewFileGroup(name)
	}
	return
}

func NewFileserver(dir string, fn ...func(ctype string, content []byte, err error)) func(c *znet.Context) {
	const defFile = "index.html"
	f, _ := Group(dir)
	isCb := len(fn) > 0
	return func(c *znet.Context) {
		name := c.GetParam("file")
		if name == "" {
			name = defFile
		}
		content, err := f.MustBytes(name)
		ctype := mime.TypeByExtension(filepath.Ext(name))
		if isCb {
			fn[0](ctype, content, err)
			return
		}
		if err != nil {
			c.String(404, err.Error())
			return
		}
		c.SetContent(&znet.PrevData{
			Code:    http.StatusOK,
			Type:    ctype,
			Content: content,
		})

	}
}
