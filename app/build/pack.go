package build

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/sohaha/zlsgo/zfile"
	"github.com/sohaha/zlsgo/zstring"
	zstatic "github.com/sohaha/zstatic/build"
)

func Basename(pwd string) string {
	name := filepath.Base(pwd)
	if zfile.FileExist(pwd + "go.mod") {
		content, _ := ioutil.ReadFile(pwd + "go.mod")
		str, err := zstring.RegexExtract(`module (.*)`, zstring.Bytes2String(content))
		if err == nil && len(str) > 0 {
			p := strings.Split(str[1], "/")
			name = p[len(p)-1]
		}
	}
	return name
}

func getAllFilesInDirectory(dir string) ([]string, error) {
	var result []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		path = filepath.ToSlash(path)
		// check if it is a regular file (not dir)
		if info.Mode().IsRegular() {
			result = append(result, path)
		}
		return nil
	})

	return result, err
}

// GeneratePackFileString creates the contents of a pack file
func GeneratePackFileString(assetBundle *ReferencedAssets, ignoreErrors bool) (string, error) {
	var filesProcessed = make(map[string]bool)
	result := fmt.Sprintf("package %s\n\n", assetBundle.PackageName)
	if len(assetBundle.Groups) > 0 || len(assetBundle.Assets) > 0 {
		result += "import \"github.com/sohaha/zstatic\"\n\n"
		result += "func init() {\n"
		for _, group := range assetBundle.Groups {
			// Read all assets from the directory
			files, err := getAllFilesInDirectory(group.FullPath)
			groupPrefix := filepath.ToSlash(group.FullPath)
			if err != nil {
				return "", err
			}
			for _, file := range files {
				// Read in File
				packedData, err := zstatic.CompressFile(file)
				if err != nil && !ignoreErrors {
					return "", err
				}
				localPath := strings.TrimPrefix(file, groupPrefix+"/")
				result += fmt.Sprintf("  zstatic.AddByteAsset(\"%s\", \"%s\",%#v)\n", group.LocalPath, localPath, packedData)
				// result += fmt.Sprintf("  zstatic.AddAsset(\"%s\", \"%s\", \"%s\")\n", groupPrefix, localPath, packedData)
				filesProcessed[file] = true
				// fmt.Printf("Packed: %s\n", file)
			}
		}
		for _, asset := range assetBundle.Assets {
			groupPath := "."
			if asset.Group != nil {
				groupPath = asset.Group.LocalPath
			}
			fullPath, err := filepath.Abs(filepath.Join(groupPath, asset.AssetPath))
			if err != nil {
				return "", err
			}
			// if _, exists := filesProcessed[fullPath]; exists == true {
			// 	continue
			// }
			packedData, err := zstatic.CompressFile(fullPath)
			if err != nil && !ignoreErrors {
				return "", err
			}
			result += fmt.Sprintf("  zstatic.AddByteAsset(\".\", \"%s\", %#v)\n", asset.Name, packedData)
			filesProcessed[fullPath] = true
		}
		result += "}\n"
	}
	return result, nil
}
