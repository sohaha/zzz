package static

import (
	"github.com/sohaha/zlsgo/znet"
	"mime"
	"path/filepath"

	// "github.com/sohaha/zlsgo/znet"
	lib "github.com/sohaha/zzz/util/static"
	"log"
)

// mainAssetDirectory stores all the assets
var mainAssetDirectory = lib.NewAssetDirectory()
var rootFileGroup *lib.FileGroup
var err error

func init() {
	rootFileGroup, err = mainAssetDirectory.NewFileGroup(".")
	if err != nil {
		log.Fatal(err)
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
			log.Fatal(err)
		}
	}
	_ = fileGroup.AddAsset(name, value)
}

// Entries returns the file entries as a slice of filenames
func Entries() []string {
	return rootFileGroup.Entries()
}

// Reset clears the file entries
func Reset() {
	rootFileGroup.Reset()
}

// Group holds a group of assets
func Group(name string) (result *lib.FileGroup, err error) {
	result = mainAssetDirectory.GetGroup(name)
	if result == nil {
		result, err = mainAssetDirectory.NewFileGroup(name)
	}
	return
}

func NewFileserver(dir string) func(c *znet.Context) {
	const defFile = "index.html"
	f, _ := Group(dir)
	return func(c *znet.Context) {
		name := c.GetParam("file")
		if name == "" {
			name = defFile
		}
		content, err := f.MustString(name)
		if err != nil {
			c.Engine.HandleNotFound(c, []znet.HandlerFunc{})
			return
		}
		ctype := mime.TypeByExtension(filepath.Ext(name))
		c.SetHeader("Content-Type", ctype)
		c.String(200, content)
	}
}
